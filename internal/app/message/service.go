// Package message 提供消息中心只读查询用例。
package message

import (
	"context"
	"errors"
	"strings"

	domainmessage "telegram-message-forward/internal/domain/message"
)

var ErrInvalidQuery = errors.New("消息查询条件无效")

type Service struct {
	repo domainmessage.QueryRepository
}

func NewService(repo domainmessage.QueryRepository) *Service {
	return &Service{repo: repo}
}

func (s *Service) List(ctx context.Context, q domainmessage.Query) ([]domainmessage.ListItem, bool, error) {
	q.Keyword = strings.TrimSpace(q.Keyword)
	q.SourceType = strings.TrimSpace(q.SourceType)
	q.MessageType = strings.TrimSpace(q.MessageType)
	q.DeliveryStatus = strings.TrimSpace(q.DeliveryStatus)
	if q.Limit <= 0 {
		q.Limit = 30
	}
	if q.Limit > 100 {
		q.Limit = 100
	}
	if q.BeforeID < 0 || (q.From != nil && q.To != nil && !q.From.Before(*q.To)) {
		return nil, false, ErrInvalidQuery
	}
	return s.repo.List(ctx, q)
}

func (s *Service) Get(ctx context.Context, id int64) (*domainmessage.Detail, error) {
	if id <= 0 {
		return nil, ErrInvalidQuery
	}
	return s.repo.GetDetail(ctx, id)
}
