package dispatch

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"telegram-message-forward/internal/config"
	domaindelivery "telegram-message-forward/internal/domain/delivery"
	domainmessage "telegram-message-forward/internal/domain/message"
	domainsink "telegram-message-forward/internal/domain/sink"
	domaintemplate "telegram-message-forward/internal/domain/template"
	"telegram-message-forward/internal/infra/clock"
	tmpl "telegram-message-forward/internal/template"

	_ "telegram-message-forward/internal/plugin/sink/webhook"
)

type recordingTaskRepo struct {
	attempt *domaindelivery.Attempt
	task    *domaindelivery.Task
}

func (r *recordingTaskRepo) Create(context.Context, *domaindelivery.Task) error { return nil }
func (r *recordingTaskRepo) Claim(context.Context, string, int) ([]*domaindelivery.Task, error) {
	return nil, nil
}
func (r *recordingTaskRepo) UpdateStatus(_ context.Context, t *domaindelivery.Task) error {
	cp := *t
	r.task = &cp
	return nil
}
func (r *recordingTaskRepo) RecoverStale(context.Context, time.Time) (int64, error) { return 0, nil }
func (r *recordingTaskRepo) Requeue(context.Context, int64) error                   { return nil }
func (r *recordingTaskRepo) AddAttempt(_ context.Context, a *domaindelivery.Attempt) error {
	cp := *a
	r.attempt = &cp
	return nil
}
func (r *recordingTaskRepo) GetByID(context.Context, int64) (*domaindelivery.Task, error) {
	return nil, nil
}
func (r *recordingTaskRepo) List(context.Context, domaindelivery.Status, int, int) ([]*domaindelivery.Task, error) {
	return nil, nil
}

type fakeMessageRepo struct {
	msg *domainmessage.NormalizedMessage
}

func (r fakeMessageRepo) Create(context.Context, *domainmessage.NormalizedMessage) error { return nil }
func (r fakeMessageRepo) GetByID(context.Context, int64) (*domainmessage.NormalizedMessage, error) {
	return r.msg, nil
}
func (r fakeMessageRepo) ExistsByExternalID(context.Context, int64, int64) (bool, error) {
	return false, nil
}

type fakeSinkRepo struct {
	sink *domainsink.Sink
}

func (r fakeSinkRepo) Create(context.Context, *domainsink.Sink) error { return nil }
func (r fakeSinkRepo) Update(context.Context, *domainsink.Sink) error { return nil }
func (r fakeSinkRepo) GetByID(context.Context, int64) (*domainsink.Sink, error) {
	return r.sink, nil
}
func (r fakeSinkRepo) List(context.Context) ([]*domainsink.Sink, error) { return nil, nil }
func (r fakeSinkRepo) Delete(context.Context, int64) error              { return nil }

type fakeTemplateRepo struct {
	tpl *domaintemplate.Template
}

func (r fakeTemplateRepo) Create(context.Context, *domaintemplate.Template) error { return nil }
func (r fakeTemplateRepo) Update(context.Context, *domaintemplate.Template) error { return nil }
func (r fakeTemplateRepo) GetByID(context.Context, int64) (*domaintemplate.Template, error) {
	return r.tpl, nil
}
func (r fakeTemplateRepo) List(context.Context) ([]*domaintemplate.Template, error) { return nil, nil }
func (r fakeTemplateRepo) Delete(context.Context, int64) error                      { return nil }

func TestWorkerUsesMessageSnapshotForDelivery(t *testing.T) {
	var gotBody atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody.Store(string(b))
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	worker := NewWorker(
		"test-worker",
		config.DispatchConfig{},
		nil,
		fakeSinkRepo{sink: &domainsink.Sink{
			ID: 1, Type: "webhook", Enabled: true,
			Config:       map[string]any{"url": srv.URL},
			Capabilities: domainsink.Capabilities{SupportsText: true},
		}},
		fakeTemplateRepo{},
		fakeMessageRepo{msg: &domainmessage.NormalizedMessage{ID: 10, Text: "original"}},
		tmpl.NewRenderer(),
		clock.System{},
		slog.Default(),
	)

	task := &domaindelivery.Task{
		ID: 1, MessageID: 10, SinkID: 1,
		MessageSnapshot: &domainmessage.NormalizedMessage{ID: 10, Text: "processed"},
	}
	res, err := worker.deliver(context.Background(), task)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Success {
		t.Fatalf("投递应成功: %+v", res)
	}
	body, _ := gotBody.Load().(string)
	if !strings.Contains(body, "processed") || strings.Contains(body, "original") {
		t.Fatalf("应投递处理后的消息快照，实际 body=%q", body)
	}
}

func TestWorkerRejectsUnsupportedTemplateFormat(t *testing.T) {
	var called atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	tplID := int64(7)
	worker := NewWorker(
		"test-worker",
		config.DispatchConfig{},
		nil,
		fakeSinkRepo{sink: &domainsink.Sink{
			ID: 1, Type: "webhook", Enabled: true,
			Config:       map[string]any{"url": srv.URL},
			Capabilities: domainsink.Capabilities{SupportsText: true},
		}},
		fakeTemplateRepo{tpl: &domaintemplate.Template{ID: tplID, Format: domaintemplate.FormatHTML, Content: "{{.Text}}"}},
		fakeMessageRepo{msg: &domainmessage.NormalizedMessage{ID: 10, Text: "hello"}},
		tmpl.NewRenderer(),
		clock.System{},
		slog.Default(),
	)

	res, err := worker.deliver(context.Background(), &domaindelivery.Task{ID: 1, MessageID: 10, SinkID: 1, TemplateID: &tplID})
	if err != nil {
		t.Fatal(err)
	}
	if res == nil || res.Success || !strings.Contains(res.Error, "不被渠道") {
		t.Fatalf("应拒绝不支持的模板格式: %+v", res)
	}
	if called.Load() {
		t.Fatal("模板格式不支持时不应调用外部 webhook")
	}
}

func TestWorkerCancelsDisabledSinkWithoutSending(t *testing.T) {
	var called atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	tasks := &recordingTaskRepo{}
	worker := NewWorker(
		"test-worker",
		config.DispatchConfig{},
		tasks,
		fakeSinkRepo{sink: &domainsink.Sink{
			ID: 1, Type: "webhook", Enabled: false,
			Config:       map[string]any{"url": srv.URL},
			Capabilities: domainsink.Capabilities{SupportsText: true},
		}},
		fakeTemplateRepo{},
		fakeMessageRepo{msg: &domainmessage.NormalizedMessage{ID: 10, Text: "hello"}},
		tmpl.NewRenderer(),
		clock.System{},
		slog.Default(),
	)

	task := &domaindelivery.Task{ID: 1, MessageID: 10, SinkID: 1, MaxAttempts: 3}
	worker.process(context.Background(), task)

	if called.Load() {
		t.Fatal("渠道禁用时不应调用外部 webhook")
	}
	if tasks.task == nil {
		t.Fatal("应更新投递任务状态")
	}
	if tasks.task.Status != domaindelivery.StatusCancelled {
		t.Fatalf("禁用渠道任务状态 = %s, want %s", tasks.task.Status, domaindelivery.StatusCancelled)
	}
	if tasks.task.AttemptCount != 1 {
		t.Fatalf("attempt_count = %d, want 1", tasks.task.AttemptCount)
	}
	if tasks.attempt == nil || tasks.attempt.Status != domaindelivery.AttemptFailed {
		t.Fatalf("应记录一次失败尝试: %+v", tasks.attempt)
	}
	if tasks.task.NextRetryAt != nil {
		t.Fatal("禁用渠道取消任务后不应设置下次重试时间")
	}
}
