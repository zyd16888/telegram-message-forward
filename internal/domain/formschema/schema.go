// Package formschema defines lightweight field metadata for admin UI forms.
package formschema

import "fmt"

// FieldType is a coarse UI input type.
type FieldType string

const (
	FieldText        FieldType = "text"
	FieldPassword    FieldType = "password"
	FieldTextarea    FieldType = "textarea"
	FieldNumber      FieldType = "number"
	FieldBoolean     FieldType = "boolean"
	FieldSelect      FieldType = "select"
	FieldMultiSelect FieldType = "multi_select"
	FieldStringList  FieldType = "string_list"
	FieldKeyValue    FieldType = "key_value"
)

// Option is a selectable value for select-like fields.
type Option struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// FieldSpec describes one configurable field.
type FieldSpec struct {
	Key         string    `json:"key"`
	Label       string    `json:"label"`
	Type        FieldType `json:"type"`
	Required    bool      `json:"required,omitempty"`
	Secret      bool      `json:"secret,omitempty"`
	Default     any       `json:"default,omitempty"`
	Placeholder string    `json:"placeholder,omitempty"`
	Help        string    `json:"help,omitempty"`
	Options     []Option  `json:"options,omitempty"`
	Min         *float64  `json:"min,omitempty"`
	Max         *float64  `json:"max,omitempty"`
}

// Validate checks required fields against a config map. It intentionally keeps
// validation shallow; plugins still own channel-specific semantics.
func Validate(fields []FieldSpec, config map[string]any) error {
	for _, field := range fields {
		if !field.Required {
			continue
		}
		value, ok := config[field.Key]
		if !ok || isEmpty(value) {
			return fmt.Errorf("缺少必填配置: %s", field.Label)
		}
	}
	return nil
}

func isEmpty(value any) bool {
	switch v := value.(type) {
	case nil:
		return true
	case string:
		return v == ""
	case []string:
		return len(v) == 0
	case []any:
		return len(v) == 0
	case map[string]any:
		return len(v) == 0
	default:
		return false
	}
}
