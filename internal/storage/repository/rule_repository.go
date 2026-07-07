package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"gorm.io/datatypes"
	"gorm.io/gorm"

	domainrule "telegram-message-forward/internal/domain/rule"
	"telegram-message-forward/internal/storage/model"
)

// RuleRepository 是 rule.Repository 的 PostgreSQL 实现。
//
// 规则本体存 rules 表，来源与目标分别存 rule_sources / rule_targets 关联表，
// 读写在同一事务内完成，对外组装成完整的 rule.Rule。
type RuleRepository struct {
	db *gorm.DB
}

// NewRuleRepository 创建规则仓储。
func NewRuleRepository(db *gorm.DB) *RuleRepository {
	return &RuleRepository{db: db}
}

var _ domainrule.Repository = (*RuleRepository)(nil)

// Create 插入规则及其关联的来源与目标。
func (r *RuleRepository) Create(ctx context.Context, rule *domainrule.Rule) error {
	m, err := toRuleModel(rule)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(m).Error; err != nil {
			return err
		}
		rule.ID = m.ID
		return r.replaceAssociations(tx, rule)
	})
}

// Update 更新规则并重建其关联。
func (r *RuleRepository) Update(ctx context.Context, rule *domainrule.Rule) error {
	m, err := toRuleModel(rule)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(m).Error; err != nil {
			return err
		}
		return r.replaceAssociations(tx, rule)
	})
}

// replaceAssociations 用规则当前的 SourceIDs / Targets 重建关联表。
func (r *RuleRepository) replaceAssociations(tx *gorm.DB, rule *domainrule.Rule) error {
	if err := tx.Where("rule_id = ?", rule.ID).Delete(&model.RuleSource{}).Error; err != nil {
		return err
	}
	if err := tx.Where("rule_id = ?", rule.ID).Delete(&model.RuleFilter{}).Error; err != nil {
		return err
	}
	if err := tx.Where("rule_id = ?", rule.ID).Delete(&model.RuleTarget{}).Error; err != nil {
		return err
	}
	for _, sid := range rule.SourceIDs {
		if err := tx.Create(&model.RuleSource{RuleID: rule.ID, SourceID: sid}).Error; err != nil {
			return err
		}
	}
	for i, fid := range rule.FilterIDs {
		if err := tx.Create(&model.RuleFilter{RuleID: rule.ID, FilterID: fid, SortOrder: i}).Error; err != nil {
			return err
		}
	}
	for _, t := range rule.Targets {
		if err := tx.Create(&model.RuleTarget{RuleID: rule.ID, SinkID: t.SinkID, TemplateID: t.TemplateID}).Error; err != nil {
			return err
		}
	}
	return nil
}

// GetByID 按 id 查询规则及其关联。
func (r *RuleRepository) GetByID(ctx context.Context, id int64) (*domainrule.Rule, error) {
	var m model.Rule
	if err := r.db.WithContext(ctx).First(&m, id).Error; err != nil {
		return nil, err
	}
	rule, err := r.loadFull(ctx, &m)
	if err != nil {
		return nil, err
	}
	if err := r.resolveRuleFilters(ctx, []*domainrule.Rule{rule}); err != nil {
		return nil, err
	}
	return rule, nil
}

// List 返回全部规则，按 priority、id 排序。
func (r *RuleRepository) List(ctx context.Context) ([]*domainrule.Rule, error) {
	var ms []model.Rule
	if err := r.db.WithContext(ctx).Order("priority DESC, id").Find(&ms).Error; err != nil {
		return nil, err
	}
	return r.assembleRules(ctx, ms)
}

// ListEnabledBySource 返回命中某 source 的启用规则，按 priority 排序。
func (r *RuleRepository) ListEnabledBySource(ctx context.Context, sourceID int64) ([]*domainrule.Rule, error) {
	var ms []model.Rule
	err := r.db.WithContext(ctx).
		Joins("JOIN rule_sources rs ON rs.rule_id = rules.id").
		Where("rs.source_id = ? AND rules.enabled = ?", sourceID, true).
		Order("rules.priority DESC, rules.id").
		Find(&ms).Error
	if err != nil {
		return nil, err
	}
	return r.assembleRules(ctx, ms)
}

// Delete 删除规则（关联表由外键级联删除）。
func (r *RuleRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.Rule{}, id).Error
}

func (r *RuleRepository) assembleRules(ctx context.Context, ms []model.Rule) ([]*domainrule.Rule, error) {
	out := make([]*domainrule.Rule, 0, len(ms))
	for i := range ms {
		full, err := r.loadFull(ctx, &ms[i])
		if err != nil {
			return nil, err
		}
		out = append(out, full)
	}
	if err := r.resolveRuleFilters(ctx, out); err != nil {
		return nil, err
	}
	return out, nil
}

// resolveRuleFilters 对引用了共享过滤器的规则，用过滤器条件覆盖其内联条件（批量一次查询）。
func (r *RuleRepository) resolveRuleFilters(ctx context.Context, rules []*domainrule.Rule) error {
	idset := map[int64]struct{}{}
	for _, rule := range rules {
		for _, filterID := range rule.FilterIDs {
			idset[filterID] = struct{}{}
		}
	}
	if len(idset) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(idset))
	for id := range idset {
		ids = append(ids, id)
	}
	condMap, err := loadFilterConditions(ctx, r.db, ids)
	if err != nil {
		return err
	}
	for _, rule := range rules {
		if len(rule.FilterIDs) > 0 {
			conds := make([]domainrule.ConditionConfig, 0)
			for _, filterID := range rule.FilterIDs {
				conds = append(conds, condMap[filterID]...)
			}
			rule.Conditions = conds
		}
	}
	return nil
}

func (r *RuleRepository) loadFull(ctx context.Context, m *model.Rule) (*domainrule.Rule, error) {
	rule, err := toRuleDomain(m)
	if err != nil {
		return nil, err
	}

	var rs []model.RuleSource
	if err := r.db.WithContext(ctx).Where("rule_id = ?", m.ID).Find(&rs).Error; err != nil {
		return nil, err
	}
	rule.SourceIDs = make([]int64, 0, len(rs))
	for _, x := range rs {
		rule.SourceIDs = append(rule.SourceIDs, x.SourceID)
	}

	var rf []model.RuleFilter
	if err := r.db.WithContext(ctx).Where("rule_id = ?", m.ID).Order("sort_order, filter_id").Find(&rf).Error; err != nil {
		return nil, err
	}
	rule.FilterIDs = make([]int64, 0, len(rf))
	for _, x := range rf {
		rule.FilterIDs = append(rule.FilterIDs, x.FilterID)
	}

	var rt []model.RuleTarget
	if err := r.db.WithContext(ctx).Where("rule_id = ?", m.ID).Order("id").Find(&rt).Error; err != nil {
		return nil, err
	}
	rule.Targets = make([]domainrule.Target, 0, len(rt))
	for _, x := range rt {
		rule.Targets = append(rule.Targets, domainrule.Target{SinkID: x.SinkID, TemplateID: x.TemplateID})
	}
	return rule, nil
}

func toRuleModel(rule *domainrule.Rule) (*model.Rule, error) {
	conds, err := json.Marshal(rule.Conditions)
	if err != nil {
		return nil, fmt.Errorf("序列化 conditions 失败: %w", err)
	}
	if string(conds) == "null" {
		conds = []byte("[]")
	}
	procs, err := json.Marshal(rule.Processors)
	if err != nil {
		return nil, fmt.Errorf("序列化 processors 失败: %w", err)
	}
	if string(procs) == "null" {
		procs = []byte("[]")
	}
	return &model.Rule{
		ID:          rule.ID,
		Name:        rule.Name,
		Enabled:     rule.Enabled,
		Priority:    rule.Priority,
		Conditions:  datatypes.JSON(conds),
		Processors:  datatypes.JSON(procs),
		StopOnMatch: rule.StopOnMatch,
		CreatedAt:   rule.CreatedAt,
		UpdatedAt:   rule.UpdatedAt,
	}, nil
}

func toRuleDomain(m *model.Rule) (*domainrule.Rule, error) {
	var conds []domainrule.ConditionConfig
	if len(m.Conditions) > 0 {
		if err := json.Unmarshal(m.Conditions, &conds); err != nil {
			return nil, fmt.Errorf("解析 conditions 失败: %w", err)
		}
	}
	var procs []domainrule.ProcessorConfig
	if len(m.Processors) > 0 {
		if err := json.Unmarshal(m.Processors, &procs); err != nil {
			return nil, fmt.Errorf("解析 processors 失败: %w", err)
		}
	}
	rule := &domainrule.Rule{
		ID:          m.ID,
		Name:        m.Name,
		Enabled:     m.Enabled,
		Priority:    m.Priority,
		Conditions:  conds,
		Processors:  procs,
		StopOnMatch: m.StopOnMatch,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
	return rule, nil
}
