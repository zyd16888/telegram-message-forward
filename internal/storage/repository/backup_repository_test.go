package repository

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"gorm.io/datatypes"

	"telegram-message-forward/internal/config"
	"telegram-message-forward/internal/infra/crypto"
	"telegram-message-forward/internal/storage"
	"telegram-message-forward/internal/storage/model"
)

func TestRemapJSONIDs(t *testing.T) {
	got := remapJSONIDs(datatypes.JSON([]byte(`[1,2,9]`)), map[int64]int64{1: 11, 2: 22})
	var ids []int64
	if err := json.Unmarshal(got, &ids); err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 || ids[0] != 11 || ids[1] != 22 {
		t.Fatalf("ids = %v", ids)
	}
}

func TestBackupRoundTripInTransaction(t *testing.T) {
	if os.Getenv("TMF_RUN_DB_TESTS") == "" {
		t.Skip("设置 TMF_RUN_DB_TESTS=1 运行数据库集成测试")
	}
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("未找到 go.mod")
		}
		dir = parent
	}
	cfg, err := config.Load(filepath.Join(dir, "configs", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	db, err := storage.Open(cfg.Database)
	if err != nil {
		t.Fatal(err)
	}
	cipher, err := crypto.NewCipher([]byte(cfg.Security.EncryptionKey))
	if err != nil {
		t.Fatal(err)
	}
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	repo := NewBackupRepository(tx, cipher)
	payload, manifest, err := repo.ExportPayload(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := repo.InspectPayload(context.Background(), payload)
	if err != nil {
		t.Fatal(err)
	}
	if !preview.CanRestore || preview.Manifest.InstallationID != manifest.InstallationID {
		t.Fatalf("preview = %#v", preview)
	}
	result, err := repo.RestorePayload(context.Background(), payload)
	if err != nil {
		t.Fatal(err)
	}
	if !result.RestartRequired {
		t.Fatal("restore should require restart")
	}
}

func TestMappedPtrDropsMissingReference(t *testing.T) {
	id := int64(7)
	if got := mappedPtr(&id, map[int64]int64{}); got != nil {
		t.Fatalf("got = %v", *got)
	}
	if got := mappedPtr(&id, map[int64]int64{7: 70}); got == nil || *got != 70 {
		t.Fatalf("got = %v", got)
	}
}

func TestBackupSinkReencryptsConfigForTargetInstallation(t *testing.T) {
	sourceCipher, err := crypto.NewCipher([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	targetCipher, err := crypto.NewCipher([]byte("abcdef0123456789abcdef0123456789"))
	if err != nil {
		t.Fatal(err)
	}
	config := []byte(`{"url":"https://example.com/hook?token=secret"}`)
	configEncrypted, err := sourceCipher.Encrypt(config)
	if err != nil {
		t.Fatal(err)
	}
	secretEncrypted, err := sourceCipher.Encrypt([]byte("source-secret"))
	if err != nil {
		t.Fatal(err)
	}
	sourceRepo := NewBackupRepository(nil, sourceCipher)
	item, err := sourceRepo.backupSink(model.Sink{ConfigEncrypted: configEncrypted, SecretEncrypted: secretEncrypted})
	if err != nil {
		t.Fatal(err)
	}
	if len(item.Sink.ConfigEncrypted) != 0 || string(item.Sink.Config) != string(config) {
		t.Fatalf("备份载荷应去除实例密文并保留配置明文: %+v", item)
	}
	targetRepo := NewBackupRepository(nil, targetCipher)
	restored, err := targetRepo.restoreSink(item)
	if err != nil {
		t.Fatal(err)
	}
	gotConfig, err := targetCipher.Decrypt(restored.ConfigEncrypted)
	if err != nil {
		t.Fatal(err)
	}
	gotSecret, err := targetCipher.Decrypt(restored.SecretEncrypted)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotConfig) != string(config) || string(gotSecret) != "source-secret" || string(restored.Config) != "{}" {
		t.Fatalf("目标实例重加密结果不正确: config=%s secret=%s stored=%s", gotConfig, gotSecret, restored.Config)
	}
}

func TestRestoreSinkAcceptsLegacyPlainConfigBackup(t *testing.T) {
	cipher, err := crypto.NewCipher([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	repo := NewBackupRepository(nil, cipher)
	restored, err := repo.restoreSink(backupSink{
		Sink:   model.Sink{Config: datatypes.JSON([]byte(`{"url":"https://example.com/legacy"}`))},
		Secret: "legacy-secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	config, err := cipher.Decrypt(restored.ConfigEncrypted)
	if err != nil {
		t.Fatal(err)
	}
	if string(config) != `{"url":"https://example.com/legacy"}` {
		t.Fatalf("旧备份配置恢复错误: %s", config)
	}
}
