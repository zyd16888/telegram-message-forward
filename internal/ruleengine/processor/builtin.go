package processor

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"telegram-message-forward/internal/domain/formschema"
	domainmessage "telegram-message-forward/internal/domain/message"
)

func init() {
	RegisterWithDescriptor("append_source", appendSource{}, Descriptor{
		Type:        "append_source",
		Label:       "追加来源",
		Description: "在消息前后追加一段来源说明或固定文本。",
		Fields: []formschema.FieldSpec{
			{Key: "label", Label: "来源名称", Type: formschema.FieldText, Placeholder: "例如：技术频道"},
			{Key: "text", Label: "自定义文本", Type: formschema.FieldTextarea, Placeholder: "\n\n-- via 技术频道"},
			{Key: "prefix", Label: "放在正文前", Type: formschema.FieldBoolean, Default: false},
		},
	})
	RegisterWithDescriptor("truncate_text", truncateText{}, Descriptor{
		Type:        "truncate_text",
		Label:       "截断文本",
		Description: "超过指定长度时截断，并追加省略符。",
		Fields: []formschema.FieldSpec{
			{Key: "max_length", Label: "最大长度", Type: formschema.FieldNumber, Required: true, Placeholder: "500"},
			{Key: "ellipsis", Label: "省略符", Type: formschema.FieldText, Default: "…", Placeholder: "…"},
		},
	})
	RegisterWithDescriptor("preserve_links", preserveLinks{}, Descriptor{
		Type:        "preserve_links",
		Label:       "保留链接",
		Description: "把解析到的链接和原始 Telegram 链接追加到正文，避免模板处理后丢失。",
		Fields: []formschema.FieldSpec{
			{Key: "include_original_url", Label: "包含原始链接", Type: formschema.FieldBoolean, Default: true},
		},
	})
	RegisterWithDescriptor("media_fallback_text", mediaFallbackText{}, Descriptor{
		Type:        "media_fallback_text",
		Label:       "媒体降级文本",
		Description: "把媒体附件摘要追加到正文，用于不支持媒体的渠道。",
		Fields: []formschema.FieldSpec{
			{Key: "include_size", Label: "包含大小", Type: formschema.FieldBoolean, Default: true},
			{Key: "include_url", Label: "包含链接", Type: formschema.FieldBoolean, Default: true},
		},
	})
	RegisterWithDescriptor("mask_sensitive", maskSensitive{}, Descriptor{
		Type:        "mask_sensitive",
		Label:       "遮罩敏感信息",
		Description: "遮罩手机号、邮箱和常见 token/password/secret 片段。",
		Fields: []formschema.FieldSpec{
			{Key: "mask_phone", Label: "遮罩手机号", Type: formschema.FieldBoolean, Default: true},
			{Key: "mask_email", Label: "遮罩邮箱", Type: formschema.FieldBoolean, Default: true},
			{Key: "mask_tokens", Label: "遮罩 token/secret", Type: formschema.FieldBoolean, Default: true},
		},
	})
	RegisterWithDescriptor("dedupe", dedupe{}, Descriptor{
		Type:        "dedupe",
		Label:       "去重",
		Description: "去除正文中的重复行，并去除重复链接。",
		Fields: []formschema.FieldSpec{
			{Key: "dedupe_lines", Label: "去除重复行", Type: formschema.FieldBoolean, Default: true},
			{Key: "dedupe_links", Label: "去除重复链接", Type: formschema.FieldBoolean, Default: true},
		},
	})
	RegisterWithDescriptor("quiet_hours", quietHours{}, Descriptor{
		Type:        "quiet_hours",
		Label:       "静默时间标记",
		Description: "命中静默时间时在正文前追加标记；延迟/抑制投递需后续调度能力配合。",
		Fields: []formschema.FieldSpec{
			{Key: "start", Label: "开始时间", Type: formschema.FieldText, Required: true, Placeholder: "22:00"},
			{Key: "end", Label: "结束时间", Type: formschema.FieldText, Required: true, Placeholder: "08:00"},
			{Key: "timezone", Label: "时区", Type: formschema.FieldText, Default: "Asia/Shanghai", Placeholder: "Asia/Shanghai"},
			{Key: "prefix", Label: "标记文本", Type: formschema.FieldText, Default: "[静默时间] "},
		},
	})
	RegisterWithDescriptor("batch_digest", batchDigest{}, Descriptor{
		Type:        "batch_digest",
		Label:       "摘要格式化",
		Description: "把当前消息格式化为摘要条目；跨消息聚合由后续队列能力承接。",
		Fields: []formschema.FieldSpec{
			{Key: "title", Label: "摘要标题", Type: formschema.FieldText, Default: "消息摘要"},
			{Key: "include_source", Label: "包含来源", Type: formschema.FieldBoolean, Default: true},
		},
	})
}

// appendSource 在文本末尾追加来源标注。
// config: {"template": "\n\n— 来自 {source}", "prefix": false}
// 未配置 template 时使用默认格式。source 名取自 msg（由 ingest 预填在 SenderName/SourceID 之外，
// 此处使用消息已有的 OriginalURL 或占位）。
type appendSource struct{}

func (appendSource) Process(_ context.Context, msg *domainmessage.NormalizedMessage, config map[string]any) error {
	label := configStr(config, "label")
	if label == "" {
		label = configStr(config, "source_name")
	}

	suffix := "\n\n— via source"
	if label != "" {
		suffix = "\n\n— via " + label
	}
	if custom := configStr(config, "text"); custom != "" {
		suffix = custom
	}

	if configBoolean(config, "prefix") {
		msg.Text = suffix + msg.Text
	} else {
		msg.Text += suffix
	}
	return nil
}

// truncateText 将文本截断到 max_length，超出部分追加 ellipsis。
// config: {"max_length": 500, "ellipsis": "…"}
type truncateText struct{}

func (truncateText) Process(_ context.Context, msg *domainmessage.NormalizedMessage, config map[string]any) error {
	maxLen := configInt(config, "max_length")
	if maxLen <= 0 {
		return nil
	}
	runes := []rune(msg.Text)
	if len(runes) <= maxLen {
		return nil
	}
	ellipsis := configStr(config, "ellipsis")
	if ellipsis == "" {
		ellipsis = "…"
	}
	msg.Text = string(runes[:maxLen]) + ellipsis
	return nil
}

type preserveLinks struct{}

func (preserveLinks) Process(_ context.Context, msg *domainmessage.NormalizedMessage, config map[string]any) error {
	seen := map[string]struct{}{}
	var lines []string
	for _, link := range msg.Links {
		if link.URL == "" {
			continue
		}
		if _, ok := seen[link.URL]; ok {
			continue
		}
		seen[link.URL] = struct{}{}
		lines = append(lines, link.URL)
	}
	includeOriginal := true
	if _, ok := config["include_original_url"]; ok {
		includeOriginal = configBoolean(config, "include_original_url")
	}
	if includeOriginal && msg.OriginalURL != "" {
		if _, ok := seen[msg.OriginalURL]; !ok {
			lines = append(lines, msg.OriginalURL)
		}
	}
	appendLines(msg, lines)
	return nil
}

type mediaFallbackText struct{}

func (mediaFallbackText) Process(_ context.Context, msg *domainmessage.NormalizedMessage, config map[string]any) error {
	includeSize := true
	includeURL := true
	if _, ok := config["include_size"]; ok {
		includeSize = configBoolean(config, "include_size")
	}
	if _, ok := config["include_url"]; ok {
		includeURL = configBoolean(config, "include_url")
	}
	var lines []string
	for _, item := range msg.Media {
		line := "[" + mediaLabel(item.Type) + "消息]"
		if item.Caption != "" && item.Caption != msg.Text {
			line += " " + item.Caption
		}
		if item.FileName != "" {
			line += " 文件：" + item.FileName
		}
		if includeSize && item.Size > 0 {
			line += " 大小：" + humanBytes(item.Size)
		}
		if includeURL {
			if item.RemoteURL != "" {
				line += " " + item.RemoteURL
			} else if item.URL != "" {
				line += " " + item.URL
			} else if msg.OriginalURL != "" {
				line += " " + msg.OriginalURL
			}
		}
		lines = append(lines, line)
	}
	appendLines(msg, lines)
	return nil
}

type maskSensitive struct{}

func (maskSensitive) Process(_ context.Context, msg *domainmessage.NormalizedMessage, config map[string]any) error {
	text := msg.Text
	if configBoolDefault(config, "mask_phone", true) {
		text = regexp.MustCompile(`\b1[3-9]\d{9}\b`).ReplaceAllStringFunc(text, func(s string) string {
			return s[:3] + "****" + s[7:]
		})
	}
	if configBoolDefault(config, "mask_email", true) {
		text = regexp.MustCompile(`(?i)\b[A-Z0-9._%+\-]+@[A-Z0-9.\-]+\.[A-Z]{2,}\b`).ReplaceAllString(text, "***@***")
	}
	if configBoolDefault(config, "mask_tokens", true) {
		text = regexp.MustCompile(`(?i)\b(token|secret|password|passwd|pwd)\s*[:=]\s*([^\s,;]+)`).ReplaceAllString(text, "$1=***")
	}
	msg.Text = text
	return nil
}

type dedupe struct{}

func (dedupe) Process(_ context.Context, msg *domainmessage.NormalizedMessage, config map[string]any) error {
	if configBoolDefault(config, "dedupe_lines", true) {
		lines := strings.Split(msg.Text, "\n")
		seen := map[string]struct{}{}
		out := make([]string, 0, len(lines))
		for _, line := range lines {
			key := strings.TrimSpace(line)
			if key != "" {
				if _, ok := seen[key]; ok {
					continue
				}
				seen[key] = struct{}{}
			}
			out = append(out, line)
		}
		msg.Text = strings.Join(out, "\n")
	}
	if configBoolDefault(config, "dedupe_links", true) {
		seen := map[string]struct{}{}
		out := make([]domainmessage.Link, 0, len(msg.Links))
		for _, link := range msg.Links {
			if link.URL == "" {
				continue
			}
			if _, ok := seen[link.URL]; ok {
				continue
			}
			seen[link.URL] = struct{}{}
			out = append(out, link)
		}
		msg.Links = out
	}
	return nil
}

type quietHours struct{}

func (quietHours) ValidateConfig(config map[string]any) error {
	if _, err := parseClock(configStr(config, "start")); err != nil {
		return fmt.Errorf("开始时间无效: %w", err)
	}
	if _, err := parseClock(configStr(config, "end")); err != nil {
		return fmt.Errorf("结束时间无效: %w", err)
	}
	if tz := configStr(config, "timezone"); tz != "" {
		if _, err := time.LoadLocation(tz); err != nil {
			return fmt.Errorf("时区无效: %w", err)
		}
	}
	return nil
}

func (quietHours) Process(_ context.Context, msg *domainmessage.NormalizedMessage, config map[string]any) error {
	ok, err := inClockWindow(msg, config)
	if err != nil || !ok {
		return err
	}
	prefix := configStr(config, "prefix")
	if prefix == "" {
		prefix = "[静默时间] "
	}
	if !strings.HasPrefix(msg.Text, prefix) {
		msg.Text = prefix + msg.Text
	}
	return nil
}

type batchDigest struct{}

func (batchDigest) Process(_ context.Context, msg *domainmessage.NormalizedMessage, config map[string]any) error {
	title := configStr(config, "title")
	if title == "" {
		title = "消息摘要"
	}
	var b strings.Builder
	b.WriteString("【")
	b.WriteString(title)
	b.WriteString("】\n")
	if configBoolDefault(config, "include_source", true) && msg.SenderName != "" {
		b.WriteString("来源：")
		b.WriteString(msg.SenderName)
		b.WriteString("\n")
	}
	b.WriteString("- ")
	b.WriteString(strings.TrimSpace(msg.Text))
	msg.Text = b.String()
	return nil
}

// --- config 读取辅助 ---

func configStr(config map[string]any, key string) string {
	if config == nil {
		return ""
	}
	if v, ok := config[key].(string); ok {
		return v
	}
	return ""
}

func configBoolean(config map[string]any, key string) bool {
	if config == nil {
		return false
	}
	if v, ok := config[key].(bool); ok {
		return v
	}
	return false
}

func configBoolDefault(config map[string]any, key string, fallback bool) bool {
	if config == nil {
		return fallback
	}
	if _, ok := config[key]; !ok {
		return fallback
	}
	return configBoolean(config, key)
}

func configInt(config map[string]any, key string) int {
	if config == nil {
		return 0
	}
	switch v := config[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	}
	return 0
}

func appendLines(msg *domainmessage.NormalizedMessage, lines []string) {
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.Contains(msg.Text, line) {
			continue
		}
		if msg.Text == "" {
			msg.Text = line
		} else {
			msg.Text += "\n" + line
		}
	}
}

func mediaLabel(mediaType string) string {
	switch mediaType {
	case "photo", "image":
		return "图片"
	case "audio", "voice":
		return "音频"
	case "video":
		return "视频"
	case "file", "document":
		return "文件"
	default:
		return mediaType
	}
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

func inClockWindow(msg *domainmessage.NormalizedMessage, config map[string]any) (bool, error) {
	start, err := parseClock(configStr(config, "start"))
	if err != nil {
		return false, err
	}
	end, err := parseClock(configStr(config, "end"))
	if err != nil {
		return false, err
	}
	loc := time.Local
	if tz := configStr(config, "timezone"); tz != "" {
		loc, err = time.LoadLocation(tz)
		if err != nil {
			return false, err
		}
	}
	t := msg.ReceivedAt
	if msg.SentAt != nil {
		t = *msg.SentAt
	}
	local := t.In(loc)
	minute := local.Hour()*60 + local.Minute()
	if start <= end {
		return minute >= start && minute <= end, nil
	}
	return minute >= start || minute <= end, nil
}

func parseClock(value string) (int, error) {
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return 0, fmt.Errorf("应为 HH:MM")
	}
	hour, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil || hour < 0 || hour > 23 {
		return 0, fmt.Errorf("小时应在 0-23")
	}
	minute, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil || minute < 0 || minute > 59 {
		return 0, fmt.Errorf("分钟应在 0-59")
	}
	return hour*60 + minute, nil
}
