// Package dto 定义 HTTP API 的请求/响应数据结构。
//
// DTO 与 domain 分离：敏感字段（session、secret、app_hash、代理密码、手机号）
// 在此统一脱敏或写入后不回显，不把内部字段暴露给 UI。
package dto

import (
	"encoding/json"
	"time"

	appdelivery "telegram-message-forward/internal/app/delivery"
	domainaccount "telegram-message-forward/internal/domain/account"
	domainapitoken "telegram-message-forward/internal/domain/apitoken"
	domaindelivery "telegram-message-forward/internal/domain/delivery"
	domainflow "telegram-message-forward/internal/domain/flow"
	domainloginflow "telegram-message-forward/internal/domain/loginflow"
	domainmessage "telegram-message-forward/internal/domain/message"
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
	ID            int64                   `json:"id"`
	Type          string                  `json:"type"`
	Name          string                  `json:"name"`
	Enabled       bool                    `json:"enabled"`
	Config        map[string]any          `json:"config"`
	Capabilities  domainsink.Capabilities `json:"capabilities"`
	Observability SinkObservabilityDTO    `json:"observability"`
	HasSecret     bool                    `json:"has_secret"`
	CreatedAt     time.Time               `json:"created_at"`
	UpdatedAt     time.Time               `json:"updated_at"`
}

type SinkObservabilityDTO struct {
	LastTestAt         *time.Time `json:"last_test_at,omitempty"`
	LastTestSuccess    bool       `json:"last_test_success"`
	LastTestError      string     `json:"last_test_error,omitempty"`
	RecentFailure      string     `json:"recent_failure,omitempty"`
	DeliveryTotal24h   int64      `json:"delivery_total_24h"`
	DeliverySuccess24h int64      `json:"delivery_success_24h"`
	SuccessRate24h     float64    `json:"success_rate_24h"`
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
		Observability: SinkObservabilityDTO{
			LastTestAt:         s.Observability.LastTestAt,
			LastTestSuccess:    s.Observability.LastTestSuccess,
			LastTestError:      s.Observability.LastTestError,
			RecentFailure:      s.Observability.RecentFailure,
			DeliveryTotal24h:   s.Observability.DeliveryTotal24h,
			DeliverySuccess24h: s.Observability.DeliverySuccess24h,
			SuccessRate24h:     s.Observability.SuccessRate24h,
		},
		HasSecret: len(s.Secret) > 0,
		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,
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
	ID                    int64          `json:"id"`
	Type                  string         `json:"type"`
	AccountID             int64          `json:"account_id"`
	PeerType              string         `json:"peer_type"`
	PeerID                int64          `json:"peer_id"`
	Name                  string         `json:"name"`
	Username              string         `json:"username,omitempty"`
	Enabled               bool           `json:"enabled"`
	Config                map[string]any `json:"config"`
	LastMessageID         int64          `json:"last_message_id"`
	LastSyncedAt          *time.Time     `json:"last_synced_at,omitempty"`
	RunnerStatus          string         `json:"runner_status,omitempty"`
	RunnerSubscriptions   int            `json:"runner_subscriptions,omitempty"`
	RunnerRecentMessageAt *time.Time     `json:"runner_recent_message_at,omitempty"`
	RunnerLastError       string         `json:"runner_last_error,omitempty"`
	CreatedAt             time.Time      `json:"created_at"`
	UpdatedAt             time.Time      `json:"updated_at"`
}

// NewSourceDTO 从 domain 监听源构造 DTO。
func NewSourceDTO(s *domainsource.Source) SourceDTO {
	cfg := s.Config
	if cfg == nil {
		cfg = map[string]any{}
	}
	return SourceDTO{
		ID:            s.ID,
		Type:          sourceType(s.Type),
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

// NewSourceDTOWithRuntime 从 domain 监听源构造带运行状态的 DTO。
func NewSourceDTOWithRuntime(s *domainsource.Source, status string, subscriptions int, recentMessageAt *time.Time, lastError string) SourceDTO {
	out := NewSourceDTO(s)
	out.RunnerStatus = status
	out.RunnerSubscriptions = subscriptions
	out.RunnerRecentMessageAt = recentMessageAt
	out.RunnerLastError = lastError
	return out
}

// SourceCreateRequest 是创建监听源请求。
type SourceCreateRequest struct {
	Type      string         `json:"type,omitempty"`
	AccountID int64          `json:"account_id,omitempty"`
	PeerType  string         `json:"peer_type,omitempty"`
	PeerID    int64          `json:"peer_id,omitempty"`
	Name      string         `json:"name" binding:"required"`
	Username  string         `json:"username"`
	Enabled   *bool          `json:"enabled,omitempty"`
	Config    map[string]any `json:"config"`
}

func sourceType(t string) string {
	if t == "" {
		return "telegram"
	}
	return t
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

type RulePreviewMessageRequest struct {
	SourceID       int64                 `json:"source_id"`
	MessageType    string                `json:"message_type"`
	SenderPeerType string                `json:"sender_peer_type"`
	SenderID       int64                 `json:"sender_id"`
	SenderName     string                `json:"sender_name"`
	Text           string                `json:"text"`
	Media          []domainmessage.Media `json:"media,omitempty"`
	OriginalURL    string                `json:"original_url,omitempty"`
}

type RulePreviewTargetDTO struct {
	SinkID     int64  `json:"sink_id"`
	TemplateID *int64 `json:"template_id,omitempty"`
}

type RulePreviewDTO struct {
	Matched       bool                   `json:"matched"`
	ProcessedText string                 `json:"processed_text"`
	Media         []domainmessage.Media  `json:"media,omitempty"`
	Targets       []RulePreviewTargetDTO `json:"targets"`
}

// --- Flow ---

type FlowNodeDTO struct {
	ID         int64                 `json:"id"`
	Type       string                `json:"type"`
	RefID      *int64                `json:"ref_id,omitempty"`
	Config     domainflow.NodeConfig `json:"config"`
	TemplateID *int64                `json:"template_id,omitempty"`
	PosX       float64               `json:"pos_x"`
	PosY       float64               `json:"pos_y"`
}

type FlowEdgeDTO struct {
	ID         int64 `json:"id"`
	FromNodeID int64 `json:"from_node_id"`
	ToNodeID   int64 `json:"to_node_id"`
}

type FlowDTO struct {
	ID          int64         `json:"id"`
	Name        string        `json:"name"`
	Enabled     bool          `json:"enabled"`
	Priority    int           `json:"priority"`
	StopOnMatch bool          `json:"stop_on_match"`
	Nodes       []FlowNodeDTO `json:"nodes"`
	Edges       []FlowEdgeDTO `json:"edges"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
}

type FlowRequest struct {
	Name        string        `json:"name" binding:"required"`
	Enabled     bool          `json:"enabled"`
	Priority    int           `json:"priority"`
	StopOnMatch bool          `json:"stop_on_match"`
	Nodes       []FlowNodeDTO `json:"nodes" binding:"required"`
	Edges       []FlowEdgeDTO `json:"edges"`
}

type FlowPreviewRequest struct {
	Flow    FlowRequest               `json:"flow" binding:"required"`
	Message RulePreviewMessageRequest `json:"message" binding:"required"`
}

func NewFlowDTO(f *domainflow.Flow) FlowDTO {
	nodes := make([]FlowNodeDTO, 0, len(f.Nodes))
	for _, n := range f.Nodes {
		nodes = append(nodes, FlowNodeDTO{
			ID:         n.ID,
			Type:       string(n.Type),
			RefID:      n.RefID,
			Config:     n.Config,
			TemplateID: n.TemplateID,
			PosX:       n.PosX,
			PosY:       n.PosY,
		})
	}
	edges := make([]FlowEdgeDTO, 0, len(f.Edges))
	for _, e := range f.Edges {
		edges = append(edges, FlowEdgeDTO{ID: e.ID, FromNodeID: e.FromNodeID, ToNodeID: e.ToNodeID})
	}
	return FlowDTO{
		ID:          f.ID,
		Name:        f.Name,
		Enabled:     f.Enabled,
		Priority:    f.Priority,
		StopOnMatch: f.StopOnMatch,
		Nodes:       nodes,
		Edges:       edges,
		CreatedAt:   f.CreatedAt,
		UpdatedAt:   f.UpdatedAt,
	}
}

func (r *FlowRequest) NodesToDomain() []domainflow.Node {
	out := make([]domainflow.Node, 0, len(r.Nodes))
	for _, n := range r.Nodes {
		out = append(out, domainflow.Node{
			ID:         n.ID,
			Type:       domainflow.NodeType(n.Type),
			RefID:      n.RefID,
			Config:     n.Config,
			TemplateID: n.TemplateID,
			PosX:       n.PosX,
			PosY:       n.PosY,
		})
	}
	return out
}

func (r *FlowRequest) EdgesToDomain() []domainflow.Edge {
	out := make([]domainflow.Edge, 0, len(r.Edges))
	for _, e := range r.Edges {
		out = append(out, domainflow.Edge{ID: e.ID, FromNodeID: e.FromNodeID, ToNodeID: e.ToNodeID})
	}
	return out
}

// --- Delivery ---

// DeliveryDTO 是投递任务响应。
type DeliveryDTO struct {
	ID             int64                `json:"id"`
	MessageID      int64                `json:"message_id"`
	OriginType     string               `json:"origin_type,omitempty"`
	OriginID       int64                `json:"origin_id,omitempty"`
	OriginNodeID   int64                `json:"origin_node_id,omitempty"`
	EngineName     string               `json:"engine_name,omitempty"`
	SinkID         int64                `json:"sink_id"`
	TemplateID     *int64               `json:"template_id,omitempty"`
	Status         string               `json:"status"`
	AttemptCount   int                  `json:"attempt_count"`
	MaxAttempts    int                  `json:"max_attempts"`
	NextRetryAt    *time.Time           `json:"next_retry_at,omitempty"`
	LastError          string               `json:"last_error,omitempty"`
	LastErrorReadable  string               `json:"last_error_readable,omitempty"`
	MessageText        string               `json:"message_text,omitempty"`
	MessageType    string               `json:"message_type,omitempty"`
	SenderName     string               `json:"sender_name,omitempty"`
	SourceName     string               `json:"source_name,omitempty"`
	SourceUsername string               `json:"source_username,omitempty"`
	SourcePeerType string               `json:"source_peer_type,omitempty"`
	SinkName       string               `json:"sink_name,omitempty"`
	SinkType       string               `json:"sink_type,omitempty"`
	FlowName       string               `json:"flow_name,omitempty"`
	TemplateName   string               `json:"template_name,omitempty"`
	Attempts       []DeliveryAttemptDTO `json:"attempts,omitempty"`
	CreatedAt      time.Time            `json:"created_at"`
	UpdatedAt      time.Time            `json:"updated_at"`
}

type DeliveryAttemptDTO struct {
	ID              int64           `json:"id"`
	DeliveryTaskID  int64           `json:"delivery_task_id"`
	AttemptNo       int             `json:"attempt_no"`
	Status          string          `json:"status"`
	RequestSummary  json.RawMessage `json:"request_summary,omitempty"`
	ResponseSummary json.RawMessage `json:"response_summary,omitempty"`
	Error           string          `json:"error,omitempty"`
	ErrorReadable   string          `json:"error_readable,omitempty"`
	StartedAt       *time.Time      `json:"started_at,omitempty"`
	FinishedAt      *time.Time      `json:"finished_at,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
}

// NewDeliveryDTO 从 domain 投递任务构造 DTO。
func NewDeliveryDTO(t *domaindelivery.Task) DeliveryDTO {
	return DeliveryDTO{
		ID:                t.ID,
		MessageID:         t.MessageID,
		OriginType:        deliveryOriginType(t),
		OriginID:          t.OriginID,
		OriginNodeID:      t.OriginNodeID,
		EngineName:        deliveryEngineName(t),
		SinkID:            t.SinkID,
		TemplateID:        t.TemplateID,
		Status:            string(t.Status),
		AttemptCount:      t.AttemptCount,
		MaxAttempts:       t.MaxAttempts,
		NextRetryAt:       t.NextRetryAt,
		LastError:         t.LastError,
		LastErrorReadable: appdelivery.HumanizeError(t.LastError),
		CreatedAt:         t.CreatedAt,
		UpdatedAt:         t.UpdatedAt,
	}
}

// NewDeliveryViewDTO 从投递任务及其关联数据构造页面友好的 DTO。
func NewDeliveryViewDTO(
	t *domaindelivery.Task,
	attempts []*domaindelivery.Attempt,
	msg *domainmessage.NormalizedMessage,
	src *domainsource.Source,
	sink *domainsink.Sink,
	flow *domainflow.Flow,
	tpl *domaintemplate.Template,
) DeliveryDTO {
	out := NewDeliveryDTO(t)
	if len(attempts) > 0 {
		out.Attempts = make([]DeliveryAttemptDTO, 0, len(attempts))
		for _, attempt := range attempts {
			out.Attempts = append(out.Attempts, NewDeliveryAttemptDTO(attempt))
		}
	}
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
	if flow != nil {
		out.FlowName = flow.Name
	}
	if tpl != nil {
		out.TemplateName = tpl.Name
	}
	return out
}

func deliveryOriginType(t *domaindelivery.Task) string {
	if t.OriginType != "" {
		return t.OriginType
	}
	return "flow"
}

func deliveryEngineName(t *domaindelivery.Task) string {
	switch deliveryOriginType(t) {
	case "flow":
		return "Flow 引擎"
	case "ai_digest":
		return "AI 整理"
	default:
		return "Flow 引擎"
	}
}

func NewDeliveryAttemptDTO(a *domaindelivery.Attempt) DeliveryAttemptDTO {
	return DeliveryAttemptDTO{
		ID:              a.ID,
		DeliveryTaskID:  a.DeliveryTaskID,
		AttemptNo:       a.AttemptNo,
		Status:          string(a.Status),
		RequestSummary:  json.RawMessage(a.RequestSummary),
		ResponseSummary: json.RawMessage(a.ResponseSummary),
		Error:           a.Error,
		ErrorReadable:   appdelivery.HumanizeError(a.Error),
		StartedAt:       a.StartedAt,
		FinishedAt:      a.FinishedAt,
		CreatedAt:       a.CreatedAt,
	}
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
