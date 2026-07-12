package aidigest

import (
	"context"
	"log/slog"
	"sync"
	"time"

	domainaidigest "telegram-message-forward/internal/domain/aidigest"
)

type Scheduler struct {
	svc             *Service
	log             *slog.Logger
	maintenanceMu   sync.Mutex
	lastMaintenance time.Time
}

const (
	runRetentionDays    = 30
	maintenanceInterval = 24 * time.Hour
)

func NewScheduler(svc *Service, log *slog.Logger) *Scheduler {
	return &Scheduler{svc: svc, log: log}
}

func (s *Scheduler) Run(ctx context.Context) {
	s.maintain(ctx)
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
	s.maintain(ctx)
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

func (s *Scheduler) maintain(ctx context.Context) {
	s.maintenanceMu.Lock()
	defer s.maintenanceMu.Unlock()
	now := time.Now()
	if !s.lastMaintenance.IsZero() && now.Sub(s.lastMaintenance) < maintenanceInterval {
		return
	}
	s.lastMaintenance = now
	deleted, err := s.svc.CleanupRuns(ctx, runRetentionDays)
	if err != nil {
		s.log.Warn("清理过期 AI 运行记录失败", "err", err)
		return
	}
	if deleted > 0 {
		s.log.Info("已清理过期 AI 运行记录", "deleted", deleted, "retention_days", runRetentionDays)
	}
}
