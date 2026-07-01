package dispatch

import (
	"context"
	"log/slog"
	"time"

	"telegram-message-forward/internal/config"
	domaindelivery "telegram-message-forward/internal/domain/delivery"
	domainmessage "telegram-message-forward/internal/domain/message"
	domainsink "telegram-message-forward/internal/domain/sink"
	domaintemplate "telegram-message-forward/internal/domain/template"
	"telegram-message-forward/internal/infra/clock"
	pluginsink "telegram-message-forward/internal/plugin/sink"
	tmpl "telegram-message-forward/internal/template"
)

// Worker 从数据库领取投递任务并执行。
type Worker struct {
	id        string
	cfg       config.DispatchConfig
	tasks     domaindelivery.Repository
	sinks     domainsink.Repository
	templates domaintemplate.Repository
	messages  domainmessage.Repository
	renderer  *tmpl.Renderer
	clock     clock.Clock
	log       *slog.Logger
}

// NewWorker 创建 worker。
func NewWorker(
	id string,
	cfg config.DispatchConfig,
	tasks domaindelivery.Repository,
	sinks domainsink.Repository,
	templates domaintemplate.Repository,
	messages domainmessage.Repository,
	renderer *tmpl.Renderer,
	clk clock.Clock,
	log *slog.Logger,
) *Worker {
	return &Worker{
		id: id, cfg: cfg, tasks: tasks, sinks: sinks, templates: templates,
		messages: messages, renderer: renderer, clock: clk, log: log,
	}
}

// Run 按 poll interval 循环领取并处理任务，直到 ctx 取消。
func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.tick(ctx); err != nil {
				w.log.Error("worker tick 失败", "worker", w.id, "err", err)
			}
		}
	}
}

// Tick 执行一次僵尸回收、领取与处理，供测试或外部一次性调度触发。
func (w *Worker) Tick(ctx context.Context) error {
	return w.tick(ctx)
}

func (w *Worker) tick(ctx context.Context) error {
	if _, err := w.tasks.RecoverStale(ctx, w.clock.Now().Add(-w.cfg.VisibilityTimeout)); err != nil {
		w.log.Warn("回收僵尸任务失败", "worker", w.id, "err", err)
	}

	tasks, err := w.tasks.Claim(ctx, w.id, 10)
	if err != nil {
		return err
	}
	for _, t := range tasks {
		w.process(ctx, t)
	}
	return nil
}

func (w *Worker) process(ctx context.Context, task *domaindelivery.Task) {
	startedAt := w.clock.Now()
	result, err := w.deliver(ctx, task)

	finishedAt := w.clock.Now()
	attempt := &domaindelivery.Attempt{
		DeliveryTaskID: task.ID,
		AttemptNo:      task.AttemptCount + 1,
		StartedAt:      &startedAt,
		FinishedAt:     &finishedAt,
	}

	task.AttemptCount++
	if err == nil && result != nil && result.Success {
		attempt.Status = domaindelivery.AttemptSuccess
		attempt.ResponseSummary = result.ResponseSummary
		task.Status = domaindelivery.StatusSuccess
		task.LastError = ""
	} else {
		attempt.Status = domaindelivery.AttemptFailed
		attempt.Error = errString(err, result)
		task.LastError = attempt.Error
		w.applyRetry(task)
	}

	if aerr := w.tasks.AddAttempt(ctx, attempt); aerr != nil {
		w.log.Error("记录投递尝试失败", "task", task.ID, "err", aerr)
	}
	if uerr := w.tasks.UpdateStatus(ctx, task); uerr != nil {
		w.log.Error("更新任务状态失败", "task", task.ID, "err", uerr)
	}
}

// deliver 渲染并投递单个任务。
func (w *Worker) deliver(ctx context.Context, task *domaindelivery.Task) (*pluginsink.Result, error) {
	msg := task.MessageSnapshot
	if msg == nil {
		var err error
		msg, err = w.messages.GetByID(ctx, task.MessageID)
		if err != nil {
			return nil, err
		}
	}
	s, err := w.sinks.GetByID(ctx, task.SinkID)
	if err != nil {
		return nil, err
	}

	var tpl *domaintemplate.Template
	if task.TemplateID != nil {
		tpl, err = w.templates.GetByID(ctx, *task.TemplateID)
		if err != nil {
			return nil, err
		}
	}

	rendered, err := w.renderer.Render(tpl, msg)
	if err != nil {
		return nil, err
	}
	if !supportsFormat(s.Capabilities, rendered.Format) {
		return &pluginsink.Result{
			Success: false,
			Error:   "模板格式 " + string(rendered.Format) + " 不被渠道 " + s.Type + " 支持",
		}, nil
	}

	plugin, err := pluginsink.New(s.Type)
	if err != nil {
		return nil, err
	}
	payload := pluginsink.Payload{Format: string(rendered.Format), Text: rendered.Text}
	return plugin.Send(ctx, s, payload, pluginsink.Options{})
}

func supportsFormat(c domainsink.Capabilities, f domaintemplate.Format) bool {
	switch f {
	case domaintemplate.FormatText:
		return c.SupportsText
	case domaintemplate.FormatMarkdown:
		return c.SupportsMarkdown
	case domaintemplate.FormatHTML:
		return c.SupportsHTML
	default:
		return false
	}
}

// applyRetry 根据剩余次数决定进入 retrying 还是 dead。
func (w *Worker) applyRetry(task *domaindelivery.Task) {
	if task.AttemptCount >= task.MaxAttempts {
		task.Status = domaindelivery.StatusDead
		task.NextRetryAt = nil
		return
	}
	task.Status = domaindelivery.StatusRetrying
	next := w.clock.Now().Add(backoff(task.AttemptCount))
	task.NextRetryAt = &next
}

func errString(err error, result *pluginsink.Result) string {
	if err != nil {
		return err.Error()
	}
	if result != nil && result.Error != "" {
		return result.Error
	}
	return "投递失败"
}
