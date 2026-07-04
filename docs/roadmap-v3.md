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

当前边界校准（2026-07-04）：

- v3 第一轮已经闭环的是“图片主线”：Telegram photo / image document 标准化、下载、媒体存储、公网 URL、按 Sink 能力投递或降级。
- 文件、音频、视频等复杂媒体可以在 `domain/message.Media` 和 capability 矩阵中表达，但当前内置 Telegram Source 尚未下载这些二进制，部分 Sink 也只做文本降级。后续应单独开“复杂媒体”阶段，不把它们算作 v3 第一轮已完成。
- 企业微信渠道的官方能力覆盖 file/audio/video，但当前内置实现只真正发送本地图片；文件、音频、视频上传发送路径后置，文档和 UI 能力提示必须避免让用户误以为已可用。
- Telegram Source 当前声明了 `SupportsHistory`，但历史补拉接口、游标更新和 UI 预览确认尚未实现；V3-History 开始前需要先把插件能力声明与实际接口对齐。

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

校准说明（2026-07-04）：capability 展示必须区分“官方渠道能力”和“当前内置实现”。当前第一轮实现以图片为主，企业微信 file/audio/video、飞书 image_key/file_key 等原生上传能力不计入已完成范围。

验收：

- `go test ./...` 通过。
- `cd web && npm run build` 通过。
- 后台创建/编辑 Sink 时能看到该渠道能力说明。
- 文档明确列出各渠道媒体支持边界和降级策略。

### V3-2 Telegram 媒体标准化与图片下载

目标：Telegram 图片必须能被识别、下载，并作为标准媒体进入投递 payload。

- [x] 扩展 `domain/message.Media`：
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
- [x] 扩展 Telegram normalizer：
  - 图片消息识别为 `photo`
  - document 图片识别为 image/document
  - album/grouped_id 保留
  - caption/text 语义明确
  - 原始链接 `original_url` 保留
- [x] 新增 Telegram 媒体下载服务：
  - 按 source/account 复用已登录 client 或注入下载能力。
  - 限制最大文件大小。
  - 支持图片优先，文件后置。
  - 下载产物不进数据库大字段。
  - 临时文件路径不通过 API 泄漏给用户。
- [x] 扩展 `plugin/sink.Payload`：
  - text
  - format
  - media[]
  - fallback_text
- [x] 入队时保存处理后的 message snapshot，包含媒体元数据但不包含过大二进制。
- [x] 对不支持图片的 Sink 生成清晰降级文本，例如：
  - `[图片消息] <caption>`
  - 原始 Telegram URL
  - 文件名/大小摘要

验收：

- Telegram 图片消息进入 `messages.media`。
- 至少有单测覆盖 photo/document image normalize。
- 图片下载失败不会导致整个 source 崩溃，应记录错误并按降级策略投递。
- 不支持图片的渠道能收到可读文本。

进度说明（2026-07-03）：V3-2 已完成并验证 `go test ./...`、`go vet ./...`、`go build ./...`。Telegram photo 与 document image 已标准化为媒体元数据并走现有 client 下载图片；下载失败只写入媒体状态并继续进入投递降级链路，二进制不写入数据库。

### V3-3 现有 Sink 媒体投递改造

目标：先让现有渠道按自身能力正确处理图片，不支持则明确降级。

- [x] Webhook：
  - 默认仍可保持 text JSON。
  - 可选支持 media metadata 或 URL 字段。
  - 不默认发送二进制。
- [x] 企业微信群机器人：
  - 按官方能力补图片发送路径。
  - 如需要 md5/base64 或上传流程，封装在 Sink 内。
  - 文本 + 图片组合要有明确顺序策略。
- [x] 企业微信应用消息：
  - 按官方能力补图片上传与发送路径。
  - access_token 缓存沿用现有实现。
  - 文件、音频、视频上传发送路径后置为复杂媒体阶段。
- [x] 钉钉自定义机器人：
  - 明确 markdown 图片链接、文本、文件能力边界。
  - 不支持本地图片直传时，必须给用户看见限制和降级。
- [x] Sink 测试接口支持媒体测试或至少展示“此渠道当前测试仅覆盖文本”。

验收：

- 支持图片的现有 Sink 能发出 Telegram 图片。
- 不支持图片的 Sink 不假装成功发送图片，必须降级或返回可解释错误。
- 每个改动的 Sink 有单测覆盖成功/失败/降级。

进度说明（2026-07-03）：V3-3 已完成并验证 `go test ./...`、`go vet ./...`、`go build ./...`、`cd web && npm run build`。Webhook 只外发媒体公开元数据；企业微信群机器人使用 base64/md5 发送本地图片；企业微信应用消息上传临时素材后按 `media_id` 发送图片；钉钉对公网图片使用 markdown 图片链接，本地图片降级为可读文本。真实 Telegram、企业微信、钉钉外部联调需凭证，未纳入本轮自动验证。

校准说明（2026-07-04）：本阶段验收口径是“图片能投递或可解释降级”。企业微信群机器人的文件上传、企业微信应用消息的文件/音频/视频上传发送、Telegram 非图片媒体下载均未纳入本阶段完成项。

### V3-3.5 媒体存储与公网 URL（补充项）

目标：让只认公网图片 URL 的渠道（钉钉、Bark、Gotify 等）也能收到 Telegram 图片，而不是降级为文本。

- [x] 新增 `internal/infra/mediastore`：`Store` 接口 + `Local`（本地目录 + HMAC 签名 URL）+ `S3`（S3 兼容对象存储，minio-go，覆盖 AWS S3 / Cloudflare R2 / MinIO / OSS / COS）。
- [x] ingest 落库前把 Source 下载到临时目录的媒体收编进媒体目录（`media.dir`，默认 `data/media`），`LocalPath` 指向存储层，替代散落的 `os.TempDir()`。
- [x] worker 投递时按 `StorageKey` 生成公网 URL 回填 `Media.URL`（投递时生成而非落库时，保证重试拿到未过期 URL）；降级文本同步带上该 URL。
- [x] 新增 `GET /media/*key` 端点：不走管理鉴权，用 URL 内嵌 HMAC-SHA256 签名 + 过期时间校验（密钥复用 `security.encryption_key`），配置 `media.public_base_url` 后启用。
- [x] S3 启用时公网 URL 优先用对象存储（`public_base_url` 直拼或预签名），本地目录仍作二进制缓存供企业微信 base64 / 邮件附件等直传渠道使用。
- [x] 本地媒体按 `media.retention`（默认 168h）由后台每小时清理，服务启动时先清一次。
- [x] S3 侧清理：`media.s3.auto_cleanup=true` 时由本服务按同一 `media.retention` 定时删除过期对象（共用桶必须设置 `key_prefix`）；默认关闭，交由桶生命周期规则。
- [x] 系统设置框架：复用 00001 预留的 `settings` 表（补 `secret_encrypted` 列），`internal/app/settings` 提供按组读写；媒体存储作为第一组入驻，页面保存的设置优先于配置文件，S3 secret_key 加密存储、API 脱敏（`has_s3_secret`）。
- [x] 媒体设置页面化：设置页新增「媒体存储」卡片（公网地址 / 保留时长 / S3 全量参数 / S3 连接测试），保存后通过 `mediastore.Manager` 热重载，无需重启服务。

进度说明（2026-07-04）：V3-3.5 已完成并验证 `go build ./...`、`go vet ./...`、`go test ./...`、`cd web && npm run build`。飞书等渠道的原生上传路径（image_key）仍按原计划后置为独立插件改造，不在本项范围。

### V3-4 同账号多监听源连接复用

目标：Telegram Source 从“每 source 一个 client”改为“每 account 一个 runner，多 source 订阅分发”。

- [x] 新增 account runner 概念：
  - 每个 active account 最多一个 Telegram client/update loop。
  - runner 内维护 source 订阅表。
  - source 启停只增删订阅，不重复建 client。
- [x] 更新 `app/source.Manager`：
  - 按 account 分组启动。
  - 支持动态启停 source。
  - StopAll 能正确取消所有 account runner。
- [x] 更新 Telegram plugin：
  - update 到来后根据 peer 匹配多个 source。
  - 一个消息只投递给对应 source。
  - 账号掉线、未授权、Flood Wait 等错误要进入可观测状态。
- [x] 为后续媒体下载复用同一个 account client 留接口。

验收：

- 同一账号 3 个 source 只启动一个 Telegram client。
- 启停单个 source 不影响同账号其它 source。
- `go test ./...` 通过。

进度说明（2026-07-03）：V3-4 已完成。Telegram Source 插件内部改为 `account_id -> runner`，同账号 source 作为订阅注册到同一个 update loop；停止单个 source 只删除订阅，最后一个订阅停止时才取消 runner。媒体下载复用 runner 持有的 Telegram client。无真实 Telegram 凭证时通过 runner 订阅/分发单测验证核心语义。

### V3-5 规则能力扩展

目标：把规则从基础关键词过滤扩展为常用自动化能力，并保持 UI 表单化。

- [x] 新增 Condition：
  - `source`
  - `sender`
  - `time_window`
  - `has_media`
  - `media_type`
  - `message_length`
- [x] 新增 Processor：
  - `preserve_links`
  - `media_fallback_text`
  - `mask_sensitive`
  - `dedupe`
  - `quiet_hours`
  - `batch_digest`
- [x] 扩展规则 UI：
  - 所有新 condition/processor 都通过 descriptor 渲染表单。
  - 不退回 JSON 编辑。
- [x] 新增规则预演能力：
  - 选择或构造一条消息。
  - 返回命中的规则。
  - 返回处理后的文本/媒体摘要。
  - 返回将投递到哪些 Sink。
- [x] 规则错误要可解释：
  - 正则错误
  - 模板不兼容
  - Sink 不支持媒体
  - 静默时间命中

验收：

- 每个 condition/processor 有单测。
- 规则 UI 能配置所有新增项。
- 规则预演至少支持用手工样例消息测试。

进度说明（2026-07-03）：V3-5 已完成并验证 `go test ./...`、`go vet ./...`、`go build ./...`、`cd web && npm run build`。新增 source/sender/time_window/has_media/media_type/message_length 条件和 preserve_links/media_fallback_text/mask_sensitive/dedupe/quiet_hours/batch_digest 处理器；规则编辑器继续基于 descriptor 表单渲染，并增加手工样例消息预演。`quiet_hours` 与 `batch_digest` 在当前阶段提供可解释标记/摘要格式化，不引入延迟投递或跨消息聚合队列。

### V3-6 插件扩展

目标：按“一个插件一个完整闭环”的方式扩展，不只补发送函数。

推荐顺序：

1. 飞书 Sink（已完成自定义机器人 webhook）
2. 邮件 Sink（已完成 SMTP）
3. ntfy Sink（已完成 HTTP publish）
4. Bark Sink（已完成 `/push` JSON）
5. Gotify Sink（已完成 `POST /message` JSON）
6. RSS Source（已完成 RSS/Atom 轮询）
7. Webhook Source（已完成 token 鉴权接收入口）

每个 Sink 必须包含：

- [x] 飞书 Sink 插件实现。
- [x] 飞书 Sink capability 声明。
- [x] 飞书 Sink descriptor 表单元数据。
- [x] 飞书 Sink 配置校验。
- [x] 飞书 Sink 连通性测试。
- [x] 飞书 Sink 单测。
- [x] 飞书 Sink 文档矩阵更新。
- [x] 邮件 Sink 插件实现。
- [x] 邮件 Sink capability 声明。
- [x] 邮件 Sink descriptor 表单元数据。
- [x] 邮件 Sink 配置校验。
- [x] 邮件 Sink 连通性测试。
- [x] 邮件 Sink 单测。
- [x] 邮件 Sink 文档矩阵更新。
- [x] ntfy Sink 插件实现。
- [x] ntfy Sink capability 声明。
- [x] ntfy Sink descriptor 表单元数据。
- [x] ntfy Sink 配置校验。
- [x] ntfy Sink 连通性测试。
- [x] ntfy Sink 单测。
- [x] ntfy Sink 文档矩阵更新。
- [x] Bark Sink 插件实现。
- [x] Bark Sink capability 声明。
- [x] Bark Sink descriptor 表单元数据。
- [x] Bark Sink 配置校验。
- [x] Bark Sink 连通性测试。
- [x] Bark Sink 单测。
- [x] Bark Sink 文档矩阵更新。
- [x] Gotify Sink 插件实现。
- [x] Gotify Sink capability 声明。
- [x] Gotify Sink descriptor 表单元数据。
- [x] Gotify Sink 配置校验。
- [x] Gotify Sink 连通性测试。
- [x] Gotify Sink 单测。
- [x] Gotify Sink 文档矩阵更新。

每个 Source 必须包含：

- [x] RSS Source 插件实现。
- [x] RSS Source 标准消息 normalize。
- [x] RSS Source 配置校验。
- [x] RSS Source 与 ingest 接线。
- [x] RSS Source 单测。
- [x] RSS Source UI 配置入口。
- [x] Webhook Source 插件实现。
- [x] Webhook Source 标准消息 normalize。
- [x] Webhook Source 配置校验。
- [x] Webhook Source 与 ingest 接线。
- [x] Webhook Source 单测。
- [x] Webhook Source UI 配置入口。

验收：

- 每个插件独立 commit。
- 每个 commit 保持 `go test ./...` 通过。
- 改前端则 `cd web && npm run build` 通过。

### V3-7 可观测性补强

目标：让用户能解释“为什么没收到、发到哪了、为什么失败、下一次什么时候重试”。

- [x] Delivery 详情页：
  - task 基本信息
  - message snapshot
  - rule/sink/template
  - attempts 列表
  - 每次 attempt 的 started_at/finished_at/status/error/response_summary
- [x] Delivery 列表增强：
  - 支持 rule/source/sink 过滤。
  - 支持 dead/retrying 快速筛选。
  - 支持批量重试 dead。
- [x] Source 运行状态：
  - account runner 状态
  - 最近消息时间
  - 最近错误
  - 当前订阅 source 数
- [x] Sink 观测：
  - 最近测试结果
  - 最近投递失败原因
  - 成功率统计
- [x] Dashboard：
  - 改成基于时间窗口统计，而不是只看最近 200 条。
  - 展示失败 Top sink/rule/source。
  - 展示 worker/queue 状态。

验收：

- 排查一条失败投递不需要查数据库。
- Dashboard 能看到近 24 小时投递概况。
- Source 掉线/未授权能在 UI 中看到。

进度说明（2026-07-03）：Delivery、Source 运行状态、Sink 观测与 Dashboard 时间窗口统计已完成并验证 `go test ./...`、`go vet ./...`、`go build ./...`、`cd web && npm run build`。Delivery 列表支持 status/rule/source/sink/时间窗口过滤，支持批量重试 dead；详情弹窗展示 message snapshot 和 attempts。Source 列表展示 runner 状态、订阅数、最近消息时间与最近错误。Sink 列表展示最近测试结果、近 24 小时成功率和最近投递失败原因。Dashboard 展示近 24 小时投递状态、队列状态和失败 Top sink/rule/source。

## 3. 历史补拉待办

历史补拉先记录，不进入 v3 第一批目标模式推进。

后续建议单独开 V3-History：

- [ ] 对齐 Source plugin 能力声明：在真正提供历史接口前，不应只靠 `SupportsHistory=true` 暗示功能可用。
- [ ] 基于 `sources.last_message_id` 设计游标。
- [ ] 手动回捞最近 N 条。
- [ ] 启动时补漏。
- [ ] 断线恢复后增量追平。
- [ ] 避免历史回捞重复触发已投递消息。
- [ ] UI 提供“回捞预览”和“确认执行”。

## 3.1 复杂媒体待办

复杂媒体不进入 v3 第一轮图片主线，后续建议单独开 V3-Media-Advanced：

- [ ] Telegram document/file/audio/video 下载与大小限制。
- [ ] 企业微信群机器人文件上传与 `media_id` 发送。
- [ ] 企业微信应用消息 file/audio/video 上传与对应 msgtype 发送。
- [ ] 飞书应用消息插件，支持 image_key/file_key 上传路径；当前 `feishu_bot` 自定义机器人仍以文本/post 和降级为主。
- [ ] UI capability 区分“当前已实现”和“渠道官方可支持但本项目后置”。
- [ ] 为每类复杂媒体补成功、超限、下载失败、上传失败、降级单测。

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
