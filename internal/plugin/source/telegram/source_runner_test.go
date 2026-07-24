package telegram

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/gotd/td/tg"

	domainmessage "telegram-message-forward/internal/domain/message"
	domainsource "telegram-message-forward/internal/domain/source"
)

func TestStopRemovesSourceAndCancelsLastRunner(t *testing.T) {
	p := NewPlugin(Deps{})
	cancelled := false
	runner := newAccountRunner(10, nil, func() { cancelled = true })
	runner.setSource(&domainsource.Source{ID: 1, AccountID: 10, PeerType: domainsource.PeerChannel, PeerID: 100}, nil)
	runner.setSource(&domainsource.Source{ID: 2, AccountID: 10, PeerType: domainsource.PeerChannel, PeerID: 200}, nil)
	p.runners[10] = runner

	if err := p.Stop(context.Background(), &domainsource.Source{ID: 1, AccountID: 10}); err != nil {
		t.Fatal(err)
	}
	if cancelled {
		t.Fatal("仍有订阅时不应取消 account runner")
	}
	if got := len(p.runners[10].sources); got != 1 {
		t.Fatalf("剩余订阅数 = %d, want 1", got)
	}

	if err := p.Stop(context.Background(), &domainsource.Source{ID: 2, AccountID: 10}); err != nil {
		t.Fatal(err)
	}
	if !cancelled {
		t.Fatal("最后一个订阅停止时应取消 account runner")
	}
	if _, ok := p.runners[10]; ok {
		t.Fatal("最后一个订阅停止后应移除 account runner")
	}
}

func TestStopAccountWaitsForRunnerExit(t *testing.T) {
	p := NewPlugin(Deps{})
	var runner *accountRunner
	runner = newAccountRunner(10, nil, func() { close(runner.done) })
	p.runners[10] = runner

	if err := p.StopAccount(context.Background(), 10); err != nil {
		t.Fatalf("StopAccount 失败: %v", err)
	}
	if _, ok := p.runners[10]; ok {
		t.Fatal("StopAccount 后应移除账号 runner")
	}
}

func TestRunningClientReusesReadyRunner(t *testing.T) {
	p := NewPlugin(Deps{})
	runner := newAccountRunner(10, nil, func() {})
	runner.markReady()
	p.runners[10] = runner

	client, ok, err := p.runningClient(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || client != runner.client {
		t.Fatal("同步应复用已就绪账号 runner 的 client")
	}
}

func TestForwardToSubscriptionsOnlyMatchesSource(t *testing.T) {
	p := NewPlugin(Deps{})
	var got []int64
	handler := func(_ context.Context, msg *domainmessage.NormalizedMessage) error {
		got = append(got, msg.SourceID)
		return nil
	}
	runner := newAccountRunner(10, nil, func() {})
	runner.setSource(&domainsource.Source{ID: 1, AccountID: 10, PeerType: domainsource.PeerChannel, PeerID: 100}, handler)
	runner.setSource(&domainsource.Source{ID: 2, AccountID: 10, PeerType: domainsource.PeerChannel, PeerID: 200}, handler)
	p.runners[10] = runner

	p.forwardToSubscriptions(context.Background(), 10, nil, tg.Entities{}, &tg.Message{
		ID:      42,
		PeerID:  &tg.PeerChannel{ChannelID: 100},
		Message: "hello",
	})

	if len(got) != 1 || got[0] != 1 {
		t.Fatalf("应只分发给匹配 source，got=%v", got)
	}
}

func TestPrivateUserSourceMatchesPrivateSender(t *testing.T) {
	src := &domainsource.Source{ID: 1, AccountID: 10, PeerType: domainsource.PeerUser, PeerID: 200}
	msg := &tg.Message{ID: 43, PeerID: &tg.PeerUser{UserID: 100}, Message: "hello"}
	msg.SetFromID(&tg.PeerUser{UserID: 200})

	if !matchesSource(msg, src) {
		t.Fatal("私聊消息应允许用 FromID 匹配 user source")
	}
}

func TestPrivateUserSourceDoesNotMatchGroupSender(t *testing.T) {
	src := &domainsource.Source{ID: 1, AccountID: 10, PeerType: domainsource.PeerUser, PeerID: 200}
	msg := &tg.Message{ID: 44, PeerID: &tg.PeerChat{ChatID: 300}, Message: "hello"}
	msg.SetFromID(&tg.PeerUser{UserID: 200})

	if matchesSource(msg, src) {
		t.Fatal("群消息不能仅因 FromID 相同而命中私聊 user source")
	}
}

func TestPrivateUserSourceDoesNotMatchOutgoingFromDifferentDialog(t *testing.T) {
	src := &domainsource.Source{ID: 1, AccountID: 10, PeerType: domainsource.PeerUser, PeerID: 200}
	msg := &tg.Message{ID: 45, PeerID: &tg.PeerUser{UserID: 300}, Message: "hello"}
	msg.SetOut(true)
	msg.SetFromID(&tg.PeerUser{UserID: 200})

	if matchesSource(msg, src) {
		t.Fatal("发给其他私聊对象的 outgoing 消息不能因 FromID 命中 user source")
	}
}

func TestEditResendCoalescesLatestMessage(t *testing.T) {
	p := NewPlugin(Deps{})
	p.editDebounce = 20 * time.Millisecond
	got := make(chan *domainmessage.NormalizedMessage, 2)
	handler := func(_ context.Context, msg *domainmessage.NormalizedMessage) error {
		got <- msg
		return nil
	}
	runner := newAccountRunner(10, nil, func() {})
	runner.setSource(&domainsource.Source{
		ID: 1, AccountID: 10, PeerType: domainsource.PeerChannel, PeerID: 100,
		Config: map[string]any{
			"edit_resend_enabled":        true,
			"edit_resend_suffix_enabled": true,
			"edit_resend_suffix":         "--custom",
		},
	}, handler)
	p.runners[10] = runner

	first := &tg.Message{ID: 42, PeerID: &tg.PeerChannel{ChannelID: 100}, Message: "first edit"}
	first.SetEditDate(1_700_000_001)
	second := &tg.Message{ID: 42, PeerID: &tg.PeerChannel{ChannelID: 100}, Message: "latest edit"}
	second.SetEditDate(1_700_000_002)
	p.scheduleEdit(context.Background(), 10, nil, tg.Entities{}, first)
	p.scheduleEdit(context.Background(), 10, nil, tg.Entities{}, second)

	select {
	case msg := <-got:
		if msg.Text != "latest edit" {
			t.Fatalf("补发内容 = %q, want latest edit", msg.Text)
		}
		if msg.EventKind != domainmessage.EventEdit || msg.EditTextSuffix != "--custom" {
			t.Fatalf("编辑事件元数据错误: %+v", msg)
		}
		if msg.EditedAt == nil || msg.EditedAt.Unix() != 1_700_000_002 {
			t.Fatalf("EditedAt = %v", msg.EditedAt)
		}
	case <-time.After(time.Second):
		t.Fatal("等待编辑补发超时")
	}

	select {
	case msg := <-got:
		t.Fatalf("30 秒窗口内多次编辑只应补发一次，额外收到 %+v", msg)
	case <-time.After(60 * time.Millisecond):
	}
}

func TestDeleteCancelsPendingEditResend(t *testing.T) {
	p := NewPlugin(Deps{})
	p.editDebounce = 30 * time.Millisecond
	got := make(chan *domainmessage.NormalizedMessage, 1)
	runner := newAccountRunner(10, nil, func() {})
	runner.setSource(&domainsource.Source{
		ID: 1, AccountID: 10, PeerType: domainsource.PeerChannel, PeerID: 100,
		Config: map[string]any{"edit_resend_enabled": true},
	}, func(_ context.Context, msg *domainmessage.NormalizedMessage) error {
		got <- msg
		return nil
	})
	p.runners[10] = runner

	msg := &tg.Message{ID: 42, PeerID: &tg.PeerChannel{ChannelID: 100}, Message: "edited"}
	p.scheduleEdit(context.Background(), 10, nil, tg.Entities{}, msg)
	channelID := int64(100)
	p.cancelDeletedEdits(10, []int{42}, &channelID)

	select {
	case msg := <-got:
		t.Fatalf("消息删除后不应补发编辑内容: %+v", msg)
	case <-time.After(80 * time.Millisecond):
	}
	if len(runner.pendingEdits) != 0 {
		t.Fatalf("删除后待补发队列未清空: %d", len(runner.pendingEdits))
	}
}

func TestValidateEditResendConfig(t *testing.T) {
	p := NewPlugin(Deps{})
	if err := p.ValidateConfig(map[string]any{"edit_resend_enabled": "yes"}); err == nil {
		t.Fatal("非布尔 edit_resend_enabled 应校验失败")
	}
	if err := p.ValidateConfig(map[string]any{"edit_resend_suffix": strings.Repeat("字", 201)}); err == nil {
		t.Fatal("超过 200 字符的后缀应校验失败")
	}
	if got := sourceEditResendSuffix(&domainsource.Source{Config: map[string]any{}}); got != defaultEditResendSuffix {
		t.Fatalf("默认后缀 = %q", got)
	}
}
