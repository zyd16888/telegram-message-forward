package dispatch

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	domainmessage "telegram-message-forward/internal/domain/message"
	domainsink "telegram-message-forward/internal/domain/sink"
)

type fakeMediaStore struct {
	urls map[string]string
	err  error
}

func (f fakeMediaStore) PutFile(_ context.Context, _, srcPath string) (string, error) {
	return srcPath, nil
}
func (f fakeMediaStore) PublicURL(_ context.Context, key string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.urls[key], nil
}
func (f fakeMediaStore) Open(context.Context, string) (io.ReadCloser, error) { return nil, nil }
func (f fakeMediaStore) Cleanup(context.Context, time.Duration) (int, error) { return 0, nil }

func TestPublicMediaFillsURLFromStorageKey(t *testing.T) {
	w := &Worker{log: slog.New(slog.DiscardHandler)}
	w.UseMediaStore(fakeMediaStore{urls: map[string]string{
		"telegram/source_1/10_0.jpg": "https://tmf.example.com/media/telegram/source_1/10_0.jpg?e=1&s=x",
	}})

	in := []domainmessage.Media{
		{Type: "photo", StorageKey: "telegram/source_1/10_0.jpg", LocalPath: "C:/tmp/a.jpg"},
		{Type: "image", RemoteURL: "https://cdn.example.com/b.jpg", StorageKey: "other/key.jpg"},
		{Type: "document", StorageKey: ""},
	}
	out := w.publicMedia(context.Background(), in)

	if out[0].URL == "" {
		t.Fatal("有 StorageKey 且无公网地址的媒体应回填 URL")
	}
	if out[1].URL != "" {
		t.Fatal("已有 RemoteURL 的媒体不应再生成 URL")
	}
	if in[0].URL != "" {
		t.Fatal("publicMedia 不应修改原始切片")
	}
}

func TestPublicMediaErrorKeepsDelivering(t *testing.T) {
	w := &Worker{log: slog.New(slog.DiscardHandler)}
	w.UseMediaStore(fakeMediaStore{err: fmt.Errorf("s3 不可用")})

	in := []domainmessage.Media{{Type: "photo", StorageKey: "k.jpg"}}
	out := w.publicMedia(context.Background(), in)
	if len(out) != 1 || out[0].URL != "" {
		t.Fatal("生成 URL 失败时应保留媒体且不中断")
	}
}

func TestPublicMediaWithoutStore(t *testing.T) {
	w := &Worker{log: slog.New(slog.DiscardHandler)}
	in := []domainmessage.Media{{Type: "photo", StorageKey: "k.jpg"}}
	out := w.publicMedia(context.Background(), in)
	if out[0].URL != "" {
		t.Fatal("未注入存储时不应生成 URL")
	}
}

func TestSupportsMediaItemFineGrained(t *testing.T) {
	caps := domainsink.Capabilities{
		SupportsImage: true,
		SupportsFile:  true,
		Media: []domainsink.MediaCapability{
			{Type: "image", Supported: true, MaxSizeMB: 2, SupportsBinary: true},
			{Type: "file", Supported: true, MaxSizeMB: 20, RequiresUpload: true, SupportsBinary: true},
		},
	}

	if !supportsMediaItem(caps, domainmessage.Media{Type: "document", Size: 1024, LocalPath: "C:/tmp/a.pdf"}) {
		t.Fatal("有本地文件且未超限的 document 应支持")
	}
	if supportsMediaItem(caps, domainmessage.Media{Type: "document", Size: 30 * 1024 * 1024, LocalPath: "C:/tmp/a.pdf"}) {
		t.Fatal("超过渠道文件大小上限应降级")
	}
	if supportsMediaItem(caps, domainmessage.Media{Type: "document", Size: 1024}) {
		t.Fatal("渠道只认二进制且媒体无本地文件时应降级，避免静默丢弃")
	}
	if supportsMediaItem(caps, domainmessage.Media{Type: "photo", Size: 3 * 1024 * 1024, LocalPath: "C:/tmp/a.jpg"}) {
		t.Fatal("图片超过渠道上限应降级")
	}
}

func TestSupportsMediaItemFallsBackToCoarse(t *testing.T) {
	// 渠道未声明 file 细粒度条目时，回退粗粒度布尔（如 email/ntfy/webhook）。
	caps := domainsink.Capabilities{
		SupportsFile: true,
		Media: []domainsink.MediaCapability{
			{Type: "image", Supported: true, SupportsPublicURL: true},
		},
	}
	if !supportsMediaItem(caps, domainmessage.Media{Type: "document", Size: 1024}) {
		t.Fatal("无细粒度条目时应回退粗粒度 SupportsFile")
	}

	unsupported := domainsink.Capabilities{
		Media: []domainsink.MediaCapability{{Type: "file", Supported: false}},
	}
	if supportsMediaItem(unsupported, domainmessage.Media{Type: "document", LocalPath: "C:/tmp/a.pdf"}) {
		t.Fatal("声明不支持 file 的渠道应降级")
	}
	if supportsMediaItem(domainsink.Capabilities{}, domainmessage.Media{Type: "sticker"}) {
		t.Fatal("未知媒体类型应降级")
	}
}
