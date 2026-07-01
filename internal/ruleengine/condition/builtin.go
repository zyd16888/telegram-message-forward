package condition

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	domainmessage "telegram-message-forward/internal/domain/message"
)

func init() {
	Register("keyword_contains", keywordContains{})
	Register("keyword_excludes", keywordExcludes{})
	Register("regex", regexCondition{})
	Register("message_type", messageType{})
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
