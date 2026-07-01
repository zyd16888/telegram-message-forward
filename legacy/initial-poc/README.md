# Initial PoC Archive

这里归档的是项目早期 PoC 实现，仅作为后续重建新架构时的参考。

归档内容包括：

- 旧 `gotgproto` Telegram 监听入口。
- 旧 GORM + SQLite 数据库初始化逻辑。
- 旧插件管理雏形。
- 旧企业微信文本转发实现。
- 旧消息调试样例。
- 本地配置和数据库文件。

后续主线开发不应继续在此目录内扩展功能。新代码应按 `docs/product-architecture-v1.md` 中的目录规划重建：

- Go 1.26。
- `gotd/td` Telegram Source。
- PostgreSQL。
- GORM repository。
- goose SQL migration。
- Source/Sink 编译期注册表。
- 规则引擎和投递队列分层。

本目录中的 `config.json`、`saveData.db` 等本地敏感或环境文件仍应保持不提交。
