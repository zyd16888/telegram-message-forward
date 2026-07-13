package telegram

import (
	"context"
	"fmt"
	"sort"

	"github.com/gotd/td/telegram"
	"github.com/gotd/td/tg"

	domainaccount "telegram-message-forward/internal/domain/account"
	domainpeer "telegram-message-forward/internal/domain/peer"
	domainsource "telegram-message-forward/internal/domain/source"
	pluginsource "telegram-message-forward/internal/plugin/source"
)

const (
	defaultHistoryLimit = 50
	maxHistoryLimit     = 100
)

// HistoryPreviewItem 是历史预览条目（不投递）。
type HistoryPreviewItem struct {
	ExternalMessageID int64  `json:"external_message_id"`
	MessageType       string `json:"message_type"`
	Text              string `json:"text"`
	SenderName        string `json:"sender_name,omitempty"`
	SentAtUnix        int64  `json:"sent_at_unix,omitempty"`
}

// HistoryFetchResult 是一次历史拉取结果。
type HistoryFetchResult struct {
	Items      []HistoryPreviewItem
	Fetched    int
	Ingested   int
	Skipped    int
	MaxMessage int64
}

// HistoryBackfillEnabled 读取源级历史补拉开关，默认 false。
func HistoryBackfillEnabled(src *domainsource.Source) bool {
	if src == nil || src.Config == nil {
		return false
	}
	v, _ := src.Config["history_backfill_enabled"].(bool)
	return v
}

// HistoryBackfillLimit 读取单次补拉上限，默认 50，硬顶 100。
func HistoryBackfillLimit(src *domainsource.Source, override int) int {
	limit := defaultHistoryLimit
	if override > 0 {
		limit = override
	} else if src != nil && src.Config != nil {
		switch v := src.Config["history_backfill_limit"].(type) {
		case float64:
			if int(v) > 0 {
				limit = int(v)
			}
		case int:
			if v > 0 {
				limit = v
			}
		case int64:
			if v > 0 {
				limit = int(v)
			}
		}
	}
	if limit > maxHistoryLimit {
		limit = maxHistoryLimit
	}
	if limit < 1 {
		limit = 1
	}
	return limit
}

// PreviewHistory 拉取最近 limit 条历史消息预览，不投递、不推进游标。
func (p *Plugin) PreviewHistory(ctx context.Context, acc *domainaccount.Account, src *domainsource.Source, limit int) (*HistoryFetchResult, error) {
	if !HistoryBackfillEnabled(src) {
		return nil, fmt.Errorf("该源未开启历史补拉（history_backfill_enabled）")
	}
	limit = HistoryBackfillLimit(src, limit)
	msgs, err := p.fetchHistoryMessages(ctx, acc, src, 0, limit)
	if err != nil {
		return nil, err
	}
	return &HistoryFetchResult{
		Items:      toPreviewItems(msgs),
		Fetched:    len(msgs),
		MaxMessage: maxMsgID(msgs),
	}, nil
}

// ExecuteHistoryBackfill 拉取历史并走 ingest 回调（幂等防重投）。
// minID 为 0 时表示从最新向前取 limit 条；>0 时只取 ID > minID。
func (p *Plugin) ExecuteHistoryBackfill(
	ctx context.Context,
	acc *domainaccount.Account,
	src *domainsource.Source,
	minID int64,
	limit int,
	handler pluginsource.Handler,
) (*HistoryFetchResult, error) {
	if !HistoryBackfillEnabled(src) {
		return nil, fmt.Errorf("该源未开启历史补拉（history_backfill_enabled）")
	}
	if handler == nil {
		return nil, fmt.Errorf("历史补拉缺少 ingest handler")
	}
	limit = HistoryBackfillLimit(src, limit)
	msgs, err := p.fetchHistoryMessages(ctx, acc, src, minID, limit)
	if err != nil {
		return nil, err
	}
	// 按 ID 升序 ingest，便于游标单调推进。
	sort.Slice(msgs, func(i, j int) bool { return msgs[i].ID < msgs[j].ID })

	out := &HistoryFetchResult{Items: toPreviewItems(msgs), Fetched: len(msgs)}
	for _, msg := range msgs {
		nm := Normalize(src.ID, msg, tg.Entities{})
		// 补拉路径尽量补媒体；失败不阻断。
		client, ok, cerr := p.runningClient(ctx, acc.ID)
		if cerr != nil {
			return out, cerr
		}
		if ok && client != nil {
			nm.Media = downloadMessageMedia(ctx, client, src.ID, msg, nm.Media, p.downloadPolicy(), sourceDownloadFiles(src))
		}
		if err := handler(ctx, nm); err != nil {
			return out, fmt.Errorf("ingest 历史消息 %d 失败: %w", msg.ID, err)
		}
		out.Ingested++
		if int64(msg.ID) > out.MaxMessage {
			out.MaxMessage = int64(msg.ID)
		}
	}
	return out, nil
}

// CatchUpIfNeeded 启动/断线后追平：仅当开关开启且 last_message_id>0 时增量补拉。
// last_message_id=0 的新源不灌历史。
func (p *Plugin) CatchUpIfNeeded(ctx context.Context, acc *domainaccount.Account, src *domainsource.Source, handler pluginsource.Handler) error {
	if !HistoryBackfillEnabled(src) {
		return nil
	}
	if src.LastMessageID <= 0 {
		p.deps.Log.Info("历史补拉跳过：新源 last_message_id=0，仅收实时", "source", src.ID)
		return nil
	}
	res, err := p.ExecuteHistoryBackfill(ctx, acc, src, src.LastMessageID, 0, handler)
	if err != nil {
		return err
	}
	if res != nil && res.Fetched > 0 {
		p.deps.Log.Info("历史补拉完成",
			"source", src.ID,
			"fetched", res.Fetched,
			"ingested", res.Ingested,
			"min_id", src.LastMessageID,
			"max_id", res.MaxMessage,
		)
	}
	return nil
}

func (p *Plugin) fetchHistoryMessages(
	ctx context.Context,
	acc *domainaccount.Account,
	src *domainsource.Source,
	minID int64,
	limit int,
) ([]*tg.Message, error) {
	if acc == nil || src == nil {
		return nil, fmt.Errorf("账号或源为空")
	}
	inputPeer, err := p.resolveInputPeer(ctx, acc.ID, src)
	if err != nil {
		return nil, err
	}

	var out []*tg.Message
	run := func(ctx context.Context, client *telegram.Client) error {
		res, err := client.API().MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
			Peer:  inputPeer,
			Limit: limit,
			MinID: int(minID),
		})
		if err != nil {
			return fmt.Errorf("MessagesGetHistory 失败: %w", err)
		}
		out = unpackHistoryMessages(res)
		return nil
	}

	if client, ok, err := p.runningClient(ctx, acc.ID); err != nil {
		return nil, err
	} else if ok && client != nil {
		if err := run(ctx, client); err != nil {
			return nil, err
		}
		return out, nil
	}

	client, err := p.buildClient(acc, nil)
	if err != nil {
		return nil, err
	}
	if err := client.Run(ctx, func(ctx context.Context) error {
		if _, err := p.ensureAuthorized(ctx, client); err != nil {
			return err
		}
		return run(ctx, client)
	}); err != nil {
		return nil, err
	}
	return out, nil
}

func (p *Plugin) resolveInputPeer(ctx context.Context, accountID int64, src *domainsource.Source) (tg.InputPeerClass, error) {
	if p.deps.Peers == nil {
		return nil, fmt.Errorf("peer 缓存未装配，无法解析历史拉取目标")
	}
	peer, err := p.deps.Peers.Get(ctx, accountID, domainpeer.Type(src.PeerType), src.PeerID)
	if err != nil {
		return nil, err
	}
	if peer == nil {
		return nil, fmt.Errorf("peer 缓存缺失 account=%d type=%s id=%d，请先同步会话列表", accountID, src.PeerType, src.PeerID)
	}
	switch src.PeerType {
	case domainsource.PeerChannel:
		return &tg.InputPeerChannel{ChannelID: peer.PeerID, AccessHash: peer.AccessHash}, nil
	case domainsource.PeerChat:
		return &tg.InputPeerChat{ChatID: peer.PeerID}, nil
	case domainsource.PeerUser:
		return &tg.InputPeerUser{UserID: peer.PeerID, AccessHash: peer.AccessHash}, nil
	default:
		return nil, fmt.Errorf("不支持的 peer 类型: %s", src.PeerType)
	}
}

func unpackHistoryMessages(res tg.MessagesMessagesClass) []*tg.Message {
	var list []tg.MessageClass
	switch v := res.(type) {
	case *tg.MessagesMessages:
		list = v.Messages
	case *tg.MessagesMessagesSlice:
		list = v.Messages
	case *tg.MessagesChannelMessages:
		list = v.Messages
	default:
		return nil
	}
	out := make([]*tg.Message, 0, len(list))
	for _, m := range list {
		if msg, ok := m.(*tg.Message); ok {
			out = append(out, msg)
		}
	}
	return out
}

func toPreviewItems(msgs []*tg.Message) []HistoryPreviewItem {
	out := make([]HistoryPreviewItem, 0, len(msgs))
	// 预览按新到旧展示。
	sorted := append([]*tg.Message(nil), msgs...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID > sorted[j].ID })
	for _, msg := range sorted {
		nm := Normalize(0, msg, tg.Entities{})
		item := HistoryPreviewItem{
			ExternalMessageID: int64(msg.ID),
			MessageType:       nm.MessageType,
			Text:              truncateRunes(nm.Text, 200),
			SenderName:        nm.SenderName,
		}
		if nm.SentAt != nil {
			item.SentAtUnix = nm.SentAt.Unix()
		}
		out = append(out, item)
	}
	return out
}

func maxMsgID(msgs []*tg.Message) int64 {
	var max int64
	for _, m := range msgs {
		if int64(m.ID) > max {
			max = int64(m.ID)
		}
	}
	return max
}

func truncateRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

