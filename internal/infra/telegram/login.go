package telegram

import (
	"context"
	"sync"

	"github.com/gotd/td/session"
	"github.com/gotd/td/tg"

	domainaccount "telegram-message-forward/internal/domain/account"
)

// memorySession 是内存明文 session.Storage，变更时通过 save 回调持久化。
type memorySession struct {
	mu   sync.Mutex
	data []byte
	save func(ctx context.Context, plaintext []byte) error
}

var _ session.Storage = (*memorySession)(nil)

func (s *memorySession) LoadSession(context.Context) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.data) == 0 {
		return nil, session.ErrNotFound
	}
	out := make([]byte, len(s.data))
	copy(out, s.data)
	return out, nil
}

func (s *memorySession) StoreSession(ctx context.Context, data []byte) error {
	s.mu.Lock()
	s.data = data
	s.mu.Unlock()
	if s.save != nil {
		return s.save(ctx, data)
	}
	return nil
}

// LoginRequest 是一次 CLI 登录请求。
//
// Code / Password 通常由命令行交互读取；Password 仅在开启两步验证时被调用。
// SaveSession 在 session 生成或刷新时被调用，负责把明文 session 加密落库。
type LoginRequest struct {
	Account     *domainaccount.Account
	Code        func(ctx context.Context) (string, error)
	Password    func(ctx context.Context) (string, error)
	SaveSession func(ctx context.Context, plaintext []byte) error
}

// RunLogin 执行完整登录流程（SendCode / SignIn / 2FA），成功后 session 已持久化。
//
// 该函数把 gotd/td 完全封装在 infra 层，cmd/login 无需直接依赖 gotd。
func RunLogin(ctx context.Context, req LoginRequest) error {
	store := &memorySession{data: req.Account.Session, save: req.SaveSession}

	client, err := NewClient(ClientConfig{
		AppID:        req.Account.AppID,
		AppHash:      req.Account.AppHash,
		Proxy:        req.Account.Proxy,
		SessionStore: store,
	})
	if err != nil {
		return err
	}

	return client.Run(ctx, func(ctx context.Context) error {
		authr := Authenticator{
			PhoneFunc: func(context.Context) (string, error) {
				return req.Account.PhoneNumber, nil
			},
			CodeFunc: func(ctx context.Context, _ *tg.AuthSentCode) (string, error) {
				return req.Code(ctx)
			},
			PasswordFunc: req.Password,
		}
		return Login(ctx, client, authr)
	})
}
