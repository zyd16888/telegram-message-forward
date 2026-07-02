package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"telegram-message-forward/internal/api/dto"
	appauth "telegram-message-forward/internal/app/auth"
)

// AuthHandler 处理管理后台登录相关请求。
//
// 这些端点不经过 Auth 中间件：登录前必须可访问，用于识别鉴权模式、
// 初始化首个管理凭证与校验 token。
type AuthHandler struct {
	svc *appauth.Service
}

// NewAuthHandler 创建 auth handler。
func NewAuthHandler(svc *appauth.Service) *AuthHandler {
	return &AuthHandler{svc: svc}
}

// BootstrapStatus GET /auth/bootstrap — 返回是否可初始化首个管理员。
func (h *AuthHandler) BootstrapStatus(c *gin.Context) {
	status, err := h.svc.BootstrapStatus(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": dto.BootstrapStatusDTO{
		AuthEnabled:  status.AuthEnabled,
		CanBootstrap: status.CanBootstrap,
	}})
}

// Bootstrap POST /auth/bootstrap — 创建首个管理员并返回浏览器会话。
func (h *AuthHandler) Bootstrap(c *gin.Context) {
	var req dto.BootstrapRequest
	if !bindJSON(c, &req) {
		return
	}
	res, err := h.svc.Bootstrap(c.Request.Context(), strings.TrimSpace(req.Username), req.Password)
	if err != nil {
		var closed appauth.ErrBootstrapClosed
		if errors.As(err, &closed) {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": dto.LoginResultDTO{
		Authenticated: res.Authenticated,
		Token:         res.Token,
		Username:      res.Username,
		ExpiresAt:     res.ExpiresAt,
	}})
}

// Login POST /auth/login — 校验用户名密码并签发浏览器会话。
func (h *AuthHandler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if !bindJSON(c, &req) {
		return
	}
	res, err := h.svc.Login(c.Request.Context(), strings.TrimSpace(req.Username), req.Password)
	if err != nil {
		if errors.Is(err, appauth.ErrInvalidCredential) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": dto.LoginResultDTO{
		Authenticated: res.Authenticated,
		Token:         res.Token,
		Username:      res.Username,
		ExpiresAt:     res.ExpiresAt,
	}})
}

// Me GET /auth/me — 返回当前登录身份状态。
func (h *AuthHandler) Me(c *gin.Context) {
	token := bearerToken(c.GetHeader("Authorization"))
	id, err := h.svc.Me(c.Request.Context(), token)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": dto.MeDTO{
		AuthEnabled:    id.AuthEnabled,
		Authenticated:  id.Authenticated,
		CanBootstrap:   id.CanBootstrap,
		Username:       id.Username,
		CredentialType: id.CredentialType,
	}})
}

// Logout POST /auth/logout — 前端清理本地凭证；服务端当前无会话状态。
func (h *AuthHandler) Logout(c *gin.Context) {
	token := bearerToken(c.GetHeader("Authorization"))
	if err := h.svc.Logout(c.Request.Context(), token); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// bearerToken 解析 Authorization: Bearer <token>。
func bearerToken(header string) string {
	const prefix = "Bearer "
	if strings.HasPrefix(header, prefix) {
		return strings.TrimSpace(header[len(prefix):])
	}
	return ""
}
