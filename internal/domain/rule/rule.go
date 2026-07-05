// Package rule 定义路由规则领域模型与仓储接口。
package rule

import (
	"context"
	"time"
)

// ConditionConfig 是一个条件的配置。
// Type 对应 ruleengine 中注册的条件类型，如 keyword_contains、regex 等。
type ConditionConfig struct {
	Type   string         `json:"type"`
	Config map[string]any `json:"config,omitempty"`
}

// ProcessorConfig 是一个处理器的配置。
type ProcessorConfig struct {
	Type   string         `json:"type"`
	Config map[string]any `json:"config,omitempty"`
}

// Target 是规则的一个目标渠道及其模板绑定。
// TemplateID 为空表示按纯文本渲染。
type Target struct {
	SinkID     int64
	TemplateID *int64
}

// Rule 是一条路由规则。
type Rule struct {
	ID       int64
	Name     string
	Enabled  bool
	Priority int
	// FilterID 引用共享过滤器；为 0 表示使用内联 Conditions。
	// 仓储加载时若 FilterID>0，会用过滤器的条件覆盖 Conditions。
	FilterID    int64
	Conditions  []ConditionConfig
	Processors  []ProcessorConfig
	StopOnMatch bool
	SourceIDs   []int64
	Targets     []Target
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Repository 是规则仓储接口。
type Repository interface {
	Create(ctx context.Context, r *Rule) error
	Update(ctx context.Context, r *Rule) error
	GetByID(ctx context.Context, id int64) (*Rule, error)
	List(ctx context.Context) ([]*Rule, error)
	// ListEnabledBySource 返回命中某 source 的启用规则，按 priority 排序。
	ListEnabledBySource(ctx context.Context, sourceID int64) ([]*Rule, error)
	Delete(ctx context.Context, id int64) error
}
