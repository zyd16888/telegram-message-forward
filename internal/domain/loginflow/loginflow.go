// Package loginflow 定义 Telegram 登录 flow 的领域模型与仓储接口。
//
// 登录 flow 持久化验证码登录与扫码登录的过程状态，支持服务重启后恢复、
// 页面刷新后继续、以及过期清理。敏感字段由存储层加密，验证码明文与
// 2FA 密码永不落库。
package loginflow

import (
	"context"
	"time"
)

// Method 是登录方式。
type Method string

const (
	// MethodPhoneCode 手机验证码登录。
	MethodPhoneCode Method = "phone_code"
	// MethodQR 扫码登录（V2-3）。
	MethodQR Method = "qr"
)

// Status 是登录 flow 状态。
type Status string

const (
	StatusSendingCode      Status = "sending_code"
	StatusCodeRequired     Status = "code_required"
	StatusPasswordRequired Status = "password_required"
	StatusAuthorized       Status = "authorized"
	StatusFailed           Status = "failed"
	StatusCancelled        Status = "cancelled"
	StatusExpired          Status = "expired"
	// QR 相关状态（V2-3）。
	StatusWaitingScan       Status = "waiting_scan"
	StatusQRRefreshRequired Status = "qr_refresh_required"
)

// IsTerminal 判断状态是否为终态（无需继续操作）。
func (s Status) IsTerminal() bool {
	switch s {
	case StatusAuthorized, StatusFailed, StatusCancelled, StatusExpired:
		return true
	default:
		return false
	}
}

// Flow 是一次登录流程。
//
// PhoneCodeHash 与 QRToken 属于敏感字段，领域层持有明文，存储层负责加解密。
type Flow struct {
	ID            int64
	FlowID        string
	AccountID     int64
	Method        Method
	Status        Status
	CurrentStep   string
	PhoneCodeHash string
	// QRToken 是 QR 登录 token 明文（敏感），存储层加密。
	QRToken []byte
	// QRTokenExpiresAt 是 QR token 的短过期时间，与 token 一并加密存储。
	QRTokenExpiresAt time.Time
	// QRURL 是由 QRToken 派生的 tg://login URL；仅在响应中即时构造，不落库、不持久化。
	QRURL     string
	DCID      int
	ExpiresAt time.Time
	LastError string
	CreatedAt time.Time
	UpdatedAt time.Time
	CompletedAt *time.Time
}

// Repository 是登录 flow 仓储接口。
type Repository interface {
	Create(ctx context.Context, f *Flow) error
	Update(ctx context.Context, f *Flow) error
	GetByFlowID(ctx context.Context, flowID string) (*Flow, error)
	// GetActiveByAccount 返回某账号最近一个非终态 flow，用于恢复展示；无则返回 nil,nil。
	GetActiveByAccount(ctx context.Context, accountID int64) (*Flow, error)
	// ExpireStale 将已超过 expires_at 的非终态 flow 标记为 expired 并清空敏感字段，
	// 返回受影响行数。用于服务启动与定期清理。
	ExpireStale(ctx context.Context, now time.Time) (int64, error)
}
