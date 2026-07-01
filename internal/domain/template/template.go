// Package template 定义模板领域模型与仓储接口。
package template

import (
	"context"
	"time"
)

// Format 是模板渲染格式。
type Format string

const (
	FormatText     Format = "text"
	FormatMarkdown Format = "markdown"
	FormatHTML     Format = "html"
)

// Template 是一个渲染模板。
type Template struct {
	ID        int64
	Name      string
	Format    Format
	Content   string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Repository 是模板仓储接口。
type Repository interface {
	Create(ctx context.Context, t *Template) error
	Update(ctx context.Context, t *Template) error
	GetByID(ctx context.Context, id int64) (*Template, error)
	List(ctx context.Context) ([]*Template, error)
	Delete(ctx context.Context, id int64) error
}
