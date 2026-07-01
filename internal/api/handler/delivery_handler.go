package handler

import (
	"net/http"
	"strconv"

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
	status := domaindelivery.Status(c.Query("status"))
	limit := parseIntDefault(c.Query("limit"), 50)
	offset := parseIntDefault(c.Query("offset"), 0)

	tasks, err := h.svc.List(c.Request.Context(), status, limit, offset)
	if err != nil {
		respondError(c, err)
		return
	}
	out := make([]dto.DeliveryDTO, 0, len(tasks))
	for _, t := range tasks {
		out = append(out, dto.NewDeliveryDTO(t))
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

// Get GET /deliveries/:id
func (h *DeliveryHandler) Get(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	t, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": dto.NewDeliveryDTO(t)})
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
