package condition

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"telegram-message-forward/internal/domain/formschema"
	domainmessage "telegram-message-forward/internal/domain/message"
)

func init() {
	RegisterWithDescriptor("keyword_contains", keywordContains{}, Descriptor{
		Type:        "keyword_contains",
		Label:       "包含关键词",
		Description: "消息文本包含任一关键词时命中。",
		Fields: []formschema.FieldSpec{
			{Key: "keywords", Label: "关键词", Type: formschema.FieldStringList, Required: true, Placeholder: "逐行输入关键词"},
			{Key: "case_sensitive", Label: "区分大小写", Type: formschema.FieldBoolean, Default: false},
		},
	})
	RegisterWithDescriptor("keyword_excludes", keywordExcludes{}, Descriptor{
		Type:        "keyword_excludes",
		Label:       "排除关键词",
		Description: "消息文本不包含任何排除词时命中。",
		Fields: []formschema.FieldSpec{
			{Key: "keywords", Label: "排除词", Type: formschema.FieldStringList, Required: true, Placeholder: "逐行输入排除词"},
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
