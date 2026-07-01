// Package dispatch 负责生成投递任务并由 worker 异步执行。
package dispatch

import (
	"context"

	domaindelivery "telegram-message-forward/internal/domain/delivery"
	domainmessage "telegram-message-forward/internal/domain/message"
	"telegram-message-forward/internal/ruleengine"
)

// Queue 根据规则引擎的匹配结果生成投递任务。
type Queue struct {
	tasks       domaindelivery.Repository
	maxAttempts int
}

// NewQueue 创建投递任务生成器。
func NewQueue(tasks domaindelivery.Repository, maxAttempts int) *Queue {
	if maxAttempts <= 0 {
		maxAttempts = 3
	}
	return &Queue{tasks: tasks, maxAttempts: maxAttempts}
}

// Enqueue 为每个命中规则的每个目标渠道生成一个 pending 投递任务。
//
// 任务创建按 (message_id, rule_id, sink_id) 幂等，重复不产生新任务。
func (q *Queue) Enqueue(ctx context.Context, msg *domainmessage.NormalizedMessage, matches []ruleengine.Match) error {
	for _, m := range matches {
		for _, target := range m.Targets {
			task := &domaindelivery.Task{
				MessageID:       msg.ID,
				RuleID:          m.Rule.ID,
				SinkID:          target.SinkID,
				TemplateID:      target.TemplateID,
				Status:          domaindelivery.StatusPending,
				MaxAttempts:     q.maxAttempts,
				MessageSnapshot: m.Message,
			}
			if err := q.tasks.Create(ctx, task); err != nil {
				return err
			}
		}
	}
	return nil
}
