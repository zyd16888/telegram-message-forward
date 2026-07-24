// Package telegram 实现 Telegram Source 插件：登录态复用、同步 chats、监听 updates，
// 并把 gotd/td 原始消息标准化为 domain.NormalizedMessage。
//
// 本包与 internal/infra/telegram 是仅有的两处允许出现 gotd/td 类型的位置。
package telegram

import (
	"mime"
	"path/filepath"
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
	if editDate, ok := msg.GetEditDate(); ok && editDate != 0 {
		t := time.Unix(int64(editDate), 0).UTC()
		nm.EditedAt = &t
	}

	fillSender(nm, msg, ent)
	nm.Links = extractLinks(msg.Message, msg)
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
	switch m := media.(type) {
	case *tg.MessageMediaPhoto:
		return "photo"
	case *tg.MessageMediaDocument:
		if doc, ok := mediaDocument(m); ok && isImageMIME(doc.MimeType) {
			return "image"
		}
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

// extractMedia 提取媒体的轻量描述。下载由监听路径在拥有 Telegram client 时补充。
func extractMedia(msg *tg.Message) []domainmessage.Media {
	media, ok := msg.GetMedia()
	if !ok || media == nil {
		return nil
	}
	switch m := media.(type) {
	case *tg.MessageMediaPhoto:
		item := domainmessage.Media{Type: "photo", MimeType: "image/jpeg", Caption: msg.Message}
		if photo, ok := mediaPhoto(m); ok {
			if size, ok := largestPhotoSize(photo.Sizes); ok {
				item.Width = size.width
				item.Height = size.height
				item.Size = int64(size.size)
			}
		}
		return []domainmessage.Media{item}
	case *tg.MessageMediaDocument:
		doc, ok := mediaDocument(m)
		if !ok {
			return nil
		}
		item := domainmessage.Media{Type: "document", MimeType: doc.MimeType, Size: doc.Size, Caption: msg.Message}
		if isImageMIME(doc.MimeType) {
			item.Type = "image"
		}
		for _, attr := range doc.Attributes {
			switch a := attr.(type) {
			case *tg.DocumentAttributeFilename:
				item.FileName = a.FileName
			case *tg.DocumentAttributeImageSize:
				item.Width = a.W
				item.Height = a.H
			}
		}
		if item.FileName == "" {
			item.FileName = defaultFileName(item.Type, item.MimeType)
		}
		return []domainmessage.Media{item}
	default:
		return nil
	}
}

type photoSizeInfo struct {
	typ    string
	width  int
	height int
	size   int
}

func mediaPhoto(m *tg.MessageMediaPhoto) (*tg.Photo, bool) {
	if m == nil {
		return nil, false
	}
	photoClass, ok := m.GetPhoto()
	if !ok || photoClass == nil {
		return nil, false
	}
	return photoClass.AsNotEmpty()
}

func mediaDocument(m *tg.MessageMediaDocument) (*tg.Document, bool) {
	if m == nil {
		return nil, false
	}
	docClass, ok := m.GetDocument()
	if !ok || docClass == nil {
		return nil, false
	}
	return docClass.AsNotEmpty()
}

func largestPhotoSize(sizes []tg.PhotoSizeClass) (photoSizeInfo, bool) {
	var best photoSizeInfo
	for _, size := range sizes {
		switch s := size.(type) {
		case *tg.PhotoSize:
			if s.Size > best.size {
				best = photoSizeInfo{typ: s.Type, width: s.W, height: s.H, size: s.Size}
			}
		case *tg.PhotoSizeProgressive:
			sizeBytes := 0
			for _, candidate := range s.Sizes {
				if candidate > sizeBytes {
					sizeBytes = candidate
				}
			}
			if sizeBytes > best.size {
				best = photoSizeInfo{typ: s.Type, width: s.W, height: s.H, size: sizeBytes}
			}
		}
	}
	return best, best.typ != ""
}

func isImageMIME(mimeType string) bool {
	return strings.HasPrefix(strings.ToLower(mimeType), "image/")
}

func defaultFileName(mediaType, mimeType string) string {
	ext := ".bin"
	if exts, err := mime.ExtensionsByType(mimeType); err == nil && len(exts) > 0 {
		ext = exts[0]
	}
	if mediaType == "image" && ext == ".jpe" {
		ext = ".jpg"
	}
	return mediaType + ext
}

func safeExt(name, mimeType string) string {
	if ext := filepath.Ext(name); ext != "" {
		return ext
	}
	if exts, err := mime.ExtensionsByType(mimeType); err == nil && len(exts) > 0 {
		return exts[0]
	}
	return ".bin"
}

func extractLinks(text string, msg *tg.Message) []domainmessage.Link {
	entities, ok := msg.GetEntities()
	if !ok || len(entities) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	links := make([]domainmessage.Link, 0, len(entities))
	add := func(url, title string) {
		url = strings.TrimSpace(url)
		title = strings.TrimSpace(title)
		if url == "" {
			return
		}
		if _, ok := seen[url]; ok {
			return
		}
		seen[url] = struct{}{}
		links = append(links, domainmessage.Link{URL: url, Title: title})
	}

	for _, entity := range entities {
		switch e := entity.(type) {
		case *tg.MessageEntityTextURL:
			title, _ := utf16Slice(text, e.Offset, e.Length)
			add(e.URL, title)
		case *tg.MessageEntityURL:
			url, ok := utf16Slice(text, e.Offset, e.Length)
			if !ok {
				continue
			}
			add(url, url)
		}
	}
	if len(links) == 0 {
		return nil
	}
	return links
}

func utf16Slice(text string, offset, length int) (string, bool) {
	if offset < 0 || length <= 0 {
		return "", false
	}
	startUnit := offset
	endUnit := offset + length
	unit := 0
	startByte := -1
	endByte := -1
	for i, r := range text {
		if startByte < 0 && unit >= startUnit {
			startByte = i
		}
		unit += utf16RuneLen(r)
		if endByte < 0 && unit >= endUnit {
			endByte = i + len(string(r))
			break
		}
	}
	if startByte < 0 && startUnit == unit {
		startByte = len(text)
	}
	if endByte < 0 && endUnit == unit {
		endByte = len(text)
	}
	if startByte < 0 || endByte < startByte || endUnit > unit {
		return "", false
	}
	return text[startByte:endByte], true
}

func utf16RuneLen(r rune) int {
	if r >= 0x10000 {
		return 2
	}
	return 1
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
