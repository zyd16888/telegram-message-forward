// Package delivery 提供投递记录查询与手动重试的应用服务。
package delivery

import (
	"context"
	"fmt"

	domaindelivery "telegram-message-forward/internal/domain/delivery"
	domainmessage "telegram-message-forward/internal/domain/message"
	domainrule "telegram-message-forward/internal/domain/rule"
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
	rules     domainrule.Repository
	templates domaintemplate.Repository
}

// DisplayDeps 是投递记录展示所需的关联数据依赖。
type DisplayDeps struct {
	Messages  domainmessage.Repository
	Sources   domainsource.Repository
	Sinks     domainsink.Repository
	Rules     domainrule.Repository
	Templates domaintemplate.Repository
}

// View 是投递记录及其展示关联数据。
type View struct {
	Task     *domaindelivery.Task
	Message  *domainmessage.NormalizedMessage
	Source   *domainsource.Source
	Sink     *domainsink.Sink
	Rule     *domainrule.Rule
	Template *domaintemplate.Template
}

// NewService 创建投递服务。
func NewService(tasks domaindelivery.Repository, deps ...DisplayDeps) *Service {
	s := &Service{tasks: tasks}
	if len(deps) > 0 {
		s.messages = deps[0].Messages
		s.sources = deps[0].Sources
		s.sinks = deps[0].Sinks
		s.rules = deps[0].Rules
		s.templates = deps[0].Templates
	}
	return s
}

// List 按状态分页查询投递任务；status 为空返回全部。
func (s *Service) List(ctx context.Context, status domaindelivery.Status, limit, offset int) ([]*domaindelivery.Task, error) {
	return s.tasks.List(ctx, status, limit, offset)
}

// ListViews 按状态分页查询投递任务，并补齐页面展示所需的关联名称与消息内容。
func (s *Service) ListViews(ctx context.Context, status domaindelivery.Status, limit, offset int) ([]*View, error) {
	tasks, err := s.List(ctx, status, limit, offset)
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

	if task.MessageSnapshot != nil {
		view.Message = task.MessageSnapshot
	} else if s.messages != nil {
		msg, err := s.messages.GetByID(ctx, task.MessageID)
		if err != nil {
			return nil, fmt.Errorf("查询投递消息失败 message_id=%d: %w", task.MessageID, err)
		}
		view.Message = msg
	}

	if view.Message != nil && s.sources != nil {
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

	if s.rules != nil {
		rule, err := s.rules.GetByID(ctx, task.RuleID)
		if err != nil {
			return nil, fmt.Errorf("查询投递规则失败 rule_id=%d: %w", task.RuleID, err)
		}
		view.Rule = rule
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
