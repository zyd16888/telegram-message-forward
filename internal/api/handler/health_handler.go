// Package handler 实现 HTTP handler。
//
// handler 只处理 HTTP 协议与 DTO，调用 app service，不直接访问 GORM。
package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HealthHandler 提供健康检查。
type HealthHandler struct{}

// NewHealthHandler 创建健康检查 handler。
func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

// Health 返回服务存活状态。
func (h *HealthHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
