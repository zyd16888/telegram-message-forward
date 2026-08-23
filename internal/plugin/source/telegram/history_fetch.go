package telegram

import (
	"context"
	"fmt"
	"time"

	"github.com/gotd/td/telegram"
	"github.com/gotd/td/tg"

	domainaccount "telegram-message-forward/internal/domain/account"
)

const (
	// historyPageSize 是单次 MessagesGetHistory 的条数，Telegram 单页上限即 100。
	historyPageSize = 100
	// historyPageDelay 是分页之间的间隔，配合 floodwait/ratelimit middleware 降低 FLOOD_WAIT 概率。
	historyPageDelay = 350 * time.Millisecond
	// historyMaxPages 是翻页次数硬顶，防止游标异常时无限循环。
	historyMaxPages = 500
)

// historyPage 是一页历史消息及其实体。
//
// Raw 保留服务消息（*tg.MessageService）：它们不参与转发，但必须计入分页游标，
// 否则整页都是服务消息时会被误判成「已经拉完」而提前终止。
type historyPage struct {
	Raw      []tg.MessageClass
	Entities tg.Entities
}

// messages 过滤出可转发的普通消息。
func (h historyPage) messages() []*tg.Message {
	out := make([]*tg.Message, 0, len(h.Raw))
	for _, m := range h.Raw {
		if msg, ok := m.(*tg.Message); ok {
			out = append(out, msg)
		}
	}
	return out
}

// minID 返回本页最小消息 id（含服务消息），用于向更早方向翻页。
func (h historyPage) minID() int64 {
	var min int64
	for _, m := range h.Raw {
		id := int64(m.GetID())
		if id <= 0 {
			continue
		}
		if min == 0 || id < min {
			min = id
		}
	}
	return min
}

// maxID 返回本页最大消息 id（含服务消息），用于向更新方向翻页。
func (h historyPage) maxID() int64 {
	var max int64
	for _, m := range h.Raw {
		id := int64(m.GetID())
		if id > max {
			max = id
		}
	}
	return max
}

// unpackHistoryPage 从 MessagesGetHistory 响应中取出消息与实体。
func unpackHistoryPage(res tg.MessagesMessagesClass) historyPage {
	switch v := res.(type) {
	case *tg.MessagesMessages:
		return historyPage{Raw: v.Messages, Entities: buildEntities(v.Users, v.Chats)}
	case *tg.MessagesMessagesSlice:
		return historyPage{Raw: v.Messages, Entities: buildEntities(v.Users, v.Chats)}
	case *tg.MessagesChannelMessages:
		return historyPage{Raw: v.Messages, Entities: buildEntities(v.Users, v.Chats)}
	default:
		return historyPage{Entities: emptyEntities()}
	}
}

// buildEntities 把响应里的 users/chats 组装成 tg.Entities。
//
// 缺了这一步，Normalize 的 fillSender 拿不到 users/chats，补拉与导出出来的消息
// sender_name 会全为空，和实时监听路径的数据对不上。
func buildEntities(users []tg.UserClass, chats []tg.ChatClass) tg.Entities {
	ent := emptyEntities()
	for _, u := range users {
		if user, ok := u.(*tg.User); ok {
			ent.Users[user.ID] = user
		}
	}
	for _, c := range chats {
		switch chat := c.(type) {
		case *tg.Chat:
			ent.Chats[chat.ID] = chat
		case *tg.Channel:
			ent.Channels[chat.ID] = chat
		}
	}
	return ent
}

// emptyEntities 返回可写入的空实体集合。
func emptyEntities() tg.Entities {
	return tg.Entities{
		Users:    map[int64]*tg.User{},
		Chats:    map[int64]*tg.Chat{},
		Channels: map[int64]*tg.Channel{},
	}
}

// mergeEntities 把 src 并入 dst，用于跨页累积实体。
func mergeEntities(dst, src tg.Entities) tg.Entities {
	if dst.Users == nil {
		dst = emptyEntities()
	}
	for id, u := range src.Users {
		dst.Users[id] = u
	}
	for id, c := range src.Chats {
		dst.Chats[id] = c
	}
	for id, c := range src.Channels {
		dst.Channels[id] = c
	}
	return dst
}

// withClient 在账号连接上执行 fn：优先复用运行中的 runner，否则临时建连并在返回后断开。
func (p *Plugin) withClient(ctx context.Context, acc *domainaccount.Account, fn func(context.Context, *telegram.Client) error) error {
	if acc == nil {
		return fmt.Errorf("账号为空")
	}
	client, ok, err := p.runningClient(ctx, acc.ID)
	if err != nil {
		return err
	}
	if ok && client != nil {
		return fn(ctx, client)
	}
	client, err = p.buildClient(acc, nil)
	if err != nil {
		return err
	}
	return client.Run(ctx, func(ctx context.Context) error {
		if _, err := p.ensureAuthorized(ctx, client); err != nil {
			return err
		}
		return fn(ctx, client)
	})
}

// historyQuery 是一次 MessagesGetHistory 的分页参数。
type historyQuery struct {
	// OffsetID 是分页游标；0 表示从最新一条开始。
	OffsetID int64
	// MinID / MaxID 是服务端过滤边界（开区间），0 表示不限。
	MinID int64
	MaxID int64
	// AddOffset 为负数时表示取 OffsetID 之后（更新）的消息，用于正向追平。
	AddOffset int
	PageSize  int
}

// getHistoryPage 发起一次分页请求。
func (p *Plugin) getHistoryPage(ctx context.Context, client *telegram.Client, peer tg.InputPeerClass, q historyQuery) (historyPage, error) {
	size := q.PageSize
	if size <= 0 || size > historyPageSize {
		size = historyPageSize
	}
	res, err := client.API().MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
		Peer:      peer,
		OffsetID:  int(q.OffsetID),
		AddOffset: q.AddOffset,
		Limit:     size,
		MinID:     int(q.MinID),
		MaxID:     int(q.MaxID),
	})
	if err != nil {
		return historyPage{}, fmt.Errorf("MessagesGetHistory 失败: %w", err)
	}
	return unpackHistoryPage(res), nil
}

// sleepCtx 是可被 ctx 取消的等待。
func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
