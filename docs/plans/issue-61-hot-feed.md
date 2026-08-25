# #61 计划：首页热门/精华流——内容发现新分段（热门）

> 状态：**待实施**（grilling 已定案，本计划待用户审阅；审阅通过后按 SOP 实施）。落地后如需评审，走 `design-doc-for-review` 升格设计文档 → `plan-review`。
> 生成：2026-08-25 · 实现会话（Claude Code）。
> 对应：GitHub issue #61（地图 issue #1 child：内容发现）。协同：并行 #59（notify）、#60（moderation/ops）与本票**零文件重叠**，worktree 隔离（`../ai-forum-61`）。

---

## 1. 背景与目标

首页当前只有「全部 / 关注」两段（`frontend/src/views/Feed.vue`），全部 = 时间倒序、关注 = 关注人/板块/话题的帖子。置顶/精华（pin/feature，mod 组路由）已实现，但缺**发现性**维度——好内容没有「按热度浮现」的通道。

**目标**：首页加「热门」分段——7 天窗口内按热度排序的内容流，让好内容浮上来。

**性质**：新增信息流 tab（纯增量，不改现有 all/follow 行为）；按 SOP §3 先 grilling 定案（已完成，见 §3）。

## 2. 现状（代码真值，2026-08-25 探明）

- 生产：SSH **2222**，HEAD=`9cb5ec0`，api/postgres/web 全 healthy，healthz ok。本地 `main`=origin/main=`9cb5ec0`。
- `frontend/src/views/Feed.vue`：SegTabs `全部/关注`，`tab: ref<'all'|'follow'>`；fetcher 走 `api.listPosts({tab, page, pageSize})`；关注 tab 游客显示 `LoginGuide`（IA §4）。
- `frontend/src/api/content.ts:28`：`ListPostsParams.tab?: 'all' | 'follow'`。
- `backend/content/service.go`：`ListFeed`（:504-557）按 `ListFeedQuery.Tab`（:112-121，`all|follow`）switch；`follow` 需登录（`ViewerID==0 → ErrAuthRequired`），`all` 公开；组装走 `postViews`（赞/评论/收藏数**现场 Count**，不落库）。
- `backend/content/repo.go`：`PostQuery.Feed`（:7，`all|follow`）+ `ListPosts(ctx, PostQuery)`（:30）。
- `backend/content/gorm_repo.go`：`ListPosts`（:86-118）过滤（author/board/topic/tag/follow）→ `Order("is_pinned DESC").Order("created_at DESC")` → limit/offset。
- 数据模型（`backend/content/model.go`）：`Post{...IsPinned,IsFeatured,ViewCount,CreatedAt,DeletedAt}`（:37-50）；`Like{UserID,TargetType,TargetID}` → `content.likes`（:69-77）；`Comment{PostID,...,DeletedAt}` → `content.comments`（:55-65，软删）。
- **索引已覆盖**（迁移 0002）：`idx_likes_target (target_type, target_id)`、`idx_comments_post (post_id)`——热门聚合子查询可走索引。
- handler：`internal/httpapi/handler/content.go` `ListPosts`（:96-127）`tab` 经 `DefaultQuery("tab","all")` **直通 service，无白名单** → 后端只改 content 包即可透出新 tab。
- 测试基建：service 单测跨 fake repo seam（`fake_repo_test.go`，fake 已镜像 `置顶+时间倒序` 排序）；集成测试 testcontainers 真实 PG（`internal/testutil/db.go`，无 Docker 自动 skip，CI 阻塞门控）。

## 3. 用户定案（grilling 2026-08-25，AskUserQuestion）

| # | 问题 | 定案 |
|---|---|---|
| Q1 | 热度公式 | **7 天窗口 + 赞×1 + 评论×3**，降序（浏览量暂不参与） |
| Q2 | 入口形态 | 首页三分段 **全部 / 热门 / 关注**；热门**游客可见**（公开） |
| Q3 | 置顶/精华关系 | 热门段**置顶恒顶**；**不做**精华分段，精华帖按热度自然参与 |

**MVP 补充决策**（实现时写入决策记录）：热度分同分时按 `created_at DESC` 兜底（新帖优先）；热度段复用现有 `board_id`/`tag`/`author_id` 过滤（同 query 路径，自然叠加）。

## 4. 实现方案

### 核心设计

**硬约束**：赞/评论数不落库 → 热门排序必须在 **SQL 里现算热度分再排序**（先取页再排会排错）。热度分落地为 PostgreSQL 表达式，分页在排序后做，天然正确。

**热门段排序**：`置顶优先（is_pinned DESC）→ 热度分降序 → 时间倒序兜底`。

**热度分 SQL**：
```sql
(COALESCE(hot_likes.like_cnt, 0) + 3 * COALESCE(hot_comments.cmt_cnt, 0)) DESC
```
两个 LEFT JOIN 聚合子查询（likes 按 post 计数、comments 按 post 计数且排除软删）。

### 后端 `backend/content`（4 文件）

| # | 文件 | 改动 |
|---|---|---|
| C1 | `service.go` | `ListFeedQuery.Tab` 注释补 `hot`（:113）；`ListFeed` switch 加 `case "hot": q.Feed = "hot"`（无登录墙，游客可见，置于 `case "all"` 后） |
| C2 | `repo.go` | `PostQuery.Feed` 注释补 `hot`（:7） |
| C3 | `gorm_repo.go` | `ListPosts` 加 hot 分支（:101 后）：`WHERE created_at >= now() - 7 天`；`LEFT JOIN` likes/comments 聚合子查询（`content.likes` 需 `target_type='post'`、`content.comments` 需 `deleted_at IS NULL`）；排序改为 `is_pinned DESC` → hot 时插入热度分表达式（`gorm.Expr`）→ `created_at DESC`。import 补 `time` |
| C4 | `fake_repo_test.go` | fake `ListPosts` 镜像 hot 分支：7 天窗口过滤 + `hotScore()` 排序（赞+3×评论，从 `f.likes`/`f.comments` 现算，排除软删评论） |

### 前端（2 文件）

| # | 文件 | 改动 |
|---|---|---|
| F1 | `api/content.ts` | `ListPostsParams.tab`（:28）：`'all' \| 'follow'` → `'all' \| 'hot' \| 'follow'` |
| F2 | `views/Feed.vue` | SegTabs options 插 `{ key: 'hot', label: '热门' }`（全部/热门/关注）；`tab` ref 类型补 `'hot'`；空态文案按 tab 动态（热门 →「暂无热门帖子」，其余保持「还没有帖子」）；关注游客 `LoginGuide` 逻辑不变（热门游客不弹） |

**handler / router / types 零改动**：tab 直通 + `PostList`/`PostView` 复用，`tab` 只是前端类型宽化。

## 5. 测试

| # | 层 | 用例 |
|---|---|---|
| T1 | service 单测（`service_test.go`，跨 fake seam） | `TestListFeed_Hot`：① 权重排序（同帖 3 评论 vs 1 赞 vs 混合，验证 评论×3 > 赞×1）；② 置顶恒顶（低热度置顶帖仍第一）；③ 7 天窗口外剔除（seed `CreatedAt` 超窗不出现）；④ 分页正确（窗口内多帖翻页）；⑤ 游客可访问（`Tab:"hot", ViewerID:0` 不报 `ErrAuthRequired`） |
| T2 | 前端单测（`views/Feed.test.ts` 新建，仿 `Me.test.ts`） | 三段渲染（labels = 全部/热门/关注）；hot tab → `listPosts({tab:'hot',...})`；游客切 hot **无** LoginGuide；游客切 follow **有** LoginGuide（回归） |
| T3 | handler 集成（`internal/httpapi/handler/content_test.go`，testcontainers 真 PG） | hot 端点（`?tab=hot`）真跑 LEFT JOIN SQL：造帖+赞+评论 → 断言返回顺序=热度降序、窗口外剔除、游客 200；同时兜底 C3 SQL 正确性（fake 镜像无法验 SQL） |

**既有用例不破**：C1-C4 只加 `case "hot"` 分支，不碰 all/follow 路径；`TestListFeed_All`/`_Follow` 保持绿。

## 6. 风险与已知边界

1. **热度公式双处实现**（gorm SQL + fake 镜像 Go）——项目既有模式（fake 本就镜像排序），SQL 正确性由 T3 集成测试兜底；公式权重（×3）在 fake 侧抽常量可读。
2. **LEFT JOIN 聚合子查询开销**：7 天窗口天然限行数；likes/comments 索引已覆盖（§2）。帖量显著增长后可改物化计数列，**本票不做**。
3. **GORM JOIN + Order 表达式**：`gorm.Expr` 原样透传；表名 `content.posts`/`content.likes`/`content.comments` 显式全名，规避 GORM 表名解析歧义。T3 实测。
4. **窗口时区**：`time.Now()`（Go 进程）与 PG `now()` 同一进程内一致，无跨时区歧义；窗口 =「服务器当前时间 − 7 天」。
5. **评论计数口径**：`content.comments` 全部计入（含回复），与现有 `postViews.CommentCount` 口径一致（同 `post_id` 计数），避免同一帖子两处计数打架。
6. `frontend/components.d.ts` 会被 build 改写 → 提交前 `git checkout --`。

## 7. 验证与协作

- 本地 `cd backend && go build ./... && go vet ./... && go test ./...`（含 T3 真跑）+ `cd frontend && npm run test:unit && npm run build` 全绿。
- 本地冒烟：切「热门」分段 → 高热度帖在前、置顶恒顶、窗口外帖不出现。
- 分支 `feat/hot-feed`（worktree `../ai-forum-61`，已建且干净）→ 实现 → 测试全绿 → commit → push → `MSYS_NO_PATHCONV=1 gh pr create` → **用户授权** → merge → CI 部署 → 服务器验证（HEAD 对齐 + healthz ok + `?tab=hot` 顺序核对）。
- **合并时提醒统筹方搭挂起的 PR #57**（术语表 docs）一起合。
- 地图 issue #1 只由统筹方更新；本票完成后回报统筹方。
