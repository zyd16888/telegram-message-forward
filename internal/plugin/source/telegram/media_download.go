package telegram

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	gotdtelegram "github.com/gotd/td/telegram"
	"github.com/gotd/td/tg"

	domainmessage "telegram-message-forward/internal/domain/message"
)

const maxTelegramImageDownloadBytes int64 = 10 * 1024 * 1024

func downloadMessageImages(ctx context.Context, client *gotdtelegram.Client, sourceID int64, msg *tg.Message, media []domainmessage.Media) []domainmessage.Media {
	if len(media) == 0 || client == nil || msg == nil {
		return media
	}
	out := append([]domainmessage.Media(nil), media...)
	location, ok := imageDownloadLocation(msg)
	if !ok {
		return out
	}
	if out[0].Size > maxTelegramImageDownloadBytes {
		out[0].DownloadStatus = "skipped"
		out[0].DownloadError = "图片超过下载上限"
		return out
	}

	now := time.Now().UTC()
	storageKey := mediaStorageKey(sourceID, msg.ID, 0, out[0])
	localPath := filepath.Join(os.TempDir(), "telegram-message-forward", "media", filepath.FromSlash(storageKey))
	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		out[0].DownloadStatus = "failed"
		out[0].DownloadError = err.Error()
		return out
	}
	if _, err := client.Download(location).ToPath(ctx, localPath); err != nil {
		out[0].DownloadStatus = "failed"
		out[0].DownloadError = err.Error()
		return out
	}
	out[0].LocalPath = localPath
	out[0].StorageKey = storageKey
	out[0].DownloadStatus = "downloaded"
	out[0].CreatedAt = &now
	return out
}

func imageDownloadLocation(msg *tg.Message) (tg.InputFileLocationClass, bool) {
	media, ok := msg.GetMedia()
	if !ok || media == nil {
		return nil, false
	}
	switch m := media.(type) {
	case *tg.MessageMediaPhoto:
		photo, ok := mediaPhoto(m)
		if !ok {
			return nil, false
		}
		size, ok := largestPhotoSize(photo.Sizes)
		if !ok {
			return nil, false
		}
		return &tg.InputPhotoFileLocation{
			ID:            photo.ID,
			AccessHash:    photo.AccessHash,
			FileReference: photo.FileReference,
			ThumbSize:     size.typ,
		}, true
	case *tg.MessageMediaDocument:
		doc, ok := mediaDocument(m)
		if !ok || !isImageMIME(doc.MimeType) {
			return nil, false
		}
		return doc.AsInputDocumentFileLocation(), true
	default:
		return nil, false
	}
}

func mediaStorageKey(sourceID int64, messageID int, index int, media domainmessage.Media) string {
	ext := strings.ToLower(safeExt(media.FileName, media.MimeType))
	if ext == ".jpe" {
		ext = ".jpg"
	}
	return fmt.Sprintf("telegram/source_%d/%d_%d%s", sourceID, messageID, index, ext)
}
