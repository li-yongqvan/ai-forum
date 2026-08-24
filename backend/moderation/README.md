# Package moderation — 治理域深模块接口（Interface README）

> 依据：#4 服务候选边界、#5 D6（治理最小模型）、#7 深模块规范、IA v2 §5.6 治理闭环。
> **状态：#33/#53 已实现。** service + gorm_repo + httpapi 路由已落地（2026-08-21）。

## 范围

对外提供 `Service` 接口：`CreateReport` / `HandleReport` / `ListReports` / `CountReports` / `RecordAction`。治理闭环：举报（帖子/评论/用户）→ 处理（`/reports` 最小闭环）→ append 审计 → 双通知（`report_result` 给举报人 + `report_handled` 给被举报人，仅 delete/warn，#53）。**#34 直接治理动作（ban/unban）经 `RecordAction` 独立追加审计**（不经过举报流程）。

本包**不 import content/user/notify**，跨包能力经 seam 注入：`ContentGateway`（`ResolveTarget`/`DeletePost`/`DeleteComment`）、`UserGateway`（`GetUserView`）、`Notifier`（`CreateNotification`）——组合根（httpapi）装配 adapter（S1）。

## 不变量（Invariants）

- **审计留痕**：删内容/警告**必须**追加 `moderation_actions`（append-only，仅 created_at 写死不可改）。**`dismiss`（忽略）只改 report 状态、不落审计**（F2 裁决：留痕由 `reports.handler_id/handled_at/handling_note` 承载；IA §5.6 字面「所有动作→append」与此冲突，以 README 为准）。
- **封禁/解封审计（#34 已启用）**：直接治理动作经 `RecordAction` 追加 `moderation_actions`（`ban_user`/`unban_user`，reason 必填 ≤500，TargetType 恒 `user`）；`HandleReport` 的 `validAction` **仍拒绝** ban/unban（防经举报流程绕过 admin-only 权限门）。封禁状态变更由 user 域 `Ban/Unban` 承载（**状态先改、审计后写**，跨域非原子；审计失败由 handler 显式 500 + `status_changed:true` 报告并对账，见 `docs/handoffs/grilling-decisions/issue-34-governance-permissions-decisions.md`）。
- **防重复举报**：同一举报者对同一目标（同类型）在 **pending 期**内只允许一条（DB 部分唯一索引 `uq_reports_pending` 兜底 + 服务层 `PendingExists` 预检）；已结案后可再举报。
- **`reports`/`moderation_actions` 永不删除**（审计）。
- 状态枚举：`pending/resolved/dismissed`。
- **`reason` 只存六枚举之一**（D4/O2），举报人备注独立入 `reporter_note` 列（迁移 0006）。
- **并发双处理防护**：`UpdateReportStatus` 条件更新 `WHERE status='pending'`，`RowsAffected=0` → `ErrNotFound`（F4）。
- **被举报人触达（#53）**：处理 delete_post/delete_comment/warn 后，除举报人 `report_result` 外，再发 `report_handled` 给被举报人（作者 id 来自 `reports.target_author_id` 快照，0007）；**dismiss 不通知被举报人**。存量无快照（NULL）→ 跳过 + `slog.Warn`。
- **举报频控（#53，软限）**：`CreateReport` 时同一举报人 `reportWindow`（10 分钟）内 ≤`reportLimit`（5）次，超限 → `ErrRateLimited`（429，不落库）。窗口计数含已 dismissed 举报、count+insert 非原子（并发可少量超发）——MVP 接受，硬不变量仍是 `uq_reports_pending`。

## 调用顺序约束（Ordering）

- 处理举报需先有 `CreateReport` 落库。
- `HandleReport`（O1 序列）：**事务外**执行内容网关副作用（删除/解析作者）→ **单事务**内「append 审计 + 改状态」→ **提交后事务外**发双通知（`report_result` 举报人 + `report_handled` 被举报人，#53；各吞错打日志不回滚，S3）。
- 删除动作不先 `ResolveTarget`：adapter 把「目标不存在/已删」映射为成功，保证崩溃窗口（已删未结）可幂等重处理（O1-②；副作用：作者先自删时审计记 `delete_post` 归属处理人，MVP 精度已文档化接受）。

## 错误模式（Errors）

| 哨兵错误 | 语义 | 建议 HTTP |
|---|---|---|
| `ErrNotFound` | 举报或目标不存在 | 404 |
| `ErrDuplicatePending` | pending 期内重复举报 | 409 |
| `ErrInvalidAction` | 非法处理动作/权限越界/自举报 | 400 |
| `ErrRateLimited` | 举报频控超限（#53，10 分钟 ≤5 次） | 429 |

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
| POST | `/api/v1/moderation/users/:id/ban` | admin（`RequireRole("admin")`，#34） | `{reason}`（必填 ≤500；写 `ban_user` 审计） | 200 `{"message":"ok"}` |
| POST | `/api/v1/moderation/users/:id/unban` | admin（#34） | `{reason}`（必填 ≤500；写 `unban_user` 审计） | 200 `{"message":"ok"}` |

> ban/unban 端点的错误映射在 handler 层（`respondAdminError`，user 域哨兵）：404 用户不存在 / 400 不能封禁自己·不能封禁管理员·原因必填≤500 / 409 该用户已被封禁·未被封禁；审计写入失败 → 500 `{status_changed:true}`（状态已变、需对账，见决策记录）。

`ReportView` 含 enrich 字段 `reporter_username`/`target_title`（best-effort，目标已删时缺省）；`reason` 只存枚举，举报人备注在 `reporter_note`。

## 性能特征（Performance）

- 处理为单事务内 1–2 条写入；举报列表按 `status` 索引查询。
- `ListReports` 逐行 enrich（举报人用户名 + 目标标题）为 **N+1** 读（pageSize ≤100，50 人规模零压力；F8-② 记录）。
