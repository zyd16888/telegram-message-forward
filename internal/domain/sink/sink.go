// Package sink 定义目标渠道领域模型与仓储接口。
package sink

import (
	"context"
	"errors"
	"time"
)

var ErrInUse = errors.New("渠道仍被引用，不能删除")

// MediaCapability 声明一个 Sink 对某类媒体的投递能力。
type MediaCapability struct {
	Type              string `json:"type"`
	Supported         bool   `json:"supported"`
	MaxSizeMB         int    `json:"max_size_mb,omitempty"`
	SupportsPublicURL bool   `json:"supports_public_url"`
	RequiresUpload    bool   `json:"requires_upload"`
	SupportsBinary    bool   `json:"supports_binary"`
	DeliveryMode      string `json:"delivery_mode,omitempty"`
	Fallback          string `json:"fallback,omitempty"`
}

// Capabilities 声明一个 Sink 支持的能力。
type Capabilities struct {
	SupportsText     bool              `json:"supports_text"`
	SupportsMarkdown bool              `json:"supports_markdown"`
	SupportsHTML     bool              `json:"supports_html"`
	SupportsImage    bool              `json:"supports_image"`
	SupportsFile     bool              `json:"supports_file"`
	SupportsAudio    bool              `json:"supports_audio"`
	SupportsVideo    bool              `json:"supports_video"`
	MaxTextLength    int               `json:"max_text_length,omitempty"`
	MaxTextBytes     map[string]int    `json:"max_text_bytes,omitempty"`
	MaxFileSizeMB    int               `json:"max_file_size_mb,omitempty"`
	MaxMediaItems    int               `json:"max_media_items,omitempty"`
	Media            []MediaCapability `json:"media,omitempty"`
	Notes            []string          `json:"notes,omitempty"`
}

// Sink 是一个目标渠道配置。
//
// Secret 属于敏感字段，领域层持有明文，存储层负责加解密。
type Sink struct {
	ID            int64
	Type          string // wecom_bot | wecom_app | webhook | ...
	Name          string
	Enabled       bool
	Config        map[string]any
	Secret        []byte
	Capabilities  Capabilities
	Observability Observability
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Observability 是 Sink 在管理后台展示的运行观测摘要。
type Observability struct {
	LastTestAt         *time.Time
	LastTestSuccess    bool
	LastTestError      string
	RecentFailure      string
	DeliveryTotal24h   int64
	DeliverySuccess24h int64
	SuccessRate24h     float64
}

// DeliveryStats 是 Sink 维度投递统计。
type DeliveryStats struct {
	Total       int64
	Success     int64
	LastFailure string
}

// Repository 是渠道仓储接口。
type Repository interface {
	Create(ctx context.Context, s *Sink) error
	Update(ctx context.Context, s *Sink) error
	GetByID(ctx context.Context, id int64) (*Sink, error)
	List(ctx context.Context) ([]*Sink, error)
	Delete(ctx context.Context, id int64) error
	UpdateTestResult(ctx context.Context, id int64, at time.Time, success bool, errText string) error
}
