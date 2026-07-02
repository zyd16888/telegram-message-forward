package handler

import (
	"errors"
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
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req dto.LoginCodeRequest
	if !bindJSON(c, &req) {
		return
	}
	flow, err := h.svc.SubmitCode(c.Request.Context(), id, req.FlowID, req.Code)
	if err != nil {
		h.respondFlowError(c, flow, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": dto.NewLoginFlowDTO(flow)})
}

// Password POST /accounts/:id/login/password — 提交两步验证密码。
func (h *AccountLoginHandler) Password(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req dto.LoginPasswordRequest
	if !bindJSON(c, &req) {
		return
	}
	flow, err := h.svc.SubmitPassword(c.Request.Context(), id, req.FlowID, req.Password)
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
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req dto.LoginCancelRequest
	if !bindJSON(c, &req) {
		return
	}
	flow, err := h.svc.Cancel(c.Request.Context(), id, req.FlowID)
	if err != nil {
		h.respondLoginServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": dto.NewLoginFlowDTO(flow)})
}

// QRStart POST /accounts/:id/login/qr/start — 发起扫码登录，返回二维码。
func (h *AccountLoginHandler) QRStart(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	flow, err := h.svc.StartQR(c.Request.Context(), id)
	if err != nil {
		h.respondFlowError(c, flow, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": dto.NewLoginFlowDTO(flow)})
}

// QRStatus GET /accounts/:id/login/qr/status?flow_id=... — 轮询扫码状态。
func (h *AccountLoginHandler) QRStatus(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	flowID := c.Query("flow_id")
	if flowID == "" {
		// 未带 flow_id 时回退到该账号当前活跃 flow（用于恢复）。
		active, err := h.svc.GetActiveByAccount(c.Request.Context(), id)
		if err != nil {
			respondError(c, err)
			return
		}
		if active == nil {
			c.JSON(http.StatusOK, gin.H{"data": nil})
			return
		}
		flowID = active.FlowID
	}
	flow, err := h.svc.QRStatus(c.Request.Context(), id, flowID)
	if err != nil {
		h.respondFlowError(c, flow, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": dto.NewLoginFlowDTO(flow)})
}

// QRRefresh POST /accounts/:id/login/qr/refresh — 刷新二维码。
func (h *AccountLoginHandler) QRRefresh(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req dto.LoginCancelRequest // 复用 flow_id 字段
	if !bindJSON(c, &req) {
		return
	}
	flow, err := h.svc.RefreshQR(c.Request.Context(), id, req.FlowID)
	if err != nil {
		h.respondFlowError(c, flow, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": dto.NewLoginFlowDTO(flow)})
}

// QRCancel POST /accounts/:id/login/qr/cancel — 取消扫码登录。
func (h *AccountLoginHandler) QRCancel(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req dto.LoginCancelRequest
	if !bindJSON(c, &req) {
		return
	}
	flow, err := h.svc.Cancel(c.Request.Context(), id, req.FlowID)
	if err != nil {
		h.respondLoginServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": dto.NewLoginFlowDTO(flow)})
}

// respondFlowError 返回业务错误与最新 flow 状态。flow 可能为 nil。
// account 与 flow 不匹配时按 404 处理，避免暴露该 flow_id 属于其它账号。
func (h *AccountLoginHandler) respondFlowError(c *gin.Context, flow *domainloginflow.Flow, err error) {
	if errors.Is(err, apptelegramlogin.ErrFlowAccountMismatch) {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	body := gin.H{"error": err.Error()}
	if flow != nil {
		body["data"] = dto.NewLoginFlowDTO(flow)
	}
	c.JSON(http.StatusBadRequest, body)
}

// respondLoginServiceError 处理不携带 flow 数据的登录错误：account 与 flow
// 不匹配时按 404 处理，避免暴露该 flow_id 属于其它账号；其余错误走通用分类。
func (h *AccountLoginHandler) respondLoginServiceError(c *gin.Context, err error) {
	if errors.Is(err, apptelegramlogin.ErrFlowAccountMismatch) {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	respondError(c, err)
}
