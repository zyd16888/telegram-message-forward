package dto

import (
	"time"

	appfilter "telegram-message-forward/internal/app/filter"
	domainfilter "telegram-message-forward/internal/domain/filter"
	domainflow "telegram-message-forward/internal/domain/flow"
)

// FilterDTO 是过滤器响应。
type FilterDTO struct {
	ID          int64                        `json:"id"`
	Name        string                       `json:"name"`
	Description string                       `json:"description"`
	Conditions  []domainflow.ConditionConfig `json:"conditions"`
	CreatedAt   time.Time                    `json:"created_at"`
	UpdatedAt   time.Time                    `json:"updated_at"`
}

func NewFilterDTO(f *domainfilter.Filter) FilterDTO {
	conds := f.Conditions
	if conds == nil {
		conds = []domainflow.ConditionConfig{}
	}
	return FilterDTO{
		ID:          f.ID,
		Name:        f.Name,
		Description: f.Description,
		Conditions:  conds,
		CreatedAt:   f.CreatedAt,
		UpdatedAt:   f.UpdatedAt,
	}
}

// FilterRequest 是过滤器创建/更新请求。
type FilterRequest struct {
	Name        string                       `json:"name" binding:"required"`
	Description string                       `json:"description"`
	Conditions  []domainflow.ConditionConfig `json:"conditions"`
}

func (r FilterRequest) ToInput() appfilter.Input {
	return appfilter.Input{
		Name:        r.Name,
		Description: r.Description,
		Conditions:  r.Conditions,
	}
}
