package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"

	domainsink "telegram-message-forward/internal/domain/sink"
	"telegram-message-forward/internal/infra/crypto"
	"telegram-message-forward/internal/storage/model"
)

// SinkRepository 是 sink.Repository 的 PostgreSQL 实现。secret 加密落库。
type SinkRepository struct {
	db     *gorm.DB
	cipher *crypto.Cipher
}

// NewSinkRepository 创建渠道仓储。
func NewSinkRepository(db *gorm.DB, cipher *crypto.Cipher) *SinkRepository {
	return &SinkRepository{db: db, cipher: cipher}
}

var _ domainsink.Repository = (*SinkRepository)(nil)

// Create 插入渠道，secret 加密。
func (r *SinkRepository) Create(ctx context.Context, s *domainsink.Sink) error {
	m, err := r.toModel(s)
	if err != nil {
		return err
	}
	if err := r.db.WithContext(ctx).Create(m).Error; err != nil {
		return err
	}
	s.ID = m.ID
	return nil
}

// Update 更新渠道。
func (r *SinkRepository) Update(ctx context.Context, s *domainsink.Sink) error {
	m, err := r.toModel(s)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Save(m).Error
}

// GetByID 按 id 查询渠道。
func (r *SinkRepository) GetByID(ctx context.Context, id int64) (*domainsink.Sink, error) {
	var m model.Sink
	if err := r.db.WithContext(ctx).First(&m, id).Error; err != nil {
		return nil, err
	}
	s, err := r.toDomain(&m)
	if err != nil {
		return nil, err
	}
	if err := r.migrateLegacyConfig(ctx, &m, s.Config); err != nil {
		return nil, err
	}
	return s, nil
}

// List 返回全部渠道。
func (r *SinkRepository) List(ctx context.Context) ([]*domainsink.Sink, error) {
	var ms []model.Sink
	if err := r.db.WithContext(ctx).Order("id").Find(&ms).Error; err != nil {
		return nil, err
	}
	out := make([]*domainsink.Sink, 0, len(ms))
	for i := range ms {
		s, err := r.toDomain(&ms[i])
		if err != nil {
			return nil, err
		}
		if err := r.migrateLegacyConfig(ctx, &ms[i], s.Config); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

// Delete 删除渠道。
func (r *SinkRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		checks := []struct {
			label string
			query *gorm.DB
		}{
			{label: "Flow", query: tx.Model(&model.FlowNode{}).Where("type = ? AND ref_id = ?", "target", id)},
			{label: "历史投递", query: tx.Model(&model.DeliveryTask{}).Where("sink_id = ?", id)},
			{label: "AI 整理", query: tx.Model(&model.AIDigestProfile{}).Where("target_sink_ids @> ?::jsonb", fmt.Sprintf("[%d]", id))},
		}
		for _, check := range checks {
			var count int64
			if err := check.query.Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return fmt.Errorf("%w：%s中仍有 %d 处引用，请先禁用渠道或移除引用", domainsink.ErrInUse, check.label, count)
			}
		}
		return tx.Delete(&model.Sink{}, id).Error
	})
}

// UpdateTestResult 更新渠道最近一次连通性测试结果。
func (r *SinkRepository) UpdateTestResult(ctx context.Context, id int64, at time.Time, success bool, errText string) error {
	return r.db.WithContext(ctx).
		Model(&model.Sink{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"last_test_at":      at,
			"last_test_success": success,
			"last_test_error":   errText,
			"updated_at":        time.Now(),
		}).Error
}

func (r *SinkRepository) toModel(s *domainsink.Sink) (*model.Sink, error) {
	cfg, err := marshalJSONMap(s.Config)
	if err != nil {
		return nil, fmt.Errorf("序列化 sink config 失败: %w", err)
	}
	cfgEnc, err := r.cipher.Encrypt(cfg)
	if err != nil {
		return nil, fmt.Errorf("加密 sink config 失败: %w", err)
	}
	secretEnc, err := r.cipher.Encrypt(s.Secret)
	if err != nil {
		return nil, fmt.Errorf("加密 sink secret 失败: %w", err)
	}
	caps, err := json.Marshal(s.Capabilities)
	if err != nil {
		return nil, fmt.Errorf("序列化 sink capabilities 失败: %w", err)
	}
	return &model.Sink{
		ID:              s.ID,
		Type:            s.Type,
		Name:            s.Name,
		Enabled:         s.Enabled,
		Config:          datatypes.JSON([]byte(`{}`)),
		ConfigEncrypted: cfgEnc,
		SecretEncrypted: secretEnc,
		Capabilities:    datatypes.JSON(caps),
		LastTestAt:      s.Observability.LastTestAt,
		LastTestSuccess: s.Observability.LastTestSuccess,
		LastTestError:   s.Observability.LastTestError,
		CreatedAt:       s.CreatedAt,
		UpdatedAt:       s.UpdatedAt,
	}, nil
}

func (r *SinkRepository) toDomain(m *model.Sink) (*domainsink.Sink, error) {
	cfgJSON := []byte(m.Config)
	if len(m.ConfigEncrypted) > 0 {
		var err error
		cfgJSON, err = r.cipher.Decrypt(m.ConfigEncrypted)
		if err != nil {
			return nil, fmt.Errorf("解密 sink config 失败: %w", err)
		}
	}
	cfg, err := unmarshalJSONMap(cfgJSON)
	if err != nil {
		return nil, fmt.Errorf("解析 sink config 失败: %w", err)
	}
	secret, err := r.cipher.Decrypt(m.SecretEncrypted)
	if err != nil {
		return nil, fmt.Errorf("解密 sink secret 失败: %w", err)
	}
	var caps domainsink.Capabilities
	if len(m.Capabilities) > 0 {
		if err := json.Unmarshal(m.Capabilities, &caps); err != nil {
			return nil, fmt.Errorf("解析 sink capabilities 失败: %w", err)
		}
	}
	return &domainsink.Sink{
		ID:           m.ID,
		Type:         m.Type,
		Name:         m.Name,
		Enabled:      m.Enabled,
		Config:       cfg,
		Secret:       secret,
		Capabilities: caps,
		Observability: domainsink.Observability{
			LastTestAt:      m.LastTestAt,
			LastTestSuccess: m.LastTestSuccess,
			LastTestError:   m.LastTestError,
		},
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}, nil
}

func (r *SinkRepository) migrateLegacyConfig(ctx context.Context, m *model.Sink, config map[string]any) error {
	if len(m.ConfigEncrypted) > 0 {
		return nil
	}
	cfg, err := marshalJSONMap(config)
	if err != nil {
		return err
	}
	encrypted, err := r.cipher.Encrypt(cfg)
	if err != nil {
		return fmt.Errorf("迁移 sink config 加密失败: %w", err)
	}
	return r.db.WithContext(ctx).Model(&model.Sink{}).Where("id = ? AND config_encrypted IS NULL", m.ID).Updates(map[string]any{
		"config":           datatypes.JSON([]byte(`{}`)),
		"config_encrypted": encrypted,
	}).Error
}
