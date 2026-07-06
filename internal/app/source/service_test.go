package source

import (
	"context"
	"errors"
	"fmt"
	"testing"

	appingest "telegram-message-forward/internal/app/ingest"
	domainaccount "telegram-message-forward/internal/domain/account"
	domainsource "telegram-message-forward/internal/domain/source"
	pluginsource "telegram-message-forward/internal/plugin/source"
)

func TestCreateEnabledSourceStartsRunner(t *testing.T) {
	svc, plugin, _ := newTestService()

	src, err := svc.Create(context.Background(), CreateInput{
		Type:    "rss",
		Name:    "Feed",
		Enabled: true,
		Config:  map[string]any{"feed_url": "https://example.com/feed.xml"},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if len(plugin.started) != 1 || plugin.started[0] != src.ID {
		t.Fatalf("enabled source 应立即启动，started=%v source=%d", plugin.started, src.ID)
	}
}

func TestCreateDisabledSourceDoesNotStartRunner(t *testing.T) {
	svc, plugin, _ := newTestService()

	if _, err := svc.Create(context.Background(), CreateInput{
		Type:    "rss",
		Name:    "Feed",
		Enabled: false,
		Config:  map[string]any{"feed_url": "https://example.com/feed.xml"},
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if len(plugin.started) != 0 {
		t.Fatalf("disabled source 不应启动，started=%v", plugin.started)
	}
}

func TestCreateEnabledSourceReturnsStartError(t *testing.T) {
	svc, plugin, _ := newTestService()
	plugin.startErr = errors.New("start failed")

	_, err := svc.Create(context.Background(), CreateInput{
		Type:    "rss",
		Name:    "Feed",
		Enabled: true,
		Config:  map[string]any{"feed_url": "https://example.com/feed.xml"},
	})
	if err == nil {
		t.Fatal("Create() error = nil, want start error")
	}
	if !errors.Is(err, plugin.startErr) {
		t.Fatalf("Create() error = %v, want wrap %v", err, plugin.startErr)
	}
}

func TestUpdateEnableReturnsStartError(t *testing.T) {
	svc, plugin, repo := newTestService()
	src := &domainsource.Source{
		Type:     "rss",
		PeerType: "feed",
		PeerID:   1,
		Name:     "Feed",
		Enabled:  false,
		Config:   map[string]any{"feed_url": "https://example.com/feed.xml"},
	}
	if err := repo.Create(context.Background(), src); err != nil {
		t.Fatalf("repo.Create() error = %v", err)
	}
	plugin.startErr = errors.New("start failed")
	enabled := true

	_, err := svc.Update(context.Background(), src.ID, UpdateInput{Enabled: &enabled})
	if err == nil {
		t.Fatal("Update() error = nil, want start error")
	}
	if !errors.Is(err, plugin.startErr) {
		t.Fatalf("Update() error = %v, want wrap %v", err, plugin.startErr)
	}
}

func newTestService() (*Service, *fakeSourcePlugin, *fakeSourceRepo) {
	repo := newFakeSourceRepo()
	plugin := &fakeSourcePlugin{}
	svc := NewService(repo, &fakeAccountRepo{}, plugin, &Manager{ingest: &appingest.Service{}})
	svc.RegisterPlugin("rss", plugin)
	return svc, plugin, repo
}

type fakeSourcePlugin struct {
	startErr error
	started  []int64
	stopped  []int64
}

func (p *fakeSourcePlugin) Name() string { return "fake" }

func (p *fakeSourcePlugin) ValidateConfig(map[string]any) error { return nil }

func (p *fakeSourcePlugin) Capabilities() pluginsource.Capabilities {
	return pluginsource.Capabilities{}
}

func (p *fakeSourcePlugin) Start(_ context.Context, _ *domainaccount.Account, src *domainsource.Source, _ pluginsource.Handler) error {
	if p.startErr != nil {
		return p.startErr
	}
	p.started = append(p.started, src.ID)
	return nil
}

func (p *fakeSourcePlugin) Stop(_ context.Context, src *domainsource.Source) error {
	p.stopped = append(p.stopped, src.ID)
	return nil
}

func (p *fakeSourcePlugin) SyncSources(context.Context, *domainaccount.Account) ([]pluginsource.SyncedPeer, error) {
	return nil, nil
}

func (p *fakeSourcePlugin) RunnerStatuses() []pluginsource.RunnerStatus {
	if len(p.started) == 0 {
		return nil
	}
	sourceIDs := append([]int64(nil), p.started...)
	return []pluginsource.RunnerStatus{{
		SourceIDs:         sourceIDs,
		Status:            "running",
		SubscriptionCount: len(sourceIDs),
	}}
}

type fakeSourceRepo struct {
	nextID int64
	items  map[int64]*domainsource.Source
}

func newFakeSourceRepo() *fakeSourceRepo {
	return &fakeSourceRepo{nextID: 1, items: map[int64]*domainsource.Source{}}
}

func (r *fakeSourceRepo) Create(_ context.Context, s *domainsource.Source) error {
	if s.ID == 0 {
		s.ID = r.nextID
		r.nextID++
	}
	r.items[s.ID] = cloneSource(s)
	return nil
}

func (r *fakeSourceRepo) Update(_ context.Context, s *domainsource.Source) error {
	if _, ok := r.items[s.ID]; !ok {
		return fmt.Errorf("source %d not found", s.ID)
	}
	r.items[s.ID] = cloneSource(s)
	return nil
}

func (r *fakeSourceRepo) GetByID(_ context.Context, id int64) (*domainsource.Source, error) {
	s, ok := r.items[id]
	if !ok {
		return nil, fmt.Errorf("source %d not found", id)
	}
	return cloneSource(s), nil
}

func (r *fakeSourceRepo) List(context.Context) ([]*domainsource.Source, error) {
	out := make([]*domainsource.Source, 0, len(r.items))
	for _, s := range r.items {
		out = append(out, cloneSource(s))
	}
	return out, nil
}

func (r *fakeSourceRepo) ListByAccount(_ context.Context, accountID int64) ([]*domainsource.Source, error) {
	out := make([]*domainsource.Source, 0)
	for _, s := range r.items {
		if s.AccountID == accountID {
			out = append(out, cloneSource(s))
		}
	}
	return out, nil
}

func (r *fakeSourceRepo) ListEnabled(context.Context) ([]*domainsource.Source, error) {
	out := make([]*domainsource.Source, 0)
	for _, s := range r.items {
		if s.Enabled {
			out = append(out, cloneSource(s))
		}
	}
	return out, nil
}

func (r *fakeSourceRepo) Delete(_ context.Context, id int64) error {
	delete(r.items, id)
	return nil
}

func cloneSource(s *domainsource.Source) *domainsource.Source {
	if s == nil {
		return nil
	}
	out := *s
	if s.Config != nil {
		out.Config = make(map[string]any, len(s.Config))
		for k, v := range s.Config {
			out.Config[k] = v
		}
	}
	return &out
}

type fakeAccountRepo struct{}

func (fakeAccountRepo) Create(context.Context, *domainaccount.Account) error { return nil }
func (fakeAccountRepo) Update(context.Context, *domainaccount.Account) error { return nil }
func (fakeAccountRepo) GetByID(context.Context, int64) (*domainaccount.Account, error) {
	return &domainaccount.Account{ID: 1, Status: domainaccount.StatusActive}, nil
}
func (fakeAccountRepo) List(context.Context) ([]*domainaccount.Account, error) { return nil, nil }
func (fakeAccountRepo) Delete(context.Context, int64) error                    { return nil }

var _ domainsource.Repository = (*fakeSourceRepo)(nil)
var _ domainaccount.Repository = (*fakeAccountRepo)(nil)
var _ pluginsource.Plugin = (*fakeSourcePlugin)(nil)
var _ pluginsource.RunnerStatusProvider = (*fakeSourcePlugin)(nil)
