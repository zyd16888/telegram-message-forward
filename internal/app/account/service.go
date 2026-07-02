// Package account 提供账号管理应用服务。
package account

import (
	"context"
	"errors"

	domainaccount "telegram-message-forward/internal/domain/account"
	domainconfig "telegram-message-forward/internal/domain/telegramconfig"
)

// Service 是账号应用服务。
type Service struct {
	repo    domainaccount.Repository
	apps    domainconfig.TelegramAppRepository
	proxies domainconfig.ProxyRepository
}

// NewService 创建账号服务。
func NewService(repo domainaccount.Repository, apps domainconfig.TelegramAppRepository, proxies domainconfig.ProxyRepository) *Service {
	return &Service{repo: repo, apps: apps, proxies: proxies}
}

// List 返回全部账号。
func (s *Service) List(ctx context.Context) ([]*domainaccount.Account, error) {
	return s.repo.List(ctx)
}

// Get 查询单个账号。
func (s *Service) Get(ctx context.Context, id int64) (*domainaccount.Account, error) {
	return s.repo.GetByID(ctx, id)
}

// CreateInput 是创建账号的输入。
type CreateInput struct {
	Name          string
	PhoneNumber   string
	TelegramAppID int64
	ProxyID       *int64
}

// Create 创建账号（初始状态 inactive，登录通过 cmd/login）。
func (s *Service) Create(ctx context.Context, in CreateInput) (*domainaccount.Account, error) {
	if in.TelegramAppID <= 0 {
		return nil, errors.New("请选择 Telegram App")
	}
	app, err := s.apps.GetByID(ctx, in.TelegramAppID)
	if err != nil {
		return nil, err
	}
	if !app.Enabled {
		return nil, errors.New("Telegram App 已禁用")
	}
	var proxy domainaccount.ProxyConfig
	if in.ProxyID != nil {
		p, err := s.proxies.GetByID(ctx, *in.ProxyID)
		if err != nil {
			return nil, err
		}
		if !p.Enabled {
			return nil, errors.New("代理配置已禁用")
		}
		proxy = p.ToAccountProxy()
	}
	acc := &domainaccount.Account{
		Name:          in.Name,
		PhoneNumber:   in.PhoneNumber,
		TelegramAppID: &app.ID,
		ProxyID:       in.ProxyID,
		AppID:         app.AppID,
		AppHash:       app.AppHash,
		Proxy:         proxy,
		Status:        domainaccount.StatusInactive,
	}
	if err := s.repo.Create(ctx, acc); err != nil {
		return nil, err
	}
	return acc, nil
}

// UpdateInput 是更新账号的输入。AppHash 为空时保留原值。
type UpdateInput struct {
	Name          *string
	TelegramAppID *int64
	ProxyID       *int64
	ClearProxy    bool
}

// Update 更新账号可变字段。
func (s *Service) Update(ctx context.Context, id int64, in UpdateInput) (*domainaccount.Account, error) {
	acc, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if in.Name != nil {
		acc.Name = *in.Name
	}
	if in.TelegramAppID != nil {
		app, err := s.apps.GetByID(ctx, *in.TelegramAppID)
		if err != nil {
			return nil, err
		}
		if !app.Enabled {
			return nil, errors.New("Telegram App 已禁用")
		}
		acc.TelegramAppID = &app.ID
		acc.AppID = app.AppID
		acc.AppHash = app.AppHash
	}
	if in.ClearProxy {
		acc.ProxyID = nil
		acc.Proxy = domainaccount.ProxyConfig{}
	} else if in.ProxyID != nil {
		p, err := s.proxies.GetByID(ctx, *in.ProxyID)
		if err != nil {
			return nil, err
		}
		if !p.Enabled {
			return nil, errors.New("代理配置已禁用")
		}
		acc.ProxyID = &p.ID
		acc.Proxy = p.ToAccountProxy()
	}
	if err := s.repo.Update(ctx, acc); err != nil {
		return nil, err
	}
	return acc, nil
}

// Delete 删除账号。
func (s *Service) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}
