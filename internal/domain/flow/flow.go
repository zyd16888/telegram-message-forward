// Package flow 定义 Flow 图编排领域模型与仓储接口。
package flow

import (
	"context"
	"time"

	domainmessage "telegram-message-forward/internal/domain/message"
)

type NodeType string

const (
	NodeTypeSource    NodeType = "source"
	NodeTypeFilter    NodeType = "filter"
	NodeTypeProcessor NodeType = "processor"
	NodeTypeTarget    NodeType = "target"
)

type Flow struct {
	ID          int64
	Name        string
	Enabled     bool
	Priority    int
	StopOnMatch bool
	Nodes       []Node
	Edges       []Edge
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Node struct {
	ID         int64
	FlowID     int64
	Type       NodeType
	RefID      *int64
	Config     NodeConfig
	TemplateID *int64
	PosX       float64
	PosY       float64
}

type NodeConfig struct {
	FilterIDs  []int64           `json:"filter_ids,omitempty"`
	Conditions []ConditionConfig `json:"conditions,omitempty"`
	Processors []ProcessorConfig `json:"processors,omitempty"`
}

type Edge struct {
	ID         int64
	FlowID     int64
	FromNodeID int64
	ToNodeID   int64
}

// ConditionConfig 是一个条件的配置。
// Type 对应 condition 注册表中的条件类型，如 keyword_contains、regex 等。
type ConditionConfig struct {
	Type   string         `json:"type"`
	Config map[string]any `json:"config,omitempty"`
}

// ProcessorConfig 是一个处理器的配置。
type ProcessorConfig struct {
	Type   string         `json:"type"`
	Config map[string]any `json:"config,omitempty"`
}

// Target 是 Flow target 节点产出的一个投递目标。
// TemplateID 为空表示按纯文本渲染。
type Target struct {
	SinkID     int64
	TemplateID *int64
}

// Match 是 Flow 引擎对一条消息的命中结果。
type Match struct {
	FlowID       int64
	FlowName     string
	Targets      []Target
	Message      *domainmessage.NormalizedMessage
	OriginType   string
	OriginID     int64
	OriginNodeID int64
}

type Repository interface {
	Create(ctx context.Context, f *Flow) error
	Update(ctx context.Context, f *Flow) error
	GetByID(ctx context.Context, id int64) (*Flow, error)
	List(ctx context.Context) ([]*Flow, error)
	ListEnabledBySource(ctx context.Context, sourceID int64) ([]*Flow, error)
	Delete(ctx context.Context, id int64) error
}
