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

type fakeArchive struct {
	msgBefore   time.Time
	msgLimit    int
	taskBefore  time.Time
	taskLimit   int
	msgDeleted  int64
	taskDeleted int64
	// expiredMsg / expiredTask 模拟「过期可删」计数；0 表示库中没有过期行。
	expiredMsg  int64
	expiredTask int64
}

func (f *fakeArchive) DeleteTerminalDeliveryTasksBefore(_ context.Context, before time.Time, limit int) (int64, error) {
	f.taskBefore = before
	f.taskLimit = limit
	if f.expiredTask <= 0 {
		return 0, nil
	}
	n := f.expiredTask
	if int64(limit) < n {
		n = int64(limit)
	}
	f.expiredTask -= n
	f.taskDeleted += n
	return n, nil
}

func (f *fakeArchive) DeleteMessagesBefore(_ context.Context, before time.Time, limit int) (int64, error) {
	f.msgBefore = before
	f.msgLimit = limit
	if f.expiredMsg <= 0 {
		return 0, nil
	}
	n := f.expiredMsg
	if int64(limit) < n {
		n = int64(limit)
	}
	f.expiredMsg -= n
	f.msgDeleted += n
	return n, nil
}

func TestRunArchiveCleanupZeroRetentionSkipsDelete(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, fileCfg())
	arch := &fakeArchive{expiredMsg: 10, expiredTask: 10}
	svc.SetArchiveStore(arch)
	fixed := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return fixed }

	if _, err := svc.UpdateDataRetention(context.Background(), DataRetentionSettings{
		MessagesRetentionDays:      0,
		DeliveryTasksRetentionDays: 0,
	}); err != nil {
		t.Fatal(err)
	}
	res, err := svc.RunArchiveCleanup(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.DeletedMessages != 0 || res.DeletedDeliveryTasks != 0 {
		t.Fatalf("保留天数 0 不应删除: %+v", res)
	}
	if arch.msgDeleted != 0 || arch.taskDeleted != 0 {
		t.Fatalf("fake 不应被调用删除: msg=%d task=%d", arch.msgDeleted, arch.taskDeleted)
	}
}

func TestRunArchiveCleanupDeletesExpiredKeepsFresh(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, fileCfg())
	arch := &fakeArchive{expiredMsg: 3, expiredTask: 2}
	svc.SetArchiveStore(arch)
	fixed := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return fixed }

	if _, err := svc.UpdateDataRetention(context.Background(), DataRetentionSettings{
		MessagesRetentionDays:      1,
		DeliveryTasksRetentionDays: 2,
	}); err != nil {
		t.Fatal(err)
	}
	res, err := svc.RunArchiveCleanup(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.DeletedMessages != 3 || res.DeletedDeliveryTasks != 2 {
		t.Fatalf("应删除过期行: %+v", res)
	}
	wantMsgCutoff := fixed.Add(-24 * time.Hour)
	wantTaskCutoff := fixed.Add(-48 * time.Hour)
	if !arch.msgBefore.Equal(wantMsgCutoff) {
		t.Fatalf("消息 cutoff = %v, want %v", arch.msgBefore, wantMsgCutoff)
	}
	if !arch.taskBefore.Equal(wantTaskCutoff) {
		t.Fatalf("投递 cutoff = %v, want %v", arch.taskBefore, wantTaskCutoff)
	}

	// 未过期：expired 计数为 0 时不删。
	arch2 := &fakeArchive{expiredMsg: 0, expiredTask: 0}
	svc.SetArchiveStore(arch2)
	res2, err := svc.RunArchiveCleanup(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res2.DeletedMessages != 0 || res2.DeletedDeliveryTasks != 0 {
		t.Fatalf("无过期行不应删除: %+v", res2)
	}
}

func TestUpdateDataRetentionValidation(t *testing.T) {
	svc := NewService(newFakeRepo(), fileCfg())
	if _, err := svc.UpdateDataRetention(context.Background(), DataRetentionSettings{MessagesRetentionDays: -1}); err == nil {
		t.Fatal("负数保留天数应报错")
	}
}

func TestRunDataCleanupUsesSelectedTargetsAndSavedRetention(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, fileCfg())
	arch := &fakeArchive{expiredMsg: 4, expiredTask: 7}
	svc.SetArchiveStore(arch)
	var aiDays int
	svc.SetAIRunsCleaner(func(_ context.Context, retentionDays int) (int64, error) {
		aiDays = retentionDays
		return 3, nil
	})
	if _, err := svc.UpdateDataRetention(context.Background(), DataRetentionSettings{
		MessagesRetentionDays:      5,
		DeliveryTasksRetentionDays: 6,
		AIRunsRetentionDays:        7,
	}); err != nil {
		t.Fatal(err)
	}

	result, err := svc.RunDataCleanup(context.Background(), DataCleanupTargets{Messages: true, AIRuns: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.DeletedMessages != 4 || result.DeletedDeliveryTasks != 0 || result.DeletedAIRuns != 3 {
		t.Fatalf("清理结果不符: %+v", result)
	}
	if arch.taskDeleted != 0 || aiDays != 7 || result.Source != SourceDatabase {
		t.Fatalf("应只清理选中目标并使用已保存保留期: tasks=%d aiDays=%d source=%s", arch.taskDeleted, aiDays, result.Source)
	}
}

func TestRunDataCleanupReportsBatchLimit(t *testing.T) {
	svc := NewService(newFakeRepo(), fileCfg())
	svc.SetArchiveStore(&fakeArchive{expiredMsg: int64(archiveBatchSize*maxArchiveRounds + 1)})
	result, err := svc.RunDataCleanup(context.Background(), DataCleanupTargets{Messages: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.DeletedMessages != int64(archiveBatchSize*maxArchiveRounds) {
		t.Fatalf("删除数量 = %d", result.DeletedMessages)
	}
	if len(result.LimitReached) != 1 || result.LimitReached[0] != "messages" {
		t.Fatalf("应报告消息达到单次上限: %+v", result.LimitReached)
	}
}

func TestRunMediaCleanupUsesEffectiveSettings(t *testing.T) {
	svc := NewService(newFakeRepo(), fileCfg())
	var gotRetention time.Duration
	var gotInput MediaCleanupInput
	svc.SetMediaCleaner(func(_ context.Context, retention time.Duration, in MediaCleanupInput) (MediaCleanupResult, error) {
		gotRetention = retention
		gotInput = in
		return MediaCleanupResult{DeletedLocalFiles: 2, DeletedRemoteObjects: 1}, nil
	})

	result, err := svc.RunMediaCleanup(context.Background(), MediaCleanupInput{DeleteLocal: true})
	if err != nil {
		t.Fatal(err)
	}
	if gotRetention != 168*time.Hour || !gotInput.DeleteLocal || gotInput.DeleteRemote {
		t.Fatalf("媒体清理参数不符: retention=%v input=%+v", gotRetention, gotInput)
	}
	if result.DeletedLocalFiles != 2 || result.DeletedRemoteObjects != 1 || result.RetentionHours != 168 {
		t.Fatalf("媒体清理结果不符: %+v", result)
	}
}

func TestRunMediaCleanupRejectsRemoteWhenS3Disabled(t *testing.T) {
	svc := NewService(newFakeRepo(), fileCfg())
	svc.SetMediaCleaner(func(context.Context, time.Duration, MediaCleanupInput) (MediaCleanupResult, error) {
		return MediaCleanupResult{}, nil
	})
	if _, err := svc.RunMediaCleanup(context.Background(), MediaCleanupInput{DeleteRemote: true}); err == nil {
		t.Fatal("S3 未启用时不应允许远端清理")
	}
}
