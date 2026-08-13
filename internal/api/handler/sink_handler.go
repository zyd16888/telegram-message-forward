package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"telegram-message-forward/internal/api/dto"
	appsink "telegram-message-forward/internal/app/sink"
	pluginsink "telegram-message-forward/internal/plugin/sink"
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

// Meta GET /sinks/meta
func (h *SinkHandler) Meta(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": h.svc.Descriptors()})
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

// Test POST /sinks/test — 用未保存的表单配置测试渠道。
func (h *SinkHandler) Test(c *gin.Context) {
	var req dto.SinkTestRequest
	if !bindJSON(c, &req) {
		return
	}
	result, err := h.svc.Test(c.Request.Context(), appsink.TestInput{
		Type:   req.Type,
		Config: req.Config,
		Secret: req.Secret,
		Media:  testMediaInput(req.TestMedia),
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": newSinkTestDTO(result)})
}

// TestExisting POST /sinks/:id/test — 用已有渠道测试，可传 config/secret 临时覆盖。
func (h *SinkHandler) TestExisting(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req dto.SinkTestRequest
	if !bindJSON(c, &req) {
		return
	}
	result, err := h.svc.Test(c.Request.Context(), appsink.TestInput{
		ID:     id,
		Config: req.Config,
		Secret: req.Secret,
		Media:  testMediaInput(req.TestMedia),
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": newSinkTestDTO(result)})
}

func testMediaInput(media *dto.SinkTestMedia) *appsink.TestMediaInput {
	if media == nil {
		return nil
	}
	return &appsink.TestMediaInput{Type: media.Type, URL: media.URL, FileName: media.FileName}
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

func newSinkTestDTO(result *pluginsink.Result) dto.SinkTestDTO {
	if result == nil {
		return dto.SinkTestDTO{Success: false, Error: "渠道未返回测试结果"}
	}
	return dto.SinkTestDTO{
		Success:         result.Success,
		Error:           result.Error,
		ResponseSummary: result.ResponseSummary,
	}
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
