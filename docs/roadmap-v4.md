# Telegram Message Forward v4 AI 整理与 Briefing 路线图

本文件承接 `docs/product-architecture-v1.md`、`docs/roadmap-v1.md`、`docs/roadmap-v2.md`、`docs/roadmap-v3.md`。

v4 的目标是新增一条通用的 **AI 整理旁路**：原始消息继续按现有规则实时转发，AI 只作为独立消费者按用户配置收集、去重、聚合、总结，并把整理结果作为新的消息投递到指定渠道。

本功能不绑定股票或财经场景。财经新闻、普通新闻、RSS 订阅、技术资讯、群聊讨论、Webhook 输入都应通过同一套 AI Profile 配置表达。

## 当前状态快照

> 更新日期：2026-07-05

**已完成（本轮）**

- **V4-1 到 V4-5 主链路已落地**：AI Provider 配置、OpenAI-compatible HTTP client、AI Profile CRUD、草稿预览、已保存 Profile 预览、手动执行、AI 输出快照直投、重新投递、interval/daily 进程内调度均已实现。
- **多 Provider、预设与 Cron 增强已落地**：AI Provider 已支持多个 OpenAI-compatible 配置并按 Profile 选择 Provider/Model；内置“群消息归纳整理”“今日新闻总结”预设；调度支持标准 5 段 cron 表达式，并按 Profile timezone 计算。
- **V4-6 观测与调优已完成基础闭环**：Profile 列表展示最近 run，Run 详情展示纳入/排除消息、AI 输出、token usage、delivery task ids；Dashboard 增加 AI 整理概况；提供运行记录清理 API 与页面操作。
- **数据库迁移已执行到版本 15**：
  - `00013_ai_digest.sql`：新增 `ai_digest_profiles`、`ai_digest_runs`、`ai_digest_run_items`、`ai_digest_outputs`，以及 `messages(source_id, received_at)` 索引。
  - `00014_delivery_task_origin.sql`：扩展 `delivery_tasks` 支持 `origin_type/origin_id` 与 `message_snapshot` 直投，`message_id/rule_id` 对 AI 来源可空。
  - `00015_ai_digest_run_provider_snapshot.sql`：为 AI run 增加 Provider id/name 快照，便于追溯历史输出使用的供应商配置。
- **验证已完成**：`go test ./...`、`go vet ./...`、`go build ./...`、`cd web && npm run build` 全部通过。
- **新增自动化覆盖**：补充多 Provider 存储/指定 Provider 测试调用、cron next-run 计算测试。
- **本地 E2E 已完成**：使用本地 mock OpenAI-compatible provider 与 mock Webhook Sink，验证 Provider test、Webhook Source 入库、草稿预览、手动执行、AI 输出生成 delivery task、worker 投递成功；验证结果中 AI 投递任务 `message_id=0/rule_id=0`，说明快照直投路径生效。
- **提交记录**：
  - `036b0e5 feat: 新增 AI 整理后端链路`
  - `992d3cb feat: 新增 AI 整理管理页面`
  - `a20aac9 docs: 更新 V4 AI 整理真实进度`

**本轮增强（2026-07-05：输出结构模板复用 + AI 页面重构）**

- **输出结构模板抽成可复用实体**：
  - `00017_ai_digest_output_templates.sql`：新增 `ai_digest_output_templates` 表（含 `built_in` 标记与 4 个内置模板种子），`ai_digest_profiles` 增加 `output_template_id` 外键（`ON DELETE SET NULL`）。
  - domain/model/repository/service/dto/handler/router 全链路补齐模板 CRUD；`GET/POST/PUT/DELETE /ai/output-templates`。
  - Profile 通过 `output_template_id` 引用共享模板，运行时由 `resolveOutputTemplate` 解析最新内容；未选共享模板时回退内联 `output_template`，再回退内置默认。改共享模板会联动所有引用它的 Profile。
  - 删除保护：内置模板不可删除；被 Profile 引用的模板不可删除（返回引用数）。
- **AI 页面重构**：`AIDigestsPage.vue` 由三卡片竖排改为单页 Tab 分区（整理任务 / 输出模板 / AI 服务）；运行记录从常驻卡片改为从 Profile 打开的右侧抽屉；Provider/模板/Profile 删除统一走确认弹窗。
- **Profile 表单精简**：`AIDigestProfileForm.vue` 拆分区块（基础 / 输入与过滤 / 窗口与调度 / Prompt / 模型与输出 / 输出结构 / 输出渠道）；输出结构改为「选共享模板（含预览）/ 自定义内联」两态，并提供「复制为自定义」与「管理模板」入口。
- **验证**：`go build ./...`、`go vet ./...`、`go test ./...`、`cd web && npm run build` 均通过。**注意：`00017` 迁移尚未对真实库执行**，联调前需 `go run ./cmd/migrate -config ./configs/config.yaml up`。

**本轮增强（2026-07-05：可复用「过滤器」+ 术语澄清）**

- **术语澄清**：原「规则」实为带目标渠道的**转发规则**，侧栏/顶栏/页面标题统一为「转发规则」；「账号」→「TG 账号」、「模板」→「渲染模板」。
- **可复用过滤器实体（仅条件）**：
  - `00018_filters.sql`：新增 `filters` 表（一组命名的匹配条件），`rules` 与 `ai_digest_profiles` 各加 `filter_id` 外键（`ON DELETE SET NULL`）。
  - `00019_rule_filters.sql`：转发规则改为通过 `rule_filters` 多选共享过滤器，旧 `rules.filter_id` 已移除；AI Profile 暂保留单 `filter_id`。
  - domain/filter + model + repository（含 `CountReferences` 引用计数）+ app/filter service（校验 + 删除保护）+ dto/handler/router `/filters` + bootstrap 接线。
  - **引用即覆盖内联**：转发规则在仓储加载时按 `filter_ids` 批量解析并合并覆盖 `Conditions`（多个过滤器按 AND 语义全部通过才命中）；AI 整理在 `resolveConditions` 解析单个 `filter_id`。二者均支持「引用共享过滤器 / 自定义内联条件」二选一。规则预览与保存都会校验/解析引用的过滤器。
  - 处理器**不进共享过滤器**（仍留在转发规则自身；AI 不跑处理器）。
  - 前端新增「过滤器」侧栏页 + `FilterEditorModal`（复用条件构建器与 `/rules/meta` 描述符）；`RuleEditorModal` 与 `AIDigestProfileForm` 增加「条件来源」选择与引用预览。
  - 删除保护：被转发规则或 AI Profile 引用的过滤器不可删除（返回引用数）。
- **验证**：`go build/vet/test ./...` 与 `npm run build` 均通过。**注意：`00018` 迁移尚未对真实库执行**，联调前需 `go run ./cmd/migrate -config ./configs/config.yaml up`。

**本轮增强（2026-07-07：转发编排画布化 + 规则编辑器管道化，纯前端）**

- **编排页改为真连线画布**：引入 `@vue-flow/core`，`FlowPage` 的三列卡片板改为分层画布（来源/规则/渠道三栏自动布局 + 贝塞尔连线）。新增 `components/flow/`（`FlowCanvas.vue` + 三类自定义节点 + `types.ts`）。
  - 连线即编排：拖 来源→规则 追加 `source_ids`、规则→渠道 追加 `targets`（均走全量 PUT）；来源→渠道 直接弹出预填的新建规则；点击连线可「解除关联」。
  - 连线上显示模板名；引用缺失/端点停用的链路显示红色虚线告警；点击节点仍保留上下游联动高亮（选中/关联/变暗）与上下文操作条。
  - 数据模型与后端 API 零改动；概览卡、配置检查清单、搜索/只看异常工具栏全部保留。
- **规则编辑器管道化**：`RuleEditorModal` 由五段长表单改为「路由→来源→过滤→处理→目标/模板→预演」横向管道节点 + 单环节聚焦面板；名称/优先级/启用/命中即停收进「路由」环节；校验失败自动跳到问题环节；props 不变，`RulesPage` 与编排页无缝复用。
- **顺手修复**：编排页规则启停的全量 PUT 此前漏传 `filter_ids`，会把共享过滤器引用清空；已统一走含 `filter_ids` 的 `ruleBodyOf` 组装。
- **画布聚焦（第二轮）**：画布默认隐藏未接入任何规则的来源/渠道，工具栏加「显示未接入资源」开关；隐藏时用「未接入资源」货架列出（最多各 8 个，超出显示「还有 N 个…」），点 pill 显示到画布并选中（context-bar 提供沿此建规则等操作）；搜索/筛选导致画布为空时区分提示并提供「清除筛选」。
- **验证**：`npm run build`（vue-tsc + vite）通过；用本地 mock API + Chrome 冒烟画布渲染、节点选中联动、连线告警样式、拖拽连线（PUT 生效）、解除关联、管道编辑器与规则预演，控制台无报错。

**本轮增强（2026-07-07：编排页结构治理 + 路径检查器 + 过滤器资源节点，纯前端）**

- **拆分编排页结构**：`FlowPage` 拆出 `FlowToolbar` / `ResourceShelf` 组件，筛选、可见节点、选中联动等状态收进 `composables/useFlowBoard.ts`；为后续资源节点铺地基。
- **右侧路径检查器（RouteInspector）**：吸收并替代原 context-bar。选中规则显示完整路径（来源→规则→渠道/模板）、同源规则匹配顺序（priority 降序 + id 升序，与后端 `ListEnabledBySource` 一致）与条件/处理器；选中来源/渠道/过滤器显示「经过它的规则路径」；选中连线显示绑定详情并提供解除关联。规则节点加匹配序号角标与「命中即停」标记。
- **过滤器资源节点（只读引用试验）**：过滤器可作为画布节点出现——默认不渲染，仅在选中规则或工具栏开「显示资源层」时显示；过滤器→规则连线映射为追加 `filter_ids`，点连线可解除；模板不节点化（仍在边标签与检查器中展示）。后端执行模型零改动。
- **review 修复**：匹配序号只对启用规则编号（停用规则不占位、标「停用」，与引擎语义一致）；选中过滤器连线时保持端点节点可见（原先 selection 清空导致节点与连线当场消失）；给带内联条件的规则连过滤器时弹确认框（后端语义为 filter_ids 覆盖内联条件，连线不再静默覆盖）。
- **验证**：`npm run build`（vue-tsc + vite）通过。
- **已知观察项（暂不处理）**：选中带过滤器的规则时 rule/sink 列 x 坐标会位移且不自动 refit；检查器「处理」区块条件与处理器 chip 混排仅靠颜色区分。

**待真实外部联调**

- [ ] 真实 AI provider key 联调：当前仅用本地 mock 验证 OpenAI-compatible 协议路径，未调用真实模型供应商。
- [ ] 真实外部 Sink 联调：当前仅用本地 mock Webhook Sink 验证投递，未向企业微信、邮件、ntfy、Webhook 等真实生产渠道发送测试消息。
- [ ] 大窗口/高频 Profile 的成本与性能压测：当前完成功能与基础限制，尚未做大量消息窗口下的成本评估。

**已知待补 / 调优项**

- [ ] V4-6 的“复制输出”按钮尚未实现。
- [ ] V4-6 的“从 run 复制为新的 prompt/profile”尚未实现。
- [ ] Profile 列表当前展示最近 run 状态与时间，未专门展示耗时字段。
- [ ] Profile 列表当前展示调度配置，未专门展示下一次预计运行时间。
- [ ] `max_messages_per_run` 超限会写入 excluded reason；单条文本与 prompt 字符上限当前以截断方式处理，未为截断单独生成 excluded item。
- [ ] Sink 禁用时 AI 投递创建阶段会跳过该 Sink；如需“禁用 Sink 也生成 cancelled delivery task 方便审计”，需另行调整语义。

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
  -> 以 message_snapshot 直接创建 origin_type=ai_digest 的 delivery tasks
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
    "window"            jsonb NOT NULL DEFAULT '{}'::jsonb,
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
    profile_id            bigint REFERENCES ai_digest_profiles(id) ON DELETE CASCADE,
    status                text NOT NULL
                          CHECK (status IN ('pending','running','success','failed','cancelled')),
    trigger_type          text NOT NULL DEFAULT 'manual'
                          CHECK (trigger_type IN ('manual','schedule','preview')),
    window_start          timestamptz NOT NULL,
    window_end            timestamptz NOT NULL,
    input_message_count   integer NOT NULL DEFAULT 0,
    included_count        integer NOT NULL DEFAULT 0,
    excluded_count        integer NOT NULL DEFAULT 0,
    delivery_task_ids     jsonb NOT NULL DEFAULT '[]'::jsonb,
    model_name            text,
    token_usage           jsonb NOT NULL DEFAULT '{}'::jsonb,
    error                 text,
    started_at            timestamptz,
    finished_at           timestamptz,
    created_at            timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_ai_digest_runs_profile ON ai_digest_runs (profile_id, created_at DESC);
```

说明：运行记录列表和「同一 profile 防并发」检查（查询是否存在 running run）都依赖 `(profile_id, created_at)` 索引。不设 `output_message_id`：v4 第一轮 AI 输出不落 `messages` 表（见 9.2），输出通过 `ai_digest_outputs.run_id` 关联即可。

实现说明：草稿预览不强制保存 Profile，因此 `profile_id` 允许为空；已保存 Profile 的预览、手动执行和定时执行仍会关联实际 Profile。

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

### 4.5 delivery_tasks 扩展

现有 `delivery_tasks` 的 `message_id`、`rule_id` 均为 NOT NULL 外键，且有 `UNIQUE (message_id, rule_id, sink_id)`。AI 输出没有对应的 message 行和 rule 行，因此投递前必须先做一次 schema 扩展（归入 V4-4 里程碑）：

```sql
ALTER TABLE delivery_tasks ALTER COLUMN message_id DROP NOT NULL;
ALTER TABLE delivery_tasks ALTER COLUMN rule_id DROP NOT NULL;
ALTER TABLE delivery_tasks
    ADD COLUMN origin_type text NOT NULL DEFAULT 'rule'
    CHECK (origin_type IN ('rule','ai_digest'));
ALTER TABLE delivery_tasks ADD COLUMN origin_id bigint;

-- rule 来源保持原有幂等语义；ai_digest 来源允许重新投递创建新任务
ALTER TABLE delivery_tasks
    DROP CONSTRAINT delivery_tasks_message_id_rule_id_sink_id_key;
CREATE UNIQUE INDEX idx_delivery_tasks_rule_unique
    ON delivery_tasks (message_id, rule_id, sink_id)
    WHERE origin_type = 'rule';
CREATE INDEX idx_delivery_tasks_origin
    ON delivery_tasks (origin_type, origin_id);

ALTER TABLE delivery_tasks ADD CONSTRAINT chk_delivery_tasks_origin CHECK (
    (origin_type = 'rule' AND message_id IS NOT NULL AND rule_id IS NOT NULL)
    OR (origin_type = 'ai_digest' AND origin_id IS NOT NULL
        AND message_snapshot IS NOT NULL)
);
```

约定：

- `origin_type = 'ai_digest'` 时，`origin_id` 指向 `ai_digest_runs.id`，任务必须携带 `message_snapshot`（dispatch worker 已优先消费 snapshot）。
- worker 侧需要兼容 `message_id` 为空的任务：snapshot 为空且 `message_id` 为空时直接判定任务失败，不回查 messages。
- ai_digest 来源不加唯一约束，重新投递（`/runs/:id/deliver`）由 app service 控制频率，天然允许创建新任务。

### 4.6 设置表复用

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

时区约定：

- `schedule` 配置必须带 `timezone` 字段（IANA 名称，例如 `Asia/Shanghai`），`daily` 按该时区计算执行时刻。
- 未配置时默认使用服务进程时区，但 UI 必须显式展示实际生效的时区，避免容器内 UTC 与用户本地时间错位。

### 7.2 window 类型

```text
last_duration    最近 N 分钟/小时
since_last_run   从上次成功 run 到现在
fixed_daily      当天固定时间段
```

第一轮建议支持：

- `last_duration`
- `since_last_run`

窗口一律基于 `received_at` 划分（原因见 8.1）。`since_last_run` 以上次成功 run 的 `window_end` 作为本次 `window_start`，保证多次 run 之间不重不漏。

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
- 窗口一律按 `received_at` 划分，区间 `[window_start, window_end)`。入库时间连续、单调且不可空，能保证多次 run 之间不重不漏。
- `sent_at` 只用于 prompt 内的时间展示和消息排序，不参与窗口过滤。
- 默认排除空文本消息；有媒体但无文本的消息可以用媒体摘要进入输入。

不用 `sent_at` 划窗的原因：消息入库存在延迟（RSS 轮询间隔、Telegram 断线补拉、Webhook 重试），按 `sent_at` 划窗会永久漏掉「发送时间落在旧窗口、但入库晚于该窗口 run 时刻」的消息；且 `sent_at` 可空，混合列查询无法稳定利用索引。

现有索引只有 `messages(source_id, sent_at)`，V4-3 需新增 migration 建 `messages(source_id, received_at)` 索引支撑窗口查询。

### 8.2 条件过滤

第一轮复用现有 condition descriptor 的配置结构：

- `keyword_contains`
- `keyword_excludes`
- `regex`
- `message_type`
- `sender`
- `has_media`
- `media_type`
- `message_length`

注意：

- AI Profile 的 conditions 用于选择进入 AI 输入的消息，不影响原始转发规则。
- 不复用 `source` 条件：Profile 的 `source_ids` 已经完成来源筛选，再暴露 source 条件属于冗余配置面。

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

前提：现有 `delivery_tasks` 的 `message_id`、`rule_id` 均为 NOT NULL 外键并带 `UNIQUE (message_id, rule_id, sink_id)`，AI 输出既没有 message 行也没有 rule 行，**无论选哪条路都必须先做 4.5 的 delivery_tasks migration**，不存在零 schema 改动的方案。

两个候选方案：

1. **快照直投（v4 第一轮采用）**：app/aidigest 根据 target sink ids 直接创建 `origin_type=ai_digest` 的 delivery task，`origin_id` 记 run id，`message_snapshot` 使用 AI 输出构造的 `NormalizedMessage`（`message_type=text`，`text=output.content`）。依赖 4.5 migration：`message_id`/`rule_id` 可空 + origin 判别列。
2. **系统 Source**：新增 `source_type='ai_digest'` 的系统 Source（00010 已把 `sources.account_id` 改为可空，扩展 CHECK 约束即可），AI 输出落一条 message（`external_message_id` 用 run id），再走 delivery。此方案让 AI 输出进入消息列表并可被规则二次处理，但同样绕不开 `rule_id NOT NULL` 的问题，且引入「AI 输出再触发规则」的回环风险，需要额外的防环设计。

v4 第一轮结论：**快照直投 + 保存 output**。等确有「AI 输出参与消息列表 / 二次规则处理」的需求时，再评估系统 Source 方案并补防环设计。

重新投递语义：

- `/runs/:id/deliver` 直接用已保存的 output 重建 snapshot 创建新任务，不重复调用 AI。
- ai_digest 来源的任务不受 `(message_id, rule_id, sink_id)` 唯一约束限制（见 4.5 的 partial unique index），重复投递频率由 app service 控制。

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

POST   /api/v1/ai/digests/preview
POST   /api/v1/ai/digests/:id/preview
POST   /api/v1/ai/digests/:id/run
GET    /api/v1/ai/digests/:id/runs
GET    /api/v1/ai/runs/:id
POST   /api/v1/ai/runs/:id/cancel
POST   /api/v1/ai/runs/:id/deliver
```

说明：

- `preview` 调用 AI 但不投递，保存 run 可选；建议保存为 `trigger_type=preview`，方便对比效果。
- `POST /api/v1/ai/digests/preview` 是草稿预览：请求体携带完整 Profile 草稿配置，不要求 Profile 已保存，支撑「保存前先看效果」的表单流程（见 11.2）；`:id/preview` 则基于已保存配置。两者共用同一条预览执行逻辑。
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
- preview（含草稿预览）同样计入频率限制，防止表单页连点预览刷 token。

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

- [x] 新增 `settings` 读写 AI provider 配置。
- [x] API 支持读取、保存、测试 provider。
- [x] `internal/infra/ai` 实现 OpenAI-compatible HTTP client。
- [x] API key 加密存储，响应只返回 `has_api_key`。
- [x] Provider 测试接口发送一条短测试 prompt，并返回脱敏结果。
- [x] 前端 AI 页面提供 Provider 配置表单。

验收：

- [x] `go test ./...`、`go vet ./...`、`go build ./...` 通过。
- [x] 改前端则 `cd web && npm run build` 通过。
- [x] 未配置 api key 时测试返回可读错误。
- [x] api key 不出现在 API 响应、日志、git diff。

进度说明（2026-07-05）：Provider 配置复用 `settings.key=ai.provider`，API key 存入 `secret_encrypted`；本地 mock provider 已通过 `/ai/provider/test` 验证。

### V4-2 AI Digest Profile CRUD

目标：建立通用 Profile 配置能力。

- [x] 新增 migration：`ai_digest_profiles`。
- [x] 新增 domain/app/repository/API。
- [x] 支持 Profile CRUD。
- [x] 支持 source_ids、conditions、window、schedule、prompt_template、target_sink_ids、model_config、limits。
- [x] 前端新增「AI 整理」页面和 Profile 表单。

验收：

- [x] Profile 能创建、编辑、启停、删除。
- [x] DTO 不复用 GORM model。
- [x] handler 不直接访问 DB。
- [x] 表单不要求用户写 JSON。

进度说明（2026-07-05）：`internal/domain/aidigest`、`internal/app/aidigest`、`internal/storage/repository/aidigest_repository.go`、`internal/api/handler/aidigest_handler.go` 与 `web/src/pages/AIDigestsPage.vue` 已落地。

### V4-3 手动预览

目标：用户能在不投递的情况下验证整理效果。

- [x] 新增 migration：`ai_digest_runs`、`ai_digest_run_items`、`ai_digest_outputs`，以及 `messages(source_id, received_at)` 索引。
- [x] 实现基于 `received_at` 的窗口消息查询。
- [x] 支持草稿预览端点（未保存 Profile 也能预览）。
- [x] 复用 condition 过滤消息。
- [x] 实现轻量去重。
- [x] 构造 prompt。
- [x] 调用 AI provider。
- [x] 保存 preview run、items、output。
- [x] 前端显示预览结果、纳入消息、排除原因。

验收：

- [x] 预览不创建 delivery task。
- [x] AI 失败只记录 run failed，不影响原始消息。
- [x] 输出必须带来源编号。
- [~] 超限消息有 excluded reason。

进度说明（2026-07-05）：本地 E2E 已验证草稿预览成功，纳入 1 条 Webhook Source 消息并保存 output。`max_messages_per_run` 超限会写入 excluded reason；单条字符与 prompt 总字符上限当前按截断处理。

### V4-4 手动执行与投递

目标：AI 输出可以投递到指定 Sink。

- [x] 新增 migration：`delivery_tasks` 扩展 origin_type/origin_id、放开 message_id/rule_id 非空约束、调整唯一约束（见 4.5）。
- [x] dispatch worker 兼容 message_id 为空、只带 snapshot 的任务。
- [x] Profile 配置 target_sink_ids。
- [x] 手动 run 成功后为每个 target sink 创建 origin_type=ai_digest 的 delivery task。
- [x] message_snapshot 使用 AI 输出构造。
- [x] run 记录 delivery_task_ids。
- [x] 支持对已生成 output 重新投递。
- [x] 前端运行详情展示投递状态入口。

验收：

- [x] 原消息实时转发不受影响。
- [x] AI 输出能投递到单独 Sink。
- [~] Sink 禁用、模板格式不支持等错误走现有 delivery 机制。
- [x] 手动重新投递不会重新调用 AI。

进度说明（2026-07-05）：本地 E2E 已验证手动执行成功创建 `origin_type=ai_digest` 投递任务并由 worker 投递成功。当前实现对禁用 Sink 在创建阶段跳过，不创建 cancelled 任务；模板格式不兼容仍沿用 worker 校验路径。

### V4-5 定时执行

目标：Profile 能按 interval/daily 自动生成整理。

- [x] 新增 app/aidigest scheduler。
- [x] 服务启动时加载 enabled profiles。
- [x] 每分钟 tick 检查到期任务。
- [x] 防止同一 Profile 并发运行。
- [x] 支持 interval 和 daily，daily 按 schedule.timezone 计算执行时刻。
- [x] 支持 since_last_run 和 last_duration 窗口（按 received_at 划分，since_last_run 衔接上次 window_end）。
- [~] 前端展示下一次预计运行时间和生效时区。

验收：

- [x] 重启服务不会补跑大量历史窗口。
- [x] 同一 profile 不会并发创建两个 running run。
- [x] provider 失败会记录错误，下一轮仍可继续。

进度说明（2026-07-05）：scheduler 已随服务启动，每分钟扫描 enabled profiles。前端 Profile 表单可配置 schedule/timezone，列表展示调度配置；下一次预计运行时间字段已在后端 Profile DTO 中预留并计算，但列表尚未独立展示该字段。

### V4-6 可观测性与调优

目标：让 AI 整理可解释、可调优。

- [~] Profile 列表展示最近运行状态、耗时、错误。
- [x] Run 详情展示 input/included/excluded/output/token usage/delivery tasks。
- [ ] 支持复制输出。
- [ ] 支持从 run 复制为新的 prompt/profile。
- [~] Dashboard 可选展示 AI 近 24 小时成功率和 token usage。
- [x] 增加运行记录清理策略。

验收：

- [x] 用户不查数据库也能知道某条消息为什么没进入摘要。
- [~] 用户能比较不同 prompt 的输出效果。
- [x] 长期运行不会无限增长无用运行记录。

进度说明（2026-07-05）：Run 详情已展示纳入/排除消息、输出和 token usage；Dashboard 已展示 AI 整理概况，但当前统计基于各 Profile 最近 run，不是完整 24 小时 run 聚合。复制输出、从 run 复制为新 prompt/profile 尚未实现。

## 15. 推荐提交拆分

建议按以下 commit 边界推进：

1. `feat: 新增 AI provider 配置`
2. `feat: 接入 OpenAI 兼容 AI client`
3. `feat: 新增 AI 整理 Profile 模型`
4. `feat: 实现 AI Profile 管理 API`
5. `feat: 新增 AI 整理管理页面`
6. `feat: 支持 AI 整理预览`
7. `feat: 记录 AI 整理运行明细`
8. `feat: 扩展投递任务支持 AI 来源`
9. `feat: 支持 AI 整理结果投递`
10. `feat: 支持 AI Profile 定时执行`
11. `feat: 补强 AI 整理运行观测`
12. `docs: 补充 AI 整理使用说明`

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
按 docs/roadmap-v4.md 一次性推进完通用 AI 整理与 Briefing 功能，覆盖 V4-1 到 V4-6 全部里程碑，直到 AI Profile 可以手动预览、手动投递、定时执行，并且原始消息转发链路不受影响。中途不要停下来向用户请示或等待确认，遇到报错、编译失败、接口不通就自行调试解决后继续推进；只有在缺少只有用户才能提供的凭证，或出现路线图未覆盖的方向性架构分歧时才停下来提问。

运行与调试授权：
- 授权在本机开发环境自由运行和调试本项目，该运行运行，该调试调试，不需要逐步请示。
- 后端直接使用仓库现有 configs/config.yaml 启动：go run ./cmd/server；数据库 migration 用 go run ./cmd/migrate。
- 前端启动：cd web && npm run dev。
- 数据库等依赖如未运行，可用 docker compose 启动，但不得修改、覆盖或提交用户对 docker-compose.yml 的已有改动。
- 可以自由重启服务、查看日志、调用本地 API、用浏览器调试工具打开前端页面做端到端验证。
- 授权范围仅限本机开发环境；不得对外部生产渠道做破坏性操作，不得把测试消息投递到真实生产 Sink，除非用户明确要求。

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
   - 新增 ai_digest_runs / ai_digest_run_items / ai_digest_outputs，以及 messages(source_id, received_at) 索引。
   - 从 messages 按 received_at 窗口查询。
   - 复用 condition 过滤。
   - 轻量去重。
   - 构造 prompt 并调用 AI。
   - 保存 preview run 和 output。
   - 提供草稿预览端点，未保存 Profile 也能预览。
   - 前端展示纳入消息、排除原因、AI 输出。

4. 手动执行与投递：
   - 先做 delivery_tasks 扩展 migration：origin_type/origin_id、message_id/rule_id 可空、唯一约束按 roadmap 4.5 调整。
   - dispatch worker 兼容 message_id 为空、仅带 snapshot 的任务。
   - AI 输出以 origin_type=ai_digest 生成 delivery task，投递到 target sinks。
   - run 记录 delivery_task_ids。
   - 支持已生成 output 重新投递，不重复调用 AI。

5. 定时执行：
   - 新增 scheduler。
   - 支持 interval/daily，daily 按 schedule.timezone 计算执行时刻。
   - 支持 last_duration/since_last_run，均按 received_at 划窗。
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
- 除编译和测试外，用本地启动的后端和前端做端到端自测：配置 Provider、创建 Profile、跑草稿预览、手动执行、检查运行记录和投递任务状态，能实际调通的链路都要实际调通。
- 如没有真实 AI provider key 或外部渠道凭证，对应链路可用本地 mock 或桩服务验证，并在最终说明中列出未做真实联调的项和原因。

提交要求：
- 按功能边界拆中文 commit。
- 每个 commit 保持可 build。
- 提交前 git status，确认不包含无关文件。
- 不 stage 用户已有的 docker-compose.yml 或其它无关改动。

优先级：
按 V4-1 到 V4-6 顺序推进，一口气全部完成。预览完成后不需要停下来等用户确认输出质量，自测通过就继续做投递和定时调度；输出质量调优放在全链路跑通之后。
```

## 18. Flow 图引擎（转发编排引擎化）设计与路线

### 18.0 背景与结论

编排画布已完成三轮演进（画布化 → 结构治理 + 路径检查器 → 过滤器资源节点），但执行模型仍是 Rule 列表。用户确认需要**图本身成为执行模型**：源、过滤、处理、目标节点在画布上自由组合，连线即数据流，全实时逐条处理。已拍板的方向性决策：

1. **模板由目标节点继承**：模板是目标节点的属性（可空 = 原文投递），不做独立模板节点。
2. **priority / stop_on_match 提升为 Flow 级属性**：同源多条 Flow 按 priority 排序，命中即停作用于 Flow 之间；Flow 内部分支并行、无顺序语义。
3. **RulesPage 转「简单模式」**：读写线性形状的 Flow，与画布双向等价；后续视使用情况考虑砍掉。
4. batch 汇聚（合并转发/AI 摘要节点）**不在本期**：图引擎先做无状态实时链路，节点接口为未来有状态节点（实时去重、batch 汇聚）留扩展位。aidigest 保持独立垂直不动。

### 18.1 语义模型

**节点类型（4 种）**：

| 类型 | 语义 | 配置 |
|---|---|---|
| `source` | 入口：消息从这里进图 | `ref_id` = source_id |
| `filter` | 谓词：全部条件通过才继续向下游 | `config` = `{filter_ids}` 或 `{conditions}`（二选一，节点内 AND，复用 condition 注册表与共享过滤器语义） |
| `processor` | 改写本分支的消息快照 | `config` = `{processors: []}`（有序，复用 processor 注册表；通常一个，允许多个以减少节点数） |
| `target` | 出口：生成投递任务 | `ref_id` = sink_id，`template_id` 可空 = 原文投递 |

**连线与求值语义（引擎实现的规范）**：

- 串联 filter = AND；一个节点多条出边 = **分支扇出**，扇出时对每条分支克隆消息快照（沿用 `cloneMessage`），processor 只改本分支快照。
- 同一源被多条链/多条 Flow 引用 = 天然 OR。
- 汇入（多条入边指向同一节点）：每条到达路径独立执行该节点及其下游；但**同一 target 节点在一次求值中最多产出一次任务**（按 node id 去重，取先到达的快照），避免菱形拓扑重复投递。
- 图约束（保存时校验）：DAG 无环；source 只有出边、target 只有入边；每个 Flow 至少一条 source→target 通路；节点数/深度上限（建议 64 节点 / 16 深度）。
- Flow 间语义：按 `priority DESC, id ASC` 求值；某 Flow 产出 ≥1 个 target 任务且 `stop_on_match=true` 时，停止更低优先级 Flow——与现有 Rule 语义逐位对齐，保证迁移零失真。

### 18.2 执行架构

- 新增 `internal/domain/flow`（Flow/Node/Edge 领域模型 + Repository 接口）与 `internal/flowengine`（编译 + 求值）。
- **编译-执行分离**：加载图 → 编译为可执行计划（按 source_id 索引的邻接表 + 拓扑序），按 Flow `updated_at` 版本缓存，图不变不重编译。
- `ingest.Ingest()` 把 `rules.ListEnabledBySource + engine.Evaluate` 替换为 flow 版本；产出仍是 `[]Match`（快照 + sink + template），`dispatch.Queue` 及以下投递平面零改动。
- condition/processor 插件注册表、模板渲染、sink 适配器、重试机制全部原样复用。`ruleengine` 包在影子期保留，切换后移除或退化为 flowengine 的内部工具。
- 节点执行接口设计为可扩展（未来 stream 去重节点、batch 汇聚节点以新节点类型接入，不改图模型）。

### 18.3 数据模型（goose migration）

```sql
flows      (id, name, enabled, priority, stop_on_match, created_at, updated_at)
flow_nodes (id, flow_id, type, ref_id NULL, config JSON, template_id NULL, pos_x, pos_y)
           -- 自由编辑要求布局持久化；索引 (type, ref_id) 支持按源反查 Flow
flow_edges (id, flow_id, from_node_id, to_node_id, UNIQUE(flow_id, from_node_id, to_node_id))
```

- `delivery_tasks`：`origin_type` 增加 `'flow'`（复用 V4-5 已有的 origin 机制），新增 `origin_node_id` 记录产出任务的 target 节点；幂等键 flow 任务按 `(message_id, origin_type, origin_id, origin_node_id)`。`rule_id` 保留供存量数据查询。
- API DTO / domain / GORM model 三层分离；handler 不触 GORM；正式 schema 变更只走 `migrations/*.sql`。

### 18.4 迁移与影子验证（切换安全带，必须做）

1. **Rule→Flow 编译器**：`cmd/flowmigrate`（Go 命令，读 rules 写 flows）。每条 Rule 编译为一条线性 Flow：源节点们 → filter 节点（filter_ids/内联条件）→ processor 节点 → 各 target 节点（含 template_id）；Flow 的 name/priority/stop_on_match/enabled 原样继承。迁移可重复执行（按 rule id 幂等）。
2. **影子并跑**：config 增加 `flow_engine.mode: off | shadow | primary`。shadow 模式下 ingest 主链路仍走旧引擎，同时用 flowengine 求值同一消息，diff 两边产出（命中 flow/rule 集合、sink+template+快照文本），不一致记 WARN 日志；跑稳后切 primary。
3. 切 primary 后旧 Rule 表进入只读期，RulesPage 改造完成后再废弃 Rule API。

### 18.5 阶段任务

**F5-1 后端引擎（先行）**
- [x] `internal/domain/flow` + storage model/repository + goose migration（flows/flow_nodes/flow_edges + delivery_tasks 扩展）
- [x] `internal/flowengine`：编译（含图校验）+ 求值（扇出克隆、target 去重、Flow 间 priority/stop）+ 单元测试（含菱形、多源、多级 filter、stop_on_match 用例）
- [x] `/flows` CRUD API（DTO 校验图合法性，返回可读的校验错误）
- [x] `cmd/flowmigrate` + 影子模式接入 ingest + 引擎切换开关
- [x] 影子 diff 清零后切 primary（默认仍为 `off`，`shadow`/`primary` 由 `flow_engine.mode` 显式切换）

**F5-2 画布自由编辑**
- [x] 画布进入「编辑模式」：节点可拖动、布局写回 pos_x/pos_y；节点面板（来源/过滤器/渠道资源添加建节点，processor 节点从注册表选型）
- [x] 连线建边/删边直接读写 flow_edges；保存时后端校验错误在画布侧展示
- [x] Route Inspector 适配 Flow（编辑侧展示节点配置摘要，概览侧保留路径与匹配顺序）
- [x] 现有三栏自动布局保留为「概览模式」（只读投影）

**F5-3 RulesPage 转简单模式**
- [ ] RuleEditorModal 管道表单改为读写线性 Flow（表单壳 + 线性图的双向转换）
- [ ] Rule API 标记 deprecated，前端全部改走 `/flows`
- [ ] roadmap 记录 Rule 表/`ruleengine` 的移除计划

**F5-4 有状态节点（按需，另行确认后再做）**
- [ ] stream 去重节点（逐条实时判定 + 键值状态）
- [ ] batch 汇聚/AI 摘要节点（含 aidigest 收敛评估）

### 18.6 验证与提交要求

- 每个 Go 阶段：`go build ./...`、`go vet ./...`、`go test ./...`；flowengine 与旧 engine 的**对照测试**（同一批样例消息在两个引擎产出一致）作为 F5-1 的硬性验收。
- 改 web 后：`cd web && npm run build`；画布编辑用本地后端 + 浏览器端到端自测（建图、连线、保存、校验错误展示、消息实际流经新链路产生投递任务）。
- 迁移验证：对至少覆盖「多源、多目标、共享过滤器、内联条件、处理器、stop_on_match」的规则集合执行 flowmigrate，影子模式 diff 为零。
- 按功能边界拆中文 commit，每个 commit 可 build；提交前 `git status`，不 stage 无关文件（尤其用户的 docker-compose.yml 改动）。

### 18.7 目标模式提示词

```text
你在 D:\project\go_project\telegram-message-forward 工作。

先阅读：
- AGENTS.md
- docs/product-architecture-v1.md
- docs/roadmap-v4.md 第 18 章（Flow 图引擎设计与路线）
- internal/ruleengine/、internal/app/ingest/、internal/dispatch/ 现有实现

目标：
按 roadmap-v4 第 18 章推进 Flow 图引擎，完成 F5-1（后端引擎 + 迁移 + 影子验证 + 切换）与 F5-2（画布自由编辑），F5-3 视进度推进。执行语义、数据模型、校验规则以 18.1–18.4 为准，不要自行更改已拍板的决策（模板由目标节点继承、priority/stop_on_match 为 Flow 级、图必须 DAG、target 节点单次求值去重）。中途不要停下来请示；遇到编译失败、测试不过、接口不通自行调试解决。只有出现第 18 章未覆盖的方向性分歧，或需要只有用户能提供的凭证时才停下提问。

运行与调试授权：
- 授权在本机开发环境自由运行调试：go run ./cmd/server、go run ./cmd/migrate、cd web && npm run dev；依赖可用 docker compose 启动，但不得修改或提交用户对 docker-compose.yml 的已有改动。
- 不得对外部生产渠道做投递测试。

必须遵守：
- 默认中文沟通。
- 切换前旧链路行为不得变化；影子模式 diff 清零是切 primary 的前置条件。
- flowengine 复用 condition/processor 注册表与模板渲染，不重造轮子；dispatch 及以下投递平面零改动。
- handler 不直接访问 GORM；GORM model 只在 storage/model；API DTO、domain、model 三层分离；schema 变更只新增 migrations/*.sql。
- 不打印不提交 session、密钥、密码；不提交 configs/config.yaml、web/dist、node_modules、本地数据库。
- 工作区已有的用户改动不覆盖不回滚；提交前只 stage 当前任务相关文件。
- 每完成一个阶段更新 roadmap-v4 第 18.5 节的勾选状态。

推进顺序：
1. F5-1 按 18.5 清单顺序：domain/storage/migration → flowengine + 单测与对照测试 → /flows API → flowmigrate + 影子模式 → diff 清零切 primary。
2. F5-2 画布编辑模式，端到端自测通过。
3. F5-3 视进度推进；F5-4 不做，仅保留接口扩展位。
```
