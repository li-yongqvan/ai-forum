# Package content — 内容域深模块接口（Interface README）

> 依据：#4 服务候选边界、#5 数据模型、#7 深模块规范、IA v2 §5.2/§5.3、#9 §5.0 权限矩阵。
> **状态：已实现（2026-08-18；#54 标签 2026-08-24；#78 搜索词检索 2026-09-26）**。服务、GORM repo、HTTP 端点与测试就位；通知依赖延迟，见「遗留」。

## 范围

对外提供 `Service` 接口（粗粒度命令 + 读模型查询）。`Repo` 为实现内部 seam，调用方不应使用；测试跨 `Service` 验证（#7 接口即测试面）。

## 不变量（Invariants）

- **发帖必选板块**（`ErrBoardNotFound`），话题可选且必须从属该板块（`ErrTopicNotInBoard`）。
- **评论树为邻接表**：`parent_id` 指向父评论，回复必须属于同一帖子（`ErrParentNotInPost`）；`floor` 仅顶层 1..N 自动分配；软删评论**留占位节点**（`Deleted=true`、回复链保留），楼层不回收。
- **计数无冗余列**（#5）：like/comment/favorite 计数均读侧聚合；`posts.view_count` 于 GET 详情时自增。
- **关注单向**（#5 D4）：关注板块/话题防重复（`ErrAlreadyFollowed`）；取关幂等。
- **点赞/收藏去重**：重复返回 `ErrAlreadyLiked`/`ErrAlreadyFavorited`；取消幂等。
- **收藏/关注列表按关系时间倒序**（#23）：`favorites`/`follows_*.created_at` DESC，分页 offset/limit；软删目标自动排除；游客一律 `ErrAuthRequired`。
- **标签（#54）**：发帖时解析正文 `#关键词` 落库（`content.tags` + `content.post_tags`）。标签名**归一化小写**、≤30 字符、大小写不敏感（`#AI`=`#ai`）；`PostView.Tags` 只读、无则省略。**标签写失败不阻断发帖**（slog 记录）；`postViews` 的标签富化失败**仅缺省不报错**（best-effort）。按标签聚合：`ListFeedQuery.Tag` 过滤 + `GET /posts?tag=`，软删帖自动排除。
- **搜索词（#78）**：`ListFeedQuery.Query` 收**原样字符串**，归一化在 service 层（`NormalizeQuery`），调用方（handler）不校验、不切词、不转义。契约：
  - 按空白切词，词间 **AND**；每词须**标题或正文**含之（`(title ILIKE ? OR content ILIKE ?)`）。
  - **不区分大小写**（`ILIKE` 语义），**按字面匹配**——`\` `%` `_` 先转义再进 pattern，故搜 `50%` 不会被当成前缀通配。中文按**子串**召回（这是选 `ILIKE` 而非 `to_tsvector` 的唯一原因：内置分词对整串中文只切一个 token，搜「论坛」召回 0）。
  - 长度 **2..64 码点**、词数 **≤4**，越界一律 `ErrInvalidQuery`，**不静默截断**。
  - **无相关性打分**：排序 =「标题含全部词」者优先 → `created_at` DESC，且搜索态**置顶不参与**排序（非搜索态仍 `is_pinned DESC` 优先）。繁简、变音符号不归一。
  - **治理与全站同源**：走同一个查询构造器与同一个 GORM 软删 scope ⇒ 软删帖不召回、封禁/注销作者内容照常召回，搜索**不新增**任何治理谓词。
  - **仅 `q` 非空时**才调 `CountFeed` 并在响应里给 `total`；`total` 与 `items` 由同一 `PostQuery` 构造器保证同条件，且**不参与翻页**。
  - 术语：本域称**搜索词（query）**，与「标签 `#hashtag`」「话题」是三件事，不可混称。
- **角色映射**：`OperatorRole` 来自 JWT（已由 user 边界映射为 user/moderator/admin）。

## 调用顺序约束（Ordering）

- 命令独立，无强制顺序。删除/置顶/精华须先有帖子/评论。
- `GetCommentTree` 一次深接口调用完成「查整帖 + 内存拼树」，调用方不得分步查询。
- **关注流（tab=follow）**：聚合关注用户（经 `UserProvider` 进程内调用 user 包）+ 关注板块/话题（本包 repo）；游客请求返回 `ErrAuthRequired`。
- **我的收藏/关注列表（#23）**：`ListFavorites`（收藏的帖子）/`ListFollowedBoards`/`ListFollowedTopics`，均需登录、按关系时间倒序 + 分页；返回带 `viewer.following/favorited` 初始态。
- **发帖落标签（#54）**：`CreatePost` 内解析 `content` 提取标签并写入关联；`ReplacePostTags` 为幂等替换（删旧→upsert→插新，事务），写失败仅记录不阻断发帖。标签聚合过滤归一化在 service 层兜底（调用方传入即可）。
- **搜索结果条数（#78）**：`CountFeed` 与 `ListFeed` 必须传**同一个** `ListFeedQuery`（内部共用 `buildFeedQuery`）；只数不取行，忽略分页。调用方仅在 `Query` 非空时调它。

## 错误模式（Errors）

| 哨兵错误 | 语义 | 建议 HTTP |
|---|---|---|
| `ErrPostNotFound` / `ErrCommentNotFound` / `ErrBoardNotFound` / `ErrTopicNotFound` | 目标不存在（含软删） | 404 |
| `ErrForbidden` | 非作者且非 moderator+ 删帖/评论；非 moderator+ 置顶/精华 | 403 |
| `ErrAlreadyLiked` / `ErrAlreadyFavorited` / `ErrAlreadyFollowed` | 重复操作 | 409 |
| `ErrTopicNotInBoard` / `ErrParentNotInPost` / `ErrContentEmpty` / `ErrInvalidTargetType` / `ErrInvalidFeedTab` / `ErrInvalidQuery` | 参数非法（`ErrInvalidQuery` = #78 搜索词长度不在 2..64 或词数 >4） | 400 |
| `ErrAuthRequired` | 关注流 / 收藏与关注列表游客 | 401 |

> 其余 error 为基础设施故障，调用方按 500 处理。权限校验在命令内（#9 §5.0 双保险），前端可见性仅是体验层。

## 必需配置（Required config）

- `Repo` 依赖注入（`NewGormRepo`），需 schema `content` 已迁移（0001~0004）+ 种子数据（0005，6 板块/10 话题）。
- `UserProvider` 依赖（`NewService(repo, users)`）：作者画像与关注用户数据；由 httpapi 组合根 `NewUserProvider(userSvc)` 适配 user 服务（content 不 import user）。

## 性能特征（Performance）

- `ListFeed` 读模型：1 主查询 + 3 分组 COUNT + 作者/板块/话题 enrich + 登录态成员查询，O(页长) 独立于全表。
- **收藏/关注列表（#23）**：JOIN 关系表按 `created_at` 排序，单查询 + `postViews` 复用；显式 `Select(主表.*)` 防 JOIN 列遮蔽（评审 D1）。
- **标签（#54）**：聚合过滤用 `id IN (子查询 post_tags JOIN tags)`（主查询无 JOIN，免列遮蔽；索引路径 `tags.name` UNIQUE → `post_tags.tag_id` → `post_tags.post_id`）；读模型富化批量 `ListTagsByPostIDs` 一次带出。
- `GetCommentTree` 单次整帖查询，O(评论数) 内存拼树。50 人规模零压力，不做 scale 索引（#5）。
- 分页 `limit/offset`（IA v2 §10：MVP 简化，前端无限滚动）。
- **搜索词（#78）**：`ILIKE '%词%'` 前置通配 ⇒ **顺序扫描** `content.posts`，成本随行数**线性**增长，且搜索态多一次同条件的 `CountFeed`（第二次扫描）。零索引、零扩展、零迁移（D7：`pg_trgm` 的 gin 索引在「`ILIKE` + `ORDER BY … LIMIT`」形态下不被规划器选中，建了也不提速）。触发重估的条件：帖子量到数万级且首屏可感知变慢——见设计文档 §9-2 的处置路径，勿在本包内预埋。

## 遗留（#7 单 Adapter 原则，待 notify 包落地）

- **通知延迟**：like/comment/reply 的通知未实现——notify 尚为骨架，现在引入 `Notifier` 只会是 no-op 单实现（假设性 seam）。待 notify 包落地时给 content 增加 `Notifier` 依赖并异步发出。
- **排序简化**：feed「置顶优先」按 `is_pinned DESC, created_at DESC`（#5 无 pinned_at 列，置顶帖按创建时间排序；IA v2 §5.2「按操作时间」待有操作时间列时再实现）。
