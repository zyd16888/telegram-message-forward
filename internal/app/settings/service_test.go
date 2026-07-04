package settings

import (
	"context"
	"testing"
	"time"

	"telegram-message-forward/internal/config"
	domainsettings "telegram-message-forward/internal/domain/settings"
)

type fakeRepo struct {
	rows map[string]*domainsettings.Setting
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{rows: map[string]*domainsettings.Setting{}}
}

func (r *fakeRepo) Get(_ context.Context, key string) (*domainsettings.Setting, error) {
	return r.rows[key], nil
}

func (r *fakeRepo) Upsert(_ context.Context, s *domainsettings.Setting) error {
	cp := *s
	r.rows[s.Key] = &cp
	return nil
}

func fileCfg() config.MediaConfig {
	return config.MediaConfig{
		Dir:       "data/media",
		URLTTL:    24 * time.Hour,
		Retention: 168 * time.Hour,
		S3:        config.S3Config{UseSSL: true, SecretKey: "file-secret", Enabled: false},
	}
}

func TestEffectiveMediaFallsBackToFile(t *testing.T) {
	svc := NewService(newFakeRepo(), fileCfg())
	ms, secret, source, err := svc.EffectiveMedia(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if source != SourceFile {
		t.Fatalf("无数据库记录时来源应为 file, got %s", source)
	}
	if ms.Dir != "data/media" || ms.RetentionHours != 168 || secret != "file-secret" {
		t.Fatalf("文件默认值不符: %+v secret=%s", ms, secret)
	}
}

func TestUpdateMediaOverridesFileAndKeepsSecret(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, fileCfg())
	var reloaded *MediaSettings
	svc.SetMediaReloader(func(ms MediaSettings, s3Secret string) error {
		reloaded = &ms
		if s3Secret != "new-secret" {
			t.Fatalf("重载应拿到新 secret, got %s", s3Secret)
		}
		return nil
	})

	in := MediaSettings{
		Dir:            "  ",
		RetentionHours: 720,
		S3: S3Settings{
			Enabled:   true,
			Endpoint:  "s3.example.com",
			Bucket:    "media",
			AccessKey: "ak",
			KeyPrefix: "/tmf/",
		},
	}
	secret := "new-secret"
	saved, err := svc.UpdateMedia(context.Background(), in, &secret)
	if err != nil {
		t.Fatalf("UpdateMedia: %v", err)
	}
	if saved.Dir != "data/media" {
		t.Fatalf("空目录应回落默认值, got %q", saved.Dir)
	}
	if saved.S3.KeyPrefix != "tmf" {
		t.Fatalf("key_prefix 应去除首尾斜杠, got %q", saved.S3.KeyPrefix)
	}
	if reloaded == nil {
		t.Fatal("保存后应触发热重载")
	}

	// 再次读取：来源变为 database。
	ms, gotSecret, source, err := svc.EffectiveMedia(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if source != SourceDatabase || ms.RetentionHours != 720 || gotSecret != "new-secret" {
		t.Fatalf("数据库设置未生效: source=%s %+v secret=%s", source, ms, gotSecret)
	}

	// secret 传 nil：保留已有值。
	ms.S3.Endpoint = "s3.other.com"
	if _, err := svc.UpdateMedia(context.Background(), ms, nil); err != nil {
		t.Fatalf("UpdateMedia(nil secret): %v", err)
	}
	_, gotSecret, _, _ = svc.EffectiveMedia(context.Background())
	if gotSecret != "new-secret" {
		t.Fatalf("secret 传 nil 应保留原值, got %s", gotSecret)
	}
}

func TestUpdateMediaValidation(t *testing.T) {
	svc := NewService(newFakeRepo(), fileCfg())

	// 启用 S3 但缺少必填项。
	bad := MediaSettings{S3: S3Settings{Enabled: true}}
	empty := ""
	if _, err := svc.UpdateMedia(context.Background(), bad, &empty); err == nil {
		t.Fatal("启用 S3 缺少必填项应报错")
	}

	// 公网地址协议校验。
	bad2 := MediaSettings{PublicBaseURL: "tmf.example.com"}
	if _, err := svc.UpdateMedia(context.Background(), bad2, nil); err == nil {
		t.Fatal("公网地址缺少协议应报错")
	}
}
