# Package content — 内容域深模块接口（Interface README）

> 依据：#4 服务候选边界、#5 数据模型、#7 深模块规范、IA v2 §5.2。
> **状态：接口骨架。** 模型与接口已定义，实现（service/gorm_repo）留待后续 ticket。

## 范围

对外提供 `Service` 接口（粗粒度命令 + 读模型查询）。`Repo` 为实现内部 seam，调用方不应使用。

## 不变量（Invariants）

- **发帖必选板块**，话题可选且从属板块（IA v2 §5.3）。
- **点赞/收藏唯一**（DB 唯一约束兜底）；重复操作返回明确错误。
- **评论树为邻接表**：`parent_id` 指向父评论，`floor` 仅顶层 1..N；软删评论**留占位节点**，回复链保留（#5 D3）。
- **读侧冗余计数**：`posts` 无 like/comment 计数列，计数由调用方经查询聚合（50 人规模零压力，#5）。
- **无重定义**：置顶/精华状态存 `is_pinned`/`is_featured`，管理切换动作的审计由 moderation 包负责。

## 调用顺序约束（Ordering）

- 无强制顺序；`CreateComment` 的 `ParentID` 若指定，需先有目标评论存在。
- `GetCommentTree` 一次深接口调用完成「查整帖评论 + 内存拼树」，调用方不得自行分步查询拼树。

## 错误模式（Errors）

| 哨兵错误 | 语义 | 建议 HTTP |
|---|---|---|
| `ErrNotFound` | 帖子/评论/板块/话题不存在 | 404 |
| `ErrBoardRequired` | 发帖未选板块 | 400 |
| `ErrLikeExists` / `ErrFavoriteExists` | 重复点赞/收藏 | 409 |
| `ErrNotExists` | 取消不存在的点赞/收藏 | 404 |

> 哨兵错误常量随实现落地，本骨架仅约定形态。基础设施故障为其余 error，调用方按 500 处理。

## 必需配置（Required config）

- `Repo` 依赖注入（#7 接受依赖而非创建）；真实实现需 schema `content` 已迁移。

## 性能特征（Performance）

- `GetCommentTree` 单次整帖查询，O(评论数) 内存拼树；50 人规模无放大。
- 列表查询按 `limit/offset` 分页（IA v2 §10：MVP 简化，前端无限滚动）；不做 scale 索引（#5）。
