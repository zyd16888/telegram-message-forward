// Package chatarchive 定义聊天归档领域模型与仓储接口。
//
// 聊天归档是与 Flow 实时转发、AI 整理并列的第三条独立旁路：
// 归档数据不进 Flow、不产生投递任务、不写 messages 表，
// 因此也不受 messages_retention_days 保留期清理影响。
package chatarchive

import (
	"context"
	"time"
)

// PeerType 是被归档会话的 peer 类型。
type PeerType string

const (
	PeerUser    PeerType = "user"
	PeerChat    PeerType = "chat"
	PeerChannel PeerType = "channel"
)

// Valid 判断 peer 类型是否受支持。
func (t PeerType) Valid() bool {
	switch t {
	case PeerUser, PeerChat, PeerChannel:
		return true
	}
	return false
}

// Archive 是一个被归档的会话。
type Archive struct {
	ID           int64
	AccountID    int64
	PeerType     PeerType
	PeerID       int64
	PeerName     string
	PeerUsername string
	// MinMessageID / MaxMessageID 是已覆盖的消息 id 区间，用于增量补拉。
	MinMessageID int64
	MaxMessageID int64
	MessageCount int64
	MediaCount   int64
	LastSyncedAt *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Entity 是一段富文本标记（链接、引用、mention 等），保留原始 utf16 offset。
type Entity struct {
	Type     string `json:"type"`
	Offset   int    `json:"offset"`
	Length   int    `json:"length"`
	URL      string `json:"url,omitempty"`
	UserID   int64  `json:"user_id,omitempty"`
	Language string `json:"language,omitempty"`
}

// Media 是归档消息的媒体描述。默认只记录元信息，开启下载后才有 StorageKey。
type Media struct {
	Type       string `json:"type"`
	FileName   string `json:"file_name,omitempty"`
	MimeType   string `json:"mime_type,omitempty"`
	Size       int64  `json:"size,omitempty"`
	Width      int    `json:"width,omitempty"`
	Height     int    `json:"height,omitempty"`
	Duration   int    `json:"duration,omitempty"`
	Caption    string `json:"caption,omitempty"`
	StorageKey string `json:"storage_key,omitempty"`
	// Downloaded 为 false 表示只归档了元信息，文件本身没有落盘。
	Downloaded bool `json:"downloaded"`
}

// Forward 是转发来源信息。
type Forward struct {
	FromID   int64      `json:"from_id,omitempty"`
	FromName string     `json:"from_name,omitempty"`
	Date     *time.Time `json:"date,omitempty"`
	// ChannelPost 是原频道内的消息 id。
	ChannelPost int64 `json:"channel_post,omitempty"`
}

// Reaction 是一个表态及其计数。
type Reaction struct {
	Emoticon string `json:"emoticon"`
	Count    int    `json:"count"`
}

// Message 是一条归档消息。比 message.NormalizedMessage 宽：
// 额外保留回复链、方向、转发来源、服务消息等分析所需字段。
type Message struct {
	ID        int64
	ArchiveID int64
	MessageID int64
	GroupedID *int64
	// ReplyToMessageID 是被回复消息的 id，0 表示不是回复。
	ReplyToMessageID int64
	// Out 为 true 表示本账号发出的消息。
	Out            bool
	SenderPeerType string
	SenderID       int64
	SenderName     string
	SenderUsername string
	MessageType    string
	Text           string
	Entities       []Entity
	Media          []Media
	Forward        *Forward
	Reactions      []Reaction
	// ServiceAction 非空表示这是服务消息（入群、改名等）。
	ServiceAction string
	Views         int
	Date          *time.Time
	EditDate      *time.Time
	CreatedAt     time.Time
}

// JobStatus 是导出任务状态。
type JobStatus string

const (
	JobPending   JobStatus = "pending"
	JobRunning   JobStatus = "running"
	JobSucceeded JobStatus = "succeeded"
	JobFailed    JobStatus = "failed"
	JobCancelled JobStatus = "cancelled"
)

// Terminal 判断任务是否已到终态。
func (s JobStatus) Terminal() bool {
	switch s {
	case JobSucceeded, JobFailed, JobCancelled:
		return true
	}
	return false
}

// Job 是一次拉取任务。
type Job struct {
	ID        int64
	ArchiveID int64
	Status    JobStatus
	FromDate  *time.Time
	ToDate    *time.Time
	// IncludeMedia 默认 false：只归档媒体元信息，不下载文件。
	IncludeMedia  bool
	MediaMaxBytes int64
	// MaxMessages 是本次拉取的条数硬顶，0 表示不限。
	MaxMessages  int
	FetchedCount int
	MediaCount   int
	// CursorOffsetID 是断点续传游标。
	CursorOffsetID int64
	// CancelRequested 由 API 置位，执行器在页边界检查后停止。
	CancelRequested bool
	LastError       string
	StartedAt       *time.Time
	FinishedAt      *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// MessageQuery 是归档消息的检索条件。BeforeID 用于按主键倒序游标分页。
type MessageQuery struct {
	ArchiveID int64
	Keyword   string
	SenderID  int64
	// Out 为 nil 表示不限方向。
	Out *bool
	// IncludeService 为 false 时过滤掉服务消息。
	IncludeService bool
	From           *time.Time
	To             *time.Time
	BeforeID       int64
	Limit          int
}

// ExportQuery 是导出渲染的筛选条件，按时间升序全量流式读取。
type ExportQuery struct {
	ArchiveID      int64
	From           *time.Time
	To             *time.Time
	IncludeService bool
	// AfterID 是流式分批读取的游标（主键升序）。
	AfterID int64
	Limit   int
}

// ArchiveRepository 是归档会话仓储。
type ArchiveRepository interface {
	// Ensure 按 (account_id, peer_type, peer_id) 幂等获取或创建归档。
	Ensure(ctx context.Context, a *Archive) (*Archive, error)
	GetByID(ctx context.Context, id int64) (*Archive, error)
	List(ctx context.Context) ([]*Archive, error)
	Delete(ctx context.Context, id int64) error
	// RefreshStats 依据实际归档消息重算计数与覆盖区间。
	RefreshStats(ctx context.Context, archiveID int64, syncedAt time.Time) error
}

// MessageRepository 是归档消息仓储。
type MessageRepository interface {
	// BulkUpsert 按 (archive_id, message_id) 幂等写入一批消息，返回新增条数。
	BulkUpsert(ctx context.Context, archiveID int64, msgs []*Message) (int64, error)
	Search(ctx context.Context, q MessageQuery) ([]*Message, bool, error)
	// StreamPage 按主键升序取一页，供导出渲染分批读取，避免全量载入内存。
	StreamPage(ctx context.Context, q ExportQuery) ([]*Message, error)
	CountByArchive(ctx context.Context, archiveID int64) (int64, error)
}

// JobRepository 是导出任务仓储。
type JobRepository interface {
	Create(ctx context.Context, j *Job) error
	Update(ctx context.Context, j *Job) error
	GetByID(ctx context.Context, id int64) (*Job, error)
	ListByArchive(ctx context.Context, archiveID int64, limit int) ([]*Job, error)
	// ListActive 返回未到终态的任务，用于服务重启后回收。
	ListActive(ctx context.Context) ([]*Job, error)
	RequestCancel(ctx context.Context, id int64) error
}
