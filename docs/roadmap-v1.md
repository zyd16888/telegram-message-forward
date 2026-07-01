# Telegram Message Forward v1 路线图与进度跟踪

本文件是 v1 的**执行跟踪清单**，与 `docs/product-architecture-v1.md`（设计定稿）配套。
设计以架构文档为准，本文件负责"做到哪了、下一步做什么、做的时候要注意什么"。

约定：
- `[x]` 已完成并验证，`[~]` 进行中，`[ ]` 未开始。
- 每个任务尽量给出**验收标准**和**关键约束/坑**，方便任何一次会话独立接手。
- 完成一项就更新勾选，必要时补一行说明。

---

## 0. 当前状态快照

> 更新日期：2026-07-01

**已完成**

- [x] 新 Go 1.26 模块初始化，后端骨架搭建，`go build ./... / go vet ./... / go test ./...` 全绿。
- [x] `migrations/00001_init.sql`：§10 全部 15 张表（含 access_hash、幂等唯一约束、reaper 字段、索引）。
- [x] goose 迁移执行封装 `internal/storage/migrate`，`cmd/migrate` 与启动自动迁移共用。
- [x] `database.auto_migrate` 开关接入 `bootstrap.Build`（true=启动时 goose up，默认 false）。
- [x] 线上库接通：Supabase，**session 模式 5432**，schema = `message_forward`，迁移已应用。

**已知技术债 / 待办前置**

- [ ] `security.encryption_key` 仍是占位符 `CHANGE_ME_...`（正好 32 字节能跑）。**存真实 session/secret 前必须换成 `openssl rand -hex 16`，且一旦启用不可再改**。
- [ ] worker/queue 代码已存在（`internal/dispatch`）但**尚未在 `bootstrap.Run` 启动**，见 M1-5。
- [ ] 仅 `account_repository` 已实现，其余仓储待补，见 M1-1。

**骨架现状（已存在 vs 待实现）**

| 模块 | 状态 |
|---|---|
| config / infra(logger,crypto,clock) | ✅ 已实现 |
| storage: db / transaction / model(全表) / migrate | ✅ 已实现 |
| storage/repository | ⏳ 仅 account |
| domain/*（类型 + Repository 接口） | ✅ 已实现 |
| plugin/source, plugin/sink（接口 + 注册表） | ✅ 骨架，无内置实现 |
| ruleengine（engine + condition/processor 注册表） | ✅ 骨架，无内置条件/处理器 |
| template/renderer | ✅ 已实现 |
| dispatch（queue/worker/retry） | ✅ 代码存在，未接线启动 |
| security/token | ✅ 已实现 |
| api（router + middleware + health handler） | ✅ 骨架，仅 /healthz |
| bootstrap / cmd(server, migrate) | ✅ 已实现 |
| telegram / sinks 实现 / app services / handlers / web | ❌ 未开始 |

---

## 里程碑总览

- **M1 投递主链路打通（不含 Telegram）** — 先做，最快拿到端到端可演示
- **M2 Telegram Source** — Phase 0/1 核心，工作量最大
- **M3 WeCom Sink + 可靠性**
- **M4 管理 API + 鉴权**
- **M5 Web UI**（后置）

---

## M1 — 投递主链路打通（不含 Telegram）⭐

目标：在线上库上跑通 `消息 → 规则匹配 → 渲染 → 投递 → 记录`，用 Webhook Sink 验证整条脊柱。

- [ ] **M1-1 补齐仓储**（`internal/storage/repository`）：source / sink / rule / template / message / delivery。
  - 参考 `account_repository.go` 的 domain↔model 映射 + 敏感字段加解密模式。
  - **约束**：repository 对外只返回 domain 对象，不返回 GORM model；SQL Raw 只允许出现在 repository 内部。
  - `rule` 仓储要处理 `rule_sources` / `rule_targets` 两张关联表的读写，组装成 `rule.Rule{SourceIDs, Targets}`。
  - `message.Create` 命中 `(source_id, external_message_id)` 唯一冲突时按**幂等成功**处理（gorm `TranslateError` 已开，判 `gorm.ErrDuplicatedKey`）。
  - `delivery.Create` 按 `(message_id, rule_id, sink_id)` 幂等。
  - `delivery.Claim`：`SELECT ... FOR UPDATE SKIP LOCKED`，只领 `status='pending'` 或（`status='retrying'` 且 `next_retry_at<=now`），同事务置 `processing` / `locked_at` / `locked_by`。
  - `delivery.RecoverStale`：`processing` 且 `locked_at < now-visibility_timeout` 的回退 `retrying`。
  - 验收：对每个仓储写最小单测或集成测试（可用线上库或本地 pg），CRUD + 幂等 + Claim 正确。
- [ ] **M1-2 内置 Condition**（`internal/ruleengine/condition`）：`keyword_contains` / `keyword_excludes` / `regex` / `message_type`，各自 `init()` 注册。
  - 验收：单测覆盖命中/不命中。
- [ ] **M1-3 内置 Processor**（`internal/ruleengine/processor`）：`append_source` / `truncate_text`，`init()` 注册。
- [ ] **M1-4 Webhook Sink**（`internal/plugin/sink/webhook`）：实现 `sink.Plugin` 接口 + `init()` 注册 `sink.Register("webhook", ...)`。
  - `Send` 用 `httpclient` POST JSON，返回脱敏后的 `Result`。
  - **约束**：Sink 不查库、不判规则、不做重试调度。
  - 验收：对本地/httpbin 端点发一条，返回 success。
- [ ] **M1-5 接线 + 启动 worker**：
  - `bootstrap.Build` 装配所有仓储、renderer、queue、worker；`bootstrap.Run` 按 `dispatch.worker_count` 起 worker goroutine。
  - 建一个 ingest 入口（先手动/测试触发）：`NormalizedMessage → message repo → ruleengine.Evaluate → queue.Enqueue`。
  - 验收：**M1 总验收** — 插入 sink+rule+template，投喂一条测试消息，Webhook 收到 + `delivery_tasks.status=success` + `delivery_attempts` 有记录；故意让 Webhook 失败能看到 retrying→backoff→dead。
- [ ] **M1-6 首个 API token 生成命令**（可选提前）：写 `api_tokens`（存 `security.HashToken` 后的 hash），供 M4 鉴权用。

**M1 需要但尚缺的基础件**：`internal/infra/httpclient`（带超时的 http.Client 封装）。

---

## M2 — Telegram Source（Phase 0/1 核心）

目标：真实 TG 频道消息 → 走完 M1 链路 → Webhook 收到。

- [ ] **M2-1 `internal/infra/telegram`**：gotd/td client 封装。
  - 加密 session store：对接 `accounts.session_encrypted`，用 `infra/crypto` 加解密（实现 gotd 的 session 存储接口）。
  - 代理（socks5/http）支持，读 `account.ProxyConfig`。
  - **必须**内置 FLOOD_WAIT + rate limit middleware。
  - **约束**：gotd/td 类型只能出现在 `infra/telegram` 和 `plugin/source/telegram`，不得扩散到 domain/ruleengine/dispatch/sink。
- [ ] **M2-2 CLI 登录 `cmd/login`**：SendCode / SignIn / 2FA，落 session 到 accounts。
- [ ] **M2-3 `telegram_peers` 仓储 + peer 解析**：缓存 access_hash；提供按 peer 解析 InputPeer 的能力（后续 getMessages/媒体/reply 依赖）。
- [ ] **M2-4 `internal/plugin/source/telegram`**：实现 `source.Plugin`。
  - `SyncSources`：拉账号可见 chats/channels。
  - `Start`：监听 updates → 原始消息 normalize 成 `NormalizedMessage` → 回调 handler。
  - **peer 解析要覆盖 user/chat/channel 三类**，不能像旧 POC 那样直接断言 `*tg.PeerChannel`。
- [ ] **M2-5 接入 ingest**：source handler → message repo（幂等）→ ruleengine → queue。
  - 验收：**M2 总验收** — 配置真实频道为 source + 一条规则 + webhook sink，频道发消息，Webhook 收到。

---

## M3 — WeCom Sink + 可靠性

- [ ] **M3-1 `wecom_bot` Sink**（群机器人 webhook，无 access_token）。
- [ ] **M3-2 `wecom_app` Sink**（应用消息，corpid/secret/agentid + access_token 缓存/刷新）。
- [ ] **M3-3 Sink RateLimiter**（`internal/ruleengine` 或 dispatch 侧，WeCom 群机器人 20 条/分限频）。
- [ ] **M3-4 dead 任务手动重试入口**（先做 service 层方法，API 在 M4 暴露）。
- 验收：TG 消息成功投递到企业微信；触发限频不报错、能排队。

---

## M4 — 管理 API + 鉴权

- [ ] **M4-1 `internal/app/*` 应用服务**：account / source / sink / rule / delivery，编排仓储 + 事务边界。
- [ ] **M4-2 `internal/api/dto` + `handler`**：各资源 CRUD；敏感字段在 DTO 层脱敏。
  - **约束**：handler 只调 app service，不碰 GORM；DTO 与 domain 分离。
  - 挂到 `router.go` 里 `/api/v1` 分组（已带 `Auth` 中间件）。
- [ ] **M4-3 token 管理**：生成/吊销 api_token；Auth 中间件已就绪（`security.TokenValidator`）。
- 验收：带 token 能完成账号/来源/渠道/规则/投递记录的增删改查，脱敏正确，无 token 返回 401。

---

## M5 — Web UI（后置）

- [ ] Vue 3 + TS + Vite 工程骨架（`web/`）。
- [ ] 页面：dashboard / accounts / sources / sinks / rules / templates / deliveries / settings。
- [ ] `npm run build` 通过。

---

## 关键约束速查（改代码前必读）

- **依赖方向**：domain 不依赖 api/storage/plugin/infra；api 不碰 GORM；plugin/sink 不查库；plugin/source 不跑规则；ruleengine 不调外部 Sink；dispatch 不解析 TG 原始消息。
- **gotd/td** 只在 `infra/telegram` + `plugin/source/telegram`。
- **GORM** 只在 `internal/storage`；repository 返回 domain，不返回 model。
- **schema 变更**只能改 `migrations/` 下的 goose SQL，不用 AutoMigrate 作正式迁移。
- **敏感字段**（session/secret/proxy/app_hash）经 `infra/crypto` 加密落库；日志/响应/备份不出现明文。
- **投递语义** at-least-once：ingest 幂等（messages 唯一约束）+ 任务幂等（delivery_tasks 唯一约束）。可能乱序为已知限制。
- **插件注册**用编译期 `init()` + `Register()`；新增 Sink/Source/Condition/Processor 都走注册表。
- 每次改 Go 代码后必须 `go build ./...`（能测则 `go test ./...`）；改前端后 `npm run build`。

---

## 环境与运维备忘

**数据库（Supabase）**

- 连接走 **session 模式，端口 5432**（长驻服务用；不要用 6543 事务池，会触发 pgx 预处理语句冲突 `42P05`）。
- schema = `message_forward`，通过服务端角色默认 search_path 生效（不依赖连接串 `options`）：
  ```sql
  create schema if not exists message_forward;
  alter role postgres set search_path to message_forward, public;
  ```
- 本地真实配置在 `configs/config.yaml`（已被 `.gitignore` 忽略，含明文凭据，勿 `git add -f`）。
- 若 5432 仍偶发 `42P05`，DSN 末尾加 `?default_query_exec_mode=simple_protocol`。

**常用命令**

```bash
# 迁移
go run ./cmd/migrate -config ./configs/config.yaml up
go run ./cmd/migrate -config ./configs/config.yaml status
go run ./cmd/migrate -config ./configs/config.yaml down

# 起服务
go run ./cmd/server -config ./configs/config.yaml
# 健康检查
curl http://localhost:8080/healthz

# 生成加密主密钥（32 字节）
openssl rand -hex 16
```

**配置覆盖**：环境变量前缀 `TMF_`，嵌套用下划线，如 `TMF_DATABASE_DSN`、`TMF_SECURITY_ENCRYPTION_KEY`（优先级高于配置文件）。
