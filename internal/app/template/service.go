// Package template 提供模板管理应用服务。
package template

import (
	"context"
	"fmt"

	domaintemplate "telegram-message-forward/internal/domain/template"
)

// Service 是模板应用服务。
type Service struct {
	repo domaintemplate.Repository
}

// NewService 创建模板服务。
func NewService(repo domaintemplate.Repository) *Service {
	return &Service{repo: repo}
}

// List 返回全部模板。
func (s *Service) List(ctx context.Context) ([]*domaintemplate.Template, error) {
	return s.repo.List(ctx)
}

// Get 查询单个模板。
func (s *Service) Get(ctx context.Context, id int64) (*domaintemplate.Template, error) {
	return s.repo.GetByID(ctx, id)
}

// Input 是模板的创建/更新输入。
type Input struct {
	Name    string
	Format  string
	Content string
}

func validFormat(f string) bool {
	switch domaintemplate.Format(f) {
	case domaintemplate.FormatText, domaintemplate.FormatMarkdown, domaintemplate.FormatHTML:
		return true
	}
	return false
}

// Create 创建模板。
func (s *Service) Create(ctx context.Context, in Input) (*domaintemplate.Template, error) {
	if !validFormat(in.Format) {
		return nil, fmt.Errorf("非法模板格式: %s", in.Format)
	}
	t := &domaintemplate.Template{Name: in.Name, Format: domaintemplate.Format(in.Format), Content: in.Content}
	if err := s.repo.Create(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

// Update 更新模板。
func (s *Service) Update(ctx context.Context, id int64, in Input) (*domaintemplate.Template, error) {
	t, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if in.Name != "" {
		t.Name = in.Name
	}
	if in.Format != "" {
		if !validFormat(in.Format) {
			return nil, fmt.Errorf("非法模板格式: %s", in.Format)
		}
		t.Format = domaintemplate.Format(in.Format)
	}
	if in.Content != "" {
		t.Content = in.Content
	}
	if err := s.repo.Update(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

// Delete 删除模板。
func (s *Service) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}
