// Package filter 提供可复用过滤器的管理应用服务。
package filter

import (
	"context"
	"errors"
	"fmt"
	"strings"

	domainfilter "telegram-message-forward/internal/domain/filter"
	domainrule "telegram-message-forward/internal/domain/rule"
	"telegram-message-forward/internal/ruleengine/condition"
)

// Service 是过滤器应用服务。
type Service struct {
	repo domainfilter.Repository
}

func NewService(repo domainfilter.Repository) *Service {
	return &Service{repo: repo}
}

// Input 是过滤器创建/更新输入。
type Input struct {
	Name        string
	Description string
	Conditions  []domainrule.ConditionConfig
}

func (s *Service) List(ctx context.Context) ([]*domainfilter.Filter, error) {
	return s.repo.List(ctx)
}

func (s *Service) Get(ctx context.Context, id int64) (*domainfilter.Filter, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) Create(ctx context.Context, in Input) (*domainfilter.Filter, error) {
	f := fromInput(0, in)
	if err := s.validate(f); err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, f); err != nil {
		return nil, err
	}
	return f, nil
}

func (s *Service) Update(ctx context.Context, id int64, in Input) (*domainfilter.Filter, error) {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	f := fromInput(id, in)
	f.CreatedAt = existing.CreatedAt
	if err := s.validate(f); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, f); err != nil {
		return nil, err
	}
	return f, nil
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	count, err := s.repo.CountReferences(ctx, id)
	if err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("该过滤器仍被 %d 处（转发规则/AI 整理）引用，无法删除", count)
	}
	return s.repo.Delete(ctx, id)
}

func fromInput(id int64, in Input) *domainfilter.Filter {
	return &domainfilter.Filter{
		ID:          id,
		Name:        strings.TrimSpace(in.Name),
		Description: strings.TrimSpace(in.Description),
		Conditions:  in.Conditions,
	}
}

func (s *Service) validate(f *domainfilter.Filter) error {
	if f.Name == "" {
		return errors.New("过滤器名称不能为空")
	}
	if len(f.Conditions) == 0 {
		return errors.New("过滤器至少需要一个匹配条件")
	}
	for _, cfg := range f.Conditions {
		if err := condition.ValidateConfig(cfg.Type, cfg.Config); err != nil {
			return fmt.Errorf("条件 %s 配置无效: %w", cfg.Type, err)
		}
	}
	return nil
}
