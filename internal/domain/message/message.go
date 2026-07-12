// Package message 定义标准化消息领域模型与仓储接口。
package message

import (
	"context"
	"time"
)

// Media 是一个媒体附件的内部描述。
type Media struct {
	Type           string     `json:"type"` // photo | image | video | audio | document | ...
	URL            string     `json:"url,omitempty"`
	RemoteURL      string     `json:"remote_url,omitempty"`
	FileName       string     `json:"file_name,omitempty"`
	MimeType       string     `json:"mime_type,omitempty"`
	Size           int64      `json:"size,omitempty"`
	Width          int        `json:"width,omitempty"`
	Height         int        `json:"height,omitempty"`
	Caption        string     `json:"caption,omitempty"`
	LocalPath      string     `json:"local_path,omitempty"`
	StorageKey     string     `json:"storage_key,omitempty"`
	DownloadStatus string     `json:"download_status,omitempty"`
	DownloadError  string     `json:"download_error,omitempty"`
	CreatedAt      *time.Time `json:"created_at,omitempty"`
}

// Link 是消息中的链接。
type Link struct {
	URL   string `json:"url"`
	Title string `json:"title,omitempty"`
}

// NormalizedMessage 是所有 Source 输入统一转换后的内部消息模型。
type NormalizedMessage struct {
	ID                int64
	SourceID          int64
	ExternalMessageID int64
	GroupedID         *int64
	MessageType       string
	SenderPeerType    string
	SenderID          int64
	SenderName        string
	Text              string
	Media             []Media
	Links             []Link
	OriginalURL       string
	RawPayload        []byte
	SentAt            *time.Time
	ReceivedAt        time.Time
	CreatedAt         time.Time
}

// Repository 是消息仓储接口。
//
// Create 需保证按 (source_id, external_message_id) 幂等：重复消息不产生新记录。
type Repository interface {
	Create(ctx context.Context, m *NormalizedMessage) error
	GetByID(ctx context.Context, id int64) (*NormalizedMessage, error)
	ExistsByExternalID(ctx context.Context, sourceID, externalMessageID int64) (bool, error)
}

// Query 是消息中心的只读查询条件。BeforeID 用于按消息 ID 倒序游标分页。
type Query struct {
	SourceID       int64
	SourceType     string
	MessageType    string
	Keyword        string
	HasMedia       *bool
	DeliveryStatus string
	From           *time.Time
	To             *time.Time
	BeforeID       int64
	Limit          int
}

// DeliveryCounts 汇总一条消息关联的投递任务状态。
type DeliveryCounts struct {
	Total      int64
	Pending    int64
	Processing int64
	Success    int64
	Failed     int64
	Retrying   int64
	Dead       int64
	Cancelled  int64
}

// ListItem 是消息中心列表所需的消息、来源和投递摘要。
type ListItem struct {
	Message        *NormalizedMessage
	SourceName     string
	SourceType     string
	SourceUsername string
	Deliveries     DeliveryCounts
}

// DeliveryRef 是消息详情中的精简投递引用。
type DeliveryRef struct {
	ID           int64
	SinkID       int64
	SinkName     string
	SinkType     string
	OriginType   string
	OriginID     int64
	OriginNodeID int64
	FlowName     string
	Status       string
	AttemptCount int
	MaxAttempts  int
	LastError    string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Detail 是消息中心详情，不包含 raw_payload。
type Detail struct {
	Item       *ListItem
	Deliveries []DeliveryRef
}

// QueryRepository 是消息中心使用的只读仓储接口，与实时采集写入契约分离。
type QueryRepository interface {
	List(ctx context.Context, q Query) ([]ListItem, bool, error)
	GetDetail(ctx context.Context, id int64) (*Detail, error)
}
