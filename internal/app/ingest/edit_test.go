package ingest

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"telegram-message-forward/internal/dispatch"
	domaindelivery "telegram-message-forward/internal/domain/delivery"
	domainmessage "telegram-message-forward/internal/domain/message"
	"telegram-message-forward/internal/infra/clock"
)

type editMessageRepo struct {
	result domainmessage.EditResult
}

func (r *editMessageRepo) Create(context.Context, *domainmessage.NormalizedMessage) error {
	return nil
}
func (r *editMessageRepo) ApplyEdit(_ context.Context, msg *domainmessage.NormalizedMessage) (domainmessage.EditResult, error) {
	msg.ID = 10
	if r.result.Changed {
		msg.ContentRevision = 2
	}
	return r.result, nil
}
func (r *editMessageRepo) GetByID(context.Context, int64) (*domainmessage.NormalizedMessage, error) {
	return nil, nil
}
func (r *editMessageRepo) ExistsByExternalID(context.Context, int64, int64) (bool, error) {
	return false, nil
}

type editTaskRepo struct {
	routes  []domaindelivery.FlowRoute
	created []*domaindelivery.Task
}

func (r *editTaskRepo) Create(_ context.Context, task *domaindelivery.Task) error {
	r.created = append(r.created, task)
	return nil
}
func (r *editTaskRepo) Claim(context.Context, string, int) ([]*domaindelivery.Task, error) {
	return nil, nil
}
func (r *editTaskRepo) UpdateStatus(context.Context, *domaindelivery.Task) error { return nil }
func (r *editTaskRepo) RecoverStale(context.Context, time.Time) (int64, error) {
	return 0, nil
}
func (r *editTaskRepo) Requeue(context.Context, int64) error { return nil }
func (r *editTaskRepo) AddAttempt(context.Context, *domaindelivery.Attempt) error {
	return nil
}
func (r *editTaskRepo) GetByID(context.Context, int64) (*domaindelivery.Task, error) {
	return nil, nil
}
func (r *editTaskRepo) List(context.Context, domaindelivery.Status, int, int) ([]*domaindelivery.Task, error) {
	return nil, nil
}
func (r *editTaskRepo) Count(context.Context, domaindelivery.Status) (int64, error) {
	return 0, nil
}
func (r *editTaskRepo) ListFlowRoutesByMessage(context.Context, int64) ([]domaindelivery.FlowRoute, error) {
	return r.routes, nil
}

func TestIngestEditSkipsUnchangedContent(t *testing.T) {
	messages := &editMessageRepo{result: domainmessage.EditResult{Found: true, Changed: false}}
	tasks := &editTaskRepo{routes: []domaindelivery.FlowRoute{{SinkID: 3, OriginID: 4, OriginNodeID: 5}}}
	service := NewService(messages, nil, nil, dispatch.NewQueue(tasks, 3), clock.System{}, testLogger())

	err := service.Ingest(context.Background(), &domainmessage.NormalizedMessage{
		SourceID: 1, ExternalMessageID: 2, Text: "same", EventKind: domainmessage.EventEdit,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks.created) != 0 {
		t.Fatalf("内容未变化不应创建补发任务，created=%d", len(tasks.created))
	}
}

func TestIngestEditEnqueuesNewRevision(t *testing.T) {
	messages := &editMessageRepo{result: domainmessage.EditResult{Found: true, Changed: true}}
	tasks := &editTaskRepo{routes: []domaindelivery.FlowRoute{{SinkID: 3, OriginID: 4, OriginNodeID: 5}}}
	service := NewService(messages, nil, nil, dispatch.NewQueue(tasks, 3), clock.System{}, testLogger())

	err := service.Ingest(context.Background(), &domainmessage.NormalizedMessage{
		SourceID: 1, ExternalMessageID: 2, Text: "edited", EventKind: domainmessage.EventEdit,
		EditTextSuffix: "--edited",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks.created) != 1 {
		t.Fatalf("有效编辑应创建 1 个补发任务，created=%d", len(tasks.created))
	}
	task := tasks.created[0]
	if task.MessageRevision != 2 || task.TextSuffix != "--edited" {
		t.Fatalf("补发任务 revision/后缀错误: %+v", task)
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
