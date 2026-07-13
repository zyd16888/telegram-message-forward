package dashboard

import (
	"context"
	"time"

	"telegram-message-forward/internal/storage/repository"
)

// DeliveryRepoStats 把 DeliveryRepository 适配为 DeliveryStats。
type DeliveryRepoStats struct {
	Repo *repository.DeliveryRepository
}

// CountByStatusSince 实现 DeliveryStats。
func (d DeliveryRepoStats) CountByStatusSince(ctx context.Context, since *time.Time) (map[string]int64, error) {
	return d.Repo.CountByStatusSince(ctx, since)
}

// CountQueue 实现 DeliveryStats。
func (d DeliveryRepoStats) CountQueue(ctx context.Context) (pending, processing, retrying int64, err error) {
	return d.Repo.CountQueue(ctx)
}

// FailureTopSince 实现 DeliveryStats。
func (d DeliveryRepoStats) FailureTopSince(ctx context.Context, since time.Time, limit int) (sinks, flows, sources []FailureBucket, err error) {
	sRows, fRows, srcRows, err := d.Repo.FailureTopSince(ctx, since, limit)
	if err != nil {
		return nil, nil, nil, err
	}
	return toBuckets(sRows), toBuckets(fRows), toBuckets(srcRows), nil
}

func toBuckets(rows []repository.FailureTopRow) []FailureBucket {
	out := make([]FailureBucket, 0, len(rows))
	for _, r := range rows {
		out = append(out, FailureBucket{Key: r.Key, Label: r.Label, Count: r.Count})
	}
	return out
}
