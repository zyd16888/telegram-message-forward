package dispatch

import (
	"context"
	"testing"
	"time"

	domaindelivery "telegram-message-forward/internal/domain/delivery"
	domainmessage "telegram-message-forward/internal/domain/message"
	domainrule "telegram-message-forward/internal/domain/rule"
	"telegram-message-forward/internal/ruleengine"
)

type notifyTaskRepo struct {
	created int
}

func (r *notifyTaskRepo) Create(context.Context, *domaindelivery.Task) error {
	r.created++
	return nil
}
func (r *notifyTaskRepo) Claim(context.Context, string, int) ([]*domaindelivery.Task, error) {
	return nil, nil
}
func (r *notifyTaskRepo) UpdateStatus(context.Context, *domaindelivery.Task) error { return nil }
func (r *notifyTaskRepo) RecoverStale(context.Context, time.Time) (int64, error)   { return 0, nil }
func (r *notifyTaskRepo) Requeue(context.Context, int64) error                     { return nil }
func (r *notifyTaskRepo) AddAttempt(context.Context, *domaindelivery.Attempt) error {
	return nil
}
func (r *notifyTaskRepo) GetByID(context.Context, int64) (*domaindelivery.Task, error) {
	return nil, nil
}
func (r *notifyTaskRepo) List(context.Context, domaindelivery.Status, int, int) ([]*domaindelivery.Task, error) {
	return nil, nil
}

func TestQueueNotifiesAfterEnqueue(t *testing.T) {
	notifier := NewNotifier()
	repo := &notifyTaskRepo{}
	queue := NewQueue(repo, 3, notifier)

	msg := &domainmessage.NormalizedMessage{ID: 10}
	matches := []ruleengine.Match{{
		Rule:    &domainrule.Rule{ID: 20},
		Targets: []domainrule.Target{{SinkID: 30}},
		Message: msg,
	}}
	if err := queue.Enqueue(context.Background(), msg, matches); err != nil {
		t.Fatal(err)
	}
	if repo.created != 1 {
		t.Fatalf("created = %d, want 1", repo.created)
	}

	select {
	case <-notifier.C():
	case <-time.After(time.Second):
		t.Fatal("enqueue 后应唤醒 worker")
	}
}
