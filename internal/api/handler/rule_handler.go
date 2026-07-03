package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"telegram-message-forward/internal/api/dto"
	apprule "telegram-message-forward/internal/app/rule"
)

// RuleHandler 处理规则相关请求。
type RuleHandler struct {
	svc *apprule.Service
}

// NewRuleHandler 创建规则 handler。
func NewRuleHandler(svc *apprule.Service) *RuleHandler {
	return &RuleHandler{svc: svc}
}

// List GET /rules
func (h *RuleHandler) List(c *gin.Context) {
	rs, err := h.svc.List(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	out := make([]dto.RuleDTO, 0, len(rs))
	for _, r := range rs {
		out = append(out, dto.NewRuleDTO(r))
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

// Meta GET /rules/meta
func (h *RuleHandler) Meta(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"conditions": h.svc.ConditionDescriptors(),
		"processors": h.svc.ProcessorDescriptors(),
	}})
}

// Preview POST /rules/preview
func (h *RuleHandler) Preview(c *gin.Context) {
	var req dto.RulePreviewRequest
	if !bindJSON(c, &req) {
		return
	}
	result, err := h.svc.Preview(c.Request.Context(), apprule.PreviewInput{
		Rule: toRuleInput(req.Rule),
		Message: apprule.PreviewMessage{
			SourceID:       req.Message.SourceID,
			MessageType:    req.Message.MessageType,
			SenderPeerType: req.Message.SenderPeerType,
			SenderID:       req.Message.SenderID,
			SenderName:     req.Message.SenderName,
			Text:           req.Message.Text,
			Media:          req.Message.Media,
			OriginalURL:    req.Message.OriginalURL,
		},
	})
	if err != nil {
		respondError(c, err)
		return
	}
	targets := make([]dto.RulePreviewTargetDTO, 0, len(result.Targets))
	for _, target := range result.Targets {
		targets = append(targets, dto.RulePreviewTargetDTO{SinkID: target.SinkID, TemplateID: target.TemplateID})
	}
	c.JSON(http.StatusOK, gin.H{"data": dto.RulePreviewDTO{
		Matched:       result.Matched,
		ProcessedText: result.ProcessedText,
		Media:         result.Media,
		Targets:       targets,
	}})
}

// Get GET /rules/:id
func (h *RuleHandler) Get(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	r, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": dto.NewRuleDTO(r)})
}

// Create POST /rules
func (h *RuleHandler) Create(c *gin.Context) {
	var req dto.RuleRequest
	if !bindJSON(c, &req) {
		return
	}
	r, err := h.svc.Create(c.Request.Context(), toRuleInput(req))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": dto.NewRuleDTO(r)})
}

// Update PUT /rules/:id
func (h *RuleHandler) Update(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req dto.RuleRequest
	if !bindJSON(c, &req) {
		return
	}
	r, err := h.svc.Update(c.Request.Context(), id, toRuleInput(req))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": dto.NewRuleDTO(r)})
}

// Delete DELETE /rules/:id
func (h *RuleHandler) Delete(c *gin.Context) {
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

func toRuleInput(req dto.RuleRequest) apprule.Input {
	return apprule.Input{
		Name:        req.Name,
		Enabled:     req.Enabled,
		Priority:    req.Priority,
		Conditions:  req.Conditions,
		Processors:  req.Processors,
		StopOnMatch: req.StopOnMatch,
		SourceIDs:   req.SourceIDs,
		Targets:     req.TargetsToDomain(),
	}
}
