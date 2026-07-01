package condition

import (
	"context"
	"testing"

	domainmessage "telegram-message-forward/internal/domain/message"
)

func msg(text, mtype string) *domainmessage.NormalizedMessage {
	return &domainmessage.NormalizedMessage{Text: text, MessageType: mtype}
}

func TestKeywordContains(t *testing.T) {
	c, err := Get("keyword_contains")
	if err != nil {
		t.Fatal(err)
	}
	cfg := map[string]any{"keywords": []any{"golang", "rust"}}

	ok, err := c.Evaluate(context.Background(), msg("I love GoLang", ""), cfg)
	if err != nil || !ok {
		t.Fatalf("大小写不敏感应命中: ok=%v err=%v", ok, err)
	}

	ok, _ = c.Evaluate(context.Background(), msg("I love python", ""), cfg)
	if ok {
		t.Fatal("不含关键词不应命中")
	}

	// case_sensitive
	cfgCS := map[string]any{"keywords": []any{"Go"}, "case_sensitive": true}
	ok, _ = c.Evaluate(context.Background(), msg("go go go", ""), cfgCS)
	if ok {
		t.Fatal("大小写敏感下不应命中小写")
	}
}

func TestKeywordExcludes(t *testing.T) {
	c, _ := Get("keyword_excludes")
	cfg := map[string]any{"keywords": []any{"广告", "spam"}}

	ok, _ := c.Evaluate(context.Background(), msg("正常消息", ""), cfg)
	if !ok {
		t.Fatal("不含排除词应通过")
	}
	ok, _ = c.Evaluate(context.Background(), msg("这是广告", ""), cfg)
	if ok {
		t.Fatal("含排除词不应通过")
	}
}

func TestRegex(t *testing.T) {
	c, _ := Get("regex")
	cfg := map[string]any{"pattern": `\d{6}`}

	ok, _ := c.Evaluate(context.Background(), msg("验证码 123456", ""), cfg)
	if !ok {
		t.Fatal("应匹配 6 位数字")
	}
	ok, _ = c.Evaluate(context.Background(), msg("no digits", ""), cfg)
	if ok {
		t.Fatal("无数字不应匹配")
	}
	if _, err := c.Evaluate(context.Background(), msg("x", ""), map[string]any{"pattern": "["}); err == nil {
		t.Fatal("非法正则应返回错误")
	}
}

func TestMessageType(t *testing.T) {
	c, _ := Get("message_type")
	cfg := map[string]any{"types": []any{"text", "photo"}}

	ok, _ := c.Evaluate(context.Background(), msg("hi", "photo"), cfg)
	if !ok {
		t.Fatal("photo 应命中")
	}
	ok, _ = c.Evaluate(context.Background(), msg("hi", "video"), cfg)
	if ok {
		t.Fatal("video 不应命中")
	}
}
