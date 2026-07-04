package telegram

import (
	"context"
	"strings"
	"testing"

	gotdtelegram "github.com/gotd/td/telegram"
	"github.com/gotd/td/tg"
)

// newTestClient 构造一个未连接的 gotd client，仅用于走到下载前的策略分支。
func newTestClient() *gotdtelegram.Client {
	return gotdtelegram.NewClient(1, "hash", gotdtelegram.Options{})
}

func newDocumentMessage(mimeType, fileName string, size int64) *tg.Message {
	media := &tg.MessageMediaDocument{}
	media.SetDocument(&tg.Document{
		ID: 9, AccessHash: 10, FileReference: []byte{1},
		MimeType: mimeType,
		Size:     size,
		Attributes: []tg.DocumentAttributeClass{
			&tg.DocumentAttributeFilename{FileName: fileName},
		},
	})
	msg := &tg.Message{ID: 100, PeerID: &tg.PeerChannel{ChannelID: 1}}
	msg.SetMedia(media)
	return msg
}

func TestDownloadPolicyAllowsFileType(t *testing.T) {
	unrestricted := DownloadPolicy{}
	if !unrestricted.allowsFileType("report.pdf", "application/pdf") {
		t.Fatal("空白名单应放行任意类型")
	}

	policy := DownloadPolicy{FileTypes: []string{"pdf", "docx"}}
	if !policy.allowsFileType("report.pdf", "application/pdf") {
		t.Fatal("pdf 应命中白名单")
	}
	if !policy.allowsFileType("", "application/pdf") {
		t.Fatal("无文件名时应按 MIME 推断扩展名命中白名单")
	}
	if policy.allowsFileType("setup.exe", "application/octet-stream") {
		t.Fatal("exe 不应命中白名单")
	}
}

func TestDownloadMessageMediaFileSwitchOff(t *testing.T) {
	msg := newDocumentMessage("application/pdf", "report.pdf", 1024)
	media := extractMedia(msg)

	out := downloadMessageMedia(context.Background(), newTestClient(), 1, msg, media, DownloadPolicy{}, false)
	if out[0].DownloadStatus != "skipped" {
		t.Fatalf("DownloadStatus = %q, want skipped", out[0].DownloadStatus)
	}
	if !strings.Contains(out[0].DownloadError, "未开启文件下载") {
		t.Fatalf("DownloadError = %q", out[0].DownloadError)
	}
}

func TestDownloadMessageMediaFileTypeNotAllowed(t *testing.T) {
	msg := newDocumentMessage("application/octet-stream", "setup.exe", 1024)
	media := extractMedia(msg)

	policy := DownloadPolicy{FileTypes: []string{"pdf"}}
	out := downloadMessageMedia(context.Background(), newTestClient(), 1, msg, media, policy, true)
	if out[0].DownloadStatus != "skipped" || !strings.Contains(out[0].DownloadError, "白名单") {
		t.Fatalf("media = %+v", out[0])
	}
}

func TestDownloadMessageMediaFileOverLimit(t *testing.T) {
	msg := newDocumentMessage("application/pdf", "big.pdf", 60*1024*1024)
	media := extractMedia(msg)

	policy := DownloadPolicy{FileMaxBytes: 50 * 1024 * 1024}
	out := downloadMessageMedia(context.Background(), newTestClient(), 1, msg, media, policy, true)
	if out[0].DownloadStatus != "skipped" || !strings.Contains(out[0].DownloadError, "超过下载上限") {
		t.Fatalf("media = %+v", out[0])
	}
}

func TestDownloadMessageMediaImageOverLimit(t *testing.T) {
	msg := newDocumentMessage("image/png", "huge.png", 30*1024*1024)
	media := extractMedia(msg)

	policy := DownloadPolicy{ImageMaxBytes: 20 * 1024 * 1024}
	// 图片不受 source 文件开关影响：downloadFiles=false 时仍按图片上限判定。
	out := downloadMessageMedia(context.Background(), newTestClient(), 1, msg, media, policy, false)
	if out[0].DownloadStatus != "skipped" || !strings.Contains(out[0].DownloadError, "图片超过下载上限") {
		t.Fatalf("media = %+v", out[0])
	}
}

func TestDownloadMessageMediaSkippedKeepsMetadata(t *testing.T) {
	msg := newDocumentMessage("application/pdf", "report.pdf", 1024)
	media := extractMedia(msg)

	out := downloadMessageMedia(context.Background(), newTestClient(), 1, msg, media, DownloadPolicy{}, false)
	got := out[0]
	if got.Type != "document" || got.FileName != "report.pdf" || got.Size != 1024 {
		t.Fatalf("跳过下载时应保留元数据: %+v", got)
	}
	if got.LocalPath != "" || got.StorageKey != "" {
		t.Fatalf("跳过下载不应有本地产物: %+v", got)
	}
}
