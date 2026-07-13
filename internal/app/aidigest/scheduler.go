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
	// retentionDays 返回 AI run 保留天数；nil 或返回值 <=0 时使用默认 30（0 表示不清理需显式通过 GetRetention 返回 0）。
	retentionDays   func(ctx context.Context) int
	maintenanceMu   sync.Mutex
	lastMaintenance time.Time
}

const (
	defaultRunRetentionDays = 30
	maintenanceInterval     = 24 * time.Hour
	staleRunTimeout         = 2 * time.Hour
)

func NewScheduler(svc *Service, log *slog.Logger) *Scheduler {
	return &Scheduler{svc: svc, log: log}
}

// SetRetentionDaysFunc 注入热读的 AI run 保留天数（设置页配置）。
func (s *Scheduler) SetRetentionDaysFunc(fn func(ctx context.Context) int) {
	s.retentionDays = fn
}

func (s *Scheduler) Run(ctx context.Context) {
	s.recoverStale(ctx)
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
	s.recoverStale(ctx)
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

func (s *Scheduler) recoverStale(ctx context.Context) {
	recovered, err := s.svc.RecoverStaleRuns(ctx, staleRunTimeout)
	if err != nil {
		s.log.Warn("恢复僵死 AI 运行任务失败", "err", err)
		return
	}
	if recovered > 0 {
		s.log.Warn("已恢复僵死 AI 运行任务", "recovered", recovered)
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
	days := defaultRunRetentionDays
	if s.retentionDays != nil {
		days = s.retentionDays(ctx)
	}
	if days <= 0 {
		return // 0 = 不清理
	}
	deleted, err := s.svc.CleanupRuns(ctx, days)
	if err != nil {
		s.log.Warn("清理过期 AI 运行记录失败", "err", err)
		return
	}
	if deleted > 0 {
		s.log.Info("已清理过期 AI 运行记录", "deleted", deleted, "retention_days", days)
	}
}
