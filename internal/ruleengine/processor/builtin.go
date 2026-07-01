package processor

import (
	"context"

	domainmessage "telegram-message-forward/internal/domain/message"
)

func init() {
	Register("append_source", appendSource{})
	Register("truncate_text", truncateText{})
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
