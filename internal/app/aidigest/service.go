// Package aidigest 提供 AI 整理 profile、预览、执行和投递编排。
package aidigest

import (
	"context"
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

	domainaidigest "telegram-message-forward/internal/domain/aidigest"
	domaindelivery "telegram-message-forward/internal/domain/delivery"
	domainmessage "telegram-message-forward/internal/domain/message"
	domainrule "telegram-message-forward/internal/domain/rule"
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
4. 末尾保留“说明：以上内容仅基于本窗口内消息整理，未使用外部事实补全。”`

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
		sinks: deps.Sinks, tasks: deps.Tasks, clk: deps.Clock, log: deps.Logger, wake: deps.Wake,
	}
}

type ProviderInput struct {
	ProviderType       string
	BaseURL            string
	Model              string
	TimeoutSeconds     int
	MaxRetries         int
	DefaultTemperature float64
	APIKey             *string
}

func (s *Service) GetProvider(ctx context.Context) (domainaidigest.ProviderConfig, error) {
	cfg, _, err := s.effectiveProvider(ctx)
	return cfg, err
}

func (s *Service) UpdateProvider(ctx context.Context, in ProviderInput) (domainaidigest.ProviderConfig, error) {
	cfg, currentKey, err := s.effectiveProvider(ctx)
	if err != nil {
		return domainaidigest.ProviderConfig{}, err
	}
	cfg.ProviderType = strings.TrimSpace(in.ProviderType)
	if cfg.ProviderType == "" {
		cfg.ProviderType = "openai_compatible"
	}
	cfg.BaseURL = strings.TrimRight(strings.TrimSpace(in.BaseURL), "/")
	cfg.Model = strings.TrimSpace(in.Model)
	cfg.TimeoutSeconds = in.TimeoutSeconds
	cfg.MaxRetries = in.MaxRetries
	cfg.DefaultTemperature = in.DefaultTemperature
	normalizeProvider(&cfg)
	apiKey := currentKey
	if in.APIKey != nil {
		apiKey = strings.TrimSpace(*in.APIKey)
	}
	value, err := providerToMap(cfg)
	if err != nil {
		return domainaidigest.ProviderConfig{}, err
	}
	secret, err := json.Marshal(map[string]string{"api_key": apiKey})
	if err != nil {
		return domainaidigest.ProviderConfig{}, err
	}
	if err := s.settings.Upsert(ctx, &domainsettings.Setting{
		Key:    domainsettings.KeyAIProvider,
		Value:  value,
		Secret: secret,
	}); err != nil {
		return domainaidigest.ProviderConfig{}, err
	}
	cfg.HasAPIKey = apiKey != ""
	return cfg, nil
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

type ProfileInput struct {
	Name           string
	Enabled        bool
	SourceIDs      []int64
	Conditions     []domainrule.ConditionConfig
	Schedule       domainaidigest.ScheduleConfig
	Window         domainaidigest.WindowConfig
	Dedupe         domainaidigest.DedupeConfig
	PromptTemplate string
	OutputFormat   string
	TargetSinkIDs  []int64
	ModelConfig    domainaidigest.ModelConfig
	Limits         domainaidigest.LimitsConfig
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
		s.withNextRun(ctx, p)
	}
	return profiles, nil
}

func (s *Service) GetProfile(ctx context.Context, id int64) (*domainaidigest.Profile, error) {
	p, err := s.repo.GetProfile(ctx, id)
	if err != nil {
		return nil, err
	}
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
	return s.execute(ctx, p, domainaidigest.TriggerPreview, false)
}

func (s *Service) RunProfile(ctx context.Context, id int64, trigger domainaidigest.TriggerType) (*domainaidigest.RunDetail, error) {
	p, err := s.repo.GetProfile(ctx, id)
	if err != nil {
		return nil, err
	}
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
	cfg, apiKey, err := s.effectiveProvider(ctx)
	if err != nil {
		return nil, err
	}
	userPrompt := s.buildPrompt(ctx, p, run, included)
	client := s.providerClient(cfg, apiKey)
	res, err := client.Generate(ctx, ai.GenerateRequest{
		Model:       firstNonEmpty(p.ModelConfig.Model, cfg.Model),
		System:      systemPrompt,
		User:        userPrompt,
		Temperature: firstPositiveFloat(p.ModelConfig.Temperature, cfg.DefaultTemperature),
		MaxTokens:   firstPositiveInt(p.ModelConfig.MaxTokens, 0),
	})
	if err != nil {
		return nil, err
	}
	run.ModelName = firstNonEmpty(res.Model, p.ModelConfig.Model, cfg.Model)
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
	items := make([]*domainaidigest.RunItem, 0, len(msgs))
	included := make([]*domainmessage.NormalizedMessage, 0, len(msgs))
	seen := map[string]struct{}{}
	for i, msg := range msgs {
		item := &domainaidigest.RunItem{RunID: runID, MessageID: msg.ID, SourceID: msg.SourceID, Included: false, SortOrder: i, Message: msg}
		reason := ""
		text := strings.TrimSpace(msg.Text)
		if text == "" && len(msg.Media) == 0 {
			reason = "empty_text"
		}
		if reason == "" {
			ok, err := s.matchesConditions(ctx, msg, p.Conditions)
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

func (s *Service) matchesConditions(ctx context.Context, msg *domainmessage.NormalizedMessage, configs []domainrule.ConditionConfig) (bool, error) {
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
	var messages strings.Builder
	for i, msg := range msgs {
		sourceName := sourceNames[msg.SourceID]
		if sourceName == "" {
			sourceName = fmt.Sprintf("Source #%d", msg.SourceID)
		}
		t := msg.ReceivedAt
		if msg.SentAt != nil {
			t = *msg.SentAt
		}
		fmt.Fprintf(&messages, "#%d\nsource: %s\ntime: %s\nsender: %s\ntype: %s\nurl: %s\ntext:\n%s\n\n",
			i+1, sourceName, t.Format(time.RFC3339), emptyDash(msg.SenderName), msg.MessageType, emptyDash(msg.OriginalURL), msg.Text)
	}
	sourceList := make([]string, 0, len(sourceNames))
	for _, name := range sourceNames {
		sourceList = append(sourceList, name)
	}
	sort.Strings(sourceList)
	replacer := strings.NewReplacer(
		"{{profile_name}}", p.Name,
		"{{window_start}}", run.WindowStart.Format(time.RFC3339),
		"{{window_end}}", run.WindowEnd.Format(time.RFC3339),
		"{{message_count}}", fmt.Sprintf("%d", len(msgs)),
		"{{messages}}", messages.String(),
		"{{source_list}}", strings.Join(sourceList, ", "),
		"{{output_format}}", p.OutputFormat,
	)
	b.WriteString(replacer.Replace(tmpl))
	if !strings.Contains(tmpl, "{{messages}}") {
		b.WriteString("\n\n输入消息：\n")
		b.WriteString(messages.String())
	}
	prompt := b.String()
	if limits.MaxPromptChars > 0 && len([]rune(prompt)) > limits.MaxPromptChars {
		prompt = truncateRunes(prompt, limits.MaxPromptChars) + "\n[已按 prompt 总字数上限截断]"
	}
	return prompt
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
		next, ok := s.nextRunAfter(ctx, p, now.Add(-24*time.Hour))
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
	last, _ := s.repo.ListRuns(ctx, p.ID, 1, 0)
	anchor := p.CreatedAt
	if len(last) > 0 {
		anchor = last[0].CreatedAt
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
		if next.Before(from) {
			delta := from.Sub(anchor)
			steps := int(delta/(time.Duration(minutes)*time.Minute)) + 1
			next = anchor.Add(time.Duration(steps*minutes) * time.Minute)
		}
		return next, true
	case "daily":
		loc := time.Local
		if p.Schedule.Timezone != "" {
			if l, err := time.LoadLocation(p.Schedule.Timezone); err == nil {
				loc = l
			}
		}
		hour, minute := parseHHMM(p.Schedule.Time)
		local := from.In(loc)
		next := time.Date(local.Year(), local.Month(), local.Day(), hour, minute, 0, 0, loc)
		if !next.After(from) {
			next = next.Add(24 * time.Hour)
		}
		return next, true
	default:
		return time.Time{}, false
	}
}

func (s *Service) validateProfile(ctx context.Context, p *domainaidigest.Profile, preview bool) error {
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		p.Name = "未命名 AI 整理"
	}
	if len(p.SourceIDs) == 0 {
		return errors.New("请至少选择一个输入 Source")
	}
	if !preview && len(p.TargetSinkIDs) == 0 {
		return errors.New("请至少选择一个输出 Sink")
	}
	if p.OutputFormat == "" {
		p.OutputFormat = "markdown"
	}
	if p.PromptTemplate == "" {
		p.PromptTemplate = defaultPrompt
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
	cfg := domainaidigest.ProviderConfig{
		ProviderType:       "openai_compatible",
		BaseURL:            "https://api.openai.com/v1",
		Model:              "gpt-4o-mini",
		TimeoutSeconds:     60,
		MaxRetries:         1,
		DefaultTemperature: 0.2,
	}
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
	normalizeProvider(&cfg)
	cfg.HasAPIKey = apiKey != ""
	return cfg, apiKey, nil
}

func (s *Service) providerClient(cfg domainaidigest.ProviderConfig, apiKey string) ai.Client {
	return ai.NewOpenAICompatibleClient(ai.OpenAICompatibleConfig{
		BaseURL:         cfg.BaseURL,
		APIKey:          apiKey,
		DefaultModel:    cfg.Model,
		Timeout:         time.Duration(cfg.TimeoutSeconds) * time.Second,
		MaxRetries:      cfg.MaxRetries,
		Temperature:     cfg.DefaultTemperature,
		DefaultMaxToken: 1000,
	})
}

func profileFromInput(id int64, in ProfileInput) *domainaidigest.Profile {
	return &domainaidigest.Profile{
		ID:             id,
		Name:           in.Name,
		Enabled:        in.Enabled,
		SourceIDs:      in.SourceIDs,
		Conditions:     in.Conditions,
		Schedule:       in.Schedule,
		Window:         in.Window,
		Dedupe:         in.Dedupe,
		PromptTemplate: in.PromptTemplate,
		OutputFormat:   in.OutputFormat,
		TargetSinkIDs:  in.TargetSinkIDs,
		ModelConfig:    in.ModelConfig,
		Limits:         in.Limits,
	}
}

func normalizedLimits(l domainaidigest.LimitsConfig) domainaidigest.LimitsConfig {
	if l.MaxMessagesPerRun <= 0 {
		l.MaxMessagesPerRun = 50
	}
	if l.MaxCharsPerMessage <= 0 {
		l.MaxCharsPerMessage = 1200
	}
	if l.MaxPromptChars <= 0 {
		l.MaxPromptChars = 30000
	}
	return l
}

func normalizeProvider(cfg *domainaidigest.ProviderConfig) {
	if cfg.ProviderType == "" {
		cfg.ProviderType = "openai_compatible"
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
}

func providerToMap(cfg domainaidigest.ProviderConfig) (map[string]any, error) {
	cfg.HasAPIKey = false
	raw, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	return out, json.Unmarshal(raw, &out)
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
