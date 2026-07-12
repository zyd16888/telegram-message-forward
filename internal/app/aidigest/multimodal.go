package aidigest

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"image/gif"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	domainaidigest "telegram-message-forward/internal/domain/aidigest"
	domainmessage "telegram-message-forward/internal/domain/message"
	"telegram-message-forward/internal/infra/ai"
)

func (s *Service) buildMultimodalContent(
	ctx context.Context,
	p *domainaidigest.Profile,
	messages []*domainmessage.NormalizedMessage,
) ([]ai.ContentPart, []domainaidigest.MediaAudit) {
	if p == nil || !p.Multimodal.Enabled {
		return nil, nil
	}
	cfg := normalizedMultimodal(p.Multimodal)
	parts := make([]ai.ContentPart, 0)
	audit := make([]domainaidigest.MediaAudit, 0)
	included := 0
	var totalBytes int64
	for messageIndex, message := range messages {
		for mediaIndex, media := range message.Media {
			if !isImageMedia(media) {
				continue
			}
			item := domainaidigest.MediaAudit{
				MessageID: message.ID, MediaIndex: mediaIndex, GroupedID: message.GroupedID,
				FileName: media.FileName, MimeType: media.MimeType, Size: media.Size,
			}
			if included >= cfg.MaxImagesPerRun {
				item.Status = "skipped"
				item.Reason = "超过单次图片数量上限"
				audit = append(audit, item)
				continue
			}
			if media.Size > cfg.MaxImageBytes {
				item.Status = "skipped"
				item.Reason = "超过单图字节上限"
				audit = append(audit, item)
				continue
			}
			data, mimeType, err := s.readImage(ctx, media, cfg.MaxImageBytes)
			if err != nil {
				item.Status = "failed"
				item.Reason = err.Error()
				audit = append(audit, item)
				continue
			}
			item.Size = int64(len(data))
			item.MimeType = mimeType
			if totalBytes+item.Size > cfg.MaxTotalImageBytes {
				item.Status = "skipped"
				item.Reason = "超过单次图片总字节上限"
				audit = append(audit, item)
				continue
			}
			hash := sha256.Sum256(data)
			item.SHA256 = hex.EncodeToString(hash[:])
			item.Status = "included"
			audit = append(audit, item)
			parts = append(parts,
				ai.ContentPart{Type: "text", Text: imageContextLabel(messageIndex+1, message, mediaIndex)},
				ai.ContentPart{
					Type: "image", Detail: cfg.ImageDetail,
					ImageURL: "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data),
				},
			)
			included++
			totalBytes += item.Size
		}
	}
	return parts, audit
}

func (s *Service) readImage(ctx context.Context, media domainmessage.Media, maxBytes int64) ([]byte, string, error) {
	var reader io.ReadCloser
	var err error
	if s.media != nil && strings.TrimSpace(media.StorageKey) != "" {
		reader, err = s.media.Open(ctx, media.StorageKey)
	} else if strings.TrimSpace(media.LocalPath) != "" {
		reader, err = os.Open(filepath.Clean(media.LocalPath))
	} else {
		return nil, "", errors.New("图片没有可读取的存储内容")
	}
	if err != nil {
		return nil, "", errors.New("读取图片失败")
	}
	defer reader.Close()

	data, err := io.ReadAll(io.LimitReader(reader, maxBytes+1))
	if err != nil {
		return nil, "", errors.New("读取图片失败")
	}
	if int64(len(data)) > maxBytes {
		return nil, "", errors.New("超过单图字节上限")
	}
	if len(data) == 0 {
		return nil, "", errors.New("图片内容为空")
	}
	mimeType := normalizeImageMIME(media.MimeType, data)
	if mimeType == "" {
		return nil, "", errors.New("不支持的图片格式")
	}
	if mimeType == "image/gif" {
		decoded, err := gif.DecodeAll(bytes.NewReader(data))
		if err != nil || len(decoded.Image) != 1 {
			return nil, "", errors.New("仅支持非动画 GIF")
		}
	}
	return data, mimeType, nil
}

func isImageMedia(media domainmessage.Media) bool {
	t := strings.ToLower(strings.TrimSpace(media.Type))
	return t == "image" || t == "photo" || strings.HasPrefix(strings.ToLower(media.MimeType), "image/")
}

func normalizeImageMIME(declared string, data []byte) string {
	detected := strings.ToLower(strings.TrimSpace(strings.Split(http.DetectContentType(data), ";")[0]))
	declared = strings.ToLower(strings.TrimSpace(strings.Split(declared, ";")[0]))
	if supportedImageMIME(detected) {
		return detected
	}
	if detected == "application/octet-stream" && supportedImageMIME(declared) {
		return declared
	}
	return ""
}

func supportedImageMIME(value string) bool {
	switch value {
	case "image/jpeg", "image/png", "image/webp", "image/gif":
		return true
	default:
		return false
	}
}

func imageContextLabel(index int, message *domainmessage.NormalizedMessage, mediaIndex int) string {
	group := ""
	if message.GroupedID != nil {
		group = fmt.Sprintf("，相册组 %d", *message.GroupedID)
	}
	caption := ""
	if mediaIndex < len(message.Media) && strings.TrimSpace(message.Media[mediaIndex].Caption) != "" {
		caption = "，图片说明：" + strings.TrimSpace(message.Media[mediaIndex].Caption)
	}
	return fmt.Sprintf("以下图片来自消息 [#%d]%s%s。只基于图片中可见内容分析。", index, group, caption)
}
