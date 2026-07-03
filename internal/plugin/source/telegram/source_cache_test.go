package telegram

import (
	"context"
	"testing"

	"github.com/gotd/td/tg"

	domainpeer "telegram-message-forward/internal/domain/peer"
	domainsource "telegram-message-forward/internal/domain/source"
	pluginsource "telegram-message-forward/internal/plugin/source"
)

type fakePeerRepo struct {
	peers []*domainpeer.Peer
	bulk  []*domainpeer.Peer
}

func (r *fakePeerRepo) Upsert(_ context.Context, p *domainpeer.Peer) error {
	r.peers = append(r.peers, p)
	return nil
}

func (r *fakePeerRepo) BulkUpsert(_ context.Context, peers []*domainpeer.Peer) error {
	r.bulk = append(r.bulk, peers...)
	return nil
}

func (r *fakePeerRepo) Get(_ context.Context, _ int64, _ domainpeer.Type, _ int64) (*domainpeer.Peer, error) {
	return nil, nil
}

func (r *fakePeerRepo) ListByAccount(_ context.Context, accountID int64) ([]*domainpeer.Peer, error) {
	out := make([]*domainpeer.Peer, 0, len(r.peers))
	for _, peer := range r.peers {
		if peer.AccountID == accountID {
			out = append(out, peer)
		}
	}
	return out, nil
}

func TestEmitCachedPeers(t *testing.T) {
	repo := &fakePeerRepo{peers: []*domainpeer.Peer{
		{AccountID: 1, PeerType: domainpeer.TypeChannel, PeerID: 42, Title: "Cached Channel", Username: "cached"},
		{AccountID: 2, PeerType: domainpeer.TypeUser, PeerID: 99, Title: "Other Account"},
	}}
	plugin := NewPlugin(Deps{Peers: repo})

	var emitted []string
	err := plugin.emitCachedPeers(context.Background(), 1, func(peer pluginsource.SyncedPeer) error {
		emitted = append(emitted, string(peer.PeerType)+":"+peer.Name)
		if !peer.Cached {
			t.Fatal("缓存 peer 应带 cached 标记")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(emitted) != 1 || emitted[0] != "channel:Cached Channel" {
		t.Fatalf("缓存发射结果不正确: %+v", emitted)
	}
}

func TestEmitDialogPeersUpsertsCache(t *testing.T) {
	repo := &fakePeerRepo{}
	plugin := NewPlugin(Deps{Peers: repo})
	dialogs := []tg.DialogClass{
		&tg.Dialog{Peer: &tg.PeerUser{UserID: 10}, TopMessage: 1},
		&tg.Dialog{Peer: &tg.PeerChat{ChatID: 20}, TopMessage: 2},
		&tg.Dialog{Peer: &tg.PeerChannel{ChannelID: 30}, TopMessage: 3},
	}
	users := []tg.UserClass{&tg.User{ID: 10, AccessHash: 1010, Username: "ada", FirstName: "Ada"}}
	chats := []tg.ChatClass{
		&tg.Chat{ID: 20, Title: "Basic Group"},
		&tg.Channel{ID: 30, AccessHash: 3030, Title: "Super Group", Megagroup: true},
	}

	var emitted []domainsource.PeerType
	newCount, err := plugin.emitDialogPeers(context.Background(), 7, dialogs, chats, users, map[string]struct{}{}, func(peer pluginsource.SyncedPeer) error {
		emitted = append(emitted, peer.PeerType)
		if peer.Cached {
			t.Fatal("远端刷新 peer 不应带 cached 标记")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if newCount != 3 {
		t.Fatalf("应返回 3 个新增 peer，实际 %d", newCount)
	}
	if len(emitted) != 3 {
		t.Fatalf("应发射 3 个 peer，实际 %d", len(emitted))
	}
	if len(repo.bulk) != 3 {
		t.Fatalf("应批量写入 3 个缓存，实际 %d", len(repo.bulk))
	}
	if repo.bulk[0].AccountID != 7 || repo.bulk[0].PeerType != domainpeer.TypeUser || repo.bulk[0].AccessHash != 1010 {
		t.Fatalf("用户缓存不正确: %+v", repo.bulk[0])
	}
	if repo.bulk[2].PeerType != domainpeer.TypeChannel || repo.bulk[2].AccessHash != 3030 {
		t.Fatalf("频道缓存不正确: %+v", repo.bulk[2])
	}
}
