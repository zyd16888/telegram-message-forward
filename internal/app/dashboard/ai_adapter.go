package dashboard

import (
	"context"

	appaidigest "telegram-message-forward/internal/app/aidigest"
)

// AIDigestStats 把 AI Digest 服务适配为 Dashboard AIStatsProvider。
type AIDigestStats struct {
	Svc *appaidigest.Service
}

func (a AIDigestStats) GlobalStats(ctx context.Context, sinceHours int) (runs, success, failed, tokens int64, err error) {
	if a.Svc == nil {
		return 0, 0, 0, 0, nil
	}
	st, err := a.Svc.GlobalStats(ctx, sinceHours)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	return st.Runs, st.Success, st.Failed, st.Tokens, nil
}

func (a AIDigestStats) CountProfiles(ctx context.Context) (int64, error) {
	if a.Svc == nil {
		return 0, nil
	}
	profiles, err := a.Svc.ListProfiles(ctx)
	if err != nil {
		return 0, err
	}
	return int64(len(profiles)), nil
}
