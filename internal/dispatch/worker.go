package dispatch

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"time"

	"telegram-message-forward/internal/config"
	domaindelivery "telegram-message-forward/internal/domain/delivery"
	domainmessage "telegram-message-forward/internal/domain/message"
	domainsink "telegram-message-forward/internal/domain/sink"
	domaintemplate "telegram-message-forward/internal/domain/template"
	"telegram-message-forward/internal/infra/clock"
	"telegram-message-forward/internal/infra/mediastore"
	pluginsink "telegram-message-forward/internal/plugin/sink"
	tmpl "telegram-message-forward/internal/template"
)

var errSinkDisabled = errors.New("目标渠道已禁用，已取消投递")

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
	notifier  *Notifier
	media     mediastore.Store
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
	notifiers ...*Notifier,
) *Worker {
	var notifier *Notifier
	if len(notifiers) > 0 {
		notifier = notifiers[0]
	}
	return &Worker{
		id: id, cfg: cfg, tasks: tasks, sinks: sinks, templates: templates,
		messages: messages, renderer: renderer, clock: clk, log: log, notifier: notifier,
	}
}

// UseMediaStore 注入媒体存储：投递时为有 StorageKey 的媒体生成公网 URL。
func (w *Worker) UseMediaStore(store mediastore.Store) *Worker {
	w.media = store
	return w
}

// Run 在收到新任务唤醒信号时立即领取任务，并保留 poll interval 作为兜底扫描。
func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.cfg.PollInterval)
	defer ticker.Stop()
	wake := w.notifier.C()

	for {
		select {
		case <-ctx.Done():
			return
		case <-wake:
			if err := w.tick(ctx); err != nil {
				w.log.Error("worker tick 失败", "worker", w.id, "trigger", "notify", "err", err)
			}
		case <-ticker.C:
			if err := w.tick(ctx); err != nil {
				w.log.Error("worker tick 失败", "worker", w.id, "trigger", "poll", "err", err)
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
	if errors.Is(err, errSinkDisabled) {
		attempt.Status = domaindelivery.AttemptFailed
		attempt.Error = err.Error()
		task.Status = domaindelivery.StatusCancelled
		task.LastError = err.Error()
		task.NextRetryAt = nil
	} else if err == nil && result != nil && result.Success {
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
	if !s.Enabled {
		return nil, errSinkDisabled
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
	media := w.publicMedia(ctx, msg.Media)
	fallbackText := mediaFallbackText(rendered.Text, media, msg.OriginalURL)
	payload := pluginsink.Payload{
		Format:       string(rendered.Format),
		Text:         rendered.Text,
		Media:        media,
		FallbackText: fallbackText,
	}
	if len(msg.Media) > 0 && !supportsAllMedia(s.Capabilities, msg.Media) {
		payload.Format = string(domaintemplate.FormatText)
		payload.Text = fallbackText
	}
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

func supportsAllMedia(c domainsink.Capabilities, media []domainmessage.Media) bool {
	for _, item := range media {
		if !supportsMediaItem(c, item) {
			return false
		}
	}
	return true
}

// supportsMediaItem 判断渠道能否真实投递单个媒体。
// 优先使用 Capabilities.Media[] 细粒度条目（含大小上限与本地文件可用性），
// 渠道未声明对应条目时回退粗粒度布尔能力。
func supportsMediaItem(c domainsink.Capabilities, item domainmessage.Media) bool {
	var kind string
	var coarse bool
	switch item.Type {
	case "photo", "image":
		kind, coarse = "image", c.SupportsImage
	case "file", "document":
		kind, coarse = "file", c.SupportsFile
	case "audio", "voice":
		kind, coarse = "audio", c.SupportsAudio
	case "video":
		kind, coarse = "video", c.SupportsVideo
	default:
		return false
	}
	for _, mc := range c.Media {
		if mc.Type != kind {
			continue
		}
		if !mc.Supported {
			return false
		}
		if mc.MaxSizeMB > 0 && item.Size > int64(mc.MaxSizeMB)*1024*1024 {
			return false
		}
		// 只认二进制/上传的渠道（不支持公网 URL），媒体没有本地文件时无法真实发送，
		// 提前降级为可读文本，避免 Sink 侧静默丢弃媒体。
		if !mc.SupportsPublicURL && item.LocalPath == "" && item.RemoteURL == "" {
			return false
		}
		return true
	}
	return coarse
}

// publicMedia 为有 StorageKey 但无公网地址的媒体生成访问 URL。
// URL 在投递时生成而非落库时生成：签名/预签名 URL 有有效期，重试时需要新鲜 URL。
func (w *Worker) publicMedia(ctx context.Context, media []domainmessage.Media) []domainmessage.Media {
	if w.media == nil || len(media) == 0 {
		return media
	}
	out := append([]domainmessage.Media(nil), media...)
	for i := range out {
		if out[i].StorageKey == "" || out[i].URL != "" || out[i].RemoteURL != "" {
			continue
		}
		u, err := w.media.PublicURL(ctx, out[i].StorageKey)
		if err != nil {
			w.log.Warn("生成媒体公网 URL 失败", "storage_key", out[i].StorageKey, "err", err)
			continue
		}
		out[i].URL = u
	}
	return out
}

func mediaFallbackText(text string, media []domainmessage.Media, originalURL string) string {
	if len(media) == 0 {
		return text
	}
	out := text
	for _, item := range media {
		label := item.Type
		if label == "photo" || label == "image" {
			label = "图片"
		}
		line := "[" + label + "消息]"
		if item.Caption != "" && item.Caption != text {
			line += " " + item.Caption
		}
		if item.FileName != "" {
			line += " 文件：" + item.FileName
		}
		if item.Size > 0 {
			line += " 大小：" + humanBytes(item.Size)
		}
		if item.RemoteURL != "" {
			line += " " + item.RemoteURL
		} else if item.URL != "" {
			line += " " + item.URL
		} else if originalURL != "" {
			line += " " + originalURL
		}
		if out == "" {
			out = line
		} else {
			out += "\n" + line
		}
	}
	return out
}

func humanBytes(n int64) string {
	const mb = 1024 * 1024
	const kb = 1024
	switch {
	case n >= mb:
		return strconv.FormatFloat(float64(n)/mb, 'f', 1, 64) + " MB"
	case n >= kb:
		return strconv.FormatFloat(float64(n)/kb, 'f', 1, 64) + " KB"
	default:
		return strconv.FormatInt(n, 10) + " B"
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
