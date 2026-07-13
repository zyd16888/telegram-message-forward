package telegram

import (
	"testing"

	domainsource "telegram-message-forward/internal/domain/source"
)

func TestHistoryBackfillEnabledDefaultFalse(t *testing.T) {
	if HistoryBackfillEnabled(nil) {
		t.Fatal("nil source 默认应关闭")
	}
	if HistoryBackfillEnabled(&domainsource.Source{}) {
		t.Fatal("无 config 默认应关闭")
	}
	if HistoryBackfillEnabled(&domainsource.Source{Config: map[string]any{}}) {
		t.Fatal("空 config 默认应关闭")
	}
	if !HistoryBackfillEnabled(&domainsource.Source{Config: map[string]any{"history_backfill_enabled": true}}) {
		t.Fatal("显式 true 应开启")
	}
}

func TestHistoryBackfillLimitBounds(t *testing.T) {
	if got := HistoryBackfillLimit(nil, 0); got != defaultHistoryLimit {
		t.Fatalf("default limit = %d", got)
	}
	if got := HistoryBackfillLimit(nil, 1000); got != maxHistoryLimit {
		t.Fatalf("hard cap = %d, want %d", got, maxHistoryLimit)
	}
	src := &domainsource.Source{Config: map[string]any{"history_backfill_limit": float64(20)}}
	if got := HistoryBackfillLimit(src, 0); got != 20 {
		t.Fatalf("config limit = %d", got)
	}
	if got := HistoryBackfillLimit(src, 10); got != 10 {
		t.Fatalf("override limit = %d", got)
	}
}

func TestCapabilitiesSupportsHistory(t *testing.T) {
	p := NewPlugin(Deps{})
	if !p.Capabilities().SupportsHistory {
		t.Fatal("历史接口落地后 SupportsHistory 应为 true")
	}
}

func TestCatchUpSkipsWhenDisabledOrZeroCursor(t *testing.T) {
	p := NewPlugin(Deps{})
	// disabled
	if err := p.CatchUpIfNeeded(t.Context(), nil, &domainsource.Source{ID: 1, LastMessageID: 10}, nil); err != nil {
		t.Fatal(err)
	}
	// enabled but last_message_id=0 → 不灌历史
	src := &domainsource.Source{
		ID:            2,
		LastMessageID: 0,
		Config:        map[string]any{"history_backfill_enabled": true},
	}
	if err := p.CatchUpIfNeeded(t.Context(), nil, src, nil); err != nil {
		t.Fatalf("新源不应报错: %v", err)
	}
}
