# AI 多模态分析与财经 VIP 配置设计

## 1. 目标

AI Digest 在保留纯文本整理能力的基础上，增加通用图片理解能力。通用层只负责把消息中的图片可靠、安全、可审计地提交给支持视觉输入的 AI Provider；财经新闻、盘中宝、机构推荐和游资推荐通过可配置的 Profile/Preset 实现，不把股票规则写进 AI Client、消息领域或 Telegram 适配层。

本阶段交付边界：

- 同时支持 OpenAI-compatible `chat_completions` 与 `responses` 图片输入。
- Provider 显式声明是否支持视觉以及视觉模型。
- Profile 可控制是否分析图片、图片精度、数量和字节限制，以及是否允许把媒体发送给外部 Provider。
- 从统一媒体存储读取图片，进行 MIME、大小和总量校验；单张图片失败时保留文字并记录原因。
- Telegram 相册按 `grouped_id` 作为同一组上下文提交，普通消息按消息分组。
- Run 详情保存并展示实际提交的媒体摘要、哈希、处理结果和跳过原因，不保存 Base64 正文。
- 提供“财经 VIP 图片候选池”预设，输出结构化候选标的和证据，明确区分事实提取与投资判断。

## 2. 第一性原理与边界

图片理解的本质不是 OCR 插件，而是 AI 请求多了一类内容部件。因此核心抽象是 `ContentPart{text|image}`，而不是“股票图片识别器”。财经场景只是一套 Prompt、输出结构和聚合规则。

数据流保持旁路：

```text
已入库消息 -> AI Digest 选窗/过滤 -> 媒体解析与限制 -> 多模态 AI 请求
           -> Run 审计/结构化结果 -> 原有投递队列 -> 各 Sink 分段投递
```

- 不修改 ingest 和 ruleengine 的实时语义，多模态失败不阻塞原始消息转发。
- 不把 `gotd/td`、GORM model 或 Provider 私有 DTO 扩散到领域层。
- 不下载任意外部 URL。只读取 ingest 已收编的 `StorageKey`，必要时才读取仍然存在的本地文件。
- VIP 图片属于可能受版权保护的付费内容，Profile 必须显式开启“允许发送媒体到外部 AI”。默认关闭。
- AI 输出仅形成“被推荐标的/候选池”，不自动给出买入指令，不伪造价格、行情或推荐主体。

## 3. 通用领域模型

### 3.1 Provider 能力

`ProviderConfig` 增加：

- `supports_vision`: 是否允许图片内容块。
- `vision_model`: 可选；为空时沿用文本模型。

Provider 测试继续保留纯文本探测，避免管理页测试依赖样例图片。Profile 开启图片分析时，服务端校验 Provider 已启用视觉能力。

### 3.2 Profile 多模态配置

`MultimodalConfig`：

- `enabled`: 是否将图片加入请求。
- `allow_external_media`: 是否明确允许付费/私有媒体外传。
- `image_detail`: `low|high|original|auto`，默认 `high`，兼顾财经小字识别与成本。
- `max_images_per_run`: 默认 12。
- `max_image_bytes`: 默认 10 MiB。
- `max_total_image_bytes`: 默认 40 MiB。
- `failure_mode`: 本阶段固定 `continue_text`，图片失败时继续文字分析并写审计。

限制是产品级保护，不直接照搬某一家 Provider 的最大值。Provider 的上限更大，不代表一次 Digest 应无限制发送图片。

### 3.3 通用请求

AI Client 请求从 `System + User` 扩为：

```go
type ContentPart struct {
    Type     string // text | image
    Text     string
    DataURL  string
    Detail   string
    MediaRef MediaRef
}
```

`MediaRef` 只用于内部审计，包含消息 ID、媒体序号、MIME、大小和 SHA256。序列化到 Provider 前不发送内部路径和存储键。

- Chat Completions: user `content` 为 `text` 与 `image_url` 数组。
- Responses: `input` 为 user message，内容为 `input_text` 与 `input_image` 数组。
- 无图片时继续生成原来的字符串形态，确保兼容已有 OpenAI-compatible Provider。

### 3.4 媒体处理

媒体解析器依赖 `mediastore.Store`：

1. 只接受 `image/photo` 且 MIME 为 JPEG、PNG、WEBP 或非动画 GIF。
2. 优先通过 `StorageKey` 打开；没有存储键时才尝试可读 `LocalPath`。
3. 流式读取并施加单图和总量上限，同时计算 SHA256。
4. 转为 data URL；请求与 Run 审计只保存摘要，不保存 Base64。
5. 按消息顺序和媒体顺序稳定排列；同一 `grouped_id` 的图片连续提交。

本阶段不引入图像压缩库。超过上限即跳过并在 Run 中说明，避免新增复杂编码链和画质损失。代表性样本评估后再决定是否加入自适应缩放或 OCR fallback。

## 4. 审计与数据保留

`ai_digest_runs.request_config` 增加多模态配置快照和媒体统计；新增 `media_audit` JSONB 字段，逐张记录：

- `message_id`、`media_index`、`file_name`、`mime_type`、`size`、`sha256`。
- `status`: `included|skipped|failed`。
- `reason` 和 `grouped_id`。

不保存 `local_path`、`storage_key`、Base64 或 Provider secret。现有 Run 清理会级联清理审计；媒体本体仍由统一媒体保留策略管理。Run 详情即使媒体本体已清理，仍可复盘当时提交了哪张图及为何跳过。

图片 SHA256 为未来缓存键预留事实数据，但本阶段不做跨 Run 响应缓存：同一图片在不同上下文、Prompt 和模型下结论可能不同，直接缓存完整分析结果会引入难以察觉的语义错误。

## 5. 财经 VIP 配置方案

内置 `finance_vip_candidates` Preset，建议绑定盘中宝、机构、游资等 VIP Source，并开启多模态。Prompt 要求模型：

- 先识别栏目、发布时间、盘中/盘后语境和推荐主体。
- 提取股票名称、代码、市场、推荐类型、题材、催化、价格信息、原图证据和置信度。
- 无法确认时保留 `unknown`，禁止依据常识补代码、价格或机构名。
- 同股票多来源观点合并展示，但保留每条证据来源。
- 明确标注“图片识别结果，非投资建议”。

输出采用 Markdown，内嵌稳定的 JSON 区块供后续候选池消费：

```json
{
  "document_type": "institution_recommendation",
  "publish_time": "2026-07-12T10:30:00+08:00",
  "market_session": "intraday",
  "recommendations": [
    {
      "stock_name": "示例股份",
      "stock_code": "600000",
      "market": "A",
      "recommender_type": "institution",
      "themes": ["机器人"],
      "catalysts": ["行业政策"],
      "price_mentions": [],
      "evidence": "图片中的原始推荐语句",
      "confidence": 0.94
    }
  ]
}
```

当前系统没有实时行情和证券主数据，因此不做“自动选出可买股票”。正确口径是从付费资讯里提取并排序被推荐标的。后续准确率评估通过后，可增加证券代码基础表校验、行情时效校验和人工确认工作台。

## 6. API 与 UI

- Provider 表单增加“支持图片理解”和“视觉模型”。
- Profile 的模型配置区增加图片分析开关；开启后显示外传确认、图片精度与三项限制。
- 未选择支持视觉的 Provider 时给出明确校验提示。
- Run 详情新增“图片请求”区，显示包含/跳过/失败数量及逐图审计，不展示内部路径。
- 修复 Profile 表单强制提交 `max_tokens: 0` 与 `max_prompt_chars: 0` 的问题，让已有后端配置真正生效。

## 7. TODO 与验收

- [x] M1：通用 `ContentPart`，两种 OpenAI-compatible API 图片请求及单元测试。
- [x] M2：Provider/Profile 多模态配置、媒体读取和限制、相册顺序、Run 媒体审计及 migration。
- [x] M3：管理 API、前端配置与 Run 详情；修复 Token/Prompt 上限表单提交。
- [x] M4：内置财经 VIP 候选池 Preset、Prompt 和输出模板。
- [x] M5：Go test/vet/build、前端 build、桌面和移动端视觉验证、分功能提交。

验收标准：纯文本 Profile 行为不变；视觉 Profile 能把已存储图片提交给两种 API；外传未确认或图片不合规时不会提交且原因可见；财经 Preset 可直接套用；Run 可复盘 Prompt、原文、图片摘要和结果；代码与前端构建全部通过。

## 8. 后续评估项

上线后用脱敏代表样本建立 50 到 100 张评估集，统计股票名/代码准确率、漏识别率、证据一致率和单次成本。只有当小字或复杂表格仍明显失败时，再引入 PaddleOCR 作为文字增强输入；只有当代码映射错误成为主要问题时，再引入 A/HK/US 证券基础表。两者都不应提前进入通用 AI Client。
