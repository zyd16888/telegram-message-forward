// Package account 提供账号管理应用服务。
package account

import (
	"context"

	domainaccount "telegram-message-forward/internal/domain/account"
)

// Service 是账号应用服务。
type Service struct {
	repo domainaccount.Repository
}

// NewService 创建账号服务。
func NewService(repo domainaccount.Repository) *Service {
	return &Service{repo: repo}
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
	Name        string
	PhoneNumber string
	AppID       int
	AppHash     string
	Proxy       domainaccount.ProxyConfig
}

// Create 创建账号（初始状态 inactive，登录通过 cmd/login）。
func (s *Service) Create(ctx context.Context, in CreateInput) (*domainaccount.Account, error) {
	acc := &domainaccount.Account{
		Name:        in.Name,
		PhoneNumber: in.PhoneNumber,
		AppID:       in.AppID,
		AppHash:     in.AppHash,
		Proxy:       in.Proxy,
		Status:      domainaccount.StatusInactive,
	}
	if err := s.repo.Create(ctx, acc); err != nil {
		return nil, err
	}
	return acc, nil
}

// UpdateInput 是更新账号的输入。AppHash 为空时保留原值。
type UpdateInput struct {
	Name    *string
	AppID   *int
	AppHash *string
	Proxy   *domainaccount.ProxyConfig
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
	if in.AppID != nil {
		acc.AppID = *in.AppID
	}
	if in.AppHash != nil {
		acc.AppHash = *in.AppHash
	}
	if in.Proxy != nil {
		acc.Proxy = *in.Proxy
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
