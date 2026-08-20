# Package content — 内容域深模块接口（Interface README）

> 依据：#4 服务候选边界、#5 数据模型、#7 深模块规范、IA v2 §5.2/§5.3、#9 §5.0 权限矩阵。
> **状态：已实现（2026-08-18）**。服务、GORM repo、HTTP 端点与测试就位；通知依赖延迟，见「遗留」。

## 范围

对外提供 `Service` 接口（粗粒度命令 + 读模型查询）。`Repo` 为实现内部 seam，调用方不应使用；测试跨 `Service` 验证（#7 接口即测试面）。

## 不变量（Invariants）

- **发帖必选板块**（`ErrBoardNotFound`），话题可选且必须从属该板块（`ErrTopicNotInBoard`）。
- **评论树为邻接表**：`parent_id` 指向父评论，回复必须属于同一帖子（`ErrParentNotInPost`）；`floor` 仅顶层 1..N 自动分配；软删评论**留占位节点**（`Deleted=true`、回复链保留），楼层不回收。
- **计数无冗余列**（#5）：like/comment/favorite 计数均读侧聚合；`posts.view_count` 于 GET 详情时自增。
- **关注单向**（#5 D4）：关注板块/话题防重复（`ErrAlreadyFollowed`）；取关幂等。
- **点赞/收藏去重**：重复返回 `ErrAlreadyLiked`/`ErrAlreadyFavorited`；取消幂等。
- **收藏/关注列表按关系时间倒序**（#23）：`favorites`/`follows_*.created_at` DESC，分页 offset/limit；软删目标自动排除；游客一律 `ErrAuthRequired`。
- **角色映射**：`OperatorRole` 来自 JWT（已由 user 边界映射为 user/moderator/admin）。

## 调用顺序约束（Ordering）

- 命令独立，无强制顺序。删除/置顶/精华须先有帖子/评论。
- `GetCommentTree` 一次深接口调用完成「查整帖 + 内存拼树」，调用方不得分步查询。
- **关注流（tab=follow）**：聚合关注用户（经 `UserProvider` 进程内调用 user 包）+ 关注板块/话题（本包 repo）；游客请求返回 `ErrAuthRequired`。
- **我的收藏/关注列表（#23）**：`ListFavorites`（收藏的帖子）/`ListFollowedBoards`/`ListFollowedTopics`，均需登录、按关系时间倒序 + 分页；返回带 `viewer.following/favorited` 初始态。

## 错误模式（Errors）

| 哨兵错误 | 语义 | 建议 HTTP |
|---|---|---|
| `ErrPostNotFound` / `ErrCommentNotFound` / `ErrBoardNotFound` / `ErrTopicNotFound` | 目标不存在（含软删） | 404 |
| `ErrForbidden` | 非作者且非 moderator+ 删帖/评论；非 moderator+ 置顶/精华 | 403 |
| `ErrAlreadyLiked` / `ErrAlreadyFavorited` / `ErrAlreadyFollowed` | 重复操作 | 409 |
| `ErrTopicNotInBoard` / `ErrParentNotInPost` / `ErrContentEmpty` / `ErrInvalidTargetType` / `ErrInvalidFeedTab` | 参数非法 | 400 |
| `ErrAuthRequired` | 关注流 / 收藏与关注列表游客 | 401 |

> 其余 error 为基础设施故障，调用方按 500 处理。权限校验在命令内（#9 §5.0 双保险），前端可见性仅是体验层。

## 必需配置（Required config）

- `Repo` 依赖注入（`NewGormRepo`），需 schema `content` 已迁移（0001~0004）+ 种子数据（0005，6 板块/10 话题）。
- `UserProvider` 依赖（`NewService(repo, users)`）：作者画像与关注用户数据；由 httpapi 组合根 `NewUserProvider(userSvc)` 适配 user 服务（content 不 import user）。

## 性能特征（Performance）

- `ListFeed` 读模型：1 主查询 + 3 分组 COUNT + 作者/板块/话题 enrich + 登录态成员查询，O(页长) 独立于全表。
- **收藏/关注列表（#23）**：JOIN 关系表按 `created_at` 排序，单查询 + `postViews` 复用；显式 `Select(主表.*)` 防 JOIN 列遮蔽（评审 D1）。
- `GetCommentTree` 单次整帖查询，O(评论数) 内存拼树。50 人规模零压力，不做 scale 索引（#5）。
- 分页 `limit/offset`（IA v2 §10：MVP 简化，前端无限滚动）。

## 遗留（#7 单 Adapter 原则，待 notify 包落地）

- **通知延迟**：like/comment/reply 的通知未实现——notify 尚为骨架，现在引入 `Notifier` 只会是 no-op 单实现（假设性 seam）。待 notify 包落地时给 content 增加 `Notifier` 依赖并异步发出。
- **排序简化**：feed「置顶优先」按 `is_pinned DESC, created_at DESC`（#5 无 pinned_at 列，置顶帖按创建时间排序；IA v2 §5.2「按操作时间」待有操作时间列时再实现）。
