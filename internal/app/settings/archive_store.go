package settings

import (
	"context"
	"time"
)

// RepoArchiveStore 把消息/投递仓储适配为 ArchiveStore。
type RepoArchiveStore struct {
	Deliveries deliveryArchive
	Messages   messageArchive
}

type deliveryArchive interface {
	DeleteTerminalBefore(ctx context.Context, before time.Time, limit int) (int64, error)
}

type messageArchive interface {
	DeleteBefore(ctx context.Context, before time.Time, limit int) (int64, error)
}

// DeleteTerminalDeliveryTasksBefore 实现 ArchiveStore。
func (s RepoArchiveStore) DeleteTerminalDeliveryTasksBefore(ctx context.Context, before time.Time, limit int) (int64, error) {
	if s.Deliveries == nil {
		return 0, nil
	}
	return s.Deliveries.DeleteTerminalBefore(ctx, before, limit)
}

// DeleteMessagesBefore 实现 ArchiveStore。
func (s RepoArchiveStore) DeleteMessagesBefore(ctx context.Context, before time.Time, limit int) (int64, error) {
	if s.Messages == nil {
		return 0, nil
	}
	return s.Messages.DeleteBefore(ctx, before, limit)
}
