package dispatch

import (
	"context"
	"testing"
	"time"

	domaindelivery "telegram-message-forward/internal/domain/delivery"
	domainmessage "telegram-message-forward/internal/domain/message"
	domainrule "telegram-message-forward/internal/domain/rule"
	domainsink "telegram-message-forward/internal/domain/sink"
	"telegram-message-forward/internal/ruleengine"
)

type notifyTaskRepo struct {
	created int
	tasks   []*domaindelivery.Task
}

func (r *notifyTaskRepo) Create(_ context.Context, t *domaindelivery.Task) error {
	r.created++
	r.tasks = append(r.tasks, t)
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
func (r *notifyTaskRepo) Count(context.Context, domaindelivery.Status) (int64, error) {
	return int64(r.created), nil
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

func TestQueueFlowMatchLeavesRuleIDEmpty(t *testing.T) {
	repo := &notifyTaskRepo{}
	queue := NewQueue(repo, 3)

	msg := &domainmessage.NormalizedMessage{ID: 10}
	matches := []ruleengine.Match{{
		Rule:         &domainrule.Rule{ID: 7}, // flowengine 伪造的 Rule，ID 实为 flow.ID
		Targets:      []domainrule.Target{{SinkID: 30}},
		Message:      msg,
		OriginType:   "flow",
		OriginID:     7,
		OriginNodeID: 42,
	}}
	if err := queue.Enqueue(context.Background(), msg, matches); err != nil {
		t.Fatal(err)
	}
	if len(repo.tasks) != 1 {
		t.Fatalf("created = %d, want 1", len(repo.tasks))
	}
	task := repo.tasks[0]
	if task.RuleID != 0 {
		t.Fatalf("flow 任务不应写 rule_id，got %d", task.RuleID)
	}
	if task.OriginType != "flow" || task.OriginID != 7 || task.OriginNodeID != 42 {
		t.Fatalf("origin 字段错误: %+v", task)
	}
}

func TestQueueSkipsDisabledSink(t *testing.T) {
	notifier := NewNotifier()
	repo := &notifyTaskRepo{}
	queue := NewQueue(repo, 3, notifier).UseSinks(fakeSinkRepo{sink: &domainsink.Sink{ID: 30, Enabled: false}})

	msg := &domainmessage.NormalizedMessage{ID: 10}
	matches := []ruleengine.Match{{
		Rule:    &domainrule.Rule{ID: 20},
		Targets: []domainrule.Target{{SinkID: 30}},
		Message: msg,
	}}
	if err := queue.Enqueue(context.Background(), msg, matches); err != nil {
		t.Fatal(err)
	}
	if repo.created != 0 {
		t.Fatalf("禁用渠道不应创建投递任务，created = %d", repo.created)
	}

	select {
	case <-notifier.C():
		t.Fatal("没有创建任务时不应唤醒 worker")
	default:
	}
}
