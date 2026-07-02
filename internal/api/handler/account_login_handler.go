package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"telegram-message-forward/internal/api/dto"
	apptelegramlogin "telegram-message-forward/internal/app/telegramlogin"
	domainloginflow "telegram-message-forward/internal/domain/loginflow"
)

// AccountLoginHandler 处理 Telegram 账号登录 flow 请求。
//
// handler 只调用 app service，不直接接触 gotd/td。业务错误（验证码错误、过期、
// 步骤不匹配等）以 400 返回，并附带最新 flow 状态，便于 UI 更新与提示。
type AccountLoginHandler struct {
	svc *apptelegramlogin.Service
}

// NewAccountLoginHandler 创建登录 handler。
func NewAccountLoginHandler(svc *apptelegramlogin.Service) *AccountLoginHandler {
	return &AccountLoginHandler{svc: svc}
}

// Start POST /accounts/:id/login/start — 发起手机验证码登录。
func (h *AccountLoginHandler) Start(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	flow, err := h.svc.Start(c.Request.Context(), id)
	if err != nil {
		h.respondFlowError(c, flow, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": dto.NewLoginFlowDTO(flow)})
}

// Code POST /accounts/:id/login/code — 提交验证码。
func (h *AccountLoginHandler) Code(c *gin.Context) {
	if _, ok := parseID(c); !ok {
		return
	}
	var req dto.LoginCodeRequest
	if !bindJSON(c, &req) {
		return
	}
	flow, err := h.svc.SubmitCode(c.Request.Context(), req.FlowID, req.Code)
	if err != nil {
		h.respondFlowError(c, flow, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": dto.NewLoginFlowDTO(flow)})
}

// Password POST /accounts/:id/login/password — 提交两步验证密码。
func (h *AccountLoginHandler) Password(c *gin.Context) {
	if _, ok := parseID(c); !ok {
		return
	}
	var req dto.LoginPasswordRequest
	if !bindJSON(c, &req) {
		return
	}
	flow, err := h.svc.SubmitPassword(c.Request.Context(), req.FlowID, req.Password)
	if err != nil {
		h.respondFlowError(c, flow, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": dto.NewLoginFlowDTO(flow)})
}

// Status GET /accounts/:id/login/status — 返回该账号当前可恢复的登录 flow。
func (h *AccountLoginHandler) Status(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	flow, err := h.svc.GetActiveByAccount(c.Request.Context(), id)
	if err != nil {
		respondError(c, err)
		return
	}
	if flow == nil {
		c.JSON(http.StatusOK, gin.H{"data": nil})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": dto.NewLoginFlowDTO(flow)})
}

// Cancel POST /accounts/:id/login/cancel — 取消进行中的登录 flow。
func (h *AccountLoginHandler) Cancel(c *gin.Context) {
	if _, ok := parseID(c); !ok {
		return
	}
	var req dto.LoginCancelRequest
	if !bindJSON(c, &req) {
		return
	}
	flow, err := h.svc.Cancel(c.Request.Context(), req.FlowID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": dto.NewLoginFlowDTO(flow)})
}

// respondFlowError 返回业务错误与最新 flow 状态。flow 可能为 nil。
func (h *AccountLoginHandler) respondFlowError(c *gin.Context, flow *domainloginflow.Flow, err error) {
	body := gin.H{"error": err.Error()}
	if flow != nil {
		body["data"] = dto.NewLoginFlowDTO(flow)
	}
	c.JSON(http.StatusBadRequest, body)
}
