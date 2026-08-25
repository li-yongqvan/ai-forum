# #53 举报闭环遗留收尾：被举报人触达 + 举报频控 — 设计文档（供评审）

> 文档用途：交付专业评审 agent 的评审对象。范围 = 背景 / 真值核对 / 决策记录 / 实现方案 / 不变量 / 验证。
> 溯源约定：每个结论标注来源。**事实**标来源（代码 `file:line` / DB 实查输出 / GitHub issue / grilling 用户确认）；**判断性裁决**单独标注【决策】并给出理由与备选，不冒充事实。
> 数据时点：2026-08-24（真值核对执行日；服务器/DB 实查同为 2026-08-24）。
> 评审状态：**有条件通过（0 阻塞）**，评审意见书 `docs/plans/issue-53-report-followup-设计文档-评审意见书.md`（2026-08-24）。F1-F6 + Q4 附带全部采纳，采纳记录见 §10。**本版为实现基线**。

## 0. 项目上下文（给零背景评审 agent，先读本节）

**这是什么**：AI 智联论坛——社团内部 AI 主题社区的移动端论坛 MVP。
- 前端：Vue 3 + TypeScript + Vite + Vant（PWA），纯静态部署（`frontend/`）。
- 后端：Go（Gin + GORM + PostgreSQL），单进程模块化单体（`backend/`）。
- 数据库：PostgreSQL 单实例、**多 schema**，每域一个：`user` / `content` / `notify` / `moderation`。

**后端架构约定（评审本设计必须理解）**：
- 4 个领域包，每包结构固定 `model.go`/`repo.go`/`gorm_repo.go`/`service.go`/`README.md`（SOP `docs/workflow.md` §5）。
- **跨包调用禁止直接 import 他包**：调用方在本包定义所需接口（seam），由组合根装配 adapter（如 `backend/internal/httpapi/moderation_adapter.go`）。
- 哨兵错误变量 + 调用方映射 HTTP 状态码；测试跨 seam（fake repo / testcontainers 真 PG）。
- **notify 快照自足**：通知行快照字段（ActorName/Avatar/TargetTitle）由调用方算好传入，notify 从不 join 他表（`backend/notify/model.go:7` 注释；#4 D4）。
- 迁移：`backend/migrations/*.sql` 按字典序 embed，启动自动跑，**每文件单事务、按 ASCII `;` 拆语句**（`backend/migrations/migrate.go:43-57,64-75`）。

**角色与权限**：
- `user.users.role`：`member | moderator | admin`；JWT claims 映射 `user | moderator | admin`。
- 中间件：`Auth`（纯验签）、`OptionalAuth`、`RequireRole(roles...)`（未登录 401、角色不足 403）。写组额外 `RequireActive`（封禁拦截）。

**治理链现状（#33 举报闭环已完成；#34 封禁已上线）**：
- `moderation.reports` + `moderation.moderation_actions`（append-only 审计，永不删除）。
- 举报流程：`POST /reports`（登录墙）→ `POST /moderation/reports/:id/handle`（mod 组，动作 dismiss/delete_post/delete_comment/warn）→ 审计 + `report_result` 通知（**目前只发举报人**，D33-2）。
- **本票两项遗留**：#53 ①补发被举报人（推翻 D33-2 部分定案）；②举报频控（MVP 已知缺口 F7）。

## 1. 背景与目标（为什么做）

- 需求来源：GitHub **issue #53**「#33 遗留收尾：被举报人触达 + 举报频控」（地图 Roadmap #4）。
- 交接依据：`docs/handoffs/53-report-followup.md`（自包含交接，§3 列出 open decisions）；前置决策 `docs/handoffs/grilling-decisions/issue-33-report-moderation-decisions.md`（D33-2 只发举报人、O4 警告仅留痕、O3 被举报人触达留待未来）。
- **目标**：①处理动作 delete_post/delete_comment/warn 后，被举报人收到「内容被处理」通知（举报人通知保留）；②`POST /reports` 加全局频控（同一举报人 10 分钟 ≤5 次，超限 429 不落库）。
- **性质**：补发被举报人 = **策略变更**，已按 SOP §3 第 0 步 grilling 定案（§3）。

## 2. 真值核对（数据来源，全部可复现）

### 2.1 服务器真值（2026-08-24 实查，SSH 端口 2222）

命令：`ssh -p 2222 liyongquan@122.51.233.225 'cd ~/ai-forum && git log --oneline -1 && docker compose ps && curl -s http://localhost:8888/healthz'`

| 项 | 实查结果（摘录） | 含义 |
|---|---|---|
| HEAD | `0301601 Merge pull request #52 from li-yongqvan/fix/deploy-ssh-port-2222` | 线上 = PR #52，与本地 origin/main 一致 |
| 容器 | `ai-forum-api` Up(healthy) / `ai-forum-postgres` Up(healthy) / `ai-forum-web` Up | 部署形态 8888 容器化 |
| healthz | `{"status":"ok"}` | 服务正常 |

### 2.2 线上 DB 真值（2026-08-24 实查，约束名）

命令：`psql -c "SELECT conname, contype FROM pg_constraint WHERE conrelid='notify.notifications'::regclass;"`
```
notifications_type_check | c | CHECK (((type)::text = ANY ((ARRAY['follow', 'like', 'comment', 'reply', 'report_result'])::text[])))
```
→ **事实：`notify.notifications.type` CHECK 约束名 = `notifications_type_check`**（0003 内联列级约束，Postgres 确定性命名），迁移 0008 drop/add 可引用该名。

### 2.3 代码真值（本机仓库 `C:\Users\liyongquan\ai-forum`，逐条核对）

**notify 域**：
- `CreateNotification(ctx, CreateNotificationCmd)` **单接收人单行 INSERT**，无批量接口（`backend/notify/service.go:96-110`，`:100` `s.repo.CreateNotification(ctx, &Notification{...})`）。
- Notification 快照字段：`RecipientID / Type / ActorID* / ActorName* / ActorAvatar* / TargetType* / TargetID* / TargetTitle*`，**无 content 字段**（`backend/notify/model.go:8-21`）。
- type 白名单 5 类含 `report_result`：应用层 `backend/notify/service.go:80-82`；DB CHECK `backend/migrations/0003_notify_schema.sql:7`。
- `report_handled` = 14 字符，`type VARCHAR(16)` 可装（`0003_notify_schema.sql:7`）。

**moderation 域**：
- `Report` 模型**无被举报人 user id 字段**（`backend/moderation/model.go:8-21`）。
- `notifyReportResult` **只发举报人**（`r.ReporterID`）、无 actor、`TargetTitle=conclusionFor(action)`（`backend/moderation/service.go:354-367`）。
- `conclusionFor`：delete→「内容已删除」/ warn→「已警告违规用户」/ dismiss→「未采取处理」（`service.go:177-186`）。
- `HandleReport` O1 序列：事务外内容副作用 → 单事务（审计+状态）→ **提交后** `notifyReportResult`，失败打日志不回滚（`service.go:275-351`，S3 注释 `:349`）。
- **delete 路径故意不 ResolveTarget**（幂等重处理 O1-②，`service.go:307-317`；adapter 把「不存在/已删」当成功 `backend/internal/httpapi/moderation_adapter.go:46-68`）。→ 处理时内容可能已删，现查作者 id 不可靠，**须 CreateReport 时快照**。
- `CreateReport` 已 ResolveTarget 做存在性校验 + 自举报拒绝（`service.go:236-251`），可顺路快照作者 id。
- 哨兵错误：`ErrNotFound/ErrDuplicatePending/ErrInvalidAction`（`service.go:138-142`）。
- 频控现状：**无任何时间窗口逻辑**；仅 `uq_reports_pending` 部分唯一索引（`backend/migrations/0004_moderation_schema.sql:23`）+ `PendingExists` 预检（`service.go:254`）。
- `reports.created_at`（`0004_moderation_schema.sql:16`）+ `idx_reports_reporter`（`:19`）已可用于窗口计数。

**handler/router**：
- `respondModerationError` 映射：404/409/400/500，**无 429**（`backend/internal/httpapi/handler/moderation.go:28-39`）。
- `CreateReport` handler 返回 201（`moderation.go:69`）；登录墙取 `middleware.Identity`（`:43-47`）。
- 路由：`writers.POST("/reports", mh.CreateReport)`（`backend/internal/httpapi/router.go:128`）；mod 组 `GET /moderation/reports`、`GET .../count`、`POST .../:id/handle`（`router.go:137-139`）。
- Notifier adapter 透传 `moderation.NotificationCmd → notify.CreateNotificationCmd`，**无 ActorAvatar**（`backend/internal/httpapi/moderation_adapter.go:120-131`）；`NotificationCmd` 最小子集含 RecipientID/Type/ActorID/ActorName/TargetType/TargetID/TargetTitle（`backend/moderation/service.go:120-129`）。

**迁移器**：
- `applyStatements` 按 ASCII `;` 拆语句逐条执行（`backend/migrations/migrate.go:64-75`）→ **迁移文件注释里禁半角分号；DO 块不可用**（体内 `;` 会被拆断）。
- 每文件单事务（`migrate.go:43-57`）；失败 → `main.go:41` fatal，下次启动重试。

**前端**：
- `NotificationType` 联合 5 类（`frontend/src/api/types.ts:124`）。
- `Notifications.vue textFor()`：`report_result` → `举报结果：${target_title}`（举报人视角硬编码，`frontend/src/views/Notifications.vue:70-71`）；`linkTo()` default → null（`:88-89`）。
- `Reports.vue` 警告按钮文案 `'警告（仅留痕，对方不会收到通知）'`（`frontend/src/views/Reports.vue:42`）。
- `client.ts request()`：非 2xx 抛 `ApiError(status, data.error)`（`frontend/src/api/client.ts:53,64-67`）；**429 不在 sessionLost 分支**（仅 401/403 banned，`:57-58`）→ 429 正常透传为普通错误。
- `ReportSheet.vue` 提交失败 toast `(e as Error).message`（`frontend/src/components/ReportSheet.vue:66-69`）→ 429 body 文案会透出，**前端举报入口无需改**。

### 2.4 既有测试断言（改后必红，须同步更新）

- `backend/moderation/service_test.go`：`TestHandleReportDismiss`(:151)、`TestHandleReportDeletePost`(:176)、`TestHandleReportWarnAttribsAuthor`(:193) 均断言 `len(notify.calls) == 1`。
- `frontend/src/views/Reports.test.ts`：`:65` 与 `:72` 断言旧警告文案 `'警告（仅留痕，对方不会收到通知）'`。

### 2.5 GitHub 状态

- issue #53 OPEN（地图 Roadmap #4）；地图 issue #1 决策 22 记录 #33（D33-2 只发举报人）。
- 本地 `git fetch origin`：origin/main = 本地 HEAD = `0301601`；工作区无已跟踪脏文件（仅 untracked handoff 文档）。

## 3. Grilling 决策记录（4 项，用户确认 2026-08-24）

| 编号 | 决策问题 | 定案 | 依据 |
|---|---|---|---|
| D1 | 被举报人触达：report_result 发谁 | **举报人 + 被举报人都发**（保留 `report_result` 给举报人，补发 `report_handled` 给被举报人，各自视角） | 用户确认（2026-08-24，AskUserQuestion Q1） |
| D2 | 被举报人收哪些动作 | **delete_post / delete_comment / warn**；dismiss 不通知被举报人（举报人照旧收结论） | 用户确认（2026-08-24，Q2）。备选「仅删帖删评」未选：warn 是实质处理，补触达意图含 warn |
| D3 | 举报频控规则 | **全局窗口**：同一举报人 10 分钟 ≤5 次（跨目标） | 用户确认（2026-08-24，Q3）。备选「同目标冷却 / 组合」未选：MVP 防刷量够用 |
| D4 | 频控超限处理 | **HTTP 429 + 提示，不落库** | 用户确认（2026-08-24，Q4） |

## 4. 范围收敛与明确不做

| 项 | 决策 | 依据 |
|---|---|---|
| 动后端 `backend/notify` + `backend/moderation` + handler/路由 + 前端微调（types.ts / Notifications.vue / Reports.vue 文案） | 做 | handoff §4 红线；issue #53 范围 |
| 碰 content 域（`backend/content/`） | **不做** | handoff §4 红线（并行 #54 占用） |
| 碰 `.github/workflows/` | **不做** | handoff §4 红线（并行 #43 占用） |
| 新增 API 端点 / 改 `ReportView` JSON | **不做** | 通知在 service 层发；`target_author_id` 不进 JSON（mod 队列不需要） |
| 被举报人通知拼入 `reporter_note`（举报人自由文本） | **不做** | 【决策】隐私 + `target_title` VARCHAR(255) 逼紧；见 §6.2 |
| 存量 pending 报告回填 `target_author_id` | **不做** | post/comment 可能已删，join 不可靠；0006 先例纯 additive；存量 NULL → 跳过 + slog.Warn（§7 #4） |
| `issue-33-report-moderation-decisions.md` 追加 D33-2/O4 反转注记 | **做**（随本票 PR） | 评审 F5：D33-2（只发举报人）/O4（警告仅留痕）将被本票反转，防未来会话以旧定案为准 |
| 封禁动作/用户举报前端入口 | **不做** | #33 D1/D3 既定，非本票 |

## 5. 实现方案（每项给依据）

### 5.1 迁移（2 个新文件，`backend/migrations/`）

**0007_reports_target_author.sql**：
```sql
ALTER TABLE moderation.reports ADD COLUMN target_author_id BIGINT;
```
- 依据：Report 无作者字段（§2.3）、delete 路径不 ResolveTarget（§2.3）→ 快照列；additive 可空，纯 additive 先例 `0006_reports_reporter_note.sql`。

**0008_notify_type_report_handled.sql**：
```sql
ALTER TABLE notify.notifications DROP CONSTRAINT IF EXISTS notifications_type_check;
ALTER TABLE notify.notifications ADD CONSTRAINT notifications_type_check CHECK (type IN ('follow','like','comment','reply','report_result','report_handled'));
```
- 依据：约束名已实查（§2.2）；`report_handled` 14 字符可装（§2.3）；**注释禁半角 `;`**（迁移器拆分，§2.3）。
- 注意：不用 DO 块（拆分器会拆断，§2.3）；两条 ALTER 由迁移事务包裹，ADD 失败则 DROP 回滚、启动 fatal fail-fast（§2.3）。

### 5.2 moderation 域（`backend/moderation/`）

1. `model.go`：`Report` 加 `TargetAuthorID *int64`（GORM snake_case 自动映射 `target_author_id`，不进 `ReportView`）。依据：§2.3。
2. `repo.go`：`Repo` 加 `CountReportsSince(ctx, reporterID int64, since time.Time) (int64, error)`。依据：§2.3 无窗口逻辑。
3. `gorm_repo.go`：`WHERE reporter_id=? AND created_at >= ?` COUNT（`idx_reports_reporter` 覆盖，§2.3）。
4. `service.go`：
   - 哨兵 `ErrRateLimited`；常量 `reportWindow = 10 * time.Minute`、`reportLimit = 5`。依据：D3/D4。
   - `CreateReport`：post/comment 分支快照 `ref.AuthorID`、user 分支 `&in.TargetID` 进 `rep.TargetAuthorID`。**检查顺序：目标校验 → PendingExists(409) → 频控(429) → 落库**（见 §6.4）。依据：§2.3 + D3/D4。
   - `notifyReportResult` → 改名 `notifyReportOutcome`：举报人 `report_result` 保留；`notifiesReportedUser(action)`（delete/warn）且 `TargetAuthorID != nil` 时补发 `report_handled`（RecipientID=作者、TargetTitle=`handledConclusionFor(action, r.Reason)`、无 actor）；nil → slog.Warn。两条各自吞错打日志不回滚（S3）。依据：D1/D2 + §2.3（通知在 service 层）。
   - **F2（评审采纳）**：`report_handled` 快照字段——`TargetType`/`TargetID` **携带**（与 `report_result` 一致，照抄 `r`，供未来客户端筛选；前端 `linkTo` 对 report_handled 返回 null，无死链风险）；`target_title` 承载**整句文案**（「你的内容因「垃圾广告」被举报，已删除」）属**刻意语义复用**（notify 域定义是「跳转锚点+标题」，此处作消息体）——实现时在 `notifyReportOutcome` 注释明示。
   - 新增 helper：`notifiesReportedUser(action)`（dismiss→false）、`handledConclusionFor(action, reason)`（delete→「你的内容因「{reason}」被举报，已删除」；warn→「你因「{reason}」被举报，已警告」）。依据：D2 + §6.2。
5. `fake_repo_test.go`：`fakeRepo` 加 `CountReportsSince` 实现（按 map 扫描计数）。依据：测试跨 seam（§0）。

### 5.3 notify / moderation 域（README 同步，F1 评审采纳）

- `backend/notify/service.go` `validTypes`（:80-82）加 `"report_handled": true`；顺带更新 `:14`/`:57` 注释、`model.go:11` 注释、`notify/README.md:27` 白名单描述。依据：§2.3（白名单两处需同步，防文档漂移）。
- **`backend/moderation/README.md` 纳入同步（F1，重要）**：3 处随本票陈旧（§2.3 已核）——
  - `:8` 范围「`report_result` 通知（只发举报人，#33 D2）」→ 改「`report_result`（举报人）+ `report_handled`（被举报人，仅 delete/warn）」；
  - `:25` Ordering「提交后事务外发 `report_result` 通知」→ 改「提交后事务外发双通知（各吞错不回滚）」；
  - `:30-35` 错误表补 `ErrRateLimited → 429`；
  - 不变量节补「被举报人通知仅 delete/warn（dismiss 不发）」与「频控软限：窗口计数 ≥5 → 429，含 dismissed，MVP 精度」。

### 5.4 handler（`backend/internal/httpapi/handler/moderation.go`）

- `respondModerationError`（:28-39）加：`case errors.Is(err, moderation.ErrRateLimited): respondError(c, http.StatusTooManyRequests, "举报过于频繁，请稍后再试")`。依据：D4 + §2.3（无 429 映射）。

### 5.5 前端（4 处微调）

1. `frontend/src/api/types.ts:124`：`NotificationType` 加 `'report_handled'`。依据：§2.3。
2. `frontend/src/views/Notifications.vue`：`textFor()` 加 `case 'report_handled': return n.target_title ? n.target_title : '你的内容已被处理'`（作者视角全句由后端算好）；`linkTo()` 显式 `case 'report_handled': return null`；:3 注释补类型。依据：§2.3（文案前端按 type 拼）。
3. `frontend/src/views/Reports.vue:42`：警告按钮文案 → `'警告（会通知被举报人）'`；:35 注释同步。依据：O4 反转（warn 现在通知被举报人，D2）。
4. **Q4 附带（评审采纳）**：`Notifications.vue` 渲染层对 `report_handled` 用系统默认头像替代「?」（纯渲染分支，不改后端；消除「?」读作「数据缺失」的歧义）。依据：评审意见 Q4。备选：不修，接受「?」（已记入 §6.3）。

## 6. 关键设计裁决（【决策】，含理由与备选）

### 6.1 通知类型：新增 `report_handled` 而非复用 `report_result`
- **问题**：被举报人通知用新类型还是复用现有 `report_result`？
- **定案【决策】**：**新增类型 `report_handled`**。
- **理由**：前端 `report_result` 模板硬编码「举报结果：${target_title}」（举报人视角，§2.3）；复用会渲染成「举报结果：你的内容因…」——视角错乱。新类型让 notify 域两类语义清晰，各配模板。
- **备选（不选）**：复用 `report_result` + 改前端模板去掉前缀 + 后端拼全句——省一次迁移，但**同时改变现有举报人侧文案与既有断言**（§2.4），且通知列表里两类无法用 type 区分，blast radius 更大。

### 6.2 被举报人通知内容：只带原因枚举，不拼 `reporter_note`
- **问题**：原因选「其他」时，通知是否带举报人备注？
- **定案【决策】**：**只带原因枚举**（「你的内容因「其他」被举报，已删除」）。
- **理由**：`reporter_note` 是 ≤200 字自由文本，可能含主观攻击/身份线索——泄露给被举报人 = 隐私 + 升级风险；且 `target_title` VARCHAR(255) 逼紧。
- **备选（不选）**：拼 `reporter_note`（信息量更大）——风险 > 收益，MVP 不做；留 §9 Q2 供评审。

### 6.3 `report_handled` 不发 actor
- **问题**：通知是否带处理人/平台 actor 快照？
- **定案【决策】**：**不发 actor**（与现有 `report_result` 一致）。
- **理由**：带处理人 = 暴露具体 moderator；带「平台」= 硬编码字符串进治理域。头像显示「?」是已知 UX 瑕疵（`Notifications.vue:146` `actor_name || '?'`），可接受。
- **备选（不选）**：`ActorName="平台"` 或经 `UserGateway.GetUserView(HandlerID)` 取处理人名——前者引入魔法字符串、后者多一次查询 + 暴露 moderator，MVP 不值。

### 6.4 频控检查顺序与软限性质
- **问题**：频控与既有 `PendingExists` 防重复的顺序？是否承诺严格限额？
- **定案【决策】**：**PendingExists(409) 先于频控(429)**；频控为**软限**（count+insert 非原子，并发可少量超发）。
- **理由**：同目标 pending 重复的「你已举报过该内容」比「过于频繁」更准确；硬不变量仍是 `uq_reports_pending`（23505），频控不承诺严格（MVP 规模 ~50 人）。
- **备选（不选）**：频控前置（省一次内容网关调用）——语义上重复举报会错误地给 429。

### 6.5 作者 id 快照在 CreateReport（迁移 0007）而非 HandleReport 现查
- **问题**：delete 处理时内容已删，`ResolveTarget` 现查作者会 `ErrNotFound`（§2.3），如何可靠拿被举报人 id？
- **定案【决策】**：**CreateReport 时快照 `target_author_id`**（0007），HandleReport 只读快照。
- **理由**：CreateReport 已 ResolveTarget 做存在性校验（§2.3），顺路快照零额外查询；处理后即使内容已删也能定位作者。与 notify「快照自足」哲学一致（§0）。
- **备选（不选）**：HandleReport 删除前 ResolveTarget——破坏 delete 幂等设计（O1-②），且已删内容无法归责。

## 7. 边界与不变量清单（含防护层）

| # | 不变量 | 防护层 | 依据 |
|---|---|---|---|
| 1 | 被举报人通知仅 delete/warn 动作 | `notifiesReportedUser(action)` 门（dismiss→false） | §5.2 / D2 |
| 2 | 举报人通知保留（所有动作含 dismiss） | `notifyReportOutcome` 第一段不变（`report_result`） | §5.2 / D1 |
| 3 | 举报人 ≠ 被举报人（双通知不发给同一人） | `CreateReport` 自举报守卫（既有 `service.go:241,245`） | §2.3 |
| 4 | 存量 `target_author_id` NULL 不 panic | nil 检查 → 跳过 + slog.Warn | §5.2 / §4 |
| 5 | 窗口内计数 ≥5 → 429 且不落库 | `CountReportsSince` 前置返回 `ErrRateLimited`；无插入 | §5.2 / D3/D4 |
| 6 | 同人同目标 pending 期一条 | `uq_reports_pending` + `PendingExists`（既有，回归） | §2.3 |
| 7 | 通知失败不回滚处理结果 | 两条通知各自吞错 + slog.Warn（S3） | §2.3 / §5.2 |
| 8 | 仅白名单类型可入通知表 | DB CHECK（0008）+ `notify.validTypes` 双白名单 | §5.1 / §5.3 |
| 9 | `target_author_id` 不泄露给 mod 队列 JSON | `ReportView` 不含该字段（model 内部字段） | §5.2 / §4 |
| 10 | 迁移 0007/0008 幂等可重跑 | 迁移事务包裹；0008 `DROP IF EXISTS` + ADD | §5.1 / §2.3 |
| 11 | 游客 401 / 普通用户 403（既有墙回归） | `writers` + `RequireRole` 组 | §2.3 / §0 |
| 12 | 频控并发超发不破坏数据 | 软限（接受少量超发）；硬不变量 #6 不变 | §6.4 |
| 13 | 作者先自删 + 后处理：被举报人收「你的内容已删除」通知（内容早已自删） | O1-② 幂等删除已接受此精度，现第一次用户可见；通知由快照作者发出 | §5.2 / 评审 F4 |
| 14 | 频控窗口计数**含已 dismissed 举报**（不区分状态） | `CountReportsSince` 按 `created_at >= since` 计数、无状态过滤；MVP 接受为已知精度 | §5.2 / 评审 F6 |

## 8. 测试与验证计划

**moderation service 单测**（`backend/moderation/service_test.go` + `fake_repo_test.go`）：
- 必改：`TestHandleReportDismiss`/`DeletePost`/`WarnAttribsAuthor` 通知断言改双发（`fakeNotifier.calls` 断言 `calls[1]` = report_handled→100 + 结论句）。依据：§2.4。
- 新增：delete_comment 通知作者；warn user 目标（report_handled→100「你因「垃圾广告」被举报，已警告」）；存量 nil 跳过（仅 1 条）；频控三件套（第 6 次 `ErrRateLimited` / 第 5 次放行 / 窗口外放行——直接改 `repo.reports[id].CreatedAt` 到过去，fake 存指针 `fake_repo_test.go:43`）。

**notify service 单测**：`report_handled` 合法 type 写入成功；`bogus` 仍拒（回归 `TestCreateNotificationInvalidType`）。

**handler 集成**（真 PG，`backend/internal/httpapi/handler/moderation_test.go`，`testutil.SetupPG` 自动跑 0007/0008 验证约束名）：
- `TestReportedUserNotification`：**累计语义**——dismiss→作者 0 条 → delete_post→作者 1 条 `report_handled` → warn→作者**累计 2 条**（最新为 warn 文案「你因「人身攻击」被举报，已警告」）。复用 `reportCode`/`handle`/`registerNotifyUser`/`makeModerator` helper（§2.3 `moderation_test.go:42-165`）。
- `TestReportRateLimit`：同举报人 5 连报（**5 个不同帖子**，避免 `uq_reports_pending` 先 409）全 201；**第 6 次打在全新帖子**（打旧目标会被 409 先拦，无法验证 429）→ 429；`reports` 计数 5（不落库）。

**前端单测**：
- `Reports.test.ts:65,72`：断言改新文案。
- `Notifications.test.ts`：新增 `report_handled` 渲染（target_title 原文）。
- `ReportSheet.test.ts`：新增 429 透出文案用例（mock `createReport` reject）。

**本地全量**：`cd backend && go build ./... && go vet ./... && go test ./...`；`cd frontend && npm run test:unit && npm run build`；提交前 `git checkout -- frontend/components.d.ts`。

## 9. 待评审焦点（Q1-Q7，作者最想让评审盯的点）

> 以下为评审 agent 的定向问题；每条注明为什么值得盯。

- **Q1** §6.1 新增 `report_handled` 类型的裁决是否成立？备选「复用 report_result + 改前端模板」是否被我低估？（命名 `report_handled` vs `report_processed` 亦可议）
- **Q2** §6.2 被举报人通知**不拼 `reporter_note`**（「其他」原因只显示枚举）——隐私与信息量的取舍是否合适？
- **Q3** D3 频控参数 **10 分钟 / 5 次**是否合理？§6.4 软限（并发超发 1-2 条）在 MVP 是否可接受？
- **Q4** §6.3 `report_handled` 不发 actor → 通知头像显示「?」——是否值得为此引入 `ActorName="平台"`？
- **Q5** §4 存量 `target_author_id` NULL → 跳过通知 + slog.Warn，是否应改回填（如仅对仍存在的目标）？
- **Q6** 0008 迁移依赖确定性约束名 `notifications_type_check`（已实查 §2.2）；是否需更强的动态兜底（受迁移器 `;` 拆分限制，DO 块不可用）？
- **Q7** D2 反转 #33 O4（warn 从「仅留痕、不通知」→「通知被举报人」）——属规格偏离，是否符合 #53 意图？需在决策记录明示偏离。

## 10. 评审意见采纳记录（2026-08-24，有条件通过 0 阻塞）

评审意见书：`docs/plans/issue-53-report-followup-设计文档-评审意见书.md`。Q1-Q7 全部认可（Q4 附前端优化建议）；F1-F6 全部采纳。推翻项：无。

| 评审项 | 结论 | 采纳落地 |
|---|---|---|
| **F1** `moderation/README.md` 未列入同步（3 处陈旧） | 重要，属实 | §5.3 纳入 `moderation/README.md` 同步：范围/Ordering 改双通知描述、错误表补 `ErrRateLimited→429`、不变量补「被举报人通知仅 delete/warn」与频控软限 |
| **F2** `report_handled` 的 `target_title` 语义过载未明示 | 属实 | §5.2 显式写明：`target_title` 承载整句文案属刻意复用；`report_handled` **携带 `TargetType`/`TargetID`**（供未来客户端筛选，前端 linkTo 已 null 无死链） |
| **F3** 测试描述歧义 | 属实 | §8 修正：warn 场景为**累计**（dismiss 0 → delete 1 → warn 2）；频控测试第 6 次打在**全新目标**（旧目标先 409） |
| **F4** 作者先自删 + 后处理边角 | 属实 | §7 补记不变量 #13：作者自删后 mod 处理 → 被举报人收「内容已删除」通知（O1-② 已接受精度，现用户可见，接受） |
| **F5** issue-33 决策文件将被本票反转 | 属实 | §4 范围增列：随本票 PR 在 `issue-33-report-moderation-decisions.md` D33-2/O4 处追加反转修订注记 |
| **F6** 频控窗口含 dismissed 举报 | 属实 | §7 补记不变量 #14：窗口计数含已 dismissed 举报，MVP 接受为已知精度 |
| **Q4 附带** 前端「?」头像优化 | 建议，采纳 | §5.5 增列：前端 `Notifications.vue` 对 `report_handled` 渲染系统默认头像（纯渲染层，不改后端） |
| **Q1-Q7** 定向裁决 | 全部认可 | 维持 §6 各裁决；命名 `report_handled` 维持 |

**推翻项**：无。评审方独立复现服务器/DB 实查，全部证据链成立。

---

*本文档为实现基线（供 plan-review 评审）。评审通过后进入实现（分支 → 实现 → 测试 → PR → 用户授权 → 合并 → 部署 → 服务器验证 → 回报统筹方更新地图 #1）。*
