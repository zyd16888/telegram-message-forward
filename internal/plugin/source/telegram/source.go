package telegram

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/gotd/td/telegram"
	"github.com/gotd/td/tg"

	domainaccount "telegram-message-forward/internal/domain/account"
	domainpeer "telegram-message-forward/internal/domain/peer"
	domainsource "telegram-message-forward/internal/domain/source"
	infratelegram "telegram-message-forward/internal/infra/telegram"
	pluginsource "telegram-message-forward/internal/plugin/source"
)

// Deps 是 Telegram Source 插件的依赖，由 bootstrap 注入（避免 plugin 直连存储层）。
type Deps struct {
	Peers domainpeer.Repository
	Log   *slog.Logger

	// LoadSession 返回账号明文 session（无则返回 nil）。
	LoadSession func(ctx context.Context, accountID int64) ([]byte, error)
	// SaveSession 持久化账号明文 session（由实现方负责加密落库）。
	SaveSession func(ctx context.Context, accountID int64, session []byte) error
}

// Plugin 实现 source.Plugin。
//
// 注意：Telegram 客户端本质是「每账号一个」；本实现按 source 粒度 Start 一个客户端并按 peer 过滤，
// 适合 v1 单账号少量 source。多 source 共账号的连接复用后置到 SourceManager。
type Plugin struct {
	deps    Deps
	mu      sync.Mutex
	running map[int64]context.CancelFunc // source_id -> cancel
}

// NewPlugin 创建 Telegram Source 插件。
func NewPlugin(deps Deps) *Plugin {
	if deps.Log == nil {
		deps.Log = slog.Default()
	}
	return &Plugin{deps: deps, running: map[int64]context.CancelFunc{}}
}

var _ pluginsource.Plugin = (*Plugin)(nil)

// Name 返回插件名。
func (p *Plugin) Name() string { return "telegram" }

// Capabilities 声明能力。
func (p *Plugin) Capabilities() pluginsource.Capabilities {
	return pluginsource.Capabilities{SupportsSync: true, SupportsMedia: true, SupportsHistory: true}
}

// ValidateConfig 目前无额外配置校验。
func (p *Plugin) ValidateConfig(map[string]any) error { return nil }

// buildClient 按账号构建客户端。update 为 nil 时用于一次性 API 调用（如同步）。
func (p *Plugin) buildClient(acc *domainaccount.Account, update telegram.UpdateHandler) (*telegram.Client, error) {
	store := newAccountSession(acc.ID, p.deps.LoadSession, p.deps.SaveSession)
	// 预填账号当前已知 session，避免首连额外读库。
	if len(acc.Session) > 0 {
		store.set(acc.Session)
	}
	return infratelegram.NewClient(infratelegram.ClientConfig{
		AppID:         acc.AppID,
		AppHash:       acc.AppHash,
		Proxy:         acc.Proxy,
		SessionStore:  store,
		UpdateHandler: update,
	})
}

// SyncSources 拉取账号可见的 chats/channels/users，缓存 access_hash 并返回同步结果。
func (p *Plugin) SyncSources(ctx context.Context, acc *domainaccount.Account) ([]pluginsource.SyncedPeer, error) {
	client, err := p.buildClient(acc, nil)
	if err != nil {
		return nil, err
	}

	var peers []pluginsource.SyncedPeer
	runErr := client.Run(ctx, func(ctx context.Context) error {
		if err := p.ensureAuthorized(ctx, client); err != nil {
			return err
		}
		res, err := client.API().MessagesGetDialogs(ctx, &tg.MessagesGetDialogsRequest{
			Limit:      100,
			OffsetPeer: &tg.InputPeerEmpty{},
		})
		if err != nil {
			return fmt.Errorf("拉取会话列表失败: %w", err)
		}
		var chats []tg.ChatClass
		var users []tg.UserClass
		switch v := res.(type) {
		case *tg.MessagesDialogs:
			chats, users = v.Chats, v.Users
		case *tg.MessagesDialogsSlice:
			chats, users = v.Chats, v.Users
		default:
			return fmt.Errorf("未预期的会话列表类型: %T", res)
		}
		peers = p.collectPeers(ctx, acc.ID, chats, users)
		return nil
	})
	if runErr != nil {
		return nil, runErr
	}
	return peers, nil
}

// collectPeers 缓存 access_hash 并组装 SyncedPeer；单条缓存失败不阻断整体同步。
func (p *Plugin) collectPeers(ctx context.Context, accountID int64, chats []tg.ChatClass, users []tg.UserClass) []pluginsource.SyncedPeer {
	var peers []pluginsource.SyncedPeer
	for _, c := range chats {
		switch ch := c.(type) {
		case *tg.Channel:
			p.cachePeer(ctx, accountID, domainpeer.TypeChannel, ch.ID, ch.AccessHash, ch.Username, ch.Title)
			peers = append(peers, pluginsource.SyncedPeer{
				PeerType: domainsource.PeerChannel, PeerID: ch.ID, Name: ch.Title, Username: ch.Username,
			})
		case *tg.Chat:
			// 基础群组无 access_hash。
			p.cachePeer(ctx, accountID, domainpeer.TypeChat, ch.ID, 0, "", ch.Title)
			peers = append(peers, pluginsource.SyncedPeer{
				PeerType: domainsource.PeerChat, PeerID: ch.ID, Name: ch.Title,
			})
		}
	}
	for _, u := range users {
		user, ok := u.(*tg.User)
		if !ok {
			continue
		}
		p.cachePeer(ctx, accountID, domainpeer.TypeUser, user.ID, user.AccessHash, user.Username, userDisplayName(user))
		peers = append(peers, pluginsource.SyncedPeer{
			PeerType: domainsource.PeerUser, PeerID: user.ID, Name: userDisplayName(user), Username: user.Username,
		})
	}
	return peers
}

func (p *Plugin) cachePeer(ctx context.Context, accountID int64, t domainpeer.Type, id, accessHash int64, username, title string) {
	if p.deps.Peers == nil {
		return
	}
	err := p.deps.Peers.Upsert(ctx, &domainpeer.Peer{
		AccountID: accountID, PeerType: t, PeerID: id, AccessHash: accessHash, Username: username, Title: title,
	})
	if err != nil {
		p.deps.Log.Warn("缓存 peer access_hash 失败", "account", accountID, "peer", id, "err", err)
	}
}

// Start 为一个 source 启动监听：连接账号客户端，过滤该 source 的消息并回调 handler。
func (p *Plugin) Start(_ context.Context, acc *domainaccount.Account, src *domainsource.Source, handler pluginsource.Handler) error {
	runCtx, cancel := context.WithCancel(context.Background())

	forward := func(runCtx context.Context, e tg.Entities, m tg.MessageClass) {
		msg, ok := m.(*tg.Message)
		if !ok || !matchesSource(msg, src) {
			return
		}
		nm := Normalize(src.ID, msg, e)
		if err := handler(runCtx, nm); err != nil {
			p.deps.Log.Error("处理 Telegram 消息失败", "source", src.ID, "err", err)
		}
	}

	dispatcher := tg.NewUpdateDispatcher()
	dispatcher.OnNewChannelMessage(func(ctx context.Context, e tg.Entities, u *tg.UpdateNewChannelMessage) error {
		forward(runCtx, e, u.Message)
		return nil
	})
	dispatcher.OnNewMessage(func(ctx context.Context, e tg.Entities, u *tg.UpdateNewMessage) error {
		forward(runCtx, e, u.Message)
		return nil
	})

	client, err := p.buildClient(acc, dispatcher)
	if err != nil {
		cancel()
		return err
	}

	p.mu.Lock()
	if old, exists := p.running[src.ID]; exists {
		old()
	}
	p.running[src.ID] = cancel
	p.mu.Unlock()

	go func() {
		err := client.Run(runCtx, func(ctx context.Context) error {
			if err := p.ensureAuthorized(ctx, client); err != nil {
				return err
			}
			p.deps.Log.Info("Telegram source 监听中", "source", src.ID, "peer", src.PeerID)
			<-ctx.Done()
			return ctx.Err()
		})
		if err != nil && !errors.Is(err, context.Canceled) {
			p.deps.Log.Error("Telegram source 运行退出", "source", src.ID, "err", err)
		}
	}()

	return nil
}

// Stop 停止某 source 的监听。
func (p *Plugin) Stop(_ context.Context, src *domainsource.Source) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if cancel, ok := p.running[src.ID]; ok {
		cancel()
		delete(p.running, src.ID)
	}
	return nil
}

func (p *Plugin) ensureAuthorized(ctx context.Context, client *telegram.Client) error {
	st, err := infratelegram.Status(ctx, client)
	if err != nil {
		return fmt.Errorf("查询登录状态失败: %w", err)
	}
	if st == nil || !st.Authorized {
		return errors.New("账号未登录，请先通过 cmd/login 完成登录")
	}
	return nil
}

// matchesSource 判断消息是否来自指定 source 的 peer。
func matchesSource(msg *tg.Message, src *domainsource.Source) bool {
	switch p := msg.PeerID.(type) {
	case *tg.PeerChannel:
		return src.PeerType == domainsource.PeerChannel && p.ChannelID == src.PeerID
	case *tg.PeerChat:
		return src.PeerType == domainsource.PeerChat && p.ChatID == src.PeerID
	case *tg.PeerUser:
		return src.PeerType == domainsource.PeerUser && p.UserID == src.PeerID
	}
	return false
}
