// Package ruleengine 编排条件判断、处理器执行与目标渠道选取。
//
// 规则引擎不直接调用外部 Sink，只产出命中的目标，交由 dispatch 生成投递任务。
package ruleengine

import (
	"context"

	domainmessage "github.com/zyd16888/telegram-message-forward/internal/domain/message"
	domainrule "github.com/zyd16888/telegram-message-forward/internal/domain/rule"
	"github.com/zyd16888/telegram-message-forward/internal/ruleengine/condition"
	"github.com/zyd16888/telegram-message-forward/internal/ruleengine/processor"
)

// Match 是一条命中规则及其目标渠道。
type Match struct {
	Rule    *domainrule.Rule
	Targets []domainrule.Target
}

// Engine 是规则引擎。
type Engine struct{}

// NewEngine 创建规则引擎。
func NewEngine() *Engine {
	return &Engine{}
}

// Evaluate 对消息按优先级顺序匹配 rules，返回命中的规则与目标。
//
// rules 应已按 priority 排序。命中规则时执行其处理器（原地修改 msg）；
// 若规则设置 stop_on_match，则命中后停止后续匹配。
func (e *Engine) Evaluate(ctx context.Context, msg *domainmessage.NormalizedMessage, rules []*domainrule.Rule) ([]Match, error) {
	var matches []Match

	for _, r := range rules {
		if !r.Enabled {
			continue
		}

		ok, err := e.evalConditions(ctx, msg, r.Conditions)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}

		if err := e.runProcessors(ctx, msg, r.Processors); err != nil {
			return nil, err
		}

		matches = append(matches, Match{Rule: r, Targets: r.Targets})

		if r.StopOnMatch {
			break
		}
	}

	return matches, nil
}

// evalConditions 要求全部条件通过（AND 语义）。空条件视为通过。
func (e *Engine) evalConditions(ctx context.Context, msg *domainmessage.NormalizedMessage, configs []domainrule.ConditionConfig) (bool, error) {
	for _, cfg := range configs {
		c, err := condition.Get(cfg.Type)
		if err != nil {
			return false, err
		}
		ok, err := c.Evaluate(ctx, msg, cfg.Config)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
	}
	return true, nil
}

func (e *Engine) runProcessors(ctx context.Context, msg *domainmessage.NormalizedMessage, configs []domainrule.ProcessorConfig) error {
	for _, cfg := range configs {
		p, err := processor.Get(cfg.Type)
		if err != nil {
			return err
		}
		if err := p.Process(ctx, msg, cfg.Config); err != nil {
			return err
		}
	}
	return nil
}
