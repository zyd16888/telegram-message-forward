# 聊天归档与导出（Chat Archive）执行清单

> 创建日期：2026-08-23
> 口径来源：产品 review 后用户确认的迭代范围。
> 本文件是**执行跟踪清单**，完成一项勾一项；每个任务按功能边界单独 commit。

## 0. 需求背景

用户有长期私人对话，Telegram 自带搜索不好用，需要把聊天记录取出来做分析。

因此本产品线要解决的是**「归档 + 检索 + 导出」**，不是单纯一个导出按钮。

## 1. 硬性约束

- **第三条独立旁路**：与 Flow 实时转发、AI 整理并列，三线互不合并。
- **不进 Flow、不产生 delivery、不写 `messages` 表**。理由：
  1. 一次导出几万条走 `ingest.Ingest` 会给所有下游渠道刷屏；
  2. `messages` 受 `messages_retention_days` 保留期清理，归档数据会被删掉。
- `gotd/td` 不扩散：分页拉取只能落在 `internal/plugin/source/telegram`。
- GORM 只在 `internal/storage`；handler 不碰 DB；DTO / domain / model 分离。
- schema 变更只走 `migrations/` 下的 goose SQL。
- 导出内容是全系统最敏感数据（私人对话原文）：下载必须鉴权，落盘文件纳入保留期清理，日志绝不打正文。
- 改 Go 后 `go build ./...` + `go test ./...`；改前端后 `cd web && npm run build`。

## 2. 已确认的产品决策

| 决策点 | 结论 |
|---|---|
| 功能形态 | 归档落库 + 可搜索 + 可导出（三张表 + 搜索接口 + 多格式导出） |
| 媒体 | 默认只存元信息（类型/文件名/大小/caption），下载做成显式开关 + 单文件大小上限 |
| 历史补拉 P0 | 并入本轮，作为前置修复；与导出共用同一套分页代码 |

## 3. 前置修复（阻塞项）

现有 `internal/plugin/source/telegram/history.go` 的三个缺陷必须先修，导出层与之共用分页代码。

- [x] **F1 分页缺失导致静默丢消息**
  - 现状：`history.go:185` 只发一次 `MessagesGetHistory`，`OffsetID` 恒为 0，`Limit` 硬顶 100（`maxHistoryLimit`）。`MinID` 在 MTProto 里只是过滤下界，服务端仍从最新往回返回。
  - 后果：断线超过 100 条时只补最新 100 条，而 `internal/app/ingest/service.go:84` 把 `last_message_id` 推进到本批最大值 → 中间那段永久跳过，日志却打「历史补拉完成」。
  - 修法：抽出共用的分页迭代器（循环 `OffsetID` = 上批最小 id，直到返回空 / 越过 `MinID` / 撞硬顶），补漏路径改为按游标真正翻完。
  - 验收：构造 >100 条断档的场景，追平后无缺口；`last_message_id` 只在确实连续覆盖后推进。

- [x] **F2 手动回捞污染游标**
  - 现状：`internal/app/source/history.go:43` 的 `minID` 写死 `0`（语义为「拉最新 N 条」），但 ingest 无条件推进游标。
  - 后果：游标落后很多的源，点一次「回捞最近 50 条」就把游标推到最新，之后再也补不回中间那段。
  - 修法（已选 B）：回捞路径不推进游标。保持「拉最新 N 条」的 UI 语义不变，
    重复拉取的代价由 `messages` 的 `(source_id, external_message_id)` 唯一约束吸收。
    实现为 `NormalizedMessage.SkipCursorAdvance`，由 ingest 判定。
  - 验收：预览无副作用；确认执行后不产生新的不可恢复缺口。

- [x] **F3 补拉路径丢失发送者名字**
  - 现状：`history.go:122` / `history.go:271` 传 `tg.Entities{}`，`fillSender` 拿不到 users/chats，`sender_name` 恒为空；而 `MessagesGetHistory` 响应本就带 `Users`/`Chats`，被 `unpackHistoryMessages` 丢弃。
  - 修法：`unpackHistoryMessages` 一并返回 users/chats，构造 `tg.Entities` 后再 `Normalize`。
  - 验收：补拉消息与实时消息的 `sender_name` 一致。

**建议 commit**：`fix: Telegram 历史拉取补全分页与 entities`

## 4. 数据模型（migration `00032_chat_archive.sql`）

三张表。归档会话与导出任务分离，才能增量补拉、反复导出不同格式而不重拉。

```sql
chat_archives            -- 一个被归档的会话
  id, account_id, peer_type, peer_id, peer_name, peer_username
  min_message_id, max_message_id      -- 已覆盖区间
  message_count, media_count, last_synced_at
  created_at, updated_at
  UNIQUE (account_id, peer_type, peer_id)

chat_archive_messages    -- 归档消息（比 messages 表宽）
  id, archive_id → ON DELETE CASCADE
  message_id, grouped_id
  reply_to_message_id      -- 回复链，分析必需
  outgoing boolean         -- 方向（out 是 PG 关键字，改名规避）
  sender_peer_type, sender_id, sender_name, sender_username
  message_type, text
  entities jsonb           -- 富文本 offset（引用/链接/mention）
  media jsonb, fwd_from jsonb, reactions jsonb
  service_action text      -- 入群/改名等服务消息
  date timestamptz, edit_date timestamptz
  UNIQUE (archive_id, message_id)
  INDEX (archive_id, date)
  INDEX USING GIN (text gin_trgm_ops)

chat_export_jobs         -- 一次拉取/渲染任务
  id, archive_id → ON DELETE CASCADE
  status                   -- pending/running/succeeded/failed/cancelled
  from_date, to_date, include_media, media_max_bytes, max_messages
  fetched_count, media_count
  cursor_offset_id         -- 断点续传
  last_error, started_at, finished_at, created_at, updated_at
```

**中文检索说明**：Postgres 的 `to_tsvector` 不分中文词，全文索引对中文基本无效。
采用 `pg_trgm` GIN 索引 + 继续用 ILIKE，支持中文子串搜索且不需要分词器。
迁移需 `CREATE EXTENSION IF NOT EXISTS pg_trgm`（Supabase 支持）。

- [x] 4.1 编写 migration（含 down）
- [x] 4.2 `internal/domain/chatarchive` 领域模型与仓储接口
- [x] 4.3 `internal/storage/model` + `internal/storage/repository` 实现（幂等 upsert by `(archive_id, message_id)`）

**建议 commit**：`feat: 新增聊天归档数据模型与仓储`

## 5. 拉取层 `internal/plugin/source/telegram/export.go`

与现有 `history.go` 的关键差异：

- [ ] **真分页**：复用 F1 抽出的迭代器，循环 `MessagesGetHistory{Peer, OffsetID, Limit:100, MinID, OffsetDate}`，直到返回空 / 越过时间下界 / 撞 `max_messages` 硬顶。
- [ ] **保住 entities**：从 `res.GetUsers()/GetChats()` 构造 `tg.Entities`（复用 F3）。
- [ ] **独立的 `ExportMessage` 映射**：不改 `NormalizedMessage`，避免污染转发主链路。需覆盖 reply_to / out / fwd_from / edit_date / reactions / 服务消息。
- [ ] **限速**：复用 `infra/telegram` 已有的 floodwait + ratelimit middleware，页间再加可配 sleep。几万条全量拉必然撞 FLOOD_WAIT。
- [ ] **复用连接**：`p.runningClient(ctx, accountID)`，无 runner 时临时 `buildClient`，照抄 `fetchHistoryMessages` 现有模式。
- [ ] **媒体**：默认只写元信息；开关开启时复用 `downloadMessageMedia` + `mediastore`，受单文件大小上限约束。
- [ ] 单测：分页游标推进、时间窗边界、entities 解析、服务消息不丢。

**建议 commit**：`feat: Telegram 会话全量导出拉取层`

## 6. 应用层 `internal/app/chatarchive`

- [ ] 6.1 `CreateJob`：校验账号 active、peer 在缓存（缺失时提示先同步会话列表），落 `chat_export_jobs`
- [ ] 6.2 异步执行器：后台 goroutine 跑拉取，按页落库并写 `cursor_offset_id`（断点续传），照抄 AI 整理 run 的任务编排形态
- [ ] 6.3 进度上报与取消
- [ ] 6.4 `SearchMessages`：按 archive + 关键词 / 时间窗 / 发送者 / 方向查询
- [ ] 6.5 单测：任务状态机、取消、断点续传

**建议 commit**：`feat: 聊天归档任务编排与进度`

## 7. 渲染与下载

- [ ] **JSONL**（首选）：一行一条，喂分析脚本或 LLM
- [ ] **CSV**：date / direction / sender / text / reply_to / media_type，给 Excel、pandas
- [ ] **Markdown**：按天分节，可读，也适合直接丢给 LLM 做分析
- [ ] HTML 带媒体存档 —— **本轮不做**
- [ ] **必须流式写出**。几万条 JSONL 是几十 MB，不能用 backup 那种 `c.Data(...)` 一次性塞内存。
      用 `c.Stream`，或落 `data/exports/` 后给签名下载链接（复用 mediastore 的 HMAC 端点思路）。
- [ ] 落盘导出文件纳入保留期清理

**建议 commit**：`feat: 聊天归档导出渲染与流式下载`

## 8. API

```
POST   /api/v1/chat-exports                      建任务
GET    /api/v1/chat-exports                      任务列表
GET    /api/v1/chat-exports/:id                  进度
POST   /api/v1/chat-exports/:id/cancel
GET    /api/v1/chat-exports/:id/download?format=jsonl|csv|md
GET    /api/v1/chat-archives                     已归档会话
GET    /api/v1/chat-archives/:id
GET    /api/v1/chat-archives/:id/messages?q=&from=&to=&sender=&out=
```

- [ ] 8.1 DTO + handler（handler 只调 app service）
- [ ] 8.2 挂到 `/api/v1`（走 Auth 中间件）
- [ ] 8.3 鉴权集成测试

**建议 commit**：`feat: 聊天归档管理 API`

## 9. 前端 `web/src/pages/ChatArchivesPage.vue`

- [ ] 选账号 → 选会话（复用 sources 页的 peer 同步下拉）→ 时间窗 / 媒体开关 / 条数上限 → 开始
- [ ] 任务进度轮询（照抄 AI 整理 run 的形态）+ 取消
- [ ] 归档详情：消息列表 + 搜索框（关键词 / 时间 / 发送者 / 方向）
- [ ] 导出下载按钮（格式选择）
- [ ] 侧栏入口

**建议 commit**：`feat: 聊天归档页面`

## 10. 明确不做（本轮）

- HTML 带媒体的可浏览存档
- 归档消息接入 AI 整理 / Flow
- 增量自动定时归档（先只做手动触发）
- 跨会话全局搜索
- 导出文件加密码（如需，可复用 backup 的加密 archive 模式，后置）

## 11. 推荐实施顺序

1. **F1/F2/F3** 前置修复（导出层依赖同一套分页代码）
2. **4** 数据模型
3. **5** 拉取层
4. **6** 任务编排
5. **7** 渲染与下载
6. **8** API
7. **9** 前端

## 12. 工作量

约 4.5 人天。不含媒体下载与 HTML 的 Phase 1 可压到 ~3 天。

## 13. 验证清单

```bash
go test ./...
go vet ./...
go build ./...
cd web && npm run build
```

手动冒烟：

- [ ] 断线 >100 条后追平无缺口（F1）
- [ ] 手动回捞不产生不可恢复缺口（F2）
- [ ] 补拉消息有 `sender_name`（F3）
- [ ] 私聊全量归档可跑完，双向消息、回复链、服务消息齐全
- [ ] 中途取消后可断点续传
- [ ] 中文关键词搜索命中且走索引（`EXPLAIN` 确认）
- [ ] 三种格式均可流式下载，大归档不 OOM
- [ ] 归档过程不产生任何 delivery_task，`messages` 表无新增

## 14. 进度勾选

| ID | 状态 | 备注 |
|----|------|------|
| F1 | [x] | 正向/反向分页迭代器，服务消息计入游标 |
| F2 | [x] | B 方案：回捞不推进游标 |
| F3 | [x] | 响应 users/chats 装配为 tg.Entities |
| 4  | [x] | migration 00032；SQL 未在真实 PG 上执行验证 |
| 5  | [ ] | 拉取层 |
| 6  | [ ] | 任务编排 |
| 7  | [ ] | 渲染与下载 |
| 8  | [ ] | API |
| 9  | [ ] | 前端 |
