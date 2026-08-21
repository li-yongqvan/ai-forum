# #34 治理权限矩阵：用户封禁/解封 + 权限校验 — 设计文档（供评审）· 归档

> 本文件 = **#34 设计评审归档 + 决策记录**（评审 F4：grilling 决策落档，随 PR 入库）。
> 文档用途：设计评审归档（独立评审 agent 已于 2026-08-21 评审，有条件通过、0 阻塞）；实现基线。
> 溯源约定：每个结论标注来源。**事实**标来源（代码 `file:line` / DB 实查输出 / GitHub issue / grilling 用户确认）；**判断性裁决**单独标注【决策】并给出理由与备选。
> 数据时点：2026-08-21。评审意见书原件在评审 workspace，不在仓库内；本文件 §10 为采纳记录。

---

## 0. 项目上下文（给零背景评审 agent，先读本节）

> 评审 agent 对项目一无所知，本节提供判断本设计所需的最小项目背景。每个事实附仓库内可自行查阅的源文件。

**这是什么**：AI 智联论坛——社团内部 AI 主题社区的移动端论坛 MVP。
- 前端：Vue 3 + TypeScript + Vite + Vant（PWA），纯静态部署（`frontend/`）。
- 后端：Go（Gin HTTP 框架 + GORM ORM + PostgreSQL），单进程模块化单体（`backend/`）。
- 数据库：PostgreSQL 单实例、**多 schema**，每域一个：`user` / `content` / `notify` / `moderation`。
- 部署：单台阿里云服务器 Docker Compose 三容器（web=nginx@8888 / api=:8080 / postgres），无域名无 HTTPS，线上 `http://122.51.233.225:8888/`。零迁移 = 本票不需要改任何 schema。

**后端架构约定（深模块 Deep Module，评审本设计必须理解）**：
- 4 个领域包（`user`/`content`/`notify`/`moderation`），每包结构固定：`model.go`（领域模型+DTO）、`repo.go`（Repo 接口=持久化 seam）、`gorm_repo.go`（GORM 实现）、`service.go`（Service 接口+实现：粗粒度命令+读模型查询）、`README.md`（Interface README，外部调用方必读）。
- **跨包调用禁止直接 import 他包**：调用方在本包定义所需接口（seam，如 `ContentGateway`/`UserGateway`/`Notifier`/`UserProvider`），由组合根 `backend/internal/httpapi/`（`router.go` + 各 `*_adapter.go`）装配具体实现注入。**本设计的 `ActiveChecker` seam 即此纪律的产物**。
- **跨包聚合在 handler 层做**（先例：`follow.go` 聚合 user+content+notify；`profile.go` 合并两包）。
- 哨兵错误变量（`ErrNotFound` 等）+ 调用方据此映射 HTTP 状态码；错误映射辅助 `respond*Error`。
- 测试跨 seam（service 接口）：单测用 fake repo（`<包>/fake_repo_test.go`），handler 集成用 testcontainers 真实 PG（`internal/testutil/db.go`，无 Docker 自动 skip）。
- 可查阅：`docs/workflow.md`（SOP）、`docs/design-principles.md`（深模块规范）、`backend/moderation/README.md`（治理域 Interface README）。

**角色与权限**：
- `user.users.role` 存 `member | moderator | admin`；JWT claims 与对外 API 映射为 `user | moderator | admin`（`guest` 仅前端概念，不入库不入 token）。
- 中间件（`backend/internal/httpapi/middleware/auth.go`）：`Auth`（Bearer JWT，**纯验签不查 DB**）、`OptionalAuth`、`RequireRole(roles...)`（读 JWT `claims.Role`，未登录 401、角色不足 403）。
- **权限矩阵定稿于 IA**（`docs/ux/info-architecture.md` §5.0）：封禁/解封**仅 admin**；moderator 可删他人内容/置顶/精华/警告/处理举报（动作集不含封禁）。

**治理链现状（#33 举报闭环已完成，本票 #34 是它的下游）**：
- `moderation.moderation_actions`：**append-only 审计表**（`ModeratorID/Action/TargetType/TargetID/Reason/CreatedAt`），action 枚举 `delete_post|delete_comment|ban_user|warn|unban_user`（线上 DB 已核实含 ban/unban）。
- `moderation.reports`：举报表（`pending→resolved/dismissed`），部分唯一索引防重复举报；`HandleReport` 事务内追加审计 + 条件更新报告状态。
- `user.users.status`：`active|banned` 两态；**登录已拒绝 banned 用户**（`user/service.go:219-221`）。
- **缺失**（= 本票 #34 范围）：封禁/解封 API、被封用户的写操作拦截、admin-only 权限门、前端封禁入口。

**工作流程（评审无需执行，仅了解背景）**：
- 项目用 `docs/workflow.md`（SOP）管理：取票→设计（grilling HITL 定决策，§3 决策来自用户确认）→实现→测试→PR→**用户授权**合并→CI 部署→服务器验证→更新地图 issue #1。
- 决策记录落 `docs/handoffs/grilling-decisions/issue-N-slug-decisions.md`（#33 先例：`issue-33-report-moderation-decisions.md`）。

**术语速查**：seam（跨包接口注入点）/ 组合根（httpapi 装配层）/ 哨兵错误（sentinel errors）/ handler（HTTP 处理器）/ 写操作（需登录且变更数据的请求）/ 审计（append-only 治理留痕）。

---

## 1. 背景与目标（为什么做）

- 需求来源：GitHub **issue #34**「治理权限矩阵：用户封禁/解封与 mod 权限校验落地」（正文：users 有 role 但"无封禁能力、无权限强制校验"；范围 = 封禁/解封 API、拦截中间件、权限矩阵校验、枚举迁移）。
- 交接依据：`docs/handoffs/34-governance-permissions.md` §1-§3（目标、已探明现状、5 项开放决策；含一处「范围纠正」：**`unban_user` 枚举已在线上 DB，无需枚举迁移**，本票真正做 ban/unban API + 登录/写拦截 + 权限校验）。
- 治理链位置：地图 issue #1 决策 22（#33 举报闭环，D33-1「封禁留 #34」）→ 本票补齐封禁环。
- **目标**：admin 可封禁/解封用户 → 被封用户全禁（登录拒绝 + 写操作拦截）→ 每次封禁/解封写 `moderation_actions` 审计 → 前端（用户主页）提供封禁/解封入口与状态展示。

---

## 2. 真值核对（数据来源，全部可复现）

### 2.1 服务器真值（2026-08-21 实查，SSH `cd ~/ai-forum && git log --oneline -1 && docker compose ps && curl -s http://localhost:8888/healthz`）

| 项 | 实查结果 | 含义 |
|---|---|---|
| HEAD | `3074565 Merge pull request #44 from li-yongqvan/chore/frontend-test-promote` | 线上 = PR #44（前端单测转正，地图决策 24） |
| 容器 | api/postgres/web 三容器 Up，api healthy，web @8888 | 部署形态 8888 容器化 |
| healthz | `{"status":"ok"}` | 服务正常 |

### 2.2 线上 DB 枚举核实（SSH + `psql` 查 `moderation.moderation_actions` 约束，2026-08-21 两次实查）

```
moderation_actions_action_check: CHECK (((action)::text = ANY ((ARRAY['delete_post'::character varying,'delete_comment'::character varying,'ban_user'::character varying,'warn'::character varying,'unban_user'::character varying])::text[])))
```

→ **事实：`moderation_actions.action` CHECK 已含 `ban_user` 与 `unban_user`**。据此：
- handoff「范围纠正」成立：**本票零枚举迁移**。
- 数据模型佐证：`backend/migrations/0004_moderation_schema.sql:30`（action CHECK 含五值）。
- 评审 Q8 前置：开工第 0 步已重查线上枚举（2026-08-21），与 §2.2 对照一致。

### 2.3 代码真值（本机仓库，git status：main 落后 origin 2 commit）

**user 域**（`backend/user/`）：
- `model.go:10-22`：`User.Status string // active | banned`（0001 迁移 `0001_user_schema.sql:13` 有 CHECK `status IN ('active','banned')`）。
- `service.go:211-223` `Login`：`u.Status == "banned"` → 返回 `ErrBanned`（登录已拦截封禁用户）。
- `service.go:330-335` `Ban`：`GetUserByID` 存在性检查 → `repo.UpdateUserStatus(id, "banned")` **无条件更新**；`OperatorID` 未使用；**无 Unban**。
- `repo.go:12` `UpdateUserStatus(ctx, id, status)` 无条件更新；`gorm_repo.go:51-53` 实现。
- `service.go:78-93` `Service` 接口：含 `Ban`、无 `Unban`、无封禁状态查询。
- `service.go:101-111` `PublicProfileView`：**不含 role/status/banned**。
- 事实佐证：`user.Service.Ban` **无外部调用方**（仅 `service_test.go` 引用，全库 grep），可安全改造语义。

**moderation 域**（`backend/moderation/`）：
- `model.go:27-35` `ModerationAction`（append-only 审计）：`ModeratorID/Action/TargetType/TargetID/Reason/CreatedAt`。
- `service.go:17-24` 常量 `ActionBan="ban_user"`、`ActionUnban="unban_user"` 已定义，注释标注「仅 admin；#34 本票不启用」。
- `service.go:147-155` `validAction`：只放行 `dismiss/delete_post/delete_comment/warn`，**拒绝 ban/unban**（服务端拦截）。
- `repo.go:16` `AppendAction(ctx, *ModerationAction) error`；`gorm_repo.go:115-117` 实现（`Create`）。
- `service.go:187-192` `Service` 接口：`CreateReport/HandleReport/ListReports/CountReports`——**无独立审计方法**。
- `service.go:262-338` `HandleReport`：事务内 `AppendAction` + 条件更新 `UpdateReportStatus`（`WHERE status='pending'`，F4 并发防护先例）；dismiss 不落审计。

**中间件**（`backend/internal/httpapi/middleware/auth.go`）：
- `Auth`(20-36)：仅校验 JWT 签名/有效期并注入 claims，**不查 DB** → 已签发 token 在封禁后仍有效（#34 写拦截要补的缺口）。
- `RequireRole`(65-82)：从 `claims.Role`（映射后 `user/moderator/admin`）判断；未登录 401「未登录」、角色不足 403「权限不足」。

**路由**（`backend/internal/httpapi/router.go`，130 行）：
- `authed` 组(73-74) `middleware.Auth`；挂 /auth/logout、/auth/me、内容写、通知、/reports、/uploads。
- `mod` 组(119-126) `RequireRole("moderator","admin")`：pin/feature/举报队列。
- 事实佐证：**无 admin-only 组**；**无 ban/unban 路由**。

**handler**（`backend/internal/httpapi/handler/`）：
- `moderation.go` `ModerationHandler`；`HandleReport`(105-139) 模式：Identity → pathID → bind → note≤500 → svc 传 `OperatorRole: claims.Role`；`respondModerationError`(24-35)。
- 跨包聚合先例：`follow.go`（NewFollowHandler 聚合 user+content+notify，SOP §5.8）；`profile.go`（合并两包）。

**前端**（`frontend/src/`）：
- `stores/auth.ts:12-14`：`isLoggedIn = !!s.token`（store state）、`isMod`、`isAdmin`；`clear()`(56-60) 重置 store + localStorage；`fetchMe`(33-42) catch 调 `clear()`。
- `views/UserProfile.vue:67-72`：pacts 操作区（仅 关注/私信，`v-if="!isSelf"`）——封禁按钮位置。
- `components/ReportSheet.vue:34-43`：targetType 支持 `user`（default 分支标题「举报用户」）——用户举报入口可复用。
- `api/report.ts`：无 ban/unban 函数。
- `api/types.ts:102-112` `UserProfile`：不含 `banned`。
- `api/client.ts:39-42`：仅 401+`requireAuth` 清 token（**只清 localStorage，不清 Pinia store**）；**无 403 处理**。

### 2.4 GitHub 状态

- 地图 issue #1 决策 24：PR #44（前端单测转正）已合并，服务器 HEAD `3074565` 一致。
- 本地 `git log --oneline main..origin/main`：落后 `3074565`（PR #44 merge）+ `a5005de`（frontend-test 转正 commit）。开工前需 `git pull`（当前 github.com 443 间歇不可达，重试≤4——SOP 已知坑位）。

---

## 3. Grilling 决策记录（6 项，用户确认）

> 依据：handoff §3 列出的 5 项开放决策 + 本会话与用户确认；第 6 项范围决策由本会话提出并获确认。每项给出定案与依据。

| # | 决策问题 | 定案 | 依据 |
|---|---|---|---|
| D1 | 封禁粒度：全禁 vs 仅禁发帖/评论 | **全禁**（登录拒绝 + 写拦截） | 现状 `status` 仅两态且登录已拒 banned（§2.3）；「半禁」需新状态/字段，IA 未定义。用户确认 |
| D2 | 封禁时长：永久 vs 限时 | **永久**（status='banned'，手工解封） | 零到期逻辑；封禁时间/原因由审计承载。用户确认 |
| D3 | 拦截范围：哪些写操作要拦 | **全部登录写操作**（读不禁） | 一致性防漏网；游客写操作本就被登录墙 401，无需特判。用户确认 |
| D4 | 本票范围：纯后端 vs 含前端 | **后端 + 前端封禁入口**（UserProfile 按钮 + 状态展示 + 用户举报入口） | IA §5.5「admin 另有封禁/解封」；#33 D3「用户举报前端入口留 #34」。用户确认 |
| D5 | 迁移形态：列 vs 表 vs 零迁移 | **零迁移**（复用 `user.users.status` + `moderation_actions` 审计） | §2.2 线上 DB 已含枚举；`status` 已含 banned；审计表已承载时间/操作者/原因。用户确认 |
| D6 | 权限矩阵：谁可封禁/解封 | **admin-only**；解封必须写 `unban_user` 审计 | **IA §5.0 权限矩阵定稿**（`docs/ux/info-architecture.md` §5.0：「封禁/解封用户」仅 admin；解封须记 moderation_actions，见 §10）。用户确认 |

IA §5.0 权限矩阵（`docs/ux/info-architecture.md:118-131`，引用原文核心行）：

| 动作 | guest | user | moderator | admin |
|---|---|---|---|---|
| 封禁/解封用户 | — | — | — | ✓（解封须记 moderation_actions，见 §10） |
| 处理举报 | — | — | ✓（动作集不含封禁） | ✓（全动作集） |

实现约束（IA §5.0 原文）：所有管理操作前后端双重鉴权，前端按角色渲染只是体验层，接口必须独立校验。

---

## 4. 范围收敛与明确不做

| 项 | 决策 | 依据 |
|---|---|---|
| 举报处理弹窗不加入「封禁用户」动作 | 不做（本期）；**IA §5.6 偏离已登记 + 用户显式确认（2026-08-21）** | 依据①#33 D1「处理动作=忽略+删帖删评+警告，封禁留 #34」；②IA §5.6 字面含「封禁用户(仅admin)」（`info-architecture.md:167`）与本收敛**冲突属规格偏离**——按评审 F2 已登记进决策记录、用户确认、合并后回写 IA §5.6（同 #33 D33-3 先例）。封禁能力经独立端点 + 用户主页入口完整提供 |
| 不通知被封用户 | 不做 | #33 D2「`report_result` 只发举报人」；被举报人触达 = 地图开放项「#33 遗留」 |
| 无枚举迁移 | 确认无需 | §2.2 线上 DB 核实 |
| `HandleReport` 仍拒 `ban/unban` | 维持 | `validAction` 白名单不动（防经举报流程绕过权限矩阵），新增独立 `RecordAction` 方法 |

---

## 5. 实现方案（每项给出依据）

### 5.1 user 域（`backend/user/`）

1. `repo.go`：**删除**无条件的 `UpdateUserStatus`，替换为两个领域化条件更新方法（幂等关键）：
   - `SetBanned(ctx, id)`：`UPDATE ... SET status='banned' WHERE id=? AND status='active'`；`RowsAffected==0 → ErrAlreadyBanned`。
   - `SetActive(ctx, id)`：`UPDATE ... SET status='active' WHERE id=? AND status='banned'`；`RowsAffected==0 → ErrNotBanned`。
   - 依据：现有 `UpdateUserStatus` 无条件（§2.3），无法表达"仅当当前状态是 X 才变"；条件更新 = F4 并发防护先例（#33 `UpdateReportStatus WHERE status='pending'`，`docs/impl` + `moderation/repo.go` 注释）。gorm_repo.go + fake_repo_test.go 同步替换。
   - （评审 Q3 附带）`RowsAffected=0` 不区分「已封」与「不存在」——`Ban` 先 `GetUserByID` 得 404，两操作间目标被软删会落 409 而非 404，50 人规模可忽略，**repo 注释点明**。
2. `service.go` 哨兵错误新增：`ErrSelfBan`、`ErrCannotBanAdmin`、`ErrAlreadyBanned`、`ErrNotBanned`（`service.go:57-68` 区）。
3. `service.go` `Ban` 改造：`GetUserByID`（不存在→`ErrNotFound`）→ 自封守卫（`TargetID==OperatorID`→`ErrSelfBan`）→ 封 admin 守卫（`target.Role=="admin"`→`ErrCannotBanAdmin`）→ `SetBanned`。
   - 依据：现有 `Ban` 无调用方（§2.3），可安全改造；防护规则 = 治理常识边界（自封/封管理员会锁死治理能力），标注【决策】。
4. `service.go` 新增 `Unban(ctx, UnbanCmd{OperatorID, TargetID})`：`GetUserByID` → `SetActive`。
5. `service.go` 新增 `IsActive(ctx, userID) (bool, error)`：`GetUserByID`；`ErrNotFound`（软删/不存在）→ `false`（fail-closed）；否则 `status != "banned"`。加入 `Service` 接口。
   - 依据：写拦截中间件需查 DB 反映即时封禁（§2.3 Auth 不查 DB 的缺口）；`UserView` 不含 status（§2.3），新增独立查询。
6. `service.go` `PublicProfileView` 增 `Banned bool`；`PublicProfile` 填充 `u.Status == "banned"`（`u` 已加载零额外查询）。可见性门控在 handler（§5.6）。

### 5.2 moderation 域（`backend/moderation/`）

- `service.go` 新增 `RecordAction(ctx, RecordActionCmd) error`：
  - `RecordActionCmd{ModeratorID, Action, TargetID, Reason}`；`Action` 白名单 `{ban_user, unban_user}`（独立 `validRecordAction`，**不动** `validAction`/`HandleReport`）。
  - reason 必填且 ≤500（`moderation_actions.reason VARCHAR(500)`，0004）。
  - `TargetType` 恒 `"user"`；写入 `repo.AppendAction`（已存在）。
  - （评审 Q1 条件 3）handler 预校验与 `RecordAction` 内部校验**同源**（同一规则：trim + 非空 + ≤500 runes），避免校验不一致导致状态已改而审计被拒。
  - 依据：moderation.Service 现无独立审计方法（§2.3）；`AppendAction` 是现成 seam。

### 5.3 写拦截中间件（`backend/internal/httpapi/middleware/`）

- `ActiveChecker` seam 接口（middleware 包，**不 import user**）：`IsActive(ctx, userID) (bool, error)`。
- `RequireActive(checker)`：`Identity` 缺失→401；`IsActive` 查询失败→500（**fail-closed**）；`!active`→**403 `{"error":"账号已被封禁","code":"account_banned"}`**（F5：机器可读 code 供前端匹配，文案可自由变更；不引入全局错误体 schema 变更）。
- httpapi 组合根适配 `NewActiveChecker(svc user.Service)`（仿 `userProviderAdapter` 模式，`router.go:21-44`）。
- 依据：S1 跨包经接口纪律（moderation/user 域均不 import 他包，见 `moderation/service.go:1-3`）；中间件是 HTTP 层，经 seam 注入查询能力。

### 5.4 路由重排（`backend/internal/httpapi/router.go`）

```
authed := api.Group(""); authed.Use(middleware.Auth(jwtMgr))
  authed.POST /auth/logout                      // 豁免 RequireActive（登出放行）
  authed.GET  /auth/me                          // 豁免（读）
  authed.GET  /favorites /follows /notifications /notifications/unread_count   // 读，不拦

writers := authed.Group(""); writers.Use(middleware.RequireActive(NewActiveChecker(userSvc)))
  writers.POST/DELETE：/posts /comments /likes /favorites /follows
  writers.POST：/notifications/:id/read /notifications/read-all /reports /uploads

mod := writers.Group(""); mod.Use(middleware.RequireRole("moderator", "admin"))
  mod.POST /posts/:id/pin /feature；mod.GET/POST /moderation/reports*

admin := writers.Group(""); admin.Use(middleware.RequireRole("admin"))
  admin.POST /moderation/users/:id/ban    // #34 新增
  admin.POST /moderation/users/:id/unban  // #34 新增
```

- 依据：D3（全部登录写操作，读不禁）；`/auth/logout`/`/auth/me` 豁免（账户生命周期接口）；mod/admin 作为 writers 子组 → 被封用户连管理操作也被拦（D1 全禁的一致性）。

### 5.5 ban/unban handler（`backend/internal/httpapi/handler/moderation.go`）

- `NewModerationHandler(svc moderation.Service, users user.Service)`（签名加 users；`router.go:60` 装配改）。
- `BanUser`/`UnbanUser`：`Identity` → `pathID` → bind `{reason}`（必填 ≤500）→ **`users.Ban/Unban`（状态先改）** → 成功 → **`svc.RecordAction`（审计后写）** → 200。
- **审计失败语义【决策】**：`slog.Error` + 返回 **500 `{"error":"账号状态已更新，但审计记录失败，请联系管理员","status_changed":true}`**——不吞错；`status_changed:true` 供前端区分渲染（F6：避免误判"封禁失败"、重试撞 409 的困惑）。理由与备选见 §6.1。
- 错误映射 `respondAdminError`：`ErrNotFound`→404；`ErrSelfBan`→400「不能封禁自己」；`ErrCannotBanAdmin`→400；`ErrAlreadyBanned`→409；`ErrNotBanned`→409；其余→500。
- 依据：跨包聚合在 handler 层 = SOP §5.8 既有模式（follow.go）；`OperatorRole`/`HandlerID` 注入先例（#33 F3）。

### 5.6 profile 封禁状态（`backend/internal/httpapi/handler/profile.go`）

- `profileResp` 增 `Banned *bool \`json:"banned,omitempty"\``；`GetUserProfile` **显式读 `claims.Role`**（不只 viewerID，评审 Q5 附带 2）取 viewerRole；**`viewerRole=="admin"` 或 `viewerID==targetID`（self-view）时填充**（评审 Q5 附带 1：被封用户凭旧 token 看自己主页也能看到被封徽标）。
- 依据【决策】：治理信息不公开给游客/普通用户（防社工、隐私）；IA §5.5 只要求 admin 有封禁/解封入口；self-view 属本人状态，不涉隐私。服务层始终计算（深模块），序列化门控在 handler。

### 5.7 前端（`frontend/src/`）

1. `api/types.ts`：`UserProfile` 加 `banned?: boolean`（仅 admin 响应含）。
2. `api/report.ts`：`banUser(id, reason)` / `unbanUser(id, reason)` → `POST /moderation/users/:id/ban|unban`，`{requireAuth:true}`。
3. `api/client.ts`：**会话重置钩子（F1）**——`export function setAuthFailureHandler(fn)` 注册回调；`stores/auth.ts` store 外注册 `() => useAuthStore().clear()`；`client.ts` 的 **401 分支与 403(`data.code==='account_banned'`) 分支统一调用钩子**（消灭现有 401 只在 `fetchMe` catch 清 store 的潜伏不一致）。403 匹配用 **`code` 字段**（F5），普通 403「权限不足」不清 token（防止 mod 误调 admin 接口被登出）。
4. `views/UserProfile.vue`：
   - pacts 区（`!isSelf && auth.isAdmin`）加「封禁/解封」按钮（按 `profile.banned` 切换文案），弹原因输入（maxlength=500）→ `banUser`/`unbanUser` → 成功翻转 `banned` + toast。
   - `profile.banned` 时显示「该用户已封禁」徽标。
   - pacts 区加「举报」按钮 → 复用 `<ReportSheet target-type="user" :target-id="id" />`（未登录 `goLoginWithReturn`）。
5. `views/Reports.vue` 不改（§4 范围收敛）。

### 5.8 接口契约文档同步（评审 F3/F4）

- **README（F3）**：`backend/moderation/README.md`（范围加 `RecordAction`、不变量加 ban/unban 审计、HTTP API 面加 2 端点、错误表）+ `backend/user/README.md`（`Ban` 语义改造、新哨兵 `ErrSelfBan/ErrCannotBanAdmin/ErrAlreadyBanned/ErrNotBanned`、`Unban`/`IsActive`/`PublicProfileView.Banned`）同步更新。深模块规范：README 是外部调用方必读的接口契约（§0）。
- **决策记录（F4）**：本文件即为 #34 决策记录归档（D1-D6 见 §3、IA 偏离见 §4、D3 点明与 F7 限制见 §10 附、对账 SQL 见 §10 附）。

---

## 6. 关键设计裁决（【决策】，含理由与备选）

### 6.1 跨包原子性：状态先改、审计后写

- **问题**：状态变更（user 域）与审计写入（moderation 域）分属两个域，各自持私有 Repo 事务，**无法跨包单事务**（S1 纪律：不引入共享 tx 协调器）。两者非原子，必须定序。
- **定案【决策】**：`users.Ban/Unban`（状态）→ `moderation.RecordAction`（审计）；审计失败 → `slog.Error` + 500 + 前端明示「审计记录失败」。
- **理由**：
  - 状态是功能真相；`SetBanned/SetActive` 条件更新幂等 → 重试/双点击第二个请求 `ErrAlreadyBanned/ErrNotBanned` → 409，**不产生重复审计**。
  - 反向（审计先写）的双击/重试会**永久污染审计日志**（每条重试多一行 `ban_user`），且状态失败时残留"假审计"——比状态先改的罕见"审计缺口"更糟。
  - 审计写入失败（同机 PG INSERT）概率近零；即便发生，**显式 500 + 文案**告知 admin（非静默），缺口可经 DB 查 `moderation_actions` 对账补录。
  - IA §10「不得以『仅改 users.status』方式绕过审计」：本方案**总是**尝试审计并在失败时显式报告，非"仅改 status"的静默路径，精神满足。【决策】
- **备选（不选）**：审计先写再改状态（避免审计缺口但污染日志）；组合根跨域共享事务（需破坏 S1 或大改装配，收益低）。

### 6.2 拦截响应 403 vs 401

- **定案【决策】**：被封写拦截返回 **403 `code="account_banned"`**（区别于 401 未登录/登录过期）。
- **理由**：已认证但不可用 = 授权/状态问题而非认证问题；前端 `client.ts` 匹配 `code` 清 token（普通 403「权限不足」不清）。登录接口维持现状 401 同文案（防枚举，§2.3）。

### 6.3 `banned` 可见性

- **定案【决策】**：`PublicProfileView.Banned` 服务层计算，handler 门控——`viewerRole=="admin"` **或 self-view（viewerID==targetID）** 时序列化。
- **理由**：治理信息不公开（防社工）；self-view 属本人状态；非 admin/非本人不返回。

### 6.4 条件更新 vs 读-改-写

- **定案【决策】**：状态变更用单条条件 UPDATE（`WHERE status=...`），不用"读状态→判断→写"。
- **理由**：并发双 admin 同时 ban 时，条件更新 RowsAffected=0 原子地保证仅一方成功（#33 F4 同款）；读-改-写有竞态窗口。

---

## 7. 边界与不变量清单（含防护层）

| # | 不变量 | 防护层 | 依据 |
|---|---|---|---|
| 1 | 不能封自己 | `Ban` 自封守卫→400 | §5.1 |
| 2 | 不能封 admin | `Ban` 角色守卫→400 | §5.1 |
| 3 | 重复 ban 不重复审计 | 条件更新 `WHERE status='active'`→`ErrAlreadyBanned`(409) | §5.1/§6.4 |
| 4 | 重复 unban 不重复审计 | 条件更新 `WHERE status='banned'`→`ErrNotBanned`(409) | §5.1/§6.4 |
| 5 | 目标不存在→404 | `GetUserByID`→`ErrNotFound` | §5.1 |
| 6 | 审计缺失防护 | 状态先改审计后写；失败 slog.Error+500 `status_changed:true`+明示（不吞错），对账 SQL 见 §10 附 | §6.1/§5.5 |
| 7 | 写拦截不拦 `/auth/me`、`/auth/logout` | 两者留 authed，不进 writers | §5.4 |
| 8 | moderator 被封后管理操作也被拦 | mod 组在 writers 下 | §5.4 |
| 9 | admin 被封后 ban/unban 也被拦 | admin 组在 writers 下 | §5.4 |
| 10 | 登出放行 | logout 豁免 RequireActive；JWT 无状态 | §5.4 |
| 11 | 登录拒绝 | `Login` 已拦 banned→`ErrBanned`→401 同文案 | 已有（§2.3） |
| 12 | reason 必填且 ≤500 | `RecordAction` 服务端校验 + 前端必填/maxlength=500 | §5.2 |
| 13 | 不能经 HandleReport 绕过封禁 | `validAction` 仍拒 ban/unban | §4 |
| 14 | `banned` 仅 admin/self 可见 | handler 门控 `viewerRole=="admin" \|\| viewerID==targetID` | §5.6 |
| 15 | 零迁移 | 复用 `user.users.status` + `moderation_actions` | §2.2 |
| 16 | RequireActive DB 查询失败 fail-closed | checker error→500 | §5.3 |
| 17 | 游客在 RequireActive 下→401 | `Identity` 缺失 | §5.3 |
| 18 | 软删用户被拦 | `GetUserByID` 过滤软删→`IsActive=false`→403 | §5.1 |
| 19 | 并发双 admin 同时 ban | 条件更新 RowsAffected=0→仅一方成功+一方 409 | §6.4 |
| 20 | 普通 403「权限不足」不清前端 token | client.ts 匹配 `code==='account_banned'` 才清 | §5.7 |
| 21 | 举报用户入口复用 | UserProfile 加 ReportSheet（target_type=user） | §5.7 |
| 22 | 已知缺口：审计写入失败罕见窗口 → 对账 SQL 补录 | 文档记录 | §6.1/§10 附 |

---

## 8. 测试与验证计划

- **user 单测**（`user/service_test.go` + fake_repo 补 `SetBanned/SetActive`）：Ban 成功/不存在/自封/封 admin/重复 ban→ErrAlreadyBanned；Unban 成功/不存在/未封；IsActive 三态（active/banned/软删）；Login banned 拒绝（回归）。
- **moderation 单测**：`RecordAction` 成功（ban/unban）/非白名单拒绝/reason 空或>500；回归 `HandleReport` 拒 `ActionBan`（已有用例）。
- **middleware 单测**（fake ActiveChecker）：无 token→401；active→next；inactive→403；error→500。
- **handler 集成**（真 PG，新建 `admin_test.go`，仿 `moderation_test.go` 模式 + helpers `makeAdmin`/`makeBanned` 直接 UPDATE）：admin ban→200+status=banned+审计 `ban_user` 行；unban→active+`unban_user` 行；普通 user/moderator→403；被封旧 token `POST /posts`→403；被封登录→401；豁免 `/auth/me`、`/auth/logout`→200；边界：自封/封 admin/重复 ban/未封 unban/reason 缺失或>500；profile `banned` 仅 admin 可见（断言 body）。
- **前端单测**：`report.test.ts` 增 banUser/unbanUser；`UserProfile.test.ts`（新建，仿 `PostDetail.test.ts` mock 模式）：非 admin 无封禁按钮、admin 封禁/解封切换、提交调 API、举报入口开 ReportSheet；**auth store mock 需含 `clear`**（F8）；**`client.ts` 补 403(`code='account_banned'`) 触发会话重置断言**（fake 钩子被调、`isLoggedIn` 变 false，F1/F8）。
- **契约文档**：README 与决策记录是否随 PR 提交（F3/F4）列入提交前自检。
- **本地全量**：`cd backend && go build ./... && go vet ./... && go test ./...`；`cd frontend && npm run test:unit && npm run build`。
- **提交前**：`git checkout -- frontend/components.d.ts`（SOP §4.5）。

---

## 9. 待评审焦点（Q1-Q8，已评审裁决）

> 已由独立评审 agent 逐条裁决（2026-08-21），裁决全部采纳，落地见 §10。Q1 认可（附 3 条件）/ Q2 认可无漏挂 / Q3 认可（附 TOCTOU 注释）/ Q4 认可自洽 / Q5 认可（附 2 建议）/ Q6 裁决有条件认可本票不做（须登记偏离 + 用户确认，已完成）/ Q7 裁决为 F1 失效缺陷（已修复）/ Q8 认可（附前置：开工第 0 步 psql 重查线上枚举，已完成）。

---

## 10. 评审意见采纳记录（2026-08-21）

| 评审项 | 结论 | 采纳落地 |
|---|---|---|
| **F1** 封禁登出失效（`setToken(null)` 不清 Pinia store） | 重要，属实（已二次验证 `auth.ts:12/39-40`、`router/index.ts:27`） | client.ts 会话重置钩子 + auth store 注册 `clear()`，401/403-banned 统一走钩子（§5.7.3、§8） |
| **F2** IA §5.6 弹窗封禁动作偏离未成记录 | 重要，属实 | 偏离登记进决策记录（§4）+ **用户显式确认（2026-08-21）** + 合并后回写 IA §5.6 |
| **F3** 两域 Interface README 未列入改动 | 重要，属实 | §5.8 增 README 同步（moderation/user），§8 加自检项 |
| **F4** #34 grilling 决策记录未入库 | 重要，属实 | 本文件即为决策记录归档，随 PR 入库 |
| **F5** 中文文案精确匹配脆弱 | 建议，采纳 | 403 响应加 `code:"account_banned"`，前端匹配 code（§5.3、§5.7） |
| **F6** 审计失败 500 语义歧义 | 建议，采纳 | 500 + `status_changed:true` + 明示文案（§5.5、§7#6） |
| **F7** 封禁原因无处可见 | 建议，采纳 | 决策记录写明已知限制（§10 附）；后续 ticket `GET /moderation/actions` |
| **F8** 前端单测 mock 缺 `clear` | 建议，采纳 | §8 增 client.ts 403 会话重置断言 + auth mock 含 `clear` |
| **Q1 条件 1-3** | 认可附条件 | ①`status_changed` ②对账 SQL（§10 附）③handler/RecordAction reason 校验同源（§5.2、§5.5） |
| **Q3 附带** | 认可 | `SetBanned/SetActive` RowsAffected=0 不区分「已封/不存在」→ repo 注释点明（§5.1） |
| **Q5 附带** | 认可 | profile handler 显式读 `claims.Role`；self-view 也填充 `banned`（§5.6） |
| **D3 附带** | 认可 | 决策记录写清「`POST /reports` 被拦」「被封 mod 无法查看举报队列」（§10 附） |
| **Q8 前置** | 认可 | 开工第 0 步 psql 重查线上 `moderation_actions` 枚举（§2.2，已完成） |

**推翻项**：无。全部评审发现经独立复核属实或合理，无一处不合理。

---

## 附：决策记录要点（评审 F4/D3/F7、Q1 条件 2 落地）

- **D3 点明（全禁的边界确认）**：
  - `POST /reports`（举报创建）在 writers 组 → **被封用户不能举报**（符合「全禁」，接受）。
  - `mod` 组 `GET /moderation/reports*`（举报队列读）随组落入 writers → **被封 moderator 无法查看举报队列**（符合「全禁」，接受）。
- **F7 已知限制（ban 原因可见性）**：零迁移下 ban 原因只存 `moderation_actions`，无查询端点，UI 不展示「为何被封」。MVP 接受；后续 ticket 加只读 `GET /moderation/actions?target_id=`（按 target 查审计历史）。
- **Q1 条件 2 对账 SQL（审计缺口补录形态）**：若出现「状态已改、审计缺失」的罕见窗口（handler 返回 `status_changed:true` 的 500），admin 按以下 SQL 自查目标用户的治理动作，确认缺口后人工补 `INSERT INTO moderation.moderation_actions (moderator_id, action, target_type, target_id, reason, created_at) VALUES (...)`（人工补录时 reason 注明"人工补录"）：

```sql
-- 查某用户（:target_id）的治理动作历史，定位审计缺口
SELECT id, moderator_id, action, target_type, target_id, reason, created_at
FROM moderation.moderation_actions
WHERE target_type = 'user' AND target_id = :target_id
ORDER BY created_at DESC;
```
