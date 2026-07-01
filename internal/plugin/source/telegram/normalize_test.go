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
	msg.SetMedia(&tg.MessageMediaPhoto{})

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
