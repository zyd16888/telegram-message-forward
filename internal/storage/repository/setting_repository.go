package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	domainsettings "telegram-message-forward/internal/domain/settings"
	"telegram-message-forward/internal/infra/crypto"
	"telegram-message-forward/internal/storage/model"
)

// SettingRepository 是 settings.Repository 的 PostgreSQL 实现。secret 加密落库。
type SettingRepository struct {
	db     *gorm.DB
	cipher *crypto.Cipher
}

// NewSettingRepository 创建系统设置仓储。
func NewSettingRepository(db *gorm.DB, cipher *crypto.Cipher) *SettingRepository {
	return &SettingRepository{db: db, cipher: cipher}
}

var _ domainsettings.Repository = (*SettingRepository)(nil)

// Get 按 key 查询设置；不存在时返回 (nil, nil)。
func (r *SettingRepository) Get(ctx context.Context, key string) (*domainsettings.Setting, error) {
	var m model.Setting
	if err := r.db.WithContext(ctx).First(&m, "key = ?", key).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return r.toDomain(&m)
}

// Upsert 插入或更新设置。
func (r *SettingRepository) Upsert(ctx context.Context, s *domainsettings.Setting) error {
	value, err := json.Marshal(s.Value)
	if err != nil {
		return fmt.Errorf("序列化设置失败: %w", err)
	}
	secretEnc, err := r.cipher.Encrypt(s.Secret)
	if err != nil {
		return fmt.Errorf("加密设置 secret 失败: %w", err)
	}
	m := &model.Setting{
		Key:             s.Key,
		Value:           value,
		SecretEncrypted: secretEnc,
		UpdatedAt:       time.Now(),
	}
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "key"}},
			DoUpdates: clause.AssignmentColumns([]string{"value", "secret_encrypted", "updated_at"}),
		}).
		Create(m).Error
}

func (r *SettingRepository) toDomain(m *model.Setting) (*domainsettings.Setting, error) {
	out := &domainsettings.Setting{Key: m.Key, UpdatedAt: m.UpdatedAt}
	if len(m.Value) > 0 {
		if err := json.Unmarshal(m.Value, &out.Value); err != nil {
			return nil, fmt.Errorf("解析设置失败: %w", err)
		}
	}
	if len(m.SecretEncrypted) > 0 {
		secret, err := r.cipher.Decrypt(m.SecretEncrypted)
		if err != nil {
			return nil, fmt.Errorf("解密设置 secret 失败: %w", err)
		}
		out.Secret = secret
	}
	return out, nil
}
