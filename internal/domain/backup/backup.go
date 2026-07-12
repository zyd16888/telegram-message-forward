// Package backup 定义配置备份的存储契约与清单。
package backup

import (
	"context"
	"time"
)

const FormatVersion = 1

type Counts map[string]int

type Manifest struct {
	Version         int       `json:"version"`
	InstallationID  string    `json:"installation_id"`
	CreatedAt       time.Time `json:"created_at"`
	IncludesSession bool      `json:"includes_session"`
	Counts          Counts    `json:"counts"`
}

type Preview struct {
	Manifest         Manifest `json:"manifest"`
	TargetEmpty      bool     `json:"target_empty"`
	SameInstallation bool     `json:"same_installation"`
	CanRestore       bool     `json:"can_restore"`
	Warning          string   `json:"warning,omitempty"`
}

type RestoreResult struct {
	Counts          Counts `json:"counts"`
	RestartRequired bool   `json:"restart_required"`
}

// Repository 只接收已解密但仍留在内存中的 opaque payload。
type Repository interface {
	ExportPayload(ctx context.Context, includeSessions bool) ([]byte, Manifest, error)
	InspectPayload(ctx context.Context, payload []byte) (*Preview, error)
	RestorePayload(ctx context.Context, payload []byte) (*RestoreResult, error)
}
