// Package rule 提供路由规则管理应用服务。
package rule

import (
	"context"
	"fmt"

	domainrule "telegram-message-forward/internal/domain/rule"
	domainsink "telegram-message-forward/internal/domain/sink"
	domaintemplate "telegram-message-forward/internal/domain/template"
	"telegram-message-forward/internal/ruleengine/condition"
	"telegram-message-forward/internal/ruleengine/processor"
)

// Service 是规则应用服务。
type Service struct {
	repo      domainrule.Repository
	sinks     domainsink.Repository
	templates domaintemplate.Repository
}

// ValidatorDeps 是规则目标校验所需依赖；测试可不传，此时只做仓储写入。
type ValidatorDeps struct {
	Sinks     domainsink.Repository
	Templates domaintemplate.Repository
}

// NewService 创建规则服务。
func NewService(repo domainrule.Repository, deps ...ValidatorDeps) *Service {
	s := &Service{repo: repo}
	if len(deps) > 0 {
		s.sinks = deps[0].Sinks
		s.templates = deps[0].Templates
	}
	return s
}

// List 返回全部规则。
func (s *Service) List(ctx context.Context) ([]*domainrule.Rule, error) {
	return s.repo.List(ctx)
}

// Get 查询单个规则。
func (s *Service) Get(ctx context.Context, id int64) (*domainrule.Rule, error) {
	return s.repo.GetByID(ctx, id)
}

// Input 是规则的创建/更新输入。
type Input struct {
	Name        string
	Enabled     bool
	Priority    int
	Conditions  []domainrule.ConditionConfig
	Processors  []domainrule.ProcessorConfig
	StopOnMatch bool
	SourceIDs   []int64
	Targets     []domainrule.Target
}

// Create 创建规则及其来源/目标关联。
func (s *Service) Create(ctx context.Context, in Input) (*domainrule.Rule, error) {
	r := toRule(0, in)
	if err := s.validateRuleConfig(r); err != nil {
		return nil, err
	}
	if err := s.validateTargets(ctx, r.Targets); err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, r); err != nil {
		return nil, err
	}
	return r, nil
}

// Update 更新规则并重建关联。
func (s *Service) Update(ctx context.Context, id int64, in Input) (*domainrule.Rule, error) {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	r := toRule(existing.ID, in)
	r.CreatedAt = existing.CreatedAt
	if err := s.validateRuleConfig(r); err != nil {
		return nil, err
	}
	if err := s.validateTargets(ctx, r.Targets); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, r); err != nil {
		return nil, err
	}
	return r, nil
}

// Delete 删除规则。
func (s *Service) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}

// ConditionDescriptors 返回已注册条件的后台配置元数据。
func (s *Service) ConditionDescriptors() []condition.Descriptor {
	return condition.Descriptors()
}

// ProcessorDescriptors 返回已注册处理器的后台配置元数据。
func (s *Service) ProcessorDescriptors() []processor.Descriptor {
	return processor.Descriptors()
}

func toRule(id int64, in Input) *domainrule.Rule {
	return &domainrule.Rule{
		ID:          id,
		Name:        in.Name,
		Enabled:     in.Enabled,
		Priority:    in.Priority,
		Conditions:  in.Conditions,
		Processors:  in.Processors,
		StopOnMatch: in.StopOnMatch,
		SourceIDs:   in.SourceIDs,
		Targets:     in.Targets,
	}
}

func (s *Service) validateRuleConfig(r *domainrule.Rule) error {
	for _, cfg := range r.Conditions {
		if err := condition.ValidateConfig(cfg.Type, cfg.Config); err != nil {
			return fmt.Errorf("条件 %s 配置无效: %w", cfg.Type, err)
		}
	}
	for _, cfg := range r.Processors {
		if err := processor.ValidateConfig(cfg.Type, cfg.Config); err != nil {
			return fmt.Errorf("处理器 %s 配置无效: %w", cfg.Type, err)
		}
	}
	return nil
}

func (s *Service) validateTargets(ctx context.Context, targets []domainrule.Target) error {
	if s.sinks == nil || s.templates == nil {
		return nil
	}
	for _, target := range targets {
		sk, err := s.sinks.GetByID(ctx, target.SinkID)
		if err != nil {
			return fmt.Errorf("查询规则目标渠道失败 sink_id=%d: %w", target.SinkID, err)
		}
		format := domaintemplate.FormatText
		if target.TemplateID != nil {
			tpl, err := s.templates.GetByID(ctx, *target.TemplateID)
			if err != nil {
				return fmt.Errorf("查询规则目标模板失败 template_id=%d: %w", *target.TemplateID, err)
			}
			format = tpl.Format
		}
		if !sinkSupportsFormat(sk.Capabilities, format) {
			return fmt.Errorf("渠道 %s 不支持模板格式 %s", sk.Type, format)
		}
	}
	return nil
}

func sinkSupportsFormat(c domainsink.Capabilities, f domaintemplate.Format) bool {
	switch f {
	case domaintemplate.FormatText:
		return c.SupportsText
	case domaintemplate.FormatMarkdown:
		return c.SupportsMarkdown
	case domaintemplate.FormatHTML:
		return c.SupportsHTML
	default:
		return false
	}
}
