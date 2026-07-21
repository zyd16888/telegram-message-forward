package mediastore

import (
	"context"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"
)

// Manager 持有当前生效的媒体存储，支持设置变更后运行时热切换，
// 无需重启服务。它同时实现 Store 与 /media 端点所需的签名校验能力。
type Manager struct {
	cur       atomic.Pointer[bundle]
	cleanupMu sync.Mutex
}

type bundle struct {
	store     Store
	local     *Local
	retention time.Duration
}

// NewManager 创建空的媒体存储管理器；使用前必须先 Swap 装入实现。
func NewManager() *Manager {
	return &Manager{}
}

var _ Store = (*Manager)(nil)

// Swap 原子替换当前生效的存储实现与保留期。
func (m *Manager) Swap(store Store, local *Local, retention time.Duration) {
	m.cur.Store(&bundle{store: store, local: local, retention: retention})
}

// Retention 返回当前生效的媒体保留时长。
func (m *Manager) Retention() time.Duration {
	if b := m.cur.Load(); b != nil {
		return b.retention
	}
	return 0
}

// PutFile 委托当前存储。
func (m *Manager) PutFile(ctx context.Context, key, srcPath string) (string, error) {
	b := m.cur.Load()
	if b == nil {
		return "", fmt.Errorf("媒体存储未初始化")
	}
	return b.store.PutFile(ctx, key, srcPath)
}

// PublicURL 委托当前存储。
func (m *Manager) PublicURL(ctx context.Context, key string) (string, error) {
	b := m.cur.Load()
	if b == nil {
		return "", nil
	}
	return b.store.PublicURL(ctx, key)
}

// Open 委托当前存储。
func (m *Manager) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	b := m.cur.Load()
	if b == nil {
		return nil, fmt.Errorf("媒体存储未初始化")
	}
	return b.store.Open(ctx, key)
}

// Cleanup 委托当前存储。
func (m *Manager) Cleanup(ctx context.Context, olderThan time.Duration) (int, error) {
	if !m.cleanupMu.TryLock() {
		return 0, ErrCleanupInProgress
	}
	defer m.cleanupMu.Unlock()
	b := m.cur.Load()
	if b == nil {
		return 0, nil
	}
	return b.store.Cleanup(ctx, olderThan)
}

// CleanupDetailed 对当前生效的存储执行一次手动清理。
func (m *Manager) CleanupDetailed(ctx context.Context, olderThan time.Duration, deleteLocal, deleteRemote bool) (CleanupResult, error) {
	if !m.cleanupMu.TryLock() {
		return CleanupResult{}, ErrCleanupInProgress
	}
	defer m.cleanupMu.Unlock()
	b := m.cur.Load()
	if b == nil {
		return CleanupResult{}, nil
	}
	cleaner, ok := b.store.(DetailedCleaner)
	if !ok {
		if deleteRemote {
			return CleanupResult{}, fmt.Errorf("当前媒体存储不支持远端清理")
		}
		removed, err := b.store.Cleanup(ctx, olderThan)
		return CleanupResult{DeletedLocalFiles: removed}, err
	}
	return cleaner.CleanupDetailed(ctx, olderThan, deleteLocal, deleteRemote)
}

// VerifySignedPath 委托当前本地存储的签名校验（/media 端点使用）。
func (m *Manager) VerifySignedPath(key string, expires int64, sig string) bool {
	b := m.cur.Load()
	if b == nil || b.local == nil {
		return false
	}
	return b.local.VerifySignedPath(key, expires, sig)
}
