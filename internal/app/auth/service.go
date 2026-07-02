// Package auth 提供管理后台登录相关的应用服务。
//
// 首版采用单管理员模型，复用 API token 哈希校验能力：
//   - bootstrap 仅在没有任何 active 管理 token 时开放，用于 UI 首次初始化管理员凭证。
//   - login 只校验 token 是否有效，不返回明文、不创建新的长期 secret。
//   - me 用于前端判断当前凭证是否仍有效，或识别开发免鉴权模式。
//
// 当 auth_enabled=false 时，管理 API 不再强制 Bearer Token，登录流程退化为
// “开发免鉴权”状态；此时 bootstrap/login 仍可用，但不是进入后台的必要条件。
package auth

import (
	"context"

	apptoken "telegram-message-forward/internal/app/apitoken"
	domainapitoken "telegram-message-forward/internal/domain/apitoken"
	"telegram-message-forward/internal/security"
)

// Service 是管理后台登录应用服务。
type Service struct {
	repo        domainapitoken.Repository
	tokens      *apptoken.Service
	authEnabled bool
}

// NewService 创建登录服务。authEnabled 表示 /api/v1 是否启用 Bearer Token 鉴权。
func NewService(repo domainapitoken.Repository, tokens *apptoken.Service, authEnabled bool) *Service {
	return &Service{repo: repo, tokens: tokens, authEnabled: authEnabled}
}

// BootstrapStatus 描述首次初始化管理凭证的可用性。
type BootstrapStatus struct {
	// AuthEnabled 表示当前是否启用鉴权。
	AuthEnabled bool
	// CanBootstrap 表示当前没有任何 active token，可创建首个管理凭证。
	CanBootstrap bool
}

// BootstrapStatus 返回 bootstrap 可用性。
func (s *Service) BootstrapStatus(ctx context.Context) (*BootstrapStatus, error) {
	can, err := s.canBootstrap(ctx)
	if err != nil {
		return nil, err
	}
	return &BootstrapStatus{AuthEnabled: s.authEnabled, CanBootstrap: can}, nil
}

// canBootstrap 判断是否没有任何 active token。
func (s *Service) canBootstrap(ctx context.Context) (bool, error) {
	count, err := s.repo.CountActive(ctx)
	if err != nil {
		return false, err
	}
	return count == 0, nil
}

// ErrBootstrapClosed 表示已存在 active token，bootstrap 入口已关闭。
type ErrBootstrapClosed struct{}

func (ErrBootstrapClosed) Error() string { return "已存在管理凭证，初始化入口已关闭" }

// Bootstrap 在没有任何 active token 时创建首个管理凭证，返回明文（仅此一次）。
//
// 检查与创建在存储层同一事务内通过 advisory lock 原子完成，防止并发 bootstrap
// 请求都看到“无 active token”而各自创建出多个首个管理凭证。
func (s *Service) Bootstrap(ctx context.Context, name string) (*apptoken.Created, error) {
	if name == "" {
		name = "admin"
	}
	created, ok, err := s.tokens.CreateIfNoneActive(ctx, name)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrBootstrapClosed{}
	}
	return created, nil
}

// Login 校验管理 token 是否有效（存在且未吊销）。
func (s *Service) Login(ctx context.Context, token string) (bool, error) {
	if token == "" {
		return false, nil
	}
	return s.repo.ExistsActiveHash(ctx, security.HashToken(token))
}

// Identity 是当前登录身份状态。
type Identity struct {
	// AuthEnabled 表示是否启用鉴权。
	AuthEnabled bool
	// Authenticated 表示当前凭证是否有效；免鉴权模式下恒为 true。
	Authenticated bool
	// CanBootstrap 表示是否可创建首个管理凭证。
	CanBootstrap bool
}

// Me 根据鉴权模式与传入 token 返回当前身份状态。
func (s *Service) Me(ctx context.Context, token string) (*Identity, error) {
	can, err := s.canBootstrap(ctx)
	if err != nil {
		return nil, err
	}
	if !s.authEnabled {
		// 开发免鉴权模式：直接视为已登录。
		return &Identity{AuthEnabled: false, Authenticated: true, CanBootstrap: can}, nil
	}
	authed, err := s.Login(ctx, token)
	if err != nil {
		return nil, err
	}
	return &Identity{AuthEnabled: true, Authenticated: authed, CanBootstrap: can}, nil
}
