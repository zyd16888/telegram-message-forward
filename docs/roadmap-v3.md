# Telegram Message Forward v3 媒体、规则、插件与观测增强路线图

本文件承接 `docs/product-architecture-v1.md`、`docs/roadmap-v1.md`、`docs/roadmap-v2.md`。

v1/v2 已经把消息转发主链路、管理后台登录、Telegram 登录 UI 化和 QR 登录推进到可用状态。v3 的目标是把项目从“文本消息可靠转发”推进到“媒体可转发、能力可解释、规则可扩展、插件可持续扩展、运行状态可排障”的阶段。

## 0. 口径与约束

- 默认使用简体中文沟通。
- 继续遵守既有分层边界：
  - `gotd/td` 只出现在 `internal/infra/telegram` 和 `internal/plugin/source/telegram`。
  - handler 只调用 app service，不直接访问 GORM。
  - GORM model 只在 `internal/storage/model`。
  - schema 变更只能新增 `migrations/*.sql`，不修改已执行 migration。
- 配置类凭证以“方便用户查看和修改”为优先，API/UI 可回显已保存的渠道或外部系统配置凭证；不要为了通用脱敏规则把这类配置隐藏或替换为 `***`，除非用户另行明确要求。
- 以下内容仍不得明文输出到日志、错误响应或提交到 Git：
  - Telegram session
  - 登录验证码
  - 2FA 密码
  - 数据库密码
  - 加密主密钥
  - 本地真实 `configs/config.yaml`
- 一键部署和内置 PostgreSQL 不进入 v3，本项目继续默认外部 PostgreSQL。
- Telegram 历史补拉先记录待办，不混入 v3 第一阶段媒体主线。

## 1. v3 总目标

v3 优先完成六条主线：

1. 建立目标渠道媒体能力矩阵，并让 UI 在配置时明确展示各 Sink 支持什么。
2. 让 Telegram 图片消息必须能传到支持图片的目标渠道，不支持图片的渠道有清晰降级文本。
3. 将 Telegram Source 改造成同账号连接复用，避免每个监听源都启动一个 Telegram client。
4. 扩展规则条件与处理器，让路由能力从基础关键词过滤升级到可解释的日常自动化规则。
5. 扩展 Sink/Source 插件，但每个插件都必须带能力声明、配置表单、测试和文档。
6. 补强可观测性，让失败投递、source 连接、sink 测试和规则命中都能排障。

## 2. 里程碑顺序

建议严格按以下顺序推进，不要一上来同时改 Telegram 更新循环、媒体下载和多个 Sink。

### V3-1 媒体能力矩阵与 capability 契约

目标：先把“哪些渠道支持哪些消息形态”定成系统契约，再改具体发送逻辑。

- [x] 新增 `docs/media-capability-matrix.md`，调研并记录以下渠道能力：
  - Webhook
  - 企业微信群机器人
  - 企业微信应用消息
  - 钉钉自定义机器人
  - 飞书机器人/应用消息
  - 邮件
  - Bark
  - ntfy
  - Gotify
- [x] 矩阵至少包含：
  - text
  - markdown
  - html
  - image/photo
  - file/document
  - audio/voice
  - video
  - 最大文本长度
  - 最大文件大小
  - 是否支持公网 URL
  - 是否需要先上传媒体
  - 是否支持二进制直传
  - 不支持时推荐降级方式
- [x] 扩展 `domain/sink.Capabilities`，从粗粒度 `SupportsImage/SupportsFile` 升级为可执行的媒体能力描述。
- [x] 扩展 Sink descriptor，让后台配置页能展示“此渠道支持/不支持”的格式能力。
- [x] 更新现有 Sink capabilities：
  - `webhook`
  - `wecom_bot`
  - `wecom_app`
  - `dingtalk_bot`
- [x] UI 配置渠道时展示 capability，不让用户猜渠道能否发图、发文件或发 markdown。

进度说明（2026-07-03）：V3-1 已完成并验证 `go test ./...`、`go vet ./...`、`go build ./...`、`cd web && npm run build`。真实外部渠道能力按官方文档建模，V3-2/V3-3 再落实际媒体下载和发送路径。

验收：

- `go test ./...` 通过。
- `cd web && npm run build` 通过。
- 后台创建/编辑 Sink 时能看到该渠道能力说明。
- 文档明确列出各渠道媒体支持边界和降级策略。

### V3-2 Telegram 媒体标准化与图片下载

目标：Telegram 图片必须能被识别、下载，并作为标准媒体进入投递 payload。

- [ ] 扩展 `domain/message.Media`：
  - type
  - mime_type
  - file_name
  - size
  - width
  - height
  - caption
  - local_path 或 storage_key
  - remote_url（可选）
  - ttl/created_at（如使用临时文件缓存）
- [ ] 扩展 Telegram normalizer：
  - 图片消息识别为 `photo`
  - document 图片识别为 image/document
  - album/grouped_id 保留
  - caption/text 语义明确
  - 原始链接 `original_url` 保留
- [ ] 新增 Telegram 媒体下载服务：
  - 按 source/account 复用已登录 client 或注入下载能力。
  - 限制最大文件大小。
  - 支持图片优先，文件后置。
  - 下载产物不进数据库大字段。
  - 临时文件路径不通过 API 泄漏给用户。
- [ ] 扩展 `plugin/sink.Payload`：
  - text
  - format
  - media[]
  - fallback_text
- [ ] 入队时保存处理后的 message snapshot，包含媒体元数据但不包含过大二进制。
- [ ] 对不支持图片的 Sink 生成清晰降级文本，例如：
  - `[图片消息] <caption>`
  - 原始 Telegram URL
  - 文件名/大小摘要

验收：

- Telegram 图片消息进入 `messages.media`。
- 至少有单测覆盖 photo/document image normalize。
- 图片下载失败不会导致整个 source 崩溃，应记录错误并按降级策略投递。
- 不支持图片的渠道能收到可读文本。

### V3-3 现有 Sink 媒体投递改造

目标：先让现有渠道按自身能力正确处理图片，不支持则明确降级。

- [ ] Webhook：
  - 默认仍可保持 text JSON。
  - 可选支持 media metadata 或 URL 字段。
  - 不默认发送二进制。
- [ ] 企业微信群机器人：
  - 按官方能力补图片发送路径。
  - 如需要 md5/base64 或上传流程，封装在 Sink 内。
  - 文本 + 图片组合要有明确顺序策略。
- [ ] 企业微信应用消息：
  - 按官方能力补图片/文件上传与发送路径。
  - access_token 缓存沿用现有实现。
- [ ] 钉钉自定义机器人：
  - 明确 markdown 图片链接、文本、文件能力边界。
  - 不支持本地图片直传时，必须给用户看见限制和降级。
- [ ] Sink 测试接口支持媒体测试或至少展示“此渠道当前测试仅覆盖文本”。

验收：

- 支持图片的现有 Sink 能发出 Telegram 图片。
- 不支持图片的 Sink 不假装成功发送图片，必须降级或返回可解释错误。
- 每个改动的 Sink 有单测覆盖成功/失败/降级。

### V3-4 同账号多监听源连接复用

目标：Telegram Source 从“每 source 一个 client”改为“每 account 一个 runner，多 source 订阅分发”。

- [ ] 新增 account runner 概念：
  - 每个 active account 最多一个 Telegram client/update loop。
  - runner 内维护 source 订阅表。
  - source 启停只增删订阅，不重复建 client。
- [ ] 更新 `app/source.Manager`：
  - 按 account 分组启动。
  - 支持动态启停 source。
  - StopAll 能正确取消所有 account runner。
- [ ] 更新 Telegram plugin：
  - update 到来后根据 peer 匹配多个 source。
  - 一个消息只投递给对应 source。
  - 账号掉线、未授权、Flood Wait 等错误要进入可观测状态。
- [ ] 为后续媒体下载复用同一个 account client 留接口。

验收：

- 同一账号 3 个 source 只启动一个 Telegram client。
- 启停单个 source 不影响同账号其它 source。
- `go test ./...` 通过。

### V3-5 规则能力扩展

目标：把规则从基础关键词过滤扩展为常用自动化能力，并保持 UI 表单化。

- [ ] 新增 Condition：
  - `source`
  - `sender`
  - `time_window`
  - `has_media`
  - `media_type`
  - `message_length`
- [ ] 新增 Processor：
  - `preserve_links`
  - `media_fallback_text`
  - `mask_sensitive`
  - `dedupe`
  - `quiet_hours`
  - `batch_digest`
- [ ] 扩展规则 UI：
  - 所有新 condition/processor 都通过 descriptor 渲染表单。
  - 不退回 JSON 编辑。
- [ ] 新增规则预演能力：
  - 选择或构造一条消息。
  - 返回命中的规则。
  - 返回处理后的文本/媒体摘要。
  - 返回将投递到哪些 Sink。
- [ ] 规则错误要可解释：
  - 正则错误
  - 模板不兼容
  - Sink 不支持媒体
  - 静默时间命中

验收：

- 每个 condition/processor 有单测。
- 规则 UI 能配置所有新增项。
- 规则预演至少支持用手工样例消息测试。

### V3-6 插件扩展

目标：按“一个插件一个完整闭环”的方式扩展，不只补发送函数。

推荐顺序：

1. 飞书 Sink
2. 邮件 Sink
3. ntfy Sink
4. Bark Sink
5. Gotify Sink
6. RSS Source
7. Webhook Source

每个 Sink 必须包含：

- [ ] 插件实现。
- [ ] capability 声明。
- [ ] descriptor 表单元数据。
- [ ] 配置校验。
- [ ] 连通性测试。
- [ ] 单测。
- [ ] 文档矩阵更新。

每个 Source 必须包含：

- [ ] Source 插件实现。
- [ ] 标准消息 normalize。
- [ ] 配置校验。
- [ ] 与 ingest 接线。
- [ ] 单测。
- [ ] UI 配置入口。

验收：

- 每个插件独立 commit。
- 每个 commit 保持 `go test ./...` 通过。
- 改前端则 `cd web && npm run build` 通过。

### V3-7 可观测性补强

目标：让用户能解释“为什么没收到、发到哪了、为什么失败、下一次什么时候重试”。

- [ ] Delivery 详情页：
  - task 基本信息
  - message snapshot
  - rule/sink/template
  - attempts 列表
  - 每次 attempt 的 started_at/finished_at/status/error/response_summary
- [ ] Delivery 列表增强：
  - 支持 rule/source/sink 过滤。
  - 支持 dead/retrying 快速筛选。
  - 支持批量重试 dead。
- [ ] Source 运行状态：
  - account runner 状态
  - 最近消息时间
  - 最近错误
  - 当前订阅 source 数
- [ ] Sink 观测：
  - 最近测试结果
  - 最近投递失败原因
  - 成功率统计
- [ ] Dashboard：
  - 改成基于时间窗口统计，而不是只看最近 200 条。
  - 展示失败 Top sink/rule/source。
  - 展示 worker/queue 状态。

验收：

- 排查一条失败投递不需要查数据库。
- Dashboard 能看到近 24 小时投递概况。
- Source 掉线/未授权能在 UI 中看到。

## 3. 历史补拉待办

历史补拉先记录，不进入 v3 第一批目标模式推进。

后续建议单独开 V3-History：

- [ ] 基于 `sources.last_message_id` 设计游标。
- [ ] 手动回捞最近 N 条。
- [ ] 启动时补漏。
- [ ] 断线恢复后增量追平。
- [ ] 避免历史回捞重复触发已投递消息。
- [ ] UI 提供“回捞预览”和“确认执行”。

## 4. 推荐提交拆分

建议按以下 commit 边界推进：

1. `docs: 新增媒体能力矩阵`
2. `refactor: 扩展 Sink 媒体能力契约`
3. `feat: 展示渠道媒体能力`
4. `feat: 支持 Telegram 图片媒体标准化`
5. `feat: 支持 Telegram 图片下载`
6. `refactor: 扩展投递 payload 媒体结构`
7. `feat: 支持企业微信图片投递`
8. `feat: 支持钉钉图片降级投递`
9. `refactor: 复用同账号 Telegram 监听连接`
10. `feat: 扩展规则条件`
11. `feat: 扩展规则处理器`
12. `feat: 新增规则预演`
13. `feat: 补充投递详情与 attempts 展示`
14. `feat: 补强运行观测面板`
15. `feat: 新增飞书 Sink`
16. `feat: 新增邮件 Sink`
17. `feat: 新增 ntfy Sink`
18. `feat: 新增 Bark Sink`
19. `feat: 新增 Gotify Sink`

## 5. 验证命令

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

仅文档改动：

```bash
git diff --check
```

真实外部联调需要单独标注是否执行：

- Telegram 真实账号登录与收消息。
- Telegram 图片消息接收与下载。
- 企业微信真实投递。
- 钉钉真实投递。
- 后续新增插件真实投递。

## 6. 目标模式提示词

下面提示词可直接交给目标模式 agent 使用。

```text
你在 D:\project\go_project\telegram-message-forward 工作。

先阅读：
- AGENTS.md
- docs/product-architecture-v1.md
- docs/roadmap-v1.md
- docs/roadmap-v2.md
- docs/roadmap-v3.md

目标：
按 docs/roadmap-v3.md 推进 v3 媒体、规则、插件与观测增强。采用目标模式持续推进，直到 v3 第一轮可用闭环完成；每个阶段保持可编译、可测试、可提交。

必须遵守：
- 默认中文沟通。
- 不处理一键部署和内置 PostgreSQL，本项目继续使用外部 PostgreSQL。
- Telegram 历史补拉只记录待办，不混入本轮媒体主线。
- 配置类凭证为方便用户编辑，API/UI 可回显已保存的渠道或外部系统配置凭证；不要把这些配置强制脱敏为 ***。
- Telegram session、验证码、2FA 密码、数据库密码、加密主密钥、日志里的 token/secret 不得明文输出或提交。
- gotd/td 只允许出现在 internal/infra/telegram 与 internal/plugin/source/telegram。
- handler 不直接访问 GORM；GORM model 只在 storage/model；schema 变更只能新增 migrations/*.sql。
- 不提交 configs/config.yaml、web/dist、node_modules、.tsbuildinfo、真实密钥或本地临时文件。
- 工作区如果有用户已有改动，不要覆盖、回滚或顺手整理；提交前只 stage 当前任务相关文件。

推进顺序：
1. 媒体能力矩阵与 capability 契约：
   - 新增 docs/media-capability-matrix.md。
   - 调研并记录 Webhook、企业微信机器人、企业微信应用、钉钉、飞书、邮件、Bark、ntfy、Gotify 的 text/markdown/html/image/file/audio/video 支持、大小限制、是否需要上传、降级策略。
   - 扩展 domain/sink.Capabilities 和 Sink descriptor。
   - 后台 Sink 配置页展示渠道能力。

2. Telegram 媒体标准化与图片下载：
   - 扩展 domain/message.Media。
   - 让 Telegram photo/document image 能写入 messages.media。
   - 增加图片下载能力，产物不进数据库大字段。
   - 下载失败要可降级，不要让 source 崩溃。

3. 投递 payload 与现有 Sink 媒体适配：
   - 扩展 plugin/sink.Payload 为 text + format + media[] + fallback_text。
   - Webhook 保持文本为主，可选带 media metadata。
   - 企业微信群机器人/企业微信应用按官方能力支持图片投递。
   - 钉钉按官方能力支持或明确降级。
   - 不支持图片的 Sink 必须收到可读降级文本。

4. 同账号多监听源连接复用：
   - 改成每个 account 一个 Telegram runner。
   - runner 内维护 source 订阅表。
   - source 启停只增删订阅，不重复启动 Telegram client。
   - 同账号多个 source 不互相影响。

5. 规则能力扩展：
   - 新增 source、sender、time_window、has_media、media_type、message_length 条件。
   - 新增 preserve_links、media_fallback_text、mask_sensitive、dedupe、quiet_hours、batch_digest 处理器。
   - UI 继续用 descriptor 表单配置，不退回 JSON 编辑。
   - 增加规则预演接口和 UI。

6. 可观测性补强：
   - Delivery 详情展示 attempts。
   - Delivery 支持 rule/source/sink 过滤与批量重试 dead。
   - Source 展示 runner 状态、最近消息时间、最近错误。
   - Dashboard 改成时间窗口统计，展示失败 Top sink/rule/source 和 queue/worker 状态。

7. 插件扩展：
   - 按飞书、邮件、ntfy、Bark、Gotify、RSS Source、Webhook Source 顺序推进。
   - 每个插件必须包含 capability、descriptor、配置校验、连通性测试、单测和文档矩阵更新。

验证要求：
- 每个 Go 阶段运行 go test ./...、go vet ./...、go build ./...。
- 改 web 后运行 cd web && npm run build。
- 只改文档时运行 git diff --check。
- 真实 Telegram/企业微信/钉钉/飞书等外部联调如果没有凭证，最终说明未验证原因。

提交要求：
- 按功能边界拆中文 commit。
- 每个 commit 保持可 build。
- 提交前 git status，确认不包含无关文件。
- 不 stage 用户已有的 docker-compose.yml 或其它无关改动。

优先级：
先完成 V3-1 到 V3-3，确保 Telegram 图片能进入系统并投递/降级；再做连接复用、规则扩展、可观测性和新插件。
```
