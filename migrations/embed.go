// Package migrations 内嵌 goose SQL migration 文件，供 cmd/migrate 使用。
package migrations

import "embed"

// FS 内嵌所有 SQL migration 文件。
//
//go:embed *.sql
var FS embed.FS
