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

// 下载上限内置默认值；正式值由设置页/配置文件通过 Deps.DownloadPolicy 提供。
const (
	defaultImageMaxDownloadBytes int64 = 20 * 1024 * 1024
	defaultFileMaxDownloadBytes  int64 = 50 * 1024 * 1024
	imageDownloadMaxAttempts           = 3
)

var (
	downloadRetryBaseDelay = 500 * time.Millisecond
	downloadMediaToPath    = func(ctx context.Context, client *gotdtelegram.Client, location tg.InputFileLocationClass, localPath string) error {
		_, err := client.Download(location).ToPath(ctx, localPath)
		return err
	}
)

// DownloadPolicy 是媒体下载策略（bootstrap 从系统设置转换注入，设置保存后热生效）。
type DownloadPolicy struct {
	// ImageMaxBytes 是图片下载大小上限，<=0 使用内置默认。
	ImageMaxBytes int64
	// FileMaxBytes 是文件下载大小上限，<=0 使用内置默认。
	FileMaxBytes int64
	// FileTypes 是文件扩展名白名单（小写、不带点）；空表示不限类型。
	FileTypes []string
}

func (p DownloadPolicy) imageMaxBytes() int64 {
	if p.ImageMaxBytes > 0 {
		return p.ImageMaxBytes
	}
	return defaultImageMaxDownloadBytes
}

func (p DownloadPolicy) fileMaxBytes() int64 {
	if p.FileMaxBytes > 0 {
		return p.FileMaxBytes
	}
	return defaultFileMaxDownloadBytes
}

// allowsFileType 判断文件是否命中扩展名白名单；白名单为空表示不限类型。
func (p DownloadPolicy) allowsFileType(fileName, mimeType string) bool {
	if len(p.FileTypes) == 0 {
		return true
	}
	ext := strings.ToLower(strings.TrimPrefix(safeExt(fileName, mimeType), "."))
	for _, allowed := range p.FileTypes {
		if ext == allowed {
			return true
		}
	}
	return false
}

// downloadMessageMedia 下载消息媒体二进制到临时目录。
//
// 图片（photo / image document）始终尝试下载；文件（PDF 等非图片 document）
// 仅在 source 开启 download_files 且命中大小/类型策略时下载。
// 音频、视频等复杂媒体暂不下载，保持元数据走降级链路。
func downloadMessageMedia(ctx context.Context, client *gotdtelegram.Client, sourceID int64, msg *tg.Message, media []domainmessage.Media, policy DownloadPolicy, downloadFiles bool) []domainmessage.Media {
	if len(media) == 0 || client == nil || msg == nil {
		return media
	}
	out := append([]domainmessage.Media(nil), media...)
	item := &out[0]

	location, kind := mediaDownloadLocation(msg)
	if location == nil {
		return out
	}

	switch kind {
	case "image":
		if item.Size > policy.imageMaxBytes() {
			item.DownloadStatus = "skipped"
			item.DownloadError = fmt.Sprintf("图片超过下载上限（%s > %s）", humanMB(item.Size), humanMB(policy.imageMaxBytes()))
			return out
		}
	case "file":
		if !downloadFiles {
			item.DownloadStatus = "skipped"
			item.DownloadError = "该监听源未开启文件下载"
			return out
		}
		if !policy.allowsFileType(item.FileName, item.MimeType) {
			item.DownloadStatus = "skipped"
			item.DownloadError = "文件类型不在下载白名单内"
			return out
		}
		if item.Size > policy.fileMaxBytes() {
			item.DownloadStatus = "skipped"
			item.DownloadError = fmt.Sprintf("文件超过下载上限（%s > %s）", humanMB(item.Size), humanMB(policy.fileMaxBytes()))
			return out
		}
	default:
		return out
	}

	now := time.Now().UTC()
	storageKey := mediaStorageKey(sourceID, msg.ID, 0, *item)
	localPath := filepath.Join(os.TempDir(), "telegram-message-forward", "media", filepath.FromSlash(storageKey))
	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		item.DownloadStatus = "failed"
		item.DownloadError = err.Error()
		return out
	}
	if err := downloadWithRetry(ctx, client, location, localPath, kind); err != nil {
		item.DownloadStatus = "failed"
		item.DownloadError = err.Error()
		return out
	}
	item.LocalPath = localPath
	item.StorageKey = storageKey
	item.DownloadStatus = "downloaded"
	item.CreatedAt = &now
	return out
}

func downloadWithRetry(ctx context.Context, client *gotdtelegram.Client, location tg.InputFileLocationClass, localPath, kind string) error {
	attempts := 1
	if kind == "image" {
		attempts = imageDownloadMaxAttempts
	}
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		if err := downloadMediaToPath(ctx, client, location, localPath); err != nil {
			lastErr = err
			_ = os.Remove(localPath)
			if attempt == attempts {
				break
			}
			if err := waitDownloadRetry(ctx, attempt); err != nil {
				return err
			}
			continue
		}
		return nil
	}
	if attempts > 1 && lastErr != nil {
		return fmt.Errorf("下载失败（已重试 %d 次）: %w", attempts, lastErr)
	}
	return lastErr
}

func waitDownloadRetry(ctx context.Context, attempt int) error {
	delay := time.Duration(attempt) * downloadRetryBaseDelay
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// mediaDownloadLocation 返回可下载媒体的位置与类别：image（照片或图片 document）、file（其它 document）。
func mediaDownloadLocation(msg *tg.Message) (tg.InputFileLocationClass, string) {
	media, ok := msg.GetMedia()
	if !ok || media == nil {
		return nil, ""
	}
	switch m := media.(type) {
	case *tg.MessageMediaPhoto:
		photo, ok := mediaPhoto(m)
		if !ok {
			return nil, ""
		}
		size, ok := largestPhotoSize(photo.Sizes)
		if !ok {
			return nil, ""
		}
		return &tg.InputPhotoFileLocation{
			ID:            photo.ID,
			AccessHash:    photo.AccessHash,
			FileReference: photo.FileReference,
			ThumbSize:     size.typ,
		}, "image"
	case *tg.MessageMediaDocument:
		doc, ok := mediaDocument(m)
		if !ok {
			return nil, ""
		}
		if isImageMIME(doc.MimeType) {
			return doc.AsInputDocumentFileLocation(), "image"
		}
		return doc.AsInputDocumentFileLocation(), "file"
	default:
		return nil, ""
	}
}

func humanMB(n int64) string {
	return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
}

func mediaStorageKey(sourceID int64, messageID int, index int, media domainmessage.Media) string {
	ext := strings.ToLower(safeExt(media.FileName, media.MimeType))
	if ext == ".jpe" {
		ext = ".jpg"
	}
	return fmt.Sprintf("telegram/source_%d/%d_%d%s", sourceID, messageID, index, ext)
}
