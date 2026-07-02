package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"telegram-message-forward/internal/api/dto"
	appconfig "telegram-message-forward/internal/app/telegramconfig"
)

// TelegramConfigHandler 处理 Telegram App 与代理配置。
type TelegramConfigHandler struct {
	svc *appconfig.Service
}

// NewTelegramConfigHandler 创建 handler。
func NewTelegramConfigHandler(svc *appconfig.Service) *TelegramConfigHandler {
	return &TelegramConfigHandler{svc: svc}
}

func (h *TelegramConfigHandler) ListApps(c *gin.Context) {
	apps, err := h.svc.ListApps(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	out := make([]dto.TelegramAppDTO, 0, len(apps))
	for _, app := range apps {
		out = append(out, dto.NewTelegramAppDTO(app))
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

func (h *TelegramConfigHandler) CreateApp(c *gin.Context) {
	var req dto.TelegramAppRequest
	if !bindJSON(c, &req) {
		return
	}
	app, err := h.svc.CreateApp(c.Request.Context(), appconfig.AppInput{
		Name: req.Name, AppID: req.AppID, AppHash: req.AppHash, Enabled: req.Enabled,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": dto.NewTelegramAppDTO(app)})
}

func (h *TelegramConfigHandler) UpdateApp(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req dto.TelegramAppRequest
	if !bindJSON(c, &req) {
		return
	}
	app, err := h.svc.UpdateApp(c.Request.Context(), id, appconfig.AppInput{
		Name: req.Name, AppID: req.AppID, AppHash: req.AppHash, Enabled: req.Enabled,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": dto.NewTelegramAppDTO(app)})
}

func (h *TelegramConfigHandler) DeleteApp(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.svc.DeleteApp(c.Request.Context(), id); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *TelegramConfigHandler) ListProxies(c *gin.Context) {
	proxies, err := h.svc.ListProxies(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	out := make([]dto.SharedProxyDTO, 0, len(proxies))
	for _, p := range proxies {
		out = append(out, dto.NewSharedProxyDTO(p))
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

func (h *TelegramConfigHandler) CreateProxy(c *gin.Context) {
	var req dto.SharedProxyRequest
	if !bindJSON(c, &req) {
		return
	}
	p, err := h.svc.CreateProxy(c.Request.Context(), appconfig.ProxyInput{
		Name: req.Name, Type: req.Type, Addr: req.Addr, Username: req.Username, Password: req.Password, Enabled: req.Enabled,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": dto.NewSharedProxyDTO(p)})
}

func (h *TelegramConfigHandler) UpdateProxy(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req dto.SharedProxyRequest
	if !bindJSON(c, &req) {
		return
	}
	p, err := h.svc.UpdateProxy(c.Request.Context(), id, appconfig.ProxyInput{
		Name: req.Name, Type: req.Type, Addr: req.Addr, Username: req.Username, Password: req.Password, Enabled: req.Enabled,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": dto.NewSharedProxyDTO(p)})
}

func (h *TelegramConfigHandler) DeleteProxy(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.svc.DeleteProxy(c.Request.Context(), id); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
