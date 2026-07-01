# Telegram Message Forward 产品需求与架构定稿 v1

## 1. 背景与目标

Telegram 的频道、群组和机器人生态成熟，很多信息源会优先发布在 Telegram。但在中国大陆网络环境下，普通用户无法稳定访问 Telegram，导致高价值消息无法被及时消费。

本项目的目标不是做一个简单的转发脚本，而是做一个可配置、可观测、可扩展的消息转发与路由工具：

> 通过 Telegram 用户账号监听频道、群组或私聊消息，将消息标准化后按规则过滤、模板化和分发到企业微信、飞书、钉钉、邮件、Webhook 等目标渠道。

第一阶段优先保证核心链路可靠：能登录、能监听、能配置、能转发、能看到投递结果。

## 2. 产品定位

### 2.1 核心定位

一个支持多 Telegram 账号、多监听源、多目标渠道、多路由规则的消息分发平台。

### 2.2 典型场景

- 订阅 Telegram 技术频道，将消息转发到企业微信群。
- 监听 Telegram 公告频道，将关键字命中的消息推送到飞书或钉钉。
- 将 Telegram 消息转为 Webhook，接入自有系统。
- 将多个频道的信息按主题分组，统一推送到不同通知渠道。
- 在大陆网络环境中，通过海外部署或代理实现 Telegram 到国内渠道的稳定转发。

### 2.3 非目标

v1 不追求以下能力：

- 不做完整 Telegram 客户端。
- 不做复杂 IM 多端同步。
- 不做全媒体格式 100% 还原。
- 不做复杂低代码/脚本执行平台。
- 不在第一阶段引入分布式部署、外部 MQ 或复杂插件市场。

## 3. 核心概念

### 3.1 Account

Telegram 用户账号。

Account 负责：

- 保存 Telegram 登录态和 session。
- 管理手机号、登录状态、代理配置。
- 同步账号可见的频道、群组和私聊。

### 3.2 Source

消息来源。

v1 主要是 Telegram 频道、群组或私聊。后续可以扩展 RSS、Webhook、GitHub、监控告警等来源。

### 3.3 NormalizedMessage

内部标准消息模型。

所有 Source 输入都需要先转换为统一消息结构，再进入规则引擎，避免下游插件直接依赖 Telegram 原始结构。

建议字段：

```text
NormalizedMessage
- id
- source_id
- source_type
- source_name
- account_id
- external_message_id
- grouped_id
- sender_type
- sender_id
- sender_name
- message_type
- text
- media[]
- links[]
- original_url
- sent_at
- received_at
- raw_payload
```

### 3.4 Sink

目标渠道。

例如企业微信、飞书、钉钉、邮件、Webhook、Bark、ntfy、Gotify、Server 酱等。

### 3.5 Rule

路由规则。

Rule 决定某条标准消息是否需要转发、如何处理、发到哪些目标渠道。

建议字段：

```text
Rule
- id
- name
- enabled
- priority
- conditions
- processors
- stop_on_match
- sources          -- 关联的监听源（rule_sources）
- targets          -- 目标渠道及其模板（rule_targets: sink + template）
```

来源与目标渠道用关联表表达（`rule_sources`、`rule_targets`），模板绑定在 target 上以匹配各 Sink 格式能力，详见数据模型章节。

### 3.6 DeliveryTask

投递任务。

监听线程不应该同步直接调用所有渠道。消息命中规则后，应生成投递任务，由异步 worker 执行，便于重试、限流和排障。

## 4. 功能范围

### 4.1 MVP 必做

- Telegram 用户账号登录。
- Telegram session 持久化。
- 支持代理配置。
- 同步账号可见频道、群组和私聊列表。
- 选择监听源。
- 配置目标渠道。
- 支持企业微信 Sink。
- 支持 Webhook Sink。
- 配置路由规则。
- 支持关键词包含、关键词排除、正则、消息类型过滤。
- 支持 text 和 markdown 模板。
- 文本消息转发。
- 基础图片消息转发。
- 投递队列。
- 失败重试。
- 投递历史和错误日志。
- Web UI 管理账号、来源、渠道、规则和投递记录。
- API 最小鉴权。

### 4.2 第二阶段

- 更多 Sink：飞书、钉钉、邮件、Bark、ntfy、Gotify。
- 消息去重。
- 消息合并和摘要推送。
- 静默时间。
- 文本截断和超长消息拆分。
- 媒体大小限制和降级策略。
- 敏感字段脱敏。
- 规则分组。
- 目标渠道健康检查。
- 手动重试失败任务。

### 4.3 后续增强

- RSS Source。
- Webhook Source。
- AI 摘要和翻译。
- 多实例部署。
- PostgreSQL 备份、归档和高可用部署。
- 更完整的审计和权限模型。
- 插件包动态加载或独立进程插件。

## 5. 架构原则

### 5.1 核心原则

- 简单优先：v1 使用单体服务、PostgreSQL 和内置 worker。
- 消息标准化：所有外部消息先转为内部模型。
- 输入输出分离：Source 和 Sink 是不同插件类型。
- 规则集中编排：路由、过滤、模板属于核心规则引擎。
- 投递异步化：监听、规则匹配、投递解耦。
- 可观测优先：每条消息和每次投递都有状态记录。
- 安全默认：管理 API 必须有最小鉴权，敏感配置不进入日志。

### 5.2 部署模式

推荐支持两种模式：

1. 海外 VPS 部署  
   服务直接访问 Telegram，再转发到国内可访问渠道。稳定性最好。

2. 国内机器加代理  
   Telegram Account 配置 SOCKS5 或 HTTP 代理。适合已有代理环境的用户。

混合部署不进入 v1。

## 6. 总体架构

```text
Telegram Account
    |
    v
Source Plugin
    |
    v
Message Normalizer
    |
    v
Rule Engine
    |-- Conditions
    |-- Processors
    |-- Template Renderer
    |
    v
Dispatch Queue
    |
    v
Delivery Worker
    |
    v
Sink Plugin
```

后台 UI/API 负责管理 Account、Source、Sink、Rule、Template 和 DeliveryTask。

## 7. 技术选型

### 7.1 后端技术栈

v1 后端继续使用 Go 单体服务。

定稿选型：

```text
语言：Go 1.26
HTTP API：Gin
Telegram MTProto：gotd/td
数据库：PostgreSQL
数据访问：GORM
数据库迁移：goose
日志：slog
配置：Viper + 环境变量
队列：PostgreSQL delivery_tasks 表
插件机制：编译期注册表
前端：Vue 3 + TypeScript + Vite + Naive UI 或 Element Plus
部署：Docker Compose 启动 app + postgres
```

### 7.2 Telegram 客户端

直接使用 gotd/td，不再继续基于 gotgproto 扩展。

原因：

- gotd/td 是更底层的 Telegram MTProto Go 客户端。
- 能覆盖用户账号登录、session、updates、频道/群组消息、媒体下载和上传等核心能力。
- 相比封装库，后续做多账号、Web 登录、代理、媒体处理和错误诊断时控制力更强。
- 业务层会通过 Telegram Source 适配层接触 gotd/td，避免 gotd/td 类型扩散到规则、投递和 Sink 代码。

约束：

- gotd/td 更底层，v1 需要多写一层适配代码。
- Telegram 原始消息必须先转换成 NormalizedMessage。
- 媒体下载、album、转发消息、reply 等能力按阶段补齐，不要求第一阶段一次性完整覆盖。
- 适配层必须内置 FLOOD_WAIT 处理与限流 middleware（floodwait + rate limit），避免高频调用触发账号限制。
- peer 的 `access_hash` 必须持久化缓存（见 `telegram_peers`），否则 `getMessages`、媒体下载、reply 等无法解析 peer。gotgproto 原本代管的 peer 缓存，切换到 raw gotd/td 后需自行维护。

### 7.3 数据库与 GORM

v1 从 PostgreSQL 起步，不再以 SQLite 作为主存储。

原因：

- 投递队列天然需要更可靠的并发锁和事务语义。
- PostgreSQL 更适合 JSONB 配置、投递日志、状态索引和后续统计查询。
- 避免后期从 SQLite 迁移到 PostgreSQL 时重写队列、时间、JSON、索引和锁相关逻辑。

GORM 作为主数据访问层保留，但需要约束使用方式：

- API handler 不直接调用 GORM。
- Service 不拼接 SQL。
- 所有数据库访问收口到 repository/store 包。
- 简单 CRUD 使用 GORM model/query。
- 复杂查询可以使用 GORM Raw，但必须集中在 repository/store 内部。
- 不使用 GORM AutoMigrate 作为正式迁移机制。
- schema 演进只通过 migrations 下的 SQL 文件管理。

### 7.4 Migration

使用 goose 管理独立 SQL migration。

建议目录：

```text
migrations
  000001_init.sql
  000002_add_delivery_tasks.sql
```

原则：

- migration 文件是数据库结构的唯一来源。
- 启动服务不默认隐式修改 schema。
- 提供明确命令执行迁移，例如 `server migrate up`。
- 开发环境可通过配置允许启动时自动迁移，但生产环境不建议开启。
- 测试 seed 和正式 migration 分离。

### 7.5 队列

v1 不引入 Redis、NATS 或外部 MQ。

投递队列基于 PostgreSQL 表实现：

```text
delivery_tasks
delivery_attempts
```

worker 从数据库领取 `pending` 或 `retrying` 任务，执行后更新状态。后续如需要多实例并发 worker，可基于 PostgreSQL 事务和行锁扩展。

### 7.6 插件机制

v1 不使用 Go runtime plugin。

原因：

- Go runtime plugin 跨平台限制明显。
- Windows 开发和部署体验不好。
- 外部动态插件会提前引入安全、版本和生命周期复杂度。

v1 使用编译期注册表：

```text
source.Register("telegram", NewTelegramSource)
sink.Register("wecom", NewWeComSink)
sink.Register("webhook", NewWebhookSink)
```

后续如果确实需要外部插件，再评估独立进程、gRPC、stdio 或 WASM。

## 8. 插件化与可插拔边界

### 8.1 真正插件化的部分

Source 和 Sink 做成插件。

它们对接外部系统，差异大，失败模式不同，配置不同，适合作为明确插件边界。

#### SourcePlugin

```text
SourcePlugin
- Name()
- ValidateConfig(config)
- Start(ctx, account, source, handler)
- Stop(ctx, source)
- SyncSources(ctx, account)
- Capabilities()
```

v1 内置：

- Telegram Source

后续可扩展：

- RSS Source
- Webhook Source
- GitHub Source

#### SinkPlugin

```text
SinkPlugin
- Name()
- ValidateConfig(config)
- Capabilities()
- Send(ctx, message, payload, options)
```

v1 内置：

- WeCom Sink，区分两种子类型：
  - `wecom_bot`：群机器人 webhook，无 access_token，配置简单。
  - `wecom_app`：应用消息，需要 corpid/secret/agentid，并维护 access_token 缓存与刷新、可信 IP。
- Webhook Sink

后续可扩展：

- Feishu Sink
- DingTalk Sink
- Email Sink
- Bark Sink
- ntfy Sink
- Gotify Sink

### 8.2 内部可插拔的部分

以下能力不做成外部插件，而是做成核心系统内部的可插拔组件：

- Condition
- Processor
- TemplateRenderer
- DedupeStrategy
- RateLimiter
- RetryPolicy

原因：

- 它们属于系统核心语义。
- UI 需要理解并配置这些能力。
- 投递排障需要统一解释规则命中原因。
- 过早做成外部插件会增加安全、生命周期和调试复杂度。

### 8.3 Condition

Condition 用注册表扩展。

v1 内置：

```text
keyword_contains
keyword_excludes
regex
message_type
source
sender
time_window
```

执行方式：

```text
Condition.Evaluate(ctx, message, config) -> bool
```

### 8.4 Processor

Processor 对消息进行轻量处理。

v1 内置：

```text
append_source
truncate_text
preserve_links
media_fallback_text
```

后续可扩展：

```text
dedupe
translate
summarize
mask_sensitive
batch_digest
```

### 8.5 TemplateRenderer

模板渲染是核心能力，不由每个 Sink 自行实现。

v1 支持：

- text
- markdown

后续支持：

- html

Sink 通过能力声明告诉系统支持哪种格式。

```text
Capabilities
- supports_text
- supports_markdown
- supports_html
- supports_image
- supports_file
- max_text_length
- max_file_size_mb
```

因为一条规则的多个目标 Sink 支持的格式不同，模板不直接挂在 Rule 上，而是挂在“规则-目标渠道”关系（`rule_targets`）上：每个 target 选择与该 Sink 能力匹配的模板；未指定时回退到纯文本渲染。渲染时以 target 的模板 format 与 Sink 能力做校验，避免把 markdown 投递到只支持 text 的渠道。

## 9. 消息处理流程

### 9.1 实时监听流程

```text
1. Telegram Source 收到原始消息
2. Normalizer 转为 NormalizedMessage
3. 保存 messages 记录
4. Rule Engine 匹配启用规则
5. 为命中的 target sinks 生成 delivery_tasks
6. Worker 异步执行投递
7. 写入 delivery_attempts
8. 更新 delivery_tasks 状态
```

### 9.2 投递状态

```text
pending
processing
success
failed
retrying
dead
cancelled
```

### 9.3 失败重试

v1 使用简单重试策略：

- 默认最多重试 3 次。
- 指数退避。
- 可手动重试 dead 任务。
- 每次失败记录错误信息和响应摘要。
- worker 领取任务时置 `processing` 并写 `locked_at`；超过可见性超时仍停留在 `processing` 的任务由 reaper 回退到 `retrying`，避免进程崩溃导致任务永久卡死。

### 9.4 投递语义与顺序

- 投递语义为 at-least-once。ingest 幂等（`messages` 唯一约束）与任务幂等（`delivery_tasks` 唯一约束）共同避免重复转发。
- 异步多 worker 下不保证跨消息的严格顺序。v1 接受“可能乱序”作为已知限制；如需同源顺序，可对同一 source 串行投递，后置实现。

## 10. 数据模型（v1 全新设计）

v1 使用 PostgreSQL 作为主存储，GORM 作为数据访问层，goose SQL migration 作为唯一 schema 演进机制。本节为全新设计，不沿用 `legacy/initial-poc` 的 SQLite 表结构。

通用约定：

- 主键使用 `bigint GENERATED ALWAYS AS IDENTITY`。
- 时间列使用 `timestamptz`，默认 `now()`。
- 灵活配置使用 `jsonb`。
- 敏感字段（session、secret、token、代理凭据、app_hash）加密后以 `bytea` 存储，列名统一加 `_encrypted` 后缀，明文不落库。
- 枚举取值通过 `CHECK` 约束或 Postgres enum 类型约束。
- 关系使用显式外键和关联表，避免把 id 列表塞进 `jsonb` 影响查询与索引。

### 10.1 accounts

Telegram 账号。session 与敏感凭据加密落库，不再使用文件路径。

```text
accounts
- id
- name
- phone_number            -- UI/API 脱敏展示
- app_id
- app_hash_encrypted
- session_encrypted       -- MTProto session blob，加密存储
- proxy_config            -- jsonb，凭据字段加密
- status                  -- inactive | logging_in | active | error | banned
- last_login_at
- last_error
- created_at
- updated_at
```

### 10.2 telegram_peers

peer 缓存，保存 `access_hash`。raw gotd/td 解析 peer、拉历史、下载媒体、reply 都依赖它，缺失会导致 peer 无法解析。

```text
telegram_peers
- id
- account_id             -- fk accounts
- peer_type              -- user | chat | channel
- peer_id                -- Telegram 侧 id
- access_hash
- username
- title
- updated_at
- unique (account_id, peer_type, peer_id)
```

### 10.3 sources

监听源，指向某账号下的一个 peer。

```text
sources
- id
- account_id             -- fk accounts
- peer_type              -- user | chat | channel
- peer_id
- name
- username
- enabled
- config                 -- jsonb
- last_message_id        -- 重启后增量回捞游标
- last_synced_at
- created_at
- updated_at
- unique (account_id, peer_type, peer_id)
```

### 10.4 sinks

目标渠道，secret 与非敏感 config 分开存储。

```text
sinks
- id
- type                   -- wecom_bot | wecom_app | webhook | ...
- name
- enabled
- config                 -- jsonb，非敏感配置
- secret_encrypted       -- 加密后的 secret/token/webhook key 等
- capabilities           -- jsonb，声明支持的格式与限制
- created_at
- updated_at
```

### 10.5 templates

```text
templates
- id
- name
- format                 -- text | markdown | html
- content
- created_at
- updated_at
```

### 10.6 rules

规则本体。source 与 target 用关联表表达，模板不挂在此处。

```text
rules
- id
- name
- enabled
- priority
- conditions             -- jsonb，Condition 配置数组
- processors             -- jsonb，Processor 配置数组
- stop_on_match
- created_at
- updated_at
```

### 10.7 rule_sources

规则与监听源的多对多关系。

```text
rule_sources
- rule_id                -- fk rules
- source_id              -- fk sources
- unique (rule_id, source_id)
```

### 10.8 rule_targets

规则与目标渠道的关系。模板绑定在这一层，以匹配各 Sink 的格式能力（text/markdown/html）。

```text
rule_targets
- id
- rule_id                -- fk rules
- sink_id                -- fk sinks
- template_id            -- fk templates，可空；空则按纯文本渲染
- unique (rule_id, sink_id)
```

### 10.9 messages

标准化后的消息。ingest 层通过唯一约束保证幂等，避免 updates 重放导致重复转发。

```text
messages
- id
- source_id              -- fk sources
- external_message_id    -- Telegram message id
- grouped_id             -- album 分组，可空
- message_type
- sender_peer_type
- sender_id
- sender_name
- text
- media                  -- jsonb
- links                  -- jsonb
- original_url
- raw_payload            -- jsonb，脱敏后存储，受留存策略约束
- sent_at
- received_at
- created_at
- unique (source_id, external_message_id)
- index (source_id, sent_at)
```

### 10.10 delivery_tasks

投递任务。带 `max_attempts`、`next_retry_at` 与领取锁字段，支持重试与僵尸任务回收。

```text
delivery_tasks
- id
- message_id             -- fk messages
- rule_id                -- fk rules
- sink_id                -- fk sinks
- template_id            -- 可空，创建时快照自 rule_targets
- status                 -- pending | processing | success | failed | retrying | dead | cancelled
- attempt_count
- max_attempts
- next_retry_at
- locked_at              -- 领取时间，配合可见性超时回收
- locked_by              -- worker 标识
- last_error
- created_at
- updated_at
- unique (message_id, rule_id, sink_id)
- index (status, next_retry_at)
- index (status, locked_at)
```

### 10.11 delivery_attempts

每次投递尝试记录，只存脱敏摘要，不含 secret。

```text
delivery_attempts
- id
- delivery_task_id       -- fk delivery_tasks
- attempt_no
- status                 -- success | failed
- request_summary        -- jsonb，脱敏
- response_summary       -- jsonb
- error
- started_at
- finished_at
- created_at
```

### 10.12 api_tokens

管理 API 的最小鉴权，只存 token 哈希。

```text
api_tokens
- id
- name
- token_hash             -- 只存哈希，不存明文
- created_at
- last_used_at
- revoked_at
```

### 10.13 settings

服务级配置（日志级别、worker 参数等），kv 存储。

```text
settings
- key
- value                  -- jsonb
- updated_at
```

### 10.14 索引与并发要点

- 队列领取使用 `SELECT ... FOR UPDATE SKIP LOCKED`，即便 v1 单实例也为多 worker 预留。
- 领取任务置 `processing` 并写 `locked_at`；超过可见性超时仍未完成的任务由 reaper 回退到 `retrying`。
- messages 与 delivery_tasks 的唯一约束共同保证“同一条消息对同一规则同一渠道最多一个任务”，实现端到端幂等。
- goose 自身的迁移版本表由 goose 管理，不在业务 schema 内手工设计。

## 11. Web UI 信息架构

v1 UI 定位为管理后台，不做营销页。

建议页面：

- Dashboard：运行状态、消息数、成功率、失败任务。
- Accounts：Telegram 账号登录、状态、代理配置。
- Sources：频道、群组、私聊列表，选择监听源。
- Sinks：目标渠道配置。
- Rules：来源、条件、处理器、模板、目标渠道编排。
- Templates：text/markdown 模板管理。
- Deliveries：投递历史、错误详情、手动重试。
- Settings：服务配置、API token、日志级别。

## 12. 安全要求

- 管理 API 必须有最小 token 鉴权，token 只存哈希（`api_tokens.token_hash`），不存明文。
- 敏感数据加密落库（v1 必做，不再作为可选项）：
  - Telegram session、sink secret/token、proxy 凭据、app_hash 使用 AES-GCM 加密后存 `bytea`。
  - 主密钥（KEK）来自环境变量或外部密钥管理，不写入代码库与配置示例。
  - 加解密收口在 `internal/infra/crypto`，其它层只接触密文或解密后的内存值。
- Telegram session、企业微信 secret、Webhook token 不打印到日志。
- UI 和 API 返回配置时默认脱敏，`phone_number` 等同样脱敏。
- 导出或备份功能不得默认包含明文密钥。
- 失败日志（`delivery_attempts`）只保存脱敏后的请求/响应摘要，不保存完整敏感请求。
- 本地开发配置文件、数据库文件和 session 不提交 Git。

v1 不做复杂多用户 RBAC，但 API token 鉴权与敏感字段加密/脱敏必须进入基础设计。

## 13. 推荐代码结构

目录组织按“入口层、应用层、领域层、基础设施层、插件层、接口层、前端层”划分。

核心原则：

- `cmd` 只做进程入口和启动编排，不写业务逻辑。
- `internal/domain` 放核心领域模型和接口，不依赖 Gin、GORM、gotd/td 或具体 Sink。
- `internal/app` 放应用服务，负责编排领域能力和事务边界。
- `internal/infra` 放外部依赖适配，包括数据库、Telegram、HTTP client、加密、日志等。
- `internal/plugin` 放 Source/Sink 插件注册表和内置插件实现。
- `internal/api` 只处理 HTTP 协议、DTO、鉴权和错误响应，不直接访问数据库。
- `web` 是前端工程，前后端通过 API 契约交互。
- `migrations` 是数据库 schema 的唯一来源，不把正式 schema 变更写在 Go 代码里。

推荐结构：

```text
cmd/server
  main.go

cmd/migrate
  main.go

internal/config
  config.go
  loader.go

internal/bootstrap
  app.go
  wire.go

internal/domain
  account
  source
  message
  rule
  template
  delivery
  sink

internal/app
  account
  source
  rule
  delivery
  sink

internal/storage
  db.go
  transaction.go
  model
  repository

internal/infra
  telegram
  httpclient
  crypto
  logger
  clock

internal/plugin
  source
    registry.go
    telegram
  sink
    registry.go
    wecom
    webhook

internal/ruleengine
  engine.go
  condition
  processor

internal/template
  renderer.go

internal/dispatch
  queue.go
  worker.go
  retry.go

internal/api
  router.go
  middleware
  dto
  handler

internal/security

migrations

configs
  config.example.yaml

web
  src
  package.json
```

以旧项目为参考，在当前仓库内渐进式重建新架构；新代码基于 Go 1.26，保持每个功能阶段可编译、可构建，等新主链路完成后移除旧实现。

### 13.1 依赖方向

推荐依赖方向：

```text
cmd
  -> bootstrap
    -> api
    -> app
      -> domain
      -> storage/repository
      -> plugin
      -> ruleengine
      -> dispatch
    -> infra
```

禁止方向：

- `domain` 不依赖 `api`、`storage`、`plugin`、`infra`。
- `api` 不直接依赖 `storage/model` 或 GORM。
- `plugin/sink/*` 不直接读取数据库。
- `plugin/source/*` 不直接执行路由规则。
- `ruleengine` 不直接调用外部 Sink。
- `dispatch` 不直接解析 Telegram 原始消息。

### 13.2 领域层

`internal/domain` 放最稳定的核心对象和接口。

建议拆分：

```text
internal/domain/account
internal/domain/source
internal/domain/message
internal/domain/rule
internal/domain/template
internal/domain/delivery
internal/domain/sink
```

领域层只表达业务概念：

- Account
- Source
- NormalizedMessage
- Rule
- ConditionConfig
- ProcessorConfig
- Template
- DeliveryTask
- DeliveryAttempt
- SinkConfig

领域层不关心：

- GORM tag。
- HTTP JSON DTO。
- gotd/td 原始类型。
- 企业微信请求结构。

### 13.3 应用层

`internal/app` 负责编排用例。

建议服务：

```text
internal/app/account
  service.go

internal/app/source
  service.go

internal/app/rule
  service.go

internal/app/delivery
  service.go

internal/app/sink
  service.go
```

典型职责：

- AccountService：登录、session 状态、代理配置。
- SourceService：同步频道列表、启停监听源。
- RuleService：规则增删改查、规则校验、模板关联。
- DeliveryService：查询投递记录、手动重试、失败详情。
- SinkService：目标渠道配置、连通性检查、能力声明。

事务边界优先放在应用层。

### 13.4 存储层

`internal/storage` 封装 PostgreSQL、GORM 和 repository。

建议：

```text
internal/storage
  db.go
  transaction.go
  model
    account.go
    source.go
    sink.go
    rule.go
    message.go
    delivery.go
  repository
    account_repository.go
    source_repository.go
    sink_repository.go
    rule_repository.go
    message_repository.go
    delivery_repository.go
```

约束：

- GORM model 只放 `internal/storage/model`。
- repository 对外返回 domain 对象，不返回 GORM model。
- SQL Raw 只允许出现在 repository 内部。
- 正式 schema 变更必须写到 `migrations`。
- seed 数据不要写进 migration，单独放开发脚本或初始化命令。

### 13.5 Telegram Source

gotd/td 只允许出现在 Telegram 适配层：

```text
internal/infra/telegram
internal/plugin/source/telegram
```

`internal/plugin/source/telegram` 负责实现 SourcePlugin：

- 登录。
- session 管理。
- 代理配置。
- 同步 chats。
- 监听 updates。
- 将 gotd/td 原始消息转换为 NormalizedMessage。
- 下载媒体并转换为内部 Media 描述。

规则引擎、投递队列和 Sink 不允许直接依赖 gotd/td 类型。

### 13.6 Sink 插件

Sink 插件放在：

```text
internal/plugin/sink/wecom
internal/plugin/sink/webhook
```

每个 Sink 只做目标渠道适配：

- 校验配置。
- 声明能力。
- 将内部投递 payload 转为目标渠道请求。
- 调用外部 API。
- 返回标准投递结果。

Sink 不负责：

- 查询数据库。
- 判断规则。
- 重试调度。
- 持久化投递结果。

### 13.7 API 层

`internal/api` 只处理 HTTP 协议。

建议：

```text
internal/api
  router.go
  middleware
    auth.go
    request_id.go
    recovery.go
  dto
    account.go
    source.go
    sink.go
    rule.go
    delivery.go
  handler
    account_handler.go
    source_handler.go
    sink_handler.go
    rule_handler.go
    delivery_handler.go
```

约束：

- handler 调用 app service。
- handler 不直接访问 GORM。
- DTO 和 domain 分开，避免把内部字段暴露给 UI。
- 敏感字段在 DTO 层统一脱敏。

### 13.8 前端结构

前端建议独立放在 `web` 下：

```text
web
  src
    app
    pages
      dashboard
      accounts
      sources
      sinks
      rules
      templates
      deliveries
      settings
    components
    api
    stores
    router
    types
    utils
```

前端原则：

- 页面按业务模块拆分。
- API client 单独放 `web/src/api`。
- 后端响应类型放 `web/src/types`。
- 复杂表单组件可沉淀到 `web/src/components`。
- 不在页面组件中硬编码 API URL、token 或渠道能力。

### 13.9 渐进迁移策略

当前旧项目已归档到 `legacy/initial-poc/`，后续按以下顺序迁移：

0. 先做端到端 walking skeleton（见 Phase 0）：单账号 CLI 登录（raw gotd/td）→ 监听单个 source → Normalizer → 一条硬编码规则 → 一个 Webhook/WeCom Sink，把一条文本消息端到端转出，验证 gotd/td 适配层、NormalizedMessage、Sink 抽象三者对齐。
1. 骨架跑通后，再新建完整目标目录结构。
2. 重新初始化 Go 1.26 模块和基础依赖。
3. 先迁移配置、日志、数据库连接和 migration。
4. 将 gotd/td Telegram Source 独立出来。
5. 将企业微信逻辑迁到 `internal/plugin/sink/wecom`。
6. 建立 NormalizedMessage 和 RuleEngine。
7. 引入 DeliveryTask 表和 worker。
8. 最后补 UI 和 API。

先纵向打通主链路，再横向回填分层。把 raw gotd/td 隐性成本、模板-Sink 能力匹配、access_hash 这些高返工风险点在铺大架构之前暴露出来，避免一次性大爆炸式重构。

## 14. 阶段路线

### Phase 0：端到端 walking skeleton

- 单账号命令行登录（raw gotd/td，SendCode / SignIn / 2FA）。
- 监听单个 source，转成 NormalizedMessage。
- 一条硬编码规则 + 一个 Webhook 或 WeCom Sink，把一条文本消息端到端转出。
- 目的：先暴露 gotd/td 适配层、NormalizedMessage、Sink 抽象、access_hash、模板-Sink 匹配等高返工风险点，再铺完整分层。

### Phase 1：核心模型收口

- 整理代码结构，建立目标目录骨架。
- 将 Telegram 客户端从 gotgproto 切换为 gotd/td 适配层。
- 修正 Telegram peer 解析，建立 `telegram_peers` access_hash 缓存。
- 统一 PluginManager 生命周期。
- 引入 PostgreSQL、GORM repository 和 goose migration。
- 拆分数据库 migration 和测试 seed。
- 建立 Account、Source、Sink、Rule、Message、DeliveryTask 基础模型，落地 messages/delivery_tasks 幂等约束。
- 落地敏感数据加密（session/secret/proxy/app_hash）。
- 迁移企业微信文本转发能力到 wecom Sink（`wecom_bot` / `wecom_app` 子类型）。

### Phase 2：管理后台闭环

- 新增 API 鉴权。
- 完成账号、来源、渠道、规则、投递记录 API。
- 建立基础 Web UI。
- 支持 Telegram 账号登录和频道列表同步。
- 支持规则配置和插件配置。

### Phase 3：可靠投递

- 引入数据库投递队列。
- 实现 worker、重试、失败记录。
- 支持投递历史和手动重试。
- 完成 Webhook Sink。
- 完善企业微信 Sink。

### Phase 4：媒体与增强规则

- 支持图片转发。
- 支持媒体降级策略。
- 支持更多 Condition 和 Processor。
- 支持模板预览。
- 支持消息去重和静默时间。

### Phase 5：更多插件

- 飞书。
- 钉钉。
- 邮件。
- Bark。
- ntfy。
- RSS Source。
- Webhook Source。

## 15. 当前决策定稿

- Telegram 侧以用户账号 MTProto 为主，不以 Bot API 为主。
- 新代码基于 Go 1.26。
- Telegram 客户端直接使用 gotd/td，弃用 gotgproto。
- Source 和 Sink 做正式插件化。
- 路由、过滤、模板、去重、限流、重试策略先作为核心系统内部可插拔组件。
- v1 使用单体 Go 服务和 PostgreSQL。
- SQL 访问使用 GORM，但数据库迁移使用独立 SQL migration，不使用 AutoMigrate 作为正式迁移机制。
- 监听消息后先标准化，再规则匹配，再进入投递队列。
- UI 第一版以管理后台为主。
- MVP 优先保证文本消息稳定转发和投递可观测。
- 图片消息作为 v1 后半段能力，文件和复杂媒体后置。
- 数据库 schema 全新设计，不沿用 legacy POC 的 SQLite 结构。
- 敏感数据加密落库（session/secret/proxy/app_hash），API 用 token 哈希鉴权。
- peer 的 access_hash 持久化缓存（`telegram_peers`）。
- 模板绑定在 `rule_targets`（规则-渠道关系）上，而非 Rule 本体，以匹配各 Sink 格式能力。
- 采用 walking skeleton 先打通端到端主链路，再回填完整分层。

## 16. 已确认决策（原待确认问题定稿）

- 多账号：v1 先跑通单账号，但数据模型全程带 `account_id`，支持后续扩展多账号而不需重构。
- UI：v1 先做 API + 首次命令行登录，Web UI 作为薄管理壳后置，不阻塞核心链路。
- 渠道：WeCom Sink 与 Webhook Sink 同为 MVP 必做。Webhook 作为验证“标准消息 → Sink 抽象”的最低成本试金石优先落地。
- 登录：v1 允许首次命令行登录（SendCode / SignIn / 2FA），UI 只负责展示与管理登录状态；Web 化登录流后置。
- 敏感配置加密：v1 必须落地，不作为可选项。session 加密为最高优先级（session 等同账号完全控制权），sink secret、proxy 凭据、app_hash 一并加密落库，明文不进日志、不进备份、不进 API 响应。
