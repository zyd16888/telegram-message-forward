package telegram

import (
	"testing"

	"github.com/gotd/td/tg"

	domainsource "telegram-message-forward/internal/domain/source"
)

func TestBuildEntitiesFromHistoryResponse(t *testing.T) {
	ent := buildEntities(
		[]tg.UserClass{
			&tg.User{ID: 7, FirstName: "Ada", LastName: "Lovelace"},
			&tg.UserEmpty{ID: 8},
		},
		[]tg.ChatClass{
			&tg.Chat{ID: 20, Title: "小群"},
			&tg.Channel{ID: 30, Title: "频道"},
			&tg.ChatForbidden{ID: 40},
		},
	)

	if got := ent.Users[7]; got == nil || got.FirstName != "Ada" {
		t.Fatalf("user 未装配: %+v", ent.Users)
	}
	if _, ok := ent.Users[8]; ok {
		t.Fatal("UserEmpty 不应写入实体")
	}
	if got := ent.Chats[20]; got == nil || got.Title != "小群" {
		t.Fatalf("chat 未装配: %+v", ent.Chats)
	}
	if got := ent.Channels[30]; got == nil || got.Title != "频道" {
		t.Fatalf("channel 未装配: %+v", ent.Channels)
	}
	if _, ok := ent.Chats[40]; ok {
		t.Fatal("ChatForbidden 不应写入实体")
	}
}

// 补拉/导出路径必须带上响应里的 entities，否则 sender_name 全为空，
// 与实时监听路径的数据对不上。
func TestUnpackHistoryPageCarriesEntities(t *testing.T) {
	res := &tg.MessagesMessagesSlice{
		Messages: []tg.MessageClass{
			&tg.Message{ID: 5, Message: "hello", FromID: &tg.PeerUser{UserID: 7}, PeerID: &tg.PeerUser{UserID: 7}},
		},
		Users: []tg.UserClass{&tg.User{ID: 7, FirstName: "Ada"}},
	}

	page := unpackHistoryPage(res)
	msgs := page.messages()
	if len(msgs) != 1 {
		t.Fatalf("应解出 1 条普通消息，实际 %d", len(msgs))
	}

	nm := Normalize(1, msgs[0], page.Entities)
	if nm.SenderName != "Ada" {
		t.Fatalf("sender_name = %q，want Ada（entities 丢失会退化为空）", nm.SenderName)
	}
}

func TestUnpackHistoryPageAllVariants(t *testing.T) {
	msg := []tg.MessageClass{&tg.Message{ID: 1}}
	users := []tg.UserClass{&tg.User{ID: 2, FirstName: "U"}}

	cases := map[string]tg.MessagesMessagesClass{
		"messages":        &tg.MessagesMessages{Messages: msg, Users: users},
		"slice":           &tg.MessagesMessagesSlice{Messages: msg, Users: users},
		"channelMessages": &tg.MessagesChannelMessages{Messages: msg, Users: users},
	}
	for name, res := range cases {
		page := unpackHistoryPage(res)
		if len(page.Raw) != 1 {
			t.Fatalf("%s: Raw 长度 = %d", name, len(page.Raw))
		}
		if page.Entities.Users[2] == nil {
			t.Fatalf("%s: entities 未装配", name)
		}
	}

	empty := unpackHistoryPage(&tg.MessagesMessagesNotModified{})
	if len(empty.Raw) != 0 || empty.Entities.Users == nil {
		t.Fatal("未知响应类型应返回空页且实体可写")
	}
}

// 分页游标必须基于原始消息（含服务消息）计算。整页都是服务消息时若把该页
// 当成空页，就会被误判为「已经拉完」而提前终止，中间的消息永远拉不到。
func TestHistoryPageCursorCountsServiceMessages(t *testing.T) {
	page := historyPage{Raw: []tg.MessageClass{
		&tg.MessageService{ID: 30},
		&tg.MessageService{ID: 31},
	}}

	if len(page.messages()) != 0 {
		t.Fatal("服务消息不应作为可转发消息返回")
	}
	if got := page.minID(); got != 30 {
		t.Fatalf("minID = %d, want 30", got)
	}
	if got := page.maxID(); got != 31 {
		t.Fatalf("maxID = %d, want 31", got)
	}
	if len(page.Raw) == 0 {
		t.Fatal("Raw 非空才能让分页继续翻页")
	}
}

func TestHistoryPageCursorMixed(t *testing.T) {
	page := historyPage{Raw: []tg.MessageClass{
		&tg.Message{ID: 12},
		&tg.MessageService{ID: 10},
		&tg.Message{ID: 15},
		&tg.MessageEmpty{ID: 0},
	}}

	if got := page.minID(); got != 10 {
		t.Fatalf("minID = %d, want 10（应忽略 id=0 的空消息）", got)
	}
	if got := page.maxID(); got != 15 {
		t.Fatalf("maxID = %d, want 15", got)
	}
	if len(page.messages()) != 2 {
		t.Fatalf("普通消息数 = %d, want 2", len(page.messages()))
	}
}

func TestMergeEntitiesAccumulatesAcrossPages(t *testing.T) {
	first := buildEntities([]tg.UserClass{&tg.User{ID: 1, FirstName: "A"}}, nil)
	second := buildEntities(
		[]tg.UserClass{&tg.User{ID: 2, FirstName: "B"}},
		[]tg.ChatClass{&tg.Channel{ID: 9, Title: "C"}},
	)

	merged := mergeEntities(first, second)
	if merged.Users[1] == nil || merged.Users[2] == nil {
		t.Fatalf("跨页用户未累积: %+v", merged.Users)
	}
	if merged.Channels[9] == nil {
		t.Fatalf("跨页频道未累积: %+v", merged.Channels)
	}

	// 零值 Entities 作为起点也必须可写。
	var zero tg.Entities
	if got := mergeEntities(zero, second); got.Users[2] == nil {
		t.Fatal("零值 Entities 合并后应可用")
	}
}

func TestHistoryCatchUpMaxDefaultAndOverride(t *testing.T) {
	if got := HistoryCatchUpMax(nil); got != defaultCatchUpMaxTotal {
		t.Fatalf("默认追平上限 = %d, want %d", got, defaultCatchUpMaxTotal)
	}
	src := &domainsource.Source{Config: map[string]any{"history_catchup_max": float64(250)}}
	if got := HistoryCatchUpMax(src); got != 250 {
		t.Fatalf("配置覆盖追平上限 = %d, want 250", got)
	}
	// 非法/零值配置退回默认值，避免把上限配成 0 导致无限拉取。
	zero := &domainsource.Source{Config: map[string]any{"history_catchup_max": float64(0)}}
	if got := HistoryCatchUpMax(zero); got != defaultCatchUpMaxTotal {
		t.Fatalf("零值配置应退回默认，实际 %d", got)
	}
}
