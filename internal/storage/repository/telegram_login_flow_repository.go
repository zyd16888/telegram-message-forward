package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	domainloginflow "telegram-message-forward/internal/domain/loginflow"
	"telegram-message-forward/internal/infra/crypto"
	"telegram-message-forward/internal/storage/model"
)

// TelegramLoginFlowRepository 是 loginflow.Repository 的 PostgreSQL 实现。
//
// phone_code_hash 与 qr_token 加密存储；验证码明文与 2FA 密码不落库。
type TelegramLoginFlowRepository struct {
	db     *gorm.DB
	cipher *crypto.Cipher
}

// NewTelegramLoginFlowRepository 创建登录 flow 仓储。
func NewTelegramLoginFlowRepository(db *gorm.DB, cipher *crypto.Cipher) *TelegramLoginFlowRepository {
	return &TelegramLoginFlowRepository{db: db, cipher: cipher}
}

var _ domainloginflow.Repository = (*TelegramLoginFlowRepository)(nil)

// 终态集合，用于 GetActiveByAccount / ExpireStale 过滤。
var terminalStatuses = []string{
	string(domainloginflow.StatusAuthorized),
	string(domainloginflow.StatusFailed),
	string(domainloginflow.StatusCancelled),
	string(domainloginflow.StatusExpired),
}

// Create 插入 flow，敏感字段加密。
func (r *TelegramLoginFlowRepository) Create(ctx context.Context, f *domainloginflow.Flow) error {
	m, err := r.toModel(f)
	if err != nil {
		return err
	}
	if err := r.db.WithContext(ctx).Create(m).Error; err != nil {
		return err
	}
	f.ID = m.ID
	return nil
}

// Update 更新 flow。
func (r *TelegramLoginFlowRepository) Update(ctx context.Context, f *domainloginflow.Flow) error {
	m, err := r.toModel(f)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Save(m).Error
}

// GetByFlowID 按 flow_id 查询。
func (r *TelegramLoginFlowRepository) GetByFlowID(ctx context.Context, flowID string) (*domainloginflow.Flow, error) {
	var m model.TelegramLoginFlow
	if err := r.db.WithContext(ctx).Where("flow_id = ?", flowID).First(&m).Error; err != nil {
		return nil, err
	}
	return r.toDomain(&m)
}

// GetActiveByAccount 返回某账号最近一个非终态 flow；无则返回 nil,nil。
func (r *TelegramLoginFlowRepository) GetActiveByAccount(ctx context.Context, accountID int64) (*domainloginflow.Flow, error) {
	var m model.TelegramLoginFlow
	err := r.db.WithContext(ctx).
		Where("account_id = ? AND status NOT IN ?", accountID, terminalStatuses).
		Order("id DESC").
		First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return r.toDomain(&m)
}

// ExpireStale 将超期的非终态 flow 标记为 expired 并清空敏感字段。
func (r *TelegramLoginFlowRepository) ExpireStale(ctx context.Context, now time.Time) (int64, error) {
	res := r.db.WithContext(ctx).
		Model(&model.TelegramLoginFlow{}).
		Where("status NOT IN ? AND expires_at < ?", terminalStatuses, now).
		Updates(map[string]any{
			"status":                    string(domainloginflow.StatusExpired),
			"phone_code_hash_encrypted": nil,
			"qr_token_encrypted":        nil,
			"updated_at":                now,
		})
	return res.RowsAffected, res.Error
}

func (r *TelegramLoginFlowRepository) toModel(f *domainloginflow.Flow) (*model.TelegramLoginFlow, error) {
	var hashEnc []byte
	if f.PhoneCodeHash != "" {
		enc, err := r.cipher.Encrypt([]byte(f.PhoneCodeHash))
		if err != nil {
			return nil, fmt.Errorf("加密 phone_code_hash 失败: %w", err)
		}
		hashEnc = enc
	}
	qrEnc, err := r.cipher.Encrypt(f.QRToken)
	if err != nil {
		return nil, fmt.Errorf("加密 qr_token 失败: %w", err)
	}
	return &model.TelegramLoginFlow{
		ID:                     f.ID,
		FlowID:                 f.FlowID,
		AccountID:              f.AccountID,
		Method:                 string(f.Method),
		Status:                 string(f.Status),
		CurrentStep:            f.CurrentStep,
		PhoneCodeHashEncrypted: hashEnc,
		QRTokenEncrypted:       qrEnc,
		DCID:                   f.DCID,
		ExpiresAt:              f.ExpiresAt,
		LastError:              f.LastError,
		CreatedAt:              f.CreatedAt,
		UpdatedAt:              f.UpdatedAt,
		CompletedAt:            f.CompletedAt,
	}, nil
}

func (r *TelegramLoginFlowRepository) toDomain(m *model.TelegramLoginFlow) (*domainloginflow.Flow, error) {
	var hash string
	if len(m.PhoneCodeHashEncrypted) > 0 {
		dec, err := r.cipher.Decrypt(m.PhoneCodeHashEncrypted)
		if err != nil {
			return nil, fmt.Errorf("解密 phone_code_hash 失败: %w", err)
		}
		hash = string(dec)
	}
	var qr []byte
	if len(m.QRTokenEncrypted) > 0 {
		dec, err := r.cipher.Decrypt(m.QRTokenEncrypted)
		if err != nil {
			return nil, fmt.Errorf("解密 qr_token 失败: %w", err)
		}
		qr = dec
	}
	return &domainloginflow.Flow{
		ID:            m.ID,
		FlowID:        m.FlowID,
		AccountID:     m.AccountID,
		Method:        domainloginflow.Method(m.Method),
		Status:        domainloginflow.Status(m.Status),
		CurrentStep:   m.CurrentStep,
		PhoneCodeHash: hash,
		QRToken:       qr,
		DCID:          m.DCID,
		ExpiresAt:     m.ExpiresAt,
		LastError:     m.LastError,
		CreatedAt:     m.CreatedAt,
		UpdatedAt:     m.UpdatedAt,
		CompletedAt:   m.CompletedAt,
	}, nil
}
