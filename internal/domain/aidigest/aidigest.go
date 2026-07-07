// Package aidigest 定义 AI 整理旁路的领域模型。
package aidigest

import (
	"context"
	"time"

	domainflow "telegram-message-forward/internal/domain/flow"
	domainmessage "telegram-message-forward/internal/domain/message"
)

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
	// FilterID 引用共享过滤器；为 0 表示使用内联 Conditions。
	FilterID       int64
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
	CreatedAt        time.Time
	UpdatedAt        time.Time
	RecentRun        *Run
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
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	Description    string         `json:"description"`
	PromptTemplate string         `json:"prompt_template"`
	OutputFormat   string         `json:"output_format"`
	OutputTemplate string         `json:"output_template"`
	Schedule       ScheduleConfig `json:"schedule"`
	Window         WindowConfig   `json:"window"`
	Dedupe         DedupeConfig   `json:"dedupe"`
	ModelConfig    ModelConfig    `json:"model_config"`
	Limits         LimitsConfig   `json:"limits"`
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
	ProviderID        string
	ProviderName      string
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
	ListOutputTemplates(ctx context.Context) ([]*OutputTemplate, error)
	GetOutputTemplate(ctx context.Context, id int64) (*OutputTemplate, error)
	CreateOutputTemplate(ctx context.Context, t *OutputTemplate) error
	UpdateOutputTemplate(ctx context.Context, t *OutputTemplate) error
	DeleteOutputTemplate(ctx context.Context, id int64) error
	CountProfilesUsingTemplate(ctx context.Context, templateID int64) (int64, error)
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
