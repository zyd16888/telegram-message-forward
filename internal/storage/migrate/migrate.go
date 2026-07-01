// Package migrate 封装 goose SQL migration 的执行，
// 供 cmd/migrate 命令与启动时自动迁移复用，保证两处行为一致。
package migrate

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/pressly/goose/v3"

	"telegram-message-forward/migrations"
)

// Run 在给定连接上执行 goose 命令（up / down / status / version 等）。
// args 传递给具体命令，例如 up-to 需要一个版本号参数。
func Run(ctx context.Context, db *sql.DB, command string, args ...string) error {
	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("设置 goose 方言失败: %w", err)
	}
	if err := goose.RunContext(ctx, command, db, ".", args...); err != nil {
		return fmt.Errorf("执行 migration 失败: %w", err)
	}
	return nil
}
