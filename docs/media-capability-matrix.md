# 媒体能力矩阵

本文件用于约束 v3 媒体投递主线：Source 进入系统后先标准化为内部媒体描述，再由 Sink capability 决定直接投递、上传后投递，或降级为可读文本。

## 设计口径

- `text` 是所有 Sink 的最低降级目标。
- `markdown` 表示该渠道有原生 Markdown 或可等价承载 Markdown 的富文本能力；如果语法不完全一致，Sink 内部负责保守转换。
- `html` 目前只有 Webhook 与 Email 作为直接承载目标，其它 IM 机器人默认降级。
- `image/file/audio/video` 分别描述能否按渠道语义发送对应媒体，不等于 UI 一定已经实现发送路径。
- `需要上传` 表示必须先调用渠道上传接口获得 `media_id`、`image_key` 或同类引用。
- `公网 URL` 表示渠道可以直接引用外部 URL；本项目不会默认泄露本地临时文件路径。
- 限制以官方文档或服务默认值为准；不同企业配置、服务端配置或私有部署可能更严格。

## 能力矩阵

| 渠道 | text | markdown | html | image/photo | file/document | audio/voice | video | 文本上限 | 文件/媒体上限 | 公网 URL | 需要上传 | 二进制直传 | 推荐降级 |
|---|---|---|---|---|---|---|---|---:|---:|---|---|---|---|
| Webhook | 支持 | 支持 | 支持 | metadata/URL | metadata/URL | metadata/URL | metadata/URL | 接收方决定 | 接收方决定 | 支持 | 否 | 默认否 | payload 带媒体 metadata，接收方自行处理 |
| 企业微信群机器人 | 支持 | 支持 | 不支持 | 支持，base64 + md5 | 支持，先上传文件 | 不支持 | 不支持 | text 约 2048，markdown 约 4096 | image 约 2 MB，file 约 20 MB | 不支持本地直链 | file 需要 | image 支持 base64 | `[图片消息]` / 文件名 + 大小 + 原始链接 |
| 企业微信应用消息 | 支持 | 支持 | 不支持 | 支持，media_id | 支持，media_id | 支持，media_id | 支持，media_id | 约 2048 | image 约 10 MB，voice 约 2 MB，video 约 10 MB，file 约 20 MB | 不作为主路径 | 是 | 是 | 对超限或上传失败媒体生成文本摘要 |
| 钉钉自定义机器人 | 支持 | 支持 | 不支持 | 仅 Markdown 公网图片 URL | 不支持直传 | 不支持 | 不支持 | 项目按 20000 保守限制 | 不适用 | 支持 | 否 | 否 | 本地图片、文件、音视频统一降级为文本摘要 |
| 飞书机器人/应用消息 | 支持 | 富文本 post | 不支持 | 支持 image_key | 应用消息支持 file_key | 应用消息支持 file_key | 应用消息支持 file_key | 按消息类型限制 | image 常见 10 MB，file 常见 30 MB | 部分卡片/富文本可引用 | 通常需要 | 是 | 自定义机器人无上传凭证时降级为文本或公网 URL |
| 邮件 | 支持 | 可转文本 | 支持 | MIME inline/attachment | MIME attachment | MIME attachment | MIME attachment | 邮件服务商决定 | 邮件服务商决定，建议默认 20 MB 内 | 支持 | 否 | 是 | 超限时只发摘要和原始链接 |
| Bark | 支持 | 不支持 | 不支持 | icon/image URL | 不支持 | 不支持 | 不支持 | 未给固定上限，受 URL/body 限制 | 不适用 | 支持 | 否 | 否 | 图片作为 URL 参数；其它媒体降级为文本 |
| ntfy | 支持 | 客户端可显示 Markdown | 不支持 | attachment/URL | attachment/URL | attachment/URL | attachment/URL | 服务端配置决定 | ntfy.sh 默认附件上限约 15 MB，私有部署可配 | 支持 | 可选 | 支持 PUT/POST | 超限时发文本摘要和原始链接 |
| Gotify | 支持 | 客户端 extras 可声明 Markdown | 不支持 | 无原生附件 | 无原生附件 | 无原生附件 | 无原生附件 | 服务端配置决定 | 不适用 | 可放入消息链接 | 否 | 否 | 所有媒体降级为文本摘要和链接 |

## 当前内置 Sink 契约

### Webhook

- 支持 `text`、`markdown`、`html`。
- v3 payload 可带 `media[]`，默认只发送元数据/URL，不主动发送二进制。
- 如果接收方需要文件流，应在自定义 Webhook 服务里根据 metadata 再拉取或请求本系统后续提供的受控下载 URL。

### 企业微信群机器人

- 当前已声明 `image` 与 `file` 能力。
- `image` 后续按 base64 + md5 发送，适合 Telegram photo 或小图片 document。
- `file` 后续按上传文件再发送 `media_id`。
- `audio`、`video` 先降级为文本摘要。

### 企业微信应用消息

- 当前已声明 `image`、`file`、`audio`、`video` 能力。
- 所有媒体都走临时素材上传，再按 `media_id` 发送。
- access_token 缓存和刷新继续由 Sink 内部维护。

### 钉钉自定义机器人

- 当前只把图片声明为“公网 URL Markdown 引用”能力。
- Telegram 下载到本地的图片不能直接发给钉钉；如果没有公网可访问 URL，必须降级。
- 文件、音频、视频不走伪成功，统一文本降级。

## 后续实现要求

- Telegram 下载产物不得直接通过 API 暴露本地路径。
- Sink 不能假装支持媒体：无法真实发送时必须降级或返回可解释错误。
- 降级文本至少包含媒体类型、caption、文件名、大小和原始 Telegram URL 中能获取到的部分。
- 每个新增 Sink 都必须同步更新本文件、插件 capability、descriptor、配置校验和测试。

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
- Bark API 文档：<https://bark.day.app/#/tutorial>
- ntfy publish 文档：<https://docs.ntfy.sh/publish/>
- ntfy 附件配置：<https://docs.ntfy.sh/config/#attachments>
- Gotify message extras：<https://gotify.net/docs/msgextras>
- Email MIME RFC 2045：<https://www.rfc-editor.org/rfc/rfc2045>
