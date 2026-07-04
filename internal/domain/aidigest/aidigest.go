// Package aidigest 定义 AI 整理旁路的领域模型。
package aidigest

import (
	"context"
	"time"

	domainmessage "telegram-message-forward/internal/domain/message"
	domainrule "telegram-message-forward/internal/domain/rule"
)

const (
	ProviderSettingKey = "ai.provider"
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
	ID             int64
	Name           string
	Enabled        bool
	SourceIDs      []int64
	Conditions     []domainrule.ConditionConfig
	Schedule       ScheduleConfig
	Window         WindowConfig
	Dedupe         DedupeConfig
	PromptTemplate string
	OutputFormat   string
	TargetSinkIDs  []int64
	ModelConfig    ModelConfig
	Limits         LimitsConfig
	CreatedAt      time.Time
	UpdatedAt      time.Time
	RecentRun      *Run
}

type ScheduleConfig struct {
	Type            string `json:"type"`
	IntervalMinutes int    `json:"interval_minutes,omitempty"`
	Time            string `json:"time,omitempty"`
	Timezone        string `json:"timezone,omitempty"`
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
	Model       string  `json:"model,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
}

type LimitsConfig struct {
	MaxMessagesPerRun  int `json:"max_messages_per_run,omitempty"`
	MaxCharsPerMessage int `json:"max_chars_per_message,omitempty"`
	MaxPromptChars     int `json:"max_prompt_chars,omitempty"`
}

type ProviderConfig struct {
	ProviderType       string  `json:"provider_type"`
	BaseURL            string  `json:"base_url"`
	Model              string  `json:"model"`
	TimeoutSeconds     int     `json:"timeout_seconds"`
	MaxRetries         int     `json:"max_retries"`
	DefaultTemperature float64 `json:"default_temperature"`
	HasAPIKey          bool    `json:"has_api_key"`
}

type Run struct {
	ID                int64
	ProfileID         int64
	Status            RunStatus
	TriggerType       TriggerType
	WindowStart       time.Time
	WindowEnd         time.Time
	InputMessageCount int
	IncludedCount     int
	ExcludedCount     int
	DeliveryTaskIDs   []int64
	ModelName         string
	TokenUsage        TokenUsage
	Error             string
	StartedAt         *time.Time
	FinishedAt        *time.Time
	CreatedAt         time.Time
}

type RunItem struct {
	RunID     int64
	MessageID int64
	SourceID  int64
	Included  bool
	Reason    string
	Score     *float64
	SortOrder int
	Message   *domainmessage.NormalizedMessage
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
	CreateRun(ctx context.Context, r *Run) error
	UpdateRun(ctx context.Context, r *Run) error
	HasRunningRun(ctx context.Context, profileID int64) (bool, error)
	LastSuccessfulRun(ctx context.Context, profileID int64) (*Run, error)
	ListRuns(ctx context.Context, profileID int64, limit, offset int) ([]*Run, error)
	GetRun(ctx context.Context, id int64) (*Run, error)
	AddRunItems(ctx context.Context, items []*RunItem) error
	ListRunItems(ctx context.Context, runID int64) ([]*RunItem, error)
	UpsertOutput(ctx context.Context, out *Output) error
	GetOutputByRunID(ctx context.Context, runID int64) (*Output, error)
	CleanupRuns(ctx context.Context, before time.Time) (int64, error)
}
