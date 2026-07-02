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

const syncDialogsPageSize = 100

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

// SyncSources 拉取账号可见的 chats/channels/users 并返回同步结果。
func (p *Plugin) SyncSources(ctx context.Context, acc *domainaccount.Account) ([]pluginsource.SyncedPeer, error) {
	var peers []pluginsource.SyncedPeer
	if err := p.SyncSourcesStream(ctx, acc, func(peer pluginsource.SyncedPeer) error {
		peers = append(peers, peer)
		return nil
	}); err != nil {
		return nil, err
	}
	return peers, nil
}

// SyncSourcesStream 轻量拉取账号会话列表，并把 peer 增量返回给调用方。
//
// 这里只同步 Telegram 客户端左侧会话列表里的 user/chat/channel，不拉群成员、不拉历史消息。
// messages.getDialogs 响应会附带顶部消息用于分页 offset，但不会展开加载每个会话内部消息。
func (p *Plugin) SyncSourcesStream(ctx context.Context, acc *domainaccount.Account, emit pluginsource.SyncPeerHandler) error {
	client, err := p.buildClient(acc, nil)
	if err != nil {
		return err
	}

	runErr := client.Run(ctx, func(ctx context.Context) error {
		if err := p.ensureAuthorized(ctx, client); err != nil {
			return err
		}
		offsetPeer := tg.InputPeerClass(&tg.InputPeerEmpty{})
		offsetID := 0
		offsetDate := 0
		seen := map[string]struct{}{}

		for {
			res, err := client.API().MessagesGetDialogs(ctx, &tg.MessagesGetDialogsRequest{
				Limit:      syncDialogsPageSize,
				OffsetPeer: offsetPeer,
				OffsetID:   offsetID,
				OffsetDate: offsetDate,
			})
			if err != nil {
				return fmt.Errorf("拉取会话列表失败: %w", err)
			}
			dialogs, messages, chats, users, hasMore, err := unpackDialogs(res)
			if err != nil {
				return err
			}
			if err := p.emitDialogPeers(dialogs, chats, users, seen, emit); err != nil {
				return err
			}
			if !hasMore || len(dialogs) == 0 {
				break
			}
			nextPeer, nextID, nextDate := nextDialogOffset(dialogs, messages, chats, users)
			if nextPeer == nil || nextID == 0 {
				break
			}
			offsetPeer, offsetID, offsetDate = nextPeer, nextID, nextDate
		}
		return nil
	})
	if runErr != nil {
		return runErr
	}
	return nil
}

func unpackDialogs(res tg.MessagesDialogsClass) ([]tg.DialogClass, []tg.MessageClass, []tg.ChatClass, []tg.UserClass, bool, error) {
	switch v := res.(type) {
	case *tg.MessagesDialogs:
		return v.Dialogs, v.Messages, v.Chats, v.Users, false, nil
	case *tg.MessagesDialogsSlice:
		return v.Dialogs, v.Messages, v.Chats, v.Users, len(v.Dialogs) >= syncDialogsPageSize && len(v.Dialogs) < v.Count, nil
	default:
		return nil, nil, nil, nil, false, fmt.Errorf("未预期的会话列表类型: %T", res)
	}
}

// emitDialogPeers 只返回 dialogs 中真实存在的会话 peer。
//
// messages.getDialogs 响应里的 users 可能包含顶部消息相关用户；不能直接全量返回 users，
// 否则会把非私聊会话的人混进“可监听 peer”列表。
func (p *Plugin) emitDialogPeers(dialogs []tg.DialogClass, chats []tg.ChatClass, users []tg.UserClass, seen map[string]struct{}, emit pluginsource.SyncPeerHandler) error {
	for _, d := range dialogs {
		switch peer := d.GetPeer().(type) {
		case *tg.PeerChannel:
			ch := findChannel(chats, peer.ChannelID)
			if ch == nil || !markSeen(seen, domainpeer.TypeChannel, ch.ID) {
				continue
			}
			if err := emit(pluginsource.SyncedPeer{
				PeerType: domainsource.PeerChannel, PeerID: ch.ID, Name: ch.Title, Username: ch.Username,
			}); err != nil {
				return err
			}
		case *tg.PeerChat:
			ch := findChat(chats, peer.ChatID)
			if ch == nil || !markSeen(seen, domainpeer.TypeChat, ch.ID) {
				continue
			}
			if err := emit(pluginsource.SyncedPeer{
				PeerType: domainsource.PeerChat, PeerID: ch.ID, Name: ch.Title,
			}); err != nil {
				return err
			}
		case *tg.PeerUser:
			user := findUser(users, peer.UserID)
			if user == nil || !markSeen(seen, domainpeer.TypeUser, user.ID) {
				continue
			}
			if err := emit(pluginsource.SyncedPeer{
				PeerType: domainsource.PeerUser, PeerID: user.ID, Name: userDisplayName(user), Username: user.Username,
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

func markSeen(seen map[string]struct{}, t domainpeer.Type, id int64) bool {
	key := fmt.Sprintf("%s:%d", t, id)
	if _, ok := seen[key]; ok {
		return false
	}
	seen[key] = struct{}{}
	return true
}

func nextDialogOffset(dialogs []tg.DialogClass, messages []tg.MessageClass, chats []tg.ChatClass, users []tg.UserClass) (tg.InputPeerClass, int, int) {
	if len(dialogs) == 0 {
		return nil, 0, 0
	}
	last := dialogs[len(dialogs)-1]
	topID := last.GetTopMessage()
	input := inputPeerFromPeer(last.GetPeer(), chats, users)
	date := messageDate(messages, topID)
	return input, topID, date
}

func inputPeerFromPeer(peer tg.PeerClass, chats []tg.ChatClass, users []tg.UserClass) tg.InputPeerClass {
	switch p := peer.(type) {
	case *tg.PeerChannel:
		if ch := findChannel(chats, p.ChannelID); ch != nil {
			return &tg.InputPeerChannel{ChannelID: ch.ID, AccessHash: ch.AccessHash}
		}
	case *tg.PeerChat:
		return &tg.InputPeerChat{ChatID: p.ChatID}
	case *tg.PeerUser:
		if u := findUser(users, p.UserID); u != nil {
			return &tg.InputPeerUser{UserID: u.ID, AccessHash: u.AccessHash}
		}
	}
	return nil
}

func findChannel(chats []tg.ChatClass, id int64) *tg.Channel {
	for _, c := range chats {
		if ch, ok := c.(*tg.Channel); ok && ch.ID == id {
			return ch
		}
	}
	return nil
}

func findChat(chats []tg.ChatClass, id int64) *tg.Chat {
	for _, c := range chats {
		if ch, ok := c.(*tg.Chat); ok && ch.ID == id {
			return ch
		}
	}
	return nil
}

func findUser(users []tg.UserClass, id int64) *tg.User {
	for _, u := range users {
		if user, ok := u.(*tg.User); ok && user.ID == id {
			return user
		}
	}
	return nil
}

func messageDate(messages []tg.MessageClass, id int) int {
	for _, m := range messages {
		if msg, ok := m.(*tg.Message); ok && msg.ID == id {
			return msg.Date
		}
	}
	return 0
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
