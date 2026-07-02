# Telegram Message Forward v2 体验改造开发文档

本文件承接 `docs/product-architecture-v1.md` 和 `docs/roadmap-v1.md`。v1 已经完成消息转发主链路，但真实自用流程仍偏工程化：需要先用 CLI 创建 token、再在 UI 创建 Telegram account、再回命令行按 account ID 登录。v2 的目标是把管理后台登录、Telegram 账号登录和高频配置流程都收进 UI，让开发与自用流程完整闭环。

## 1. v2 目标

v2 优先解决四件事：

1. 管理 UI 提供自己的登录页，不再把“去命令行生成 token 并粘贴到 Settings”作为普通使用主路径。
2. 开发阶段可以关闭管理 API 鉴权，避免首次使用必须先生成 token。
3. Telegram 手机验证码登录全 UI 化，用户不再需要记 account ID 后回命令行登录。
4. 支持 Telegram 官方 QR 登录流程，作为更顺滑的可选登录方式。

本轮不是重做权限系统，也不是引入复杂多用户 RBAC。`cmd/login` 和 `cmd/token` 可以保留为运维兜底入口，但不再作为普通使用主路径。

## 2. 设计原则

- 保持 v1 分层边界：handler 只调 app service，gotd/td 只留在 `internal/infra/telegram` 与 `internal/plugin/source/telegram`。
- API 与 UI 优先服务“单人自用 + 本地/私有部署”，生产安全默认不倒退。
- 开发免鉴权必须是显式配置开关，默认仍启用鉴权。
- Telegram 登录态仍按现有 session 加密落库，不写本地 session 文件。
- 管理 UI 登录需要有明确的登录、退出和当前身份状态，不再依赖 Settings 手工保存 token 完成首屏进入。
- Telegram 登录流程需要跨服务重启可恢复、可观察、错误可读；不要只把 CLI 输入搬到网页。
- 每个阶段必须保持 `go test ./...`、`go vet ./...`、`go build ./...` 通过；改前端必须 `cd web && npm run build`。

## 3. 里程碑顺序

### V2-1 管理 UI 登录与开发免鉴权开关

目标：管理后台有自己的登录页；开发或单人本地使用时，也可以通过配置关闭 `/api/v1` 的 Bearer Token 鉴权。

管理 UI 登录不引入多用户 RBAC。首版按单管理员模型实现，复用现有 API token 哈希校验能力即可：用户在登录页输入管理 token，后端校验有效后返回当前身份状态，前端保存访问凭证并进入后台。后续如果要改成用户名/密码或多用户，可以在此基础上演进。

建议后端 API：

```text
GET  /api/v1/auth/bootstrap
POST /api/v1/auth/bootstrap
POST /api/v1/auth/login
GET  /api/v1/auth/me
POST /api/v1/auth/logout
```

说明：

- `bootstrap` 只在没有任何 active 管理 token 时开放，用于 UI 首次初始化管理员凭证；创建成功后立即关闭该入口。
- `login` 只校验 token 是否有效，不返回 token 明文，不创建新的长期 secret。
- `me` 用于前端启动时判断当前凭证是否仍有效。
- `logout` 前端清理本地凭证；如后端改为 cookie/session 模式，也在这里失效会话。
- `auth_enabled=false` 时，`me` 返回开发免鉴权状态，登录页可以直接进入或显示“开发免鉴权”。

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
- 前端新增 `LoginPage`：未登录访问后台时跳转登录页；登录成功后进入 Dashboard。
- 前端新增首次初始化态：当 `auth/bootstrap` 返回可初始化时，展示“创建首个管理凭证”，创建后自动保存本次返回的明文 token 并进入后台。
- 前端启动时调用 `auth/me` 或运行时接口识别鉴权模式；免鉴权模式下不应强制要求先保存 token。
- Settings 保留 token 管理能力，但定位为“管理凭证维护”，不再承担首屏登录入口。

验收：

- `auth_enabled=true`：无 token 请求 `/api/v1/sinks` 返回 401。
- `auth_enabled=false`：无 token 请求 `/api/v1/sinks` 返回 200。
- 首次无 active token 时：UI 可创建首个管理凭证；创建后再次访问 bootstrap 返回不可初始化。
- `auth_enabled=true`：UI 未登录时显示登录页；输入有效 token 后进入后台；退出后回到登录页。
- `auth_enabled=false`：UI 可直接进入后台，并明确显示开发免鉴权状态。
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
qr_refresh_required
```

实现要求：

- 在 `internal/infra/telegram` 抽出可复用的登录 flow service，不要让 API handler 直接使用 gotd/td。
- app service 负责登录编排、账号状态更新、session 保存。
- flow 必须持久化到数据库，服务重启后 UI 能查询到未完成 flow 并继续展示当前步骤。
- flow 需要有过期时间和定期清理，避免验证码流程永久悬挂。
- code hash、flow 状态、过期时间、最近错误可以落库；2FA 密码、验证码明文不得落库。
- 2FA 密码只在内存中短暂使用，不落库、不进日志。
- 错误响应要可读，例如验证码错误、验证码过期、2FA 必填、Telegram 限流。
- UI `AccountsPage` 新建/编辑账号后提供“登录”按钮，弹窗分步展示：发送验证码 -> 输入验证码 -> 必要时输入 2FA -> 完成。
- UI 刷新页面或后端服务重启后，再打开账号登录弹窗应恢复到可继续/可重试/已过期的明确状态，而不是要求用户无条件从头开始。
- 登录成功后刷新账号列表，状态变为 `active`，并可直接进入 Sources 同步。
- `cmd/login` 保留，但页面不再把它作为主操作说明。

验收：

- 不需要命令行 account ID，UI 可完成手机验证码登录主流程。
- 账号需要 2FA 时 UI 能进入密码步骤。
- 服务重启后，未过期登录 flow 能被 UI 查询并恢复展示；已过期 flow 显示过期并允许重新发起。
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
- 页面刷新或服务重启后，UI 能从 flow 状态恢复二维码登录面板；若原 token 已不可继续，明确提示并一键刷新，不丢失账号与登录上下文。
- 成功后关闭弹窗并刷新账号状态。
- 失败时展示明确错误，不打印 token。

验收：

- 能生成有效二维码。
- 二维码过期可刷新。
- 服务重启后，未完成 QR flow 不会从 UI 消失；可恢复等待状态或进入“需刷新二维码”状态。
- 扫码成功后 session 加密落库，账号变为 `active`。
- DC migrate 分支至少有代码路径和测试覆盖；如无法真实触发，使用 mock/fake 覆盖。
- QR token 不明文落库，不在日志、响应历史、delivery attempts 或前端持久化里保存；如恢复流程必须保存 token，只能使用短 TTL 加密字段。

## 4. 推荐 UI 信息架构调整

### Login

- 作为管理后台首屏鉴权入口，替代“先去 Settings 粘贴 token”的普通路径。
- 首次没有 active 管理 token 时，显示初始化页，创建第一个管理凭证；明文 token 仅显示/保存一次。
- `auth_enabled=true` 时，输入管理 token 并登录；登录失败显示明确错误。
- `auth_enabled=false` 时，显示开发免鉴权状态，可直接进入后台。
- 提供退出登录操作，退出后清理本地凭证并回到登录页。

### Settings

- 显示当前 API 鉴权状态：已启用 / 开发免鉴权。
- `auth_enabled=false` 时，token 管理区域标注为可选。
- `auth_enabled=true` 时，保留 token 列表、生成、吊销能力；不再要求用户在 Settings 手工粘贴 token 才能使用后台。

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

Telegram 登录 flow 必须持久化，避免服务重启、页面刷新或后端短暂崩溃导致用户只能无条件重来。建议新增独立 migration：

```text
telegram_login_flows
- flow_id
- account_id
- method                -- phone_code | qr
- status
- current_step          -- sending_code | code_required | password_required | waiting_scan ...
- phone_code_hash_encrypted
- qr_token_encrypted    -- 可选，仅短 TTL 恢复需要；不能明文落库
- dc_id                 -- QR migrate 或恢复时需要
- expires_at
- last_error
- created_at
- updated_at
- completed_at
```

恢复策略：

- 服务启动时扫描未完成且未过期的 flow，恢复为可查询状态。
- 对 phone_code flow：保留加密后的 `phone_code_hash`，允许用户继续输入验证码或 2FA。
- 对 QR flow：优先恢复等待状态；如果 gotd 监听上下文无法恢复，状态置为 `qr_refresh_required`，UI 一键刷新二维码。
- 过期 flow 标记为 `expired`，UI 提供重新开始登录按钮。
- 完成、取消、过期后的敏感字段应清空或定期清理。

注意：

- `phone_code_hash`、2FA password、QR token 都不能写日志。
- 2FA password 和验证码明文永不落库。
- flow 过期后要清理，敏感字段优先清空。
- 服务重启后登录 flow 丢失不可接受；UI 必须能恢复、继续、刷新或明确展示已过期状态。

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
1. V2-1 管理 UI 登录与开发免鉴权开关：
   - 新增管理后台 LoginPage，未登录时显示登录页，不再要求用户先去 Settings 粘贴 token。
   - 新增 auth bootstrap/login/me/logout API；没有 active token 时，UI 可创建首个管理凭证。
   - 首版可复用现有 API token 哈希校验能力，不引入多用户 RBAC。
   - 增加 security.auth_enabled，默认 true，支持 TMF_SECURITY_AUTH_ENABLED=false。
   - auth_enabled=false 时 /api/v1 跳过 Bearer Token。
   - 前端 Settings 保留 token 管理，但不再承担首屏登录入口，可展示当前鉴权模式。
   - 补测试：true 无 token 401；false 无 token 200。

2. V2-2 Telegram 手机验证码登录 UI 化：
   - 设计 app service 登录 flow，不让 handler 直接接触 gotd/td。
   - 新增 telegram_login_flows migration 和 repository，flow 必须跨服务重启可查询、可继续、可过期清理。
   - UI Accounts 页提供登录弹窗，完成发送验证码、输入验证码、2FA 密码、成功状态刷新。
   - 页面刷新或服务重启后，UI 能恢复未完成登录 flow；已过期时明确提示并允许重新开始。
   - cmd/login 保留，但 UI 不再要求用户复制 account ID 去命令行。
   - 补 service 或 handler 层测试；无法真实 Telegram 联调时在最终说明里明确。

3. V2-3 Telegram QR 登录：
   - 基于官方 auth.exportLoginToken / gotd qrlogin 评估并实现。
   - UI 提供二维码、倒计时、刷新、扫码成功状态。
   - QR flow 必须持久化状态；服务重启后能恢复等待状态或进入需刷新二维码状态。
   - QR token 不明文落库不打印；如恢复必须保存，只能使用短 TTL 加密字段。
   - DC migrate 分支至少有 mock 测试覆盖。

约束：
- 每个阶段保持 go test ./...、go vet ./...、go build ./... 通过。
- 改 web 后必须 cd web && npm run build。
- schema 变更必须新增 migrations/*.sql，不改已执行 migration。
- 按功能边界提交中文 commit，每个 commit 保持可 build。
- 不提交 configs/config.yaml、web/dist、node_modules、.tsbuildinfo 或任何真实密钥。
```
