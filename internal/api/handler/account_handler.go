package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"telegram-message-forward/internal/api/dto"
	appaccount "telegram-message-forward/internal/app/account"
)

// AccountHandler 处理账号相关请求。
type AccountHandler struct {
	svc *appaccount.Service
}

// NewAccountHandler 创建账号 handler。
func NewAccountHandler(svc *appaccount.Service) *AccountHandler {
	return &AccountHandler{svc: svc}
}

// List GET /accounts
func (h *AccountHandler) List(c *gin.Context) {
	accs, err := h.svc.List(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	out := make([]dto.AccountDTO, 0, len(accs))
	for _, a := range accs {
		out = append(out, dto.NewAccountDTO(a))
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

// Get GET /accounts/:id
func (h *AccountHandler) Get(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	acc, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": dto.NewAccountDTO(acc)})
}

// Create POST /accounts
func (h *AccountHandler) Create(c *gin.Context) {
	var req dto.AccountCreateRequest
	if !bindJSON(c, &req) {
		return
	}
	acc, err := h.svc.Create(c.Request.Context(), appaccount.CreateInput{
		Name:          req.Name,
		PhoneNumber:   req.PhoneNumber,
		TelegramAppID: req.TelegramAppID,
		ProxyID:       req.ProxyID,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": dto.NewAccountDTO(acc)})
}

// Update PUT /accounts/:id
func (h *AccountHandler) Update(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req dto.AccountUpdateRequest
	if !bindJSON(c, &req) {
		return
	}
	in := appaccount.UpdateInput{Name: req.Name, TelegramAppID: req.TelegramAppID, ProxyID: req.ProxyID, ClearProxy: req.ClearProxy}
	acc, err := h.svc.Update(c.Request.Context(), id, in)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": dto.NewAccountDTO(acc)})
}

// Delete DELETE /accounts/:id
func (h *AccountHandler) Delete(c *gin.Context) {
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
