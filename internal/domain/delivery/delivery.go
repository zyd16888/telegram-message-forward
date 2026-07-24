// Package delivery 定义投递任务领域模型与仓储接口。
package delivery

import (
	"context"
	"time"

	domainmessage "telegram-message-forward/internal/domain/message"
)

// Status 是投递任务状态。
type Status string

const (
	StatusPending    Status = "pending"
	StatusProcessing Status = "processing"
	StatusSuccess    Status = "success"
	StatusFailed     Status = "failed"
	StatusRetrying   Status = "retrying"
	StatusDead       Status = "dead"
	StatusCancelled  Status = "cancelled"
)

// AttemptStatus 是单次投递尝试状态。
type AttemptStatus string

const (
	AttemptSuccess AttemptStatus = "success"
	AttemptFailed  AttemptStatus = "failed"
)

// Task 是一个投递任务。
type Task struct {
	ID              int64
	MessageID       int64
	RuleID          int64
	SinkID          int64
	TemplateID      *int64
	OriginType      string
	OriginID        int64
	OriginNodeID    int64
	Status          Status
	AttemptCount    int
	MaxAttempts     int
	NextRetryAt     *time.Time
	LockedAt        *time.Time
	LockedBy        string
	LastError       string
	MessageRevision int
	TextSuffix      string
	// MessageSnapshot 是 Flow/AI 产出的消息快照；为空时按 MessageID 读取原始消息。
	MessageSnapshot *domainmessage.NormalizedMessage
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// FlowRoute 是原消息实际生成过的 Flow 投递路由。
type FlowRoute struct {
	SinkID       int64
	TemplateID   *int64
	OriginID     int64
	OriginNodeID int64
}

// Attempt 是一次投递尝试记录，只保存脱敏摘要。
type Attempt struct {
	ID              int64
	DeliveryTaskID  int64
	AttemptNo       int
	Status          AttemptStatus
	RequestSummary  []byte
	ResponseSummary []byte
	Error           string
	StartedAt       *time.Time
	FinishedAt      *time.Time
	CreatedAt       time.Time
}

// Query 是投递列表过滤条件。0 值表示不过滤。
type Query struct {
	Status   Status
	SourceID int64
	SinkID   int64
	Since    *time.Time
	Limit    int
	Offset   int
}

// Repository 是投递任务仓储接口。
type Repository interface {
	// Create 需按来源类型幂等。
	Create(ctx context.Context, t *Task) error
	// Claim 使用 FOR UPDATE SKIP LOCKED 领取一批可执行任务，置为 processing。
	Claim(ctx context.Context, workerID string, limit int) ([]*Task, error)
	// UpdateStatus 更新任务状态与重试信息。
	UpdateStatus(ctx context.Context, t *Task) error
	// RecoverStale 将超过可见性超时仍处于 processing 的任务回退到 retrying。
	RecoverStale(ctx context.Context, olderThan time.Time) (int64, error)
	// Requeue 将一个终态（dead/failed/cancelled）任务重置为 pending 以手动重试。
	Requeue(ctx context.Context, id int64) error
	// AddAttempt 追加一次投递尝试记录。
	AddAttempt(ctx context.Context, a *Attempt) error
	GetByID(ctx context.Context, id int64) (*Task, error)
	List(ctx context.Context, status Status, limit, offset int) ([]*Task, error)
	Count(ctx context.Context, status Status) (int64, error)
	ListFlowRoutesByMessage(ctx context.Context, messageID int64) ([]FlowRoute, error)
}
