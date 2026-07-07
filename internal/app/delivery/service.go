// Package delivery 提供投递记录查询与手动重试的应用服务。
package delivery

import (
	"context"
	"fmt"

	domaindelivery "telegram-message-forward/internal/domain/delivery"
	domainflow "telegram-message-forward/internal/domain/flow"
	domainmessage "telegram-message-forward/internal/domain/message"
	domainsink "telegram-message-forward/internal/domain/sink"
	domainsource "telegram-message-forward/internal/domain/source"
	domaintemplate "telegram-message-forward/internal/domain/template"
)

// Service 是投递应用服务。
type Service struct {
	tasks     domaindelivery.Repository
	messages  domainmessage.Repository
	sources   domainsource.Repository
	sinks     domainsink.Repository
	flows     domainflow.Repository
	templates domaintemplate.Repository
}

// DisplayDeps 是投递记录展示所需的关联数据依赖。
type DisplayDeps struct {
	Messages  domainmessage.Repository
	Sources   domainsource.Repository
	Sinks     domainsink.Repository
	Flows     domainflow.Repository
	Templates domaintemplate.Repository
}

// View 是投递记录及其展示关联数据。
type View struct {
	Task     *domaindelivery.Task
	Attempts []*domaindelivery.Attempt
	Message  *domainmessage.NormalizedMessage
	Source   *domainsource.Source
	Sink     *domainsink.Sink
	Flow     *domainflow.Flow
	Template *domaintemplate.Template
}

type queryRepository interface {
	ListByQuery(ctx context.Context, q domaindelivery.Query) ([]*domaindelivery.Task, error)
	CountByQuery(ctx context.Context, q domaindelivery.Query) (int64, error)
}

type attemptRepository interface {
	ListAttempts(ctx context.Context, taskID int64) ([]*domaindelivery.Attempt, error)
}

type bulkRequeueRepository interface {
	RequeueByStatus(ctx context.Context, status domaindelivery.Status) (int64, error)
}

// NewService 创建投递服务。
func NewService(tasks domaindelivery.Repository, deps ...DisplayDeps) *Service {
	s := &Service{tasks: tasks}
	if len(deps) > 0 {
		s.messages = deps[0].Messages
		s.sources = deps[0].Sources
		s.sinks = deps[0].Sinks
		s.flows = deps[0].Flows
		s.templates = deps[0].Templates
	}
	return s
}

// List 按状态分页查询投递任务；status 为空返回全部。
func (s *Service) List(ctx context.Context, status domaindelivery.Status, limit, offset int) ([]*domaindelivery.Task, error) {
	return s.tasks.List(ctx, status, limit, offset)
}

// ListByQuery 按复合条件分页查询投递任务。
func (s *Service) ListByQuery(ctx context.Context, q domaindelivery.Query) ([]*domaindelivery.Task, error) {
	if repo, ok := s.tasks.(queryRepository); ok {
		return repo.ListByQuery(ctx, q)
	}
	return s.tasks.List(ctx, q.Status, q.Limit, q.Offset)
}

// Count 统计投递任务数量。
func (s *Service) Count(ctx context.Context, status domaindelivery.Status) (int64, error) {
	return s.tasks.Count(ctx, status)
}

// CountByQuery 按复合条件统计投递任务。
func (s *Service) CountByQuery(ctx context.Context, q domaindelivery.Query) (int64, error) {
	if repo, ok := s.tasks.(queryRepository); ok {
		return repo.CountByQuery(ctx, q)
	}
	return s.tasks.Count(ctx, q.Status)
}

// ListViews 按状态分页查询投递任务，并补齐页面展示所需的关联名称与消息内容。
func (s *Service) ListViews(ctx context.Context, status domaindelivery.Status, limit, offset int) ([]*View, error) {
	return s.ListViewsByQuery(ctx, domaindelivery.Query{Status: status, Limit: limit, Offset: offset})
}

// ListViewsByQuery 按复合条件分页查询投递任务，并补齐页面展示所需关联数据。
func (s *Service) ListViewsByQuery(ctx context.Context, q domaindelivery.Query) ([]*View, error) {
	tasks, err := s.ListByQuery(ctx, q)
	if err != nil {
		return nil, err
	}
	views := make([]*View, 0, len(tasks))
	for _, task := range tasks {
		view, err := s.enrich(ctx, task)
		if err != nil {
			return nil, err
		}
		views = append(views, view)
	}
	return views, nil
}

// Get 查询单个投递任务。
func (s *Service) Get(ctx context.Context, id int64) (*domaindelivery.Task, error) {
	return s.tasks.GetByID(ctx, id)
}

// GetView 查询单条投递任务并补齐展示关联数据。
func (s *Service) GetView(ctx context.Context, id int64) (*View, error) {
	task, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.enrich(ctx, task)
}

func (s *Service) enrich(ctx context.Context, task *domaindelivery.Task) (*View, error) {
	view := &View{Task: task}
	if repo, ok := s.tasks.(attemptRepository); ok {
		attempts, err := repo.ListAttempts(ctx, task.ID)
		if err != nil {
			return nil, fmt.Errorf("查询投递尝试失败 task_id=%d: %w", task.ID, err)
		}
		view.Attempts = attempts
	}

	if task.MessageSnapshot != nil {
		view.Message = task.MessageSnapshot
	} else if s.messages != nil {
		msg, err := s.messages.GetByID(ctx, task.MessageID)
		if err != nil {
			return nil, fmt.Errorf("查询投递消息失败 message_id=%d: %w", task.MessageID, err)
		}
		view.Message = msg
	}

	if view.Message != nil && view.Message.SourceID > 0 && s.sources != nil {
		src, err := s.sources.GetByID(ctx, view.Message.SourceID)
		if err != nil {
			return nil, fmt.Errorf("查询投递来源失败 source_id=%d: %w", view.Message.SourceID, err)
		}
		view.Source = src
	}

	if s.sinks != nil {
		sink, err := s.sinks.GetByID(ctx, task.SinkID)
		if err != nil {
			return nil, fmt.Errorf("查询投递目标渠道失败 sink_id=%d: %w", task.SinkID, err)
		}
		view.Sink = sink
	}

	if task.OriginType == "flow" && task.OriginID > 0 && s.flows != nil {
		flow, err := s.flows.GetByID(ctx, task.OriginID)
		if err != nil {
			return nil, fmt.Errorf("查询投递 Flow 失败 flow_id=%d: %w", task.OriginID, err)
		}
		view.Flow = flow
	}

	if task.TemplateID != nil && s.templates != nil {
		tpl, err := s.templates.GetByID(ctx, *task.TemplateID)
		if err != nil {
			return nil, fmt.Errorf("查询投递模板失败 template_id=%d: %w", *task.TemplateID, err)
		}
		view.Template = tpl
	}

	return view, nil
}

// RetryDead 将一个终态任务（dead/failed/cancelled）重置为 pending 以手动重试。
//
// 重置后 worker 会在下一轮领取并重新投递。
func (s *Service) RetryDead(ctx context.Context, id int64) error {
	task, err := s.tasks.GetByID(ctx, id)
	if err != nil {
		return err
	}
	switch task.Status {
	case domaindelivery.StatusDead, domaindelivery.StatusFailed, domaindelivery.StatusCancelled:
		return s.tasks.Requeue(ctx, id)
	default:
		return fmt.Errorf("任务状态 %s 不可手动重试（仅 dead/failed/cancelled 可重试）", task.Status)
	}
}

// RetryDeadBatch 批量重试 dead 任务。
func (s *Service) RetryDeadBatch(ctx context.Context) (int64, error) {
	if repo, ok := s.tasks.(bulkRequeueRepository); ok {
		return repo.RequeueByStatus(ctx, domaindelivery.StatusDead)
	}
	return 0, fmt.Errorf("当前存储实现不支持批量重试")
}
