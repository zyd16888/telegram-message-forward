// Package message 定义标准化消息领域模型与仓储接口。
package message

import (
	"context"
	"time"
)

// Media 是一个媒体附件的内部描述。
type Media struct {
	Type     string `json:"type"` // photo | video | document | ...
	URL      string `json:"url,omitempty"`
	FileName string `json:"file_name,omitempty"`
	MimeType string `json:"mime_type,omitempty"`
	Size     int64  `json:"size,omitempty"`
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
