# Telegram Message Forward v1 路线图与进度跟踪

本文件是 v1 的**执行跟踪清单**，与 `docs/product-architecture-v1.md`（设计定稿）配套。
设计以架构文档为准，本文件负责"做到哪了、下一步做什么、做的时候要注意什么"。

约定：
- `[x]` 已完成并验证，`[~]` 进行中，`[ ]` 未开始。
- 每个任务尽量给出**验收标准**和**关键约束/坑**，方便任何一次会话独立接手。
- 完成一项就更新勾选，必要时补一行说明。

---

## 0. 当前状态快照

> 更新日期：2026-07-01（M1–M5 主体落地）

**已完成（本轮）**

- **M1–M5 全部里程碑的可运行/可测试部分均已落地**，`gofmt` / `go build ./...` / `go vet ./...` / `go test ./...` 全绿，`web` 侧 `npm run build` 通过。
- 端到端主链路（消息→规则→渲染→Webhook 投递→状态流转）已有 Supabase 库集成测试：`TestPipelineSuccess` / `TestPipelineDead`。默认 `go test ./...` 会跳过，需设置 `TMF_RUN_DB_TESTS=1` 才会真实跑库。
- 管理 API 鉴权与脱敏已有集成测试：`TestAPIAuthAndCRUD`（无/错 token→401，有 token→CRUD，secret 不回显、敏感 config 脱敏）。默认同样跳过真实 DB。
- 依赖新增：`gotd/td v0.154`、`gotd/contrib`、`golang.org/x/net/proxy`、`golang.org/x/time/rate`（架构已定稿 gotd/td）。

**待真实外部账号联调（代码已就绪）**

- **M2 Telegram**：需真实 Telegram 账号 + `app_id/app_hash` 跑 `cmd/login` 登录、`SyncSources` 拉列表、配置真实频道 source 后端到端收消息。
- **M3 WeCom**：需真实企业微信 `corpid/secret/agentid` 或群机器人 key 做端到端投递联调（请求构造/错误处理/token 刷新已单测覆盖）。

**已知技术债 / 注意**

- [ ] `security.encryption_key` 仍是占位符。**存真实 session/secret 前必须换成 `openssl rand -hex 16`，且一旦启用不可再改**。
- [ ] Telegram Source 目前「每 source 一个客户端」，多 source 共账号的连接复用（SourceManager 去重）后置。
- [ ] Telegram 历史补拉后置排期：基于 `sources.last_message_id` 设计手动回捞、启动补漏和断线恢复后的增量追平，不混入媒体转发主线。
- [ ] http 代理未实现（仅 socks5）；媒体仅记录轻量描述，不下载文件。
- [ ] 前端 naive-ui 单 chunk 体积告警（未做手动分包）。

**模块现状**

| 模块 | 状态 |
|---|---|
| config / infra(logger,crypto,clock,httpclient,telegram) | ✅ 已实现 |
| storage: db / transaction / model / migrate | ✅ 已实现 |
| storage/repository（account/source/sink/template/rule/message/delivery/telegram_peer/api_token） | ✅ 全部实现 |
| domain/*（含 peer / apitoken） | ✅ 已实现 |
| plugin/sink（webhook / wecom_bot / wecom_app） | ✅ 已实现 + 单测 |
| plugin/source/telegram（gotd/td 适配 + 归一化） | ✅ 已实现（外部联调待办）|
| ruleengine（condition/processor 内置） | ✅ 已实现 + 单测 |
| template/renderer | ✅ 已实现 |
| dispatch（queue/worker/retry）+ app/ingest 接线启动 | ✅ 已接线启动 |
| app services（account/source/sink/template/rule/delivery/apitoken/ingest） | ✅ 已实现 |
| api（router + dto + 全资源 handler + auth） | ✅ 已实现 + 集成测试 |
| cmd(server, migrate, login, token) | ✅ 已实现 |
| web（Vue3+TS+Vite+Naive UI，8 页面） | ✅ 已实现，build 通过 |

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

- [x] **M1-1 补齐仓储**（`internal/storage/repository`）：source / sink / rule / template / message / delivery / api_token 全部实现。
  - domain↔model 映射；sink secret 经 `infra/crypto` 加密落库；repository 只返回 domain。
  - `rule` 仓储在事务内读写 `rule_sources` / `rule_targets`，组装 `rule.Rule{SourceIDs, Targets}`。
  - `message.Create` / `delivery.Create` 命中唯一冲突时判 `gorm.ErrDuplicatedKey` 幂等回填已存在 id。
  - `delivery.Claim`：`UPDATE ... WHERE id IN (SELECT ... FOR UPDATE SKIP LOCKED) RETURNING *`，领 `pending` 或到期 `retrying`，置 `processing`/`locked_at`/`locked_by`。
  - `delivery.RecoverStale`：`processing` 且 `locked_at < olderThan` 回退 `retrying`。
  - 验收：集成测试（`internal/app/ingest/pipeline_integration_test.go`，Supabase 库）覆盖 CRUD + 幂等 + Claim + 状态流转；默认跳过，需 `TMF_RUN_DB_TESTS=1`。
- [x] **M1-2 内置 Condition**（`internal/ruleengine/condition/builtin.go`）：`keyword_contains` / `keyword_excludes` / `regex` / `message_type`，`init()` 注册。单测 `builtin_test.go` 覆盖命中/不命中，PASS。
- [x] **M1-3 内置 Processor**（`internal/ruleengine/processor/builtin.go`）：`append_source` / `truncate_text`，`init()` 注册。单测 PASS。
- [x] **M1-4 Webhook Sink**（`internal/plugin/sink/webhook`）：实现 `sink.Plugin` + `init()` 注册；`Send` 用 `httpclient` POST JSON，返回脱敏 `Result`（截断 body、状态码）。secret 作为可选 Bearer。集成测试验证 200→success、500→失败。
- [x] **M1-5 接线 + 启动 worker**：`bootstrap.Build` 装配全部仓储/engine/renderer/queue/worker + `app/ingest` 服务；`bootstrap.Run` 按 `worker_count` 起 worker goroutine（ctx 取消优雅退出）。ingest 入口 `NormalizedMessage → message repo（幂等）→ ruleengine.Evaluate → queue.Enqueue`。
  - 验收：**M1 总验收已补集成测试** — `TestPipelineSuccess`（Webhook 收到渲染文本 + `delivery_tasks.status=success` + `delivery_attempts` 有记录）、`TestPipelineDead`（Webhook 500 + `max_attempts=1` → `dead` + `last_error`）。真实 DB 运行需 `TMF_RUN_DB_TESTS=1`，重试/backoff/retrying 状态由 `applyRetry` 覆盖。
- [x] **M1-6 API token CLI**（`cmd/token`）：`create`/`list`/`revoke`，写 `api_tokens`（存 `security.HashToken` 哈希），明文仅打印一次。已冒烟通过。

**M1 基础件**：`internal/infra/httpclient`（带超时、body 限长的 http.Client 封装）✅。

---

## M2 — Telegram Source（Phase 0/1 核心）

目标：真实 TG 频道消息 → 走完 M1 链路 → Webhook 收到。

- [x] **M2-1 `internal/infra/telegram`**：gotd/td v0.154 client 封装（gotd/contrib floodwait+ratelimit middleware）。
  - `SessionStore`（加密 load/store 回调）+ `accountSession`（plugin 层，明文内存 + save 回调，加密由 account 仓储完成）。session 加密落库，明文不进磁盘。单测 `session_test.go` 验证密文落库 + 解密回读。
  - 代理：socks5（含账号密码）经 `dcs.Plain(PlainOptions{Dial})` 注入；http 代理标注未实现（返回明确错误）。
  - FLOOD_WAIT（`floodwait.NewSimpleWaiter`）+ rate limit（`ratelimit.New`，默认 ~10 req/s）已内置。
  - gotd/td 类型仅出现在 `infra/telegram` + `plugin/source/telegram`，未扩散。
- [x] **M2-2 CLI 登录 `cmd/login`**：`RunLogin` 封装 SendCode/SignIn/2FA（`auth.Flow` 统一编排，Password 仅 2FA 触发），成功后 session 加密落库、账号置 active。cmd 不直接依赖 gotd。**编译通过；真实登录需外部账号。**
- [x] **M2-3 `telegram_peers` 仓储 + peer 缓存**：`TelegramPeerRepository`（`ON CONFLICT` upsert access_hash）+ `domain/peer`。SyncSources 落缓存，覆盖 user/chat/channel。
- [x] **M2-4 `internal/plugin/source/telegram`**：实现 `source.Plugin`。
  - `SyncSources`：`MessagesGetDialogs` 拉 chats/channels/users，缓存 access_hash，返回 SyncedPeer。
  - `Start`：`tg.UpdateDispatcher` 监听 `OnNewMessage` / `OnNewChannelMessage`，按 source peer 过滤 → `Normalize` → 回调 handler。`Stop` 取消 context。
  - **peer 解析覆盖 user/chat/channel 三类**（`matchesSource` + `Normalize`），单测 `normalize_test.go` 验证 channel-photo / user-text 两路。
- [x] **M2-5 接入 ingest**：`app/source.Manager.StartAll` 对「已启用且账号 active」的 source 调 `plugin.Start(handler=ingest.Ingest)`；bootstrap 注册 telegram source + 注入 LoadSession/SaveSession + peer 仓储；`Run` 启动、优雅关闭时 `StopAll`。服务启动冒烟通过（0 启用 source 时不连接、不报错）。
  - 验收：**M2 总验收（可运行部分 PASS，外部联调待办）** — 编译/单测/服务启动全通过。**剩余：需真实 Telegram 账号 + app_id/app_hash 跑 `cmd/login` 登录、SyncSources 拉列表、配置真实频道 source 后端到端收消息。**

---

## M3 — WeCom Sink + 可靠性

- [x] **M3-1 `wecom_bot` Sink**（群机器人 webhook，无 access_token）：secret=webhook key 或 config.webhook_url；text/markdown；`errcode` 判成功。单测覆盖成功/错误码。
- [x] **M3-2 `wecom_app` Sink**（应用消息）：corpid/agentid/touser 在 config，corpsecret 在 secret；access_token 包级缓存（按 corpid 分桶，提前 60s 过期），`42001/40014` 失效时强制刷新并重试一次。单测覆盖 token 缓存命中 + 过期刷新重试 + 请求体（agentid/touser）。
- [x] **M3-3 Sink RateLimiter**：包级 `rate.Limiter` 按渠道标识分桶（~20 条/分，突发 20），`Wait(ctx)` 触发限频时排队而非报错。
- [x] **M3-4 dead 任务手动重试**：`delivery.Repository.Requeue`（终态→pending、清零计数与锁）+ `app/delivery.Service.RetryDead`（校验状态后重排），API 在 M4 暴露。
- 验收：**可运行部分 PASS（单测验证请求构造/错误处理/token 刷新/限流）**。**剩余：需真实企业微信 corpid/secret/agentid 或群机器人 key 做端到端联调。**

## Phase 5 补充 — 钉钉自定义机器人 Sink

- [x] **`dingtalk_bot` Sink**（`internal/plugin/sink/dingtalk`）：自定义机器人 webhook，config.access_token（或 webhook_url 覆盖）+ 可选 secret 加签（HMAC-SHA256 + timestamp，钉钉安全设置三选一，未配置密钥则不加签）；text/markdown（markdown title 取 sink 名称）；`errcode` 判成功；限流沿用 wecom 的~20条/分包级分桶模式。单测覆盖成功/错误码/加签签名交叉校验/markdown title/配置校验。已在 `bootstrap/app.go` blank import 注册。
  - 剩余：需真实钉钉群机器人 access_token（+可选加签密钥）做端到端联调。

---

## M4 — 管理 API + 鉴权

- [x] **M4-1 `internal/app/*` 应用服务**：account / sink / template / rule / source（CRUD+sync+start/stop）/ delivery / apitoken，编排仓储。source 服务 enabled 切换联动启停监听。
- [x] **M4-2 `internal/api/dto` + `handler`**：account/sink/source/template/rule/delivery/token 全套 CRUD；DTO 脱敏（手机号掩码、secret 不回显 `has_secret`、sink config 敏感键 `***`、代理密码仅 `has_password`）。handler 只调 app service，不碰 GORM；DTO 与 domain 分离。全部挂到 `/api/v1`（`Auth` 中间件）。
- [x] **M4-3 token 管理**：`app/apitoken` + `cmd/token` 生成/列出/吊销；`POST /tokens` 明文仅回一次。
- 验收：**已补集成测试** — `TestAPIAuthAndCRUD`（无 token/错误 token → 401；有 token → 200；创建 webhook sink 201 且 secret 不回显、`webhook_url` 脱敏 `***`；删除 204）。真实 DB 运行需 `TMF_RUN_DB_TESTS=1`。
  - 额外接口：`POST /sources/sync?account_id=`（拉可选 peer）、`POST /sources/:id/start|stop`、`POST /deliveries/:id/retry`（手动重试）、`GET /sinks/types`。

---

## M5 — Web UI（后置）

- [x] Vue 3 + TS + Vite + Naive UI + Pinia + vue-router + axios 工程骨架（`web/`）；`/api` 开发代理到 8080；token 存 localStorage，请求拦截器注入 `Authorization: Bearer`。
- [x] 页面：dashboard（计数+投递成功率）/ accounts（列表+新建+删除，登录走 CLI 提示）/ sources（账号同步可选 peer→添加、启停、删除）/ sinks（类型下拉+config JSON+secret、启停、删除）/ rules（条件/处理器 JSON、来源多选、目标 sink+模板动态列表）/ templates（增删改）/ deliveries（状态筛选+手动重试）/ settings（token 配置+生成/吊销）。
- [x] `npm run build` 通过（`vue-tsc -b && vite build`，类型检查 + 生产构建，dist 产出成功；naive-ui 单 chunk 体积告警为已知，不阻断）。

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
