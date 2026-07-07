// Package dispatch 负责生成投递任务并由 worker 异步执行。
package dispatch

import (
	"context"

	domaindelivery "telegram-message-forward/internal/domain/delivery"
	domainmessage "telegram-message-forward/internal/domain/message"
	domainsink "telegram-message-forward/internal/domain/sink"
	"telegram-message-forward/internal/ruleengine"
)

// Queue 根据规则引擎的匹配结果生成投递任务。
type Queue struct {
	tasks       domaindelivery.Repository
	sinks       domainsink.Repository
	maxAttempts int
	notifier    *Notifier
}

// NewQueue 创建投递任务生成器。
func NewQueue(tasks domaindelivery.Repository, maxAttempts int, notifiers ...*Notifier) *Queue {
	if maxAttempts <= 0 {
		maxAttempts = 3
	}
	var notifier *Notifier
	if len(notifiers) > 0 {
		notifier = notifiers[0]
	}
	return &Queue{tasks: tasks, maxAttempts: maxAttempts, notifier: notifier}
}

// UseSinks 注入渠道仓储，用于入队前跳过已禁用目标渠道。
func (q *Queue) UseSinks(sinks domainsink.Repository) *Queue {
	q.sinks = sinks
	return q
}

// Enqueue 为每个命中规则的每个目标渠道生成一个 pending 投递任务。
//
// 任务创建按来源类型幂等：rule 为 (message_id, rule_id, sink_id)，
// flow 为 (message_id, origin_id, origin_node_id)，重复不产生新任务。
func (q *Queue) Enqueue(ctx context.Context, msg *domainmessage.NormalizedMessage, matches []ruleengine.Match) error {
	created := false
	enabledSinks := map[int64]bool{}
	for _, m := range matches {
		for _, target := range m.Targets {
			enabled, err := q.sinkEnabled(ctx, target.SinkID, enabledSinks)
			if err != nil {
				return err
			}
			if !enabled {
				continue
			}
			task := &domaindelivery.Task{
				MessageID:       msg.ID,
				RuleID:          ruleID(m),
				SinkID:          target.SinkID,
				TemplateID:      target.TemplateID,
				OriginType:      matchOriginType(m),
				OriginID:        m.OriginID,
				OriginNodeID:    m.OriginNodeID,
				Status:          domaindelivery.StatusPending,
				MaxAttempts:     q.maxAttempts,
				MessageSnapshot: m.Message,
			}
			if err := q.tasks.Create(ctx, task); err != nil {
				return err
			}
			created = true
		}
	}
	if created {
		q.notifier.Notify()
	}
	return nil
}

func ruleID(m ruleengine.Match) int64 {
	// flow 来源的任务不写 rule_id：delivery_tasks.rule_id 外键指向 rules 表，
	// flow.ID 既可能不存在于 rules（插入失败），也可能撞上无关规则（级联误删）。
	if m.OriginType == "flow" || m.Rule == nil {
		return 0
	}
	return m.Rule.ID
}

func matchOriginType(m ruleengine.Match) string {
	if m.OriginType != "" {
		return m.OriginType
	}
	return "rule"
}

func (q *Queue) sinkEnabled(ctx context.Context, sinkID int64, cache map[int64]bool) (bool, error) {
	if q.sinks == nil {
		return true, nil
	}
	if enabled, ok := cache[sinkID]; ok {
		return enabled, nil
	}
	sink, err := q.sinks.GetByID(ctx, sinkID)
	if err != nil {
		return false, err
	}
	cache[sinkID] = sink.Enabled
	return sink.Enabled, nil
}
