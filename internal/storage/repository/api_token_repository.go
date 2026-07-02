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

// CountActive 返回未吊销的 token 数量。
func (r *APITokenRepository) CountActive(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.APIToken{}).
		Where("revoked_at IS NULL").
		Count(&count).Error
	return count, err
}

// bootstrapAdvisoryLockKey 是 bootstrap 初始化专用的 PostgreSQL advisory lock key，
// 用于把并发 bootstrap 请求串行化，避免都看到 count=0 而各自创建出多个管理凭证。
// 取值任意，只需在本项目内唯一即可；事务结束自动释放锁。
const bootstrapAdvisoryLockKey = 8823001

// CreateIfNoneActive 在同一事务内加 advisory lock 后检查并创建，保证并发调用下
// 最多只有一次创建成功：先到的请求持锁完成检查+创建并提交，后到的请求阻塞到锁释放后
// 会看到已存在 active token，从而 created=false 不重复创建。
func (r *APITokenRepository) CreateIfNoneActive(ctx context.Context, name, hash string) (int64, bool, error) {
	var id int64
	created := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(?)", bootstrapAdvisoryLockKey).Error; err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&model.APIToken{}).Where("revoked_at IS NULL").Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return nil
		}
		m := &model.APIToken{Name: name, TokenHash: hash}
		if err := tx.Create(m).Error; err != nil {
			return err
		}
		id = m.ID
		created = true
		return nil
	})
	if err != nil {
		return 0, false, err
	}
	return id, created, nil
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
