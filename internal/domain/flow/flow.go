// Package flow 定义 Flow 图编排领域模型与仓储接口。
package flow

import (
	"context"
	"time"

	domainrule "telegram-message-forward/internal/domain/rule"
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
	FilterIDs  []int64                      `json:"filter_ids,omitempty"`
	Conditions []domainrule.ConditionConfig `json:"conditions,omitempty"`
	Processors []domainrule.ProcessorConfig `json:"processors,omitempty"`
}

type Edge struct {
	ID         int64
	FlowID     int64
	FromNodeID int64
	ToNodeID   int64
}

type Repository interface {
	Create(ctx context.Context, f *Flow) error
	Update(ctx context.Context, f *Flow) error
	GetByID(ctx context.Context, id int64) (*Flow, error)
	List(ctx context.Context) ([]*Flow, error)
	ListEnabledBySource(ctx context.Context, sourceID int64) ([]*Flow, error)
	Delete(ctx context.Context, id int64) error
}
