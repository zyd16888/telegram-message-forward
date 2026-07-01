// Package delivery 提供投递记录查询与手动重试的应用服务。
package delivery

import (
	"context"
	"fmt"

	domaindelivery "telegram-message-forward/internal/domain/delivery"
)

// Service 是投递应用服务。
type Service struct {
	tasks domaindelivery.Repository
}

// NewService 创建投递服务。
func NewService(tasks domaindelivery.Repository) *Service {
	return &Service{tasks: tasks}
}

// List 按状态分页查询投递任务；status 为空返回全部。
func (s *Service) List(ctx context.Context, status domaindelivery.Status, limit, offset int) ([]*domaindelivery.Task, error) {
	return s.tasks.List(ctx, status, limit, offset)
}

// Get 查询单个投递任务。
func (s *Service) Get(ctx context.Context, id int64) (*domaindelivery.Task, error) {
	return s.tasks.GetByID(ctx, id)
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
