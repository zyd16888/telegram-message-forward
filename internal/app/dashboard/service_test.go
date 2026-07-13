package dashboard

import (
	"context"
	"testing"
	"time"

	domainaccount "telegram-message-forward/internal/domain/account"
	domainflow "telegram-message-forward/internal/domain/flow"
	domainsink "telegram-message-forward/internal/domain/sink"
	domainsource "telegram-message-forward/internal/domain/source"
)

type fakeDeliveryStats struct {
	status                 map[string]int64
	pending, processing, retrying int64
	sinks, flows, sources  []FailureBucket
}

func (f *fakeDeliveryStats) CountByStatusSince(context.Context, *time.Time) (map[string]int64, error) {
	return f.status, nil
}
func (f *fakeDeliveryStats) CountQueue(context.Context) (int64, int64, int64, error) {
	return f.pending, f.processing, f.retrying, nil
}
func (f *fakeDeliveryStats) FailureTopSince(context.Context, time.Time, int) ([]FailureBucket, []FailureBucket, []FailureBucket, error) {
	return f.sinks, f.flows, f.sources, nil
}

type fakeAccounts struct {
	items []*domainaccount.Account
}

func (f *fakeAccounts) List(context.Context) ([]*domainaccount.Account, error) { return f.items, nil }

type fakeSources struct {
	items []*domainsource.Source
}

func (f *fakeSources) List(context.Context) ([]*domainsource.Source, error) { return f.items, nil }

type fakeSinks struct {
	items []*domainsink.Sink
}

func (f *fakeSinks) List(context.Context) ([]*domainsink.Sink, error) { return f.items, nil }

type fakeFlows struct {
	items []*domainflow.Flow
}

func (f *fakeFlows) List(context.Context) ([]*domainflow.Flow, error) { return f.items, nil }

type fakeApps struct{ n int64 }

func (f *fakeApps) Count(context.Context) (int64, error) { return f.n, nil }

func TestSummaryAggregatesWindowAndSetup(t *testing.T) {
	fixed := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	svc := NewService(Deps{
		Deliveries: &fakeDeliveryStats{
			status:     map[string]int64{"success": 8, "dead": 2, "pending": 1},
			pending:    3,
			processing: 1,
			retrying:   2,
			sinks:      []FailureBucket{{Key: "1", Label: "企微", Count: 2}},
			flows:      []FailureBucket{{Key: "9", Label: "Flow A", Count: 1}},
			sources:    []FailureBucket{{Key: "3", Label: "频道", Count: 1}},
		},
		Accounts: &fakeAccounts{items: []*domainaccount.Account{
			{ID: 1, Status: domainaccount.StatusActive},
		}},
		Sources: &fakeSources{items: []*domainsource.Source{
			{ID: 1, Enabled: true},
		}},
		Sinks: &fakeSinks{items: []*domainsink.Sink{
			{ID: 1, Enabled: true},
		}},
		Flows: &fakeFlows{items: []*domainflow.Flow{
			{ID: 1, Enabled: true},
		}},
		TelegramApps: &fakeApps{n: 1},
		MediaURL:     func(context.Context) (bool, error) { return true, nil },
		Now:          func() time.Time { return fixed },
	})

	sum, err := svc.Summary(context.Background(), 24)
	if err != nil {
		t.Fatal(err)
	}
	if sum.SinceHours != 24 || sum.WindowTotal != 11 {
		t.Fatalf("window: hours=%d total=%d", sum.SinceHours, sum.WindowTotal)
	}
	if sum.Status["success"] != 8 || sum.Status["dead"] != 2 {
		t.Fatalf("status: %+v", sum.Status)
	}
	if sum.Queue.Pending != 3 || sum.Queue.Retrying != 2 {
		t.Fatalf("queue: %+v", sum.Queue)
	}
	if sum.Resources.Accounts != 1 || sum.Resources.Flows != 1 {
		t.Fatalf("resources: %+v", sum.Resources)
	}
	if len(sum.TopFailures.Sink) != 1 || sum.TopFailures.Sink[0].Label != "企微" {
		t.Fatalf("top sink: %+v", sum.TopFailures.Sink)
	}
	if !sum.Setup.HasTelegramApp || !sum.Setup.HasActiveAccount || !sum.Setup.HasEnabledFlow || !sum.Setup.HasMediaPublicURL {
		t.Fatalf("setup incomplete: %+v", sum.Setup)
	}
}

func TestSummaryDefaultSinceHours(t *testing.T) {
	svc := NewService(Deps{
		Deliveries: &fakeDeliveryStats{status: map[string]int64{}},
		Accounts:   &fakeAccounts{},
		Sources:    &fakeSources{},
		Sinks:      &fakeSinks{},
		Flows:      &fakeFlows{},
		TelegramApps: &fakeApps{},
		MediaURL:   func(context.Context) (bool, error) { return false, nil },
	})
	sum, err := svc.Summary(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if sum.SinceHours != 24 {
		t.Fatalf("default since_hours = %d", sum.SinceHours)
	}
	if sum.Setup.HasActiveAccount || sum.Setup.HasMediaPublicURL {
		t.Fatalf("empty setup should be false: %+v", sum.Setup)
	}
}
