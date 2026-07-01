// Package telegram 封装 gotd/td 客户端：session 加密存储、代理、限流与登录。
//
// gotd/td 类型只允许出现在本包与 internal/plugin/source/telegram，
// 不得扩散到 domain / ruleengine / dispatch / sink。
package telegram

import (
	"context"

	"github.com/gotd/td/session"

	"telegram-message-forward/internal/infra/crypto"
)

// LoadFunc 返回账号当前的加密 session blob（无 session 时返回 nil/空）。
type LoadFunc func(ctx context.Context) ([]byte, error)

// StoreFunc 持久化加密后的 session blob。
type StoreFunc func(ctx context.Context, encrypted []byte) error

// SessionStore 实现 gotd session.Storage，对 session blob 加解密后落库。
//
// gotd 在内存中拿到的是明文 session，落库的是密文，明文不进磁盘。
type SessionStore struct {
	cipher *crypto.Cipher
	load   LoadFunc
	store  StoreFunc
}

// NewSessionStore 创建加密 session 存储。
func NewSessionStore(cipher *crypto.Cipher, load LoadFunc, store StoreFunc) *SessionStore {
	return &SessionStore{cipher: cipher, load: load, store: store}
}

var _ session.Storage = (*SessionStore)(nil)

// LoadSession 读取并解密 session；无 session 时返回 session.ErrNotFound。
func (s *SessionStore) LoadSession(ctx context.Context) ([]byte, error) {
	enc, err := s.load(ctx)
	if err != nil {
		return nil, err
	}
	if len(enc) == 0 {
		return nil, session.ErrNotFound
	}
	plain, err := s.cipher.Decrypt(enc)
	if err != nil {
		return nil, err
	}
	if len(plain) == 0 {
		return nil, session.ErrNotFound
	}
	return plain, nil
}

// StoreSession 加密并持久化 session。
func (s *SessionStore) StoreSession(ctx context.Context, data []byte) error {
	enc, err := s.cipher.Encrypt(data)
	if err != nil {
		return err
	}
	return s.store(ctx, enc)
}
