package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	domainadmin "telegram-message-forward/internal/domain/admin"
	"telegram-message-forward/internal/storage/model"
)

// AdminRepository 是管理后台用户与会话的 PostgreSQL 实现。
type AdminRepository struct {
	db *gorm.DB
}

// NewAdminRepository 创建管理后台认证仓储。
func NewAdminRepository(db *gorm.DB) *AdminRepository {
	return &AdminRepository{db: db}
}

var _ domainadmin.Repository = (*AdminRepository)(nil)

// CountActiveUsers 返回 active 管理员数量。
func (r *AdminRepository) CountActiveUsers(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.AdminUser{}).Where("active = ?", true).Count(&count).Error
	return count, err
}

// CreateUserIfNoneActive 在无 active 管理员时创建首个用户。
func (r *AdminRepository) CreateUserIfNoneActive(ctx context.Context, u *domainadmin.User) (bool, error) {
	created := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(?)", bootstrapAdvisoryLockKey).Error; err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&model.AdminUser{}).Where("active = ?", true).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return nil
		}
		m := &model.AdminUser{Username: u.Username, PasswordHash: u.PasswordHash, Active: true}
		if err := tx.Create(m).Error; err != nil {
			return err
		}
		u.ID = m.ID
		u.CreatedAt = m.CreatedAt
		u.UpdatedAt = m.UpdatedAt
		created = true
		return nil
	})
	return created, err
}

// GetUserByUsername 按用户名查询 active 用户；不存在返回 (nil, nil)。
func (r *AdminRepository) GetUserByUsername(ctx context.Context, username string) (*domainadmin.User, error) {
	var m model.AdminUser
	err := r.db.WithContext(ctx).Where("username = ? AND active = ?", username, true).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return toAdminUserDomain(&m), nil
}

// MarkUserLogin 更新最近登录时间。
func (r *AdminRepository) MarkUserLogin(ctx context.Context, userID int64, at time.Time) error {
	return r.db.WithContext(ctx).Model(&model.AdminUser{}).Where("id = ?", userID).Update("last_login_at", at).Error
}

// CreateSession 创建会话。
func (r *AdminRepository) CreateSession(ctx context.Context, s *domainadmin.Session) error {
	m := &model.AdminSession{
		UserID:    s.UserID,
		TokenHash: s.TokenHash,
		ExpiresAt: s.ExpiresAt,
	}
	if err := r.db.WithContext(ctx).Create(m).Error; err != nil {
		return err
	}
	s.ID = m.ID
	s.CreatedAt = m.CreatedAt
	return nil
}

// GetActiveSessionByHash 查询未吊销且未过期的会话及用户。
func (r *AdminRepository) GetActiveSessionByHash(ctx context.Context, hash string, now time.Time) (*domainadmin.Session, *domainadmin.User, error) {
	var s model.AdminSession
	err := r.db.WithContext(ctx).
		Where("token_hash = ? AND revoked_at IS NULL AND expires_at > ?", hash, now).
		First(&s).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	var u model.AdminUser
	if err := r.db.WithContext(ctx).Where("id = ? AND active = ?", s.UserID, true).First(&u).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	return toAdminSessionDomain(&s), toAdminUserDomain(&u), nil
}

// TouchSession 更新会话最近使用时间。
func (r *AdminRepository) TouchSession(ctx context.Context, id int64, at time.Time) error {
	return r.db.WithContext(ctx).Model(&model.AdminSession{}).Where("id = ?", id).Update("last_used_at", at).Error
}

// RevokeSessionByHash 吊销当前会话。
func (r *AdminRepository) RevokeSessionByHash(ctx context.Context, hash string) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&model.AdminSession{}).
		Where("token_hash = ? AND revoked_at IS NULL", hash).
		Update("revoked_at", now).Error
}

func toAdminUserDomain(m *model.AdminUser) *domainadmin.User {
	return &domainadmin.User{
		ID:           m.ID,
		Username:     m.Username,
		PasswordHash: m.PasswordHash,
		Active:       m.Active,
		LastLoginAt:  m.LastLoginAt,
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
	}
}

func toAdminSessionDomain(m *model.AdminSession) *domainadmin.Session {
	return &domainadmin.Session{
		ID:         m.ID,
		UserID:     m.UserID,
		TokenHash:  m.TokenHash,
		ExpiresAt:  m.ExpiresAt,
		LastUsedAt: m.LastUsedAt,
		RevokedAt:  m.RevokedAt,
		CreatedAt:  m.CreatedAt,
	}
}
