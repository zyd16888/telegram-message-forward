// Package repository 实现 domain 仓储接口。
//
// repository 对外返回 domain 对象，不返回 GORM model；敏感字段在此加解密。
package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"gorm.io/gorm"

	domainaccount "github.com/zyd16888/telegram-message-forward/internal/domain/account"
	"github.com/zyd16888/telegram-message-forward/internal/infra/crypto"
	"github.com/zyd16888/telegram-message-forward/internal/storage/model"
)

// AccountRepository 是 account.Repository 的 PostgreSQL 实现。
type AccountRepository struct {
	db     *gorm.DB
	cipher *crypto.Cipher
}

// NewAccountRepository 创建账号仓储。
func NewAccountRepository(db *gorm.DB, cipher *crypto.Cipher) *AccountRepository {
	return &AccountRepository{db: db, cipher: cipher}
}

var _ domainaccount.Repository = (*AccountRepository)(nil)

// Create 插入账号，敏感字段加密。
func (r *AccountRepository) Create(ctx context.Context, a *domainaccount.Account) error {
	m, err := r.toModel(a)
	if err != nil {
		return err
	}
	if err := r.db.WithContext(ctx).Create(m).Error; err != nil {
		return err
	}
	a.ID = m.ID
	return nil
}

// Update 更新账号。
func (r *AccountRepository) Update(ctx context.Context, a *domainaccount.Account) error {
	m, err := r.toModel(a)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Save(m).Error
}

// GetByID 按 id 查询账号。
func (r *AccountRepository) GetByID(ctx context.Context, id int64) (*domainaccount.Account, error) {
	var m model.Account
	if err := r.db.WithContext(ctx).First(&m, id).Error; err != nil {
		return nil, err
	}
	return r.toDomain(&m)
}

// List 返回全部账号。
func (r *AccountRepository) List(ctx context.Context) ([]*domainaccount.Account, error) {
	var ms []model.Account
	if err := r.db.WithContext(ctx).Order("id").Find(&ms).Error; err != nil {
		return nil, err
	}
	out := make([]*domainaccount.Account, 0, len(ms))
	for i := range ms {
		a, err := r.toDomain(&ms[i])
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, nil
}

// Delete 删除账号。
func (r *AccountRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.Account{}, id).Error
}

func (r *AccountRepository) toModel(a *domainaccount.Account) (*model.Account, error) {
	appHashEnc, err := r.cipher.Encrypt([]byte(a.AppHash))
	if err != nil {
		return nil, fmt.Errorf("加密 app_hash 失败: %w", err)
	}
	sessionEnc, err := r.cipher.Encrypt(a.Session)
	if err != nil {
		return nil, fmt.Errorf("加密 session 失败: %w", err)
	}
	proxyJSON, err := json.Marshal(a.Proxy)
	if err != nil {
		return nil, fmt.Errorf("序列化代理配置失败: %w", err)
	}
	return &model.Account{
		ID:               a.ID,
		Name:             a.Name,
		PhoneNumber:      a.PhoneNumber,
		AppID:            a.AppID,
		AppHashEncrypted: appHashEnc,
		SessionEncrypted: sessionEnc,
		ProxyConfig:      proxyJSON,
		Status:           string(a.Status),
		LastLoginAt:      a.LastLoginAt,
		LastError:        a.LastError,
		CreatedAt:        a.CreatedAt,
		UpdatedAt:        a.UpdatedAt,
	}, nil
}

func (r *AccountRepository) toDomain(m *model.Account) (*domainaccount.Account, error) {
	appHash, err := r.cipher.Decrypt(m.AppHashEncrypted)
	if err != nil {
		return nil, fmt.Errorf("解密 app_hash 失败: %w", err)
	}
	session, err := r.cipher.Decrypt(m.SessionEncrypted)
	if err != nil {
		return nil, fmt.Errorf("解密 session 失败: %w", err)
	}
	var proxy domainaccount.ProxyConfig
	if len(m.ProxyConfig) > 0 {
		if err := json.Unmarshal(m.ProxyConfig, &proxy); err != nil {
			return nil, fmt.Errorf("解析代理配置失败: %w", err)
		}
	}
	return &domainaccount.Account{
		ID:          m.ID,
		Name:        m.Name,
		PhoneNumber: m.PhoneNumber,
		AppID:       m.AppID,
		AppHash:     string(appHash),
		Session:     session,
		Proxy:       proxy,
		Status:      domainaccount.Status(m.Status),
		LastLoginAt: m.LastLoginAt,
		LastError:   m.LastError,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}, nil
}
