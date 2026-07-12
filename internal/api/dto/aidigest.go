package dto

import (
	"encoding/json"
	"time"

	appaidigest "telegram-message-forward/internal/app/aidigest"
	domainaidigest "telegram-message-forward/internal/domain/aidigest"
	domainflow "telegram-message-forward/internal/domain/flow"
	domainmessage "telegram-message-forward/internal/domain/message"
)

type AIProviderDTO struct {
	ID                 string  `json:"id"`
	Name               string  `json:"name"`
	ProviderType       string  `json:"provider_type"`
	APIType            string  `json:"api_type"`
	BaseURL            string  `json:"base_url"`
	Model              string  `json:"model"`
	TimeoutSeconds     int     `json:"timeout_seconds"`
	MaxRetries         int     `json:"max_retries"`
	DefaultTemperature float64 `json:"default_temperature"`
	SupportsVision     bool    `json:"supports_vision"`
	VisionModel        string  `json:"vision_model,omitempty"`
	Enabled            bool    `json:"enabled"`
	IsDefault          bool    `json:"is_default"`
	HasAPIKey          bool    `json:"has_api_key"`
}

type AIProviderRequest struct {
	Name               string  `json:"name"`
	ProviderType       string  `json:"provider_type"`
	APIType            string  `json:"api_type"`
	BaseURL            string  `json:"base_url"`
	Model              string  `json:"model"`
	TimeoutSeconds     int     `json:"timeout_seconds"`
	MaxRetries         int     `json:"max_retries"`
	DefaultTemperature float64 `json:"default_temperature"`
	SupportsVision     bool    `json:"supports_vision"`
	VisionModel        string  `json:"vision_model"`
	Enabled            bool    `json:"enabled"`
	IsDefault          bool    `json:"is_default"`
	APIKey             *string `json:"api_key,omitempty"`
}

func NewAIProviderDTO(cfg domainaidigest.ProviderConfig) AIProviderDTO {
	return AIProviderDTO{
		ID:                 cfg.ID,
		Name:               cfg.Name,
		ProviderType:       cfg.ProviderType,
		APIType:            cfg.APIType,
		BaseURL:            cfg.BaseURL,
		Model:              cfg.Model,
		TimeoutSeconds:     cfg.TimeoutSeconds,
		MaxRetries:         cfg.MaxRetries,
		DefaultTemperature: cfg.DefaultTemperature,
		SupportsVision:     cfg.SupportsVision,
		VisionModel:        cfg.VisionModel,
		Enabled:            cfg.Enabled,
		IsDefault:          cfg.IsDefault,
		HasAPIKey:          cfg.HasAPIKey,
	}
}

func (r AIProviderRequest) ToInput() appaidigest.ProviderInput {
	return appaidigest.ProviderInput{
		Name:               r.Name,
		ProviderType:       r.ProviderType,
		APIType:            r.APIType,
		BaseURL:            r.BaseURL,
		Model:              r.Model,
		TimeoutSeconds:     r.TimeoutSeconds,
		MaxRetries:         r.MaxRetries,
		DefaultTemperature: r.DefaultTemperature,
		SupportsVision:     r.SupportsVision,
		VisionModel:        r.VisionModel,
		Enabled:            r.Enabled,
		IsDefault:          r.IsDefault,
		APIKey:             r.APIKey,
	}
}

type AIDigestPresetDTO = domainaidigest.Preset

type AIDigestOutputTemplateDTO struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Format      string    `json:"format"`
	Content     string    `json:"content"`
	BuiltIn     bool      `json:"built_in"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type AIDigestOutputTemplateRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Format      string `json:"format"`
	Content     string `json:"content"`
}

func NewAIDigestOutputTemplateDTO(t *domainaidigest.OutputTemplate) AIDigestOutputTemplateDTO {
	return AIDigestOutputTemplateDTO{
		ID:          t.ID,
		Name:        t.Name,
		Description: t.Description,
		Format:      t.Format,
		Content:     t.Content,
		BuiltIn:     t.BuiltIn,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
	}
}

func (r AIDigestOutputTemplateRequest) ToInput() appaidigest.OutputTemplateInput {
	return appaidigest.OutputTemplateInput{
		Name:        r.Name,
		Description: r.Description,
		Format:      r.Format,
		Content:     r.Content,
	}
}

type AIDigestProfileDTO struct {
	ID               int64                           `json:"id"`
	Name             string                          `json:"name"`
	Enabled          bool                            `json:"enabled"`
	SourceIDs        []int64                         `json:"source_ids"`
	FilterID         int64                           `json:"filter_id"`
	Conditions       []domainflow.ConditionConfig    `json:"conditions"`
	Schedule         domainaidigest.ScheduleConfig   `json:"schedule"`
	Window           domainaidigest.WindowConfig     `json:"window"`
	Dedupe           domainaidigest.DedupeConfig     `json:"dedupe"`
	PromptTemplate   string                          `json:"prompt_template"`
	OutputFormat     string                          `json:"output_format"`
	OutputTemplateID int64                           `json:"output_template_id"`
	OutputTemplate   string                          `json:"output_template"`
	TargetSinkIDs    []int64                         `json:"target_sink_ids"`
	ModelConfig      domainaidigest.ModelConfig      `json:"model_config"`
	Limits           domainaidigest.LimitsConfig     `json:"limits"`
	Multimodal       domainaidigest.MultimodalConfig `json:"multimodal"`
	RecentRun        *AIDigestRunDTO                 `json:"recent_run,omitempty"`
	CreatedAt        time.Time                       `json:"created_at"`
	UpdatedAt        time.Time                       `json:"updated_at"`
}

type AIDigestProfileRequest struct {
	Name             string                          `json:"name"`
	Enabled          bool                            `json:"enabled"`
	SourceIDs        []int64                         `json:"source_ids"`
	FilterID         int64                           `json:"filter_id"`
	Conditions       []domainflow.ConditionConfig    `json:"conditions"`
	Schedule         domainaidigest.ScheduleConfig   `json:"schedule"`
	Window           domainaidigest.WindowConfig     `json:"window"`
	Dedupe           domainaidigest.DedupeConfig     `json:"dedupe"`
	PromptTemplate   string                          `json:"prompt_template"`
	OutputFormat     string                          `json:"output_format"`
	OutputTemplateID int64                           `json:"output_template_id"`
	OutputTemplate   string                          `json:"output_template"`
	TargetSinkIDs    []int64                         `json:"target_sink_ids"`
	ModelConfig      domainaidigest.ModelConfig      `json:"model_config"`
	Limits           domainaidigest.LimitsConfig     `json:"limits"`
	Multimodal       domainaidigest.MultimodalConfig `json:"multimodal"`
}

func NewAIDigestProfileDTO(p *domainaidigest.Profile) AIDigestProfileDTO {
	var recent *AIDigestRunDTO
	if p.RecentRun != nil {
		dto := NewAIDigestRunDTO(p.RecentRun)
		recent = &dto
	}
	return AIDigestProfileDTO{
		ID:               p.ID,
		Name:             p.Name,
		Enabled:          p.Enabled,
		SourceIDs:        p.SourceIDs,
		FilterID:         p.FilterID,
		Conditions:       p.Conditions,
		Schedule:         p.Schedule,
		Window:           p.Window,
		Dedupe:           p.Dedupe,
		PromptTemplate:   p.PromptTemplate,
		OutputFormat:     p.OutputFormat,
		OutputTemplateID: p.OutputTemplateID,
		OutputTemplate:   p.OutputTemplate,
		TargetSinkIDs:    p.TargetSinkIDs,
		ModelConfig:      p.ModelConfig,
		Limits:           p.Limits,
		Multimodal:       p.Multimodal,
		RecentRun:        recent,
		CreatedAt:        p.CreatedAt,
		UpdatedAt:        p.UpdatedAt,
	}
}

func (r AIDigestProfileRequest) ToInput() appaidigest.ProfileInput {
	return appaidigest.ProfileInput{
		Name:             r.Name,
		Enabled:          r.Enabled,
		SourceIDs:        r.SourceIDs,
		FilterID:         r.FilterID,
		Conditions:       r.Conditions,
		Schedule:         r.Schedule,
		Window:           r.Window,
		Dedupe:           r.Dedupe,
		PromptTemplate:   r.PromptTemplate,
		OutputFormat:     r.OutputFormat,
		OutputTemplateID: r.OutputTemplateID,
		OutputTemplate:   r.OutputTemplate,
		TargetSinkIDs:    r.TargetSinkIDs,
		ModelConfig:      r.ModelConfig,
		Limits:           r.Limits,
		Multimodal:       r.Multimodal,
	}
}

type AIDigestRunDTO struct {
	ID                int64                       `json:"id"`
	ProfileID         int64                       `json:"profile_id,omitempty"`
	Status            string                      `json:"status"`
	TriggerType       string                      `json:"trigger_type"`
	WindowStart       time.Time                   `json:"window_start"`
	WindowEnd         time.Time                   `json:"window_end"`
	InputMessageCount int                         `json:"input_message_count"`
	IncludedCount     int                         `json:"included_count"`
	ExcludedCount     int                         `json:"excluded_count"`
	DeliveryTaskIDs   []int64                     `json:"delivery_task_ids"`
	ProviderID        string                      `json:"provider_id,omitempty"`
	ProviderName      string                      `json:"provider_name,omitempty"`
	ModelName         string                      `json:"model_name,omitempty"`
	TokenUsage        domainaidigest.TokenUsage   `json:"token_usage"`
	MediaAudit        []domainaidigest.MediaAudit `json:"media_audit,omitempty"`
	Error             string                      `json:"error,omitempty"`
	StartedAt         *time.Time                  `json:"started_at,omitempty"`
	FinishedAt        *time.Time                  `json:"finished_at,omitempty"`
	CreatedAt         time.Time                   `json:"created_at"`
}

func NewAIDigestRunDTO(r *domainaidigest.Run) AIDigestRunDTO {
	return AIDigestRunDTO{
		ID:                r.ID,
		ProfileID:         r.ProfileID,
		Status:            string(r.Status),
		TriggerType:       string(r.TriggerType),
		WindowStart:       r.WindowStart,
		WindowEnd:         r.WindowEnd,
		InputMessageCount: r.InputMessageCount,
		IncludedCount:     r.IncludedCount,
		ExcludedCount:     r.ExcludedCount,
		DeliveryTaskIDs:   r.DeliveryTaskIDs,
		ProviderID:        r.ProviderID,
		ProviderName:      r.ProviderName,
		ModelName:         r.ModelName,
		TokenUsage:        r.TokenUsage,
		MediaAudit:        r.MediaAudit,
		Error:             r.Error,
		StartedAt:         r.StartedAt,
		FinishedAt:        r.FinishedAt,
		CreatedAt:         r.CreatedAt,
	}
}

type AIDigestRunItemDTO struct {
	MessageID int64                  `json:"message_id"`
	SourceID  int64                  `json:"source_id"`
	Included  bool                   `json:"included"`
	Reason    string                 `json:"reason,omitempty"`
	SortOrder int                    `json:"sort_order"`
	Message   *AIDigestMessageRefDTO `json:"message,omitempty"`
}

type AIDigestMessageRefDTO struct {
	ID                int64                         `json:"id"`
	SourceID          int64                         `json:"source_id"`
	ExternalMessageID int64                         `json:"external_message_id,omitempty"`
	GroupedID         *int64                        `json:"grouped_id,omitempty"`
	MessageType       string                        `json:"message_type"`
	SenderPeerType    string                        `json:"sender_peer_type,omitempty"`
	SenderID          int64                         `json:"sender_id,omitempty"`
	SenderName        string                        `json:"sender_name,omitempty"`
	Text              string                        `json:"text,omitempty"`
	OriginalURL       string                        `json:"original_url,omitempty"`
	SentAt            *time.Time                    `json:"sent_at,omitempty"`
	ReceivedAt        time.Time                     `json:"received_at"`
	CreatedAt         time.Time                     `json:"created_at"`
	Media             []domainaidigest.MessageMedia `json:"media,omitempty"`
	Links             []domainmessage.Link          `json:"links,omitempty"`
}

type AIDigestRequestAuditDTO struct {
	SystemPrompt  string                       `json:"system_prompt"`
	UserPrompt    string                       `json:"user_prompt"`
	RequestConfig domainaidigest.RequestConfig `json:"request_config"`
}

type AIDigestOutputDTO struct {
	ID        int64           `json:"id"`
	RunID     int64           `json:"run_id"`
	Format    string          `json:"format"`
	Title     string          `json:"title,omitempty"`
	Content   string          `json:"content"`
	Raw       json.RawMessage `json:"raw_response,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
}

type AIDigestRunDetailDTO struct {
	Run     AIDigestRunDTO           `json:"run"`
	Items   []AIDigestRunItemDTO     `json:"items"`
	Output  *AIDigestOutputDTO       `json:"output,omitempty"`
	Request *AIDigestRequestAuditDTO `json:"request,omitempty"`
}

func NewAIDigestRunDetailDTO(d *domainaidigest.RunDetail) AIDigestRunDetailDTO {
	items := make([]AIDigestRunItemDTO, 0, len(d.Items))
	for _, item := range d.Items {
		dto := AIDigestRunItemDTO{
			MessageID: item.MessageID,
			SourceID:  item.SourceID,
			Included:  item.Included,
			Reason:    item.Reason,
			SortOrder: item.SortOrder,
		}
		snapshot := item.MessageSnapshot
		if snapshot == nil {
			snapshot = domainaidigest.NewMessageSnapshot(item.Message)
		}
		if snapshot != nil {
			dto.Message = &AIDigestMessageRefDTO{
				ID: snapshot.ID, SourceID: snapshot.SourceID, ExternalMessageID: snapshot.ExternalMessageID,
				GroupedID: snapshot.GroupedID, MessageType: snapshot.MessageType, SenderPeerType: snapshot.SenderPeerType,
				SenderID: snapshot.SenderID, SenderName: snapshot.SenderName, Text: snapshot.Text,
				OriginalURL: snapshot.OriginalURL, SentAt: snapshot.SentAt, ReceivedAt: snapshot.ReceivedAt,
				CreatedAt: snapshot.CreatedAt, Media: snapshot.Media, Links: snapshot.Links,
			}
		}
		items = append(items, dto)
	}
	var out *AIDigestOutputDTO
	if d.Output != nil {
		out = &AIDigestOutputDTO{
			ID:        d.Output.ID,
			RunID:     d.Output.RunID,
			Format:    d.Output.Format,
			Title:     d.Output.Title,
			Content:   d.Output.Content,
			Raw:       json.RawMessage(d.Output.RawResponse),
			CreatedAt: d.Output.CreatedAt,
		}
	}
	var request *AIDigestRequestAuditDTO
	if d.Run.SystemPrompt != "" || d.Run.UserPrompt != "" {
		request = &AIDigestRequestAuditDTO{
			SystemPrompt: d.Run.SystemPrompt, UserPrompt: d.Run.UserPrompt, RequestConfig: d.Run.RequestConfig,
		}
	}
	return AIDigestRunDetailDTO{
		Run:     NewAIDigestRunDTO(d.Run),
		Items:   items,
		Output:  out,
		Request: request,
	}
}
