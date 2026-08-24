# #53 计划：举报闭环遗留收尾——被举报人触达 + 举报频控

> 状态：**已升格设计文档 + 已评审（有条件通过，0 阻塞）**。落地基线 = `docs/plans/issue-53-report-followup-设计文档.md`（§10 已回填采纳记录）+ `docs/plans/issue-53-report-followup-设计文档-评审意见书.md`。
> 生成：2026-08-24 · 实现会话（Claude Code）。
> 对应：GitHub issue #53（地图 Roadmap #4）；前置 #33 决策记录 `docs/handoffs/grilling-decisions/issue-33-report-moderation-decisions.md`（D33-2）。
> 协同：并行 #43（CI/服务器）、#54（content 标签）与本票零文件重叠；worktree 隔离。

---

## 1. 背景与目标

#33 举报闭环已上线，地图待办登记两项遗留（issue #53）：

1. **被举报人触达**：`report_result` 目前只发举报人（D33-2 定案）；被举报人收不到「内容被处理」通知。
2. **举报频控**：MVP 未做（已知缺口 F7）；当前仅 `uq_reports_pending` 天然限「同举报者同目标 pending 期一条」，无全局频控。

**目标**：治理闭环**完整触达 + 有频控**。

**性质**：补发被举报人 = **策略变更**（推翻 D33-2 部分定案），须按 SOP §3 先 grilling 定案（已完成，见 §3）。

## 2. 现状（代码真值，2026-08-24 探明）

- 生产：SSH 2222，HEAD=`0301601`，api/postgres healthy，healthz ok。
- notify 域（#32）：`CreateNotification(ctx, CreateNotificationCmd)` 单接收人单行 INSERT（`backend/notify/service.go:59,96-110`）；Notification 字段 `RecipientID/Type/ActorID*/ActorName*/ActorAvatar*/TargetType*/TargetID*/TargetTitle*`（`backend/notify/model.go:8-21`），**无 content 字段、文案前端按 type 拼**；type 白名单 5 类含 `report_result`（DB CHECK `0003_notify_schema.sql:7` + 应用层 `notify/service.go:80-82`）。
- moderation 域（#33）：`notifyReportResult`（`moderation/service.go:354-367`）只发 `r.ReporterID`、无 actor、`TargetTitle=conclusionFor(action)`（"内容已删除"/"已警告违规用户"/"未采取处理"，:177-186）。**通知在 service 层发，不在 handler**。
- Report 模型**无被举报人 user id 字段**（`moderation/model.go:8-21`）；post/comment 作者经 `ContentGateway.ResolveTarget` 现查（`moderation_adapter.go:26-43`）；**delete 路径故意不 ResolveTarget**（幂等 O1-②，内容已删查不到）→ 必须 CreateReport 时快照作者 id。
- HandleReport O1 序列（`moderation/service.go:275-351`）：角色/动作门 → 备注校验 → 取举报 → 一致性 → 事务外内容副作用 → 单事务（审计+改状态）→ **提交后**发通知（失败打日志不回滚，S3）。
- 频控现状：**无任何时间窗口逻辑**；仅 `uq_reports_pending` 部分唯一索引 + `PendingExists` 预检（`0004_moderation_schema.sql:23`、`service.go:254`）。`reports.created_at` 可用于窗口，`idx_reports_reporter` 索引已覆盖。
- 迁移 0001-0006（0007/0008 可用）。迁移器 `migrate.go` 按 ASCII `;` 拆语句 → **迁移注释禁半角分号**、DO 块不可用。
- 约束名已核实（生产库 pg_constraint）：`notify.notifications` type CHECK = **`notifications_type_check`**。

## 3. 用户定案（grilling 2026-08-24，AskUserQuestion）

| # | 问题 | 定案 |
|---|---|---|
| Q1 | 通知对象 | **举报人 + 被举报人都发**（保留 `report_result` 给举报人，补发 `report_handled` 给被举报人，各自视角） |
| Q2 | 被举报人收哪些动作 | **delete_post/delete_comment/warn**；**dismiss 不通知被举报人**（举报人照旧收结论）。顺带反转 O4（warn 现在通知） |
| Q3 | 频控规则 | **全局窗口**：同一举报人 10 分钟 ≤5 次（跨目标） |
| Q4 | 超限处理 | **HTTP 429 + 提示，不落库** |

**MVP 补充决策**（实现时写入决策记录）：被举报人通知只带原因枚举、不拼 `reporter_note`（隐私 + `target_title` VARCHAR(255) 逼紧）；`report_handled` 不发 actor（头像「?」，与现有 report_result 一致）；存量 `target_author_id` NULL 跳过 + slog.Warn；频控软限（竞态超发可接受，硬不变量仍是 uq_reports_pending）。

## 4. 实现方案

### 4.1 被举报人触达——新通知类型 `report_handled`

**为什么新类型**：前端 `report_result` 模板硬编码「举报结果：${target_title}」（举报人视角），复用会渲染错误。

| # | 文件 | 改动 |
|---|---|---|
| M1 | `backend/migrations/0007_reports_target_author.sql`（新） | `ALTER TABLE moderation.reports ADD COLUMN target_author_id BIGINT;` |
| M2 | `backend/migrations/0008_notify_type_report_handled.sql`（新） | `DROP CONSTRAINT IF EXISTS notifications_type_check` + `ADD CONSTRAINT notifications_type_check CHECK (type IN ('follow','like','comment','reply','report_result','report_handled'))`（`report_handled`=14 字符，VARCHAR(16) 可装；注释禁 `;`） |
| C1 | `backend/moderation/model.go` | `Report` 加 `TargetAuthorID *int64`（不进 ReportView JSON） |
| C2 | `backend/moderation/service.go` | CreateReport（:225-273）：post/comment 快照 `ref.AuthorID`、user 快照 `&in.TargetID` 进 `rep.TargetAuthorID` |
| C3 | `backend/moderation/service.go` | `notifyReportResult`（:354-367）→ 改名 `notifyReportOutcome`：举报人 `report_result`（不变）；`notifiesReportedUser(action)`（delete/warn）且 `TargetAuthorID != nil` 时补发 `report_handled`（RecipientID=作者、TargetTitle=`handledConclusionFor(action, r.Reason)`、无 actor）；nil → slog.Warn。两条各自吞错不回滚（S3） |
| C4 | `backend/moderation/service.go` | 新增 helper：`notifiesReportedUser(action)`、`handledConclusionFor(action, reason)`（delete→「你的内容因「{reason}」被举报，已删除」；warn→「你因「{reason}」被举报，已警告」） |
| N1 | `backend/notify/service.go` | `validTypes`（:80-82）加 `"report_handled": true`；顺带更新 Type/doc 注释 + `notify/README.md:27` 白名单 |
| — | `backend/internal/httpapi/moderation_adapter.go` | **无需改**（NotificationCmd 字段够用，Type 透传） |

### 4.2 举报频控——全局窗口，不落库

| # | 文件 | 改动 |
|---|---|---|
| R1 | `backend/moderation/repo.go` | `Repo` 加 `CountReportsSince(ctx, reporterID int64, since time.Time) (int64, error)` |
| R2 | `backend/moderation/gorm_repo.go` | `WHERE reporter_id=? AND created_at >= ?` COUNT（索引已覆盖） |
| R3 | `backend/moderation/service.go` | 哨兵 `ErrRateLimited`；常量 `reportWindow=10*time.Minute`、`reportLimit=5`；CreateReport 检查顺序：**存在性+自举报 → PendingExists(409) → 频控(429) → 落库** |
| R4 | `backend/internal/httpapi/handler/moderation.go` | `respondModerationError`（:28-39）加 `ErrRateLimited → 429`，文案「举报过于频繁，请稍后再试」 |

### 4.3 前端（微调）

| # | 文件 | 改动 |
|---|---|---|
| F1 | `frontend/src/api/types.ts`（:124） | `NotificationType` 联合加 `'report_handled'` |
| F2 | `frontend/src/views/Notifications.vue` | `textFor()` 加 `case 'report_handled': return n.target_title || '你的内容已被处理'`；`linkTo()` 显式加 `case 'report_handled': return null` |
| F3 | `frontend/src/views/Reports.vue`（:42） | 警告按钮文案 `'警告（仅留痕，对方不会收到通知）'` → `'警告（会通知被举报人）'`；:35 注释同步 |

**ReportSheet / client.ts 零改动**：429 body `{"error":"举报过于频繁，请稍后再试"}` 经 `client.ts` 抛 `ApiError(429, msg)`，ReportSheet 已 toast `.message`。

## 5. 测试

**必改（否则红）**
- `backend/moderation/service_test.go`：`TestHandleReportDeletePost`(:176)、`TestHandleReportWarnAttribsAuthor`(:193) 断言 `len(notify.calls)==1` → 改 2 并断言 `calls[1]`（report_handled→100 + 结论句）；`TestHandleReportDismiss`(:151) 保持 1 条并断言 `Type=="report_result"`。
- `frontend/src/views/Reports.test.ts`(:65,72)：断言新文案。

**新增**
- moderation service：delete_comment 通知作者；warn user 目标（report_handled→100「你因「垃圾广告」被举报，已警告」）；存量 nil 跳过（仅 1 条）；频控三件套（第 6 次 `ErrRateLimited` / 第 5 次放行 / 窗口外放行——fake 手工改 `CreatedAt`）。
- notify service：`report_handled` 合法 type 写入成功，bogus 仍拒。
- handler 集成（真 PG，`backend/internal/httpapi/handler/moderation_test.go`）：`TestReportedUserNotification`（dismiss→作者 0 条；delete_post→作者 1 条；warn→作者 2 条最新文案）；`TestReportRateLimit`（5 连报全 201、第 6 次 429、reports 计数 5 不落库）；迁移 0007/0008 在 `testutil.SetupPG` 自动跑。
- 前端：`Notifications.test.ts` 补 `report_handled` 渲染；`ReportSheet.test.ts` 补 429 透出文案。

## 6. 风险与已知边界

1. 迁移注释禁半角 `;`（migrate.go 按分号拆）；0008 ADD 失败 → 启动 fatal（`main.go:41`），迁移事务包裹可安全重试，约束名确定性已核实。
2. 频控软限：`CountReportsSince` + INSERT 非原子，并发可超发 1-2 条（MVP 可接受；硬不变量 uq_reports_pending）。
3. 存量 pending 报告 `target_author_id` NULL → 跳过被举报人通知 + slog.Warn（不回填，0006 先例纯 additive）。
4. 「其他」原因 → 通知文案「你的内容因「其他」被举报，已删除」信息量低，但为隐私不拼 reporter_note（产品决策点，MVP 只带枚举）。
5. `report_handled` 无 actor → 头像「?」（与现有 report_result 一致）。
6. 作者账号已注销 → notify 无 FK、孤儿行无害（与现状一致）。

## 7. 验证与协作

- 本地 `cd backend && go build ./... && go vet ./... && go test ./...` + `cd frontend && npm run test:unit && npm run build` 全绿。
- 分支 `feat/report-followup`（worktree `../ai-forum-53`）→ commit → push → `MSYS_NO_PATHCONV=1 gh pr create` → **用户授权** → merge → CI 部署（迁移自动跑）→ 服务器验证。
- 决策记录 `docs/handoffs/grilling-decisions/issue-53-report-followup-decisions.md` 随 PR 入库；地图 issue #1 只由统筹方更新。
