package telegram

import (
	"context"
	"fmt"
	"sort"

	"github.com/gotd/td/telegram"
	"github.com/gotd/td/tg"

	domainaccount "telegram-message-forward/internal/domain/account"
	domainsource "telegram-message-forward/internal/domain/source"
	pluginsource "telegram-message-forward/internal/plugin/source"
)

const (
	defaultHistoryLimit = 50
	maxHistoryLimit     = 100
	// defaultCatchUpMaxTotal 是单次断线追平的条数硬顶。命中上限不会留下缺口：
	// 正向分页保证游标连续，下一次追平从断点继续。
	defaultCatchUpMaxTotal = 1000
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
	// Truncated 表示命中条数上限、本次未拉完；游标仍连续，下次可继续。
	Truncated bool
}

// HistoryBackfillEnabled 读取源级历史补拉开关，默认 false。
func HistoryBackfillEnabled(src *domainsource.Source) bool {
	if src == nil || src.Config == nil {
		return false
	}
	v, _ := src.Config["history_backfill_enabled"].(bool)
	return v
}

// HistoryBackfillLimit 读取单次手动回捞上限，默认 50，硬顶 100。
func HistoryBackfillLimit(src *domainsource.Source, override int) int {
	limit := defaultHistoryLimit
	if override > 0 {
		limit = override
	} else if src != nil {
		if v, ok := configInt(src, "history_backfill_limit"); ok {
			limit = v
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

// HistoryCatchUpMax 读取单次断线追平的条数硬顶，默认 1000。
func HistoryCatchUpMax(src *domainsource.Source) int {
	if src != nil {
		if v, ok := configInt(src, "history_catchup_max"); ok {
			return v
		}
	}
	return defaultCatchUpMaxTotal
}

// configInt 从 source config 读取正整数（JSON 反序列化后可能是 float64）。
func configInt(src *domainsource.Source, key string) (int, bool) {
	if src == nil || src.Config == nil {
		return 0, false
	}
	switch v := src.Config[key].(type) {
	case float64:
		if int(v) > 0 {
			return int(v), true
		}
	case int:
		if v > 0 {
			return v, true
		}
	case int64:
		if v > 0 {
			return int(v), true
		}
	}
	return 0, false
}

// PreviewHistory 拉取最近 limit 条历史消息预览，不投递、不推进游标。
func (p *Plugin) PreviewHistory(ctx context.Context, acc *domainaccount.Account, src *domainsource.Source, limit int) (*HistoryFetchResult, error) {
	if !HistoryBackfillEnabled(src) {
		return nil, fmt.Errorf("该源未开启历史补拉（history_backfill_enabled）")
	}
	limit = HistoryBackfillLimit(src, limit)
	msgs, ent, err := p.fetchHistoryBackward(ctx, acc, src, 0, limit)
	if err != nil {
		return nil, err
	}
	return &HistoryFetchResult{
		Items:      toPreviewItems(msgs, ent),
		Fetched:    len(msgs),
		MaxMessage: maxMsgID(msgs),
	}, nil
}

// ExecuteHistoryBackfill 拉取最近 limit 条历史并走 ingest 回调（幂等防重投）。
//
// 语义是「补投递最近 N 条」，不是「补齐断档」，因此不推进 last_message_id：
// 若在此推进游标，一个游标落后很久的源点一次回捞就会把游标顶到最新，
// 中间那段再也无法被自动追平补回。重复拉取的代价由 messages 唯一约束吸收。
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
	msgs, ent, err := p.fetchHistoryBackward(ctx, acc, src, minID, limit)
	if err != nil {
		return nil, err
	}
	// 按 ID 升序 ingest，保证顺序稳定。
	sort.Slice(msgs, func(i, j int) bool { return msgs[i].ID < msgs[j].ID })

	out := &HistoryFetchResult{Items: toPreviewItems(msgs, ent), Fetched: len(msgs)}
	err = p.withClient(ctx, acc, func(ctx context.Context, client *telegram.Client) error {
		for _, msg := range msgs {
			if err := p.ingestHistoryMessage(ctx, client, src, msg, ent, handler, true); err != nil {
				return err
			}
			out.Ingested++
			if int64(msg.ID) > out.MaxMessage {
				out.MaxMessage = int64(msg.ID)
			}
		}
		return nil
	})
	if err != nil {
		return out, err
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
	if handler == nil {
		return fmt.Errorf("历史补拉缺少 ingest handler")
	}
	maxTotal := HistoryCatchUpMax(src)
	res, err := p.catchUpForward(ctx, acc, src, src.LastMessageID, maxTotal, handler)
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
			"truncated", res.Truncated,
		)
		if res.Truncated {
			p.deps.Log.Warn("历史补拉命中单次条数上限，剩余部分将在下次追平继续",
				"source", src.ID, "max_total", maxTotal, "resume_from", res.MaxMessage)
		}
	}
	return nil
}

// fetchHistoryBackward 自最新一条向更早方向分页拉取，最多 total 条，只取 id > minID。
//
// 旧实现只发一次请求且 OffsetID 恒为 0。MinID 在 MTProto 里只是过滤下界，服务端
// 仍从最新往回返回，因此断档超过单页容量时中间那段永远拉不到。这里改为真正按游标翻页。
func (p *Plugin) fetchHistoryBackward(
	ctx context.Context,
	acc *domainaccount.Account,
	src *domainsource.Source,
	minID int64,
	total int,
) ([]*tg.Message, tg.Entities, error) {
	entities := emptyEntities()
	if acc == nil || src == nil {
		return nil, entities, fmt.Errorf("账号或源为空")
	}
	inputPeer, err := p.resolveInputPeer(ctx, acc.ID, src)
	if err != nil {
		return nil, entities, err
	}

	var out []*tg.Message
	err = p.withClient(ctx, acc, func(ctx context.Context, client *telegram.Client) error {
		offsetID := int64(0)
		for page := 0; page < historyMaxPages; page++ {
			if total > 0 && len(out) >= total {
				return nil
			}
			size := historyPageSize
			if total > 0 && total-len(out) < size {
				size = total - len(out)
			}
			got, err := p.getHistoryPage(ctx, client, inputPeer, historyQuery{
				OffsetID: offsetID, MinID: minID, PageSize: size,
			})
			if err != nil {
				return err
			}
			if len(got.Raw) == 0 {
				return nil
			}
			entities = mergeEntities(entities, got.Entities)
			out = append(out, got.messages()...)

			next := got.minID()
			// 游标必须严格前进，否则停止以免死循环。
			if next <= 0 || (offsetID > 0 && next >= offsetID) {
				return nil
			}
			offsetID = next
			if minID > 0 && next <= minID+1 {
				return nil
			}
			if err := sleepCtx(ctx, historyPageDelay); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, entities, err
	}
	if total > 0 && len(out) > total {
		out = out[:total]
	}
	return out, entities, nil
}

// catchUpForward 自 cursor 起按 id 升序连续补拉并逐条 ingest。
//
// 用 OffsetID=cursor + AddOffset=-size 的正向分页：每批都是紧邻游标的「最旧的一批」，
// 升序 ingest 后 last_message_id 连续前进。命中 maxTotal 上限时提前收工，游标依然连续，
// 下次追平从断点继续 —— 不会像「只取最新 N 条再把游标推到最新」那样留下永久缺口。
func (p *Plugin) catchUpForward(
	ctx context.Context,
	acc *domainaccount.Account,
	src *domainsource.Source,
	cursor int64,
	maxTotal int,
	handler pluginsource.Handler,
) (*HistoryFetchResult, error) {
	if acc == nil || src == nil {
		return nil, fmt.Errorf("账号或源为空")
	}
	inputPeer, err := p.resolveInputPeer(ctx, acc.ID, src)
	if err != nil {
		return nil, err
	}

	out := &HistoryFetchResult{MaxMessage: cursor}
	err = p.withClient(ctx, acc, func(ctx context.Context, client *telegram.Client) error {
		for page := 0; page < historyMaxPages; page++ {
			if maxTotal > 0 && out.Fetched >= maxTotal {
				out.Truncated = true
				return nil
			}
			size := historyPageSize
			if maxTotal > 0 && maxTotal-out.Fetched < size {
				size = maxTotal - out.Fetched
			}
			got, err := p.getHistoryPage(ctx, client, inputPeer, historyQuery{
				OffsetID: cursor, MinID: cursor, AddOffset: -size, PageSize: size,
			})
			if err != nil {
				return err
			}
			if len(got.Raw) == 0 {
				return nil
			}

			msgs := got.messages()
			sort.Slice(msgs, func(i, j int) bool { return msgs[i].ID < msgs[j].ID })
			out.Fetched += len(msgs)
			for _, msg := range msgs {
				if err := p.ingestHistoryMessage(ctx, client, src, msg, got.Entities, handler, false); err != nil {
					return err
				}
				out.Ingested++
				if int64(msg.ID) > out.MaxMessage {
					out.MaxMessage = int64(msg.ID)
				}
			}

			next := got.maxID()
			// 游标必须严格前进，否则停止以免死循环。
			if next <= cursor {
				return nil
			}
			cursor = next
			if err := sleepCtx(ctx, historyPageDelay); err != nil {
				return err
			}
		}
		out.Truncated = true
		return nil
	})
	if err != nil {
		return out, err
	}
	return out, nil
}

// ingestHistoryMessage 归一化单条历史消息并交给 ingest。
// skipCursor=true 时不推进 last_message_id（手动回捞路径）。
func (p *Plugin) ingestHistoryMessage(
	ctx context.Context,
	client *telegram.Client,
	src *domainsource.Source,
	msg *tg.Message,
	ent tg.Entities,
	handler pluginsource.Handler,
	skipCursor bool,
) error {
	nm := Normalize(src.ID, msg, ent)
	nm.SkipCursorAdvance = skipCursor
	// 补拉路径尽量补媒体；失败不阻断，由 downloadMessageMedia 内部降级。
	if client != nil {
		nm.Media = downloadMessageMedia(ctx, client, sourceNamespace(src.ID), msg, nm.Media, p.downloadPolicy(), sourceDownloadFiles(src))
	}
	if err := handler(ctx, nm); err != nil {
		return fmt.Errorf("ingest 历史消息 %d 失败: %w", msg.ID, err)
	}
	return nil
}

// resolveInputPeer 解析监听源对应的 InputPeer。
func (p *Plugin) resolveInputPeer(ctx context.Context, accountID int64, src *domainsource.Source) (tg.InputPeerClass, error) {
	return p.resolveInputPeerByRef(ctx, accountID, string(src.PeerType), src.PeerID)
}

func toPreviewItems(msgs []*tg.Message, ent tg.Entities) []HistoryPreviewItem {
	out := make([]HistoryPreviewItem, 0, len(msgs))
	// 预览按新到旧展示。
	sorted := append([]*tg.Message(nil), msgs...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID > sorted[j].ID })
	for _, msg := range sorted {
		nm := Normalize(0, msg, ent)
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
