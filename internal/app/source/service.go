package source

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"hash/fnv"
	"strings"
	"sync"
	"time"

	domainaccount "telegram-message-forward/internal/domain/account"
	domainmessage "telegram-message-forward/internal/domain/message"
	domainsource "telegram-message-forward/internal/domain/source"
	pluginsource "telegram-message-forward/internal/plugin/source"
)

// minFullSyncInterval 是两次「全量网络同步」之间的最短间隔。
//
// messages.getDialogs 在 Telegram 服务端有固定冷却（约 25~30 秒/次），账号 dialog 越多、
// 需要翻的页数越多，一次全量同步可能耗时十几分钟。短时间内重复点「同步」只会重复触发这个
// 耗时过程，因此默认在冷却窗口内改为只返回缓存，除非调用方显式要求 force 全量刷新。
const minFullSyncInterval = 15 * time.Minute

var (
	ErrWebhookDisabled     = errors.New("webhook source 未启用")
	ErrWebhookUnauthorized = errors.New("webhook token 无效")
)

// Service 是监听源应用服务（CRUD + 同步 + 启停）。
type Service struct {
	sources  domainsource.Repository
	accounts domainaccount.Repository
	plugins  map[string]pluginsource.Plugin
	manager  *Manager

	syncMu       sync.Mutex
	lastFullSync map[int64]time.Time // account_id -> 上次全量网络同步完成时间
}

type RuntimeStatus struct {
	Status            string
	SubscriptionCount int
	RecentMessageAt   *time.Time
	LastError         string
}

// NewService 创建监听源服务。
func NewService(
	sources domainsource.Repository,
	accounts domainaccount.Repository,
	plugin pluginsource.Plugin,
	manager *Manager,
) *Service {
	return &Service{
		sources:      sources,
		accounts:     accounts,
		plugins:      map[string]pluginsource.Plugin{"telegram": plugin},
		manager:      manager,
		lastFullSync: map[int64]time.Time{},
	}
}

// RegisterPlugin 注册非 Telegram Source 插件。
func (s *Service) RegisterPlugin(name string, plugin pluginsource.Plugin) {
	if s.plugins == nil {
		s.plugins = map[string]pluginsource.Plugin{}
	}
	s.plugins[name] = plugin
}

// List 返回全部监听源。
func (s *Service) List(ctx context.Context) ([]*domainsource.Source, error) {
	return s.sources.List(ctx)
}

// RuntimeStatusBySource 返回当前插件 runner 映射到 source 的运行状态。
func (s *Service) RuntimeStatusBySource() map[int64]RuntimeStatus {
	out := map[int64]RuntimeStatus{}
	for _, plugin := range s.plugins {
		provider, ok := plugin.(pluginsource.RunnerStatusProvider)
		if !ok {
			continue
		}
		for _, status := range provider.RunnerStatuses() {
			mapped := RuntimeStatus{
				Status:            status.Status,
				SubscriptionCount: status.SubscriptionCount,
				RecentMessageAt:   status.RecentMessageAt,
				LastError:         status.LastError,
			}
			for _, sourceID := range status.SourceIDs {
				out[sourceID] = mapped
			}
		}
	}
	return out
}

// Get 查询单个监听源。
func (s *Service) Get(ctx context.Context, id int64) (*domainsource.Source, error) {
	return s.sources.GetByID(ctx, id)
}

// CreateInput 是创建监听源的输入。
type CreateInput struct {
	Type      string
	AccountID int64
	PeerType  string
	PeerID    int64
	Name      string
	Username  string
	Enabled   bool
	Config    map[string]any
}

// Create 创建监听源。
func (s *Service) Create(ctx context.Context, in CreateInput) (*domainsource.Source, error) {
	sourceType := normalizeSourceType(in.Type)
	if sourceType == "rss" {
		if in.PeerType == "" {
			in.PeerType = "feed"
		}
		if in.PeerID == 0 {
			in.PeerID = stablePositiveID(configString(in.Config, "feed_url"))
		}
	}
	if sourceType == "webhook" {
		if in.PeerType == "" {
			in.PeerType = "webhook"
		}
		if in.PeerID == 0 {
			in.PeerID = stablePositiveID(in.Name + ":" + configString(in.Config, "token"))
		}
	}
	if sourceType == "telegram" && in.AccountID <= 0 {
		return nil, fmt.Errorf("telegram source 缺少 account_id")
	}
	if sourceType == "telegram" && in.PeerType == "" {
		return nil, fmt.Errorf("telegram source 缺少 peer_type")
	}
	if sourceType == "telegram" && in.PeerID == 0 {
		return nil, fmt.Errorf("telegram source 缺少 peer_id")
	}
	src := &domainsource.Source{
		Type:      sourceType,
		AccountID: in.AccountID,
		PeerType:  domainsource.PeerType(in.PeerType),
		PeerID:    in.PeerID,
		Name:      in.Name,
		Username:  in.Username,
		Enabled:   in.Enabled,
		Config:    in.Config,
	}
	plugin, err := s.pluginForSource(src)
	if err != nil {
		return nil, err
	}
	if err := plugin.ValidateConfig(src.Config); err != nil {
		return nil, err
	}
	if err := s.sources.Create(ctx, src); err != nil {
		return nil, err
	}
	if src.Enabled {
		if err := s.startSource(ctx, src); err != nil {
			return nil, fmt.Errorf("source 已保存但启动失败: %w", err)
		}
	}
	return src, nil
}

// UpdateInput 是更新监听源的输入。
type UpdateInput struct {
	Name    *string
	Enabled *bool
	Config  map[string]any
}

// Update 更新监听源；enabled 变化时联动启停监听。
func (s *Service) Update(ctx context.Context, id int64, in UpdateInput) (*domainsource.Source, error) {
	src, err := s.sources.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if in.Name != nil {
		src.Name = *in.Name
	}
	if in.Config != nil {
		src.Config = in.Config
	}
	toggled := false
	if in.Enabled != nil && *in.Enabled != src.Enabled {
		src.Enabled = *in.Enabled
		toggled = true
	}
	if err := s.sources.Update(ctx, src); err != nil {
		return nil, err
	}
	if toggled {
		if src.Enabled {
			if err := s.startSource(ctx, src); err != nil {
				return nil, err
			}
		} else {
			if err := s.stopSource(ctx, src); err != nil {
				return nil, err
			}
		}
	}
	return src, nil
}

// Delete 删除监听源（先停止监听）。
func (s *Service) Delete(ctx context.Context, id int64) error {
	src, err := s.sources.GetByID(ctx, id)
	if err == nil && src != nil {
		if plugin, perr := s.pluginForSource(src); perr == nil {
			_ = plugin.Stop(ctx, src)
		}
	}
	return s.sources.Delete(ctx, id)
}

// Sync 拉取某账号可见的 chats/channels/users（用于 UI 选择监听源）。
func (s *Service) Sync(ctx context.Context, accountID int64, force bool) ([]pluginsource.SyncedPeer, error) {
	var peers []pluginsource.SyncedPeer
	if _, err := s.SyncStream(ctx, accountID, force, func(peer pluginsource.SyncedPeer) error {
		peers = append(peers, peer)
		return nil
	}); err != nil {
		return nil, err
	}
	return peers, nil
}

// SyncStream 流式拉取某账号可见的 chats/channels/users（用于 UI 增量展示）。
//
// 非 force 且距上次全量同步不足 minFullSyncInterval 时，只回放缓存 peer（不走网络），
// 避免短时间内重复触发耗时的全量分页拉取；返回值 usedCache 标记本次是否走了缓存快路径。
func (s *Service) SyncStream(ctx context.Context, accountID int64, force bool, emit pluginsource.SyncPeerHandler) (usedCache bool, err error) {
	acc, err := s.accounts.GetByID(ctx, accountID)
	if err != nil {
		return false, err
	}
	if acc.Status != domainaccount.StatusActive {
		return false, fmt.Errorf("账号未登录，无法同步（当前状态 %s）", acc.Status)
	}

	if !force && s.withinFullSyncCooldown(accountID) {
		telegramPlugin := s.plugins["telegram"]
		if cacher, ok := telegramPlugin.(interface {
			SyncCachedPeers(context.Context, *domainaccount.Account, pluginsource.SyncPeerHandler) error
		}); ok {
			return true, cacher.SyncCachedPeers(ctx, acc, emit)
		}
	}

	telegramPlugin := s.plugins["telegram"]
	if streaming, ok := telegramPlugin.(interface {
		SyncSourcesStream(context.Context, *domainaccount.Account, pluginsource.SyncPeerHandler) error
	}); ok {
		if err := streaming.SyncSourcesStream(ctx, acc, emit); err != nil {
			return false, err
		}
		s.markFullSynced(accountID)
		return false, nil
	}

	peers, err := telegramPlugin.SyncSources(ctx, acc)
	if err != nil {
		return false, err
	}
	for _, peer := range peers {
		if err := emit(peer); err != nil {
			return false, err
		}
	}
	s.markFullSynced(accountID)
	return false, nil
}

func (s *Service) withinFullSyncCooldown(accountID int64) bool {
	s.syncMu.Lock()
	defer s.syncMu.Unlock()
	last, ok := s.lastFullSync[accountID]
	return ok && time.Since(last) < minFullSyncInterval
}

func (s *Service) markFullSynced(accountID int64) {
	s.syncMu.Lock()
	defer s.syncMu.Unlock()
	s.lastFullSync[accountID] = time.Now()
}

// Start 启动某监听源。
func (s *Service) Start(ctx context.Context, id int64) error {
	src, err := s.sources.GetByID(ctx, id)
	if err != nil {
		return err
	}
	return s.startSource(ctx, src)
}

func (s *Service) startSource(ctx context.Context, src *domainsource.Source) error {
	plugin, err := s.pluginForSource(src)
	if err != nil {
		return err
	}
	var acc *domainaccount.Account
	if sourceRequiresAccount(src) {
		acc, err = s.accounts.GetByID(ctx, src.AccountID)
		if err != nil {
			return err
		}
		if acc.Status != domainaccount.StatusActive {
			return fmt.Errorf("账号未登录，无法启动监听（当前状态 %s）", acc.Status)
		}
	}
	if s.manager == nil || s.manager.ingest == nil {
		return fmt.Errorf("source manager 未初始化")
	}
	return plugin.Start(ctx, acc, src, s.manager.ingest.Ingest)
}

// Stop 停止某监听源。
func (s *Service) Stop(ctx context.Context, id int64) error {
	src, err := s.sources.GetByID(ctx, id)
	if err != nil {
		return err
	}
	return s.stopSource(ctx, src)
}

func (s *Service) stopSource(ctx context.Context, src *domainsource.Source) error {
	plugin, err := s.pluginForSource(src)
	if err != nil {
		return err
	}
	return plugin.Stop(ctx, src)
}

// ReceiveWebhook 校验并接收外部 Webhook 请求。
func (s *Service) ReceiveWebhook(ctx context.Context, id int64, req pluginsource.WebhookRequest) (*domainmessage.NormalizedMessage, error) {
	src, err := s.sources.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if normalizeSourceType(src.Type) != "webhook" {
		return nil, fmt.Errorf("source %d 不是 webhook 类型", id)
	}
	if !src.Enabled {
		return nil, ErrWebhookDisabled
	}
	expected := configString(src.Config, "token")
	if expected == "" || !sameToken(expected, webhookToken(req)) {
		return nil, ErrWebhookUnauthorized
	}
	plugin, err := s.pluginForSource(src)
	if err != nil {
		return nil, err
	}
	receiver, ok := plugin.(pluginsource.WebhookReceiver)
	if !ok {
		return nil, fmt.Errorf("source 插件不支持 webhook 接收: %s", normalizeSourceType(src.Type))
	}
	msg, err := receiver.Receive(ctx, src, req)
	if err != nil {
		return nil, err
	}
	if err := s.manager.ingest.Ingest(ctx, msg); err != nil {
		return nil, err
	}
	return msg, nil
}

func (s *Service) pluginForSource(src *domainsource.Source) (pluginsource.Plugin, error) {
	sourceType := normalizeSourceType(src.Type)
	plugin, ok := s.plugins[sourceType]
	if !ok {
		return nil, fmt.Errorf("未注册的 source 插件: %s", sourceType)
	}
	return plugin, nil
}

func normalizeSourceType(t string) string {
	if t == "" {
		return "telegram"
	}
	return t
}

func sourceRequiresAccount(src *domainsource.Source) bool {
	return normalizeSourceType(src.Type) == "telegram"
}

func configString(config map[string]any, key string) string {
	v, _ := config[key].(string)
	return strings.TrimSpace(v)
}

func stablePositiveID(value string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(value))
	return int64(h.Sum64() & 0x7fffffffffffffff)
}

func webhookToken(req pluginsource.WebhookRequest) string {
	for _, key := range []string{"X-TMF-Webhook-Token", "x-tmf-webhook-token"} {
		if v := strings.TrimSpace(req.Headers[key]); v != "" {
			return v
		}
	}
	if values := req.Query["token"]; len(values) > 0 {
		return strings.TrimSpace(values[0])
	}
	return ""
}

func sameToken(expected string, got string) bool {
	if expected == "" || got == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(expected), []byte(got)) == 1
}
