# 媒体能力矩阵

本文件用于约束 v3 媒体投递主线：Source 进入系统后先标准化为内部媒体描述，再由 Sink capability 决定直接投递、上传后投递，或降级为可读文本。

## 设计口径

- `text` 是所有 Sink 的最低降级目标。
- `markdown` 表示该渠道有原生 Markdown 或可等价承载 Markdown 的富文本能力；如果语法不完全一致，Sink 内部负责保守转换。
- `html` 目前只有 Webhook 与 Email 作为直接承载目标，其它 IM 机器人默认降级。
- `image/file/audio/video` 在能力矩阵中描述渠道官方或协议层可承载的媒体形态；“当前内置 Sink 契约”小节才描述本项目当前实现到什么程度。
- `需要上传` 表示必须先调用渠道上传接口获得 `media_id`、`image_key` 或同类引用。
- `公网 URL` 表示渠道可以直接引用外部 URL；本项目不会默认泄露本地临时文件路径。
- 限制以官方文档或服务默认值为准；不同企业配置、服务端配置或私有部署可能更严格。
- 截至 2026-07-13，本项目媒体实现覆盖图片与文件两条主线：
  - 图片：Telegram photo / image document 始终下载（大小上限见设置页「媒体下载策略」，默认 20 MB）、收编和投递。
  - 文件：PDF 等非图片 document 在监听源开启「文件下载」开关后下载，受设置页文件大小上限（默认 50 MB）与扩展名白名单约束；下载后按 Sink 能力投递（企业微信上传 media_id、邮件附件、ntfy 附件、Webhook 元数据）或降级为带公网 URL 的文本摘要。
  - **音频、视频**：Telegram Source **不下载** audio/video 二进制；部分 Sink（邮件 MIME、ntfy 附件、企微按文件上传）在 capability 中声明「若有本地文件则可投递」，对无本地文件的音视频统一文本降级。原生音视频下载与专用投递形态属后续阶段，UI 不应理解为「已支持音视频原样转发」。
  - **历史补拉**：Telegram Source 当前 `SupportsHistory=false`（历史预览/回捞 API 尚未实现）；实现前 UI 不得展示「支持历史回捞」。

## 能力矩阵

| 渠道 | text | markdown | html | image/photo | file/document | audio/voice | video | 文本上限 | 文件/媒体上限 | 公网 URL | 需要上传 | 二进制直传 | 推荐降级 |
|---|---|---|---|---|---|---|---|---:|---:|---|---|---|---|
| Webhook | 支持 | 支持 | 支持 | metadata/URL | metadata/URL | metadata/URL | metadata/URL | 接收方决定 | 接收方决定 | 支持 | 否 | 默认否 | payload 带媒体 metadata，接收方自行处理 |
| 企业微信群机器人 | 支持 | 支持 | 不支持 | 支持，base64 + md5 | 支持，先上传文件 | 不支持 | 不支持 | text 约 2048，markdown 约 4096 | image 约 2 MB，file 约 20 MB | 不支持本地直链 | file 需要 | image 支持 base64 | `[图片消息]` / 文件名 + 大小 + 原始链接 |
| 企业微信应用消息 | 支持 | 支持 | 不支持 | 支持，临时素材 media_id | 支持，临时素材 media_id | 支持，临时素材 media_id | 支持，临时素材 media_id | 约 2048 | 临时素材 image 约 10 MB，voice 约 2 MB，video 约 10 MB，file 约 20 MB | 不作为主路径 | 是 | 是 | 对超限或上传失败媒体生成文本摘要 |
| 钉钉自定义机器人 | 支持 | 支持 | 不支持 | 仅 Markdown 公网图片 URL | 不支持直传 | 不支持 | 不支持 | 项目按 20000 保守限制 | 不适用 | 支持 | 否 | 否 | 本地图片、文件、音视频统一降级为文本摘要 |
| 飞书机器人/应用消息 | 支持 | 富文本 post | 不支持 | 支持 image_key | 应用消息支持 file_key | 应用消息支持 file_key | 应用消息支持 file_key | 按消息类型限制 | image 常见 10 MB，file 常见 30 MB | 部分卡片/富文本可引用 | 通常需要 | 是 | 自定义机器人无上传凭证时降级为文本或公网 URL |
| 邮件 | 支持 | 可转文本 | 支持 | MIME inline/attachment | MIME attachment | MIME attachment | MIME attachment | 邮件服务商决定 | 邮件服务商决定，建议默认 20 MB 内 | 支持 | 否 | 是 | 超限时只发摘要和原始链接 |
| Bark | 支持 | 不支持 | 不支持 | icon/image URL | 不支持 | 不支持 | 不支持 | 未给固定上限，受 URL/body 限制 | 不适用 | 支持 | 否 | 否 | 图片作为 URL 参数；其它媒体降级为文本 |
| ntfy | 支持 | 客户端可显示 Markdown | 不支持 | attachment/URL | attachment/URL | attachment/URL | attachment/URL | 服务端配置决定 | ntfy.sh 默认附件上限约 15 MB，私有部署可配 | 支持 | 可选 | 支持 PUT/POST | 超限时发文本摘要和原始链接 |
| Gotify | 支持 | 客户端 extras 可声明 Markdown | 不支持 | 远程 bigImageUrl | 不支持 | 不支持 | 不支持 | 服务端配置决定 | 不适用 | 支持图片 URL | 否 | 否 | 本地图片和其它媒体降级为文本摘要和链接 |

## 当前内置 Sink 契约

### Webhook

- 支持 `text`、`markdown`、`html`。
- v3 payload 可带 `media[]`，默认只发送元数据/URL，不主动发送二进制。
- 如果接收方需要文件流，应在自定义 Webhook 服务里根据 metadata 再拉取或请求本系统后续提供的受控下载 URL。

### 企业微信群机器人

- 当前运行时已实现 `image`：按 base64 + md5 发送，适合 Telegram photo 或小图片 document。
- 当前运行时已实现 `file`：有本地文件时先调用 `webhook/upload_media?type=file` 获取 `media_id`，再按 `msgtype=file` 发送（上限 20 MB）；upload 地址由 webhook send 地址推导，自定义网关地址不含 `/webhook/send` 时返回可解释错误。
- 无本地文件（未下载/超限）或超过渠道上限的文件由 worker 统一降级为文本摘要。
- `audio`、`video` 先降级为文本摘要。

### 企业微信应用消息

- 当前运行时已实现 `image`：调用上传临时素材接口 `media/upload?type=image` 获取 `media_id`，再发送图片消息；不使用面向图片 URL 的 `media/uploadimg` 接口，避免触发该接口的配额语义。
- 当前运行时已实现 `file`：先上传 `type=file` 临时素材，再按 `msgtype=file` 发送（上限 20 MB）。
- `audio`：有本地文件时按普通文件上传发送（非 voice 原生形态）；无本地文件则文本降级。Telegram 当前不下载音频，故实际多为降级路径。
- `video`：官方支持但当前内置实现未接入，capability 声明为不支持并按文本摘要降级。
- access_token 缓存和刷新继续由 Sink 内部维护。

### 钉钉自定义机器人

- 当前只把图片声明为“公网 URL Markdown 引用”能力。
- Telegram 下载到本地的图片不能直接发给钉钉；如果没有公网可访问 URL，必须降级。
- 文件、音频、视频不走伪成功，统一文本降级。

## 当前内置 Source 契约

### Telegram Source

- `SupportsSync=true`：可同步账号可见 peer 列表。
- `SupportsMedia=true`：图片始终尝试下载；非图片 document 受源级「文件下载」开关与下载策略约束。
- `SupportsHistory=true`：源级 `history_backfill_enabled`（默认 false）。关闭时仅实时 update；开启后支持手动预览/确认回捞，且 `last_message_id>0` 时启动/恢复增量补漏（新源游标为 0 不灌历史）。
- 不下载 audio/video 原生媒体；消息可带元数据进入链路并按 Sink 降级。

### RSS Source

- 支持 RSS 2.0 和 Atom 常见字段。
- 条目会标准化为内部 `NormalizedMessage`：标题与摘要进入文本，链接进入 `links/original_url`，enclosure 的 image/audio/video 进入 `media[]` 的远程 URL 描述。
- RSS Source 不下载二进制媒体，不保存正文外的大字段；是否能投递媒体继续由目标 Sink capability 决定。
- `feed_url` 必填，`poll_interval_seconds` 默认 300 秒，`max_items` 默认 20 条；重复条目通过稳定 external message id 交给消息入库幂等处理。

### Webhook Source

- 接收路径为 `POST /api/v1/sources/{id}/webhook`。
- 每个 Webhook Source 必须配置 `token`；外部请求通过 `X-TMF-Webhook-Token` header 或 `?token=` 传入，错误响应不会回显 token。
- JSON payload 支持 `message_id`、`text`、`sender/sender_name`、`timestamp`、`original_url`、`links[]`、`media[]`；非 JSON body 会按纯文本消息处理。
- `media[]` 使用内部 `domain/message.Media` 字段结构，支持远程 URL 元数据，不在 Webhook Source 内下载二进制。

## 媒体下载策略

- 下载策略在管理后台「设置 → 媒体存储 → 媒体下载策略」配置，保存后热生效；配置文件 `media.download` 段仅作页面未保存时的默认值。
- 图片上限（默认 20 MB）：photo 与 image document 超过后不下载，`download_status=skipped`，按文本摘要降级。
- 文件上限（默认 50 MB）与扩展名白名单（默认 pdf/doc/docx/xls/xlsx/ppt/pptx/csv/txt/md/epub/zip/rar/7z）：仅约束非图片 document；白名单清空表示不限类型。
- 文件下载还需要在监听源列表按 source 开启「文件下载」开关（默认关闭），避免所有频道的大文件占满磁盘。
- 未下载（开关关闭、超限、白名单不命中、下载失败）的文件保留元数据进入投递链路，降级文本包含文件名、大小和原始链接。

## 媒体存储与公网 URL

- 媒体存储推荐在管理后台「设置」页配置：保存后热生效（`mediastore.Manager` 热切换），并优先于配置文件 `media` 段；数据库无记录时回退到配置文件默认值。S3 secret_key 加密存储在 `settings` 表，API 只返回 `has_s3_secret`。
- 媒体二进制统一由 `internal/infra/mediastore` 管理：Source 下载的临时文件在 ingest 落库前收编到 `media.dir`（默认 `data/media`）。
- 配置 `media.public_base_url` 后，本地媒体可通过 `GET /media/*key` 生成带 HMAC 签名和过期时间的公网 URL；未配置时行为与之前一致（无公网 URL 的渠道继续降级）。
- 配置 `media.s3`（S3 兼容：AWS S3 / Cloudflare R2 / MinIO / OSS / COS）后媒体额外上传对象存储，公网 URL 优先使用对象存储地址（公开桶/CDN 直拼或预签名）。
- 公网 URL 在 worker 投递时按 `StorageKey` 现生成并回填 `Media.URL`，不落库，保证重试时 URL 未过期；钉钉、Bark、Gotify 等「仅公网 URL」渠道由此获得真实图片投递能力。
- 本地媒体按 `media.retention` 定期清理（服务启动时清一次，之后每小时一次）；对象存储侧开启 `media.s3.auto_cleanup` 后按同一保留期由本服务删除过期对象，未开启时请配置桶生命周期规则。与其他数据共用桶时务必设置 `media.s3.key_prefix`，auto_cleanup 只清理该前缀下的对象。

## Flow 处理器语义（当前实现）

以下 type 字符串保持兼容，**展示名与实际行为**如下（避免误解为延迟投递或跨消息聚合）：

| type | 展示名 | 实际行为 |
|---|---|---|
| `quiet_hours` | 静默时间标记 | 命中时段时在正文前追加标记；**不**抑制/延迟投递 |
| `batch_digest` | 摘要样式格式化 | 对**当前单条**消息套摘要样式；**不**跨消息聚合 |
| `dedupe` | 本条去重 | 仅去重当前消息正文行/链接；**不**做跨消息去重 |

## 后续实现要求

- Telegram 下载产物不得直接通过 API 暴露本地路径。
- Sink 不能假装支持媒体：无法真实发送时必须降级或返回可解释错误。
- 降级文本至少包含媒体类型、caption、文件名、大小和原始 Telegram URL 中能获取到的部分。
- 每个新增 Sink 都必须同步更新本文件、插件 capability、descriptor、配置校验和测试。
- UI 展示 capability 时应优先展示“当前项目已实现能力”；如果同时展示官方可支持能力，必须明确标注为后置或未接入，避免误导用户配置后期待可直接发送。
- Source `SupportsHistory` 仅在历史接口真正可用后为 true。

## 参考资料

- 企业微信群机器人配置说明：<https://developer.work.weixin.qq.com/document/path/91770>
- 企业微信发送应用消息：<https://developer.work.weixin.qq.com/document/path/90236>
- 企业微信上传临时素材：<https://developer.work.weixin.qq.com/document/path/90253>
- 钉钉自定义机器人接入：<https://open.dingtalk.com/document/orgapp/custom-robot-access>
- 飞书发送消息 API：<https://open.feishu.cn/document/server-docs/im-v1/message/create>
- 飞书上传图片 API：<https://open.feishu.cn/document/server-docs/im-v1/image/create>
- 飞书上传文件 API：<https://open.feishu.cn/document/server-docs/im-v1/file/create>
- 当前内置 `feishu_bot` 实现为自定义机器人 webhook：支持 text/post，未持有应用 token，图片/文件/音视频按文本摘要降级；后续如新增飞书应用消息插件，再启用 image_key/file_key 上传路径。
- 当前内置 `email` 实现为 SMTP Sink：支持 text/html，媒体有本地文件时作为 MIME attachment 发送；只有 URL 或下载失败时追加降级文本摘要。
- 当前内置 `ntfy` 实现为 HTTP publish Sink：远程媒体 URL 使用 `Attach` header，本地媒体作为附件请求体上传；文本支持 Markdown header。
- 当前内置 `bark` 实现为 `/push` JSON Sink：远程图片 URL 写入 `image` 字段；本地图片和其它媒体降级为文本摘要。
- 当前内置 `gotify` 实现为 `POST /message` JSON Sink：通过 `X-Gotify-Key` header 传应用 token；Markdown 使用 `client::display.contentType`，远程图片 URL 使用 `client::notification.bigImageUrl`，本地媒体降级为文本摘要。
- Bark API 文档：<https://bark.day.app/#/tutorial>
- ntfy publish 文档：<https://docs.ntfy.sh/publish/>
- ntfy 附件配置：<https://docs.ntfy.sh/config/#attachments>
- Gotify push message：<https://gotify.net/docs/pushmsg>
- Gotify message extras：<https://gotify.net/docs/msgextras>
- Email MIME RFC 2045：<https://www.rfc-editor.org/rfc/rfc2045>
