package telegram

import (
	"testing"

	"github.com/gotd/td/tg"
)

func TestNormalizeChannelPhotoMessage(t *testing.T) {
	msg := &tg.Message{
		ID:      42,
		Message: "hello from channel",
		Date:    1_700_000_000,
		PeerID:  &tg.PeerChannel{ChannelID: 123},
	}
	msg.SetGroupedID(456)
	media := &tg.MessageMediaPhoto{}
	media.SetPhoto(&tg.Photo{
		ID: 1, AccessHash: 2, FileReference: []byte{3},
		Sizes: []tg.PhotoSizeClass{
			&tg.PhotoSize{Type: "m", W: 320, H: 240, Size: 2048},
			&tg.PhotoSize{Type: "x", W: 1280, H: 720, Size: 8192},
		},
	})
	msg.SetMedia(media)

	ent := tg.Entities{
		Channels: map[int64]*tg.Channel{
			123: {ID: 123, Title: "My Channel", Username: "mychan"},
		},
	}

	nm := Normalize(7, msg, ent)

	if nm.SourceID != 7 {
		t.Fatalf("SourceID = %d, want 7", nm.SourceID)
	}
	if nm.ExternalMessageID != 42 {
		t.Fatalf("ExternalMessageID = %d, want 42", nm.ExternalMessageID)
	}
	if nm.Text != "hello from channel" {
		t.Fatalf("Text = %q", nm.Text)
	}
	if nm.MessageType != "photo" {
		t.Fatalf("MessageType = %q, want photo", nm.MessageType)
	}
	if nm.SenderPeerType != "channel" || nm.SenderID != 123 {
		t.Fatalf("sender = %s/%d, want channel/123", nm.SenderPeerType, nm.SenderID)
	}
	if nm.SenderName != "My Channel" {
		t.Fatalf("SenderName = %q", nm.SenderName)
	}
	if nm.GroupedID == nil || *nm.GroupedID != 456 {
		t.Fatalf("GroupedID = %v, want 456", nm.GroupedID)
	}
	if nm.OriginalURL != "https://t.me/mychan/42" {
		t.Fatalf("OriginalURL = %q", nm.OriginalURL)
	}
	if nm.SentAt == nil {
		t.Fatal("SentAt 应被填充")
	}
	if len(nm.Media) != 1 || nm.Media[0].Type != "photo" {
		t.Fatalf("Media = %+v", nm.Media)
	}
	if nm.Media[0].Width != 1280 || nm.Media[0].Height != 720 || nm.Media[0].Size != 8192 {
		t.Fatalf("photo media metadata = %+v", nm.Media[0])
	}
	if nm.Media[0].Caption != "hello from channel" {
		t.Fatalf("Caption = %q", nm.Media[0].Caption)
	}
}

func TestNormalizeDocumentImageMessage(t *testing.T) {
	media := &tg.MessageMediaDocument{}
	media.SetDocument(&tg.Document{
		ID: 9, AccessHash: 10, FileReference: []byte{1, 2},
		MimeType: "image/png",
		Size:     4096,
		Attributes: []tg.DocumentAttributeClass{
			&tg.DocumentAttributeFilename{FileName: "chart.png"},
			&tg.DocumentAttributeImageSize{W: 640, H: 480},
		},
	})
	msg := &tg.Message{
		ID:      77,
		Message: "chart caption",
		PeerID:  &tg.PeerChannel{ChannelID: 123},
	}
	msg.SetMedia(media)

	ent := tg.Entities{
		Channels: map[int64]*tg.Channel{
			123: {ID: 123, Title: "Images", Username: "images"},
		},
	}

	nm := Normalize(8, msg, ent)
	if nm.MessageType != "image" {
		t.Fatalf("MessageType = %q, want image", nm.MessageType)
	}
	if len(nm.Media) != 1 {
		t.Fatalf("Media length = %d, want 1", len(nm.Media))
	}
	got := nm.Media[0]
	if got.Type != "image" || got.MimeType != "image/png" || got.FileName != "chart.png" {
		t.Fatalf("image media identity = %+v", got)
	}
	if got.Width != 640 || got.Height != 480 || got.Size != 4096 {
		t.Fatalf("image media metadata = %+v", got)
	}
	if got.Caption != "chart caption" {
		t.Fatalf("Caption = %q", got.Caption)
	}
}

func TestNormalizeUserTextMessage(t *testing.T) {
	msg := &tg.Message{
		ID:      9,
		Message: "hi",
		PeerID:  &tg.PeerUser{UserID: 555},
	}
	msg.SetFromID(&tg.PeerUser{UserID: 555})

	ent := tg.Entities{
		Users: map[int64]*tg.User{
			555: {ID: 555, FirstName: "Ada", LastName: "Lovelace"},
		},
	}

	nm := Normalize(3, msg, ent)
	if nm.MessageType != "text" {
		t.Fatalf("MessageType = %q, want text", nm.MessageType)
	}
	if nm.SenderPeerType != "user" || nm.SenderID != 555 {
		t.Fatalf("sender = %s/%d", nm.SenderPeerType, nm.SenderID)
	}
	if nm.SenderName != "Ada Lovelace" {
		t.Fatalf("SenderName = %q", nm.SenderName)
	}
	if nm.OriginalURL != "" {
		t.Fatalf("user 消息不应有 OriginalURL: %q", nm.OriginalURL)
	}
}

func TestNormalizeExtractsTelegramLinks(t *testing.T) {
	prefix := "前缀😀"
	title := "游资大V复盘文章汇总20260705"
	bareURL := "https://example.com/a"
	text := prefix + title + "\n裸链 " + bareURL
	msg := &tg.Message{
		ID:      10,
		Message: text,
		PeerID:  &tg.PeerUser{UserID: 555},
	}
	msg.SetEntities([]tg.MessageEntityClass{
		&tg.MessageEntityTextURL{
			Offset: testUTF16Len(prefix),
			Length: testUTF16Len(title),
			URL:    "https://pan.baidu.com/s/1Hg80L07OLfcW99xhco5UQQ?pwd=uscp",
		},
		&tg.MessageEntityURL{
			Offset: testUTF16Len(prefix + title + "\n裸链 "),
			Length: testUTF16Len(bareURL),
		},
	})

	nm := Normalize(3, msg, tg.Entities{})

	if len(nm.Links) != 2 {
		t.Fatalf("Links length = %d, want 2: %+v", len(nm.Links), nm.Links)
	}
	if nm.Links[0].URL != "https://pan.baidu.com/s/1Hg80L07OLfcW99xhco5UQQ?pwd=uscp" || nm.Links[0].Title != title {
		t.Fatalf("hidden link = %+v", nm.Links[0])
	}
	if nm.Links[1].URL != bareURL || nm.Links[1].Title != bareURL {
		t.Fatalf("bare link = %+v", nm.Links[1])
	}
}

func testUTF16Len(s string) int {
	n := 0
	for _, r := range s {
		n += utf16RuneLen(r)
	}
	return n
}
