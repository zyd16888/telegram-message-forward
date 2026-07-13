# 下一冲刺任务清单（可靠 + 体验闭环）

> 更新日期：2026-07-13  
> 口径来源：产品 review 后用户确认的迭代范围。  
> 本文件是**执行跟踪清单**，完成一项勾一项；每个任务按功能边界单独 commit。

## 0. 硬性约束

- **AI 整理与 Flow 实时转发必须保持两条独立产品线**，禁止合并数据模型、禁止把 AI 塞进 Flow 节点、禁止统一成一条编排。
- UI 文案可并列展示（侧栏已有「转发编排」与「AI 整理」），但概念、API、调度、投递 origin 继续分离。
- 遵守 `AGENTS.md`：分层边界、`gotd` 不扩散、无 AutoMigrate 正式迁移、敏感信息不落日志/提交。
- 改 Go 后 `go build ./...` + 能跑则 `go test ./...`；改前端后 `cd web && npm run build`。
- **一个 commit 只做一个明确功能**；中文 commit message（`feat:` / `fix:` / `docs:` / `refactor:`）。
- 不扩大范围：本清单未列的相册整组、音视频原生投递、真实跨消息 batch 聚合、AI/Flow 统一 **不做**。

---

## 1. 范围总览

| 批次 | 主题 | 是否做 |
|------|------|--------|
| A | 可靠与诚实 | ✅ 全做（历史补拉**必须有源级开关**，默认关闭） |
| B | 体验闭环 | ✅ 全做 |
| C | 媒体高级 / AI-Flow 统一 | ❌ 不做；仅遵守「AI 与 Flow 双线」 |

---

## 2. 批次 A — 可靠与诚实

### A1. 文档与配置收敛到 Flow-only 真相

**目标**：消除 Rule / shadow 引擎残留误导。

- [x] 更新 `configs/config.example.yaml`：删除或标注废弃 `flow_engine.mode`；注释写明当前始终走 Flow 引擎。
- [x] 清理 `internal/config` 中无用默认值/注释，避免新装用户以为还要配 shadow。
- [x] 在 `README.md` 与本文件对齐：主链路描述为 Source → Flow → Queue → Sink；AI 为旁路。
- [x] 在 `docs/product-architecture-v1.md` 顶部加「现状勘误」短节（不必全文重写）：Rule 已下线、Flow-only、AI 独立。
- [x] 统一协作文档口径：配置类凭证可回显编辑（`AGENTS.md`）；session/2FA/主密钥仍不回显。

**验收**：新用户读 example 配置与 README 不会再配置 `flow_engine.mode` 期望切换旧引擎。

**建议 commit**：`docs: 收敛 Flow-only 与配置示例口径`（若含小范围 config 代码清理可拆 `refactor: 移除 flow_engine 切换残留`）

---

### A2. Capability「已实现」对齐 + 假强处理器改名/说明

**目标**：用户看到的能力 = 当前真能用的能力。

- [x] Source/Sink capability 契约区分或至少文档+UI 标明「当前实现」：
  - Telegram `SupportsHistory`：仅在历史接口真正可用后为 true；或拆 `SupportsHistory`（协议）与实现状态，**UI 只展示已实现**。
  - 音视频等未实现路径不得在 UI 标为「支持发送」。
- [x] 处理器命名与描述诚实化（二选一，优先改名+兼容旧 type）：
  - `quiet_hours` → 展示名「静默时间标记」（描述写清：不抑制投递；真实静默本冲刺不做）。
  - `batch_digest` → 展示名「摘要样式格式化」（描述写清：单条格式化，非跨消息聚合）。
  - `dedupe` → 展示名「本条去重」（描述写清：仅当前消息正文/链接）。
- [x] 若保留旧 type 字符串，descriptor label/description 必须改；Flow 画布与过滤器编辑器走 descriptor 自动刷新。
- [x] 同步 `docs/media-capability-matrix.md` 中「当前内置实现」小节。

**验收**：UI 无「支持历史回捞」而实际无入口；静默/摘要处理器文案不会让用户以为已延迟投递或合并多条。

**建议 commit**：`fix: 对齐能力声明与处理器展示语义`

---

### A3. 消息与投递记录归档

**目标**：长期运行库不无限膨胀。

- [x] 系统设置增加可热生效配置（DB settings 优先，与媒体设置同模式）：
  - `messages_retention_days`（0=不清理，默认建议 30 或 90）
  - `delivery_tasks_retention_days`（0=不清理，默认同上或略长）
  - 可选：AI run 保留天数（若表增长快可一并）
- [x] 后台定时任务（启动时一次 + 周期）：按 `received_at` / `created_at` 批量删除过期消息与终态投递任务；注意外键/级联与分批 `LIMIT`，避免长锁。
- [x] 删除消息时：关联 `delivery_tasks` / attempts 策略明确（先删任务再删消息，或依赖 FK）。
- [x] 设置页 UI：保留天数、说明「仅影响历史数据，不影响实时监听」。
- [x] 单测：边界（0 关闭、过期删除、未过期保留）。

**验收**：配置 1 天保留时，过期行被清理；0 时不清理；服务日志有清理计数。

**建议 commit**：`feat: 支持消息与投递记录按保留期归档`

---

### A4. Telegram 历史补拉 / 断线追平（源级开关，默认关）

**目标**：需要连续性的源可补漏；新闻类默认丢弃断档，不自动刷历史。

#### 产品语义

- [x] **源级开关** `history_backfill_enabled`（或 config 字段，默认 `false`）。
  - `false`：维持现状——只处理实时 update；断线/重启期间消息丢弃，**不**自动补拉。
  - `true`：启用下列补拉能力。
- [x] 可选上限：`history_backfill_limit`（单次最多条数，默认如 50/100，硬顶防止 FLOOD）。
- [x] 游标：复用/维护 `sources.last_message_id`（成功 ingest 后推进；仅实时路径与补拉成功路径更新）。

#### 能力拆分

1. [x] **手动回捞**（开关开启才显示入口）  
   - API：预览最近 N 条（不投递）+ 确认执行（走标准 ingest，幂等防重）。  
   - UI：Sources 页操作「预览 / 确认回捞」。
2. [x] **启动补漏**（仅 `history_backfill_enabled=true` 的源）  
   - runner 启动后，若 `last_message_id>0`，增量拉取 `(last_message_id, head]` 再进入实时监听。  
   - `last_message_id=0` 的新源：**不**全量灌历史，只从「开启后」的实时消息开始（可文档说明）。
3. [x] **断线恢复追平**（同开关）  
   - 连接恢复后同样按游标增量追平，再恢复 update 流。
4. [x] **幂等**  
   - 必须走 `messages` 唯一约束 + 既有 ingest；已投递消息不重复入队。
5. [x] **Capability**  
   - 实现完成后才声明历史相关能力；UI 与 A2 一致。

#### 实现注意

- 逻辑只在 `plugin/source/telegram` + app/source API；gotd 不扩散。
- FLOOD_WAIT / rate limit 必须遵守现有 middleware。
- 预览与执行权限走现有鉴权。
- 失败可观测：`runner_last_error` 或专用 last_backfill 状态字段（最小可用即可）。

**验收**：

- 默认新源开关关：重启不补历史。
- 开关开 + 有游标：启动可补到断档消息并投递一次；重复执行幂等。
- 预览不产生 delivery；确认后产生。
- 新闻类用户保持关即可。

**建议 commits（可拆）**：

1. `feat: Source 增加历史补拉开关与游标字段`
2. `feat: Telegram 历史消息拉取与 ingest 接入`
3. `feat: 历史回捞预览与确认 API`
4. `feat: 监听源页支持历史补拉开关与回捞操作`

---

## 3. 批次 B — 体验闭环

### B1. 首次使用 Checklist（Dashboard）

**目标**：新装用户知道下一步做什么。

- [x] 后端可选：`GET /api/v1/setup/status` 聚合布尔项；或前端用现有 list API 计算（优先少接口，但避免 Dashboard 过多请求——可与 B3 一并设计）。
- [x] Checklist 项：
  1. 已配置 Telegram App ID/Hash  
  2. 至少一个 active TG 账号  
  3. 至少一个启用中的监听源  
  4. 至少一个启用中的目标渠道  
  5. 至少一条启用中的 Flow  
  6. 若已选 Sink 需要公网媒体 URL，则提示配置媒体公网地址/S3  
- [x] 每项可跳转到对应页面；全部完成可折叠/收起。
- [x] **不要**做成强制 wizard 阻断使用；Dashboard 顶部卡片即可。

**建议 commit**：`feat: Dashboard 增加首次配置检查清单`

---

### B2. Flow 模板与线性默认骨架

**目标**：降低画布门槛；复杂 DAG 仍可用。

- [x] 新建 Flow 时默认生成线性骨架节点：Source 占位 →（可选 Filter）→ Target 占位，或「空 Flow + 引导」。
- [x] 提供 2～3 个一键模板（前端预置 graph 或后端 seed）：
  1. 单源 → 关键词过滤 → 单渠道  
  2. 单源 → 渲染模板 → Webhook/企微  
  3. 多源合并 → 单渠道  
- [x] 模板创建后进入编辑模式，未绑定的 source/sink 用占位提示用户选择。
- [x] 保存校验错误尽量标到节点（至少错误文案含 node id/名称）。

**建议 commit**：`feat: Flow 新建模板与线性默认骨架`

---

### B3. Dashboard 聚合 API + 失败原因可读化

**目标**：统计准确、请求少、失败可懂。

- [x] 新增聚合 API，例如 `GET /api/v1/dashboard/summary?since_hours=24`：
  - 资源计数（accounts/sources/sinks/flows）
  - 投递状态计数（pending/processing/success/retrying/dead/cancelled…）
  - 失败 Top（按 sink / flow / source）
  - 队列积压摘要
  - 可选：setup checklist 布尔
- [x] Dashboard 前端改为单次（或极少次）请求，删除「每个状态 page 一次 + 前端 250 条算 Top」。
- [x] 投递详情/列表：对媒体降级、Sink 禁用取消、配置错误等，展示**人类可读原因**（可复用 `last_error` 规范化）。
- [x] 单测：聚合计数与筛选时间窗。

**建议 commits**：

1. `feat: 新增 Dashboard 聚合统计 API`
2. `feat: Dashboard 改用聚合接口并优化失败展示`

---

### B4. HTTP/HTTPS 代理 + 投递降级原因透出

**目标**：代理可用面扩大；降级不再「静默变文本」。

#### 代理

- [x] `internal/infra/telegram/proxy.go` 实现 http/https 代理（含可选账号密码）。
- [x] 共享代理配置 UI/校验接受 `http`/`https`/`socks5`。
- [x] 单测或表驱动校验 dial 配置（可用 mock transport / 单元级构造检查）。

#### 降级透出

- [x] worker 在媒体因 capability/无公网 URL/超限/本地文件缺失而降级时，将原因写入 attempt 结果或 task 可读字段（避免只剩最终成功文本、看不出降级）。
- [x] 投递详情 UI 展示降级说明（如「渠道不支持图片且无公网 URL，已降级为文本」）。
- [x] Sink 测试连通性若依赖媒体，可提示公网 URL 未配置（最小：设置页与 Sink 能力提示联动）。

**建议 commits**：

1. `feat: Telegram 支持 HTTP/HTTPS 代理`
2. `feat: 投递媒体降级原因可观测`

---

## 4. 明确不做（本冲刺）

- 相册 `grouped_id` 整组实时合并投递  
- Telegram audio/video 下载与原生投递扩展  
- 真实「静默时段抑制/延迟投递」调度  
- 跨消息 stream 去重节点 / batch 汇聚节点  
- **AI 与 Flow 合并为一条编排**  
- 多实例水平扩展、外部 MQ  
- 多用户 RBAC  

---

## 5. 推荐实施顺序

严格建议顺序（降低联调风险）：

1. **A1** 文档配置收敛（无行为风险）  
2. **A2** 能力/文案诚实化  
3. **A3** 归档（独立，可先落地）  
4. **B3** Dashboard 聚合（给后续 UI 打底）  
5. **B1** Checklist（可吃 B3 的 summary）  
6. **B4** HTTP 代理 + 降级透出  
7. **B2** Flow 模板  
8. **A4** 历史补拉（最重，放后半；默认关不影响现网）  

若时间紧：A4 可再拆「仅开关+手动回捞」与「启动/断线自动追平」两段提交，但同一冲刺内应做完。

---

## 6. 验证清单（整冲刺结束）

```bash
go test ./...
go vet ./...
go build ./...
cd web && npm run build
```

手动冒烟：

- [ ] 新源默认不补历史；开启后手动预览/确认与启动补漏符合预期  
- [ ] 归档配置 0 / 非 0 行为正确  
- [ ] Dashboard 一次加载出统计与 checklist  
- [ ] 新建 Flow 可从模板出发并保存  
- [ ] HTTP 代理配置可保存；错误类型有明确报错  
- [ ] 无公网 URL 时钉钉类渠道投递详情可见降级原因  
- [ ] AI 整理页与 Flow 页仍完全独立可用  

---

## 7. 进度勾选（执行时更新）

| ID | 状态 | 备注 |
|----|------|------|
| A1 | [x] | Flow-only 文档与配置收敛 |
| A2 | [x] | Capability/处理器展示诚实 |
| A3 | [x] | 消息/投递按保留天数归档 |
| A4 | [x] | 默认关；开关+预览/确认+启动补漏 |
| B1 | [x] | 复用 dashboard summary setup |
| B2 | [x] | 线性骨架 + 一键模板 |
| B3 | [x] | 聚合 API + 失败可读 |
| B4 | [x] | HTTP(S) 代理 + 媒体降级透出 |
