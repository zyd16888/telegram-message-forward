package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"telegram-message-forward/internal/api/dto"
	apptemplate "telegram-message-forward/internal/app/template"
)

// TemplateHandler 处理模板相关请求。
type TemplateHandler struct {
	svc *apptemplate.Service
}

// NewTemplateHandler 创建模板 handler。
func NewTemplateHandler(svc *apptemplate.Service) *TemplateHandler {
	return &TemplateHandler{svc: svc}
}

// List GET /templates
func (h *TemplateHandler) List(c *gin.Context) {
	ts, err := h.svc.List(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	out := make([]dto.TemplateDTO, 0, len(ts))
	for _, t := range ts {
		out = append(out, dto.NewTemplateDTO(t))
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

// Get GET /templates/:id
func (h *TemplateHandler) Get(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	t, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": dto.NewTemplateDTO(t)})
}

// Preview POST /templates/preview
func (h *TemplateHandler) Preview(c *gin.Context) {
	var req dto.TemplatePreviewRequest
	if !bindJSON(c, &req) {
		return
	}
	text, err := h.svc.Preview(apptemplate.Input{Format: req.Format, Content: req.Content})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": dto.TemplatePreviewDTO{Format: req.Format, Text: text}})
}

// Create POST /templates
func (h *TemplateHandler) Create(c *gin.Context) {
	var req dto.TemplateRequest
	if !bindJSON(c, &req) {
		return
	}
	t, err := h.svc.Create(c.Request.Context(), apptemplate.Input{Name: req.Name, Format: req.Format, Content: req.Content})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": dto.NewTemplateDTO(t)})
}

// Update PUT /templates/:id
func (h *TemplateHandler) Update(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req dto.TemplateRequest
	if !bindJSON(c, &req) {
		return
	}
	t, err := h.svc.Update(c.Request.Context(), id, apptemplate.Input{Name: req.Name, Format: req.Format, Content: req.Content})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": dto.NewTemplateDTO(t)})
}

// Delete DELETE /templates/:id
func (h *TemplateHandler) Delete(c *gin.Context) {
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
