// Package sink 提供目标渠道管理应用服务。
package sink

import (
	"context"
	"fmt"

	domainsink "telegram-message-forward/internal/domain/sink"
	pluginsink "telegram-message-forward/internal/plugin/sink"
)

// Service 是渠道应用服务。
type Service struct {
	repo domainsink.Repository
}

// NewService 创建渠道服务。
func NewService(repo domainsink.Repository) *Service {
	return &Service{repo: repo}
}

// List 返回全部渠道。
func (s *Service) List(ctx context.Context) ([]*domainsink.Sink, error) {
	return s.repo.List(ctx)
}

// Get 查询单个渠道。
func (s *Service) Get(ctx context.Context, id int64) (*domainsink.Sink, error) {
	return s.repo.GetByID(ctx, id)
}

// CreateInput 是创建渠道的输入。Secret 为明文，存储层加密。
type CreateInput struct {
	Type    string
	Name    string
	Enabled bool
	Config  map[string]any
	Secret  string
}

// Create 创建渠道，校验类型已注册且配置合法，并按插件声明补齐能力。
func (s *Service) Create(ctx context.Context, in CreateInput) (*domainsink.Sink, error) {
	plugin, err := pluginsink.New(in.Type)
	if err != nil {
		return nil, err
	}
	if err := plugin.ValidateConfig(in.Config); err != nil {
		return nil, fmt.Errorf("渠道配置校验失败: %w", err)
	}
	sk := &domainsink.Sink{
		Type:         in.Type,
		Name:         in.Name,
		Enabled:      in.Enabled,
		Config:       in.Config,
		Secret:       []byte(in.Secret),
		Capabilities: plugin.Capabilities(),
	}
	if err := s.repo.Create(ctx, sk); err != nil {
		return nil, err
	}
	return sk, nil
}

// UpdateInput 是更新渠道的输入。Secret 为 nil 时保留原值。
type UpdateInput struct {
	Name    *string
	Enabled *bool
	Config  map[string]any
	Secret  *string
}

// Update 更新渠道可变字段。
func (s *Service) Update(ctx context.Context, id int64, in UpdateInput) (*domainsink.Sink, error) {
	sk, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if in.Name != nil {
		sk.Name = *in.Name
	}
	if in.Enabled != nil {
		sk.Enabled = *in.Enabled
	}
	if in.Config != nil {
		plugin, err := pluginsink.New(sk.Type)
		if err != nil {
			return nil, err
		}
		if err := plugin.ValidateConfig(in.Config); err != nil {
			return nil, fmt.Errorf("渠道配置校验失败: %w", err)
		}
		sk.Config = in.Config
	}
	if in.Secret != nil {
		sk.Secret = []byte(*in.Secret)
	}
	if err := s.repo.Update(ctx, sk); err != nil {
		return nil, err
	}
	return sk, nil
}

// Delete 删除渠道。
func (s *Service) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}

// Types 返回已注册的 Sink 类型。
func (s *Service) Types() []string {
	return pluginsink.Names()
}

// Descriptors 返回已注册 Sink 的后台配置元数据。
func (s *Service) Descriptors() []pluginsink.Descriptor {
	return pluginsink.Descriptors()
}
