# Package moderation — 治理域深模块接口（Interface README）

> 依据：#4 服务候选边界、#5 D6（治理最小模型）、#7 深模块规范、IA v2 §5.6 治理闭环。
> **状态：接口骨架。** 实现（service/gorm_repo）留待后续 ticket。

## 范围

对外提供 `Service` 接口。治理闭环：举报（帖子/评论/用户）→ 处理（`/reports` 最小闭环）→ append 审计 → `report_result` 通知。

## 不变量（Invariants）

- **审计留痕**：处理动作中的删内容/警告/封禁/解封**必须**追加 `moderation_actions`（append-only，仅 created_at 写死不可改）。`dismiss`（忽略）只改 report 状态，不落审计。
- **解封必须审计**：`unban_user` 在 `moderation_actions.action` 枚举内（下游依赖#1，已固化于迁移 0004）；不得以「仅改 users.status」绕过审计。
- **防重复举报**：同一举报者对同一目标（同类型）在 **pending 期**内只允许一条（DB 部分唯一索引 `WHERE status='pending'` 兜底）；已结案（resolved/dismissed）后可再举报（IA v2 §5.6）。
- **`reports`/`moderation_actions` 永不删除**（审计）。
- 状态枚举：`pending/resolved/dismissed`（一致性修正：以 IA v2 为准）。

## 调用顺序约束（Ordering）

- 处理举报需先有 `CreateReport` 落库。
- `HandleReport` 在同一事务内完成「改状态 + 追加审计」（跨内容删除需与 content/user 服务协作，实现 ticket 内约定事务边界）。

## 错误模式（Errors）

| 哨兵错误 | 语义 | 建议 HTTP |
|---|---|---|
| `ErrNotFound` | 举报不存在 | 404 |
| `ErrDuplicatePending` | pending 期内重复举报 | 409 |
| `ErrInvalidAction` | 非法处理动作/权限越界 | 400 / 403 |

> 其余 error 为基础设施故障，调用方按 500 处理。

## 必需配置（Required config）

- `Repo` 依赖注入；真实实现需 schema `moderation` 已迁移。

## 性能特征（Performance）

- 处理为单事务内 1–2 条写入；举报列表按 `status` 索引查询。50 人规模零压力，不做 scale 索引（#5）。
