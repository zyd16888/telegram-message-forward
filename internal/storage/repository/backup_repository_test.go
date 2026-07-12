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
