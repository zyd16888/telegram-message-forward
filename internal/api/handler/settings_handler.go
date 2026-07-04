package handler

import (
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
