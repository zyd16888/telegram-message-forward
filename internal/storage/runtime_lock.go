package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

const (
	runtimeLockNamespace = 0x544d46 // "TMF"
	runtimeLockKey       = 1
)

// RuntimeLock 使用 PostgreSQL session advisory lock 保证同一数据库只运行一个服务实例。
type RuntimeLock struct {
	conn *sql.Conn
}

// TryAcquireRuntimeLock 尝试获取运行时单实例锁。锁由独占数据库连接持有，直到 Release。
func TryAcquireRuntimeLock(ctx context.Context, db *sql.DB) (*RuntimeLock, bool, error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("获取运行时锁连接失败: %w", err)
	}

	var acquired bool
	if err := conn.QueryRowContext(ctx,
		"SELECT pg_try_advisory_lock($1, $2)", runtimeLockNamespace, runtimeLockKey,
	).Scan(&acquired); err != nil {
		_ = conn.Close()
		return nil, false, fmt.Errorf("获取运行时单实例锁失败: %w", err)
	}
	if !acquired {
		_ = conn.Close()
		return nil, false, nil
	}
	return &RuntimeLock{conn: conn}, true, nil
}

// Release 释放运行时单实例锁及其独占连接。
func (l *RuntimeLock) Release() error {
	if l == nil || l.conn == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, unlockErr := l.conn.ExecContext(ctx,
		"SELECT pg_advisory_unlock($1, $2)", runtimeLockNamespace, runtimeLockKey,
	)
	closeErr := l.conn.Close()
	l.conn = nil
	if unlockErr != nil {
		return fmt.Errorf("释放运行时单实例锁失败: %w", unlockErr)
	}
	return closeErr
}
