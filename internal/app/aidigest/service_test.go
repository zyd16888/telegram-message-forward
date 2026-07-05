package aidigest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

type memoryDigestRepo struct{}

func (r *memoryDigestRepo) CreateProfile(context.Context, *domainaidigest.Profile) error { return nil }
func (r *memoryDigestRepo) UpdateProfile(context.Context, *domainaidigest.Profile) error { return nil }
func (r *memoryDigestRepo) GetProfile(context.Context, int64) (*domainaidigest.Profile, error) {
	return nil, nil
}
func (r *memoryDigestRepo) ListProfiles(context.Context) ([]*domainaidigest.Profile, error) {
	return nil, nil
}
func (r *memoryDigestRepo) DeleteProfile(context.Context, int64) error { return nil }
func (r *memoryDigestRepo) CreateRun(context.Context, *domainaidigest.Run) error {
	return nil
}
func (r *memoryDigestRepo) UpdateRun(context.Context, *domainaidigest.Run) error { return nil }
func (r *memoryDigestRepo) HasRunningRun(context.Context, int64) (bool, error)   { return false, nil }
func (r *memoryDigestRepo) LastSuccessfulRun(context.Context, int64) (*domainaidigest.Run, error) {
	return nil, nil
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
