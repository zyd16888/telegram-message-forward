package source

import (
	"context"
	"fmt"

	domainaccount "telegram-message-forward/internal/domain/account"
	domainsource "telegram-message-forward/internal/domain/source"
	pluginsource "telegram-message-forward/internal/plugin/source"
)

// Service 是监听源应用服务（CRUD + 同步 + 启停）。
type Service struct {
	sources  domainsource.Repository
	accounts domainaccount.Repository
	plugin   pluginsource.Plugin
	manager  *Manager
}

// NewService 创建监听源服务。
func NewService(
	sources domainsource.Repository,
	accounts domainaccount.Repository,
	plugin pluginsource.Plugin,
	manager *Manager,
) *Service {
	return &Service{sources: sources, accounts: accounts, plugin: plugin, manager: manager}
}

// List 返回全部监听源。
func (s *Service) List(ctx context.Context) ([]*domainsource.Source, error) {
	return s.sources.List(ctx)
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
func (s *Service) Sync(ctx context.Context, accountID int64) ([]pluginsource.SyncedPeer, error) {
	var peers []pluginsource.SyncedPeer
	if err := s.SyncStream(ctx, accountID, func(peer pluginsource.SyncedPeer) error {
		peers = append(peers, peer)
		return nil
	}); err != nil {
		return nil, err
	}
	return peers, nil
}

// SyncStream 流式拉取某账号可见的 chats/channels/users（用于 UI 增量展示）。
func (s *Service) SyncStream(ctx context.Context, accountID int64, emit pluginsource.SyncPeerHandler) error {
	acc, err := s.accounts.GetByID(ctx, accountID)
	if err != nil {
		return err
	}
	if acc.Status != domainaccount.StatusActive {
		return fmt.Errorf("账号未登录，无法同步（当前状态 %s）", acc.Status)
	}
	if streaming, ok := s.plugin.(interface {
		SyncSourcesStream(context.Context, *domainaccount.Account, pluginsource.SyncPeerHandler) error
	}); ok {
		return streaming.SyncSourcesStream(ctx, acc, emit)
	}
	peers, err := s.plugin.SyncSources(ctx, acc)
	if err != nil {
		return err
	}
	for _, peer := range peers {
		if err := emit(peer); err != nil {
			return err
		}
	}
	return nil
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
