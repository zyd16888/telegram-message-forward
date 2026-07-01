// Package telegram 实现 Telegram Source 插件：登录态复用、同步 chats、监听 updates，
// 并把 gotd/td 原始消息标准化为 domain.NormalizedMessage。
//
// 本包与 internal/infra/telegram 是仅有的两处允许出现 gotd/td 类型的位置。
package telegram

import (
	"strconv"
	"strings"
	"time"

	"github.com/gotd/td/tg"

	domainmessage "telegram-message-forward/internal/domain/message"
)

// Normalize 把一条 Telegram 消息标准化为内部消息模型。
//
// sourceID 是命中的监听源 id；ent 提供 users/chats/channels 实体用于解析发送者与来源。
func Normalize(sourceID int64, msg *tg.Message, ent tg.Entities) *domainmessage.NormalizedMessage {
	nm := &domainmessage.NormalizedMessage{
		SourceID:          sourceID,
		ExternalMessageID: int64(msg.ID),
		Text:              msg.Message,
		MessageType:       messageType(msg),
		ReceivedAt:        time.Now(),
	}

	if g, ok := msg.GetGroupedID(); ok {
		nm.GroupedID = &g
	}
	if msg.Date != 0 {
		t := time.Unix(int64(msg.Date), 0).UTC()
		nm.SentAt = &t
	}

	fillSender(nm, msg, ent)
	nm.Media = extractMedia(msg)
	nm.OriginalURL = originalURL(msg, ent)

	return nm
}

// messageType 根据 media 判定消息类型，纯文本为 "text"。
func messageType(msg *tg.Message) string {
	media, ok := msg.GetMedia()
	if !ok || media == nil {
		return "text"
	}
	switch media.(type) {
	case *tg.MessageMediaPhoto:
		return "photo"
	case *tg.MessageMediaDocument:
		return "document"
	case *tg.MessageMediaWebPage:
		return "text"
	case *tg.MessageMediaGeo, *tg.MessageMediaGeoLive:
		return "geo"
	case *tg.MessageMediaContact:
		return "contact"
	case *tg.MessageMediaPoll:
		return "poll"
	default:
		return "media"
	}
}

// fillSender 解析发送者类型、id 与展示名。
func fillSender(nm *domainmessage.NormalizedMessage, msg *tg.Message, ent tg.Entities) {
	from, ok := msg.GetFromID()
	if !ok || from == nil {
		// 频道 post 无 FromID，发送者视为来源 peer 本身。
		from = msg.PeerID
	}
	switch p := from.(type) {
	case *tg.PeerUser:
		nm.SenderPeerType = "user"
		nm.SenderID = p.UserID
		if u, ok := ent.Users[p.UserID]; ok {
			nm.SenderName = userDisplayName(u)
		}
	case *tg.PeerChat:
		nm.SenderPeerType = "chat"
		nm.SenderID = p.ChatID
		if c, ok := ent.Chats[p.ChatID]; ok {
			nm.SenderName = c.Title
		}
	case *tg.PeerChannel:
		nm.SenderPeerType = "channel"
		nm.SenderID = p.ChannelID
		if c, ok := ent.Channels[p.ChannelID]; ok {
			nm.SenderName = c.Title
		}
	}
}

// userDisplayName 组合用户展示名，优先 first+last，退化到 username。
func userDisplayName(u *tg.User) string {
	name := strings.TrimSpace(strings.TrimSpace(u.FirstName) + " " + strings.TrimSpace(u.LastName))
	if name != "" {
		return name
	}
	if u.Username != "" {
		return "@" + u.Username
	}
	return strconv.FormatInt(u.ID, 10)
}

// extractMedia 提取媒体的轻量描述（v1 不下载文件，仅标注类型与元信息）。
func extractMedia(msg *tg.Message) []domainmessage.Media {
	media, ok := msg.GetMedia()
	if !ok || media == nil {
		return nil
	}
	switch m := media.(type) {
	case *tg.MessageMediaPhoto:
		return []domainmessage.Media{{Type: "photo"}}
	case *tg.MessageMediaDocument:
		item := domainmessage.Media{Type: "document"}
		if doc, ok := m.Document.(*tg.Document); ok {
			item.MimeType = doc.MimeType
			item.Size = doc.Size
			for _, attr := range doc.Attributes {
				if fn, ok := attr.(*tg.DocumentAttributeFilename); ok {
					item.FileName = fn.FileName
				}
			}
		}
		return []domainmessage.Media{item}
	default:
		return nil
	}
}

// originalURL 为带 username 的频道消息构造 t.me 链接。
func originalURL(msg *tg.Message, ent tg.Entities) string {
	pc, ok := msg.PeerID.(*tg.PeerChannel)
	if !ok {
		return ""
	}
	ch, ok := ent.Channels[pc.ChannelID]
	if !ok || ch.Username == "" {
		return ""
	}
	return "https://t.me/" + ch.Username + "/" + strconv.Itoa(msg.ID)
}
