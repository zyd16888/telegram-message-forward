// Package aidigest 提供 AI 整理 profile、预览、执行和投递编排。
package aidigest

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	domainaidigest "telegram-message-forward/internal/domain/aidigest"
	domaindelivery "telegram-message-forward/internal/domain/delivery"
	domainfilter "telegram-message-forward/internal/domain/filter"
	domainflow "telegram-message-forward/internal/domain/flow"
	domainmessage "telegram-message-forward/internal/domain/message"
	domainsettings "telegram-message-forward/internal/domain/settings"
	domainsink "telegram-message-forward/internal/domain/sink"
	domainsource "telegram-message-forward/internal/domain/source"
	"telegram-message-forward/internal/infra/ai"
	"telegram-message-forward/internal/infra/clock"
	"telegram-message-forward/internal/ruleengine/condition"
)

const defaultPrompt = `请整理以下窗口内的消息，输出重点摘要、分类列表和来源编号。

要求：
1. 只基于输入消息，不补充外部事实。
2. 每条关键结论必须标注来源编号。
3. 合并重复信息，低价值重复内容放到低优先级。
4. 严格按照输出结构模板组织内容。
5. 末尾保留“说明：以上内容仅基于本窗口内消息整理，未使用外部事实补全。”

输出结构模板：
{{output_template}}

窗口：{{window_start}} 至 {{window_end}}
来源：{{source_list}}
消息数：{{message_count}}

{{messages}}

输出载体：{{output_format}}`

const defaultOutputTemplate = `# {{profile_name}}

## 一句话总结
用 1 段话概括本窗口最重要的信息。

## 重点摘要
- 列出 3-7 条重点，每条都标注来源编号，例如 [#1]。
- 合并重复消息，不要重复罗列同一件事。

## 分类整理
按主题分组整理，每组包含关键事实、背景线索和来源编号。

## 待关注事项
列出需要继续关注的问题、风险、待办或后续进展。

## 说明
以上内容仅基于本窗口内消息整理，未使用外部事实补全。`

const systemPrompt = `你是信息整理助手。只能基于用户提供的消息内容整理，不要编造事实。
如果输入不足以得出结论，请明确说明信息不足。
输出中的每条关键结论都要标注来源编号。
不要输出任何未在输入中出现的 secret、token、手机号、验证码或账号敏感信息。
如果内容涉及财经、医疗、法律或其它高风险领域，仅做信息整理，不构成建议。`

type Service struct {
	repo     domainaidigest.Repository
	settings domainsettings.Repository
	messages messageWindowRepository
	sources  domainsource.Repository
	sinks    domainsink.Repository
	tasks    domaindelivery.Repository
	filters  domainfilter.Repository
	clk      clock.Clock
	log      *slog.Logger
	wake     func()
	mu       sync.Mutex
}

type Deps struct {
	Repo     domainaidigest.Repository
	Settings domainsettings.Repository
	Messages messageWindowRepository
	Sources  domainsource.Repository
	Sinks    domainsink.Repository
	Tasks    domaindelivery.Repository
	Filters  domainfilter.Repository
	Clock    clock.Clock
	Logger   *slog.Logger
	Wake     func()
}

type messageWindowRepository interface {
	ListBySourcesAndReceivedAt(ctx context.Context, sourceIDs []int64, start, end time.Time, limit int) ([]*domainmessage.NormalizedMessage, error)
}

func NewService(deps Deps) *Service {
	return &Service{
		repo: deps.Repo, settings: deps.Settings, messages: deps.Messages, sources: deps.Sources,
		sinks: deps.Sinks, tasks: deps.Tasks, filters: deps.Filters,
		clk: deps.Clock, log: deps.Logger, wake: deps.Wake,
	}
}

type ProviderInput struct {
	Name               string
	ProviderType       string
	APIType            string
	BaseURL            string
	Model              string
	TimeoutSeconds     int
	MaxRetries         int
	DefaultTemperature float64
	Enabled            bool
	IsDefault          bool
	APIKey             *string
}

func (s *Service) GetProvider(ctx context.Context) (domainaidigest.ProviderConfig, error) {
	cfg, _, err := s.effectiveProvider(ctx)
	return cfg, err
}

func (s *Service) UpdateProvider(ctx context.Context, in ProviderInput) (domainaidigest.ProviderConfig, error) {
	cfg, _, err := s.effectiveProvider(ctx)
	if err != nil {
		return domainaidigest.ProviderConfig{}, err
	}
	in.Name = firstNonEmpty(in.Name, cfg.Name)
	in.IsDefault = true
	out, err := s.UpdateProviderByID(ctx, cfg.ID, in)
	if err != nil {
		return domainaidigest.ProviderConfig{}, err
	}
	return out, nil
}

func (s *Service) TestProvider(ctx context.Context) (string, error) {
	cfg, apiKey, err := s.effectiveProvider(ctx)
	if err != nil {
		return "", err
	}
	client := s.providerClient(cfg, apiKey)
	res, err := client.Generate(ctx, ai.GenerateRequest{
		Model:       cfg.Model,
		System:      "请用一句中文回复：AI provider 连接测试成功。",
		User:        "ping",
		Temperature: cfg.DefaultTemperature,
		MaxTokens:   80,
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(res.Text), nil
}

func (s *Service) ListProviders(ctx context.Context) ([]domainaidigest.ProviderConfig, error) {
	store, secrets, err := s.loadProviderStore(ctx)
	if err != nil {
		return nil, err
	}
	return s.providersForResponse(store, secrets), nil
}

func (s *Service) CreateProvider(ctx context.Context, in ProviderInput) (domainaidigest.ProviderConfig, error) {
	store, secrets, err := s.loadProviderStore(ctx)
	if err != nil {
		return domainaidigest.ProviderConfig{}, err
	}
	cfg := providerFromInput(in)
	cfg.ID = newProviderID()
	if cfg.Name == "" {
		cfg.Name = "AI Provider"
	}
	normalizeProvider(&cfg)
	if len(store.Providers) == 0 || in.IsDefault {
		store.DefaultProviderID = cfg.ID
	}
	store.Providers = append(store.Providers, cfg)
	if in.APIKey != nil {
		secrets.APIKeys[cfg.ID] = strings.TrimSpace(*in.APIKey)
	}
	if err := s.saveProviderStore(ctx, store, secrets); err != nil {
		return domainaidigest.ProviderConfig{}, err
	}
	cfg.IsDefault = cfg.ID == store.DefaultProviderID
	cfg.HasAPIKey = secrets.APIKeys[cfg.ID] != ""
	return cfg, nil
}

func (s *Service) GetProviderByID(ctx context.Context, id string) (domainaidigest.ProviderConfig, error) {
	store, secrets, err := s.loadProviderStore(ctx)
	if err != nil {
		return domainaidigest.ProviderConfig{}, err
	}
	cfg, _, ok := findProvider(store, id)
	if !ok {
		return domainaidigest.ProviderConfig{}, fmt.Errorf("AI Provider 不存在: %s", id)
	}
	cfg.IsDefault = cfg.ID == store.DefaultProviderID
	cfg.HasAPIKey = secrets.APIKeys[cfg.ID] != ""
	return cfg, nil
}

func (s *Service) UpdateProviderByID(ctx context.Context, id string, in ProviderInput) (domainaidigest.ProviderConfig, error) {
	store, secrets, err := s.loadProviderStore(ctx)
	if err != nil {
		return domainaidigest.ProviderConfig{}, err
	}
	cfg, idx, ok := findProvider(store, id)
	if !ok {
		return domainaidigest.ProviderConfig{}, fmt.Errorf("AI Provider 不存在: %s", id)
	}
	next := providerFromInput(in)
	next.ID = cfg.ID
	if next.Name == "" {
		next.Name = cfg.Name
	}
	if next.Name == "" {
		next.Name = "AI Provider"
	}
	normalizeProvider(&next)
	store.Providers[idx] = next
	if in.IsDefault || store.DefaultProviderID == "" {
		store.DefaultProviderID = next.ID
	}
	if in.APIKey != nil {
		secrets.APIKeys[next.ID] = strings.TrimSpace(*in.APIKey)
	}
	if err := s.saveProviderStore(ctx, store, secrets); err != nil {
		return domainaidigest.ProviderConfig{}, err
	}
	next.IsDefault = next.ID == store.DefaultProviderID
	next.HasAPIKey = secrets.APIKeys[next.ID] != ""
	return next, nil
}

func (s *Service) DeleteProvider(ctx context.Context, id string) error {
	store, secrets, err := s.loadProviderStore(ctx)
	if err != nil {
		return err
	}
	_, idx, ok := findProvider(store, id)
	if !ok {
		return fmt.Errorf("AI Provider 不存在: %s", id)
	}
	store.Providers = append(store.Providers[:idx], store.Providers[idx+1:]...)
	delete(secrets.APIKeys, id)
	if store.DefaultProviderID == id {
		store.DefaultProviderID = ""
		if len(store.Providers) > 0 {
			store.DefaultProviderID = store.Providers[0].ID
		}
	}
	return s.saveProviderStore(ctx, store, secrets)
}

func (s *Service) TestProviderByID(ctx context.Context, id string) (string, error) {
	cfg, apiKey, err := s.providerByID(ctx, id)
	if err != nil {
		return "", err
	}
	return s.testProviderConfig(ctx, cfg, apiKey)
}

func (s *Service) TestProviderDraft(ctx context.Context, id string, in ProviderInput) (string, error) {
	apiKey := ""
	var base domainaidigest.ProviderConfig
	if strings.TrimSpace(id) != "" {
		cfg, savedKey, err := s.providerByID(ctx, id)
		if err != nil {
			return "", err
		}
		base = cfg
		apiKey = savedKey
	}
	cfg := providerFromInput(in)
	if base.ID != "" {
		cfg.ID = base.ID
		if cfg.Name == "" {
			cfg.Name = base.Name
		}
		if cfg.ProviderType == "" {
			cfg.ProviderType = base.ProviderType
		}
		if cfg.APIType == "" {
			cfg.APIType = base.APIType
		}
		if cfg.BaseURL == "" {
			cfg.BaseURL = base.BaseURL
		}
		if cfg.Model == "" {
			cfg.Model = base.Model
		}
		if cfg.TimeoutSeconds <= 0 {
			cfg.TimeoutSeconds = base.TimeoutSeconds
		}
		if cfg.MaxRetries < 0 {
			cfg.MaxRetries = base.MaxRetries
		}
		if cfg.DefaultTemperature == 0 {
			cfg.DefaultTemperature = base.DefaultTemperature
		}
	}
	normalizeProvider(&cfg)
	if in.APIKey != nil {
		apiKey = strings.TrimSpace(*in.APIKey)
	}
	return s.testProviderConfig(ctx, cfg, apiKey)
}

func (s *Service) testProviderConfig(ctx context.Context, cfg domainaidigest.ProviderConfig, apiKey string) (string, error) {
	client := s.providerClient(cfg, apiKey)
	res, err := client.Generate(ctx, ai.GenerateRequest{
		Model:       cfg.Model,
		System:      "请用一句中文回复：AI provider 连接测试成功。",
		User:        "ping",
		Temperature: cfg.DefaultTemperature,
		MaxTokens:   80,
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(res.Text), nil
}

func (s *Service) ListPresets(ctx context.Context) []domainaidigest.Preset {
	_ = ctx
	return defaultPresets()
}

// OutputTemplateInput 是创建/更新共享输出结构模板的输入。
type OutputTemplateInput struct {
	Name        string
	Description string
	Format      string
	Content     string
}

func (s *Service) ListOutputTemplates(ctx context.Context) ([]*domainaidigest.OutputTemplate, error) {
	return s.repo.ListOutputTemplates(ctx)
}

func (s *Service) GetOutputTemplate(ctx context.Context, id int64) (*domainaidigest.OutputTemplate, error) {
	return s.repo.GetOutputTemplate(ctx, id)
}

func (s *Service) CreateOutputTemplate(ctx context.Context, in OutputTemplateInput) (*domainaidigest.OutputTemplate, error) {
	t := outputTemplateFromInput(0, in)
	if err := validateOutputTemplate(t); err != nil {
		return nil, err
	}
	if err := s.repo.CreateOutputTemplate(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *Service) UpdateOutputTemplate(ctx context.Context, id int64, in OutputTemplateInput) (*domainaidigest.OutputTemplate, error) {
	existing, err := s.repo.GetOutputTemplate(ctx, id)
	if err != nil {
		return nil, err
	}
	t := outputTemplateFromInput(id, in)
	t.BuiltIn = existing.BuiltIn
	t.CreatedAt = existing.CreatedAt
	if err := validateOutputTemplate(t); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateOutputTemplate(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *Service) DeleteOutputTemplate(ctx context.Context, id int64) error {
	existing, err := s.repo.GetOutputTemplate(ctx, id)
	if err != nil {
		return err
	}
	if existing.BuiltIn {
		return errors.New("内置模板不可删除")
	}
	count, err := s.repo.CountProfilesUsingTemplate(ctx, id)
	if err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("该模板仍被 %d 个 Profile 引用，无法删除", count)
	}
	return s.repo.DeleteOutputTemplate(ctx, id)
}

func outputTemplateFromInput(id int64, in OutputTemplateInput) *domainaidigest.OutputTemplate {
	format := strings.TrimSpace(in.Format)
	if format == "" {
		format = "markdown"
	}
	return &domainaidigest.OutputTemplate{
		ID:          id,
		Name:        strings.TrimSpace(in.Name),
		Description: strings.TrimSpace(in.Description),
		Format:      format,
		Content:     in.Content,
	}
}

func validateOutputTemplate(t *domainaidigest.OutputTemplate) error {
	if t.Name == "" {
		return errors.New("模板名称不能为空")
	}
	if strings.TrimSpace(t.Content) == "" {
		return errors.New("模板内容不能为空")
	}
	switch t.Format {
	case "markdown", "text", "html":
	default:
		return fmt.Errorf("不支持的模板格式: %s", t.Format)
	}
	return nil
}

// resolveConditions 优先返回引用的共享过滤器条件，否则回退内联条件。
func (s *Service) resolveConditions(ctx context.Context, p *domainaidigest.Profile) []domainflow.ConditionConfig {
	if p.FilterID > 0 && s.filters != nil {
		if f, err := s.filters.GetByID(ctx, p.FilterID); err == nil && f != nil {
			return f.Conditions
		}
	}
	return p.Conditions
}

// resolveOutputTemplate 优先返回引用的共享模板内容，否则回退内联自定义，最后回退内置默认。
func (s *Service) resolveOutputTemplate(ctx context.Context, p *domainaidigest.Profile) string {
	if p.OutputTemplateID > 0 && s.repo != nil {
		if t, err := s.repo.GetOutputTemplate(ctx, p.OutputTemplateID); err == nil && t != nil {
			if strings.TrimSpace(t.Content) != "" {
				return t.Content
			}
		}
	}
	if strings.TrimSpace(p.OutputTemplate) != "" {
		return p.OutputTemplate
	}
	return defaultOutputTemplate
}

type ProfileInput struct {
	Name             string
	Enabled          bool
	SourceIDs        []int64
	FilterID         int64
	Conditions       []domainflow.ConditionConfig
	Schedule         domainaidigest.ScheduleConfig
	Window           domainaidigest.WindowConfig
	Dedupe           domainaidigest.DedupeConfig
	PromptTemplate   string
	OutputFormat     string
	OutputTemplateID int64
	OutputTemplate   string
	TargetSinkIDs    []int64
	ModelConfig      domainaidigest.ModelConfig
	Limits           domainaidigest.LimitsConfig
}

func (s *Service) CreateProfile(ctx context.Context, in ProfileInput) (*domainaidigest.Profile, error) {
	p := profileFromInput(0, in)
	if err := s.validateProfile(ctx, p, false); err != nil {
		return nil, err
	}
	if err := s.repo.CreateProfile(ctx, p); err != nil {
		return nil, err
	}
	return s.withNextRun(ctx, p), nil
}

func (s *Service) UpdateProfile(ctx context.Context, id int64, in ProfileInput) (*domainaidigest.Profile, error) {
	existing, err := s.repo.GetProfile(ctx, id)
	if err != nil {
		return nil, err
	}
	p := profileFromInput(id, in)
	p.CreatedAt = existing.CreatedAt
	if err := s.validateProfile(ctx, p, false); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateProfile(ctx, p); err != nil {
		return nil, err
	}
	return s.withNextRun(ctx, p), nil
}

func (s *Service) ListProfiles(ctx context.Context) ([]*domainaidigest.Profile, error) {
	profiles, err := s.repo.ListProfiles(ctx)
	if err != nil {
		return nil, err
	}
	for _, p := range profiles {
		normalizeProfileDefaults(p)
		s.withNextRun(ctx, p)
	}
	return profiles, nil
}

func (s *Service) GetProfile(ctx context.Context, id int64) (*domainaidigest.Profile, error) {
	p, err := s.repo.GetProfile(ctx, id)
	if err != nil {
		return nil, err
	}
	normalizeProfileDefaults(p)
	return s.withNextRun(ctx, p), nil
}

func (s *Service) DeleteProfile(ctx context.Context, id int64) error {
	return s.repo.DeleteProfile(ctx, id)
}

func (s *Service) PreviewDraft(ctx context.Context, in ProfileInput) (*domainaidigest.RunDetail, error) {
	p := profileFromInput(0, in)
	if err := s.validateProfile(ctx, p, true); err != nil {
		return nil, err
	}
	return s.execute(ctx, p, domainaidigest.TriggerPreview, false)
}

func (s *Service) PreviewProfile(ctx context.Context, id int64) (*domainaidigest.RunDetail, error) {
	p, err := s.repo.GetProfile(ctx, id)
	if err != nil {
		return nil, err
	}
	normalizeProfileDefaults(p)
	return s.execute(ctx, p, domainaidigest.TriggerPreview, false)
}

func (s *Service) RunProfile(ctx context.Context, id int64, trigger domainaidigest.TriggerType) (*domainaidigest.RunDetail, error) {
	p, err := s.repo.GetProfile(ctx, id)
	if err != nil {
		return nil, err
	}
	normalizeProfileDefaults(p)
	return s.execute(ctx, p, trigger, true)
}

func (s *Service) DeliverRun(ctx context.Context, runID int64) ([]int64, error) {
	run, err := s.repo.GetRun(ctx, runID)
	if err != nil {
		return nil, err
	}
	p, err := s.repo.GetProfile(ctx, run.ProfileID)
	if err != nil {
		return nil, err
	}
	out, err := s.repo.GetOutputByRunID(ctx, runID)
	if err != nil {
		return nil, err
	}
	taskIDs, err := s.createDeliveryTasks(ctx, p, run, out)
	if err != nil {
		return nil, err
	}
	run.DeliveryTaskIDs = append(run.DeliveryTaskIDs, taskIDs...)
	if err := s.repo.UpdateRun(ctx, run); err != nil {
		return nil, err
	}
	return taskIDs, nil
}

func (s *Service) ListRuns(ctx context.Context, profileID int64, limit, offset int) ([]*domainaidigest.Run, error) {
	return s.repo.ListRuns(ctx, profileID, limit, offset)
}

func (s *Service) GetRunDetail(ctx context.Context, id int64) (*domainaidigest.RunDetail, error) {
	run, err := s.repo.GetRun(ctx, id)
	if err != nil {
		return nil, err
	}
	items, err := s.repo.ListRunItems(ctx, id)
	if err != nil {
		return nil, err
	}
	out, err := s.repo.GetOutputByRunID(ctx, id)
	if err != nil {
		out = nil
	}
	return &domainaidigest.RunDetail{Run: run, Items: items, Output: out}, nil
}

func (s *Service) CancelRun(ctx context.Context, id int64) error {
	run, err := s.repo.GetRun(ctx, id)
	if err != nil {
		return err
	}
	if run.Status != domainaidigest.RunPending && run.Status != domainaidigest.RunRunning {
		return fmt.Errorf("run 状态 %s 不可取消", run.Status)
	}
	now := s.clk.Now()
	run.Status = domainaidigest.RunCancelled
	run.FinishedAt = &now
	return s.repo.UpdateRun(ctx, run)
}

func (s *Service) CleanupRuns(ctx context.Context, retentionDays int) (int64, error) {
	if retentionDays <= 0 {
		retentionDays = 30
	}
	return s.repo.CleanupRuns(ctx, s.clk.Now().Add(-time.Duration(retentionDays)*24*time.Hour))
}

func (s *Service) execute(ctx context.Context, p *domainaidigest.Profile, trigger domainaidigest.TriggerType, deliver bool) (*domainaidigest.RunDetail, error) {
	if p.ID > 0 && trigger != domainaidigest.TriggerPreview {
		running, err := s.repo.HasRunningRun(ctx, p.ID)
		if err != nil {
			return nil, err
		}
		if running {
			return nil, errors.New("同一 AI Profile 已有运行中的任务")
		}
	}
	start, end, err := s.resolveWindow(ctx, p)
	if err != nil {
		return nil, err
	}
	now := s.clk.Now()
	run := &domainaidigest.Run{
		ProfileID:   p.ID,
		Status:      domainaidigest.RunRunning,
		TriggerType: trigger,
		WindowStart: start,
		WindowEnd:   end,
		StartedAt:   &now,
	}
	if err := s.repo.CreateRun(ctx, run); err != nil {
		return nil, err
	}
	detail, execErr := s.runAI(ctx, p, run, deliver)
	finished := s.clk.Now()
	run.FinishedAt = &finished
	if execErr != nil {
		run.Status = domainaidigest.RunFailed
		run.Error = execErr.Error()
		_ = s.repo.UpdateRun(ctx, run)
		return &domainaidigest.RunDetail{Run: run}, execErr
	}
	run.Status = domainaidigest.RunSuccess
	if err := s.repo.UpdateRun(ctx, run); err != nil {
		return nil, err
	}
	detail.Run = run
	return detail, nil
}

func (s *Service) runAI(ctx context.Context, p *domainaidigest.Profile, run *domainaidigest.Run, deliver bool) (*domainaidigest.RunDetail, error) {
	limit := normalizedLimits(p.Limits).MaxMessagesPerRun
	msgs, err := s.messages.ListBySourcesAndReceivedAt(ctx, p.SourceIDs, run.WindowStart, run.WindowEnd, limit*3)
	if err != nil {
		return nil, err
	}
	run.InputMessageCount = len(msgs)
	items, included := s.filterMessages(ctx, p, run.ID, msgs)
	run.IncludedCount = len(included)
	run.ExcludedCount = len(items) - len(included)
	if err := s.repo.AddRunItems(ctx, items); err != nil {
		return nil, err
	}
	if len(included) == 0 {
		return &domainaidigest.RunDetail{Run: run, Items: items}, errors.New("窗口内没有符合条件的消息")
	}
	cfg, apiKey, err := s.providerForModelConfig(ctx, p.ModelConfig)
	if err != nil {
		return nil, err
	}
	userPrompt := s.buildPrompt(ctx, p, run, included)
	request, requestConfig := buildGenerateRequest(p, cfg, userPrompt)
	run.ProviderID = cfg.ID
	run.ProviderName = cfg.Name
	run.ModelName = request.Model
	run.SystemPrompt = request.System
	run.UserPrompt = request.User
	run.RequestConfig = requestConfig
	client := s.providerClient(cfg, apiKey)
	res, err := client.Generate(ctx, request)
	if err != nil {
		return nil, err
	}
	run.ModelName = firstNonEmpty(res.Model, request.Model)
	run.TokenUsage = res.Usage
	output := &domainaidigest.Output{
		RunID:       run.ID,
		Format:      p.OutputFormat,
		Title:       p.Name,
		Content:     ensureSourceNotice(res.Text),
		RawResponse: res.Raw,
	}
	if err := s.repo.UpsertOutput(ctx, output); err != nil {
		return nil, err
	}
	if deliver {
		ids, err := s.createDeliveryTasks(ctx, p, run, output)
		if err != nil {
			return nil, err
		}
		run.DeliveryTaskIDs = ids
	}
	return &domainaidigest.RunDetail{Run: run, Items: items, Output: output}, nil
}

func buildGenerateRequest(p *domainaidigest.Profile, cfg domainaidigest.ProviderConfig, userPrompt string) (ai.GenerateRequest, domainaidigest.RequestConfig) {
	request := ai.GenerateRequest{
		Model:       firstNonEmpty(p.ModelConfig.Model, cfg.Model),
		System:      systemPrompt,
		User:        userPrompt,
		Temperature: firstPositiveFloat(p.ModelConfig.Temperature, cfg.DefaultTemperature),
		MaxTokens:   p.ModelConfig.MaxTokens,
	}
	return request, domainaidigest.RequestConfig{
		ProviderID: cfg.ID, ProviderName: cfg.Name, APIType: cfg.APIType,
		Model: request.Model, Temperature: request.Temperature, MaxTokens: request.MaxTokens,
	}
}

func (s *Service) createDeliveryTasks(ctx context.Context, p *domainaidigest.Profile, run *domainaidigest.Run, out *domainaidigest.Output) ([]int64, error) {
	ids := make([]int64, 0, len(p.TargetSinkIDs))
	for _, sinkID := range p.TargetSinkIDs {
		sk, err := s.sinks.GetByID(ctx, sinkID)
		if err != nil {
			return nil, err
		}
		if !sk.Enabled {
			continue
		}
		msg := &domainmessage.NormalizedMessage{
			SourceID:       0,
			MessageType:    "text",
			Text:           out.Content,
			SenderName:     "AI Briefing",
			ReceivedAt:     s.clk.Now(),
			CreatedAt:      s.clk.Now(),
			OriginalURL:    "",
			RawPayload:     nil,
			Media:          nil,
			Links:          nil,
			SenderPeerType: "system",
		}
		task := &domaindelivery.Task{
			SinkID:          sinkID,
			OriginType:      "ai_digest",
			OriginID:        run.ID,
			Status:          domaindelivery.StatusPending,
			MaxAttempts:     3,
			MessageSnapshot: msg,
		}
		if err := s.tasks.Create(ctx, task); err != nil {
			return nil, err
		}
		ids = append(ids, task.ID)
	}
	if len(ids) > 0 && s.wake != nil {
		s.wake()
	}
	return ids, nil
}

func (s *Service) filterMessages(ctx context.Context, p *domainaidigest.Profile, runID int64, msgs []*domainmessage.NormalizedMessage) ([]*domainaidigest.RunItem, []*domainmessage.NormalizedMessage) {
	limits := normalizedLimits(p.Limits)
	conditions := s.resolveConditions(ctx, p)
	items := make([]*domainaidigest.RunItem, 0, len(msgs))
	included := make([]*domainmessage.NormalizedMessage, 0, len(msgs))
	seen := map[string]struct{}{}
	for i, msg := range msgs {
		item := &domainaidigest.RunItem{
			RunID: runID, MessageID: msg.ID, SourceID: msg.SourceID, Included: false, SortOrder: i,
			Message: msg, MessageSnapshot: domainaidigest.NewMessageSnapshot(msg),
		}
		reason := ""
		text := strings.TrimSpace(msg.Text)
		if text == "" && len(msg.Media) == 0 {
			reason = "empty_text"
		}
		if reason == "" {
			ok, err := s.matchesConditions(ctx, msg, conditions)
			if err != nil {
				reason = "condition_error: " + err.Error()
			} else if !ok {
				reason = "condition_not_match"
			}
		}
		if reason == "" && p.Dedupe.Enabled {
			key := dedupeKey(msg)
			if key != "" {
				if _, ok := seen[key]; ok {
					reason = "duplicate"
				} else {
					seen[key] = struct{}{}
				}
			}
		}
		if reason == "" && len(included) >= limits.MaxMessagesPerRun {
			reason = "limit_max_messages"
		}
		if reason == "" {
			item.Included = true
			msgCopy := *msg
			if limits.MaxCharsPerMessage > 0 && len([]rune(msgCopy.Text)) > limits.MaxCharsPerMessage {
				msgCopy.Text = truncateRunes(msgCopy.Text, limits.MaxCharsPerMessage) + "\n[已按单条消息字数上限截断]"
			}
			included = append(included, &msgCopy)
		} else {
			item.Reason = reason
		}
		items = append(items, item)
	}
	return items, included
}

func (s *Service) matchesConditions(ctx context.Context, msg *domainmessage.NormalizedMessage, configs []domainflow.ConditionConfig) (bool, error) {
	for _, cfg := range configs {
		if cfg.Type == "source" {
			continue
		}
		c, err := condition.Get(cfg.Type)
		if err != nil {
			return false, err
		}
		ok, err := c.Evaluate(ctx, msg, cfg.Config)
		if err != nil || !ok {
			return ok, err
		}
	}
	return true, nil
}

func (s *Service) buildPrompt(ctx context.Context, p *domainaidigest.Profile, run *domainaidigest.Run, msgs []*domainmessage.NormalizedMessage) string {
	limits := normalizedLimits(p.Limits)
	sourceNames := map[int64]string{}
	for _, id := range p.SourceIDs {
		if src, err := s.sources.GetByID(ctx, id); err == nil {
			sourceNames[id] = src.Name
		}
	}
	var b strings.Builder
	tmpl := strings.TrimSpace(p.PromptTemplate)
	if tmpl == "" {
		tmpl = defaultPrompt
	}
	if !strings.Contains(tmpl, "{{output_template}}") {
		tmpl += "\n\n输出结构模板：\n{{output_template}}"
	}
	outputTemplate := s.resolveOutputTemplate(ctx, p)
	sourceList := make([]string, 0, len(sourceNames))
	for _, name := range sourceNames {
		sourceList = append(sourceList, name)
	}
	sort.Strings(sourceList)
	staticReplacer := strings.NewReplacer(
		"{{profile_name}}", p.Name,
		"{{window_start}}", run.WindowStart.Format(time.RFC3339),
		"{{window_end}}", run.WindowEnd.Format(time.RFC3339),
		"{{message_count}}", fmt.Sprintf("%d", len(msgs)),
		"{{messages}}", "",
		"{{source_list}}", strings.Join(sourceList, ", "),
		"{{output_format}}", p.OutputFormat,
		"{{output_template}}", outputTemplate,
	)
	basePrompt := staticReplacer.Replace(tmpl)
	messageBudget := -1
	if limits.MaxPromptChars > 0 {
		messageBudget = limits.MaxPromptChars - len([]rune(basePrompt))
		if !strings.Contains(tmpl, "{{messages}}") {
			messageBudget -= len([]rune("\n\n输入消息：\n"))
		}
	}
	messages := buildMessagePromptBlocks(msgs, sourceNames, messageBudget)
	replacer := strings.NewReplacer(
		"{{profile_name}}", p.Name,
		"{{window_start}}", run.WindowStart.Format(time.RFC3339),
		"{{window_end}}", run.WindowEnd.Format(time.RFC3339),
		"{{message_count}}", fmt.Sprintf("%d", len(msgs)),
		"{{messages}}", messages,
		"{{source_list}}", strings.Join(sourceList, ", "),
		"{{output_format}}", p.OutputFormat,
		"{{output_template}}", outputTemplate,
	)
	b.WriteString(replacer.Replace(tmpl))
	if !strings.Contains(tmpl, "{{messages}}") {
		b.WriteString("\n\n输入消息：\n")
		b.WriteString(messages)
	}
	return b.String()
}

func buildMessagePromptBlocks(msgs []*domainmessage.NormalizedMessage, sourceNames map[int64]string, budget int) string {
	var messages strings.Builder
	used := 0
	for i, msg := range msgs {
		sourceName := sourceNames[msg.SourceID]
		if sourceName == "" {
			sourceName = fmt.Sprintf("Source #%d", msg.SourceID)
		}
		t := msg.ReceivedAt
		if msg.SentAt != nil {
			t = *msg.SentAt
		}
		block := fmt.Sprintf("#%d\nsource: %s\ntime: %s\nsender: %s\ntype: %s\nurl: %s\ntext:\n%s\n\n",
			i+1, sourceName, t.Format(time.RFC3339), emptyDash(msg.SenderName), msg.MessageType, emptyDash(msg.OriginalURL), msg.Text)
		if budget >= 0 {
			remain := budget - used
			if remain <= 0 {
				messages.WriteString("[后续消息已因消息内容上限省略]")
				break
			}
			blockRunes := []rune(block)
			if len(blockRunes) > remain {
				if remain > 0 {
					messages.WriteString(string(blockRunes[:remain]))
				}
				messages.WriteString("\n[已按消息内容上限截断，提示词未截断]")
				break
			}
			used += len(blockRunes)
		}
		messages.WriteString(block)
	}
	return messages.String()
}

func (s *Service) resolveWindow(ctx context.Context, p *domainaidigest.Profile) (time.Time, time.Time, error) {
	now := s.clk.Now()
	w := p.Window
	if w.Type == "" {
		w.Type = "last_duration"
	}
	switch w.Type {
	case "since_last_run":
		if p.ID > 0 {
			last, err := s.repo.LastSuccessfulRun(ctx, p.ID)
			if err != nil {
				return time.Time{}, time.Time{}, err
			}
			if last != nil {
				return last.WindowEnd, now, nil
			}
		}
		fallthrough
	case "last_duration":
		minutes := w.DurationMinutes
		if minutes <= 0 {
			minutes = 60
		}
		return now.Add(-time.Duration(minutes) * time.Minute), now, nil
	default:
		return time.Time{}, time.Time{}, fmt.Errorf("不支持的窗口类型: %s", w.Type)
	}
}

func (s *Service) DueProfiles(ctx context.Context) ([]*domainaidigest.Profile, error) {
	profiles, err := s.repo.ListProfiles(ctx)
	if err != nil {
		return nil, err
	}
	now := s.clk.Now()
	out := make([]*domainaidigest.Profile, 0)
	for _, p := range profiles {
		if !p.Enabled || p.Schedule.Type == "" || p.Schedule.Type == "manual" {
			continue
		}
		next, ok := s.nextRunAfter(ctx, p, now)
		if !ok || next.After(now) {
			continue
		}
		running, err := s.repo.HasRunningRun(ctx, p.ID)
		if err != nil {
			return nil, err
		}
		if !running {
			out = append(out, p)
		}
	}
	return out, nil
}

func (s *Service) withNextRun(ctx context.Context, p *domainaidigest.Profile) *domainaidigest.Profile {
	if p == nil || p.Schedule.Type == "" || p.Schedule.Type == "manual" {
		return p
	}
	if next, ok := s.nextRunAfter(ctx, p, s.clk.Now()); ok {
		p.Schedule.NextRunAt = next.Format(time.RFC3339)
	}
	return p
}

func (s *Service) nextRunAfter(ctx context.Context, p *domainaidigest.Profile, from time.Time) (time.Time, bool) {
	last, _ := s.repo.LastExecutionRun(ctx, p.ID)
	anchor := p.CreatedAt
	if last != nil {
		anchor = last.CreatedAt
	}
	if anchor.IsZero() {
		anchor = from
	}
	switch p.Schedule.Type {
	case "interval":
		minutes := p.Schedule.IntervalMinutes
		if minutes <= 0 {
			minutes = 60
		}
		next := anchor.Add(time.Duration(minutes) * time.Minute)
		return next, true
	case "daily":
		loc := time.Local
		if p.Schedule.Timezone != "" {
			if l, err := time.LoadLocation(p.Schedule.Timezone); err == nil {
				loc = l
			}
		}
		hour, minute := parseHHMM(p.Schedule.Time)
		local := anchor.In(loc)
		next := time.Date(local.Year(), local.Month(), local.Day(), hour, minute, 0, 0, loc)
		if !next.After(anchor) {
			next = next.Add(24 * time.Hour)
		}
		return next, true
	case "cron":
		loc := time.Local
		if p.Schedule.Timezone != "" {
			if l, err := time.LoadLocation(p.Schedule.Timezone); err == nil {
				loc = l
			}
		}
		expr := strings.TrimSpace(p.Schedule.Cron)
		if expr == "" {
			return time.Time{}, false
		}
		sched, err := cron.ParseStandard(expr)
		if err != nil {
			return time.Time{}, false
		}
		return sched.Next(anchor.In(loc)), true
	default:
		return time.Time{}, false
	}
}

func (s *Service) validateProfile(ctx context.Context, p *domainaidigest.Profile, preview bool) error {
	normalizeProfileDefaults(p)
	if len(p.SourceIDs) == 0 {
		return errors.New("请至少选择一个输入 Source")
	}
	if !preview && len(p.TargetSinkIDs) == 0 {
		return errors.New("请至少选择一个输出 Sink")
	}
	if p.Schedule.Type == "cron" {
		if strings.TrimSpace(p.Schedule.Cron) == "" {
			return errors.New("cron 调度需要填写表达式")
		}
		if _, err := cron.ParseStandard(strings.TrimSpace(p.Schedule.Cron)); err != nil {
			return fmt.Errorf("cron 表达式无效: %w", err)
		}
	}
	if p.Schedule.Timezone != "" {
		if _, err := time.LoadLocation(p.Schedule.Timezone); err != nil {
			return fmt.Errorf("时区无效: %w", err)
		}
	}
	if _, _, err := s.providerForModelConfig(ctx, p.ModelConfig); err != nil {
		return err
	}
	if p.OutputTemplateID > 0 {
		if _, err := s.repo.GetOutputTemplate(ctx, p.OutputTemplateID); err != nil {
			return fmt.Errorf("引用的输出模板不存在 (id=%d): %w", p.OutputTemplateID, err)
		}
	}
	if p.FilterID > 0 && s.filters != nil {
		if _, err := s.filters.GetByID(ctx, p.FilterID); err != nil {
			return fmt.Errorf("引用的过滤器不存在 (id=%d): %w", p.FilterID, err)
		}
	}
	p.Dedupe.Enabled = true
	p.Limits = normalizedLimits(p.Limits)
	for _, cfg := range p.Conditions {
		if cfg.Type == "source" {
			continue
		}
		if err := condition.ValidateConfig(cfg.Type, cfg.Config); err != nil {
			return fmt.Errorf("条件 %s 配置无效: %w", cfg.Type, err)
		}
	}
	for _, id := range p.TargetSinkIDs {
		if _, err := s.sinks.GetByID(ctx, id); err != nil {
			return fmt.Errorf("查询输出 Sink 失败 sink_id=%d: %w", id, err)
		}
	}
	return nil
}

func (s *Service) effectiveProvider(ctx context.Context) (domainaidigest.ProviderConfig, string, error) {
	store, _, err := s.loadProviderStore(ctx)
	if err != nil {
		return domainaidigest.ProviderConfig{}, "", err
	}
	return s.providerByID(ctx, store.DefaultProviderID)
}

func (s *Service) providerForModelConfig(ctx context.Context, cfg domainaidigest.ModelConfig) (domainaidigest.ProviderConfig, string, error) {
	return s.providerByID(ctx, cfg.ProviderID)
}

func (s *Service) providerByID(ctx context.Context, id string) (domainaidigest.ProviderConfig, string, error) {
	store, secrets, err := s.loadProviderStore(ctx)
	if err != nil {
		return domainaidigest.ProviderConfig{}, "", err
	}
	if strings.TrimSpace(id) == "" {
		id = store.DefaultProviderID
	}
	cfg, _, ok := findProvider(store, id)
	if !ok {
		return domainaidigest.ProviderConfig{}, "", fmt.Errorf("AI Provider 不存在: %s", id)
	}
	normalizeProvider(&cfg)
	cfg.IsDefault = cfg.ID == store.DefaultProviderID
	apiKey := secrets.APIKeys[cfg.ID]
	cfg.HasAPIKey = apiKey != ""
	return cfg, apiKey, nil
}

func (s *Service) loadProviderStore(ctx context.Context) (domainaidigest.ProviderStore, domainaidigest.ProviderSecretStore, error) {
	store := domainaidigest.ProviderStore{}
	secrets := domainaidigest.ProviderSecretStore{APIKeys: map[string]string{}}
	row, err := s.settings.Get(ctx, domainsettings.KeyAIProviders)
	if err != nil {
		return store, secrets, err
	}
	if row != nil {
		raw, _ := json.Marshal(row.Value)
		if err := json.Unmarshal(raw, &store); err != nil {
			return store, secrets, fmt.Errorf("解析 AI Provider 设置失败: %w", err)
		}
		if len(row.Secret) > 0 {
			if err := json.Unmarshal(row.Secret, &secrets); err != nil {
				return store, secrets, fmt.Errorf("解析 AI Provider secret 失败: %w", err)
			}
		}
		if secrets.APIKeys == nil {
			secrets.APIKeys = map[string]string{}
		}
		normalizeProviderStore(&store, secrets)
		if len(store.Providers) > 0 {
			return store, secrets, nil
		}
	}
	cfg, apiKey, err := s.loadLegacyProvider(ctx)
	if err != nil {
		return store, secrets, err
	}
	if cfg.ID == "" {
		cfg.ID = "default"
	}
	if cfg.Name == "" {
		cfg.Name = "默认 Provider"
	}
	cfg.Enabled = true
	cfg.IsDefault = true
	normalizeProvider(&cfg)
	store.DefaultProviderID = cfg.ID
	store.Providers = []domainaidigest.ProviderConfig{cfg}
	if apiKey != "" {
		secrets.APIKeys[cfg.ID] = apiKey
	}
	return store, secrets, nil
}

func (s *Service) saveProviderStore(ctx context.Context, store domainaidigest.ProviderStore, secrets domainaidigest.ProviderSecretStore) error {
	normalizeProviderStore(&store, secrets)
	value, err := providerStoreToMap(store)
	if err != nil {
		return err
	}
	secret, err := json.Marshal(secrets)
	if err != nil {
		return err
	}
	return s.settings.Upsert(ctx, &domainsettings.Setting{
		Key:    domainsettings.KeyAIProviders,
		Value:  value,
		Secret: secret,
	})
}

func (s *Service) loadLegacyProvider(ctx context.Context) (domainaidigest.ProviderConfig, string, error) {
	cfg := defaultProviderConfig()
	row, err := s.settings.Get(ctx, domainsettings.KeyAIProvider)
	if err != nil {
		return cfg, "", err
	}
	apiKey := ""
	if row != nil {
		raw, _ := json.Marshal(row.Value)
		_ = json.Unmarshal(raw, &cfg)
		var sec map[string]string
		if len(row.Secret) > 0 {
			_ = json.Unmarshal(row.Secret, &sec)
			apiKey = sec["api_key"]
		}
	}
	if cfg.ID == "" {
		cfg.ID = "default"
	}
	if cfg.Name == "" {
		cfg.Name = "默认 Provider"
	}
	cfg.Enabled = true
	normalizeProvider(&cfg)
	cfg.HasAPIKey = apiKey != ""
	return cfg, apiKey, nil
}

func (s *Service) providerClient(cfg domainaidigest.ProviderConfig, apiKey string) ai.Client {
	return ai.NewOpenAICompatibleClient(ai.OpenAICompatibleConfig{
		BaseURL:      cfg.BaseURL,
		APIKey:       apiKey,
		APIType:      cfg.APIType,
		DefaultModel: cfg.Model,
		Timeout:      time.Duration(cfg.TimeoutSeconds) * time.Second,
		MaxRetries:   cfg.MaxRetries,
		Temperature:  cfg.DefaultTemperature,
	})
}

func defaultProviderConfig() domainaidigest.ProviderConfig {
	return domainaidigest.ProviderConfig{
		ID:                 "default",
		Name:               "默认 Provider",
		ProviderType:       "openai_compatible",
		APIType:            "chat_completions",
		BaseURL:            "https://api.openai.com/v1",
		Model:              "gpt-4o-mini",
		TimeoutSeconds:     60,
		MaxRetries:         1,
		DefaultTemperature: 0.2,
		Enabled:            true,
	}
}

func providerFromInput(in ProviderInput) domainaidigest.ProviderConfig {
	return domainaidigest.ProviderConfig{
		Name:               strings.TrimSpace(in.Name),
		ProviderType:       strings.TrimSpace(in.ProviderType),
		APIType:            strings.TrimSpace(in.APIType),
		BaseURL:            strings.TrimRight(strings.TrimSpace(in.BaseURL), "/"),
		Model:              strings.TrimSpace(in.Model),
		TimeoutSeconds:     in.TimeoutSeconds,
		MaxRetries:         in.MaxRetries,
		DefaultTemperature: in.DefaultTemperature,
		Enabled:            true,
	}
}

func profileFromInput(id int64, in ProfileInput) *domainaidigest.Profile {
	return &domainaidigest.Profile{
		ID:               id,
		Name:             in.Name,
		Enabled:          in.Enabled,
		SourceIDs:        in.SourceIDs,
		FilterID:         in.FilterID,
		Conditions:       in.Conditions,
		Schedule:         in.Schedule,
		Window:           in.Window,
		Dedupe:           in.Dedupe,
		PromptTemplate:   in.PromptTemplate,
		OutputFormat:     in.OutputFormat,
		OutputTemplateID: in.OutputTemplateID,
		OutputTemplate:   in.OutputTemplate,
		TargetSinkIDs:    in.TargetSinkIDs,
		ModelConfig:      in.ModelConfig,
		Limits:           in.Limits,
	}
}

func normalizeProfileDefaults(p *domainaidigest.Profile) {
	if p == nil {
		return
	}
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		p.Name = "未命名 AI 整理"
	}
	if strings.TrimSpace(p.OutputFormat) == "" {
		p.OutputFormat = "markdown"
	}
	if strings.TrimSpace(p.PromptTemplate) == "" {
		p.PromptTemplate = defaultPrompt
	}
	if strings.TrimSpace(p.OutputTemplate) == "" && p.OutputTemplateID == 0 {
		p.OutputTemplate = defaultOutputTemplate
	}
}

func normalizedLimits(l domainaidigest.LimitsConfig) domainaidigest.LimitsConfig {
	if l.MaxMessagesPerRun <= 0 {
		l.MaxMessagesPerRun = 50
	}
	if l.MaxCharsPerMessage <= 0 {
		l.MaxCharsPerMessage = 1200
	}
	return l
}

func normalizeProvider(cfg *domainaidigest.ProviderConfig) {
	cfg.ID = strings.TrimSpace(cfg.ID)
	cfg.Name = strings.TrimSpace(cfg.Name)
	if cfg.ProviderType == "" {
		cfg.ProviderType = "openai_compatible"
	}
	if cfg.APIType == "" {
		cfg.APIType = "chat_completions"
	}
	if cfg.APIType != "responses" {
		cfg.APIType = "chat_completions"
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.openai.com/v1"
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	if cfg.Model == "" {
		cfg.Model = "gpt-4o-mini"
	}
	if cfg.TimeoutSeconds <= 0 {
		cfg.TimeoutSeconds = 60
	}
	if cfg.MaxRetries < 0 {
		cfg.MaxRetries = 0
	}
	if cfg.DefaultTemperature == 0 {
		cfg.DefaultTemperature = 0.2
	}
	cfg.Enabled = true
}

func normalizeProviderStore(store *domainaidigest.ProviderStore, secrets domainaidigest.ProviderSecretStore) {
	seen := map[string]struct{}{}
	out := make([]domainaidigest.ProviderConfig, 0, len(store.Providers))
	for _, cfg := range store.Providers {
		if cfg.ID == "" {
			cfg.ID = newProviderID()
		}
		if _, ok := seen[cfg.ID]; ok {
			continue
		}
		if cfg.Name == "" {
			cfg.Name = "AI Provider"
		}
		normalizeProvider(&cfg)
		cfg.HasAPIKey = false
		cfg.IsDefault = false
		seen[cfg.ID] = struct{}{}
		out = append(out, cfg)
	}
	store.Providers = out
	if len(store.Providers) == 0 {
		cfg := defaultProviderConfig()
		store.DefaultProviderID = cfg.ID
		store.Providers = []domainaidigest.ProviderConfig{cfg}
	}
	if _, _, ok := findProvider(*store, store.DefaultProviderID); !ok {
		store.DefaultProviderID = store.Providers[0].ID
	}
	if secrets.APIKeys == nil {
		secrets.APIKeys = map[string]string{}
	}
}

func (s *Service) providersForResponse(store domainaidigest.ProviderStore, secrets domainaidigest.ProviderSecretStore) []domainaidigest.ProviderConfig {
	_ = s
	out := make([]domainaidigest.ProviderConfig, 0, len(store.Providers))
	for _, cfg := range store.Providers {
		cfg.IsDefault = cfg.ID == store.DefaultProviderID
		cfg.HasAPIKey = secrets.APIKeys[cfg.ID] != ""
		out = append(out, cfg)
	}
	return out
}

func providerStoreToMap(store domainaidigest.ProviderStore) (map[string]any, error) {
	for i := range store.Providers {
		store.Providers[i].HasAPIKey = false
		store.Providers[i].IsDefault = store.Providers[i].ID == store.DefaultProviderID
	}
	raw, err := json.Marshal(store)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	return out, json.Unmarshal(raw, &out)
}

func findProvider(store domainaidigest.ProviderStore, id string) (domainaidigest.ProviderConfig, int, bool) {
	for i, cfg := range store.Providers {
		if cfg.ID == id {
			return cfg, i, true
		}
	}
	return domainaidigest.ProviderConfig{}, -1, false
}

func newProviderID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("p%d", time.Now().UnixNano())
	}
	return "p" + hex.EncodeToString(b[:])
}

func dedupeKey(msg *domainmessage.NormalizedMessage) string {
	if msg.OriginalURL != "" {
		return "url:" + msg.OriginalURL
	}
	text := strings.TrimSpace(strings.ToLower(msg.Text))
	if text == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(fmt.Sprintf("%d:%s", msg.SourceID, text)))
	return "text:" + hex.EncodeToString(sum[:])
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

func ensureSourceNotice(text string) string {
	if strings.Contains(text, "仅基于") {
		return text
	}
	return strings.TrimSpace(text) + "\n\n说明：以上内容仅基于本窗口内消息整理，未使用外部事实补全。"
}

func emptyDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func firstPositiveFloat(values ...float64) float64 {
	for _, v := range values {
		if v > 0 {
			return v
		}
	}
	return 0
}

func firstPositiveInt(values ...int) int {
	for _, v := range values {
		if v > 0 {
			return v
		}
	}
	return 0
}

func parseHHMM(v string) (int, int) {
	parts := strings.Split(v, ":")
	if len(parts) != 2 {
		return 9, 0
	}
	var h, m int
	_, _ = fmt.Sscanf(v, "%d:%d", &h, &m)
	if h < 0 || h > 23 {
		h = 9
	}
	if m < 0 || m > 59 {
		m = 0
	}
	return h, m
}
