package dispatch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

type progressRepository interface {
	UpdateProgress(context.Context, int64, map[string]bool) error
}

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
	result = pluginsink.ApplyErrorClassification(result, err)

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
		// 成功但发生媒体降级时，保留可读说明到 LastError，便于列表/详情发现「静默变文本」。
		if note := degradeNoteFromSummary(result.ResponseSummary); note != "" {
			task.LastError = note
			if attempt.Error == "" {
				attempt.Error = note
			}
		} else {
			task.LastError = ""
		}
		task.Progress = nil
	} else {
		attempt.Status = domaindelivery.AttemptFailed
		attempt.Error = errString(err, result)
		task.LastError = attempt.Error
		w.applyRetry(task, result)
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
		if task.MessageID <= 0 {
			return nil, errors.New("投递任务缺少 message_snapshot")
		}
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
	plugin, err := pluginsink.New(s.Type)
	if err != nil {
		return nil, err
	}
	caps := plugin.Capabilities()

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
	if !supportsFormat(caps, rendered.Format) {
		return &pluginsink.Result{
			Success: false,
			Error:   "模板格式 " + string(rendered.Format) + " 不被渠道 " + s.Type + " 支持",
		}, nil
	}

	media := w.publicMedia(ctx, msg.Media)
	media, cleanup := w.localUploadMedia(ctx, caps, media)
	defer cleanup()
	fallbackText := appendTextSuffix(mediaFallbackText(rendered.Text, media, msg.OriginalURL), task.TextSuffix)
	rendered.Text = appendTextSuffix(rendered.Text, task.TextSuffix)
	payload := pluginsink.Payload{
		Format:       string(rendered.Format),
		Text:         rendered.Text,
		Media:        media,
		FallbackText: fallbackText,
	}
	degradeReasons := mediaDegradeReasons(caps, media, w.media != nil)
	if len(media) > 0 && (!supportsAllMedia(caps, media) || exceedsMediaCount(caps, media)) {
		payload.Format = string(domaintemplate.FormatText)
		payload.Text = fallbackText
		payload.Media = nil
		if len(degradeReasons) == 0 {
			if exceedsMediaCount(caps, media) {
				degradeReasons = []string{fmt.Sprintf("渠道单条最多处理 %d 个媒体，已全部降级为文本摘要", caps.MaxMediaItems)}
			} else {
				degradeReasons = []string{"渠道不支持当前媒体形态，已降级为文本"}
			}
		}
	}
	parts := splitPayload(payload, caps)
	if task.Progress == nil {
		task.Progress = map[string]bool{}
	}
	var last *pluginsink.Result
	for i, part := range parts {
		stepPrefix := fmt.Sprintf("part:%d", i)
		if task.Progress[stepPrefix] {
			continue
		}
		opts := pluginsink.Options{
			DeliveryKey: fmt.Sprintf("delivery-%d-revision-%d-part-%d", task.ID, task.MessageRevision, i),
			StepPrefix:  stepPrefix,
			Completed:   task.Progress,
			Checkpoint: func(checkpointCtx context.Context, key string) error {
				if task.Progress == nil {
					task.Progress = map[string]bool{}
				}
				task.Progress[key] = true
				return w.updateProgress(checkpointCtx, task)
			},
		}
		result, sendErr := plugin.Send(ctx, s, part, opts)
		if sendErr != nil {
			return result, fmt.Errorf("发送第 %d/%d 段: %w", i+1, len(parts), sendErr)
		}
		if result == nil {
			return nil, fmt.Errorf("发送第 %d/%d 段未返回结果", i+1, len(parts))
		}
		if !result.Success {
			result.Error = fmt.Sprintf("第 %d/%d 段发送失败: %s", i+1, len(parts), result.Error)
			return result, nil
		}
		task.Progress[stepPrefix] = true
		if err := w.updateProgress(ctx, task); err != nil {
			return nil, fmt.Errorf("记录第 %d/%d 段投递进度: %w", i+1, len(parts), err)
		}
		last = result
		if len(parts) == 1 {
			return attachDegradeNote(result, degradeReasons), nil
		}
	}
	out := &pluginsink.Result{
		Success:         true,
		ResponseSummary: []byte(fmt.Sprintf("已分 %d 段发送", len(parts))),
	}
	if last != nil && len(last.ResponseSummary) > 0 {
		out.ResponseSummary = last.ResponseSummary
	}
	return attachDegradeNote(out, degradeReasons), nil
}

func (w *Worker) updateProgress(ctx context.Context, task *domaindelivery.Task) error {
	repo, ok := w.tasks.(progressRepository)
	if !ok {
		return nil
	}
	return repo.UpdateProgress(ctx, task.ID, task.Progress)
}

// mediaDegradeReasons 解释为何媒体会（或已经）降级为文本，供 attempt/详情展示。
func mediaDegradeReasons(c domainsink.Capabilities, media []domainmessage.Media, mediaStoreConfigured bool) []string {
	if len(media) == 0 {
		return nil
	}
	var reasons []string
	seen := map[string]struct{}{}
	add := func(s string) {
		if s == "" {
			return
		}
		if _, ok := seen[s]; ok {
			return
		}
		seen[s] = struct{}{}
		reasons = append(reasons, s)
	}
	for _, item := range media {
		if supportsMediaItem(c, item) {
			continue
		}
		// 细粒度 capability 说明。
		for _, candidate := range mediaCapabilityCandidates(c, item.Type) {
			mc, ok := findMediaCapability(c.Media, candidate.kind)
			if !ok {
				if !candidate.coarse {
					add(fmt.Sprintf("渠道未声明支持 %s，已降级为文本", mediaTypeLabel(item.Type)))
				}
				continue
			}
			if !mc.Supported {
				add(fmt.Sprintf("渠道不支持%s，已降级为文本", mediaTypeLabel(item.Type)))
				continue
			}
			if mc.MaxSizeMB > 0 && item.Size > int64(mc.MaxSizeMB)*1024*1024 {
				add(fmt.Sprintf("%s超过渠道上限 %dMB，已降级为文本", mediaTypeLabel(item.Type), mc.MaxSizeMB))
				continue
			}
			if !mc.SupportsPublicURL && !hasReadableLocalFile(item.LocalPath) && item.RemoteURL == "" {
				if item.URL == "" && !mediaStoreConfigured {
					add(fmt.Sprintf("渠道不支持直传%s且未配置媒体公网 URL，已降级为文本", mediaTypeLabel(item.Type)))
				} else if item.URL == "" {
					add(fmt.Sprintf("渠道不支持直传%s且无可用公网 URL/本地文件，已降级为文本", mediaTypeLabel(item.Type)))
				} else if !mc.SupportsPublicURL {
					// 有 URL 但渠道不认公网 URL，仍无本地文件。
					add(fmt.Sprintf("渠道仅支持二进制上传%s，本地文件不可用，已降级为文本", mediaTypeLabel(item.Type)))
				}
				continue
			}
			if mc.Fallback != "" {
				add(mc.Fallback)
			}
		}
		if !supportsMediaItem(c, item) && len(reasons) == 0 {
			add(fmt.Sprintf("%s无法按渠道能力投递，已降级为文本", mediaTypeLabel(item.Type)))
		}
	}
	return reasons
}

func mediaTypeLabel(t string) string {
	switch t {
	case "photo", "image":
		return "图片"
	case "audio", "voice":
		return "音频"
	case "video":
		return "视频"
	case "file", "document":
		return "文件"
	default:
		if t == "" {
			return "媒体"
		}
		return t
	}
}

func attachDegradeNote(result *pluginsink.Result, reasons []string) *pluginsink.Result {
	if result == nil || len(reasons) == 0 {
		return result
	}
	note := "媒体降级：" + strings.Join(reasons, "；")
	if len(result.ResponseSummary) == 0 {
		result.ResponseSummary = []byte(note)
	} else {
		result.ResponseSummary = append(result.ResponseSummary, []byte(" | "+note)...)
	}
	// 成功路径也把降级说明写入 Error 字段的可读旁路：process 成功时会清空 LastError，
	// 故额外把说明放进 ResponseSummary；若失败则拼到 Error。
	if !result.Success {
		if result.Error == "" {
			result.Error = note
		} else if !strings.Contains(result.Error, "媒体降级") {
			result.Error = result.Error + "；" + note
		}
	}
	return result
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

func exceedsMediaCount(c domainsink.Capabilities, media []domainmessage.Media) bool {
	return c.MaxMediaItems > 0 && len(media) > c.MaxMediaItems
}

// supportsMediaItem 判断渠道能否真实投递单个媒体。
// 优先使用 Capabilities.Media[] 细粒度条目（含大小上限与本地文件可用性），
// 渠道未声明对应条目时回退粗粒度布尔能力。
func supportsMediaItem(c domainsink.Capabilities, item domainmessage.Media) bool {
	for _, candidate := range mediaCapabilityCandidates(c, item.Type) {
		mc, ok := findMediaCapability(c.Media, candidate.kind)
		if !ok {
			if candidate.coarse {
				return true
			}
			continue
		}
		if mediaCapabilitySupports(mc, item) {
			return true
		}
	}
	return false
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

// localUploadMedia 为需要二进制/上传的渠道补齐可读 LocalPath。
//
// Source 入库后 LocalPath 可能为空或已因本地缓存清理而失效；只要还有 StorageKey，
// worker 就可以从 media store 重新打开内容并写入临时文件，交给 Sink 上传。
func (w *Worker) localUploadMedia(ctx context.Context, caps domainsink.Capabilities, media []domainmessage.Media) ([]domainmessage.Media, func()) {
	if w.media == nil || len(media) == 0 {
		return media, func() {}
	}
	out := append([]domainmessage.Media(nil), media...)
	var cleanupPaths []string
	for i := range out {
		if !needsLocalUpload(caps, out[i]) {
			continue
		}
		if hasReadableLocalFile(out[i].LocalPath) {
			continue
		}
		if out[i].StorageKey == "" {
			continue
		}
		path, err := w.materializeMedia(ctx, out[i])
		if err != nil {
			if w.log != nil {
				w.log.Warn("恢复媒体本地文件失败", "storage_key", out[i].StorageKey, "err", err)
			}
			continue
		}
		out[i].LocalPath = path
		cleanupPaths = append(cleanupPaths, path)
	}
	return out, func() {
		for _, path := range cleanupPaths {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				if w.log != nil {
					w.log.Warn("清理投递临时媒体失败", "path", path, "err", err)
				}
			}
		}
	}
}

func (w *Worker) materializeMedia(ctx context.Context, item domainmessage.Media) (string, error) {
	rc, err := w.media.Open(ctx, item.StorageKey)
	if err != nil {
		return "", err
	}
	defer rc.Close()

	f, err := os.CreateTemp("", "tmf-dispatch-*"+mediaTempExt(item))
	if err != nil {
		return "", err
	}
	path := f.Name()
	if _, err := io.Copy(f, rc); err != nil {
		f.Close()
		_ = os.Remove(path)
		return "", err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	return path, nil
}

func needsLocalUpload(caps domainsink.Capabilities, item domainmessage.Media) bool {
	for _, candidate := range mediaCapabilityCandidates(caps, item.Type) {
		if mc, ok := findMediaCapability(caps.Media, candidate.kind); ok {
			if mc.Supported && mc.RequiresUpload && mc.SupportsBinary {
				return true
			}
		}
	}
	return false
}

type mediaCapabilityCandidate struct {
	kind   string
	coarse bool
}

func mediaCapabilityCandidates(c domainsink.Capabilities, t string) []mediaCapabilityCandidate {
	switch t {
	case "photo", "image":
		return []mediaCapabilityCandidate{
			{kind: "image", coarse: c.SupportsImage},
			{kind: "file", coarse: c.SupportsFile},
		}
	case "file", "document":
		return []mediaCapabilityCandidate{{kind: "file", coarse: c.SupportsFile}}
	case "audio", "voice":
		return []mediaCapabilityCandidate{
			{kind: "audio", coarse: c.SupportsAudio},
			{kind: "file", coarse: c.SupportsFile},
		}
	case "video":
		return []mediaCapabilityCandidate{{kind: "video", coarse: c.SupportsVideo}}
	default:
		return nil
	}
}

func findMediaCapability(items []domainsink.MediaCapability, kind string) (domainsink.MediaCapability, bool) {
	for _, item := range items {
		if item.Type == kind {
			return item, true
		}
	}
	return domainsink.MediaCapability{}, false
}

func mediaCapabilitySupports(mc domainsink.MediaCapability, item domainmessage.Media) bool {
	if !mc.Supported {
		return false
	}
	if mc.MaxSizeMB > 0 && mediaSize(item) > int64(mc.MaxSizeMB)*1024*1024 {
		return false
	}
	hasLocal := hasReadableLocalFile(item.LocalPath)
	hasURL := item.URL != "" || item.RemoteURL != ""
	// 只认二进制/上传的渠道：无本地文件时无法真实发送（公网 URL 也不认）。
	if !mc.SupportsPublicURL && !hasLocal {
		return false
	}
	// 只认公网 URL 的渠道：既无 URL 也无本地文件时必须降级。
	if mc.SupportsPublicURL && !mc.SupportsBinary && !mc.RequiresUpload && !hasURL && !hasLocal {
		return false
	}
	// 支持公网 URL 但当前也无 URL、且无本地文件时同样无法投递媒体本体。
	if !hasLocal && !hasURL {
		return false
	}
	return true
}

func mediaSize(item domainmessage.Media) int64 {
	if item.Size > 0 {
		return item.Size
	}
	if item.LocalPath != "" {
		if info, err := os.Stat(filepath.Clean(item.LocalPath)); err == nil && !info.IsDir() {
			return info.Size()
		}
	}
	return 0
}

func hasReadableLocalFile(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func mediaTempExt(item domainmessage.Media) string {
	if ext := filepath.Ext(item.FileName); ext != "" && !strings.ContainsAny(ext, `\/`) {
		return ext
	}
	return filepath.Ext(filepath.FromSlash(item.StorageKey))
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
		if item.Caption != "" && !strings.Contains(text, item.Caption) {
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

func appendTextSuffix(text, suffix string) string {
	suffix = strings.TrimSpace(suffix)
	if suffix == "" {
		return text
	}
	text = strings.TrimRight(text, "\r\n")
	if text == "" {
		return suffix
	}
	return text + "\n" + suffix
}

func degradeNoteFromSummary(summary []byte) string {
	if len(summary) == 0 {
		return ""
	}
	s := string(summary)
	if idx := strings.Index(s, "媒体降级："); idx >= 0 {
		return strings.TrimSpace(s[idx:])
	}
	return ""
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
func (w *Worker) applyRetry(task *domaindelivery.Task, result *pluginsink.Result) {
	if result != nil && result.FailureKind == pluginsink.FailurePermanent || task.AttemptCount >= task.MaxAttempts {
		task.Status = domaindelivery.StatusDead
		task.NextRetryAt = nil
		return
	}
	task.Status = domaindelivery.StatusRetrying
	delay := backoff(task.AttemptCount)
	if result != nil && result.RetryAfter > delay {
		delay = result.RetryAfter
	}
	next := w.clock.Now().Add(delay)
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
