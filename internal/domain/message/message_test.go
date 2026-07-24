package message

import (
	"testing"
	"time"
)

func TestFingerprintIgnoresRuntimeOnlyFields(t *testing.T) {
	created := time.Unix(1, 0)
	base := &NormalizedMessage{
		ID: 1, SourceID: 2, ExternalMessageID: 3, MessageType: "text", Text: "正文",
		Links: []Link{{URL: "https://example.com", Title: "示例"}},
		Media: []Media{{
			Type: "image", FileName: "a.jpg", MimeType: "image/jpeg", Size: 12,
			Width: 10, Height: 20, Caption: "图", URL: "https://old", LocalPath: "old",
			DownloadStatus: "success", CreatedAt: &created,
		}},
		ReceivedAt: created,
	}
	changedRuntime := *base
	changedRuntime.ID = 99
	changedRuntime.ReceivedAt = time.Unix(9, 0)
	changedRuntime.Media = append([]Media(nil), base.Media...)
	changedRuntime.Media[0].URL = "https://new"
	changedRuntime.Media[0].LocalPath = "new"
	changedRuntime.Media[0].DownloadStatus = "failed"

	if Fingerprint(base) != Fingerprint(&changedRuntime) {
		t.Fatal("运行时存储和下载字段不应改变内容指纹")
	}
}

func TestFingerprintChangesWithDeliveredContent(t *testing.T) {
	base := &NormalizedMessage{MessageType: "text", Text: "old"}
	edited := *base
	edited.Text = "new"
	if Fingerprint(base) == Fingerprint(&edited) {
		t.Fatal("正文变化必须改变内容指纹")
	}
}
