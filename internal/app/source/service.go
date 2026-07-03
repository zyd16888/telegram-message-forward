package source

import (
	"context"
	"fmt"
	"sync"
	"time"

	domainaccount "telegram-message-forward/internal/domain/account"
	domainsource "telegram-message-forward/internal/domain/source"
	pluginsource "telegram-message-forward/internal/plugin/source"
)

// minFullSyncInterval 是两次「全量网络同步」之间的最短间隔。
//
// messages.getDialogs 在 Telegram 服务端有固定冷却（约 25~30 秒/次），账号 dialog 越多、
// 需要翻的页数越多，一次全量同步可能耗时十几分钟。短时间内重复点「同步」只会重复触发这个
// 耗时过程，因此默认在冷却窗口内改为只返回缓存，除非调用方显式要求 force 全量刷新。
const minFullSyncInterval = 15 * time.Minute

// Service 是监听源应用服务（CRUD + 同步 + 启停）。
type Service struct {
	sources  domainsource.Repository
	accounts domainaccount.Repository
	plugin   pluginsource.Plugin
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
		plugin:       plugin,
		manager:      manager,
		lastFullSync: map[int64]time.Time{},
	}
}

// List 返回全部监听源。
func (s *Service) List(ctx context.Context) ([]*domainsource.Source, error) {
	return s.sources.List(ctx)
}

// RuntimeStatusBySource 返回当前插件 runner 映射到 source 的运行状态。
func (s *Service) RuntimeStatusBySource() map[int64]RuntimeStatus {
	provider, ok := s.plugin.(pluginsource.RunnerStatusProvider)
	if !ok {
		return nil
	}
	out := map[int64]RuntimeStatus{}
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
	return out
}

// Get 查询单个监听源。
func (s *Service) Get(ctx context.Context, id int64) (*domainsource.Source, error) {
	return s.sources.GetByID(ctx, id)
}

// CreateInput 是创建监听源的输入。
type CreateInput struct {
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
	src := &domainsource.Source{
		AccountID: in.AccountID,
		PeerType:  domainsource.PeerType(in.PeerType),
		PeerID:    in.PeerID,
		Name:      in.Name,
		Username:  in.Username,
		Enabled:   in.Enabled,
		Config:    in.Config,
	}
	if err := s.sources.Create(ctx, src); err != nil {
		return nil, err
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
			_ = s.Start(ctx, src.ID)
		} else {
			_ = s.Stop(ctx, src.ID)
		}
	}
	return src, nil
}

// Delete 删除监听源（先停止监听）。
func (s *Service) Delete(ctx context.Context, id int64) error {
	src, err := s.sources.GetByID(ctx, id)
	if err == nil && src != nil {
		_ = s.plugin.Stop(ctx, src)
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
		if cacher, ok := s.plugin.(interface {
			SyncCachedPeers(context.Context, *domainaccount.Account, pluginsource.SyncPeerHandler) error
		}); ok {
			return true, cacher.SyncCachedPeers(ctx, acc, emit)
		}
	}

	if streaming, ok := s.plugin.(interface {
		SyncSourcesStream(context.Context, *domainaccount.Account, pluginsource.SyncPeerHandler) error
	}); ok {
		if err := streaming.SyncSourcesStream(ctx, acc, emit); err != nil {
			return false, err
		}
		s.markFullSynced(accountID)
		return false, nil
	}

	peers, err := s.plugin.SyncSources(ctx, acc)
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
	acc, err := s.accounts.GetByID(ctx, src.AccountID)
	if err != nil {
		return err
	}
	if acc.Status != domainaccount.StatusActive {
		return fmt.Errorf("账号未登录，无法启动监听（当前状态 %s）", acc.Status)
	}
	return s.plugin.Start(ctx, acc, src, s.manager.ingest.Ingest)
}

// Stop 停止某监听源。
func (s *Service) Stop(ctx context.Context, id int64) error {
	src, err := s.sources.GetByID(ctx, id)
	if err != nil {
		return err
	}
	return s.plugin.Stop(ctx, src)
}
