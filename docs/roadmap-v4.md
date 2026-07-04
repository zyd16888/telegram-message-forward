# Telegram Message Forward v4 AI 整理与 Briefing 路线图

本文件承接 `docs/product-architecture-v1.md`、`docs/roadmap-v1.md`、`docs/roadmap-v2.md`、`docs/roadmap-v3.md`。

v4 的目标是新增一条通用的 **AI 整理旁路**：原始消息继续按现有规则实时转发，AI 只作为独立消费者按用户配置收集、去重、聚合、总结，并把整理结果作为新的消息投递到指定渠道。

本功能不绑定股票或财经场景。财经新闻、普通新闻、RSS 订阅、技术资讯、群聊讨论、Webhook 输入都应通过同一套 AI Profile 配置表达。

## 0. 口径与约束

- 默认使用简体中文沟通。
- v4 不改变现有 Source -> ingest -> rule -> delivery 主链路。
- AI 整理失败不得阻塞原始消息入库、规则匹配和投递。
- AI 输出必须作为独立结果投递到用户选择的 Sink，不覆盖、不删除、不修改原始消息。
- AI 结果必须可追溯来源：至少保留 message id、source、sent_at、原文链接或原始文本片段引用。
- 不做投资建议、医疗建议、法律建议等高风险结论自动化；如果用户配置此类 prompt，也必须默认提示“仅基于输入消息整理，不构成建议”。
- 不把 Telegram session、验证码、2FA 密码、数据库密码、加密主密钥、渠道 secret、AI provider api key 明文输出到日志、错误响应或 Git。
- handler 只调用 app service，不直接访问 GORM。
- GORM model 只在 `internal/storage/model`。
- schema 变更只能新增 `migrations/*.sql`，不修改已执行 migration。
- 不提交 `configs/config.yaml`、`.env`、真实密钥、`web/dist`、`node_modules`、本地数据库或临时文件。

## 1. v4 总目标

v4 建立一套通用 AI Briefing 能力：

1. 用户可以创建多个 AI 整理 Profile，每个 Profile 选择输入 Source、过滤条件、时间窗口、Prompt、模型配置和输出 Sink。
2. 原始消息继续实时转发；AI Profile 独立按窗口收集消息并生成整理结果。
3. AI 整理支持手动预览、手动执行、定时执行和运行记录查看。
4. AI 输出复用现有 delivery queue 投递到目标渠道，例如另一个企业微信机器人、邮件、ntfy、Webhook。
5. AI 运行记录可解释：纳入了哪些消息、排除了哪些消息、模型输入摘要、输出内容、错误、耗时、token 用量。
6. AI provider 使用抽象接口，第一轮支持 OpenAI-compatible Chat Completions 或 Responses 风格的 HTTP Provider，后续可接本地模型或其它供应商。

## 2. 产品形态

### 2.1 AI Profile

AI Profile 是一套整理配置，不是单条消息处理器。

典型 Profile：

- 财经快讯整理。
- RSS 每日摘要。
- 技术资讯小时报。
- 群聊重点归纳。
- 安全漏洞快报。
- 产品反馈分类。
- 多频道今日重点。

Profile 建议字段：

```text
name
enabled
source_ids
conditions
schedule
window
dedupe
prompt_template
output_format
target_sink_ids
model_config
limits
created_at
updated_at
```

### 2.2 用户流程

1. 用户进入「AI 整理」页面。
2. 创建 Profile：
   - 选择一个或多个 Source。
   - 设置整理窗口，例如最近 30 分钟、每小时、每天固定时间。
   - 配置筛选条件，例如关键词、正则、消息类型、发送者。
   - 选择 Prompt 模板或手写 Prompt。
   - 选择输出格式：text / markdown / html。
   - 选择一个或多个输出 Sink。
3. 点击「生成预览」查看 AI 整理效果。
4. 预览满意后启用定时执行。
5. 后续在运行记录里查看每次 AI 整理的输入、输出、失败原因和投递状态。

### 2.3 输出示例

```text
【信息整理 10:00-11:00】

重点摘要：
1. 多条消息集中提到同一事件，主要变化是 xxx。
   来源：#1 #3 #8

2. 另一个值得关注的主题是 yyy。
   来源：#4 #6

分类列表：
- 政策/宏观：...
- 公司/产品：...
- 风险/待跟进：...

低优先级：
- 若干重复消息已合并。

来源：
#1 10:03 Source A - https://...
#3 10:18 Source B - https://...
#8 10:47 Source C - 无公开链接

说明：以上内容仅基于本窗口内消息整理，未使用外部事实补全。
```

## 3. 架构设计

### 3.1 总体链路

```text
Source
  -> ingest
      -> messages
      -> 原有 rule / delivery 实时转发
      -> AI Digest Collector 旁路可查询 messages

AI Profile Scheduler / Manual Run
  -> 选取窗口内 messages
  -> 条件过滤
  -> 去重与限量
  -> Prompt 构造
  -> AI Provider 调用
  -> ai_digest_outputs 保存结果
  -> 结果转换为内部消息或直接创建 delivery tasks
  -> 投递到 target sinks
```

### 3.2 为什么不用普通 Processor

现有 Processor 是单条消息内的文本处理，例如截断、脱敏、追加来源。AI 整理天然是多条消息聚合，具备时间窗口、去重、排序、批处理和运行记录。

因此 AI 整理不应塞进 `ruleengine/processor`，否则会破坏规则引擎单条消息处理模型。

### 3.3 模块边界

建议新增：

```text
internal/domain/aidigest
internal/app/aidigest
internal/infra/ai
internal/storage/repository/aidigest_repository.go
internal/api/handler/aidigest_handler.go
web/src/pages/AIDigestsPage.vue
web/src/components/ai/
```

职责：

- `domain/aidigest`：Profile、Run、RunItem、Output、ProviderConfig、Repository 接口。
- `app/aidigest`：Profile CRUD、预览、执行、调度、Prompt 构造、结果投递编排。
- `infra/ai`：AI provider client，负责 HTTP 请求、超时、错误分类、token usage 解析。
- `storage/repository`：GORM repository 与事务。
- `api/handler`：DTO、参数校验、调用 app service。
- `web`：Profile 管理、预览、运行记录、输出详情。

## 4. 数据模型

### 4.1 ai_digest_profiles

```sql
CREATE TABLE ai_digest_profiles (
    id                 bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name               text NOT NULL,
    enabled            boolean NOT NULL DEFAULT false,
    source_ids          jsonb NOT NULL DEFAULT '[]'::jsonb,
    conditions          jsonb NOT NULL DEFAULT '[]'::jsonb,
    schedule            jsonb NOT NULL DEFAULT '{}'::jsonb,
    window              jsonb NOT NULL DEFAULT '{}'::jsonb,
    dedupe              jsonb NOT NULL DEFAULT '{}'::jsonb,
    prompt_template     text NOT NULL,
    output_format       text NOT NULL DEFAULT 'markdown'
                       CHECK (output_format IN ('text','markdown','html')),
    target_sink_ids     jsonb NOT NULL DEFAULT '[]'::jsonb,
    model_config        jsonb NOT NULL DEFAULT '{}'::jsonb,
    limits              jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now()
);
```

### 4.2 ai_digest_runs

```sql
CREATE TABLE ai_digest_runs (
    id                   bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    profile_id            bigint NOT NULL REFERENCES ai_digest_profiles(id) ON DELETE CASCADE,
    status                text NOT NULL
                          CHECK (status IN ('pending','running','success','failed','cancelled')),
    trigger_type          text NOT NULL DEFAULT 'manual'
                          CHECK (trigger_type IN ('manual','schedule','preview')),
    window_start          timestamptz NOT NULL,
    window_end            timestamptz NOT NULL,
    input_message_count   integer NOT NULL DEFAULT 0,
    included_count        integer NOT NULL DEFAULT 0,
    excluded_count        integer NOT NULL DEFAULT 0,
    output_message_id     bigint REFERENCES messages(id) ON DELETE SET NULL,
    delivery_task_ids     jsonb NOT NULL DEFAULT '[]'::jsonb,
    model_name            text,
    token_usage           jsonb NOT NULL DEFAULT '{}'::jsonb,
    error                 text,
    started_at            timestamptz,
    finished_at           timestamptz,
    created_at            timestamptz NOT NULL DEFAULT now()
);
```

### 4.3 ai_digest_run_items

```sql
CREATE TABLE ai_digest_run_items (
    run_id          bigint NOT NULL REFERENCES ai_digest_runs(id) ON DELETE CASCADE,
    message_id      bigint NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    source_id       bigint NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
    included        boolean NOT NULL DEFAULT true,
    reason          text,
    score           double precision,
    sort_order      integer NOT NULL DEFAULT 0,
    PRIMARY KEY (run_id, message_id)
);
CREATE INDEX idx_ai_digest_run_items_message ON ai_digest_run_items (message_id);
```

### 4.4 ai_digest_outputs

```sql
CREATE TABLE ai_digest_outputs (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    run_id          bigint NOT NULL REFERENCES ai_digest_runs(id) ON DELETE CASCADE,
    format          text NOT NULL DEFAULT 'markdown'
                    CHECK (format IN ('text','markdown','html')),
    title           text,
    content         text NOT NULL,
    raw_response    jsonb,
    created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX idx_ai_digest_outputs_run ON ai_digest_outputs (run_id);
```

### 4.5 设置表复用

AI provider 的全局配置优先复用现有 `settings` 表：

```text
settings.key = "ai.provider"
value:
  provider_type
  base_url
  model
  timeout_seconds
  max_retries
  default_temperature
secret_encrypted:
  api_key
```

API 返回时只回显非敏感配置和 `has_api_key`。

## 5. Provider 抽象

### 5.1 接口

```go
type Client interface {
    Generate(ctx context.Context, req GenerateRequest) (*GenerateResult, error)
}

type GenerateRequest struct {
    Model       string
    System      string
    User        string
    Temperature float64
    MaxTokens   int
    Metadata    map[string]string
}

type GenerateResult struct {
    Text       string
    Raw        []byte
    Usage      TokenUsage
    Model      string
    FinishReason string
}
```

### 5.2 第一轮 Provider

第一轮只做 OpenAI-compatible HTTP provider：

- `base_url` 可配置。
- `api_key` 加密存储。
- `model` 可配置。
- 超时、重试次数、temperature、max_tokens 可配置。
- 错误分类至少区分：鉴权失败、限流、超时、响应解析失败、内容为空。

不要在第一轮绑定某个供应商的专有 SDK，优先用 HTTP client 降低依赖和部署复杂度。

## 6. Prompt 与输入构造

### 6.1 Prompt 模板原则

每个 Profile 有自己的 `prompt_template`。系统提供模板，但不写死场景。

模板变量建议：

```text
{{profile_name}}
{{window_start}}
{{window_end}}
{{message_count}}
{{messages}}
{{source_list}}
{{output_format}}
```

### 6.2 默认系统约束

所有 AI 调用必须注入不可关闭的基础约束：

```text
你是信息整理助手。只能基于用户提供的消息内容整理，不要编造事实。
如果输入不足以得出结论，请明确说明信息不足。
输出中的每条关键结论都要标注来源编号。
不要输出任何未在输入中出现的 secret、token、手机号、验证码或账号敏感信息。
如果内容涉及财经、医疗、法律或其它高风险领域，仅做信息整理，不构成建议。
```

### 6.3 输入消息格式

建议构造成结构化文本：

```text
#1
source: 财经新闻 RSS
time: 2026-07-04T10:03:00+08:00
sender: -
type: text
url: https://...
text:
...

#2
...
```

### 6.4 长输入处理

第一轮必须有硬限制：

- 每次 run 最大消息数。
- 每条消息最大输入字符数。
- 每次请求最大总字符数。
- 超限时按时间、去重和条件过滤后截断，并在 run 记录里写入 excluded reason。

后续可以做分批 map-reduce：

1. 每批消息先生成 batch summary。
2. 再对 batch summary 做最终 briefing。

第一轮可以先保留接口和限制，不必实现多轮 map-reduce。

## 7. 调度策略

### 7.1 schedule 类型

```text
manual      只手动执行
interval    每 N 分钟执行
daily       每天固定时间执行
cron        后置，不进入第一轮
```

第一轮建议支持：

- `manual`
- `interval`
- `daily`

### 7.2 window 类型

```text
last_duration    最近 N 分钟/小时
since_last_run   从上次成功 run 到现在
fixed_daily      当天固定时间段
```

第一轮建议支持：

- `last_duration`
- `since_last_run`

### 7.3 调度实现

v4 第一轮仍使用单体进程内 scheduler，不引入外部 MQ。

要求：

- 服务启动后加载 enabled profiles。
- 每分钟 tick 一次，找出到期 profile。
- 使用数据库 run 状态防止同一 profile 并发执行。
- 服务重启后不补跑大量历史窗口，只从当前时间继续调度。
- 手动执行不受 schedule 限制，但同一 profile 同时只能一个 running run。

## 8. 消息选择、过滤与去重

### 8.1 消息选择

基于 `messages` 表选择窗口内消息：

- `source_id IN profile.source_ids`
- `sent_at` 优先，缺失时用 `received_at`
- 窗口闭区间建议 `[window_start, window_end)`
- 默认排除空文本消息；有媒体但无文本的消息可以用媒体摘要进入输入。

需要为 `messages(source_id, sent_at)` 已有索引继续复用；如查询使用 `received_at` 较多，再新增索引。

### 8.2 条件过滤

第一轮复用现有 condition descriptor 的配置结构：

- `keyword_contains`
- `keyword_excludes`
- `regex`
- `message_type`
- `source`
- `sender`
- `has_media`
- `media_type`
- `message_length`

注意：AI Profile 的 conditions 用于选择进入 AI 输入的消息，不影响原始转发规则。

### 8.3 去重

第一轮做轻量去重：

- external message id 幂等已经由 messages 表保证。
- 对相同 source + 相同 text hash 的消息去重。
- 对相同 original_url 的消息去重。
- 对 RSS 里相同 link 的消息去重。

后续可以做 embedding 相似度聚类，但不进入第一轮。

## 9. 输出与投递

### 9.1 输出保存

AI 输出必须保存到 `ai_digest_outputs`，并关联 run。

### 9.2 投递策略

建议将 AI 输出转换为一条内部 message，再复用现有 delivery queue：

- 新增一个虚拟 Source 类型 `ai_digest`，或在 messages 表中保存 `source_id` 指向一个系统 Source。
- `external_message_id` 可使用 run id。
- `message_type = text`。
- `text = output.content`。
- `original_url` 留空。
- `raw_payload` 保存 profile/run 元数据。

第一轮更简单的方式：

- 不新增系统 Source。
- app/aidigest 直接根据 target sink ids 创建 delivery task，message_snapshot 使用 AI 输出构造的 `NormalizedMessage`。

两种方式取舍：

- 如果希望 AI 输出也出现在消息列表、规则链路、后续二次处理里，选择系统 Source。
- 如果只希望 AI 输出作为最终投递结果，选择直接创建 delivery task。

v4 第一轮建议：**直接创建 delivery task + 保存 output**。等需要二次规则处理时再引入系统 Source。

### 9.3 输出渠道

Profile 必须显式选择 target sinks。不要默认发到原消息同一个 Sink，避免 AI 输出被原消息刷屏淹没。

## 10. API 设计

建议新增：

```text
GET    /api/v1/ai/provider
PUT    /api/v1/ai/provider
POST   /api/v1/ai/provider/test

GET    /api/v1/ai/digests
POST   /api/v1/ai/digests
GET    /api/v1/ai/digests/:id
PUT    /api/v1/ai/digests/:id
DELETE /api/v1/ai/digests/:id

POST   /api/v1/ai/digests/:id/preview
POST   /api/v1/ai/digests/:id/run
GET    /api/v1/ai/digests/:id/runs
GET    /api/v1/ai/runs/:id
POST   /api/v1/ai/runs/:id/cancel
POST   /api/v1/ai/runs/:id/deliver
```

说明：

- `preview` 调用 AI 但不投递，保存 run 可选；建议保存为 `trigger_type=preview`，方便对比效果。
- `run` 调用 AI 并投递。
- `deliver` 用于把某次已生成 output 重新投递到 Profile target sinks。
- `cancel` 第一轮只取消 pending/running 标记；如果 HTTP 请求已发出，可通过 context 尽量中断。

## 11. 前端信息架构

新增页面：

```text
AI 整理
  - Profile 列表
  - 新建/编辑 Profile
  - 生成预览
  - 运行记录
  - 输出详情
  - Provider 设置入口
```

### 11.1 Profile 列表

展示：

- 名称。
- 启用状态。
- 输入 Source 数量。
- 输出 Sink。
- 调度方式。
- 最近运行状态。
- 最近成功时间。
- 最近错误。

操作：

- 新建。
- 编辑。
- 启用/停用。
- 预览。
- 立即执行。
- 查看运行记录。
- 删除。

### 11.2 Profile 编辑

表单分区：

1. 基础信息。
2. 输入来源。
3. 过滤条件。
4. 窗口与调度。
5. Prompt 与输出格式。
6. 模型与限制。
7. 输出渠道。

原则：

- 不让用户直接写 JSON 作为主路径。
- 条件配置复用现有 descriptor 表单。
- Prompt 使用大文本框，旁边提供模板变量说明。
- 预览按钮在保存前也能使用当前表单草稿。

### 11.3 运行详情

展示：

- run 基本信息。
- 窗口范围。
- 纳入消息列表。
- 排除消息及原因。
- Prompt 输入摘要。
- AI 输出。
- token usage。
- 投递任务 ID 和状态。
- 错误详情。

## 12. 可观测性与成本控制

必须记录：

- 每个 Profile 最近运行状态。
- 每次 run 的耗时。
- 输入消息数、纳入数、排除数。
- token usage。
- provider/model。
- 错误原因。
- 输出投递任务。

Dashboard 后续可增加：

- 近 24 小时 AI run 数。
- AI 成功率。
- token 用量估算。
- 失败 Top profiles。

成本控制：

- 每个 Profile 配置 max_messages_per_run。
- 每条消息 max_chars。
- 每次 prompt max_chars。
- 每次 output max_tokens。
- provider timeout。
- 每个 Profile 最小执行间隔，避免误配成高频刷模型。

## 13. 安全与隐私

- AI provider api key 必须加密存储。
- API 响应只返回 `has_api_key`。
- 发送给 AI 的内容可能包含用户消息，UI 需要在 Provider 设置处明确提示。
- 默认 prompt 要求不输出敏感信息。
- 运行记录里的 prompt/input 可以保存摘要和消息引用，第一轮不建议保存完整 prompt 原文到数据库；如保存必须明确受管理 API 鉴权保护。
- 错误日志不得打印完整 AI 请求体。
- AI 输出不自动执行外部动作，只投递文本。

## 14. 里程碑顺序

### V4-1 Provider 设置与 AI Client

目标：先打通可配置、可测试的 AI Provider。

- [ ] 新增 `settings` 读写 AI provider 配置。
- [ ] API 支持读取、保存、测试 provider。
- [ ] `internal/infra/ai` 实现 OpenAI-compatible HTTP client。
- [ ] API key 加密存储，响应只返回 `has_api_key`。
- [ ] Provider 测试接口发送一条短测试 prompt，并返回脱敏结果。
- [ ] 前端 Settings 或 AI 页面提供 Provider 配置表单。

验收：

- `go test ./...`、`go vet ./...`、`go build ./...` 通过。
- 改前端则 `cd web && npm run build` 通过。
- 未配置 api key 时测试返回可读错误。
- api key 不出现在 API 响应、日志、git diff。

### V4-2 AI Digest Profile CRUD

目标：建立通用 Profile 配置能力。

- [ ] 新增 migration：`ai_digest_profiles`。
- [ ] 新增 domain/app/repository/API。
- [ ] 支持 Profile CRUD。
- [ ] 支持 source_ids、conditions、window、schedule、prompt_template、target_sink_ids、model_config、limits。
- [ ] 前端新增「AI 整理」页面和 Profile 表单。

验收：

- Profile 能创建、编辑、启停、删除。
- DTO 不复用 GORM model。
- handler 不直接访问 DB。
- 表单不要求用户写 JSON。

### V4-3 手动预览

目标：用户能在不投递的情况下验证整理效果。

- [ ] 新增 migration：`ai_digest_runs`、`ai_digest_run_items`、`ai_digest_outputs`。
- [ ] 实现窗口消息查询。
- [ ] 复用 condition 过滤消息。
- [ ] 实现轻量去重。
- [ ] 构造 prompt。
- [ ] 调用 AI provider。
- [ ] 保存 preview run、items、output。
- [ ] 前端显示预览结果、纳入消息、排除原因。

验收：

- 预览不创建 delivery task。
- AI 失败只记录 run failed，不影响原始消息。
- 输出必须带来源编号。
- 超限消息有 excluded reason。

### V4-4 手动执行与投递

目标：AI 输出可以投递到指定 Sink。

- [ ] Profile 配置 target_sink_ids。
- [ ] 手动 run 成功后为每个 target sink 创建 delivery task。
- [ ] message_snapshot 使用 AI 输出构造。
- [ ] run 记录 delivery_task_ids。
- [ ] 支持对已生成 output 重新投递。
- [ ] 前端运行详情展示投递状态入口。

验收：

- 原消息实时转发不受影响。
- AI 输出能投递到单独 Sink。
- Sink 禁用、模板格式不支持等错误走现有 delivery 机制。
- 手动重新投递不会重新调用 AI。

### V4-5 定时执行

目标：Profile 能按 interval/daily 自动生成整理。

- [ ] 新增 app/aidigest scheduler。
- [ ] 服务启动时加载 enabled profiles。
- [ ] 每分钟 tick 检查到期任务。
- [ ] 防止同一 Profile 并发运行。
- [ ] 支持 interval 和 daily。
- [ ] 支持 since_last_run 和 last_duration 窗口。
- [ ] 前端展示下一次预计运行时间。

验收：

- 重启服务不会补跑大量历史窗口。
- 同一 profile 不会并发创建两个 running run。
- provider 失败会记录错误，下一轮仍可继续。

### V4-6 可观测性与调优

目标：让 AI 整理可解释、可调优。

- [ ] Profile 列表展示最近运行状态、耗时、错误。
- [ ] Run 详情展示 input/included/excluded/output/token usage/delivery tasks。
- [ ] 支持复制输出。
- [ ] 支持从 run 复制为新的 prompt/profile。
- [ ] Dashboard 可选展示 AI 近 24 小时成功率和 token usage。
- [ ] 增加运行记录清理策略。

验收：

- 用户不查数据库也能知道某条消息为什么没进入摘要。
- 用户能比较不同 prompt 的输出效果。
- 长期运行不会无限增长无用运行记录。

## 15. 推荐提交拆分

建议按以下 commit 边界推进：

1. `feat: 新增 AI provider 配置`
2. `feat: 接入 OpenAI 兼容 AI client`
3. `feat: 新增 AI 整理 Profile 模型`
4. `feat: 实现 AI Profile 管理 API`
5. `feat: 新增 AI 整理管理页面`
6. `feat: 支持 AI 整理预览`
7. `feat: 记录 AI 整理运行明细`
8. `feat: 支持 AI 整理结果投递`
9. `feat: 支持 AI Profile 定时执行`
10. `feat: 补强 AI 整理运行观测`
11. `docs: 补充 AI 整理使用说明`

每个 commit 必须保持可编译、可测试。

## 16. 验证命令

每个后端阶段：

```bash
go test ./...
go vet ./...
go build ./...
```

改前端后：

```bash
cd web
npm run build
```

只改文档：

```bash
git diff --check
```

真实外部联调需要单独标注是否执行：

- AI provider 真实调用。
- AI 输出真实投递到企业微信/邮件/ntfy/Webhook 等。
- 大量消息窗口的性能与成本评估。

## 17. 目标模式提示词

下面提示词可直接交给 Claude Code 使用。

```text
你在 D:\project\go_project\telegram-message-forward 工作。

先阅读：
- AGENTS.md
- docs/product-architecture-v1.md
- docs/roadmap-v1.md
- docs/roadmap-v2.md
- docs/roadmap-v3.md
- docs/roadmap-v4.md

目标：
按 docs/roadmap-v4.md 推进通用 AI 整理与 Briefing 功能。采用目标模式持续推进，直到 AI Profile 可以手动预览、手动投递、定时执行，并且原始消息转发链路不受影响。

必须遵守：
- 默认中文沟通。
- AI 整理是旁路能力，不得阻塞或改造现有 Source -> ingest -> rule -> delivery 实时转发主链路。
- AI 输出必须投递到 Profile 指定 Sink，不要默认混入原消息 Sink。
- 不把 AI 做成 ruleengine processor；AI Profile 是多消息聚合任务。
- Telegram session、验证码、2FA 密码、数据库密码、加密主密钥、渠道 secret、AI provider api key 不得明文输出或提交。
- AI provider api key 必须加密存储；API 响应只返回 has_api_key。
- handler 不直接访问 GORM；GORM model 只在 storage/model；schema 变更只能新增 migrations/*.sql。
- 不提交 configs/config.yaml、.env、web/dist、node_modules、.tsbuildinfo、真实密钥或本地临时文件。
- 工作区如果有用户已有改动，不要覆盖、回滚或顺手整理；提交前只 stage 当前任务相关文件。

推进顺序：
1. Provider 设置与 AI Client：
   - 复用 settings 表保存 provider 配置。
   - api key 加密存储。
   - 实现 OpenAI-compatible HTTP client。
   - 提供 provider test API 与前端配置入口。

2. AI Digest Profile CRUD：
   - 新增 ai_digest_profiles migration。
   - 新增 domain/app/repository/API。
   - 前端新增 AI 整理页面和 Profile 表单。
   - 表单支持 source、conditions、window、schedule、prompt、target sinks、model limits。

3. 手动预览：
   - 新增 ai_digest_runs / ai_digest_run_items / ai_digest_outputs。
   - 从 messages 按窗口查询。
   - 复用 condition 过滤。
   - 轻量去重。
   - 构造 prompt 并调用 AI。
   - 保存 preview run 和 output。
   - 前端展示纳入消息、排除原因、AI 输出。

4. 手动执行与投递：
   - AI 输出生成 delivery task，投递到 target sinks。
   - run 记录 delivery_task_ids。
   - 支持已生成 output 重新投递，不重复调用 AI。

5. 定时执行：
   - 新增 scheduler。
   - 支持 interval/daily。
   - 支持 last_duration/since_last_run。
   - 防止同一 profile 并发执行。
   - 服务重启后不补跑大量历史窗口。

6. 可观测性与调优：
   - Profile 列表展示最近运行状态。
   - Run 详情展示 input/included/excluded/output/token usage/delivery tasks。
   - Dashboard 可选展示 AI 运行概况。
   - 增加运行记录清理策略。

验证要求：
- 每个 Go 阶段运行 go test ./...、go vet ./...、go build ./...。
- 改 web 后运行 cd web && npm run build。
- 只改文档时运行 git diff --check。
- 如没有真实 AI provider key 或外部渠道凭证，最终说明未验证原因。

提交要求：
- 按功能边界拆中文 commit。
- 每个 commit 保持可 build。
- 提交前 git status，确认不包含无关文件。
- 不 stage 用户已有的 docker-compose.yml 或其它无关改动。

优先级：
先完成 Provider 配置、Profile CRUD 和手动预览；确认 AI 输出质量后，再做投递和定时调度。
```
