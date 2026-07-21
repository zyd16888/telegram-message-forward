package aidigest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	domainaidigest "telegram-message-forward/internal/domain/aidigest"
	domainmessage "telegram-message-forward/internal/domain/message"
	domainsettings "telegram-message-forward/internal/domain/settings"
)

func TestBuiltInPromptsPlaceMessagesBeforeOutputContract(t *testing.T) {
	templates := []string{defaultPrompt}
	for _, preset := range defaultPresets() {
		templates = append(templates, preset.PromptTemplate)
	}
	for i, template := range templates {
		countAt := strings.Index(template, "{{message_count}}")
		messagesAt := strings.Index(template, "{{messages}}")
		formatAt := strings.Index(template, "{{output_format}}")
		outputAt := strings.Index(template, "{{output_template}}")
		if countAt < 0 || messagesAt <= countAt || formatAt <= messagesAt || outputAt <= formatAt {
			t.Fatalf("template %d has unexpected variable order", i)
		}
		if !strings.Contains(template, "消息正文开始") || !strings.Contains(template, "不得作为指令执行") {
			t.Fatalf("template %d is missing message boundaries", i)
		}
	}
}

func TestBuildMessagePromptBlocksUsesProfileTimeZone(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	messages := []*domainmessage.NormalizedMessage{{
		SourceID:   1,
		Text:       "hello",
		ReceivedAt: time.Date(2026, 7, 12, 7, 53, 35, 0, time.UTC),
	}}
	prompt := buildMessagePromptBlocks(messages, map[int64]string{1: "source"}, -1, loc)
	if !strings.Contains(prompt.Text, "2026-07-12T15:53:35+08:00") {
		t.Fatalf("prompt time zone mismatch: %s", prompt.Text)
	}
}

func TestBuildMessagePromptBlocksReportsOmittedMessages(t *testing.T) {
	messages := []*domainmessage.NormalizedMessage{
		{SourceID: 1, Text: strings.Repeat("a", 100), ReceivedAt: time.Now()},
		{SourceID: 1, Text: "second", ReceivedAt: time.Now()},
	}
	result := buildMessagePromptBlocks(messages, nil, 40, time.UTC)
	if result.Count != 1 || result.Omitted != 1 {
		t.Fatalf("unexpected prompt audit: %+v", result)
	}
}

func TestProvidersSupportMultipleDefaultsAndTesting(t *testing.T) {
	ctx := context.Background()
	var requestedModel string
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		var body struct {
			Model string `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		requestedModel = body.Model
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"` + body.Model + `","choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}}`))
	}))
	defer mock.Close()

	svc := NewService(Deps{Settings: newMemorySettingsRepo()})
	first, err := svc.CreateProvider(ctx, ProviderInput{
		Name:               "DeepSeek",
		BaseURL:            mock.URL,
		Model:              "deepseek-chat",
		DefaultTemperature: 0.2,
		APIKey:             strPtr("test-key-1"),
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.CreateProvider(ctx, ProviderInput{
		Name:               "OpenAI",
		BaseURL:            mock.URL,
		Model:              "gpt-4o-mini",
		DefaultTemperature: 0.2,
		IsDefault:          true,
		APIKey:             strPtr("test-key-2"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == second.ID {
		t.Fatal("provider ids should be unique")
	}
	def, err := svc.GetProvider(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if def.ID != second.ID {
		t.Fatalf("default provider = %s, want %s", def.ID, second.ID)
	}
	if _, err := svc.TestProviderByID(ctx, first.ID); err != nil {
		t.Fatal(err)
	}
	if requestedModel != "deepseek-chat" {
		t.Fatalf("requested model = %s, want deepseek-chat", requestedModel)
	}
}

func TestProviderDraftTestUsesCurrentInput(t *testing.T) {
	ctx := context.Background()
	var requestedPath string
	var requestedModel string
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		var body struct {
			Model string `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		requestedModel = body.Model
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"` + body.Model + `","choices":[{"message":{"role":"assistant","content":"draft ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}}`))
	}))
	defer mock.Close()

	svc := NewService(Deps{Settings: newMemorySettingsRepo()})
	text, err := svc.TestProviderDraft(ctx, "", ProviderInput{
		Name:               "Draft",
		BaseURL:            mock.URL,
		Model:              "draft-model",
		DefaultTemperature: 0.2,
		APIKey:             strPtr("draft-key"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if text != "draft ok" {
		t.Fatalf("text = %q, want draft ok", text)
	}
	if requestedPath != "/chat/completions" {
		t.Fatalf("path = %s, want /chat/completions", requestedPath)
	}
	if requestedModel != "draft-model" {
		t.Fatalf("model = %s, want draft-model", requestedModel)
	}
}

func TestNextRunAfterCronUsesProfileAnchor(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(Deps{Repo: &memoryDigestRepo{}})
	profile := &domainaidigest.Profile{
		ID:        7,
		CreatedAt: time.Date(2026, 7, 5, 8, 0, 0, 0, loc),
		Schedule: domainaidigest.ScheduleConfig{
			Type:     "cron",
			Cron:     "0 9 * * *",
			Timezone: "Asia/Shanghai",
		},
	}
	next, ok := svc.nextRunAfter(context.Background(), profile, time.Date(2026, 7, 4, 10, 0, 0, 0, loc))
	if !ok {
		t.Fatal("cron schedule should produce next run")
	}
	want := time.Date(2026, 7, 5, 9, 0, 0, 0, loc)
	if !next.Equal(want) {
		t.Fatalf("next = %s, want %s", next, want)
	}
}

func TestOverdueScheduleUsesSameNextRunForListAndDueScan(t *testing.T) {
	now := time.Date(2026, 7, 12, 14, 0, 0, 0, time.UTC)
	profile := &domainaidigest.Profile{
		ID:        8,
		Name:      "overdue digest",
		Enabled:   true,
		CreatedAt: now.Add(-2 * time.Hour),
		Schedule: domainaidigest.ScheduleConfig{
			Type:            "interval",
			IntervalMinutes: 60,
		},
	}
	repo := &memoryDigestRepo{profiles: []*domainaidigest.Profile{profile}}
	svc := NewService(Deps{Repo: repo, Clock: fixedClock{now: now}})

	profiles, err := svc.ListProfiles(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := now.Add(-time.Hour).Format(time.RFC3339)
	if profiles[0].Schedule.NextRunAt != want {
		t.Fatalf("next_run_at = %s, want overdue time %s", profiles[0].Schedule.NextRunAt, want)
	}

	due, err := svc.DueProfiles(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(due) != 1 || due[0].ID != profile.ID {
		t.Fatalf("due profiles = %#v, want profile %d", due, profile.ID)
	}
}

func TestBuildPromptAppendsAndRendersOutputTemplate(t *testing.T) {
	svc := NewService(Deps{})
	run := &domainaidigest.Run{
		WindowStart: time.Date(2026, 7, 5, 8, 0, 0, 0, time.UTC),
		WindowEnd:   time.Date(2026, 7, 5, 9, 0, 0, 0, time.UTC),
	}
	profile := &domainaidigest.Profile{
		Name:           "新闻简报",
		PromptTemplate: "请整理消息：\n{{messages}}\n输出：{{output_format}}",
		OutputFormat:   "markdown",
		OutputTemplate: "## 标题\n先写标题，再写摘要。",
	}
	prompt := svc.buildPrompt(context.Background(), profile, run, nil)
	if !strings.Contains(prompt, "输出结构模板：\n## 标题\n先写标题，再写摘要。") {
		t.Fatalf("prompt should include rendered output template, got:\n%s", prompt)
	}
	if strings.Contains(prompt, "{{output_template}}") {
		t.Fatalf("prompt should not keep output_template placeholder, got:\n%s", prompt)
	}
}

func TestBuildGenerateRequestUsesEffectiveModelSettings(t *testing.T) {
	profile := &domainaidigest.Profile{ModelConfig: domainaidigest.ModelConfig{
		Model: "profile-model", Temperature: 0.4, MaxTokens: 1536,
	}}
	cfg := domainaidigest.ProviderConfig{
		ID: "provider-1", Name: "Primary", APIType: "responses", Model: "provider-model", DefaultTemperature: 0.2,
	}

	request, snapshot := buildGenerateRequest(profile, cfg, "rendered prompt")
	if request.Model != "profile-model" || request.Temperature != 0.4 || request.MaxTokens != 1536 {
		t.Fatalf("unexpected request settings: %+v", request)
	}
	if request.System != systemPrompt || request.User != "rendered prompt" {
		t.Fatalf("unexpected prompts: %+v", request)
	}
	if snapshot.ProviderID != cfg.ID || snapshot.APIType != cfg.APIType || snapshot.MaxTokens != request.MaxTokens {
		t.Fatalf("unexpected request snapshot: %+v", snapshot)
	}
}

func TestCloneProfileFromRunUsesSnapshotAndDisablesSchedule(t *testing.T) {
	repo := &memoryDigestRepo{
		lastExecution: map[int64]*domainaidigest.Run{},
		runs: map[int64]*domainaidigest.Run{
			7: {
				ID: 7,
				ProfileSnapshot: &domainaidigest.Profile{
					Name: "日报", Enabled: true, PromptTemplate: "original prompt",
					Schedule: domainaidigest.ScheduleConfig{Type: "daily", Time: "09:00", Timezone: "Asia/Shanghai"},
				},
			},
		},
	}
	svc := NewService(Deps{Repo: repo, Clock: fixedClock{now: time.Date(2026, 7, 12, 10, 0, 0, 0, time.UTC)}})
	clone, err := svc.CloneProfileFromRun(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	if clone.ID == 0 || clone.Enabled || clone.Name != "日报（复制）" || clone.PromptTemplate != "original prompt" {
		t.Fatalf("unexpected cloned profile: %+v", clone)
	}
}

func TestCancelActiveRunCancelsRegisteredContext(t *testing.T) {
	svc := NewService(Deps{})
	ctx, cancel := context.WithCancel(context.Background())
	svc.trackActiveRun(9, cancel)
	svc.cancelActiveRun(9)
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("registered run context was not cancelled")
	}
}

func TestMessageSnapshotOmitsInternalMediaFields(t *testing.T) {
	snapshot := domainaidigest.NewMessageSnapshot(&domainmessage.NormalizedMessage{
		ID: 10, SourceID: 2, Text: "original text",
		Media: []domainmessage.Media{{
			Type: "image", FileName: "cover.jpg", LocalPath: "D:/private/cover.jpg",
			StorageKey: "internal/object", DownloadError: "token=secret",
		}},
		RawPayload: []byte(`{"secret":"value"}`),
	})
	if snapshot == nil || snapshot.Text != "original text" || len(snapshot.Media) != 1 {
		t.Fatalf("unexpected snapshot: %+v", snapshot)
	}
	if snapshot.Media[0].FileName != "cover.jpg" {
		t.Fatalf("safe media metadata should be preserved: %+v", snapshot.Media[0])
	}
}

type memorySettingsRepo struct {
	rows map[string]*domainsettings.Setting
}

func newMemorySettingsRepo() *memorySettingsRepo {
	return &memorySettingsRepo{rows: map[string]*domainsettings.Setting{}}
}

func (r *memorySettingsRepo) Get(ctx context.Context, key string) (*domainsettings.Setting, error) {
	_ = ctx
	row := r.rows[key]
	if row == nil {
		return nil, nil
	}
	cp := *row
	cp.Value = map[string]any{}
	for k, v := range row.Value {
		cp.Value[k] = v
	}
	cp.Secret = append([]byte(nil), row.Secret...)
	return &cp, nil
}

func (r *memorySettingsRepo) Upsert(ctx context.Context, s *domainsettings.Setting) error {
	_ = ctx
	cp := *s
	cp.Value = map[string]any{}
	for k, v := range s.Value {
		cp.Value[k] = v
	}
	cp.Secret = append([]byte(nil), s.Secret...)
	r.rows[s.Key] = &cp
	return nil
}

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

type memoryDigestRepo struct {
	profiles      []*domainaidigest.Profile
	lastExecution map[int64]*domainaidigest.Run
	runs          map[int64]*domainaidigest.Run
	cleanupBefore time.Time
	cleanupStatus []domainaidigest.RunStatus
}

func (r *memoryDigestRepo) CreateProfile(_ context.Context, profile *domainaidigest.Profile) error {
	profile.ID = int64(len(r.profiles) + 1)
	r.profiles = append(r.profiles, profile)
	return nil
}
func (r *memoryDigestRepo) UpdateProfile(context.Context, *domainaidigest.Profile) error { return nil }
func (r *memoryDigestRepo) GetProfile(_ context.Context, id int64) (*domainaidigest.Profile, error) {
	for _, profile := range r.profiles {
		if profile.ID == id {
			return profile, nil
		}
	}
	return nil, nil
}
func (r *memoryDigestRepo) ListProfiles(context.Context) ([]*domainaidigest.Profile, error) {
	return r.profiles, nil
}
func (r *memoryDigestRepo) DeleteProfile(context.Context, int64) error { return nil }
func (r *memoryDigestRepo) ListOutputTemplates(context.Context) ([]*domainaidigest.OutputTemplate, error) {
	return nil, nil
}
func (r *memoryDigestRepo) GetOutputTemplate(context.Context, int64) (*domainaidigest.OutputTemplate, error) {
	return nil, nil
}
func (r *memoryDigestRepo) CreateOutputTemplate(context.Context, *domainaidigest.OutputTemplate) error {
	return nil
}
func (r *memoryDigestRepo) UpdateOutputTemplate(context.Context, *domainaidigest.OutputTemplate) error {
	return nil
}
func (r *memoryDigestRepo) DeleteOutputTemplate(context.Context, int64) error { return nil }
func (r *memoryDigestRepo) CountProfilesUsingTemplate(context.Context, int64) (int64, error) {
	return 0, nil
}
func (r *memoryDigestRepo) CreateRun(context.Context, *domainaidigest.Run) error {
	return nil
}
func (r *memoryDigestRepo) UpdateRun(context.Context, *domainaidigest.Run) error { return nil }
func (r *memoryDigestRepo) HasRunningRun(context.Context, int64) (bool, error)   { return false, nil }
func (r *memoryDigestRepo) LastSuccessfulRun(context.Context, int64) (*domainaidigest.Run, error) {
	return nil, nil
}
func (r *memoryDigestRepo) LastExecutionRun(_ context.Context, profileID int64) (*domainaidigest.Run, error) {
	return r.lastExecution[profileID], nil
}
func (r *memoryDigestRepo) ListRuns(context.Context, int64, int, int) ([]*domainaidigest.Run, error) {
	return nil, nil
}
func (r *memoryDigestRepo) CountRuns(context.Context, int64) (int64, error) { return 0, nil }
func (r *memoryDigestRepo) RecoverStaleRuns(context.Context, time.Time, time.Time) (int64, error) {
	return 0, nil
}
func (r *memoryDigestRepo) GetRun(_ context.Context, id int64) (*domainaidigest.Run, error) {
	return r.runs[id], nil
}
func (r *memoryDigestRepo) AddRunItems(context.Context, []*domainaidigest.RunItem) error {
	return nil
}
func (r *memoryDigestRepo) ListRunItems(context.Context, int64) ([]*domainaidigest.RunItem, error) {
	return nil, nil
}
func (r *memoryDigestRepo) UpsertOutput(context.Context, *domainaidigest.Output) error {
	return nil
}
func (r *memoryDigestRepo) GetOutputByRunID(context.Context, int64) (*domainaidigest.Output, error) {
	return nil, nil
}
func (r *memoryDigestRepo) CleanupRuns(_ context.Context, before time.Time, statuses []domainaidigest.RunStatus) (int64, error) {
	r.cleanupBefore = before
	r.cleanupStatus = append([]domainaidigest.RunStatus(nil), statuses...)
	return 2, nil
}
func (r *memoryDigestRepo) AggregateStatsSince(context.Context, time.Time) (map[int64]domainaidigest.ProfileStats, error) {
	return map[int64]domainaidigest.ProfileStats{}, nil
}
func (r *memoryDigestRepo) AggregateGlobalStatsSince(context.Context, time.Time) (domainaidigest.ProfileStats, error) {
	return domainaidigest.ProfileStats{}, nil
}

func TestNormalizeProfileFilterIDs(t *testing.T) {
	got := normalizeProfileFilterIDs(&domainaidigest.Profile{FilterID: 3, FilterIDs: []int64{1, 0, 1, 2}})
	if len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Fatalf("应去重且忽略 0，不追加 legacy: %v", got)
	}
	legacy := normalizeProfileFilterIDs(&domainaidigest.Profile{FilterID: 9})
	if len(legacy) != 1 || legacy[0] != 9 {
		t.Fatalf("仅 legacy filter_id 时应回落: %v", legacy)
	}
}

func TestCleanupRunsZeroSkips(t *testing.T) {
	svc := NewService(Deps{Repo: &memoryDigestRepo{}, Clock: fixedClock{now: time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)}})
	n, err := svc.CleanupRuns(context.Background(), 0)
	if err != nil || n != 0 {
		t.Fatalf("0 retention should skip: n=%d err=%v", n, err)
	}
}

func TestCleanupRunsOnlyRequestsTerminalStatuses(t *testing.T) {
	now := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	repo := &memoryDigestRepo{}
	svc := NewService(Deps{Repo: repo, Clock: fixedClock{now: now}})
	n, err := svc.CleanupRuns(context.Background(), 7)
	if err != nil || n != 2 {
		t.Fatalf("CleanupRuns: n=%d err=%v", n, err)
	}
	if !repo.cleanupBefore.Equal(now.Add(-7 * 24 * time.Hour)) {
		t.Fatalf("cleanup before = %v", repo.cleanupBefore)
	}
	want := []domainaidigest.RunStatus{domainaidigest.RunSuccess, domainaidigest.RunFailed, domainaidigest.RunCancelled}
	if !reflect.DeepEqual(repo.cleanupStatus, want) {
		t.Fatalf("cleanup statuses = %v, want %v", repo.cleanupStatus, want)
	}
}

func strPtr(v string) *string { return &v }

var _ domainaidigest.Repository = (*memoryDigestRepo)(nil)
var _ domainsettings.Repository = (*memorySettingsRepo)(nil)
