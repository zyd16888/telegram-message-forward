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
