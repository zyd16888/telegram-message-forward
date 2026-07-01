# Telegram Message Forward v2 体验改造开发文档

本文件承接 `docs/product-architecture-v1.md` 和 `docs/roadmap-v1.md`。v1 已经完成消息转发主链路，但真实自用流程仍偏工程化：需要先用 CLI 创建 token、再在 UI 创建 Telegram account、再回命令行按 account ID 登录。v2 的目标是把这些高频操作收进 UI，让开发与自用流程更顺。

## 1. v2 目标

v2 优先解决三件事：

1. 开发阶段可以关闭管理 API 鉴权，避免首次使用必须先生成 token。
2. Telegram 手机验证码登录全 UI 化，用户不再需要记 account ID 后回命令行登录。
3. 支持 Telegram 官方 QR 登录流程，作为更顺滑的可选登录方式。

本轮不是重做权限系统，也不是引入复杂多用户 RBAC。`cmd/login` 和 `cmd/token` 可以保留为运维兜底入口，但不再作为普通使用主路径。

## 2. 设计原则

- 保持 v1 分层边界：handler 只调 app service，gotd/td 只留在 `internal/infra/telegram` 与 `internal/plugin/source/telegram`。
- API 与 UI 优先服务“单人自用 + 本地/私有部署”，生产安全默认不倒退。
- 开发免鉴权必须是显式配置开关，默认仍启用鉴权。
- Telegram 登录态仍按现有 session 加密落库，不写本地 session 文件。
- 登录流程需要可恢复、可观察、错误可读；不要只把 CLI 输入搬到网页。
- 每个阶段必须保持 `go test ./...`、`go vet ./...`、`go build ./...` 通过；改前端必须 `cd web && npm run build`。

## 3. 里程碑顺序

### V2-1 开发免鉴权开关

目标：开发或单人本地使用时，可以通过配置关闭 `/api/v1` 的 Bearer Token 鉴权。

建议配置：

```yaml
security:
  auth_enabled: true
```

环境变量覆盖：

```text
TMF_SECURITY_AUTH_ENABLED=false
```

实现要求：

- 在 `internal/config.Config.Security` 增加 `AuthEnabled bool`，默认 `true`。
- `api.NewRouter` 或 router deps 增加鉴权开关；`auth_enabled=false` 时不挂 `middleware.Auth`。
- `/healthz` 仍保持无鉴权。
- `auth_enabled=true` 时现有 token 行为不变，原有 `TestAPIAuthAndCRUD` 不应破坏。
- 前端在免鉴权模式下不应强制要求先保存 token。可以后端新增 `/api/v1/settings/runtime` 或 `/api/runtime` 返回运行时开关，前端据此隐藏或弱化 token 提示。

验收：

- `auth_enabled=true`：无 token 请求 `/api/v1/sinks` 返回 401。
- `auth_enabled=false`：无 token 请求 `/api/v1/sinks` 返回 200。
- `cmd/token` 仍可用。
- 不提交真实 `configs/config.yaml`。

### V2-2 Telegram 手机验证码登录 UI 化

目标：用户在 UI 中完成 Telegram 登录，不再手工复制 account ID 到 `cmd/login`。

建议后端 API：

```text
POST /api/v1/accounts/login/start
POST /api/v1/accounts/login/verify-code
POST /api/v1/accounts/login/verify-password
GET  /api/v1/accounts/login/:flow_id
POST /api/v1/accounts/login/:flow_id/cancel
```

也可以把路径设计为 account 子资源：

```text
POST /api/v1/accounts/:id/login/start
POST /api/v1/accounts/:id/login/code
POST /api/v1/accounts/:id/login/password
GET  /api/v1/accounts/:id/login/status
```

二者择一，优先选择更容易实现和测试的方案。若账号尚未创建，UI 可以先创建 account，再自动进入登录流；用户界面不展示“复制 account ID 去命令行”。

建议状态：

```text
idle
sending_code
code_required
password_required
authorized
failed
cancelled
expired
```

实现要求：

- 在 `internal/infra/telegram` 抽出可复用的登录 flow service，不要让 API handler 直接使用 gotd/td。
- app service 负责登录编排、账号状态更新、session 保存。
- flow 需要有过期时间和内存清理，避免验证码流程永久悬挂。
- 2FA 密码只在内存中短暂使用，不落库、不进日志。
- 错误响应要可读，例如验证码错误、验证码过期、2FA 必填、Telegram 限流。
- UI `AccountsPage` 新建/编辑账号后提供“登录”按钮，弹窗分步展示：发送验证码 -> 输入验证码 -> 必要时输入 2FA -> 完成。
- 登录成功后刷新账号列表，状态变为 `active`，并可直接进入 Sources 同步。
- `cmd/login` 保留，但页面不再把它作为主操作说明。

验收：

- 不需要命令行 account ID，UI 可完成手机验证码登录主流程。
- 账号需要 2FA 时 UI 能进入密码步骤。
- 登录失败不会把 session 写成有效状态。
- session 仍加密落库。
- `go test ./...` 覆盖登录 flow 的状态机或 service 层关键分支；真实 Telegram 联调可在最终回复里标注是否已跑。

### V2-3 Telegram QR 登录

目标：支持官方 Telegram QR login，用户用已登录的 Telegram App 扫码授权。

官方流程参考：

- `auth.exportLoginToken` 生成登录 token。
- token 需要 base64url 编码并放入 `tg://login?token=<token>`。
- 二维码通常约 30 秒过期，过期后需要重新生成。
- 用户在已登录的 Telegram App 中扫码并确认。
- 服务端收到 `updateLoginToken` 后再次确认；若返回 `auth.loginTokenMigrateTo`，需要按 DC 迁移后导入 token；成功时返回 `auth.loginTokenSuccess`。

官方文档：

- https://core.telegram.org/api/qr-login
- https://core.telegram.org/method/auth.exportLoginToken

gotd 可优先评估：

- `github.com/gotd/td/telegram/auth/qrlogin`

建议后端 API：

```text
POST /api/v1/accounts/:id/login/qr/start
GET  /api/v1/accounts/:id/login/qr/status?flow_id=...
POST /api/v1/accounts/:id/login/qr/refresh
POST /api/v1/accounts/:id/login/qr/cancel
```

响应建议：

```json
{
  "flow_id": "...",
  "status": "waiting_scan",
  "qr_url": "tg://login?token=...",
  "expires_at": "2026-07-01T12:00:00Z"
}
```

UI 要求：

- 在账号登录弹窗中提供两个 tab：`验证码登录` / `扫码登录`。
- QR 登录 tab 展示二维码、倒计时、刷新按钮和状态提示。
- 过期后自动刷新或提示用户刷新。
- 成功后关闭弹窗并刷新账号状态。
- 失败时展示明确错误，不打印 token。

验收：

- 能生成有效二维码。
- 二维码过期可刷新。
- 扫码成功后 session 加密落库，账号变为 `active`。
- DC migrate 分支至少有代码路径和测试覆盖；如无法真实触发，使用 mock/fake 覆盖。
- 不在日志、响应历史、delivery attempts 或前端持久化里保存 QR token。

## 4. 推荐 UI 信息架构调整

### Settings

- 显示当前 API 鉴权状态：已启用 / 开发免鉴权。
- `auth_enabled=false` 时，token 输入不是必填；可以保留 token 管理区域，但标注为可选。
- `auth_enabled=true` 时，保持现有 Bearer Token 流程。

### Accounts

账号页应从“配置列表”升级为“账号工作台”：

- 新建账号：填写名称、手机号、app_id、app_hash、代理。
- 登录状态：inactive / logging_in / active / error / banned。
- 操作：登录、重新登录、同步来源、停止监听、删除。
- 登录弹窗：验证码登录与扫码登录两个入口。
- 登录成功后提供“去同步来源”的直接操作。

### Sources

- 同步来源时不要要求用户理解 account ID；使用账号名称下拉。
- 当账号未登录时，直接引导去 Accounts 登录。

## 5. 数据与状态建议

v2 初期登录 flow 可以只放内存，不必急着入库：

```text
login_flows
- flow_id
- account_id
- method                -- phone_code | qr
- status
- phone_code_hash       -- phone code 登录需要
- qr_token              -- 仅内存保存，不落库
- expires_at
- last_error
```

如果后续需要跨进程恢复，再单独设计数据库表。当前单体进程内存 flow 更符合 KISS。

注意：

- `phone_code_hash`、2FA password、QR token 都不能写日志。
- flow 过期后要清理。
- 服务重启后登录 flow 丢失可以接受，UI 提示重新开始登录。

## 6. 禁止项

- 不要把 gotd/td 类型扩散到 `internal/api`、`internal/app`、`internal/domain` 或任何非适配层。
- 不要为了 UI 登录把 session 明文写到文件。
- 不要默认关闭生产鉴权；`auth_enabled` 默认必须是 `true`。
- 不要把 `cmd/login` 和 `cmd/token` 删除，它们作为运维兜底仍有价值。
- 不要把 QR token、验证码、2FA 密码、session、app_hash 打到日志。
- 不要把真实 `configs/config.yaml`、`web/dist`、`node_modules`、`.tsbuildinfo` 提交。

## 7. Claude Code 执行提示词

下面提示词可直接交给 Claude Code 或其它 agent 继续推进：

```text
你在 D:\project\go_project\telegram-message-forward 工作。

先阅读：
- AGENTS.md
- docs/product-architecture-v1.md
- docs/roadmap-v1.md
- docs/roadmap-v2.md

目标：按 docs/roadmap-v2.md 顺序推进 v2 体验改造，不要跳阶段。

任务顺序：
1. V2-1 开发免鉴权开关：
   - 增加 security.auth_enabled，默认 true，支持 TMF_SECURITY_AUTH_ENABLED=false。
   - auth_enabled=false 时 /api/v1 跳过 Bearer Token。
   - 前端 Settings 不再强制要求 token，可展示当前鉴权模式。
   - 补测试：true 无 token 401；false 无 token 200。

2. V2-2 Telegram 手机验证码登录 UI 化：
   - 设计 app service 登录 flow，不让 handler 直接接触 gotd/td。
   - UI Accounts 页提供登录弹窗，完成发送验证码、输入验证码、2FA 密码、成功状态刷新。
   - cmd/login 保留，但 UI 不再要求用户复制 account ID 去命令行。
   - 补 service 或 handler 层测试；无法真实 Telegram 联调时在最终说明里明确。

3. V2-3 Telegram QR 登录：
   - 基于官方 auth.exportLoginToken / gotd qrlogin 评估并实现。
   - UI 提供二维码、倒计时、刷新、扫码成功状态。
   - QR token 只短暂保存在内存，不落库不打印。
   - DC migrate 分支至少有 mock 测试覆盖。

约束：
- 每个阶段保持 go test ./...、go vet ./...、go build ./... 通过。
- 改 web 后必须 cd web && npm run build。
- schema 变更必须新增 migrations/*.sql，不改已执行 migration。
- 按功能边界提交中文 commit，每个 commit 保持可 build。
- 不提交 configs/config.yaml、web/dist、node_modules、.tsbuildinfo 或任何真实密钥。
```
