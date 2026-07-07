package condition

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
	RegisterWithDescriptor("keyword_contains", keywordContains{}, Descriptor{
		Type:        "keyword_contains",
		Label:       "关键词匹配：命中后通过",
		Description: "消息文本匹配任一关键词时通过。",
		Fields: []formschema.FieldSpec{
			{Key: "keywords", Label: "关键词", Type: formschema.FieldStringList, Required: true, Placeholder: "逐行输入关键词"},
			{Key: "case_sensitive", Label: "区分大小写", Type: formschema.FieldBoolean, Default: false},
		},
	})
	RegisterWithDescriptor("keyword_excludes", keywordExcludes{}, Descriptor{
		Type:        "keyword_excludes",
		Label:       "关键词匹配：命中后阻止",
		Description: "消息文本匹配任一关键词时阻止通过。",
		Fields: []formschema.FieldSpec{
			{Key: "keywords", Label: "关键词", Type: formschema.FieldStringList, Required: true, Placeholder: "逐行输入关键词"},
			{Key: "case_sensitive", Label: "区分大小写", Type: formschema.FieldBoolean, Default: false},
		},
	})
	RegisterWithDescriptor("regex", regexCondition{}, Descriptor{
		Type:        "regex",
		Label:       "正则匹配",
		Description: "消息文本匹配给定正则表达式时命中。",
		Fields: []formschema.FieldSpec{
			{Key: "pattern", Label: "正则表达式", Type: formschema.FieldText, Required: true, Placeholder: "^ERROR|timeout$"},
		},
	})
	RegisterWithDescriptor("message_type", messageType{}, Descriptor{
		Type:        "message_type",
		Label:       "消息类型",
		Description: "消息类型属于所选集合时命中。",
		Fields: []formschema.FieldSpec{
			{
				Key:      "types",
				Label:    "消息类型",
				Type:     formschema.FieldMultiSelect,
				Required: true,
				Options: []formschema.Option{
					{Label: "文本", Value: "text"},
					{Label: "图片", Value: "photo"},
					{Label: "文档", Value: "document"},
					{Label: "位置", Value: "geo"},
					{Label: "联系人", Value: "contact"},
					{Label: "投票", Value: "poll"},
					{Label: "其他媒体", Value: "media"},
				},
			},
		},
	})
	RegisterWithDescriptor("source", sourceCondition{}, Descriptor{
		Type:        "source",
		Label:       "监听源",
		Description: "消息来自指定监听源时命中。",
		Fields: []formschema.FieldSpec{
			{Key: "source_ids", Label: "监听源 ID", Type: formschema.FieldStringList, Required: true, Placeholder: "逐行输入 source_id"},
		},
	})
	RegisterWithDescriptor("sender", senderCondition{}, Descriptor{
		Type:        "sender",
		Label:       "发送者",
		Description: "按发送者类型、ID 或名称匹配。",
		Fields: []formschema.FieldSpec{
			{
				Key:   "peer_types",
				Label: "发送者类型",
				Type:  formschema.FieldMultiSelect,
				Options: []formschema.Option{
					{Label: "用户", Value: "user"},
					{Label: "普通群", Value: "chat"},
					{Label: "频道/超级群", Value: "channel"},
				},
			},
			{Key: "sender_ids", Label: "发送者 ID", Type: formschema.FieldStringList, Placeholder: "逐行输入 sender_id"},
			{Key: "names", Label: "名称包含", Type: formschema.FieldStringList, Placeholder: "逐行输入名称关键词"},
		},
	})
	RegisterWithDescriptor("time_window", timeWindow{}, Descriptor{
		Type:        "time_window",
		Label:       "时间窗口",
		Description: "消息时间落在指定每日时间段内时命中，支持跨午夜。",
		Fields: []formschema.FieldSpec{
			{Key: "start", Label: "开始时间", Type: formschema.FieldText, Required: true, Placeholder: "09:00"},
			{Key: "end", Label: "结束时间", Type: formschema.FieldText, Required: true, Placeholder: "18:30"},
			{Key: "timezone", Label: "时区", Type: formschema.FieldText, Default: "Asia/Shanghai", Placeholder: "Asia/Shanghai"},
		},
	})
	RegisterWithDescriptor("has_media", hasMedia{}, Descriptor{
		Type:        "has_media",
		Label:       "是否有媒体",
		Description: "按消息是否携带媒体附件匹配。",
		Fields: []formschema.FieldSpec{
			{Key: "value", Label: "需要包含媒体", Type: formschema.FieldBoolean, Default: true},
		},
	})
	RegisterWithDescriptor("media_type", mediaType{}, Descriptor{
		Type:        "media_type",
		Label:       "媒体类型",
		Description: "消息包含指定媒体类型时命中。",
		Fields: []formschema.FieldSpec{
			{
				Key:      "types",
				Label:    "媒体类型",
				Type:     formschema.FieldMultiSelect,
				Required: true,
				Options: []formschema.Option{
					{Label: "图片", Value: "image"},
					{Label: "照片", Value: "photo"},
					{Label: "文件", Value: "document"},
					{Label: "音频", Value: "audio"},
					{Label: "语音", Value: "voice"},
					{Label: "视频", Value: "video"},
				},
			},
		},
	})
	RegisterWithDescriptor("message_length", messageLength{}, Descriptor{
		Type:        "message_length",
		Label:       "文本长度",
		Description: "按消息文本字符数范围匹配。",
		Fields: []formschema.FieldSpec{
			{Key: "min", Label: "最小长度", Type: formschema.FieldNumber, Placeholder: "0"},
			{Key: "max", Label: "最大长度", Type: formschema.FieldNumber, Placeholder: "500"},
		},
	})
}

// keywordContains 命中：文本包含任一关键词。
// config: {"keywords": ["a","b"], "case_sensitive": false}
type keywordContains struct{}

func (keywordContains) Evaluate(_ context.Context, msg *domainmessage.NormalizedMessage, config map[string]any) (bool, error) {
	keywords := configStringSlice(config, "keywords")
	if len(keywords) == 0 {
		return true, nil // 未配置关键词视为不过滤
	}
	caseSensitive := configBool(config, "case_sensitive")
	text := msg.Text
	if !caseSensitive {
		text = strings.ToLower(text)
	}
	for _, kw := range keywords {
		if kw == "" {
			continue
		}
		if !caseSensitive {
			kw = strings.ToLower(kw)
		}
		if strings.Contains(text, kw) {
			return true, nil
		}
	}
	return false, nil
}

// keywordExcludes 命中：文本不包含任何排除词。
// config: {"keywords": ["a","b"], "case_sensitive": false}
type keywordExcludes struct{}

func (keywordExcludes) Evaluate(_ context.Context, msg *domainmessage.NormalizedMessage, config map[string]any) (bool, error) {
	keywords := configStringSlice(config, "keywords")
	if len(keywords) == 0 {
		return true, nil
	}
	caseSensitive := configBool(config, "case_sensitive")
	text := msg.Text
	if !caseSensitive {
		text = strings.ToLower(text)
	}
	for _, kw := range keywords {
		if kw == "" {
			continue
		}
		if !caseSensitive {
			kw = strings.ToLower(kw)
		}
		if strings.Contains(text, kw) {
			return false, nil // 命中排除词，不通过
		}
	}
	return true, nil
}

// regexCondition 命中：文本匹配给定正则。
// config: {"pattern": "^\\d+$"}
type regexCondition struct{}

func (regexCondition) ValidateConfig(config map[string]any) error {
	pattern := configString(config, "pattern")
	if pattern == "" {
		return fmt.Errorf("regex 条件缺少 pattern")
	}
	if _, err := regexp.Compile(pattern); err != nil {
		return fmt.Errorf("正则编译失败: %w", err)
	}
	return nil
}

func (regexCondition) Evaluate(_ context.Context, msg *domainmessage.NormalizedMessage, config map[string]any) (bool, error) {
	pattern := configString(config, "pattern")
	if pattern == "" {
		return false, fmt.Errorf("regex 条件缺少 pattern")
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return false, fmt.Errorf("正则编译失败: %w", err)
	}
	return re.MatchString(msg.Text), nil
}

// messageType 命中：消息类型属于给定集合。
// config: {"types": ["text","photo"]}
type messageType struct{}

func (messageType) Evaluate(_ context.Context, msg *domainmessage.NormalizedMessage, config map[string]any) (bool, error) {
	types := configStringSlice(config, "types")
	if len(types) == 0 {
		return true, nil
	}
	for _, t := range types {
		if t == msg.MessageType {
			return true, nil
		}
	}
	return false, nil
}

type sourceCondition struct{}

func (sourceCondition) Evaluate(_ context.Context, msg *domainmessage.NormalizedMessage, config map[string]any) (bool, error) {
	ids := configInt64Slice(config, "source_ids")
	if len(ids) == 0 {
		return true, nil
	}
	for _, id := range ids {
		if msg.SourceID == id {
			return true, nil
		}
	}
	return false, nil
}

type senderCondition struct{}

func (senderCondition) Evaluate(_ context.Context, msg *domainmessage.NormalizedMessage, config map[string]any) (bool, error) {
	if types := configStringSlice(config, "peer_types"); len(types) > 0 && !stringIn(msg.SenderPeerType, types) {
		return false, nil
	}
	if ids := configInt64Slice(config, "sender_ids"); len(ids) > 0 {
		found := false
		for _, id := range ids {
			if msg.SenderID == id {
				found = true
				break
			}
		}
		if !found {
			return false, nil
		}
	}
	names := configStringSlice(config, "names")
	if len(names) == 0 {
		return true, nil
	}
	name := strings.ToLower(msg.SenderName)
	for _, item := range names {
		if item != "" && strings.Contains(name, strings.ToLower(item)) {
			return true, nil
		}
	}
	return false, nil
}

type timeWindow struct{}

func (timeWindow) ValidateConfig(config map[string]any) error {
	if _, err := parseClock(configString(config, "start")); err != nil {
		return fmt.Errorf("开始时间无效: %w", err)
	}
	if _, err := parseClock(configString(config, "end")); err != nil {
		return fmt.Errorf("结束时间无效: %w", err)
	}
	if tz := configString(config, "timezone"); tz != "" {
		if _, err := time.LoadLocation(tz); err != nil {
			return fmt.Errorf("时区无效: %w", err)
		}
	}
	return nil
}

func (timeWindow) Evaluate(_ context.Context, msg *domainmessage.NormalizedMessage, config map[string]any) (bool, error) {
	start, err := parseClock(configString(config, "start"))
	if err != nil {
		return false, err
	}
	end, err := parseClock(configString(config, "end"))
	if err != nil {
		return false, err
	}
	loc := time.Local
	if tz := configString(config, "timezone"); tz != "" {
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

type hasMedia struct{}

func (hasMedia) Evaluate(_ context.Context, msg *domainmessage.NormalizedMessage, config map[string]any) (bool, error) {
	want := true
	if config != nil {
		if _, ok := config["value"]; ok {
			want = configBool(config, "value")
		}
	}
	return (len(msg.Media) > 0) == want, nil
}

type mediaType struct{}

func (mediaType) Evaluate(_ context.Context, msg *domainmessage.NormalizedMessage, config map[string]any) (bool, error) {
	types := configStringSlice(config, "types")
	if len(types) == 0 {
		return true, nil
	}
	for _, item := range msg.Media {
		if stringIn(item.Type, types) {
			return true, nil
		}
	}
	return false, nil
}

type messageLength struct{}

func (messageLength) Evaluate(_ context.Context, msg *domainmessage.NormalizedMessage, config map[string]any) (bool, error) {
	n := len([]rune(msg.Text))
	minLen := configInt(config, "min")
	maxLen := configInt(config, "max")
	if minLen > 0 && n < minLen {
		return false, nil
	}
	if maxLen > 0 && n > maxLen {
		return false, nil
	}
	return true, nil
}

// --- config 读取辅助 ---

func configString(config map[string]any, key string) string {
	if config == nil {
		return ""
	}
	if v, ok := config[key].(string); ok {
		return v
	}
	return ""
}

func configBool(config map[string]any, key string) bool {
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
	case string:
		n, _ := strconv.Atoi(strings.TrimSpace(v))
		return n
	}
	return 0
}

func configStringSlice(config map[string]any, key string) []string {
	if config == nil {
		return nil
	}
	raw, ok := config[key]
	if !ok {
		return nil
	}
	switch v := raw.(type) {
	case []string:
		return v
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case string:
		return []string{v}
	}
	return nil
}

func configInt64Slice(config map[string]any, key string) []int64 {
	raw := configStringSlice(config, key)
	out := make([]int64, 0, len(raw))
	for _, item := range raw {
		n, err := strconv.ParseInt(strings.TrimSpace(item), 10, 64)
		if err == nil {
			out = append(out, n)
		}
	}
	return out
}

func stringIn(value string, candidates []string) bool {
	for _, item := range candidates {
		if item == value {
			return true
		}
	}
	return false
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
