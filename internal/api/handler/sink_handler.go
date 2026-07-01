package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"telegram-message-forward/internal/api/dto"
	appsink "telegram-message-forward/internal/app/sink"
)

// SinkHandler 处理渠道相关请求。
type SinkHandler struct {
	svc *appsink.Service
}

// NewSinkHandler 创建渠道 handler。
func NewSinkHandler(svc *appsink.Service) *SinkHandler {
	return &SinkHandler{svc: svc}
}

// List GET /sinks
func (h *SinkHandler) List(c *gin.Context) {
	sinks, err := h.svc.List(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	out := make([]dto.SinkDTO, 0, len(sinks))
	for _, s := range sinks {
		out = append(out, dto.NewSinkDTO(s))
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

// Types GET /sinks/types
func (h *SinkHandler) Types(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": h.svc.Types()})
}

// Get GET /sinks/:id
func (h *SinkHandler) Get(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	s, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": dto.NewSinkDTO(s)})
}

// Create POST /sinks
func (h *SinkHandler) Create(c *gin.Context) {
	var req dto.SinkCreateRequest
	if !bindJSON(c, &req) {
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	s, err := h.svc.Create(c.Request.Context(), appsink.CreateInput{
		Type:    req.Type,
		Name:    req.Name,
		Enabled: enabled,
		Config:  req.Config,
		Secret:  req.Secret,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": dto.NewSinkDTO(s)})
}

// Update PUT /sinks/:id
func (h *SinkHandler) Update(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req dto.SinkUpdateRequest
	if !bindJSON(c, &req) {
		return
	}
	s, err := h.svc.Update(c.Request.Context(), id, appsink.UpdateInput{
		Name:    req.Name,
		Enabled: req.Enabled,
		Config:  req.Config,
		Secret:  req.Secret,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": dto.NewSinkDTO(s)})
}

// Delete DELETE /sinks/:id
func (h *SinkHandler) Delete(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
