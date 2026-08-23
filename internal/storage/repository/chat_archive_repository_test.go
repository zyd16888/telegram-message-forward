package repository

import (
	"encoding/json"
	"testing"
	"time"

	domainarchive "telegram-message-forward/internal/domain/chatarchive"
)

// 关键词里的 LIKE 通配符必须转义，否则搜 "100%" 会退化成匹配全部。
func TestEscapeLike(t *testing.T) {
	cases := map[string]string{
		"100%":     `100\%`,
		"a_b":      `a\_b`,
		`back\sla`: `back\\sla`,
		"普通中文":     "普通中文",
		"":         "",
		"%_\\":     `\%\_\\`,
	}
	for in, want := range cases {
		if got := escapeLike(in); got != want {
			t.Fatalf("escapeLike(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestArchiveMessageModelRoundTrip(t *testing.T) {
	date := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	grouped := int64(88)
	src := &domainarchive.Message{
		MessageID:        1234,
		GroupedID:        &grouped,
		ReplyToMessageID: 1200,
		Out:              true,
		SenderPeerType:   "user",
		SenderID:         42,
		SenderName:       "阿达",
		SenderUsername:   "ada",
		MessageType:      "photo",
		Text:             "带 100% 的中文正文",
		Entities:         []domainarchive.Entity{{Type: "text_url", Offset: 2, Length: 3, URL: "https://example.com"}},
		Media:            []domainarchive.Media{{Type: "photo", MimeType: "image/jpeg", Size: 999}},
		Forward:          &domainarchive.Forward{FromID: 7, FromName: "原作者", ChannelPost: 55},
		Reactions:        []domainarchive.Reaction{{Emoticon: "👍", Count: 3}},
		Views:            12,
		Date:             &date,
	}

	mo, err := toArchiveMessageModel(9, src)
	if err != nil {
		t.Fatal(err)
	}
	if mo.ArchiveID != 9 || !mo.Outgoing {
		t.Fatalf("archive_id/outgoing 映射错误: %+v", mo)
	}

	got, err := toArchiveMessageDomain(mo)
	if err != nil {
		t.Fatal(err)
	}

	if got.MessageID != src.MessageID || got.ReplyToMessageID != src.ReplyToMessageID {
		t.Fatalf("id/回复链丢失: %+v", got)
	}
	if !got.Out {
		t.Fatal("方向（out）丢失")
	}
	if got.Text != src.Text || got.SenderName != src.SenderName {
		t.Fatalf("正文/发送者丢失: %+v", got)
	}
	if len(got.Entities) != 1 || got.Entities[0].URL != "https://example.com" {
		t.Fatalf("entities 丢失: %+v", got.Entities)
	}
	if len(got.Media) != 1 || got.Media[0].Size != 999 {
		t.Fatalf("media 丢失: %+v", got.Media)
	}
	if len(got.Reactions) != 1 || got.Reactions[0].Emoticon != "👍" {
		t.Fatalf("reactions 丢失: %+v", got.Reactions)
	}
	if got.Forward == nil || got.Forward.FromName != "原作者" || got.Forward.ChannelPost != 55 {
		t.Fatalf("转发来源丢失: %+v", got.Forward)
	}
	if got.Date == nil || !got.Date.Equal(date) {
		t.Fatalf("时间丢失: %+v", got.Date)
	}
}

// 无转发来源时 fwd_from 必须留空，而不是被写成 "[]"。
func TestArchiveMessageForwardNilStaysNull(t *testing.T) {
	mo, err := toArchiveMessageModel(1, &domainarchive.Message{MessageID: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(mo.FwdFrom) != 0 {
		t.Fatalf("无转发来源时 fwd_from 应为空，实际 %s", string(mo.FwdFrom))
	}
	got, err := toArchiveMessageDomain(mo)
	if err != nil {
		t.Fatal(err)
	}
	if got.Forward != nil {
		t.Fatalf("无转发来源时不应还原出 Forward: %+v", got.Forward)
	}
}

// 空切片必须落成 "[]" 而不是 null，保证 jsonb NOT NULL 约束与
// jsonb_array_length() 统计（media_count）不会报错。
func TestArchiveMessageEmptySlicesSerializeAsArray(t *testing.T) {
	mo, err := toArchiveMessageModel(1, &domainarchive.Message{MessageID: 2})
	if err != nil {
		t.Fatal(err)
	}
	for name, raw := range map[string][]byte{
		"entities":  mo.Entities,
		"media":     mo.Media,
		"reactions": mo.Reactions,
	} {
		if string(raw) != "[]" {
			t.Fatalf("%s 应序列化为 []，实际 %s", name, string(raw))
		}
		var arr []json.RawMessage
		if err := json.Unmarshal(raw, &arr); err != nil {
			t.Fatalf("%s 不是合法 JSON 数组: %v", name, err)
		}
	}
}

func TestJobStatusTerminal(t *testing.T) {
	for _, s := range []domainarchive.JobStatus{domainarchive.JobSucceeded, domainarchive.JobFailed, domainarchive.JobCancelled} {
		if !s.Terminal() {
			t.Fatalf("%s 应为终态", s)
		}
	}
	for _, s := range []domainarchive.JobStatus{domainarchive.JobPending, domainarchive.JobRunning} {
		if s.Terminal() {
			t.Fatalf("%s 不应为终态", s)
		}
	}
}

func TestPeerTypeValid(t *testing.T) {
	for _, p := range []domainarchive.PeerType{domainarchive.PeerUser, domainarchive.PeerChat, domainarchive.PeerChannel} {
		if !p.Valid() {
			t.Fatalf("%s 应为合法 peer 类型", p)
		}
	}
	if domainarchive.PeerType("bot").Valid() {
		t.Fatal("未知 peer 类型不应通过校验")
	}
}
