# Package moderation — 治理域深模块接口（Interface README）

> 依据：#4 服务候选边界、#5 D6（治理最小模型）、#7 深模块规范、IA v2 §5.6 治理闭环。
> **状态：#33 已实现。** service + gorm_repo + httpapi 路由已落地（2026-08-21）。

## 范围

对外提供 `Service` 接口：`CreateReport` / `HandleReport` / `ListReports` / `CountReports`。治理闭环：举报（帖子/评论/用户）→ 处理（`/reports` 最小闭环）→ append 审计 → `report_result` 通知（只发举报人，#33 D2）。

本包**不 import content/user/notify**，跨包能力经 seam 注入：`ContentGateway`（`ResolveTarget`/`DeletePost`/`DeleteComment`）、`UserGateway`（`GetUserView`）、`Notifier`（`CreateNotification`）——组合根（httpapi）装配 adapter（S1）。

## 不变量（Invariants）

- **审计留痕**：删内容/警告**必须**追加 `moderation_actions`（append-only，仅 created_at 写死不可改）。**`dismiss`（忽略）只改 report 状态、不落审计**（F2 裁决：留痕由 `reports.handler_id/handled_at/handling_note` 承载；IA §5.6 字面「所有动作→append」与此冲突，以 README 为准）。
- **封禁留 #34**：`ban_user/unban_user` 动作枚举已固化（迁移 0004），但本票 `validAction` 服务端拒绝，不启用。
- **防重复举报**：同一举报者对同一目标（同类型）在 **pending 期**内只允许一条（DB 部分唯一索引 `uq_reports_pending` 兜底 + 服务层 `PendingExists` 预检）；已结案后可再举报。
- **`reports`/`moderation_actions` 永不删除**（审计）。
- 状态枚举：`pending/resolved/dismissed`。
- **`reason` 只存六枚举之一**（D4/O2），举报人备注独立入 `reporter_note` 列（迁移 0006）。
- **并发双处理防护**：`UpdateReportStatus` 条件更新 `WHERE status='pending'`，`RowsAffected=0` → `ErrNotFound`（F4）。

## 调用顺序约束（Ordering）

- 处理举报需先有 `CreateReport` 落库。
- `HandleReport`（O1 序列）：**事务外**执行内容网关副作用（删除/解析作者）→ **单事务**内「append 审计 + 改状态」→ **提交后事务外**发 `report_result` 通知（失败打日志不回滚，S3）。
- 删除动作不先 `ResolveTarget`：adapter 把「目标不存在/已删」映射为成功，保证崩溃窗口（已删未结）可幂等重处理（O1-②；副作用：作者先自删时审计记 `delete_post` 归属处理人，MVP 精度已文档化接受）。

## 错误模式（Errors）

| 哨兵错误 | 语义 | 建议 HTTP |
|---|---|---|
| `ErrNotFound` | 举报或目标不存在 | 404 |
| `ErrDuplicatePending` | pending 期内重复举报 | 409 |
| `ErrInvalidAction` | 非法处理动作/权限越界/自举报 | 400 |

> 其余 error 为基础设施故障，调用方按 500 处理。

## 必需配置（Required config）

- `Repo` 依赖注入；schema `moderation` 已迁移（含 0006 `reporter_note` 列）。
- `ContentGateway`/`UserGateway`/`Notifier` 由组合根注入（`httpapi.NewContentGateway/NewUserGateway/NewNotifier`）。

## HTTP API 面（#33，S7 补全）

| 方法 | 路径 | 组/权限 | 请求 | 成功响应 |
|---|---|---|---|---|
| POST | `/api/v1/reports` | authed（登录墙） | `{target_type, target_id, reason, note?}`（reason 六枚举） | 201 `{"id": <举报id>, "message":"ok"}` |
| GET | `/api/v1/moderation/reports?status=&page=&page_size=` | mod（`RequireRole("moderator","admin")`） | query 缺省 status=pending, page=1, page_size=20（服务端钳 ≤100） | 200 `{"items":[ReportView],"page":N,"page_size":N}` |
| GET | `/api/v1/moderation/reports/count?status=` | mod | — | 200 `{"count": N}` |
| POST | `/api/v1/moderation/reports/:id/handle` | mod | `{action, note?}`（note ≤500；action ∈ dismiss/delete_post/delete_comment/warn） | 200 `{"message":"ok"}` |

`ReportView` 含 enrich 字段 `reporter_username`/`target_title`（best-effort，目标已删时缺省）；`reason` 只存枚举，举报人备注在 `reporter_note`。

## 性能特征（Performance）

- 处理为单事务内 1–2 条写入；举报列表按 `status` 索引查询。
- `ListReports` 逐行 enrich（举报人用户名 + 目标标题）为 **N+1** 读（pageSize ≤100，50 人规模零压力；F8-② 记录）。
