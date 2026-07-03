package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureFileCreatesStarterConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "configs", "config.yaml")

	created, err := EnsureFile(path)
	if err != nil {
		t.Fatalf("EnsureFile failed: %v", err)
	}
	if !created {
		t.Fatal("expected config file to be created")
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	text := string(content)
	if !strings.Contains(text, "CHANGE_ME_DB_PASSWORD") {
		t.Fatal("starter config should keep database dsn as a placeholder")
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load generated config: %v", err)
	}
	if len(cfg.Security.EncryptionKey) != 32 {
		t.Fatalf("generated encryption key length = %d, want 32", len(cfg.Security.EncryptionKey))
	}
	if err := ValidateFileReady(path); err == nil {
		t.Fatal("generated config should require editing before service start")
	}
}

func TestEnsureFileKeepsExistingConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("database:\n  dsn: ok\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	created, err := EnsureFile(path)
	if err != nil {
		t.Fatalf("EnsureFile failed: %v", err)
	}
	if created {
		t.Fatal("existing config should not be overwritten")
	}
}
