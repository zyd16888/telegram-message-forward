package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/gotd/td/bin"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/tg"
	"github.com/gotd/td/tgerr"
)

const (
	floodWaitMaxRetries    = 5
	floodWaitMaxSingleWait = 2 * time.Minute
)

// floodWaitMiddleware 处理 FLOOD_WAIT：等待重试的同时打日志，并设置等待/重试上限。
//
// 官方 gotd/contrib 的 floodwait.NewSimpleWaiter() 默认不限制单次等待时长和重试次数，
// 命中 FLOOD_WAIT 时会静默 sleep 后自动重试、不打印任何日志——这会让"同步很慢"这类问题
// 完全无法从日志排查。这里自实现一版：记录每次等待事件，超过上限直接返回错误，而不是
// 无限期挂起。
type floodWaitMiddleware struct {
	log *slog.Logger
}

func newFloodWaitMiddleware(log *slog.Logger) *floodWaitMiddleware {
	if log == nil {
		log = slog.Default()
	}
	return &floodWaitMiddleware{log: log}
}

// Handle 实现 telegram.Middleware。
func (m *floodWaitMiddleware) Handle(next tg.Invoker) telegram.InvokeFunc {
	return func(ctx context.Context, input bin.Encoder, output bin.Decoder) error {
		var retries int
		for {
			err := next.Invoke(ctx, input, output)
			if err == nil {
				return nil
			}
			wait, ok := tgerr.AsFloodWait(err)
			if !ok {
				return err
			}

			retries++
			m.log.Warn("telegram FLOOD_WAIT，等待后重试", "wait", wait, "retry", retries)

			if retries > floodWaitMaxRetries {
				return fmt.Errorf("触发 Telegram 频率限制（FLOOD_WAIT），已重试 %d 次仍未成功: %w", retries, err)
			}
			if wait > floodWaitMaxSingleWait {
				return fmt.Errorf("触发 Telegram 频率限制（FLOOD_WAIT %s），超过最大等待时间: %w", wait, err)
			}

			timer := time.NewTimer(wait)
			select {
			case <-timer.C:
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			}
		}
	}
}
