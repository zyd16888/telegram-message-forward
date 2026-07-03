package telegram

import (
	"testing"

	"github.com/gotd/td/tg"
)

func TestUnpackDialogsKeepsPagingForFullDialogsResponse(t *testing.T) {
	dialogs := make([]tg.DialogClass, syncDialogsPageSize+1)
	for i := range dialogs {
		dialogs[i] = &tg.Dialog{Peer: &tg.PeerUser{UserID: int64(i + 1)}, TopMessage: i + 1}
	}

	_, _, _, _, hasMore, err := unpackDialogs(&tg.MessagesDialogs{Dialogs: dialogs})
	if err != nil {
		t.Fatal(err)
	}
	if !hasMore {
		t.Fatal("messages.dialogs 返回达到 page size 时应继续分页")
	}
}

func TestUnpackDialogsStopsForShortDialogsResponse(t *testing.T) {
	dialogs := make([]tg.DialogClass, syncDialogsPageSize-1)
	for i := range dialogs {
		dialogs[i] = &tg.Dialog{Peer: &tg.PeerUser{UserID: int64(i + 1)}, TopMessage: i + 1}
	}

	_, _, _, _, hasMore, err := unpackDialogs(&tg.MessagesDialogs{Dialogs: dialogs})
	if err != nil {
		t.Fatal(err)
	}
	if hasMore {
		t.Fatal("短页不应继续分页")
	}
}

func TestNextDialogOffsetSkipsInvalidTail(t *testing.T) {
	dialogs := []tg.DialogClass{
		&tg.Dialog{Peer: &tg.PeerChannel{ChannelID: 42}, TopMessage: 77},
		&tg.DialogFolder{TopMessage: 0},
	}
	messages := []tg.MessageClass{&tg.Message{ID: 77, Date: 123456}}
	chats := []tg.ChatClass{&tg.Channel{ID: 42, AccessHash: 999}}

	peer, topID, date := nextDialogOffset(dialogs, messages, chats, nil)
	if topID != 77 || date != 123456 {
		t.Fatalf("offset = topID %d date %d, want 77/123456", topID, date)
	}
	ch, ok := peer.(*tg.InputPeerChannel)
	if !ok {
		t.Fatalf("offset peer = %T, want *tg.InputPeerChannel", peer)
	}
	if ch.ChannelID != 42 || ch.AccessHash != 999 {
		t.Fatalf("offset peer = %+v", ch)
	}
}

func TestNextDialogOffsetHandlesServiceMessageTop(t *testing.T) {
	// 会话的最新消息是服务消息（比如有人入群）时，日期查找不能静默退化为 0，
	// 否则算出的 OffsetDate=0 会破坏 Telegram 分页游标的一致性，导致分页原地打转。
	dialogs := []tg.DialogClass{
		&tg.Dialog{Peer: &tg.PeerChannel{ChannelID: 42}, TopMessage: 273},
	}
	messages := []tg.MessageClass{&tg.MessageService{ID: 273, Date: 123456}}
	chats := []tg.ChatClass{&tg.Channel{ID: 42, AccessHash: 999}}

	_, topID, date := nextDialogOffset(dialogs, messages, chats, nil)
	if topID != 273 || date != 123456 {
		t.Fatalf("offset = topID %d date %d, want 273/123456", topID, date)
	}
}
