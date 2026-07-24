// Package dispatch 负责生成投递任务并由 worker 异步执行。
package dispatch

import (
	"context"

	domaindelivery "telegram-message-forward/internal/domain/delivery"
	domainflow "telegram-message-forward/internal/domain/flow"
	domainmessage "telegram-message-forward/internal/domain/message"
	domainsink "telegram-message-forward/internal/domain/sink"
)

// Queue 根据 Flow 引擎的匹配结果生成投递任务。
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

// Enqueue 为每个 Flow 命中的每个目标渠道生成一个 pending 投递任务。
//
// Flow 任务按 (message_id, origin_type, origin_id, origin_node_id, message_revision) 幂等。
func (q *Queue) Enqueue(ctx context.Context, msg *domainmessage.NormalizedMessage, matches []domainflow.Match) error {
	revision := msg.ContentRevision
	if revision <= 0 {
		revision = 1
	}
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
				SinkID:          target.SinkID,
				TemplateID:      target.TemplateID,
				OriginType:      "flow",
				OriginID:        m.OriginID,
				OriginNodeID:    m.OriginNodeID,
				MessageRevision: revision,
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

// EnqueueEdit 按原消息实际生成过的 Flow 路由创建编辑 revision 投递任务。
func (q *Queue) EnqueueEdit(ctx context.Context, msg *domainmessage.NormalizedMessage, matches []domainflow.Match, suffix string) error {
	if msg == nil || msg.ID <= 0 || msg.ContentRevision <= 1 {
		return nil
	}
	routes, err := q.tasks.ListFlowRoutesByMessage(ctx, msg.ID)
	if err != nil {
		return err
	}
	created := false
	enabledSinks := map[int64]bool{}
	for _, route := range routes {
		enabled, err := q.sinkEnabled(ctx, route.SinkID, enabledSinks)
		if err != nil {
			return err
		}
		if !enabled {
			continue
		}
		candidate := matchMessageForRoute(matches, route)
		if candidate == nil {
			candidate = msg
		}
		snapshot := *candidate
		snapshot.ID = msg.ID
		snapshot.ContentRevision = msg.ContentRevision
		task := &domaindelivery.Task{
			MessageID:       msg.ID,
			SinkID:          route.SinkID,
			TemplateID:      route.TemplateID,
			OriginType:      "flow",
			OriginID:        route.OriginID,
			OriginNodeID:    route.OriginNodeID,
			MessageRevision: msg.ContentRevision,
			TextSuffix:      suffix,
			Status:          domaindelivery.StatusPending,
			MaxAttempts:     q.maxAttempts,
			MessageSnapshot: &snapshot,
		}
		if err := q.tasks.Create(ctx, task); err != nil {
			return err
		}
		created = true
	}
	if created {
		q.notifier.Notify()
	}
	return nil
}

func matchMessageForRoute(matches []domainflow.Match, route domaindelivery.FlowRoute) *domainmessage.NormalizedMessage {
	for _, match := range matches {
		if match.OriginID != route.OriginID || match.OriginNodeID != route.OriginNodeID {
			continue
		}
		for _, target := range match.Targets {
			if target.SinkID == route.SinkID {
				return match.Message
			}
		}
	}
	return nil
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
