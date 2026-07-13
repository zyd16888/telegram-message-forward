# Telegram Message Forward

把 Telegram 频道/群组/私聊消息（以及 RSS、Webhook 输入）经 Flow 编排转发到企业微信、钉钉、飞书、Bark、ntfy、Gotify、邮件、Webhook 等下游渠道的自托管消息转发服务。

单二进制部署：Go 后端内嵌 Vue 管理后台，除 PostgreSQL 外无其他依赖。

**主链路（始终 Flow-only）**：Source → Flow → Queue → Sink。旧 Rule 引擎与 `flow_engine.mode` 切换已下线。  
**AI 整理**是独立旁路产品线（定时批处理、独立投递 origin），不进入 Flow 节点，也不与实时转发共用编排模型。

## 功能特性

- **监听源（Source）**
  - Telegram 用户账号监听（基于 [gotd/td](https://github.com/gotd/td) MTProto，非 Bot API，可监听任意已加入的频道/群组/私聊；同账号多源复用一条连接）
  - RSS / Atom 订阅轮询
  - Webhook 被动接收（每源独立 token 鉴权）
- **Flow 图引擎**：在画布中把来源、过滤器、处理器和目标渠道连成实时转发链路，一条消息可分发到多个渠道（主路径，始终启用）
- **AI 整理（旁路）**：独立配置与调度的消息整理/摘要，与 Flow 实时转发分离
- **模板渲染**：自定义转发文本，支持 Text / Markdown / HTML，管理后台可实时预览
- **目标渠道（Sink）**：企业微信群机器人、企业微信应用消息、钉钉自定义机器人、飞书自定义机器人、Bark、ntfy、Gotify、SMTP 邮件、通用 Webhook；每个渠道声明媒体能力矩阵，不支持的媒体自动降级为可读文本
- **图片/媒体转发**：Telegram 图片自动下载并统一存储，支持本地目录（带 HMAC 签名 URL 端点）和 S3 兼容对象存储（AWS S3 / Cloudflare R2 / MinIO / OSS / COS），按保留期自动清理
- **投递保障**：数据库队列 + 多 worker、失败退避重试、死信批量重试、投递记录与渠道成功率观测
- **管理后台**：Vue 3 + Naive UI。账号登录（验证码/扫码）、源/渠道/Flow/模板管理、转发编排画布、AI 整理、投递记录、系统设置（媒体存储等设置页面保存后热生效）
- **安全**：Bearer Token 鉴权、敏感字段（session、secret、密码）AES 加密落库；渠道类配置凭证可在管理端回显编辑，Telegram session / 验证码 / 2FA / 主密钥等不回显

## 快速开始

### Docker Compose（推荐）

需要一个可用的 PostgreSQL（自建或托管均可）。

```yaml
services:
  config-init:
    image: ghcr.io/zyd16888/telegram-message-forward:nightly
    command: ["-init-config", "/app/configs/config.yaml"]
    volumes:
      - ./configs:/app/configs
    restart: "no"

  app:
    image: ghcr.io/zyd16888/telegram-message-forward:nightly
    restart: unless-stopped
    depends_on:
      config-init:
        condition: service_completed_successfully
    ports:
      - "8080:8080"
    volumes:
      - ./configs:/app/configs:ro
      - ./data:/app/data        # 媒体文件持久化
```

```bash
docker compose up -d          # 首次启动 config-init 会生成 ./configs/config.yaml 并退出
vim configs/config.yaml       # 填写 database.dsn（encryption_key 已自动生成，之后不要再改）
docker compose up -d          # 再次启动即可
```

访问 `http://localhost:8080`。

更新版本时先停止旧实例，再启动新实例，避免 Telegram session 被并行使用：

```bash
docker compose pull
docker compose stop app
docker compose up -d app
```

同一 PostgreSQL 数据库只允许一个服务实例运行；新版本会通过数据库运行时锁拒绝重复实例。

### 源码运行

依赖：Go 1.26+、Node.js 20+、PostgreSQL。

```bash
# 前端构建并内嵌到二进制
cd web && npm install && npm run build:embed && cd ..

# 生成并编辑配置
go run ./cmd/server -init-config ./configs/config.yaml
vim configs/config.yaml       # 填写 database.dsn

# 启动（auto_migrate: true 时自动执行数据库迁移）
go run ./cmd/server
```

前端开发时可以 `cd web && npm run dev` 起 Vite 开发服务器，代理到后端 8080。

## 首次使用流程

1. 打开管理后台，首次访问引导创建管理员账号
2. **Telegram 配置** 页录入 App ID / App Hash（从 [my.telegram.org](https://my.telegram.org) 申请），如需代理一并配置
3. **账号** 页添加 Telegram 账号，页面上完成验证码或扫码登录
4. **监听源** 页添加要监听的频道/群组（或 RSS、Webhook 源）
5. **目标渠道** 页添加下游渠道并测试连通性
6. **转发编排（Flow）** 页把来源、过滤条件、处理器、模板和目标渠道串起来（实时转发主路径）
7. （可选）**AI 整理** 页配置旁路批处理整理（与 Flow 独立，非必须）
8. （可选）**设置** 页配置媒体存储的公网访问地址或 S3，让钉钉/Bark/Gotify 等只认公网 URL 的渠道也能收到图片

## 配置说明

配置文件只承担「连上数据库之前」的引导配置，其余配置尽量放在管理后台页面（页面保存的设置存数据库、优先于配置文件、保存后热生效）：

```yaml
server:
  addr: ":8080"

database:
  dsn: "postgres://user:pass@host:5432/telegram_forward?sslmode=require"
  auto_migrate: true

security:
  encryption_key: "由 -init-config 自动生成的 32 字节密钥"   # 存入账号 session/密钥后不可更改
  auth_enabled: true

dispatch:
  worker_count: 2

# media 段仅作为「设置」页未保存时的默认值，可整段省略
```

所有配置项都可用环境变量覆盖，前缀 `TMF_`，嵌套用下划线：如 `TMF_DATABASE_DSN`、`TMF_SECURITY_ENCRYPTION_KEY`。

> ⚠️ `security.encryption_key` 用于加密 Telegram session 和各渠道密钥，丢失或更改后已保存的敏感数据将无法解密。

## CLI 工具

管理后台可以完成全部日常操作，CLI 作为补充：

```bash
go run ./cmd/migrate -config ./configs/config.yaml up      # 手动执行数据库迁移（up/down/status/version）
go run ./cmd/token   -config ./configs/config.yaml create -name ci-token   # 生成/吊销管理 API token
go run ./cmd/login   -config ./configs/config.yaml -account-id 1           # 命令行方式登录 Telegram 账号
```

## 目标渠道媒体能力

| 渠道 | 文本 | 图片 | 说明 |
| --- | --- | --- | --- |
| 企业微信群机器人 | Text/Markdown | ✅ base64 直传 | 文本与图片分两条发送 |
| 企业微信应用消息 | Text/Markdown | ✅ 素材上传 | 支持图片/文件/音视频 |
| 钉钉自定义机器人 | Text/Markdown | ✅ 公网 URL | 需配置媒体公网地址或 S3 |
| 飞书自定义机器人 | Text/Post | 降级文本 | 自定义机器人无图片上传能力 |
| Bark / Gotify | Text/Markdown | ✅ 公网 URL | 需配置媒体公网地址或 S3 |
| ntfy | Text/Markdown | ✅ URL 或二进制直传 | |
| SMTP 邮件 | Text/HTML | ✅ MIME 附件 | |
| 通用 Webhook | JSON | 元数据/URL | 不外发二进制 |

完整矩阵与降级策略见 [docs/media-capability-matrix.md](docs/media-capability-matrix.md)。

## 项目结构

```
cmd/                 server / migrate / login / token 入口
internal/
  api/               HTTP 路由、handler、DTO
  app/               应用服务（账号、源、渠道、Flow、设置…）
  domain/            领域模型与仓储接口
  dispatch/          投递队列与 worker
  flowengine/        Flow 图编译与执行（实时转发主路径）
  ruleengine/        过滤器与处理器（供 Flow 节点复用）
  template/          模板渲染
  plugin/source/     telegram / rss / webhook 源插件
  plugin/sink/       九个内置渠道插件
  infra/             加密、媒体存储、HTTP client 等基础设施
  storage/           GORM 模型与仓储实现（PostgreSQL）
  webui/             内嵌的前端构建产物
migrations/          goose SQL 迁移
web/                 Vue 3 + TypeScript + Naive UI 前端
docs/                产品架构、路线图、媒体能力矩阵
```

## 开发

```bash
go test ./...        # 后端测试
go build ./...       # 后端编译
cd web && npm run build   # 前端构建（类型检查 + 打包）
```

协作规范见 [AGENTS.md](AGENTS.md)，架构设计见 [docs/product-architecture-v1.md](docs/product-architecture-v1.md)，开发进度见 docs/roadmap-v*.md。
