package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	appdashboard "telegram-message-forward/internal/app/dashboard"
)

// DashboardHandler 处理首页聚合统计。
type DashboardHandler struct {
	svc *appdashboard.Service
}

// NewDashboardHandler 创建 handler。
func NewDashboardHandler(svc *appdashboard.Service) *DashboardHandler {
	return &DashboardHandler{svc: svc}
}

// Summary GET /dashboard/summary?since_hours=24
func (h *DashboardHandler) Summary(c *gin.Context) {
	hours := 24
	if s := c.Query("since_hours"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			hours = n
		}
	}
	sum, err := h.svc.Summary(c.Request.Context(), hours)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": toDashboardSummaryDTO(sum)})
}

func toDashboardSummaryDTO(s *appdashboard.Summary) gin.H {
	mapBuckets := func(items []appdashboard.FailureBucket) []gin.H {
		out := make([]gin.H, 0, len(items))
		for _, it := range items {
			out = append(out, gin.H{"key": it.Key, "label": it.Label, "count": it.Count})
		}
		return out
	}
	status := s.Status
	if status == nil {
		status = map[string]int64{}
	}
	return gin.H{
		"since_hours":  s.SinceHours,
		"window_total": s.WindowTotal,
		"resources": gin.H{
			"accounts": s.Resources.Accounts,
			"sources":  s.Resources.Sources,
			"sinks":    s.Resources.Sinks,
			"flows":    s.Resources.Flows,
		},
		"status": status,
		"queue": gin.H{
			"pending":    s.Queue.Pending,
			"processing": s.Queue.Processing,
			"retrying":   s.Queue.Retrying,
		},
		"top_failures": gin.H{
			"sink":   mapBuckets(s.TopFailures.Sink),
			"flow":   mapBuckets(s.TopFailures.Flow),
			"source": mapBuckets(s.TopFailures.Source),
		},
		"setup": gin.H{
			"has_telegram_app":       s.Setup.HasTelegramApp,
			"has_active_account":     s.Setup.HasActiveAccount,
			"has_enabled_source":     s.Setup.HasEnabledSource,
			"has_enabled_sink":       s.Setup.HasEnabledSink,
			"has_enabled_flow":       s.Setup.HasEnabledFlow,
			"has_media_public_url":   s.Setup.HasMediaPublicURL,
			"media_url_recommended":  s.Setup.MediaURLRecommended,
		},
		"ai": gin.H{
			"profiles": s.AI.Profiles,
			"runs":     s.AI.Runs,
			"success":  s.AI.Success,
			"failed":   s.AI.Failed,
			"tokens":   s.AI.Tokens,
		},
	}
}
