package storage

import (
	"context"

	"gorm.io/gorm"
)

// TxManager 提供事务边界，供应用层编排跨仓储的原子操作。
type TxManager struct {
	db *gorm.DB
}

// NewTxManager 创建事务管理器。
func NewTxManager(db *gorm.DB) *TxManager {
	return &TxManager{db: db}
}

// WithinTx 在一个事务内执行 fn。
func (m *TxManager) WithinTx(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return m.db.WithContext(ctx).Transaction(fn)
}
