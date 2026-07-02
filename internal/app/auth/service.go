// Package auth 提供管理后台登录相关的应用服务。
package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	apptoken "telegram-message-forward/internal/app/apitoken"
	domainadmin "telegram-message-forward/internal/domain/admin"
	domainapitoken "telegram-message-forward/internal/domain/apitoken"
	"telegram-message-forward/internal/security"

	"golang.org/x/crypto/bcrypt"
)

// Service 是管理后台登录应用服务。
type Service struct {
	admins      domainadmin.Repository
	apiTokens   domainapitoken.Repository
	tokens      *apptoken.Service
	authEnabled bool
	sessionTTL  time.Duration
}

// NewService 创建登录服务。authEnabled 表示 /api/v1 是否启用 Bearer Token 鉴权。
func NewService(admins domainadmin.Repository, apiTokens domainapitoken.Repository, tokens *apptoken.Service, authEnabled bool) *Service {
	return &Service{
		admins:      admins,
		apiTokens:   apiTokens,
		tokens:      tokens,
		authEnabled: authEnabled,
		sessionTTL:  30 * 24 * time.Hour,
	}
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
	count, err := s.admins.CountActiveUsers(ctx)
	if err != nil {
		return false, err
	}
	return count == 0, nil
}

// ErrBootstrapClosed 表示已存在 active token，bootstrap 入口已关闭。
type ErrBootstrapClosed struct{}

func (ErrBootstrapClosed) Error() string { return "已存在管理凭证，初始化入口已关闭" }

// ErrInvalidCredential 表示用户名或密码错误。
var ErrInvalidCredential = errors.New("用户名或密码错误")

// LoginResult 是登录/初始化成功后的浏览器会话结果。
type LoginResult struct {
	Authenticated bool
	Token         string
	Username      string
	ExpiresAt     time.Time
}

// Bootstrap 在没有任何 active 管理员时创建首个管理员，并签发浏览器会话。
func (s *Service) Bootstrap(ctx context.Context, username, password string) (*LoginResult, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		username = "admin"
	}
	if strings.TrimSpace(password) == "" {
		return nil, errors.New("密码不能为空")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	user := &domainadmin.User{Username: username, PasswordHash: string(hash), Active: true}
	ok, err := s.admins.CreateUserIfNoneActive(ctx, user)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrBootstrapClosed{}
	}
	return s.createSession(ctx, user)
}

// Login 校验用户名密码并签发浏览器会话。
func (s *Service) Login(ctx context.Context, username, password string) (*LoginResult, error) {
	user, err := s.admins.GetUserByUsername(ctx, strings.TrimSpace(username))
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrInvalidCredential
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidCredential
	}
	return s.createSession(ctx, user)
}

func (s *Service) createSession(ctx context.Context, user *domainadmin.User) (*LoginResult, error) {
	token, err := security.GenerateToken()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	expires := now.Add(s.sessionTTL)
	sess := &domainadmin.Session{
		UserID:    user.ID,
		TokenHash: security.HashToken(token),
		ExpiresAt: expires,
	}
	if err := s.admins.CreateSession(ctx, sess); err != nil {
		return nil, err
	}
	if err := s.admins.MarkUserLogin(ctx, user.ID, now); err != nil {
		return nil, err
	}
	return &LoginResult{Authenticated: true, Token: token, Username: user.Username, ExpiresAt: expires}, nil
}

// ValidateToken 校验浏览器会话 token 或运维 API token 是否有效。
func (s *Service) ValidateToken(ctx context.Context, token string) (*Identity, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return &Identity{AuthEnabled: s.authEnabled}, nil
	}
	hash := security.HashToken(token)
	now := time.Now()
	sess, user, err := s.admins.GetActiveSessionByHash(ctx, hash, now)
	if err != nil {
		return nil, err
	}
	if sess != nil && user != nil {
		_ = s.admins.TouchSession(ctx, sess.ID, now)
		return &Identity{AuthEnabled: s.authEnabled, Authenticated: true, Username: user.Username, CredentialType: "session"}, nil
	}
	ok, err := s.apiTokens.ExistsActiveHash(ctx, hash)
	if err != nil {
		return nil, err
	}
	if ok {
		return &Identity{AuthEnabled: s.authEnabled, Authenticated: true, CredentialType: "api_token"}, nil
	}
	return &Identity{AuthEnabled: s.authEnabled}, nil
}

// Logout 吊销浏览器会话；API token 不受退出操作影响。
func (s *Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return s.admins.RevokeSessionByHash(ctx, security.HashToken(token))
}

// Identity 是当前登录身份状态。
type Identity struct {
	// AuthEnabled 表示是否启用鉴权。
	AuthEnabled bool
	// Authenticated 表示当前凭证是否有效；免鉴权模式下恒为 true。
	Authenticated bool
	// CanBootstrap 表示是否可创建首个管理凭证。
	CanBootstrap bool
	// Username 是浏览器会话对应的管理员用户名；API token 为空。
	Username string
	// CredentialType 为 session / api_token / dev。
	CredentialType string
}

// Me 根据鉴权模式与传入 token 返回当前身份状态。
func (s *Service) Me(ctx context.Context, token string) (*Identity, error) {
	can, err := s.canBootstrap(ctx)
	if err != nil {
		return nil, err
	}
	if !s.authEnabled {
		// 开发免鉴权模式：直接视为已登录。
		return &Identity{AuthEnabled: false, Authenticated: true, CanBootstrap: can, CredentialType: "dev"}, nil
	}
	id, err := s.ValidateToken(ctx, token)
	if err != nil {
		return nil, err
	}
	id.CanBootstrap = can
	return id, nil
}

// TokenService 返回 API token 服务，供 Settings 继续管理运维 token。
func (s *Service) TokenService() *apptoken.Service {
	return s.tokens
}
