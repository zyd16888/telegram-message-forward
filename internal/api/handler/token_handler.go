package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"telegram-message-forward/internal/api/dto"
	apptoken "telegram-message-forward/internal/app/apitoken"
)

// TokenHandler 处理 API token 管理请求。
type TokenHandler struct {
	svc *apptoken.Service
}

// NewTokenHandler 创建 token handler。
func NewTokenHandler(svc *apptoken.Service) *TokenHandler {
	return &TokenHandler{svc: svc}
}

// List GET /tokens
func (h *TokenHandler) List(c *gin.Context) {
	tokens, err := h.svc.List(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	out := make([]dto.TokenDTO, 0, len(tokens))
	for _, t := range tokens {
		out = append(out, dto.NewTokenDTO(t))
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

// Create POST /tokens — 返回明文 token（仅此一次）。
func (h *TokenHandler) Create(c *gin.Context) {
	var req dto.TokenCreateRequest
	if !bindJSON(c, &req) {
		return
	}
	created, err := h.svc.Create(c.Request.Context(), req.Name)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": dto.TokenCreatedDTO{
		ID: created.ID, Name: created.Name, Token: created.Token,
	}})
}

// Revoke DELETE /tokens/:id
func (h *TokenHandler) Revoke(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.svc.Revoke(c.Request.Context(), id); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
