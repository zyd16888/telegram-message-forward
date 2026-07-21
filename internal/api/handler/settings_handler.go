package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"telegram-message-forward/internal/api/dto"
	appsettings "telegram-message-forward/internal/app/settings"
)

// SettingsHandler 处理系统设置。
type SettingsHandler struct {
	svc *appsettings.Service
}

// NewSettingsHandler 创建 handler。
func NewSettingsHandler(svc *appsettings.Service) *SettingsHandler {
	return &SettingsHandler{svc: svc}
}

// GetMedia 返回当前生效的媒体设置（脱敏）。
func (h *SettingsHandler) GetMedia(c *gin.Context) {
	ms, hasSecret, source, err := h.svc.GetMedia(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": dto.NewMediaSettingsDTO(ms, hasSecret, source)})
}

// UpdateMedia 保存媒体设置并热重载媒体存储。
func (h *SettingsHandler) UpdateMedia(c *gin.Context) {
	var req dto.MediaSettingsRequest
	if !bindJSON(c, &req) {
		return
	}
	ms, err := h.svc.UpdateMedia(c.Request.Context(), req.ToSettings(), req.S3SecretKey)
	if err != nil {
		respondError(c, err)
		return
	}
	hasSecret := req.S3SecretKey != nil && *req.S3SecretKey != ""
	if req.S3SecretKey == nil {
		_, hasSecret, _, err = h.svc.GetMedia(c.Request.Context())
		if err != nil {
			respondError(c, err)
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"data": dto.NewMediaSettingsDTO(ms, hasSecret, appsettings.SourceDatabase)})
}

// TestMediaS3 用请求里的设置测试 S3 连通性（secret 留空时使用已保存值）。
func (h *SettingsHandler) TestMediaS3(c *gin.Context) {
	var req dto.MediaSettingsRequest
	if !bindJSON(c, &req) {
		return
	}
	if err := h.svc.TestS3(c.Request.Context(), req.ToSettings(), req.S3SecretKey); err != nil {
		c.JSON(http.StatusOK, gin.H{"data": gin.H{"success": false, "error": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"success": true}})
}

// GetDataRetention 返回消息与投递记录保留策略。
func (h *SettingsHandler) GetDataRetention(c *gin.Context) {
	dr, source, err := h.svc.GetDataRetention(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": dto.NewDataRetentionDTO(dr, source)})
}

// UpdateDataRetention 保存归档保留策略（热生效）。
func (h *SettingsHandler) UpdateDataRetention(c *gin.Context) {
	var req dto.DataRetentionRequest
	if !bindJSON(c, &req) {
		return
	}
	dr, err := h.svc.UpdateDataRetention(c.Request.Context(), req.ToSettings())
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": dto.NewDataRetentionDTO(dr, appsettings.SourceDatabase)})
}

// CleanupDataRetention 按当前生效的保留策略立即清理选中的数据。
func (h *SettingsHandler) CleanupDataRetention(c *gin.Context) {
	var req dto.DataCleanupRequest
	if !bindJSON(c, &req) {
		return
	}
	targets, err := req.ToTargets()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := h.svc.RunDataCleanup(c.Request.Context(), targets)
	if err != nil {
		h.respondCleanupError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": dto.NewDataCleanupDTO(result)})
}

// CleanupMedia 按当前生效的媒体保留期立即清理选中的存储范围。
func (h *SettingsHandler) CleanupMedia(c *gin.Context) {
	var req dto.MediaCleanupRequest
	if !bindJSON(c, &req) {
		return
	}
	result, err := h.svc.RunMediaCleanup(c.Request.Context(), req.ToInput())
	if err != nil {
		h.respondCleanupError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": dto.NewMediaCleanupDTO(result)})
}

func (h *SettingsHandler) respondCleanupError(c *gin.Context, err error) {
	if errors.Is(err, appsettings.ErrCleanupInProgress) {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	respondError(c, err)
}
