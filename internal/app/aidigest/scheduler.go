package aidigest

import (
	"context"
	"log/slog"
	"time"

	domainaidigest "telegram-message-forward/internal/domain/aidigest"
)

type Scheduler struct {
	svc *Service
	log *slog.Logger
}

func NewScheduler(svc *Service, log *slog.Logger) *Scheduler {
	return &Scheduler{svc: svc, log: log}
}

func (s *Scheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

func (s *Scheduler) Tick(ctx context.Context) {
	s.tick(ctx)
}

func (s *Scheduler) tick(ctx context.Context) {
	profiles, err := s.svc.DueProfiles(ctx)
	if err != nil {
		s.log.Warn("扫描 AI 整理定时任务失败", "err", err)
		return
	}
	for _, p := range profiles {
		profile := p
		go func() {
			if _, err := s.svc.RunProfile(ctx, profile.ID, domainaidigest.TriggerSchedule); err != nil {
				s.log.Warn("AI 整理定时执行失败", "profile_id", profile.ID, "err", err)
			}
		}()
	}
}
