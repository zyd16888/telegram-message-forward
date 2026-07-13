// Package aidigest 定义 AI 整理旁路的领域模型。
package aidigest

import (
	"context"
	"errors"
	"time"

	domainflow "telegram-message-forward/internal/domain/flow"
	domainmessage "telegram-message-forward/internal/domain/message"
)

var ErrActiveRunExists = errors.New("同一 AI Profile 已有运行中的任务")

const (
	ProviderSettingKey  = "ai.provider"
	ProvidersSettingKey = "ai.providers"
)

type RunStatus string

const (
	RunPending   RunStatus = "pending"
	RunRunning   RunStatus = "running"
	RunSuccess   RunStatus = "success"
	RunFailed    RunStatus = "failed"
	RunCancelled RunStatus = "cancelled"
)

type TriggerType string

const (
	TriggerManual   TriggerType = "manual"
	TriggerSchedule TriggerType = "schedule"
	TriggerPreview  TriggerType = "preview"
)

type Profile struct {
	ID        int64
	Name      string
	Enabled   bool
	SourceIDs []int64
	// FilterID 兼容旧单过滤器字段；优先使用 FilterIDs。
	FilterID int64
	// FilterIDs 引用多个共享过滤器，全部命中（AND）才纳入；为空时回退 FilterID 或内联 Conditions。
	FilterIDs      []int64
	Conditions     []domainflow.ConditionConfig
	Schedule       ScheduleConfig
	Window         WindowConfig
	Dedupe         DedupeConfig
	PromptTemplate string
	OutputFormat   string
	// OutputTemplateID 引用共享输出结构模板；为 0 表示使用内联自定义 OutputTemplate。
	OutputTemplateID int64
	OutputTemplate   string
	TargetSinkIDs    []int64
	ModelConfig      ModelConfig
	Limits           LimitsConfig
	Multimodal       MultimodalConfig
	CreatedAt        time.Time
	UpdatedAt        time.Time
	RecentRun        *Run
	// Stats 是近窗统计（列表接口填充，不落库）。
	Stats *ProfileStats
}

// ProfileStats 是 Profile 近窗运行统计。
type ProfileStats struct {
	SinceHours int   `json:"since_hours"`
	Runs       int64 `json:"runs"`
	Success    int64 `json:"success"`
	Failed     int64 `json:"failed"`
	Tokens     int64 `json:"tokens"`
}

// OutputTemplate 是可被多个 Profile 复用的输出结构模板。
type OutputTemplate struct {
	ID          int64
	Name        string
	Description string
	Format      string
	Content     string
	BuiltIn     bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type ScheduleConfig struct {
	Type            string `json:"type"`
	IntervalMinutes int    `json:"interval_minutes,omitempty"`
	Time            string `json:"time,omitempty"`
	Timezone        string `json:"timezone,omitempty"`
	Cron            string `json:"cron,omitempty"`
	NextRunAt       string `json:"next_run_at,omitempty"`
}

type WindowConfig struct {
	Type            string `json:"type"`
	DurationMinutes int    `json:"duration_minutes,omitempty"`
}

type DedupeConfig struct {
	Enabled bool `json:"enabled"`
}

type ModelConfig struct {
	ProviderID  string  `json:"provider_id,omitempty"`
	Model       string  `json:"model,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
}

type LimitsConfig struct {
	MaxMessagesPerRun  int `json:"max_messages_per_run,omitempty"`
	MaxCharsPerMessage int `json:"max_chars_per_message,omitempty"`
	MaxPromptChars     int `json:"max_prompt_chars,omitempty"`
}

type MultimodalConfig struct {
	Enabled            bool   `json:"enabled"`
	AllowExternalMedia bool   `json:"allow_external_media"`
	ImageDetail        string `json:"image_detail,omitempty"`
	MaxImagesPerRun    int    `json:"max_images_per_run,omitempty"`
	MaxImageBytes      int64  `json:"max_image_bytes,omitempty"`
	MaxTotalImageBytes int64  `json:"max_total_image_bytes,omitempty"`
	FailureMode        string `json:"failure_mode,omitempty"`
}

type ProviderConfig struct {
	ID                 string  `json:"id,omitempty"`
	Name               string  `json:"name,omitempty"`
	ProviderType       string  `json:"provider_type"`
	APIType            string  `json:"api_type,omitempty"`
	BaseURL            string  `json:"base_url"`
	Model              string  `json:"model"`
	TimeoutSeconds     int     `json:"timeout_seconds"`
	MaxRetries         int     `json:"max_retries"`
	DefaultTemperature float64 `json:"default_temperature"`
	SupportsVision     bool    `json:"supports_vision"`
	VisionModel        string  `json:"vision_model,omitempty"`
	Enabled            bool    `json:"enabled"`
	IsDefault          bool    `json:"is_default,omitempty"`
	HasAPIKey          bool    `json:"has_api_key"`
}

type ProviderStore struct {
	DefaultProviderID string           `json:"default_provider_id"`
	Providers         []ProviderConfig `json:"providers"`
}

type ProviderSecretStore struct {
	APIKeys map[string]string `json:"api_keys"`
}

type Preset struct {
	ID             string           `json:"id"`
	Name           string           `json:"name"`
	Description    string           `json:"description"`
	PromptTemplate string           `json:"prompt_template"`
	OutputFormat   string           `json:"output_format"`
	OutputTemplate string           `json:"output_template"`
	Schedule       ScheduleConfig   `json:"schedule"`
	Window         WindowConfig     `json:"window"`
	Dedupe         DedupeConfig     `json:"dedupe"`
	ModelConfig    ModelConfig      `json:"model_config"`
	Limits         LimitsConfig     `json:"limits"`
	Multimodal     MultimodalConfig `json:"multimodal"`
}

type Run struct {
	ID                 int64
	ProfileID          int64
	Status             RunStatus
	TriggerType        TriggerType
	WindowStart        time.Time
	WindowEnd          time.Time
	InputMessageCount  int
	IncludedCount      int
	ExcludedCount      int
	PromptMessageCount int
	PromptOmittedCount int
	PromptChars        int
	DeliveryTaskIDs    []int64
	ProviderID         string
	ProviderName       string
	ModelName          string
	SystemPrompt       string
	UserPrompt         string
	RequestConfig      RequestConfig
	MediaAudit         []MediaAudit
	TokenUsage         TokenUsage
	Error              string
	StartedAt          *time.Time
	FinishedAt         *time.Time
	CreatedAt          time.Time
	ProfileSnapshot    *Profile
	DeliveryTasks      []DeliveryTaskSummary
}

type DeliveryTaskSummary struct {
	ID                int64  `json:"id"`
	SinkID            int64  `json:"sink_id"`
	Status            string `json:"status"`
	AttemptCount      int    `json:"attempt_count"`
	LastError         string `json:"last_error,omitempty"`
	LastErrorReadable string `json:"last_error_readable,omitempty"`
}

type RunItem struct {
	RunID           int64
	MessageID       int64
	SourceID        int64
	Included        bool
	Reason          string
	Score           *float64
	SortOrder       int
	Message         *domainmessage.NormalizedMessage
	MessageSnapshot *MessageSnapshot
}

type RequestConfig struct {
	ProviderID   string                   `json:"provider_id,omitempty"`
	ProviderName string                   `json:"provider_name,omitempty"`
	APIType      string                   `json:"api_type,omitempty"`
	Model        string                   `json:"model,omitempty"`
	Temperature  float64                  `json:"temperature,omitempty"`
	MaxTokens    int                      `json:"max_tokens,omitempty"`
	Multimodal   *MultimodalRequestConfig `json:"multimodal,omitempty"`
}

type MultimodalRequestConfig struct {
	Enabled            bool   `json:"enabled"`
	ImageDetail        string `json:"image_detail"`
	MaxImagesPerRun    int    `json:"max_images_per_run"`
	MaxImageBytes      int64  `json:"max_image_bytes"`
	MaxTotalImageBytes int64  `json:"max_total_image_bytes"`
	Included           int    `json:"included"`
	Skipped            int    `json:"skipped"`
	Failed             int    `json:"failed"`
}

type MediaAudit struct {
	MessageID  int64  `json:"message_id"`
	MediaIndex int    `json:"media_index"`
	GroupedID  *int64 `json:"grouped_id,omitempty"`
	FileName   string `json:"file_name,omitempty"`
	MimeType   string `json:"mime_type,omitempty"`
	Size       int64  `json:"size,omitempty"`
	SHA256     string `json:"sha256,omitempty"`
	Status     string `json:"status"`
	Reason     string `json:"reason,omitempty"`
}

type MessageSnapshot struct {
	ID                int64                `json:"id"`
	SourceID          int64                `json:"source_id"`
	ExternalMessageID int64                `json:"external_message_id,omitempty"`
	GroupedID         *int64               `json:"grouped_id,omitempty"`
	MessageType       string               `json:"message_type"`
	SenderPeerType    string               `json:"sender_peer_type,omitempty"`
	SenderID          int64                `json:"sender_id,omitempty"`
	SenderName        string               `json:"sender_name,omitempty"`
	Text              string               `json:"text,omitempty"`
	Media             []MessageMedia       `json:"media,omitempty"`
	Links             []domainmessage.Link `json:"links,omitempty"`
	OriginalURL       string               `json:"original_url,omitempty"`
	SentAt            *time.Time           `json:"sent_at,omitempty"`
	ReceivedAt        time.Time            `json:"received_at"`
	CreatedAt         time.Time            `json:"created_at"`
}

type MessageMedia struct {
	Type      string `json:"type"`
	URL       string `json:"url,omitempty"`
	RemoteURL string `json:"remote_url,omitempty"`
	FileName  string `json:"file_name,omitempty"`
	MimeType  string `json:"mime_type,omitempty"`
	Size      int64  `json:"size,omitempty"`
	Width     int    `json:"width,omitempty"`
	Height    int    `json:"height,omitempty"`
	Caption   string `json:"caption,omitempty"`
}

func NewMessageSnapshot(message *domainmessage.NormalizedMessage) *MessageSnapshot {
	if message == nil {
		return nil
	}
	media := make([]MessageMedia, 0, len(message.Media))
	for _, item := range message.Media {
		media = append(media, MessageMedia{
			Type: item.Type, URL: item.URL, RemoteURL: item.RemoteURL, FileName: item.FileName,
			MimeType: item.MimeType, Size: item.Size, Width: item.Width, Height: item.Height, Caption: item.Caption,
		})
	}
	return &MessageSnapshot{
		ID: message.ID, SourceID: message.SourceID, ExternalMessageID: message.ExternalMessageID,
		GroupedID: message.GroupedID, MessageType: message.MessageType, SenderPeerType: message.SenderPeerType,
		SenderID: message.SenderID, SenderName: message.SenderName, Text: message.Text, Media: media,
		Links: append([]domainmessage.Link(nil), message.Links...), OriginalURL: message.OriginalURL,
		SentAt: message.SentAt, ReceivedAt: message.ReceivedAt, CreatedAt: message.CreatedAt,
	}
}

type Output struct {
	ID          int64
	RunID       int64
	Format      string
	Title       string
	Content     string
	RawResponse []byte
	CreatedAt   time.Time
}

type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens,omitempty"`
	CompletionTokens int `json:"completion_tokens,omitempty"`
	TotalTokens      int `json:"total_tokens,omitempty"`
}

type RunDetail struct {
	Run    *Run
	Items  []*RunItem
	Output *Output
}

type Repository interface {
	CreateProfile(ctx context.Context, p *Profile) error
	UpdateProfile(ctx context.Context, p *Profile) error
	GetProfile(ctx context.Context, id int64) (*Profile, error)
	ListProfiles(ctx context.Context) ([]*Profile, error)
	DeleteProfile(ctx context.Context, id int64) error
	ListOutputTemplates(ctx context.Context) ([]*OutputTemplate, error)
	GetOutputTemplate(ctx context.Context, id int64) (*OutputTemplate, error)
	CreateOutputTemplate(ctx context.Context, t *OutputTemplate) error
	UpdateOutputTemplate(ctx context.Context, t *OutputTemplate) error
	DeleteOutputTemplate(ctx context.Context, id int64) error
	CountProfilesUsingTemplate(ctx context.Context, templateID int64) (int64, error)
	CreateRun(ctx context.Context, r *Run) error
	UpdateRun(ctx context.Context, r *Run) error
	HasRunningRun(ctx context.Context, profileID int64) (bool, error)
	LastExecutionRun(ctx context.Context, profileID int64) (*Run, error)
	LastSuccessfulRun(ctx context.Context, profileID int64) (*Run, error)
	ListRuns(ctx context.Context, profileID int64, limit, offset int) ([]*Run, error)
	CountRuns(ctx context.Context, profileID int64) (int64, error)
	GetRun(ctx context.Context, id int64) (*Run, error)
	AddRunItems(ctx context.Context, items []*RunItem) error
	ListRunItems(ctx context.Context, runID int64) ([]*RunItem, error)
	UpsertOutput(ctx context.Context, out *Output) error
	GetOutputByRunID(ctx context.Context, runID int64) (*Output, error)
	CleanupRuns(ctx context.Context, before time.Time) (int64, error)
	RecoverStaleRuns(ctx context.Context, before, finishedAt time.Time) (int64, error)
	// AggregateStatsSince 汇总 since 之后各 Profile 的运行计数与 token。
	AggregateStatsSince(ctx context.Context, since time.Time) (map[int64]ProfileStats, error)
	// AggregateGlobalStatsSince 汇总 since 之后全局 AI 运行统计。
	AggregateGlobalStatsSince(ctx context.Context, since time.Time) (ProfileStats, error)
}
