package telegram

import (
	"context"
	"sync"

	"github.com/gotd/td/session"
)

// accountSession 是账号级 gotd session.Storage：内存持有明文 session，
// 变更时通过注入的 save 回调持久化（加密落库在存储层完成）。
type accountSession struct {
	accountID int64
	mu        sync.Mutex
	data      []byte
	load      func(ctx context.Context, accountID int64) ([]byte, error)
	save      func(ctx context.Context, accountID int64, session []byte) error
	loaded    bool
}

func newAccountSession(
	accountID int64,
	load func(ctx context.Context, accountID int64) ([]byte, error),
	save func(ctx context.Context, accountID int64, session []byte) error,
) *accountSession {
	return &accountSession{accountID: accountID, load: load, save: save}
}

var _ session.Storage = (*accountSession)(nil)

func (s *accountSession) set(data []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = data
	s.loaded = true
}

// LoadSession 返回明文 session；无 session 时返回 session.ErrNotFound。
func (s *accountSession) LoadSession(ctx context.Context) ([]byte, error) {
	s.mu.Lock()
	if !s.loaded && s.load != nil {
		s.mu.Unlock()
		data, err := s.load(ctx, s.accountID)
		if err != nil {
			return nil, err
		}
		s.set(data)
		s.mu.Lock()
	}
	defer s.mu.Unlock()
	if len(s.data) == 0 {
		return nil, session.ErrNotFound
	}
	out := make([]byte, len(s.data))
	copy(out, s.data)
	return out, nil
}

// StoreSession 更新内存 session 并持久化。
func (s *accountSession) StoreSession(ctx context.Context, data []byte) error {
	s.set(data)
	if s.save != nil {
		return s.save(ctx, s.accountID, data)
	}
	return nil
}
