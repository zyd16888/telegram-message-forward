package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"gorm.io/datatypes"
	"gorm.io/gorm"

	domainsource "telegram-message-forward/internal/domain/source"
	"telegram-message-forward/internal/storage/model"
)

// SourceRepository 是 source.Repository 的 PostgreSQL 实现。
type SourceRepository struct {
	db *gorm.DB
}

// NewSourceRepository 创建监听源仓储。
func NewSourceRepository(db *gorm.DB) *SourceRepository {
	return &SourceRepository{db: db}
}

var _ domainsource.Repository = (*SourceRepository)(nil)

// Create 插入监听源。
func (r *SourceRepository) Create(ctx context.Context, s *domainsource.Source) error {
	m, err := toSourceModel(s)
	if err != nil {
		return err
	}
	if err := r.db.WithContext(ctx).Create(m).Error; err != nil {
		return err
	}
	s.ID = m.ID
	return nil
}

// Update 更新监听源。
func (r *SourceRepository) Update(ctx context.Context, s *domainsource.Source) error {
	m, err := toSourceModel(s)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Save(m).Error
}

// GetByID 按 id 查询监听源。
func (r *SourceRepository) GetByID(ctx context.Context, id int64) (*domainsource.Source, error) {
	var m model.Source
	if err := r.db.WithContext(ctx).First(&m, id).Error; err != nil {
		return nil, err
	}
	return toSourceDomain(&m)
}

// List 返回全部监听源。
func (r *SourceRepository) List(ctx context.Context) ([]*domainsource.Source, error) {
	var ms []model.Source
	if err := r.db.WithContext(ctx).Order("id").Find(&ms).Error; err != nil {
		return nil, err
	}
	return sourcesToDomain(ms)
}

// ListByAccount 返回某账号下全部监听源。
func (r *SourceRepository) ListByAccount(ctx context.Context, accountID int64) ([]*domainsource.Source, error) {
	var ms []model.Source
	if err := r.db.WithContext(ctx).Where("account_id = ?", accountID).Order("id").Find(&ms).Error; err != nil {
		return nil, err
	}
	return sourcesToDomain(ms)
}

// ListEnabled 返回全部启用的监听源。
func (r *SourceRepository) ListEnabled(ctx context.Context) ([]*domainsource.Source, error) {
	var ms []model.Source
	if err := r.db.WithContext(ctx).Where("enabled = ?", true).Order("id").Find(&ms).Error; err != nil {
		return nil, err
	}
	return sourcesToDomain(ms)
}

// Delete 删除监听源。
func (r *SourceRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.Source{}, id).Error
}

func sourcesToDomain(ms []model.Source) ([]*domainsource.Source, error) {
	out := make([]*domainsource.Source, 0, len(ms))
	for i := range ms {
		s, err := toSourceDomain(&ms[i])
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

func toSourceModel(s *domainsource.Source) (*model.Source, error) {
	cfg, err := marshalJSONMap(s.Config)
	if err != nil {
		return nil, fmt.Errorf("序列化 source config 失败: %w", err)
	}
	sourceType := s.Type
	if sourceType == "" {
		sourceType = "telegram"
	}
	var accountID *int64
	if s.AccountID > 0 {
		accountID = &s.AccountID
	}
	return &model.Source{
		ID:            s.ID,
		Type:          sourceType,
		AccountID:     accountID,
		PeerType:      string(s.PeerType),
		PeerID:        s.PeerID,
		Name:          s.Name,
		Username:      s.Username,
		Enabled:       s.Enabled,
		Config:        cfg,
		LastMessageID: s.LastMessageID,
		LastSyncedAt:  s.LastSyncedAt,
		CreatedAt:     s.CreatedAt,
		UpdatedAt:     s.UpdatedAt,
	}, nil
}

func toSourceDomain(m *model.Source) (*domainsource.Source, error) {
	cfg, err := unmarshalJSONMap(m.Config)
	if err != nil {
		return nil, fmt.Errorf("解析 source config 失败: %w", err)
	}
	sourceType := m.Type
	if sourceType == "" {
		sourceType = "telegram"
	}
	accountID := int64(0)
	if m.AccountID != nil {
		accountID = *m.AccountID
	}
	return &domainsource.Source{
		ID:            m.ID,
		Type:          sourceType,
		AccountID:     accountID,
		PeerType:      domainsource.PeerType(m.PeerType),
		PeerID:        m.PeerID,
		Name:          m.Name,
		Username:      m.Username,
		Enabled:       m.Enabled,
		Config:        cfg,
		LastMessageID: m.LastMessageID,
		LastSyncedAt:  m.LastSyncedAt,
		CreatedAt:     m.CreatedAt,
		UpdatedAt:     m.UpdatedAt,
	}, nil
}

// marshalJSONMap 序列化 map[string]any 为 datatypes.JSON，nil/空返回 "{}"。
func marshalJSONMap(m map[string]any) (datatypes.JSON, error) {
	if len(m) == 0 {
		return datatypes.JSON([]byte("{}")), nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	return datatypes.JSON(b), nil
}

// unmarshalJSONMap 解析 datatypes.JSON 为 map[string]any。
func unmarshalJSONMap(j datatypes.JSON) (map[string]any, error) {
	if len(j) == 0 {
		return nil, nil
	}
	var m map[string]any
	if err := json.Unmarshal(j, &m); err != nil {
		return nil, err
	}
	return m, nil
}
