// Package telegramconfig 提供 Telegram App 与代理配置应用服务。
package telegramconfig

import (
	"context"

	domainconfig "telegram-message-forward/internal/domain/telegramconfig"
)

// Service 是 Telegram 共享配置应用服务。
type Service struct {
	apps    domainconfig.TelegramAppRepository
	proxies domainconfig.ProxyRepository
}

// NewService 创建服务。
func NewService(apps domainconfig.TelegramAppRepository, proxies domainconfig.ProxyRepository) *Service {
	return &Service{apps: apps, proxies: proxies}
}

// AppInput 是 Telegram App 写入输入。
type AppInput struct {
	Name    string
	AppID   int
	AppHash *string
	Enabled bool
}

func (s *Service) ListApps(ctx context.Context) ([]*domainconfig.TelegramApp, error) {
	return s.apps.List(ctx)
}

func (s *Service) CreateApp(ctx context.Context, in AppInput) (*domainconfig.TelegramApp, error) {
	hash := ""
	if in.AppHash != nil {
		hash = *in.AppHash
	}
	app := &domainconfig.TelegramApp{Name: in.Name, AppID: in.AppID, AppHash: hash, Enabled: in.Enabled}
	if err := s.apps.Create(ctx, app); err != nil {
		return nil, err
	}
	return app, nil
}

func (s *Service) UpdateApp(ctx context.Context, id int64, in AppInput) (*domainconfig.TelegramApp, error) {
	app, err := s.apps.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	app.Name = in.Name
	app.AppID = in.AppID
	app.Enabled = in.Enabled
	if in.AppHash != nil {
		app.AppHash = *in.AppHash
	}
	if err := s.apps.Update(ctx, app); err != nil {
		return nil, err
	}
	return app, nil
}

func (s *Service) DeleteApp(ctx context.Context, id int64) error {
	return s.apps.Delete(ctx, id)
}

// ProxyInput 是代理配置写入输入。
type ProxyInput struct {
	Name     string
	Type     string
	Addr     string
	Username string
	Password *string
	Enabled  bool
}

func (s *Service) ListProxies(ctx context.Context) ([]*domainconfig.Proxy, error) {
	return s.proxies.List(ctx)
}

func (s *Service) CreateProxy(ctx context.Context, in ProxyInput) (*domainconfig.Proxy, error) {
	proxyType, err := domainconfig.NormalizeProxyType(in.Type)
	if err != nil {
		return nil, err
	}
	pass := ""
	if in.Password != nil {
		pass = *in.Password
	}
	p := &domainconfig.Proxy{
		Name:     in.Name,
		Type:     proxyType,
		Addr:     in.Addr,
		Username: in.Username,
		Password: pass,
		Enabled:  in.Enabled,
	}
	if err := s.proxies.Create(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *Service) UpdateProxy(ctx context.Context, id int64, in ProxyInput) (*domainconfig.Proxy, error) {
	proxyType, err := domainconfig.NormalizeProxyType(in.Type)
	if err != nil {
		return nil, err
	}
	p, err := s.proxies.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	p.Name = in.Name
	p.Type = proxyType
	p.Addr = in.Addr
	p.Username = in.Username
	p.Enabled = in.Enabled
	if in.Password != nil {
		p.Password = *in.Password
	}
	if err := s.proxies.Update(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *Service) DeleteProxy(ctx context.Context, id int64) error {
	return s.proxies.Delete(ctx, id)
}
