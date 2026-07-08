package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"telegram-message-forward/internal/api/dto"
	appflow "telegram-message-forward/internal/app/flow"
	domainflow "telegram-message-forward/internal/domain/flow"
)

type FlowHandler struct {
	svc *appflow.Service
}

func NewFlowHandler(svc *appflow.Service) *FlowHandler {
	return &FlowHandler{svc: svc}
}

func (h *FlowHandler) List(c *gin.Context) {
	flows, err := h.svc.List(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	out := make([]dto.FlowDTO, 0, len(flows))
	for _, f := range flows {
		out = append(out, dto.NewFlowDTO(f))
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

func (h *FlowHandler) Meta(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"conditions": h.svc.ConditionDescriptors(),
		"processors": h.svc.ProcessorDescriptors(),
	}})
}

func (h *FlowHandler) Preview(c *gin.Context) {
	var req dto.FlowPreviewRequest
	if !bindJSON(c, &req) {
		return
	}
	result, err := h.svc.Preview(c.Request.Context(), appflow.PreviewInput{
		Flow: toFlowInput(req.Flow),
		Message: appflow.PreviewMessage{
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
		respondFlowError(c, err)
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

func (h *FlowHandler) Get(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	f, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		respondFlowError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": dto.NewFlowDTO(f)})
}

func (h *FlowHandler) Create(c *gin.Context) {
	var req dto.FlowRequest
	if !bindJSON(c, &req) {
		return
	}
	f, err := h.svc.Create(c.Request.Context(), toFlowInput(req))
	if err != nil {
		respondFlowError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": dto.NewFlowDTO(f)})
}

func (h *FlowHandler) Update(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req dto.FlowRequest
	if !bindJSON(c, &req) {
		return
	}
	f, err := h.svc.Update(c.Request.Context(), id, toFlowInput(req))
	if err != nil {
		respondFlowError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": dto.NewFlowDTO(f)})
}

func (h *FlowHandler) Delete(c *gin.Context) {
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

func toFlowInput(req dto.FlowRequest) appflow.Input {
	return appflow.Input{
		Name:        req.Name,
		Enabled:     req.Enabled,
		Priority:    req.Priority,
		StopOnMatch: req.StopOnMatch,
		Nodes:       req.NodesToDomain(),
		Edges:       req.EdgesToDomain(),
	}
}

func respondFlowError(c *gin.Context, err error) {
	if errors.Is(err, domainflow.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "资源不存在"})
		return
	}
	c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
}
