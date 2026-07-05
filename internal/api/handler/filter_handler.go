package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"telegram-message-forward/internal/api/dto"
	appfilter "telegram-message-forward/internal/app/filter"
)

// FilterHandler 处理可复用过滤器相关请求。
type FilterHandler struct {
	svc *appfilter.Service
}

func NewFilterHandler(svc *appfilter.Service) *FilterHandler {
	return &FilterHandler{svc: svc}
}

// List GET /filters
func (h *FilterHandler) List(c *gin.Context) {
	fs, err := h.svc.List(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	out := make([]dto.FilterDTO, 0, len(fs))
	for _, f := range fs {
		out = append(out, dto.NewFilterDTO(f))
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

// Get GET /filters/:id
func (h *FilterHandler) Get(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	f, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": dto.NewFilterDTO(f)})
}

// Create POST /filters
func (h *FilterHandler) Create(c *gin.Context) {
	var req dto.FilterRequest
	if !bindJSON(c, &req) {
		return
	}
	f, err := h.svc.Create(c.Request.Context(), req.ToInput())
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": dto.NewFilterDTO(f)})
}

// Update PUT /filters/:id
func (h *FilterHandler) Update(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req dto.FilterRequest
	if !bindJSON(c, &req) {
		return
	}
	f, err := h.svc.Update(c.Request.Context(), id, req.ToInput())
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": dto.NewFilterDTO(f)})
}

// Delete DELETE /filters/:id
func (h *FilterHandler) Delete(c *gin.Context) {
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
