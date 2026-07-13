# CLAUDE.md

本文件是 Claude 参与本项目时的协作规范。若本文件与 `AGENTS.md` 有重复，以 `AGENTS.md` 和用户最新指令为准。

## 工作方式

- 默认使用简体中文沟通。
- 先理解需求、现有代码和 `docs/product-architecture-v1.md`，再提出方案。
- 进度与任务跟踪见 `docs/roadmap-v1.md`：接手前先读它了解「做到哪、下一步、注意事项」，完成任务后更新其中的勾选状态。
- 涉及架构、数据模型、插件接口、目录结构或迁移策略时，先与用户确认方向。
- 方向确认后再拆 TODO 并实施。
- 不扩大需求范围，不把无关重构混进当前任务。

## 代码与架构边界

- 按功能边界修改代码。
- 新代码基于 Go 1.26。
- `gotd/td` 只放在 Telegram Source 适配层。
- GORM 只收口在 storage/repository 层。
- HTTP handler 不直接访问数据库。
- API DTO、domain object、GORM model 必须分开。
- 正式 schema 变更使用 `migrations/` 下的 goose SQL migration，不使用 GORM `AutoMigrate` 作为正式迁移。

## 验证要求

- 修改 Go 代码后，必须完成 Go 编译或测试验证。
- 修改前端代码后，必须完成前端构建验证。
- 只改文档时可不运行编译/构建，但需要静态检查文件状态和编码。
- 如果验证无法执行，需要在回复中明确说明原因。

建议命令按实际阶段使用：

```bash
go test ./...
go build ./...
npm run build
```

## Git 规范

- 按功能边界提交代码，一个 commit 只包含一个明确功能或修复。
- 提交前必须检查 `git status`。
- 不提交本地数据库、session、密钥、日志、IDE 配置或无关文件。
- 不覆盖或回滚用户已有改动，除非用户明确要求。
- commit message 默认使用中文，建议格式：

```text
feat: 新增 Telegram Source 适配层
fix: 修复投递任务重试状态更新
docs: 补充 AI 协作规范
```

## 安全要求

- 不打印、不提交 Telegram session、手机号、企业微信 secret、Webhook token、数据库密码等敏感信息。
- 配置示例使用占位符。
- 管理后台配置类渠道凭证可回显编辑；Telegram session、验证码、2FA、encryption_key、运行日志中的 token/secret 仍不得明文输出。
