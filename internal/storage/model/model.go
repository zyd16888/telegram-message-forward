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
	MessageID       int64
	RuleID          int64
	SinkID          int64
	TemplateID      *int64
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
type Setting struct {
	Key       string `gorm:"primaryKey"`
	Value     datatypes.JSON
	UpdatedAt time.Time
}

func (Setting) TableName() string { return "settings" }
