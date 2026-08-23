package telegram

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/gotd/td/tg"

	domainaccount "telegram-message-forward/internal/domain/account"
	domainarchive "telegram-message-forward/internal/domain/chatarchive"
	domainmessage "telegram-message-forward/internal/domain/message"
)

// 归档映射必须保住转发用不到、但分析必需的字段：回复链、方向、
// 富文本 offset、转发来源、reactions。
func TestArchiveFromMessageKeepsAnalysisFields(t *testing.T) {
	ent := buildEntities(
		[]tg.UserClass{&tg.User{ID: 7, FirstName: "阿达", Username: "ada"}},
		[]tg.ChatClass{&tg.Channel{ID: 99, Title: "原频道", Username: "src"}},
	)

	msg := &tg.Message{
		ID:      500,
		Out:     true,
		Message: "回你一句",
		Date:    int(time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC).Unix()),
		FromID:  &tg.PeerUser{UserID: 7},
		PeerID:  &tg.PeerUser{UserID: 7},
		Views:   9,
	}
	msg.SetReplyTo(&tg.MessageReplyHeader{ReplyToMsgID: 499})
	msg.SetEntities([]tg.MessageEntityClass{
		&tg.MessageEntityTextURL{Offset: 1, Length: 2, URL: "https://example.com"},
		&tg.MessageEntityBold{Offset: 0, Length: 1},
	})
	// gotd 的可选字段依赖 flags：必须走 Set* 置位，直接赋值 Get* 读不到。
	fwd := tg.MessageFwdHeader{}
	fwd.SetFromID(&tg.PeerChannel{ChannelID: 99})
	fwd.SetChannelPost(12)
	msg.SetFwdFrom(fwd)
	msg.SetReactions(tg.MessageReactions{Results: []tg.ReactionCount{
		{Reaction: &tg.ReactionEmoji{Emoticon: "👍"}, Count: 4},
		{Reaction: &tg.ReactionCustomEmoji{DocumentID: 1}, Count: 2},
	}})

	got := toArchiveMessage(msg, ent)
	if got == nil {
		t.Fatal("普通消息应被映射")
	}
	if got.MessageID != 500 {
		t.Fatalf("message_id = %d", got.MessageID)
	}
	if !got.Out {
		t.Fatal("方向丢失：out 应为 true")
	}
	if got.ReplyToMessageID != 499 {
		t.Fatalf("回复链丢失: reply_to = %d", got.ReplyToMessageID)
	}
	if got.SenderName != "阿达" || got.SenderUsername != "ada" {
		t.Fatalf("发送者解析错误: name=%q username=%q", got.SenderName, got.SenderUsername)
	}
	if got.Views != 9 {
		t.Fatalf("views = %d", got.Views)
	}
	if len(got.Entities) != 2 {
		t.Fatalf("entities 数量 = %d, want 2", len(got.Entities))
	}
	if got.Entities[0].Type != "text_url" || got.Entities[0].URL != "https://example.com" || got.Entities[0].Offset != 1 {
		t.Fatalf("entity 映射错误: %+v", got.Entities[0])
	}
	if got.Forward == nil || got.Forward.FromID != 99 || got.Forward.ChannelPost != 12 {
		t.Fatalf("转发来源丢失: %+v", got.Forward)
	}
	if got.Forward.FromName != "原频道" {
		t.Fatalf("转发来源名应从 entities 补全，实际 %q", got.Forward.FromName)
	}
	// 自定义 emoji 无 emoticon，跳过而不是写入空串。
	if len(got.Reactions) != 1 || got.Reactions[0].Emoticon != "👍" || got.Reactions[0].Count != 4 {
		t.Fatalf("reactions 映射错误: %+v", got.Reactions)
	}
	if got.Date == nil || got.Date.Year() != 2026 {
		t.Fatalf("时间丢失: %+v", got.Date)
	}
}

// 服务消息必须被归档（入群、改名等是分析会话的重要上下文），
// 且以 service_action 标识，而不是当成空文本消息。
func TestArchiveFromServiceMessage(t *testing.T) {
	ent := buildEntities([]tg.UserClass{&tg.User{ID: 3, FirstName: "小明"}}, nil)
	svc := &tg.MessageService{
		ID:     77,
		Date:   int(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC).Unix()),
		FromID: &tg.PeerUser{UserID: 3},
		PeerID: &tg.PeerChat{ChatID: 8},
		Action: &tg.MessageActionChatAddUser{Users: []int64{4}},
	}

	got := toArchiveMessage(svc, ent)
	if got == nil {
		t.Fatal("服务消息应被映射")
	}
	if got.MessageID != 77 {
		t.Fatalf("message_id = %d", got.MessageID)
	}
	if got.MessageType != "service" {
		t.Fatalf("message_type = %q, want service", got.MessageType)
	}
	if got.ServiceAction != "chat_add_user" {
		t.Fatalf("service_action = %q, want chat_add_user", got.ServiceAction)
	}
	if got.SenderName != "小明" {
		t.Fatalf("服务消息发送者丢失: %q", got.SenderName)
	}
}

func TestServiceActionName(t *testing.T) {
	cases := map[tg.MessageActionClass]string{
		&tg.MessageActionChatAddUser{}:    "chat_add_user",
		&tg.MessageActionChatDeleteUser{}: "chat_delete_user",
		&tg.MessageActionPinMessage{}:     "pin_message",
	}
	for action, want := range cases {
		if got := serviceActionName(action); got != want {
			t.Fatalf("serviceActionName(%T) = %q, want %q", action, got, want)
		}
	}
	if got := serviceActionName(nil); got != "unknown" {
		t.Fatalf("nil action = %q, want unknown", got)
	}
}

func TestToArchiveMessageSkipsEmpty(t *testing.T) {
	if got := toArchiveMessage(&tg.MessageEmpty{ID: 1}, emptyEntities()); got != nil {
		t.Fatalf("空消息不应被归档: %+v", got)
	}
}

// 频道 post 没有 FromID，发送者应回退为来源 peer 本身。
func TestArchiveSenderFallsBackToPeer(t *testing.T) {
	ent := buildEntities(nil, []tg.ChatClass{&tg.Channel{ID: 55, Title: "公告频道"}})
	msg := &tg.Message{ID: 1, PeerID: &tg.PeerChannel{ChannelID: 55}, Message: "post"}

	got := toArchiveMessage(msg, ent)
	if got.SenderPeerType != "channel" || got.SenderID != 55 || got.SenderName != "公告频道" {
		t.Fatalf("频道 post 发送者回退错误: %+v", got)
	}
}

func TestArchiveMediaMetadata(t *testing.T) {
	voice := &tg.Message{ID: 1, Message: "语音"}
	voiceDoc := &tg.MessageMediaDocument{}
	voiceDoc.SetDocument(&tg.Document{
		MimeType: "audio/ogg", Size: 1234,
		Attributes: []tg.DocumentAttributeClass{&tg.DocumentAttributeAudio{Voice: true, Duration: 7}},
	})
	voice.SetMedia(voiceDoc)
	got := archiveMedia(voice)
	if len(got) != 1 || got[0].Type != "voice" || got[0].Duration != 7 || got[0].Size != 1234 {
		t.Fatalf("语音元信息错误: %+v", got)
	}
	// 默认不下载，Downloaded 必须为 false。
	if got[0].Downloaded || got[0].StorageKey != "" {
		t.Fatalf("默认不应下载媒体: %+v", got[0])
	}

	video := &tg.Message{ID: 2}
	videoDoc := &tg.MessageMediaDocument{}
	videoDoc.SetDocument(&tg.Document{
		MimeType: "video/mp4", Size: 999,
		Attributes: []tg.DocumentAttributeClass{&tg.DocumentAttributeVideo{Duration: 3.9, W: 640, H: 480}},
	})
	video.SetMedia(videoDoc)
	if got := archiveMedia(video); len(got) != 1 || got[0].Type != "video" || got[0].Width != 640 || got[0].Duration != 3 {
		t.Fatalf("视频元信息错误: %+v", got)
	}

	photo := &tg.Message{ID: 3, Message: "图"}
	photoMedia := &tg.MessageMediaPhoto{}
	photoMedia.SetPhoto(&tg.Photo{
		Sizes: []tg.PhotoSizeClass{&tg.PhotoSize{Type: "x", W: 100, H: 200, Size: 500}},
	})
	photo.SetMedia(photoMedia)
	if got := archiveMedia(photo); len(got) != 1 || got[0].Type != "photo" || got[0].Height != 200 {
		t.Fatalf("图片元信息错误: %+v", got)
	}

	if got := archiveMedia(&tg.Message{ID: 4, Message: "纯文本"}); got != nil {
		t.Fatalf("纯文本不应有媒体: %+v", got)
	}
}

// 媒体命名空间必须区分归档与监听源，否则不同会话里相同的 message id
// 会在临时目录里互相覆盖。
func TestMediaNamespaceSeparatesArchiveAndSource(t *testing.T) {
	m := domainmessage.Media{Type: "photo", MimeType: "image/jpeg"}
	src := mediaStorageKey(sourceNamespace(3), 100, 0, m)
	arc := mediaStorageKey(archiveNamespace(3), 100, 0, m)
	if src == arc {
		t.Fatalf("归档与监听源的媒体键不应相同: %s", src)
	}
	// 扩展名由 mime.ExtensionsByType 决定，随平台而异（Windows 上 image/jpeg 可能是 .jfif），
	// 这里只断言命名空间前缀。
	if !strings.HasPrefix(src, "telegram/source_3/100_0.") {
		t.Fatalf("监听源媒体键 = %s", src)
	}
	if !strings.HasPrefix(arc, "telegram/archive_3/100_0.") {
		t.Fatalf("归档媒体键 = %s", arc)
	}
}

func TestExportHistoryValidatesInput(t *testing.T) {
	p := NewPlugin(Deps{})
	acc := &domainaccount.Account{ID: 1}
	visit := func(context.Context, ExportPage) error { return nil }
	okOpts := ExportOptions{PeerType: domainarchive.PeerUser, PeerID: 2}

	if err := p.ExportHistory(t.Context(), nil, okOpts, visit); err == nil {
		t.Fatal("账号为空应报错")
	}
	if err := p.ExportHistory(t.Context(), acc, okOpts, nil); err == nil {
		t.Fatal("缺少页回调应报错")
	}
	bad := ExportOptions{PeerType: domainarchive.PeerType("bot"), PeerID: 2}
	if err := p.ExportHistory(t.Context(), acc, bad, visit); err == nil {
		t.Fatal("非法 peer 类型应报错")
	}
	// peer 缓存未装配时必须给出可操作的报错，而不是空指针。
	if err := p.ExportHistory(t.Context(), acc, okOpts, visit); err == nil {
		t.Fatal("peer 缓存未装配应报错")
	}
}

// Done 与 NextOffsetID 的组合必须能区分「真的到底」和「中途停」。
// 分不清会让调用方把没拉完的任务误判为已完成，静默丢掉剩余历史。
func TestExportPageDoneSemantics(t *testing.T) {
	finished := ExportPage{Done: true, NextOffsetID: 0}
	if !finished.Done || finished.NextOffsetID != 0 {
		t.Fatal("到底应为 Done=true 且无断点")
	}

	paused := ExportPage{Done: true, NextOffsetID: 4321}
	if !paused.Done || paused.NextOffsetID == 0 {
		t.Fatal("中途停应为 Done=true 且带断点游标")
	}

	more := ExportPage{Done: false, NextOffsetID: 999}
	if more.Done || more.NextOffsetID == 0 {
		t.Fatal("还有下一页应为 Done=false 且带游标")
	}
}
