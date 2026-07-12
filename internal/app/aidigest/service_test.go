package aidigest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	domainaidigest "telegram-message-forward/internal/domain/aidigest"
	domainsettings "telegram-message-forward/internal/domain/settings"
)

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
}

func (r *memoryDigestRepo) CreateProfile(context.Context, *domainaidigest.Profile) error { return nil }
func (r *memoryDigestRepo) UpdateProfile(context.Context, *domainaidigest.Profile) error { return nil }
func (r *memoryDigestRepo) GetProfile(context.Context, int64) (*domainaidigest.Profile, error) {
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
func (r *memoryDigestRepo) GetRun(context.Context, int64) (*domainaidigest.Run, error) {
	return nil, nil
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
func (r *memoryDigestRepo) CleanupRuns(context.Context, time.Time) (int64, error) { return 0, nil }

func strPtr(v string) *string { return &v }

var _ domainaidigest.Repository = (*memoryDigestRepo)(nil)
var _ domainsettings.Repository = (*memorySettingsRepo)(nil)
