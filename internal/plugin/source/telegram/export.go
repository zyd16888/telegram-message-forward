package telegram

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gotd/td/telegram"
	"github.com/gotd/td/tg"

	domainaccount "telegram-message-forward/internal/domain/account"
	domainarchive "telegram-message-forward/internal/domain/chatarchive"
	domainmessage "telegram-message-forward/internal/domain/message"
	domainpeer "telegram-message-forward/internal/domain/peer"
)

// MediaPersister 把下载到临时目录的媒体收编进媒体存储，返回 storage key。
// 由应用层注入，避免 plugin 直连 mediastore。
type MediaPersister func(ctx context.Context, localPath string, m *domainarchive.Media) (string, error)

// ExportOptions 是一次会话导出拉取的参数。
type ExportOptions struct {
	// ArchiveID 用作媒体临时目录的命名空间，避免与实时监听下载互相覆盖。
	ArchiveID int64

	PeerType domainarchive.PeerType
	PeerID   int64

	// FromDate / ToDate 是可选时间窗，左闭右开。
	FromDate *time.Time
	ToDate   *time.Time

	// MaxMessages 是本次拉取的条数硬顶，0 表示不限。
	MaxMessages int

	// OffsetID 是断点续传游标：从该消息 id 继续向更早翻页，0 表示从最新开始。
	OffsetID int64

	// IncludeMedia 为 false 时只记录媒体元信息，不下载文件。
	// 私聊多年的媒体可能有几十 GB，默认不下载。
	IncludeMedia  bool
	MediaMaxBytes int64
	PersistMedia  MediaPersister
}

// ExportPage 是一页导出结果。
//
// Done 与 NextOffsetID 组合表达进度：
//   - Done=false                  ：还有下一页，本次调用会继续
//   - Done=true, NextOffsetID==0  ：已拉到会话末尾或时间窗下界，真正完成
//   - Done=true, NextOffsetID>0   ：本次停在中途（命中条数上限或翻页次数上限），
//     调用方应把 NextOffsetID 存为断点，下次续传
type ExportPage struct {
	Messages     []*domainarchive.Message
	NextOffsetID int64
	Done         bool
	// MediaDownloaded 是本页实际下载成功的媒体数。
	MediaDownloaded int
}

// ExportHistory 分页拉取一个会话的历史消息，每页回调一次 visit。
//
// 采用回调而非返回全量切片：私聊几年的记录可能几万条，全量驻留内存不可接受。
// 应用层在每页回调里落库并推进断点游标，中断后可从 NextOffsetID 续传。
//
// 本方法不走 ingest：归档不进 Flow、不产生投递任务。
func (p *Plugin) ExportHistory(
	ctx context.Context,
	acc *domainaccount.Account,
	opts ExportOptions,
	visit func(context.Context, ExportPage) error,
) error {
	if acc == nil {
		return fmt.Errorf("账号为空")
	}
	if visit == nil {
		return fmt.Errorf("导出缺少页回调")
	}
	if !opts.PeerType.Valid() {
		return fmt.Errorf("不支持的 peer 类型: %s", opts.PeerType)
	}
	inputPeer, err := p.resolveInputPeerByRef(ctx, acc.ID, string(opts.PeerType), opts.PeerID)
	if err != nil {
		return err
	}

	return p.withClient(ctx, acc, func(ctx context.Context, client *telegram.Client) error {
		offsetID := opts.OffsetID
		// 仅在从头开始时用 OffsetDate 定位时间窗上界；续传时以 OffsetID 为准。
		offsetDate := 0
		if offsetID == 0 && opts.ToDate != nil {
			offsetDate = int(opts.ToDate.Unix())
		}

		total := 0
		for page := 0; page < historyMaxPages; page++ {
			if err := ctx.Err(); err != nil {
				return err
			}
			size := historyPageSize
			if opts.MaxMessages > 0 && opts.MaxMessages-total < size {
				size = opts.MaxMessages - total
			}
			if size <= 0 {
				// 条数上限用尽，不是到底：带上断点游标供续传。
				return visit(ctx, ExportPage{Done: true, NextOffsetID: offsetID})
			}

			got, err := p.getExportPage(ctx, client, inputPeer, offsetID, offsetDate, size)
			if err != nil {
				return err
			}
			if len(got.Raw) == 0 {
				return visit(ctx, ExportPage{Done: true})
			}
			// OffsetDate 只用于首次定位，后续翻页一律按 id 游标。
			offsetDate = 0

			next := got.minID()
			// 游标必须严格前进，否则停止以免死循环。
			cursorStuck := next <= 0 || (offsetID > 0 && next >= offsetID)

			out := ExportPage{Messages: make([]*domainarchive.Message, 0, len(got.Raw))}
			reachedLowerBound := false
			for _, raw := range got.Raw {
				m := toArchiveMessage(raw, got.Entities)
				if m == nil {
					continue
				}
				// 越过时间窗下界即可收工：分页是按 id 倒序的，更早的只会更旧。
				if opts.FromDate != nil && m.Date != nil && m.Date.Before(*opts.FromDate) {
					reachedLowerBound = true
					continue
				}
				if opts.ToDate != nil && m.Date != nil && !m.Date.Before(*opts.ToDate) {
					continue
				}
				out.Messages = append(out.Messages, m)
			}

			if opts.IncludeMedia {
				out.MediaDownloaded = p.downloadArchiveMedia(ctx, client, got.messages(), out.Messages, opts)
			}

			// 按时间升序交付，便于下游按时间线落库与渲染。
			sort.Slice(out.Messages, func(i, j int) bool {
				return out.Messages[i].MessageID < out.Messages[j].MessageID
			})
			total += len(out.Messages)

			exhausted := opts.MaxMessages > 0 && total >= opts.MaxMessages
			// reachedLowerBound / cursorStuck 是真正到底；exhausted 只是本次配额用完。
			out.Done = reachedLowerBound || cursorStuck || exhausted
			if !out.Done || exhausted {
				out.NextOffsetID = next
			}
			if reachedLowerBound || cursorStuck {
				out.NextOffsetID = 0
			}
			if err := visit(ctx, out); err != nil {
				return err
			}
			if out.Done {
				return nil
			}
			offsetID = next
			if err := sleepCtx(ctx, historyPageDelay); err != nil {
				return err
			}
		}
		// 翻页次数触顶：不是到底，带上断点游标交由调用方续传。
		// 少了这个游标，调用方会把没拉完的任务误判为已完成。
		return visit(ctx, ExportPage{Done: true, NextOffsetID: offsetID})
	})
}

// getExportPage 取一页导出数据。
func (p *Plugin) getExportPage(
	ctx context.Context,
	client *telegram.Client,
	peer tg.InputPeerClass,
	offsetID int64,
	offsetDate int,
	size int,
) (historyPage, error) {
	res, err := client.API().MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
		Peer:       peer,
		OffsetID:   int(offsetID),
		OffsetDate: offsetDate,
		Limit:      size,
	})
	if err != nil {
		return historyPage{}, fmt.Errorf("MessagesGetHistory 失败: %w", err)
	}
	return unpackHistoryPage(res), nil
}

// downloadArchiveMedia 下载本页媒体并回填 storage key，返回成功条数。
// 单条失败不阻断整页：归档以文本为主，媒体缺失是可接受的降级。
func (p *Plugin) downloadArchiveMedia(
	ctx context.Context,
	client *telegram.Client,
	raw []*tg.Message,
	archived []*domainarchive.Message,
	opts ExportOptions,
) int {
	if opts.PersistMedia == nil {
		return 0
	}
	byID := make(map[int64]*domainarchive.Message, len(archived))
	for _, m := range archived {
		byID[m.MessageID] = m
	}

	policy := p.downloadPolicy()
	if opts.MediaMaxBytes > 0 {
		policy.ImageMaxBytes = opts.MediaMaxBytes
		policy.FileMaxBytes = opts.MediaMaxBytes
	}

	downloaded := 0
	for _, msg := range raw {
		target, ok := byID[int64(msg.ID)]
		if !ok || len(target.Media) == 0 {
			continue
		}
		// 复用实时链路的下载实现（含大小上限、重试与降级）。
		seed := make([]domainmessage.Media, 0, len(target.Media))
		for _, m := range target.Media {
			seed = append(seed, domainmessage.Media{
				Type: m.Type, FileName: m.FileName, MimeType: m.MimeType, Size: m.Size,
				Width: m.Width, Height: m.Height, Caption: m.Caption,
			})
		}
		got := downloadMessageMedia(ctx, client, archiveNamespace(opts.ArchiveID), msg, seed, policy, true)
		for i := range got {
			if i >= len(target.Media) || got[i].LocalPath == "" || got[i].DownloadStatus == "failed" {
				continue
			}
			key, err := opts.PersistMedia(ctx, got[i].LocalPath, &target.Media[i])
			if err != nil {
				p.deps.Log.Warn("归档媒体收编失败，保留元信息继续",
					"message_id", msg.ID, "media_type", target.Media[i].Type, "err", err)
				continue
			}
			target.Media[i].StorageKey = key
			target.Media[i].Downloaded = true
			downloaded++
		}
	}
	return downloaded
}

// resolveInputPeerByRef 从 peer 缓存解析 InputPeer。
// 与 resolveInputPeer 的区别：不要求该会话已被配置成监听源。
func (p *Plugin) resolveInputPeerByRef(ctx context.Context, accountID int64, peerType string, peerID int64) (tg.InputPeerClass, error) {
	if p.deps.Peers == nil {
		return nil, fmt.Errorf("peer 缓存未装配，无法解析目标会话")
	}
	peer, err := p.deps.Peers.Get(ctx, accountID, domainpeer.Type(peerType), peerID)
	if err != nil {
		return nil, err
	}
	if peer == nil {
		return nil, fmt.Errorf("peer 缓存缺失 account=%d type=%s id=%d，请先同步会话列表", accountID, peerType, peerID)
	}
	switch peerType {
	case string(domainarchive.PeerChannel):
		return &tg.InputPeerChannel{ChannelID: peer.PeerID, AccessHash: peer.AccessHash}, nil
	case string(domainarchive.PeerChat):
		return &tg.InputPeerChat{ChatID: peer.PeerID}, nil
	case string(domainarchive.PeerUser):
		return &tg.InputPeerUser{UserID: peer.PeerID, AccessHash: peer.AccessHash}, nil
	default:
		return nil, fmt.Errorf("不支持的 peer 类型: %s", peerType)
	}
}

// toArchiveMessage 把原始消息映射为归档消息。
//
// 不复用 Normalize：NormalizedMessage 是为转发裁剪过的窄模型，
// 缺少回复链、方向、转发来源、服务消息等分析必需字段。
func toArchiveMessage(raw tg.MessageClass, ent tg.Entities) *domainarchive.Message {
	switch msg := raw.(type) {
	case *tg.Message:
		return archiveFromMessage(msg, ent)
	case *tg.MessageService:
		return archiveFromService(msg, ent)
	default:
		return nil
	}
}

func archiveFromMessage(msg *tg.Message, ent tg.Entities) *domainarchive.Message {
	m := &domainarchive.Message{
		MessageID:   int64(msg.ID),
		Out:         msg.Out,
		MessageType: messageType(msg),
		Text:        msg.Message,
		Views:       msg.Views,
		Entities:    archiveEntities(msg.Entities),
		Media:       archiveMedia(msg),
		Reactions:   archiveReactions(msg),
	}
	if g, ok := msg.GetGroupedID(); ok {
		m.GroupedID = &g
	}
	if reply, ok := msg.GetReplyTo(); ok {
		if header, ok := reply.(*tg.MessageReplyHeader); ok {
			m.ReplyToMessageID = int64(header.ReplyToMsgID)
		}
	}
	if msg.Date != 0 {
		t := time.Unix(int64(msg.Date), 0).UTC()
		m.Date = &t
	}
	if msg.EditDate != 0 {
		t := time.Unix(int64(msg.EditDate), 0).UTC()
		m.EditDate = &t
	}
	if fwd, ok := msg.GetFwdFrom(); ok {
		m.Forward = archiveForward(fwd, ent)
	}
	fillArchiveSender(m, msg.FromID, msg.PeerID, ent)
	return m
}

func archiveFromService(msg *tg.MessageService, ent tg.Entities) *domainarchive.Message {
	m := &domainarchive.Message{
		MessageID:     int64(msg.ID),
		Out:           msg.Out,
		MessageType:   "service",
		ServiceAction: serviceActionName(msg.Action),
	}
	if reply, ok := msg.GetReplyTo(); ok {
		if header, ok := reply.(*tg.MessageReplyHeader); ok {
			m.ReplyToMessageID = int64(header.ReplyToMsgID)
		}
	}
	if msg.Date != 0 {
		t := time.Unix(int64(msg.Date), 0).UTC()
		m.Date = &t
	}
	fillArchiveSender(m, msg.FromID, msg.PeerID, ent)
	return m
}

// serviceActionName 把 TL 类型名转成可读的动作标识，如
// "messageActionChatAddUser" -> "chat_add_user"。
func serviceActionName(action tg.MessageActionClass) string {
	if action == nil {
		return "unknown"
	}
	name := camelToSnake(strings.TrimPrefix(action.TypeName(), "messageAction"))
	if name == "" {
		return "unknown"
	}
	return name
}

// camelToSnake 把 TL 的 CamelCase 类型名转成 snake_case，
// 如 ChatAddUser -> chat_add_user、TextUrl -> text_url。
func camelToSnake(name string) string {
	var b strings.Builder
	for i, r := range name {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				b.WriteByte('_')
			}
			b.WriteRune(r + ('a' - 'A'))
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// fillArchiveSender 解析发送者。频道 post 无 FromID，发送者视为来源 peer 本身。
func fillArchiveSender(m *domainarchive.Message, from, peer tg.PeerClass, ent tg.Entities) {
	if from == nil {
		from = peer
	}
	switch p := from.(type) {
	case *tg.PeerUser:
		m.SenderPeerType = "user"
		m.SenderID = p.UserID
		if u, ok := ent.Users[p.UserID]; ok {
			m.SenderName = userDisplayName(u)
			m.SenderUsername = u.Username
		}
	case *tg.PeerChat:
		m.SenderPeerType = "chat"
		m.SenderID = p.ChatID
		if c, ok := ent.Chats[p.ChatID]; ok {
			m.SenderName = c.Title
		}
	case *tg.PeerChannel:
		m.SenderPeerType = "channel"
		m.SenderID = p.ChannelID
		if c, ok := ent.Channels[p.ChannelID]; ok {
			m.SenderName = c.Title
			m.SenderUsername = c.Username
		}
	}
}

func archiveForward(fwd tg.MessageFwdHeader, ent tg.Entities) *domainarchive.Forward {
	out := &domainarchive.Forward{
		FromName:    fwd.FromName,
		ChannelPost: int64(fwd.ChannelPost),
	}
	if fwd.Date != 0 {
		t := time.Unix(int64(fwd.Date), 0).UTC()
		out.Date = &t
	}
	if from, ok := fwd.GetFromID(); ok {
		switch p := from.(type) {
		case *tg.PeerUser:
			out.FromID = p.UserID
			if u, ok := ent.Users[p.UserID]; ok && out.FromName == "" {
				out.FromName = userDisplayName(u)
			}
		case *tg.PeerChannel:
			out.FromID = p.ChannelID
			if c, ok := ent.Channels[p.ChannelID]; ok && out.FromName == "" {
				out.FromName = c.Title
			}
		case *tg.PeerChat:
			out.FromID = p.ChatID
			if c, ok := ent.Chats[p.ChatID]; ok && out.FromName == "" {
				out.FromName = c.Title
			}
		}
	}
	return out
}

// archiveEntities 保留富文本标记的原始 utf16 offset，供分析还原原文结构。
func archiveEntities(entities []tg.MessageEntityClass) []domainarchive.Entity {
	if len(entities) == 0 {
		return nil
	}
	out := make([]domainarchive.Entity, 0, len(entities))
	for _, e := range entities {
		item := domainarchive.Entity{
			Type:   camelToSnake(strings.TrimPrefix(e.TypeName(), "messageEntity")),
			Offset: e.GetOffset(),
			Length: e.GetLength(),
		}
		switch v := e.(type) {
		case *tg.MessageEntityTextURL:
			item.URL = v.URL
		case *tg.MessageEntityMentionName:
			item.UserID = v.UserID
		case *tg.MessageEntityPre:
			item.Language = v.Language
		}
		out = append(out, item)
	}
	return out
}

func archiveReactions(msg *tg.Message) []domainarchive.Reaction {
	reactions, ok := msg.GetReactions()
	if !ok || len(reactions.Results) == 0 {
		return nil
	}
	out := make([]domainarchive.Reaction, 0, len(reactions.Results))
	for _, r := range reactions.Results {
		emoji, ok := r.Reaction.(*tg.ReactionEmoji)
		if !ok {
			continue
		}
		out = append(out, domainarchive.Reaction{Emoticon: emoji.Emoticon, Count: r.Count})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// archiveMedia 提取媒体元信息。默认不下载文件，Downloaded 保持 false。
func archiveMedia(msg *tg.Message) []domainarchive.Media {
	media, ok := msg.GetMedia()
	if !ok || media == nil {
		return nil
	}
	switch m := media.(type) {
	case *tg.MessageMediaPhoto:
		item := domainarchive.Media{Type: "photo", MimeType: "image/jpeg", Caption: msg.Message}
		if photo, ok := mediaPhoto(m); ok {
			if size, ok := largestPhotoSize(photo.Sizes); ok {
				item.Width = size.width
				item.Height = size.height
				item.Size = int64(size.size)
			}
		}
		return []domainarchive.Media{item}
	case *tg.MessageMediaDocument:
		doc, ok := mediaDocument(m)
		if !ok {
			return nil
		}
		item := domainarchive.Media{
			Type: "document", MimeType: doc.MimeType, Size: doc.Size, Caption: msg.Message,
		}
		for _, attr := range doc.Attributes {
			switch a := attr.(type) {
			case *tg.DocumentAttributeFilename:
				item.FileName = a.FileName
			case *tg.DocumentAttributeImageSize:
				item.Width = a.W
				item.Height = a.H
			case *tg.DocumentAttributeVideo:
				item.Type = "video"
				item.Width = a.W
				item.Height = a.H
				item.Duration = int(a.Duration)
			case *tg.DocumentAttributeAudio:
				item.Type = "audio"
				if a.Voice {
					item.Type = "voice"
				}
				item.Duration = a.Duration
			case *tg.DocumentAttributeSticker:
				item.Type = "sticker"
			}
		}
		if item.Type == "document" && isImageMIME(doc.MimeType) {
			item.Type = "image"
		}
		if item.FileName == "" {
			item.FileName = defaultFileName(item.Type, item.MimeType)
		}
		return []domainarchive.Media{item}
	case *tg.MessageMediaGeo, *tg.MessageMediaGeoLive:
		return []domainarchive.Media{{Type: "geo"}}
	case *tg.MessageMediaContact:
		return []domainarchive.Media{{Type: "contact"}}
	case *tg.MessageMediaPoll:
		return []domainarchive.Media{{Type: "poll"}}
	default:
		return nil
	}
}
