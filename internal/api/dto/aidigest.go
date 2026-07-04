package dto

import (
	"encoding/json"
	"time"

	appaidigest "telegram-message-forward/internal/app/aidigest"
	domainaidigest "telegram-message-forward/internal/domain/aidigest"
	domainmessage "telegram-message-forward/internal/domain/message"
	domainrule "telegram-message-forward/internal/domain/rule"
)

type AIProviderDTO struct {
	ProviderType       string  `json:"provider_type"`
	BaseURL            string  `json:"base_url"`
	Model              string  `json:"model"`
	TimeoutSeconds     int     `json:"timeout_seconds"`
	MaxRetries         int     `json:"max_retries"`
	DefaultTemperature float64 `json:"default_temperature"`
	HasAPIKey          bool    `json:"has_api_key"`
}

type AIProviderRequest struct {
	ProviderType       string  `json:"provider_type"`
	BaseURL            string  `json:"base_url"`
	Model              string  `json:"model"`
	TimeoutSeconds     int     `json:"timeout_seconds"`
	MaxRetries         int     `json:"max_retries"`
	DefaultTemperature float64 `json:"default_temperature"`
	APIKey             *string `json:"api_key,omitempty"`
}

func NewAIProviderDTO(cfg domainaidigest.ProviderConfig) AIProviderDTO {
	return AIProviderDTO{
		ProviderType:       cfg.ProviderType,
		BaseURL:            cfg.BaseURL,
		Model:              cfg.Model,
		TimeoutSeconds:     cfg.TimeoutSeconds,
		MaxRetries:         cfg.MaxRetries,
		DefaultTemperature: cfg.DefaultTemperature,
		HasAPIKey:          cfg.HasAPIKey,
	}
}

func (r AIProviderRequest) ToInput() appaidigest.ProviderInput {
	return appaidigest.ProviderInput{
		ProviderType:       r.ProviderType,
		BaseURL:            r.BaseURL,
		Model:              r.Model,
		TimeoutSeconds:     r.TimeoutSeconds,
		MaxRetries:         r.MaxRetries,
		DefaultTemperature: r.DefaultTemperature,
		APIKey:             r.APIKey,
	}
}

type AIDigestProfileDTO struct {
	ID             int64                         `json:"id"`
	Name           string                        `json:"name"`
	Enabled        bool                          `json:"enabled"`
	SourceIDs      []int64                       `json:"source_ids"`
	Conditions     []domainrule.ConditionConfig  `json:"conditions"`
	Schedule       domainaidigest.ScheduleConfig `json:"schedule"`
	Window         domainaidigest.WindowConfig   `json:"window"`
	Dedupe         domainaidigest.DedupeConfig   `json:"dedupe"`
	PromptTemplate string                        `json:"prompt_template"`
	OutputFormat   string                        `json:"output_format"`
	TargetSinkIDs  []int64                       `json:"target_sink_ids"`
	ModelConfig    domainaidigest.ModelConfig    `json:"model_config"`
	Limits         domainaidigest.LimitsConfig   `json:"limits"`
	RecentRun      *AIDigestRunDTO               `json:"recent_run,omitempty"`
	CreatedAt      time.Time                     `json:"created_at"`
	UpdatedAt      time.Time                     `json:"updated_at"`
}

type AIDigestProfileRequest struct {
	Name           string                        `json:"name"`
	Enabled        bool                          `json:"enabled"`
	SourceIDs      []int64                       `json:"source_ids"`
	Conditions     []domainrule.ConditionConfig  `json:"conditions"`
	Schedule       domainaidigest.ScheduleConfig `json:"schedule"`
	Window         domainaidigest.WindowConfig   `json:"window"`
	Dedupe         domainaidigest.DedupeConfig   `json:"dedupe"`
	PromptTemplate string                        `json:"prompt_template"`
	OutputFormat   string                        `json:"output_format"`
	TargetSinkIDs  []int64                       `json:"target_sink_ids"`
	ModelConfig    domainaidigest.ModelConfig    `json:"model_config"`
	Limits         domainaidigest.LimitsConfig   `json:"limits"`
}

func NewAIDigestProfileDTO(p *domainaidigest.Profile) AIDigestProfileDTO {
	var recent *AIDigestRunDTO
	if p.RecentRun != nil {
		dto := NewAIDigestRunDTO(p.RecentRun)
		recent = &dto
	}
	return AIDigestProfileDTO{
		ID:             p.ID,
		Name:           p.Name,
		Enabled:        p.Enabled,
		SourceIDs:      p.SourceIDs,
		Conditions:     p.Conditions,
		Schedule:       p.Schedule,
		Window:         p.Window,
		Dedupe:         p.Dedupe,
		PromptTemplate: p.PromptTemplate,
		OutputFormat:   p.OutputFormat,
		TargetSinkIDs:  p.TargetSinkIDs,
		ModelConfig:    p.ModelConfig,
		Limits:         p.Limits,
		RecentRun:      recent,
		CreatedAt:      p.CreatedAt,
		UpdatedAt:      p.UpdatedAt,
	}
}

func (r AIDigestProfileRequest) ToInput() appaidigest.ProfileInput {
	return appaidigest.ProfileInput{
		Name:           r.Name,
		Enabled:        r.Enabled,
		SourceIDs:      r.SourceIDs,
		Conditions:     r.Conditions,
		Schedule:       r.Schedule,
		Window:         r.Window,
		Dedupe:         r.Dedupe,
		PromptTemplate: r.PromptTemplate,
		OutputFormat:   r.OutputFormat,
		TargetSinkIDs:  r.TargetSinkIDs,
		ModelConfig:    r.ModelConfig,
		Limits:         r.Limits,
	}
}

type AIDigestRunDTO struct {
	ID                int64                     `json:"id"`
	ProfileID         int64                     `json:"profile_id,omitempty"`
	Status            string                    `json:"status"`
	TriggerType       string                    `json:"trigger_type"`
	WindowStart       time.Time                 `json:"window_start"`
	WindowEnd         time.Time                 `json:"window_end"`
	InputMessageCount int                       `json:"input_message_count"`
	IncludedCount     int                       `json:"included_count"`
	ExcludedCount     int                       `json:"excluded_count"`
	DeliveryTaskIDs   []int64                   `json:"delivery_task_ids"`
	ModelName         string                    `json:"model_name,omitempty"`
	TokenUsage        domainaidigest.TokenUsage `json:"token_usage"`
	Error             string                    `json:"error,omitempty"`
	StartedAt         *time.Time                `json:"started_at,omitempty"`
	FinishedAt        *time.Time                `json:"finished_at,omitempty"`
	CreatedAt         time.Time                 `json:"created_at"`
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
		ModelName:         r.ModelName,
		TokenUsage:        r.TokenUsage,
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
	ID          int64                 `json:"id"`
	SourceID    int64                 `json:"source_id"`
	MessageType string                `json:"message_type"`
	SenderName  string                `json:"sender_name,omitempty"`
	Text        string                `json:"text,omitempty"`
	OriginalURL string                `json:"original_url,omitempty"`
	SentAt      *time.Time            `json:"sent_at,omitempty"`
	ReceivedAt  time.Time             `json:"received_at"`
	Media       []domainmessage.Media `json:"media,omitempty"`
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
	Run    AIDigestRunDTO       `json:"run"`
	Items  []AIDigestRunItemDTO `json:"items"`
	Output *AIDigestOutputDTO   `json:"output,omitempty"`
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
		if item.Message != nil {
			dto.Message = &AIDigestMessageRefDTO{
				ID:          item.Message.ID,
				SourceID:    item.Message.SourceID,
				MessageType: item.Message.MessageType,
				SenderName:  item.Message.SenderName,
				Text:        item.Message.Text,
				OriginalURL: item.Message.OriginalURL,
				SentAt:      item.Message.SentAt,
				ReceivedAt:  item.Message.ReceivedAt,
				Media:       item.Message.Media,
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
	return AIDigestRunDetailDTO{
		Run:    NewAIDigestRunDTO(d.Run),
		Items:  items,
		Output: out,
	}
}
