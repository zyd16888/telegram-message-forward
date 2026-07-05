// Package rule 提供路由规则管理应用服务。
package rule

import (
	"context"
	"fmt"
	"time"

	domainfilter "telegram-message-forward/internal/domain/filter"
	domainmessage "telegram-message-forward/internal/domain/message"
	domainrule "telegram-message-forward/internal/domain/rule"
	domainsink "telegram-message-forward/internal/domain/sink"
	domaintemplate "telegram-message-forward/internal/domain/template"
	"telegram-message-forward/internal/ruleengine"
	"telegram-message-forward/internal/ruleengine/condition"
	"telegram-message-forward/internal/ruleengine/processor"
)

// Service 是规则应用服务。
type Service struct {
	repo      domainrule.Repository
	sinks     domainsink.Repository
	templates domaintemplate.Repository
	filters   domainfilter.Repository
}

// ValidatorDeps 是规则目标校验所需依赖；测试可不传，此时只做仓储写入。
type ValidatorDeps struct {
	Sinks     domainsink.Repository
	Templates domaintemplate.Repository
	Filters   domainfilter.Repository
}

// NewService 创建规则服务。
func NewService(repo domainrule.Repository, deps ...ValidatorDeps) *Service {
	s := &Service{repo: repo}
	if len(deps) > 0 {
		s.sinks = deps[0].Sinks
		s.templates = deps[0].Templates
		s.filters = deps[0].Filters
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
	FilterID    int64
	Conditions  []domainrule.ConditionConfig
	Processors  []domainrule.ProcessorConfig
	StopOnMatch bool
	SourceIDs   []int64
	Targets     []domainrule.Target
}

type PreviewMessage struct {
	SourceID       int64
	MessageType    string
	SenderPeerType string
	SenderID       int64
	SenderName     string
	Text           string
	Media          []domainmessage.Media
	OriginalURL    string
	ReceivedAt     time.Time
}

type PreviewInput struct {
	Rule    Input
	Message PreviewMessage
}

type PreviewTarget struct {
	SinkID     int64
	TemplateID *int64
}

type PreviewResult struct {
	Matched       bool
	ProcessedText string
	Media         []domainmessage.Media
	Targets       []PreviewTarget
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
	if err := s.validateFilter(ctx, r.FilterID); err != nil {
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
	if err := s.validateFilter(ctx, r.FilterID); err != nil {
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

// Preview 用一条手工样例消息预演当前规则草稿，不要求规则先落库。
func (s *Service) Preview(ctx context.Context, in PreviewInput) (*PreviewResult, error) {
	r := toRule(0, in.Rule)
	r.Enabled = true
	if r.FilterID > 0 && s.filters != nil {
		f, err := s.filters.GetByID(ctx, r.FilterID)
		if err != nil {
			return nil, fmt.Errorf("引用的过滤器不存在 (id=%d): %w", r.FilterID, err)
		}
		r.Conditions = f.Conditions
	}
	if err := s.validateRuleConfig(r); err != nil {
		return nil, err
	}
	if err := s.validateTargets(ctx, r.Targets); err != nil {
		return nil, err
	}
	msg := &domainmessage.NormalizedMessage{
		SourceID:       in.Message.SourceID,
		MessageType:    in.Message.MessageType,
		SenderPeerType: in.Message.SenderPeerType,
		SenderID:       in.Message.SenderID,
		SenderName:     in.Message.SenderName,
		Text:           in.Message.Text,
		Media:          in.Message.Media,
		OriginalURL:    in.Message.OriginalURL,
		ReceivedAt:     in.Message.ReceivedAt,
	}
	if msg.MessageType == "" {
		msg.MessageType = "text"
	}
	if msg.ReceivedAt.IsZero() {
		msg.ReceivedAt = time.Now()
	}
	matches, err := ruleengine.NewEngine().Evaluate(ctx, msg, []*domainrule.Rule{r})
	if err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		return &PreviewResult{Matched: false, ProcessedText: msg.Text, Media: msg.Media}, nil
	}
	match := matches[0]
	targets := make([]PreviewTarget, 0, len(match.Targets))
	for _, target := range match.Targets {
		targets = append(targets, PreviewTarget{SinkID: target.SinkID, TemplateID: target.TemplateID})
	}
	return &PreviewResult{
		Matched:       true,
		ProcessedText: match.Message.Text,
		Media:         match.Message.Media,
		Targets:       targets,
	}, nil
}

func toRule(id int64, in Input) *domainrule.Rule {
	return &domainrule.Rule{
		ID:          id,
		Name:        in.Name,
		Enabled:     in.Enabled,
		Priority:    in.Priority,
		FilterID:    in.FilterID,
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

func (s *Service) validateFilter(ctx context.Context, filterID int64) error {
	if filterID <= 0 || s.filters == nil {
		return nil
	}
	if _, err := s.filters.GetByID(ctx, filterID); err != nil {
		return fmt.Errorf("引用的过滤器不存在 (id=%d): %w", filterID, err)
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
