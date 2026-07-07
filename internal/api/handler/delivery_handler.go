package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"telegram-message-forward/internal/api/dto"
	appdelivery "telegram-message-forward/internal/app/delivery"
	domaindelivery "telegram-message-forward/internal/domain/delivery"
)

// DeliveryHandler 处理投递记录相关请求。
type DeliveryHandler struct {
	svc *appdelivery.Service
}

// NewDeliveryHandler 创建投递 handler。
func NewDeliveryHandler(svc *appdelivery.Service) *DeliveryHandler {
	return &DeliveryHandler{svc: svc}
}

// List GET /deliveries?status=&limit=&offset=
func (h *DeliveryHandler) List(c *gin.Context) {
	query := domaindelivery.Query{
		Status:   domaindelivery.Status(c.Query("status")),
		SourceID: parseInt64Default(c.Query("source_id"), 0),
		SinkID:   parseInt64Default(c.Query("sink_id"), 0),
		Since:    parseSinceHours(c.Query("since_hours")),
		Limit:    parseIntDefault(c.Query("limit"), 50),
		Offset:   parseIntDefault(c.Query("offset"), 0),
	}

	views, err := h.svc.ListViewsByQuery(c.Request.Context(), query)
	if err != nil {
		respondError(c, err)
		return
	}
	total, err := h.svc.CountByQuery(c.Request.Context(), query)
	if err != nil {
		respondError(c, err)
		return
	}
	out := make([]dto.DeliveryDTO, 0, len(views))
	for _, v := range views {
		out = append(out, dto.NewDeliveryViewDTO(v.Task, v.Attempts, v.Message, v.Source, v.Sink, v.Flow, v.Template))
	}
	c.JSON(http.StatusOK, gin.H{"data": out, "total": total})
}

func parseSinceHours(s string) *time.Time {
	if s == "" {
		return nil
	}
	hours, err := strconv.Atoi(s)
	if err != nil || hours <= 0 {
		return nil
	}
	t := time.Now().Add(-time.Duration(hours) * time.Hour)
	return &t
}

// Get GET /deliveries/:id
func (h *DeliveryHandler) Get(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	v, err := h.svc.GetView(c.Request.Context(), id)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": dto.NewDeliveryViewDTO(v.Task, v.Attempts, v.Message, v.Source, v.Sink, v.Flow, v.Template)})
}

// Retry POST /deliveries/:id/retry — 手动重试终态任务。
func (h *DeliveryHandler) Retry(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.svc.RetryDead(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "requeued"})
}

// RetryDeadBatch POST /deliveries/retry-dead — 批量重试 dead 任务。
func (h *DeliveryHandler) RetryDeadBatch(c *gin.Context) {
	count, err := h.svc.RetryDeadBatch(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"requeued": count})
}

func parseIntDefault(s string, def int) int {
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}

func parseInt64Default(s string, def int64) int64 {
	if s == "" {
		return def
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return def
	}
	return v
}
