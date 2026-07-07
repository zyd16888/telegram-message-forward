package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	domainfilter "telegram-message-forward/internal/domain/filter"
	domainrule "telegram-message-forward/internal/domain/rule"
	"telegram-message-forward/internal/storage/model"
)

// FilterRepository 是 filter.Repository 的 PostgreSQL 实现。
type FilterRepository struct {
	db *gorm.DB
}

func NewFilterRepository(db *gorm.DB) *FilterRepository {
	return &FilterRepository{db: db}
}

var _ domainfilter.Repository = (*FilterRepository)(nil)

func (r *FilterRepository) List(ctx context.Context) ([]*domainfilter.Filter, error) {
	var ms []model.Filter
	if err := r.db.WithContext(ctx).Order("id ASC").Find(&ms).Error; err != nil {
		return nil, err
	}
	out := make([]*domainfilter.Filter, 0, len(ms))
	for i := range ms {
		f, err := toFilterDomain(&ms[i])
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, nil
}

func (r *FilterRepository) GetByID(ctx context.Context, id int64) (*domainfilter.Filter, error) {
	var m model.Filter
	if err := r.db.WithContext(ctx).First(&m, id).Error; err != nil {
		return nil, err
	}
	return toFilterDomain(&m)
}

func (r *FilterRepository) Create(ctx context.Context, f *domainfilter.Filter) error {
	m, err := toFilterModel(f)
	if err != nil {
		return err
	}
	if err := r.db.WithContext(ctx).Create(m).Error; err != nil {
		return err
	}
	f.ID = m.ID
	f.CreatedAt = m.CreatedAt
	f.UpdatedAt = m.UpdatedAt
	return nil
}

func (r *FilterRepository) Update(ctx context.Context, f *domainfilter.Filter) error {
	conds, err := marshalJSON(f.Conditions)
	if err != nil {
		return err
	}
	now := time.Now()
	if err := r.db.WithContext(ctx).Model(&model.Filter{}).
		Where("id = ?", f.ID).
		Updates(map[string]any{
			"name":        f.Name,
			"description": f.Description,
			"conditions":  conds,
			"updated_at":  now,
		}).Error; err != nil {
		return err
	}
	f.UpdatedAt = now
	return nil
}

func (r *FilterRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.Filter{}, id).Error
}

func (r *FilterRepository) CountReferences(ctx context.Context, id int64) (int64, error) {
	var ruleCount int64
	if err := r.db.WithContext(ctx).Model(&model.RuleFilter{}).Where("filter_id = ?", id).Count(&ruleCount).Error; err != nil {
		return 0, err
	}
	var profileCount int64
	if err := r.db.WithContext(ctx).Model(&model.AIDigestProfile{}).Where("filter_id = ?", id).Count(&profileCount).Error; err != nil {
		return 0, err
	}
	return ruleCount + profileCount, nil
}

func toFilterModel(f *domainfilter.Filter) (*model.Filter, error) {
	conds, err := marshalJSON(f.Conditions)
	if err != nil {
		return nil, err
	}
	return &model.Filter{
		ID:          f.ID,
		Name:        f.Name,
		Description: f.Description,
		Conditions:  conds,
		CreatedAt:   f.CreatedAt,
		UpdatedAt:   f.UpdatedAt,
	}, nil
}

func toFilterDomain(m *model.Filter) (*domainfilter.Filter, error) {
	var conds []domainrule.ConditionConfig
	if err := unmarshalJSON(m.Conditions, &conds); err != nil {
		return nil, err
	}
	return &domainfilter.Filter{
		ID:          m.ID,
		Name:        m.Name,
		Description: m.Description,
		Conditions:  conds,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}, nil
}

// loadFilterConditions 一次查询取回多个过滤器的条件，供规则加载时按 filter_ids 覆盖内联条件。
func loadFilterConditions(ctx context.Context, db *gorm.DB, ids []int64) (map[int64][]domainrule.ConditionConfig, error) {
	out := map[int64][]domainrule.ConditionConfig{}
	if len(ids) == 0 {
		return out, nil
	}
	var ms []model.Filter
	if err := db.WithContext(ctx).Where("id IN ?", ids).Find(&ms).Error; err != nil {
		return nil, err
	}
	for i := range ms {
		var conds []domainrule.ConditionConfig
		if err := unmarshalJSON(ms[i].Conditions, &conds); err != nil {
			return nil, err
		}
		out[ms[i].ID] = conds
	}
	return out, nil
}
