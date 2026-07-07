package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"telegram-message-forward/internal/api/dto"
	appflow "telegram-message-forward/internal/app/flow"
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

func (h *FlowHandler) Get(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	f, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		respondError(c, err)
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
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "资源不存在"})
		return
	}
	c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
}
