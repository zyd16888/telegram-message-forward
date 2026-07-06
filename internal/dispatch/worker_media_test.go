package dispatch

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	domainmessage "telegram-message-forward/internal/domain/message"
	domainsink "telegram-message-forward/internal/domain/sink"
)

type fakeMediaStore struct {
	urls    map[string]string
	objects map[string]string
	err     error
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
func (f fakeMediaStore) Open(_ context.Context, key string) (io.ReadCloser, error) {
	if f.err != nil {
		return nil, f.err
	}
	data, ok := f.objects[key]
	if !ok {
		return nil, fmt.Errorf("object %s not found", key)
	}
	return io.NopCloser(strings.NewReader(data)), nil
}
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

func TestLocalUploadMediaMaterializesStorageKey(t *testing.T) {
	w := &Worker{log: slog.New(slog.DiscardHandler)}
	w.UseMediaStore(fakeMediaStore{objects: map[string]string{
		"telegram/source_1/10_0.docx": "fake-docx",
	}})

	caps := domainsink.Capabilities{
		SupportsFile: true,
		Media: []domainsink.MediaCapability{
			{Type: "file", Supported: true, MaxSizeMB: 20, RequiresUpload: true, SupportsBinary: true},
		},
	}
	in := []domainmessage.Media{{
		Type:       "document",
		FileName:   "report.docx",
		StorageKey: "telegram/source_1/10_0.docx",
		Size:       int64(len("fake-docx")),
	}}

	out, cleanup := w.localUploadMedia(context.Background(), caps, in)
	if out[0].LocalPath == "" {
		t.Fatal("需要上传的渠道应从 StorageKey 恢复本地临时文件")
	}
	data, err := os.ReadFile(out[0].LocalPath)
	if err != nil {
		t.Fatalf("恢复后的本地文件应可读: %v", err)
	}
	if string(data) != "fake-docx" {
		t.Fatalf("恢复内容 = %q, want fake-docx", string(data))
	}
	if !supportsAllMedia(caps, out) {
		t.Fatal("恢复 LocalPath 后文件媒体不应再降级")
	}

	path := out[0].LocalPath
	cleanup()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("投递后应清理临时文件，stat err=%v", err)
	}
}

func TestLocalUploadMediaMaterializesAudioAsFile(t *testing.T) {
	w := &Worker{log: slog.New(slog.DiscardHandler)}
	w.UseMediaStore(fakeMediaStore{objects: map[string]string{
		"telegram/source_1/11_0.mp3": "fake-mp3",
	}})

	caps := domainsink.Capabilities{
		SupportsFile: true,
		Media: []domainsink.MediaCapability{
			{Type: "audio", Supported: false},
			{Type: "file", Supported: true, MaxSizeMB: 20, RequiresUpload: true, SupportsBinary: true},
		},
	}
	in := []domainmessage.Media{{
		Type:       "audio",
		FileName:   "voice.mp3",
		MimeType:   "audio/mpeg",
		StorageKey: "telegram/source_1/11_0.mp3",
		Size:       int64(len("fake-mp3")),
	}}

	out, cleanup := w.localUploadMedia(context.Background(), caps, in)
	defer cleanup()
	if out[0].LocalPath == "" {
		t.Fatal("audio 可按 file 上传时应从 StorageKey 恢复本地临时文件")
	}
	if !supportsAllMedia(caps, out) {
		t.Fatal("audio 命中文件能力后不应再降级")
	}
}

func TestSupportsMediaItemFineGrained(t *testing.T) {
	doc := mustTempFile(t, "a-*.pdf", []byte("pdf"))
	img := mustTempFile(t, "a-*.jpg", []byte("jpg"))

	caps := domainsink.Capabilities{
		SupportsImage: true,
		SupportsFile:  true,
		Media: []domainsink.MediaCapability{
			{Type: "image", Supported: true, MaxSizeMB: 2, SupportsBinary: true},
			{Type: "file", Supported: true, MaxSizeMB: 20, RequiresUpload: true, SupportsBinary: true},
		},
	}

	if !supportsMediaItem(caps, domainmessage.Media{Type: "document", Size: 1024, LocalPath: doc}) {
		t.Fatal("有本地文件且未超限的 document 应支持")
	}
	if supportsMediaItem(caps, domainmessage.Media{Type: "document", Size: 30 * 1024 * 1024, LocalPath: doc}) {
		t.Fatal("超过渠道文件大小上限应降级")
	}
	if supportsMediaItem(caps, domainmessage.Media{Type: "document", Size: 1024}) {
		t.Fatal("渠道只认二进制且媒体无本地文件时应降级，避免静默丢弃")
	}
	if supportsMediaItem(caps, domainmessage.Media{Type: "photo", Size: 3 * 1024 * 1024, LocalPath: img}) {
		t.Fatal("图片超过渠道上限应降级")
	}
}

func TestSupportsMediaItemAudioFallsBackToFileCapability(t *testing.T) {
	mp3 := mustTempFile(t, "a-*.mp3", []byte("mp3"))
	caps := domainsink.Capabilities{
		SupportsFile: true,
		Media: []domainsink.MediaCapability{
			{Type: "audio", Supported: false},
			{Type: "file", Supported: true, MaxSizeMB: 20, RequiresUpload: true, SupportsBinary: true},
		},
	}

	if !supportsMediaItem(caps, domainmessage.Media{Type: "audio", FileName: "a.mp3", MimeType: "audio/mpeg", Size: 1024, LocalPath: mp3}) {
		t.Fatal("audio 原生不支持但 file 支持时，应按普通文件投递")
	}
	if supportsMediaItem(caps, domainmessage.Media{Type: "audio", FileName: "big.mp3", Size: 30 * 1024 * 1024, LocalPath: mp3}) {
		t.Fatal("audio 按 file 投递时仍应遵守文件大小上限")
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
	doc := mustTempFile(t, "a-*.pdf", []byte("pdf"))
	if supportsMediaItem(unsupported, domainmessage.Media{Type: "document", LocalPath: doc}) {
		t.Fatal("声明不支持 file 的渠道应降级")
	}
	if supportsMediaItem(domainsink.Capabilities{}, domainmessage.Media{Type: "sticker"}) {
		t.Fatal("未知媒体类型应降级")
	}
}

func mustTempFile(t *testing.T, pattern string, data []byte) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), pattern)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return f.Name()
}
