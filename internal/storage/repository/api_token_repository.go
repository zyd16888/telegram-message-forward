package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	domainapitoken "telegram-message-forward/internal/domain/apitoken"
	"telegram-message-forward/internal/storage/model"
)

// APITokenRepository 提供管理 API token 的存储查询。
type APITokenRepository struct {
	db *gorm.DB
}

// NewAPITokenRepository 创建 API token 仓储。
func NewAPITokenRepository(db *gorm.DB) *APITokenRepository {
	return &APITokenRepository{db: db}
}

var _ domainapitoken.Repository = (*APITokenRepository)(nil)

// Create 写入一条 token 哈希记录，返回新记录 id。
func (r *APITokenRepository) Create(ctx context.Context, name, hash string) (int64, error) {
	m := &model.APIToken{Name: name, TokenHash: hash}
	if err := r.db.WithContext(ctx).Create(m).Error; err != nil {
		return 0, err
	}
	return m.ID, nil
}

// List 返回全部 token（不含明文）。
func (r *APITokenRepository) List(ctx context.Context) ([]*domainapitoken.Token, error) {
	var ms []model.APIToken
	if err := r.db.WithContext(ctx).Order("id").Find(&ms).Error; err != nil {
		return nil, err
	}
	out := make([]*domainapitoken.Token, 0, len(ms))
	for i := range ms {
		out = append(out, toAPITokenDomain(&ms[i]))
	}
	return out, nil
}

// Revoke 吊销指定 token。
func (r *APITokenRepository) Revoke(ctx context.Context, id int64) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&model.APIToken{}).
		Where("id = ? AND revoked_at IS NULL", id).
		Update("revoked_at", now).Error
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

func toAPITokenDomain(m *model.APIToken) *domainapitoken.Token {
	return &domainapitoken.Token{
		ID:         m.ID,
		Name:       m.Name,
		TokenHash:  m.TokenHash,
		CreatedAt:  m.CreatedAt,
		LastUsedAt: m.LastUsedAt,
		RevokedAt:  m.RevokedAt,
	}
}
