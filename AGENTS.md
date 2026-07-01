# AGENTS.md

本文件是本项目的 AI 协作规范，适用于 Codex、Claude 和其他参与开发的 AI 代理。

## 1. 协作原则

- 默认使用简体中文沟通，代码标识符、命令、日志和错误信息保持原文。
- 先理解需求和现有架构，再给方案；方向确认后再实施。
- 尊重事实优先。如果发现需求、设计或实现存在问题，需要直接指出并说明原因。
- 遵循 KISS、YAGNI、DRY、SOLID，避免过早抽象和无用预留。
- 优先做窄边界、可验证、可回滚的改动。
- 不泄露、不打印、不提交敏感信息，例如 Telegram session、手机号、企业微信 secret、Webhook token、数据库密码。

## 2. 需求与方案流程

- 收到非 trivial 需求后，先梳理目标、边界、风险和方案。
- 涉及架构、数据模型、插件接口、迁移策略、部署方式时，先请求用户确认方向。
- 方向确认后，将任务拆成 TODO，并按阶段推进。
- 如果需求不明确，先提出必要问题；不要凭空扩大范围。
- 已定稿的产品和架构以 `docs/product-architecture-v1.md` 为主要依据。

## 3. 代码修改规范

- 按功能边界修改代码，不把无关重构、格式化、依赖升级混入同一次改动。
- 每个变更应保持单一目标，例如“新增 Telegram Source 适配层”“新增 WeCom Sink”“修复投递重试”。
- 不直接把第三方库类型扩散到核心领域层：
  - `gotd/td` 只应出现在 Telegram 适配层。
  - GORM model 只应出现在存储层。
  - API DTO 不应直接复用数据库 model。
- 不在 handler 中直接访问数据库，统一通过 app service 和 repository。
- 不使用 GORM `AutoMigrate` 作为正式 schema 迁移机制。
- 正式数据库结构变更必须通过 `migrations/` 下的 SQL migration 管理。
- 不把测试 seed 写进正式 migration。

## 4. 验证要求

- 只改文档、注释或非代码配置时，可不运行编译和构建，但需要做静态检查。
- 修改 Go 代码后，必须确保编译成功。
- 修改前端代码后，必须确保前端构建成功。
- 同时修改后端和前端时，两边都需要验证。
- 如果因为缺少依赖、外部服务、密钥或用户明确限制导致无法验证，必须在最终回复中说明未验证项和原因。
- 不要用“看起来没问题”代替编译、构建或必要的静态检查。

建议验证命令按实际项目阶段补齐，例如：

```bash
go test ./...
go build ./...
npm run build
```

## 5. Git 与提交规范

- 提交代码必须按功能边界拆分，一个 commit 只做一个明确功能或修复, 代码修改完成后，用户如果没有明确说不提交，就默认提交。
- 不提交无关文件、个人 IDE 配置、本地数据库、session、日志、密钥或临时文件。
- 提交前必须查看 `git status`，确认只包含本次需求相关文件。
- 工作区存在用户已有改动时，不要覆盖、回滚或顺手整理，除非用户明确要求。
- 如果用户要求提交，优先使用中文 commit message，格式建议：

```text
feat: 新增 Telegram Source 适配层
fix: 修复投递任务重试状态更新
docs: 补充 AI 协作规范
refactor: 拆分规则引擎目录结构
```

## 6. 项目当前技术方向

- 后端：Go 1.26。
- HTTP API：Gin。
- Telegram MTProto：直接使用 `gotd/td`，不继续基于 `gotgproto` 扩展。
- 数据库：PostgreSQL。
- 数据访问：GORM。
- 数据库迁移：goose SQL migration。
- 队列：PostgreSQL 表驱动的 `delivery_tasks`。
- 插件机制：编译期注册表。
- 前端：Vue 3 + TypeScript + Vite + Naive UI 或 Element Plus。

## 7. 目录边界

- `cmd/`：进程入口和启动编排。
- `internal/domain/`：核心领域模型和接口，不依赖 Gin、GORM、gotd/td 或具体 Sink。
- `internal/app/`：应用服务和用例编排。
- `internal/storage/`：数据库连接、GORM model、repository、事务封装。
- `internal/infra/`：外部依赖适配，例如 Telegram、HTTP client、加密、日志。
- `internal/plugin/`：Source/Sink 插件注册表和内置插件。
- `internal/ruleengine/`：规则匹配、条件、处理器。
- `internal/dispatch/`：投递队列、worker、重试。
- `internal/api/`：HTTP router、middleware、DTO、handler。
- `migrations/`：独立 SQL migration。
- `web/`：前端工程。

## 8. 敏感信息与安全

- 配置示例只能使用占位符。
- 日志和错误响应必须避免输出 secret、token、session、手机号等敏感信息。
- 管理 API 必须有最小鉴权。
- UI 返回敏感配置时默认脱敏。
- 本地 `config.*`、数据库文件、session 文件和 `.env` 不应提交。
