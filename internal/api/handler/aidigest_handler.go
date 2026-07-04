package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"telegram-message-forward/internal/api/dto"
	appaidigest "telegram-message-forward/internal/app/aidigest"
	domainaidigest "telegram-message-forward/internal/domain/aidigest"
)

type AIDigestHandler struct {
	svc *appaidigest.Service
}

func NewAIDigestHandler(svc *appaidigest.Service) *AIDigestHandler {
	return &AIDigestHandler{svc: svc}
}

func (h *AIDigestHandler) GetProvider(c *gin.Context) {
	cfg, err := h.svc.GetProvider(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": dto.NewAIProviderDTO(cfg)})
}

func (h *AIDigestHandler) UpdateProvider(c *gin.Context) {
	var req dto.AIProviderRequest
	if !bindJSON(c, &req) {
		return
	}
	cfg, err := h.svc.UpdateProvider(c.Request.Context(), req.ToInput())
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": dto.NewAIProviderDTO(cfg)})
}

func (h *AIDigestHandler) TestProvider(c *gin.Context) {
	text, err := h.svc.TestProvider(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"data": gin.H{"success": false, "error": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"success": true, "text": text}})
}

func (h *AIDigestHandler) ListProfiles(c *gin.Context) {
	profiles, err := h.svc.ListProfiles(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	out := make([]dto.AIDigestProfileDTO, 0, len(profiles))
	for _, p := range profiles {
		out = append(out, dto.NewAIDigestProfileDTO(p))
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

func (h *AIDigestHandler) CreateProfile(c *gin.Context) {
	var req dto.AIDigestProfileRequest
	if !bindJSON(c, &req) {
		return
	}
	p, err := h.svc.CreateProfile(c.Request.Context(), req.ToInput())
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": dto.NewAIDigestProfileDTO(p)})
}

func (h *AIDigestHandler) GetProfile(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	p, err := h.svc.GetProfile(c.Request.Context(), id)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": dto.NewAIDigestProfileDTO(p)})
}

func (h *AIDigestHandler) UpdateProfile(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req dto.AIDigestProfileRequest
	if !bindJSON(c, &req) {
		return
	}
	p, err := h.svc.UpdateProfile(c.Request.Context(), id, req.ToInput())
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": dto.NewAIDigestProfileDTO(p)})
}

func (h *AIDigestHandler) DeleteProfile(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.svc.DeleteProfile(c.Request.Context(), id); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *AIDigestHandler) PreviewDraft(c *gin.Context) {
	var req dto.AIDigestProfileRequest
	if !bindJSON(c, &req) {
		return
	}
	detail, err := h.svc.PreviewDraft(c.Request.Context(), req.ToInput())
	if err != nil && detail == nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": dto.NewAIDigestRunDetailDTO(detail)})
}

func (h *AIDigestHandler) PreviewProfile(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	detail, err := h.svc.PreviewProfile(c.Request.Context(), id)
	if err != nil && detail == nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": dto.NewAIDigestRunDetailDTO(detail)})
}

func (h *AIDigestHandler) RunProfile(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	detail, err := h.svc.RunProfile(c.Request.Context(), id, domainaidigest.TriggerManual)
	if err != nil && detail == nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": dto.NewAIDigestRunDetailDTO(detail)})
}

func (h *AIDigestHandler) ListRuns(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	runs, err := h.svc.ListRuns(c.Request.Context(), id, parseIntDefault(c.Query("limit"), 50), parseIntDefault(c.Query("offset"), 0))
	if err != nil {
		respondError(c, err)
		return
	}
	out := make([]dto.AIDigestRunDTO, 0, len(runs))
	for _, run := range runs {
		out = append(out, dto.NewAIDigestRunDTO(run))
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

func (h *AIDigestHandler) GetRun(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	detail, err := h.svc.GetRunDetail(c.Request.Context(), id)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": dto.NewAIDigestRunDetailDTO(detail)})
}

func (h *AIDigestHandler) CancelRun(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.svc.CancelRun(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "cancelled"})
}

func (h *AIDigestHandler) DeliverRun(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	ids, err := h.svc.DeliverRun(c.Request.Context(), id)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"delivery_task_ids": ids}})
}

func (h *AIDigestHandler) CleanupRuns(c *gin.Context) {
	days := parseIntDefault(c.Query("retention_days"), 30)
	n, err := h.svc.CleanupRuns(c.Request.Context(), days)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"deleted": n}})
}
