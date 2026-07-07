// Package filter 定义可复用「过滤器」领域模型：一组命名的匹配条件，
// 供转发规则与 AI 整理共同引用。过滤器只承载条件，不含处理器与目标渠道。
package filter

import (
	"context"
	"time"

	domainrule "telegram-message-forward/internal/domain/rule"
)

// Filter 是一组可复用的匹配条件。
type Filter struct {
	ID          int64
	Name        string
	Description string
	Conditions  []domainrule.ConditionConfig
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Repository 是过滤器仓储接口。
type Repository interface {
	List(ctx context.Context) ([]*Filter, error)
	GetByID(ctx context.Context, id int64) (*Filter, error)
	Create(ctx context.Context, f *Filter) error
	Update(ctx context.Context, f *Filter) error
	Delete(ctx context.Context, id int64) error
	// CountReferences 返回引用该过滤器的旧规则、Flow 节点与 AI Profile 总数。
	CountReferences(ctx context.Context, id int64) (int64, error)
}
