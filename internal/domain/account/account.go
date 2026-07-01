// Package account 定义 Telegram 账号领域模型与仓储接口。
package account

import (
	"context"
	"time"
)

// Status 是账号登录状态。
type Status string

const (
	StatusInactive  Status = "inactive"
	StatusLoggingIn Status = "logging_in"
	StatusActive    Status = "active"
	StatusError     Status = "error"
	StatusBanned    Status = "banned"
)

// ProxyConfig 是代理配置。凭据字段在存储层加密。
type ProxyConfig struct {
	Type     string `json:"type,omitempty"` // socks5 | http
	Addr     string `json:"addr,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// Account 是 Telegram 用户账号。
//
// AppHash 与 Session 属于敏感字段，领域层持有明文，存储层负责加解密。
type Account struct {
	ID          int64
	Name        string
	PhoneNumber string
	AppID       int
	AppHash     string
	Session     []byte
	Proxy       ProxyConfig
	Status      Status
	LastLoginAt *time.Time
	LastError   string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Repository 是账号仓储接口。
type Repository interface {
	Create(ctx context.Context, a *Account) error
	Update(ctx context.Context, a *Account) error
	GetByID(ctx context.Context, id int64) (*Account, error)
	List(ctx context.Context) ([]*Account, error)
	Delete(ctx context.Context, id int64) error
}
