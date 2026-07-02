package telegram

import (
	"context"
	"encoding/base64"
	"errors"
	"time"

	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth/qrlogin"
	"github.com/gotd/td/tgerr"
)

// QRExportResult 是导出 QR 登录 token 的结果。Token 为敏感字段。
type QRExportResult struct {
	Token     []byte
	URL       string
	ExpiresAt time.Time
	DCID      int
}

// QRCheckResult 是一次 QR 轮询结果。
//
// 语义互斥：Authorized / Waiting / Expired / MigrateDC 只会命中其一。
type QRCheckResult struct {
	// Authorized 表示扫码已确认，登录成功。
	Authorized bool
	// Waiting 表示仍在等待扫码；RefreshedToken/ExpiresAt 为最新 token。
	Waiting        bool
	RefreshedToken []byte
	ExpiresAt      time.Time
	// Expired 表示 token 已失效，需要刷新二维码。
	Expired bool
	// MigrateDC 大于 0 表示服务端要求迁移到该 DC 后再导入 token。
	MigrateDC int
}

// QRTokenURL 由 token 字节构造 tg://login URL，格式与 gotd qrlogin.Token.URL 一致。
func QRTokenURL(token []byte) string {
	return "tg://login?token=" + base64.URLEncoding.EncodeToString(token)
}

// QRExport 导出一个新的 QR 登录 token。
func (LoginFlowService) QRExport(ctx context.Context, cfg LoginFlowConfig) (QRExportResult, error) {
	var out QRExportResult
	err := runLoginStep(ctx, cfg, func(ctx context.Context, client *telegram.Client) error {
		q := qrlogin.NewQR(client.API(), cfg.AppID, cfg.AppHash, qrlogin.Options{})
		t, err := q.Export(ctx)
		if err != nil {
			return err
		}
		raw, err := base64.URLEncoding.DecodeString(t.String())
		if err != nil {
			return err
		}
		out.Token = raw
		out.URL = t.URL()
		out.ExpiresAt = t.Expires()
		return nil
	})
	return out, classifyLoginErr(err)
}

// QRCheck 轮询 QR 登录状态。
//
// 采用 exportLoginToken 轮询：非空 token 表示仍在等待（并顺带刷新 token）；
// 空 token 表示已被接受，随后 Import 完成登录；MigrationNeededError 表示需要 DC 迁移。
func (LoginFlowService) QRCheck(ctx context.Context, cfg LoginFlowConfig, _ []byte, _ int64) (QRCheckResult, error) {
	var out QRCheckResult
	err := runLoginStep(ctx, cfg, func(ctx context.Context, client *telegram.Client) error {
		q := qrlogin.NewQR(client.API(), cfg.AppID, cfg.AppHash, qrlogin.Options{})
		t, err := q.Export(ctx)
		if err != nil {
			var mig *qrlogin.MigrationNeededError
			if errors.As(err, &mig) {
				out.MigrateDC = mig.MigrateTo.DCID
				return nil
			}
			if tgerr.Is(err, "AUTH_TOKEN_EXPIRED", "AUTH_TOKEN_INVALID") {
				out.Expired = true
				return nil
			}
			return err
		}
		if t.Empty() {
			// 已接受：导入以完成登录。
			if _, ierr := q.Import(ctx); ierr != nil {
				var mig *qrlogin.MigrationNeededError
				if errors.As(ierr, &mig) {
					out.MigrateDC = mig.MigrateTo.DCID
					return nil
				}
				return ierr
			}
			out.Authorized = true
			return nil
		}
		// 仍在等待：刷新 token。
		raw, derr := base64.URLEncoding.DecodeString(t.String())
		if derr != nil {
			return derr
		}
		out.Waiting = true
		out.RefreshedToken = raw
		out.ExpiresAt = t.Expires()
		return nil
	})
	return out, classifyLoginErr(err)
}
