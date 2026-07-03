// Package dto 定义 HTTP API 的请求/响应数据结构。
//
// DTO 与 domain 分离：敏感字段（session、secret、app_hash、代理密码、手机号）
// 在此统一脱敏或写入后不回显，不把内部字段暴露给 UI。
package dto

import (
	"encoding/json"
	"time"

	domainaccount "telegram-message-forward/internal/domain/account"
	domainapitoken "telegram-message-forward/internal/domain/apitoken"
	domaindelivery "telegram-message-forward/internal/domain/delivery"
	domainloginflow "telegram-message-forward/internal/domain/loginflow"
	domainmessage "telegram-message-forward/internal/domain/message"
	domainrule "telegram-message-forward/internal/domain/rule"
	domainsink "telegram-message-forward/internal/domain/sink"
	domainsource "telegram-message-forward/internal/domain/source"
	domainconfig "telegram-message-forward/internal/domain/telegramconfig"
	domaintemplate "telegram-message-forward/internal/domain/template"
)

// maskPhone 手机号脱敏，仅保留末 4 位。
func maskPhone(phone string) string {
	if phone == "" {
		return ""
	}
	if len(phone) <= 4 {
		return "****"
	}
	return "****" + phone[len(phone)-4:]
}

// --- Account ---

// ProxyDTO 是脱敏后的代理配置（不含密码）。
type ProxyDTO struct {
	Type     string `json:"type,omitempty"`
	Addr     string `json:"addr,omitempty"`
	Username string `json:"username,omitempty"`
	HasPass  bool   `json:"has_password"`
}

// AccountDTO 是脱敏后的账号响应。
type AccountDTO struct {
	ID            int64      `json:"id"`
	Name          string     `json:"name"`
	PhoneNumber   string     `json:"phone_number"` // 脱敏
	TelegramAppID *int64     `json:"telegram_app_id,omitempty"`
	ProxyID       *int64     `json:"proxy_id,omitempty"`
	AppID         int        `json:"app_id"`
	Status        string     `json:"status"`
	Proxy         ProxyDTO   `json:"proxy"`
	LastLoginAt   *time.Time `json:"last_login_at,omitempty"`
	LastError     string     `json:"last_error,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// NewAccountDTO 从 domain 账号构造脱敏 DTO。
func NewAccountDTO(a *domainaccount.Account) AccountDTO {
	return AccountDTO{
		ID:            a.ID,
		Name:          a.Name,
		PhoneNumber:   maskPhone(a.PhoneNumber),
		TelegramAppID: a.TelegramAppID,
		ProxyID:       a.ProxyID,
		AppID:         a.AppID,
		Status:        string(a.Status),
		Proxy: ProxyDTO{
			Type:     a.Proxy.Type,
			Addr:     a.Proxy.Addr,
			Username: a.Proxy.Username,
			HasPass:  a.Proxy.Password != "",
		},
		LastLoginAt: a.LastLoginAt,
		LastError:   a.LastError,
		CreatedAt:   a.CreatedAt,
		UpdatedAt:   a.UpdatedAt,
	}
}

// AccountCreateRequest 是创建账号请求。
type AccountCreateRequest struct {
	Name          string `json:"name" binding:"required"`
	PhoneNumber   string `json:"phone_number" binding:"required"`
	TelegramAppID int64  `json:"telegram_app_id" binding:"required"`
	ProxyID       *int64 `json:"proxy_id,omitempty"`
}

// ProxyRequest 是代理配置请求（含明文密码，仅写入）。
type ProxyRequest struct {
	Type     string `json:"type"`
	Addr     string `json:"addr"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// --- Telegram Shared Config ---

// TelegramAppDTO 是脱敏后的 Telegram App 响应。
type TelegramAppDTO struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	AppID     int       `json:"app_id"`
	Enabled   bool      `json:"enabled"`
	HasHash   bool      `json:"has_hash"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// NewTelegramAppDTO 构造 Telegram App DTO。
func NewTelegramAppDTO(app *domainconfig.TelegramApp) TelegramAppDTO {
	return TelegramAppDTO{
		ID:        app.ID,
		Name:      app.Name,
		AppID:     app.AppID,
		Enabled:   app.Enabled,
		HasHash:   app.AppHash != "",
		CreatedAt: app.CreatedAt,
		UpdatedAt: app.UpdatedAt,
	}
}

// TelegramAppRequest 是 Telegram App 创建/更新请求。更新时 app_hash 为空表示保留原值。
type TelegramAppRequest struct {
	Name    string  `json:"name" binding:"required"`
	AppID   int     `json:"app_id" binding:"required"`
	AppHash *string `json:"app_hash,omitempty"`
	Enabled bool    `json:"enabled"`
}

// SharedProxyDTO 是脱敏后的代理配置响应。
type SharedProxyDTO struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Type        string    `json:"type"`
	Addr        string    `json:"addr"`
	Username    string    `json:"username,omitempty"`
	Enabled     bool      `json:"enabled"`
	HasPassword bool      `json:"has_password"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// NewSharedProxyDTO 构造代理 DTO。
func NewSharedProxyDTO(p *domainconfig.Proxy) SharedProxyDTO {
	return SharedProxyDTO{
		ID:          p.ID,
		Name:        p.Name,
		Type:        p.Type,
		Addr:        p.Addr,
		Username:    p.Username,
		Enabled:     p.Enabled,
		HasPassword: p.Password != "",
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
	}
}

// SharedProxyRequest 是代理配置创建/更新请求。更新时 password 为空表示保留原值。
type SharedProxyRequest struct {
	Name     string  `json:"name" binding:"required"`
	Type     string  `json:"type" binding:"required"`
	Addr     string  `json:"addr" binding:"required"`
	Username string  `json:"username"`
	Password *string `json:"password,omitempty"`
	Enabled  bool    `json:"enabled"`
}

// ToDomain 转为 domain 代理配置。
func (p *ProxyRequest) ToDomain() domainaccount.ProxyConfig {
	if p == nil {
		return domainaccount.ProxyConfig{}
	}
	return domainaccount.ProxyConfig{Type: p.Type, Addr: p.Addr, Username: p.Username, Password: p.Password}
}

// AccountUpdateRequest 是更新账号请求。
type AccountUpdateRequest struct {
	Name          *string `json:"name,omitempty"`
	TelegramAppID *int64  `json:"telegram_app_id,omitempty"`
	ProxyID       *int64  `json:"proxy_id,omitempty"`
	ClearProxy    bool    `json:"clear_proxy,omitempty"`
}

// --- Sink ---

// SinkDTO 是渠道响应（不含 secret）。
type SinkDTO struct {
	ID           int64                   `json:"id"`
	Type         string                  `json:"type"`
	Name         string                  `json:"name"`
	Enabled      bool                    `json:"enabled"`
	Config       map[string]any          `json:"config"`
	Capabilities domainsink.Capabilities `json:"capabilities"`
	HasSecret    bool                    `json:"has_secret"`
	CreatedAt    time.Time               `json:"created_at"`
	UpdatedAt    time.Time               `json:"updated_at"`
}

// NewSinkDTO 从 domain 渠道构造 DTO（secret 不回显）。
func NewSinkDTO(s *domainsink.Sink) SinkDTO {
	cfg := s.Config
	if cfg == nil {
		cfg = map[string]any{}
	}
	return SinkDTO{
		ID:           s.ID,
		Type:         s.Type,
		Name:         s.Name,
		Enabled:      s.Enabled,
		Config:       cfg,
		Capabilities: s.Capabilities,
		HasSecret:    len(s.Secret) > 0,
		CreatedAt:    s.CreatedAt,
		UpdatedAt:    s.UpdatedAt,
	}
}

// SinkCreateRequest 是创建渠道请求。
type SinkCreateRequest struct {
	Type    string         `json:"type" binding:"required"`
	Name    string         `json:"name" binding:"required"`
	Enabled *bool          `json:"enabled,omitempty"`
	Config  map[string]any `json:"config"`
	Secret  string         `json:"secret"`
}

// SinkUpdateRequest 是更新渠道请求。
type SinkUpdateRequest struct {
	Name    *string        `json:"name,omitempty"`
	Enabled *bool          `json:"enabled,omitempty"`
	Config  map[string]any `json:"config,omitempty"`
	Secret  *string        `json:"secret,omitempty"`
}

// SinkTestRequest 是渠道连通性测试请求。
type SinkTestRequest struct {
	Type   string         `json:"type,omitempty"`
	Config map[string]any `json:"config"`
	Secret *string        `json:"secret,omitempty"`
}

// SinkTestDTO 是渠道连通性测试响应。
type SinkTestDTO struct {
	Success         bool            `json:"success"`
	Error           string          `json:"error,omitempty"`
	ResponseSummary json.RawMessage `json:"response_summary,omitempty"`
}

// --- Source ---

// SourceDTO 是监听源响应。
type SourceDTO struct {
	ID            int64          `json:"id"`
	AccountID     int64          `json:"account_id"`
	PeerType      string         `json:"peer_type"`
	PeerID        int64          `json:"peer_id"`
	Name          string         `json:"name"`
	Username      string         `json:"username,omitempty"`
	Enabled       bool           `json:"enabled"`
	Config        map[string]any `json:"config"`
	LastMessageID int64          `json:"last_message_id"`
	LastSyncedAt  *time.Time     `json:"last_synced_at,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

// NewSourceDTO 从 domain 监听源构造 DTO。
func NewSourceDTO(s *domainsource.Source) SourceDTO {
	cfg := s.Config
	if cfg == nil {
		cfg = map[string]any{}
	}
	return SourceDTO{
		ID:            s.ID,
		AccountID:     s.AccountID,
		PeerType:      string(s.PeerType),
		PeerID:        s.PeerID,
		Name:          s.Name,
		Username:      s.Username,
		Enabled:       s.Enabled,
		Config:        cfg,
		LastMessageID: s.LastMessageID,
		LastSyncedAt:  s.LastSyncedAt,
		CreatedAt:     s.CreatedAt,
		UpdatedAt:     s.UpdatedAt,
	}
}

// SourceCreateRequest 是创建监听源请求。
type SourceCreateRequest struct {
	AccountID int64          `json:"account_id" binding:"required"`
	PeerType  string         `json:"peer_type" binding:"required"`
	PeerID    int64          `json:"peer_id" binding:"required"`
	Name      string         `json:"name" binding:"required"`
	Username  string         `json:"username"`
	Enabled   *bool          `json:"enabled,omitempty"`
	Config    map[string]any `json:"config"`
}

// SourceUpdateRequest 是更新监听源请求。
type SourceUpdateRequest struct {
	Name    *string        `json:"name,omitempty"`
	Enabled *bool          `json:"enabled,omitempty"`
	Config  map[string]any `json:"config,omitempty"`
}

// SyncedPeerDTO 是同步返回的可选 peer。
type SyncedPeerDTO struct {
	PeerType     string   `json:"peer_type"`
	PeerKind     string   `json:"peer_kind"`
	PeerID       int64    `json:"peer_id"`
	Name         string   `json:"name"`
	Username     string   `json:"username,omitempty"`
	DisplayType  string   `json:"display_type"`
	IsBot        bool     `json:"is_bot"`
	IsChannel    bool     `json:"is_channel"`
	IsSupergroup bool     `json:"is_supergroup"`
	IsForum      bool     `json:"is_forum"`
	Flags        []string `json:"flags,omitempty"`
	Cached       bool     `json:"cached,omitempty"`
}

// --- Template ---

// TemplateDTO 是模板响应。
type TemplateDTO struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Format    string    `json:"format"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// NewTemplateDTO 从 domain 模板构造 DTO。
func NewTemplateDTO(t *domaintemplate.Template) TemplateDTO {
	return TemplateDTO{
		ID:        t.ID,
		Name:      t.Name,
		Format:    string(t.Format),
		Content:   t.Content,
		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
	}
}

// TemplateRequest 是模板创建/更新请求。
type TemplateRequest struct {
	Name    string `json:"name" binding:"required"`
	Format  string `json:"format" binding:"required"`
	Content string `json:"content" binding:"required"`
}

// TemplatePreviewRequest 是模板预览请求。
type TemplatePreviewRequest struct {
	Format  string `json:"format" binding:"required"`
	Content string `json:"content" binding:"required"`
}

// TemplatePreviewDTO 是模板预览响应。
type TemplatePreviewDTO struct {
	Format string `json:"format"`
	Text   string `json:"text"`
}

// --- Rule ---

// RuleTargetDTO 是规则目标。
type RuleTargetDTO struct {
	SinkID     int64  `json:"sink_id"`
	TemplateID *int64 `json:"template_id,omitempty"`
}

// RuleDTO 是规则响应。
type RuleDTO struct {
	ID          int64                        `json:"id"`
	Name        string                       `json:"name"`
	Enabled     bool                         `json:"enabled"`
	Priority    int                          `json:"priority"`
	Conditions  []domainrule.ConditionConfig `json:"conditions"`
	Processors  []domainrule.ProcessorConfig `json:"processors"`
	StopOnMatch bool                         `json:"stop_on_match"`
	SourceIDs   []int64                      `json:"source_ids"`
	Targets     []RuleTargetDTO              `json:"targets"`
	CreatedAt   time.Time                    `json:"created_at"`
	UpdatedAt   time.Time                    `json:"updated_at"`
}

// NewRuleDTO 从 domain 规则构造 DTO。
func NewRuleDTO(r *domainrule.Rule) RuleDTO {
	targets := make([]RuleTargetDTO, 0, len(r.Targets))
	for _, t := range r.Targets {
		targets = append(targets, RuleTargetDTO{SinkID: t.SinkID, TemplateID: t.TemplateID})
	}
	conds := r.Conditions
	if conds == nil {
		conds = []domainrule.ConditionConfig{}
	}
	procs := r.Processors
	if procs == nil {
		procs = []domainrule.ProcessorConfig{}
	}
	srcIDs := r.SourceIDs
	if srcIDs == nil {
		srcIDs = []int64{}
	}
	return RuleDTO{
		ID:          r.ID,
		Name:        r.Name,
		Enabled:     r.Enabled,
		Priority:    r.Priority,
		Conditions:  conds,
		Processors:  procs,
		StopOnMatch: r.StopOnMatch,
		SourceIDs:   srcIDs,
		Targets:     targets,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}

// RuleRequest 是规则创建/更新请求。
type RuleRequest struct {
	Name        string                       `json:"name" binding:"required"`
	Enabled     bool                         `json:"enabled"`
	Priority    int                          `json:"priority"`
	Conditions  []domainrule.ConditionConfig `json:"conditions"`
	Processors  []domainrule.ProcessorConfig `json:"processors"`
	StopOnMatch bool                         `json:"stop_on_match"`
	SourceIDs   []int64                      `json:"source_ids"`
	Targets     []RuleTargetDTO              `json:"targets"`
}

// TargetsToDomain 转换目标列表。
func (r *RuleRequest) TargetsToDomain() []domainrule.Target {
	out := make([]domainrule.Target, 0, len(r.Targets))
	for _, t := range r.Targets {
		out = append(out, domainrule.Target{SinkID: t.SinkID, TemplateID: t.TemplateID})
	}
	return out
}

// --- Delivery ---

// DeliveryDTO 是投递任务响应。
type DeliveryDTO struct {
	ID             int64      `json:"id"`
	MessageID      int64      `json:"message_id"`
	RuleID         int64      `json:"rule_id"`
	SinkID         int64      `json:"sink_id"`
	TemplateID     *int64     `json:"template_id,omitempty"`
	Status         string     `json:"status"`
	AttemptCount   int        `json:"attempt_count"`
	MaxAttempts    int        `json:"max_attempts"`
	NextRetryAt    *time.Time `json:"next_retry_at,omitempty"`
	LastError      string     `json:"last_error,omitempty"`
	MessageText    string     `json:"message_text,omitempty"`
	MessageType    string     `json:"message_type,omitempty"`
	SenderName     string     `json:"sender_name,omitempty"`
	SourceName     string     `json:"source_name,omitempty"`
	SourceUsername string     `json:"source_username,omitempty"`
	SourcePeerType string     `json:"source_peer_type,omitempty"`
	SinkName       string     `json:"sink_name,omitempty"`
	SinkType       string     `json:"sink_type,omitempty"`
	RuleName       string     `json:"rule_name,omitempty"`
	TemplateName   string     `json:"template_name,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// NewDeliveryDTO 从 domain 投递任务构造 DTO。
func NewDeliveryDTO(t *domaindelivery.Task) DeliveryDTO {
	return DeliveryDTO{
		ID:           t.ID,
		MessageID:    t.MessageID,
		RuleID:       t.RuleID,
		SinkID:       t.SinkID,
		TemplateID:   t.TemplateID,
		Status:       string(t.Status),
		AttemptCount: t.AttemptCount,
		MaxAttempts:  t.MaxAttempts,
		NextRetryAt:  t.NextRetryAt,
		LastError:    t.LastError,
		CreatedAt:    t.CreatedAt,
		UpdatedAt:    t.UpdatedAt,
	}
}

// NewDeliveryViewDTO 从投递任务及其关联数据构造页面友好的 DTO。
func NewDeliveryViewDTO(
	t *domaindelivery.Task,
	msg *domainmessage.NormalizedMessage,
	src *domainsource.Source,
	sink *domainsink.Sink,
	rule *domainrule.Rule,
	tpl *domaintemplate.Template,
) DeliveryDTO {
	out := NewDeliveryDTO(t)
	if msg != nil {
		out.MessageText = msg.Text
		out.MessageType = msg.MessageType
		out.SenderName = msg.SenderName
	}
	if src != nil {
		out.SourceName = src.Name
		out.SourceUsername = src.Username
		out.SourcePeerType = string(src.PeerType)
	}
	if sink != nil {
		out.SinkName = sink.Name
		out.SinkType = sink.Type
	}
	if rule != nil {
		out.RuleName = rule.Name
	}
	if tpl != nil {
		out.TemplateName = tpl.Name
	}
	return out
}

// --- API Token ---

// TokenDTO 是 token 列表项（不含明文）。
type TokenDTO struct {
	ID         int64      `json:"id"`
	Name       string     `json:"name"`
	Revoked    bool       `json:"revoked"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
}

// NewTokenDTO 从 domain token 构造 DTO。
func NewTokenDTO(t *domainapitoken.Token) TokenDTO {
	return TokenDTO{
		ID:         t.ID,
		Name:       t.Name,
		Revoked:    t.RevokedAt != nil,
		CreatedAt:  t.CreatedAt,
		LastUsedAt: t.LastUsedAt,
		RevokedAt:  t.RevokedAt,
	}
}

// TokenCreateRequest 是创建 token 请求。
type TokenCreateRequest struct {
	Name string `json:"name" binding:"required"`
}

// TokenCreatedDTO 是创建 token 的响应，明文仅此一次返回。
type TokenCreatedDTO struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Token string `json:"token"`
}

// --- Telegram 登录 flow ---

// LoginFlowDTO 是登录 flow 状态响应；不包含 phone_code_hash、qr_token 等敏感字段。
//
// QRURL 仅在扫码等待时即时构造用于渲染二维码，不落库、不建议前端持久化；
// 原始 qr_token 不出现在响应中。
type LoginFlowDTO struct {
	FlowID      string     `json:"flow_id"`
	AccountID   int64      `json:"account_id"`
	Method      string     `json:"method"`
	Status      string     `json:"status"`
	CurrentStep string     `json:"current_step"`
	ExpiresAt   time.Time  `json:"expires_at"`
	QRURL       string     `json:"qr_url,omitempty"`
	QRExpiresAt *time.Time `json:"qr_expires_at,omitempty"`
	LastError   string     `json:"last_error,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// NewLoginFlowDTO 从 domain flow 构造 DTO（脱敏）。
func NewLoginFlowDTO(f *domainloginflow.Flow) LoginFlowDTO {
	d := LoginFlowDTO{
		FlowID:      f.FlowID,
		AccountID:   f.AccountID,
		Method:      string(f.Method),
		Status:      string(f.Status),
		CurrentStep: f.CurrentStep,
		ExpiresAt:   f.ExpiresAt,
		QRURL:       f.QRURL,
		LastError:   f.LastError,
		CompletedAt: f.CompletedAt,
	}
	if !f.QRTokenExpiresAt.IsZero() {
		exp := f.QRTokenExpiresAt
		d.QRExpiresAt = &exp
	}
	return d
}

// LoginCodeRequest 是提交验证码请求。
type LoginCodeRequest struct {
	FlowID string `json:"flow_id" binding:"required"`
	Code   string `json:"code" binding:"required"`
}

// LoginPasswordRequest 是提交两步验证密码请求。
type LoginPasswordRequest struct {
	FlowID   string `json:"flow_id" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginCancelRequest 是取消登录 flow 请求。
type LoginCancelRequest struct {
	FlowID string `json:"flow_id" binding:"required"`
}

// --- Auth ---

// BootstrapStatusDTO 描述首次初始化管理凭证的可用性。
type BootstrapStatusDTO struct {
	AuthEnabled  bool `json:"auth_enabled"`
	CanBootstrap bool `json:"can_bootstrap"`
}

// BootstrapRequest 是创建首个管理员请求。
type BootstrapRequest struct {
	Username string `json:"username"`
	Password string `json:"password" binding:"required"`
}

// LoginRequest 是管理后台用户名密码登录请求。
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResultDTO 是登录结果。
type LoginResultDTO struct {
	Authenticated bool      `json:"authenticated"`
	Token         string    `json:"token"`
	Username      string    `json:"username,omitempty"`
	ExpiresAt     time.Time `json:"expires_at"`
}

// MeDTO 是当前登录身份状态。
type MeDTO struct {
	AuthEnabled    bool   `json:"auth_enabled"`
	Authenticated  bool   `json:"authenticated"`
	CanBootstrap   bool   `json:"can_bootstrap"`
	Username       string `json:"username,omitempty"`
	CredentialType string `json:"credential_type,omitempty"`
}
