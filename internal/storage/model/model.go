// Package model 定义 GORM 数据库模型。
//
// GORM model 只出现在存储层，不向外泄露到 domain、api 或 plugin。
// 字段与 migrations/00001_init.sql 保持一致。
package model

import (
	"time"

	"gorm.io/datatypes"
)

// Account 对应 accounts 表。
type Account struct {
	ID               int64 `gorm:"primaryKey"`
	Name             string
	PhoneNumber      string
	TelegramAppID    *int64
	ProxyID          *int64
	AppID            int
	AppHashEncrypted []byte
	SessionEncrypted []byte
	ProxyConfig      datatypes.JSON
	Status           string
	LastLoginAt      *time.Time
	LastError        string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// TableName 指定表名。
func (Account) TableName() string { return "accounts" }

// AdminUser 对应 admin_users 表。
type AdminUser struct {
	ID           int64 `gorm:"primaryKey"`
	Username     string
	PasswordHash string
	Active       bool
	LastLoginAt  *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (AdminUser) TableName() string { return "admin_users" }

// AdminSession 对应 admin_sessions 表。
type AdminSession struct {
	ID         int64 `gorm:"primaryKey"`
	UserID     int64
	TokenHash  string
	ExpiresAt  time.Time
	LastUsedAt *time.Time
	RevokedAt  *time.Time
	CreatedAt  time.Time
}

func (AdminSession) TableName() string { return "admin_sessions" }

// TelegramApp 对应 telegram_apps 表。
type TelegramApp struct {
	ID               int64 `gorm:"primaryKey"`
	Name             string
	AppID            int
	AppHashEncrypted []byte
	Enabled          bool
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (TelegramApp) TableName() string { return "telegram_apps" }

// ProxyConfig 对应 proxy_configs 表。
type ProxyConfig struct {
	ID                int64 `gorm:"primaryKey"`
	Name              string
	Type              string
	Addr              string
	Username          string
	PasswordEncrypted []byte
	Enabled           bool
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (ProxyConfig) TableName() string { return "proxy_configs" }

// TelegramPeer 对应 telegram_peers 表，缓存 access_hash。
type TelegramPeer struct {
	ID         int64 `gorm:"primaryKey"`
	AccountID  int64
	PeerType   string
	PeerID     int64
	AccessHash int64
	Username   string
	Title      string
	UpdatedAt  time.Time
}

func (TelegramPeer) TableName() string { return "telegram_peers" }

// Source 对应 sources 表。
type Source struct {
	ID            int64  `gorm:"primaryKey"`
	Type          string `gorm:"column:source_type"`
	AccountID     *int64
	PeerType      string
	PeerID        int64
	Name          string
	Username      string
	Enabled       bool
	Config        datatypes.JSON
	LastMessageID int64
	LastSyncedAt  *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (Source) TableName() string { return "sources" }

// Sink 对应 sinks 表。
type Sink struct {
	ID              int64 `gorm:"primaryKey"`
	Type            string
	Name            string
	Enabled         bool
	Config          datatypes.JSON
	SecretEncrypted []byte
	Capabilities    datatypes.JSON
	LastTestAt      *time.Time
	LastTestSuccess bool
	LastTestError   string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (Sink) TableName() string { return "sinks" }

// Template 对应 templates 表。
type Template struct {
	ID        int64 `gorm:"primaryKey"`
	Name      string
	Format    string
	Content   string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (Template) TableName() string { return "templates" }

// Rule 对应 rules 表。
type Rule struct {
	ID          int64 `gorm:"primaryKey"`
	Name        string
	Enabled     bool
	Priority    int
	Conditions  datatypes.JSON
	Processors  datatypes.JSON
	StopOnMatch bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (Rule) TableName() string { return "rules" }

// RuleSource 对应 rule_sources 表。
type RuleSource struct {
	RuleID   int64 `gorm:"primaryKey"`
	SourceID int64 `gorm:"primaryKey"`
}

func (RuleSource) TableName() string { return "rule_sources" }

// RuleTarget 对应 rule_targets 表。
type RuleTarget struct {
	ID         int64 `gorm:"primaryKey"`
	RuleID     int64
	SinkID     int64
	TemplateID *int64
}

func (RuleTarget) TableName() string { return "rule_targets" }

// Message 对应 messages 表。
type Message struct {
	ID                int64 `gorm:"primaryKey"`
	SourceID          int64
	ExternalMessageID int64
	GroupedID         *int64
	MessageType       string
	SenderPeerType    string
	SenderID          int64
	SenderName        string
	Text              string
	Media             datatypes.JSON
	Links             datatypes.JSON
	OriginalURL       string
	RawPayload        datatypes.JSON
	SentAt            *time.Time
	ReceivedAt        time.Time
	CreatedAt         time.Time
}

func (Message) TableName() string { return "messages" }

// DeliveryTask 对应 delivery_tasks 表。
type DeliveryTask struct {
	ID              int64 `gorm:"primaryKey"`
	MessageID       *int64
	RuleID          *int64
	SinkID          int64
	TemplateID      *int64
	OriginType      string
	OriginID        *int64
	Status          string
	AttemptCount    int
	MaxAttempts     int
	NextRetryAt     *time.Time
	LockedAt        *time.Time
	LockedBy        string
	LastError       string
	MessageSnapshot datatypes.JSON
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (DeliveryTask) TableName() string { return "delivery_tasks" }

// DeliveryAttempt 对应 delivery_attempts 表。
type DeliveryAttempt struct {
	ID              int64 `gorm:"primaryKey"`
	DeliveryTaskID  int64
	AttemptNo       int
	Status          string
	RequestSummary  datatypes.JSON
	ResponseSummary datatypes.JSON
	Error           string
	StartedAt       *time.Time
	FinishedAt      *time.Time
	CreatedAt       time.Time
}

func (DeliveryAttempt) TableName() string { return "delivery_attempts" }

// AIDigestProfile 对应 ai_digest_profiles 表。
type AIDigestProfile struct {
	ID               int64 `gorm:"primaryKey"`
	Name             string
	Enabled          bool
	SourceIDs        datatypes.JSON
	Conditions       datatypes.JSON
	Schedule         datatypes.JSON
	Window           datatypes.JSON
	Dedupe           datatypes.JSON
	PromptTemplate   string
	OutputFormat     string
	OutputTemplateID *int64
	OutputTemplate   string
	TargetSinkIDs    datatypes.JSON
	ModelConfig      datatypes.JSON
	Limits           datatypes.JSON
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (AIDigestProfile) TableName() string { return "ai_digest_profiles" }

// AIDigestOutputTemplate 对应 ai_digest_output_templates 表。
type AIDigestOutputTemplate struct {
	ID          int64 `gorm:"primaryKey"`
	Name        string
	Description string
	Format      string
	Content     string
	BuiltIn     bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (AIDigestOutputTemplate) TableName() string { return "ai_digest_output_templates" }

// AIDigestRun 对应 ai_digest_runs 表。
type AIDigestRun struct {
	ID                int64 `gorm:"primaryKey"`
	ProfileID         *int64
	Status            string
	TriggerType       string
	WindowStart       time.Time
	WindowEnd         time.Time
	InputMessageCount int
	IncludedCount     int
	ExcludedCount     int
	DeliveryTaskIDs   datatypes.JSON
	ProviderID        string
	ProviderName      string
	ModelName         string
	TokenUsage        datatypes.JSON
	Error             string
	StartedAt         *time.Time
	FinishedAt        *time.Time
	CreatedAt         time.Time
}

func (AIDigestRun) TableName() string { return "ai_digest_runs" }

// AIDigestRunItem 对应 ai_digest_run_items 表。
type AIDigestRunItem struct {
	RunID     int64 `gorm:"primaryKey"`
	MessageID int64 `gorm:"primaryKey"`
	SourceID  int64
	Included  bool
	Reason    string
	Score     *float64
	SortOrder int
}

func (AIDigestRunItem) TableName() string { return "ai_digest_run_items" }

// AIDigestOutput 对应 ai_digest_outputs 表。
type AIDigestOutput struct {
	ID          int64 `gorm:"primaryKey"`
	RunID       int64
	Format      string
	Title       string
	Content     string
	RawResponse datatypes.JSON
	CreatedAt   time.Time
}

func (AIDigestOutput) TableName() string { return "ai_digest_outputs" }

// APIToken 对应 api_tokens 表。
type APIToken struct {
	ID         int64 `gorm:"primaryKey"`
	Name       string
	TokenHash  string
	CreatedAt  time.Time
	LastUsedAt *time.Time
	RevokedAt  *time.Time
}

func (APIToken) TableName() string { return "api_tokens" }

// TelegramLoginFlow 对应 telegram_login_flows 表。
//
// 敏感字段（phone_code_hash、qr_token）加密存储在 *_encrypted 列。
type TelegramLoginFlow struct {
	ID                     int64 `gorm:"primaryKey"`
	FlowID                 string
	AccountID              int64
	Method                 string
	Status                 string
	CurrentStep            string
	PhoneCodeHashEncrypted []byte
	QRTokenEncrypted       []byte
	DCID                   int
	ExpiresAt              time.Time
	LastError              string
	CreatedAt              time.Time
	UpdatedAt              time.Time
	CompletedAt            *time.Time
}

func (TelegramLoginFlow) TableName() string { return "telegram_login_flows" }

// Setting 对应 settings 表。
//
// 敏感字段（如 S3 secret_key）以 JSON 形式加密存储在 secret_encrypted 列。
type Setting struct {
	Key             string `gorm:"primaryKey"`
	Value           datatypes.JSON
	SecretEncrypted []byte
	UpdatedAt       time.Time
}

func (Setting) TableName() string { return "settings" }
