package repository

import (
	"context"

	"gorm.io/gorm"

	domaintemplate "telegram-message-forward/internal/domain/template"
	"telegram-message-forward/internal/storage/model"
)

// TemplateRepository 是 template.Repository 的 PostgreSQL 实现。
type TemplateRepository struct {
	db *gorm.DB
}

// NewTemplateRepository 创建模板仓储。
func NewTemplateRepository(db *gorm.DB) *TemplateRepository {
	return &TemplateRepository{db: db}
}

var _ domaintemplate.Repository = (*TemplateRepository)(nil)

// Create 插入模板。
func (r *TemplateRepository) Create(ctx context.Context, t *domaintemplate.Template) error {
	m := toTemplateModel(t)
	if err := r.db.WithContext(ctx).Create(m).Error; err != nil {
		return err
	}
	t.ID = m.ID
	return nil
}

// Update 更新模板。
func (r *TemplateRepository) Update(ctx context.Context, t *domaintemplate.Template) error {
	return r.db.WithContext(ctx).Save(toTemplateModel(t)).Error
}

// GetByID 按 id 查询模板。
func (r *TemplateRepository) GetByID(ctx context.Context, id int64) (*domaintemplate.Template, error) {
	var m model.Template
	if err := r.db.WithContext(ctx).First(&m, id).Error; err != nil {
		return nil, err
	}
	return toTemplateDomain(&m), nil
}

// List 返回全部模板。
func (r *TemplateRepository) List(ctx context.Context) ([]*domaintemplate.Template, error) {
	var ms []model.Template
	if err := r.db.WithContext(ctx).Order("id").Find(&ms).Error; err != nil {
		return nil, err
	}
	out := make([]*domaintemplate.Template, 0, len(ms))
	for i := range ms {
		out = append(out, toTemplateDomain(&ms[i]))
	}
	return out, nil
}

// Delete 删除模板。
func (r *TemplateRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.Template{}, id).Error
}

func toTemplateModel(t *domaintemplate.Template) *model.Template {
	return &model.Template{
		ID:        t.ID,
		Name:      t.Name,
		Format:    string(t.Format),
		Content:   t.Content,
		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
	}
}

func toTemplateDomain(m *model.Template) *domaintemplate.Template {
	return &domaintemplate.Template{
		ID:        m.ID,
		Name:      m.Name,
		Format:    domaintemplate.Format(m.Format),
		Content:   m.Content,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}
}
