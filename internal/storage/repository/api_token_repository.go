package repository

import (
	"context"

	"gorm.io/gorm"

	"github.com/zyd16888/telegram-message-forward/internal/storage/model"
)

// APITokenRepository 提供管理 API token 的存储查询。
type APITokenRepository struct {
	db *gorm.DB
}

// NewAPITokenRepository 创建 API token 仓储。
func NewAPITokenRepository(db *gorm.DB) *APITokenRepository {
	return &APITokenRepository{db: db}
}

// ExistsActiveHash 判断 token hash 是否存在且未吊销。
func (r *APITokenRepository) ExistsActiveHash(ctx context.Context, hash string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.APIToken{}).
		Where("token_hash = ? AND revoked_at IS NULL", hash).
		Count(&count).Error
	return count > 0, err
}
