# #61 首页热门流：内容发现新分段 — 设计文档（供评审）

> 文档用途：交付专业评审 agent 的评审对象。范围 = 背景 / 真值核对 / 决策记录 / 实现方案 / 不变量 / 验证。
> 溯源约定：**事实**标来源（代码 `file:line` / 服务器实查 / GitHub issue / grilling 用户确认）；**判断性裁决**单独标注【决策】并给出理由与备选，不冒充事实。
> 数据时点：2026-08-25（真值核对执行日；服务器实查 2026-08-25）。
> 评审状态：待评审（评审完成后回填，含评审意见书路径）。

---

## 0. 项目上下文（给零背景评审 agent，先读本节）

**这是什么**：AI 智联论坛——社团内部 AI 主题社区的移动端论坛 MVP。

- 前端：Vue 3 + TypeScript + Vite + Vant（PWA），纯静态部署（`frontend/`）。
- 后端：Go（Gin + GORM + PostgreSQL），单进程模块化单体（`backend/`）。
- 数据库：PostgreSQL 单实例、**多 schema**，每域一个：`user` / `content` / `notify` / `moderation`。

**后端架构约定（评审本设计必须理解）**（源：`docs/workflow.md` §5 / `docs/design-principles.md`）：
- 4 个领域包每包结构固定：`model.go` / `repo.go` / `gorm_repo.go` / `service.go` / `README.md`。
- **跨包调用禁止直接 import 他包**：调用方在本包定义所需接口（seam），由组合根装配实现。
- 哨兵错误变量 + 调用方映射 HTTP 状态码；测试跨 seam（fake repo / testcontainers 真 PG）。
- 列表查询是独立查询形态：分页列表需在 repo 层加 `offset/limit` 方法，排序在 SQL 内完成（SOP §5.3 / §12 #23 walkthrough）。

**路由与中间件**（`backend/internal/httpapi/router.go`）：
- `Auth`（纯验签）、`OptionalAuth`（登录可选，游客 id=0）、`RequireRole(roles...)`。
- 路由组：`authed`（需登录）、`reads`（可选登录）、`writers`（登录+写）、`mod`（登录+mod 角色）。
- **`GET /api/v1/posts` 挂在 `reads` 组**（router.go:102）——信息流端点游客可访问。

**本票相关的已有能力**：
- 置顶/精华：`POST /posts/:id/pin|feature`（router.go:134，mod 组）→ `posts.is_pinned` / `posts.is_featured` 布尔列。
- 信息流：`ListFeed`（`backend/content/service.go`）已支持 `tab=all|follow`；`postViews` 组装读模型。
- 测试基建：service 单测跨 fake repo seam（`fake_repo_test.go`，fake 已镜像「置顶+时间倒序」排序）；handler 集成 testcontainers 真 PG（`internal/testutil/db.go`，无 Docker 自动 skip，CI 阻塞门控）。

**术语速查**：
- Feed = 首页信息流分段；`tab` = 分段键（all/hot/follow）。
- 热度分 = 本票新概念：`赞×1 + 评论×3`，7 天窗口内按此降序。

## 1. 背景与目标（为什么做）

- 需求来源：GitHub **issue #61**「首页热门/精华流：内容发现新分段」（状态 OPEN，地图 issue #1 child，方向已定 2026-08-25）。
- 交接依据：`docs/plans/issue-61-hot-feed.md`（本票计划，§1-§7）；SOP `docs/workflow.md` §3/§5/§7/§12。
- 用户明确目标：首页加「热门」分段（按热度排序的内容流），让好内容浮上来。
- **目标一句话**：首页从「全部/关注」两段 → 加「热门」段 = 7 天窗口内按热度分降序的内容流，游客可见、置顶恒顶。

## 2. 真值核对（数据来源，全部可复现）

### 2.1 服务器真值（2026-08-25 实查，SSH 端口 **2222**）

命令：`ssh -p 2222 liyongquan@122.51.233.225 'cd ~/ai-forum && git log --oneline -1 && docker compose ps && curl -s http://localhost:8888/healthz'`

结果摘录：
```
9cb5ec0 Merge pull request #58 from li-yongqvan/chore/deploy-speedup
ai-forum-api        ... Up 28 minutes (healthy)    127.0.0.1:8080->8080/tcp
ai-forum-postgres   ... Up 5 days (healthy)        127.0.0.1:5432->5432/tcp
ai-forum-web        ... Up 4 days                  0.0.0.0:8888->80/tcp
{"status":"ok"}
```

结论：✅ 线上 HEAD=`9cb5ec0`，三容器 healthy，healthz ok——与本机 `main`/`origin/main` 一致（`git rev-parse HEAD origin/main` 均 `9cb5ec0`），开工基线明确。

### 2.2 代码真值（本机仓库 grep，2026-08-25）

**信息流入口（前端）**：
- `frontend/src/views/Feed.vue:10` `const tab = ref<'all' | 'follow'>('all')`；`:18-25` SegTabs options `全部/关注`；`:28` `<LoginGuide v-if="tab === 'follow' && !auth.isLoggedIn" />`（游客点关注 tab 原地登录引导）；`:29` PostList `:key="tab"` 重挂载 + fetcher 走 `api.listPosts`（:13）。
- `frontend/src/api/content.ts:26-35` `ListPostsParams`（`tab?: 'all' | 'follow'` 在 :28）；`listPosts`（:37+）无 `requireAuth` → 游客可调。

**信息流服务（后端）**：
- `backend/content/service.go:112-121` `ListFeedQuery`（`Tab` 在 :113 注释 `all|follow`）；`:504` `ListFeed`；`:524-550` switch：`""/all`→`q.Feed="all"`（:526）、`follow`→`ViewerID==0` 返回 `ErrAuthRequired`（:529）再 `q.Feed="follow"`（:531）、default→`ErrInvalidFeedTab`；`:552` 调 `repo.ListPosts`；`:556` `postViews`。
- `backend/content/repo.go:6-17` `PostQuery`（`Feed` 在 :7 注释 `all|follow`）；`:30` `ListPosts(ctx, PostQuery)`。
- `backend/content/gorm_repo.go:86-118` `ListPosts`：过滤（author/board/topic/tag/follow :88-105）→ `Order("is_pinned DESC").Order("created_at DESC")`（:106）→ limit/offset（:107-112）。
- `backend/content/model.go`：`Post`（:37-50，含 `IsPinned`/`IsFeatured`/`ViewCount`/`CreatedAt`/`DeletedAt`）；`Comment`（:55-65，含 `PostID`/`DeletedAt` 软删）；`Like`（:69-77，含 `TargetType`/`TargetID`）。
- `backend/internal/httpapi/handler/content.go:96-127` `ListPosts` handler：`Tab: c.DefaultQuery("tab","all")`（:100）**直通 service，无白名单**；其余过滤参数经 query 绑定。

**计数与读模型**：
- `backend/content/service.go:770/774/778` `postViews` 内经 `CountLikesByPost` / `CountCommentsByPost` / `CountFavoritesByPost` **现场 Count**（赞/评论/收藏数不落库）——这是「热度分必须在 SQL 现算」的根因。

**索引（迁移 0002）**：
- `backend/migrations/0002_content_schema.sql:56` `CREATE INDEX idx_comments_post ON content.comments (post_id);`
- `:67` `CREATE INDEX idx_likes_target ON content.likes (target_type, target_id);`
- 结论：✅ 热度聚合子查询（likes 按 `target_type='post'`+`target_id` 分组、comments 按 `post_id` 分组）可走现有索引。

**路由**：
- `backend/internal/httpapi/router.go:102` `reads.GET("/posts", ch.ListPosts)`（OptionalAuth 组）→ ✅ 游客访问架构上天然满足，**零路由改动**。

**前端可复用组件**（`frontend/src/components/`）：
- `PostList.vue` / `Empty.vue` / `SegTabs.vue` 均存在 → 热门分段复用，不新造列表。

**既有热门痕迹**：
- 全库 grep（backend + frontend/src）：`grep -rE '\bhot\b|热门' backend/ frontend/src/`（词边界过滤 `photo.png` 等假命中）→ **零命中** → 本票是全新 tab，无既有冲突。

### 2.3 GitHub 状态（2026-08-25）

- issue #61：OPEN，labels `wayfinder:task`，方向已定（用户选定 2026-08-25）。
- 挂起 **PR #57**（术语表 docs）：计划 §7 要求本票 PR 合并时搭它一起合。
- 并行 #59（notify，分支 `feat/dm`）、#60（moderation/ops）与本票零文件重叠；worktree 隔离已建（`../ai-forum-61`）。

## 3. Grilling 决策记录（3 项，用户确认）

| 编号 | 决策问题 | 定案 | 依据 |
|---|---|---|---|
| D1 | 热度公式 | **7 天窗口 + 赞×1 + 评论×3**，降序；浏览量暂不参与 | 用户确认（2026-08-25，AskUserQuestion） |
| D2 | 入口形态 | 首页三分段 **全部 / 热门 / 关注**；热门游客可见（公开） | 用户确认（2026-08-25）；`/posts` 在 reads 组（§2.2）天然支撑游客 |
| D3 | 置顶/精华关系 | 热门段**置顶恒顶**（`is_pinned` 优先）；**不做**精华分段，精华帖按热度自然参与 | 用户确认（2026-08-25）；与全部流既有置顶行为一致（§2.2 gorm_repo:106） |

**窗口语义（D1 补充，F3）**：「7 天窗口」= 帖子创建时间 `created_at ∈ [now-7d, now]`；计数 = 该帖**全生命周期**的赞/评论总数。不影响 D1 定案，只消除「近 7 天互动热度」歧义。
- D1 备选：全时间+时间衰减（HN 式）——新帖自然浮上但公式复杂、需 `power()` 表达式；24h 快热——过度强调「当下」，窗口外好帖被切走。用户选简单可解释的 7 天窗口。
- D2 备选：单独入口/徽标——交互要新设计、改动面大，不符现有 IA 分段模式。
- D3 备选：热门纯热度不看置顶（置顶帖沉底）——与全部流置顶行为不一致；同时加精华段——范围膨胀（本票只做热门，精华留待后续 ticket）。

## 4. 范围收敛与明确不做

| 项 | 决策 | 依据 |
|---|---|---|
| 精华（featured）分段 | **不做**（本票只做热门） | D3 用户确认；issue #61 标题含「精华」但 grilling 收敛为仅热门 |
| notify / moderation 域、`.github/workflows/` | **不碰** | 计划 §7 红线 / SOP §4 范围红线；本票零文件重叠 |
| 浏览量进热度分 | 不做（D1 定案） | 用户确认；保持公式简单可解释 |
| 热度分物化列（在 posts 表加冗余计数/分数列） | 不做 | §2.2 计数不落库是既有设计；7 天窗口 + 现有索引下 MVP 够用；物化需迁移+写入侧同步，留给未来 |
| 时间衰减 / 分页游标 | 不做 | D1 纯计数窗口定案；分页沿用 offset/limit（既有形态） |
| handler / router / 前端 types.ts | **零改动** | §2.2 验证：tab 直通 + reads 组 + PostView 复用 |

## 5. 实现方案（每项给出依据）

### 5.1 后端 `backend/content`（4 文件）

1. **`service.go`**：`ListFeedQuery.Tab` 注释补 `hot`（:113）；`ListFeed` switch 加 `case "hot": q.Feed = "hot"`（置于 `all` 分支后，无登录墙）。
   - 依据：`follow` 分支的登录墙在 :529（`ViewerID==0 → ErrAuthRequired`）；`hot` 与 `all` 同为公开（D2），不加墙；`ErrInvalidFeedTab` 兜底不变。
2. **`repo.go`**：`PostQuery.Feed` 注释补 `hot`（:7）。
   - 依据：`PostQuery.Feed` 是 `ListFeed` → `ListPosts` 的传递字段（§2.2），注释即契约。
3. **`gorm_repo.go`** `ListPosts` 加 hot 分支（:101 后、:106 排序前）：
   - `Select("content.posts.*")` 限定主表列（F8），防 JOIN 表列遮蔽——仓库既有惯例（`gorm_repo.go:354-356` 等）。
   - `WHERE created_at >= ?`（`time.Now().Add(-7*24*time.Hour)`）——7 天窗口（D1）。
   - 两个 `LEFT JOIN` 聚合子查询：likes（`WHERE target_type='post'`，按 `target_id` 分组）、comments（`WHERE deleted_at IS NULL`，按 `post_id` 分组）——赞/评论不落库，现场聚合（§2.2 postViews:770/774）。子查询必须保持**预聚合形态**；若直接 JOIN 原始 likes/comments 两表，会因笛卡尔积导致赞/评论计数互乘。
   - 排序改为：`is_pinned DESC` → hot 时插 `Order(gorm.Expr("(COALESCE(hot_likes.like_cnt,0) + 3*COALESCE(hot_comments.cmt_cnt,0)) DESC"))` → `created_at DESC`（兜底）。
   - import 补 `time`（当前 imports 无 `time`，§2.2 未见）。
   - 依据：D1/D3；分页在排序后（limit/offset 于 :107-112 已存在），先排后取分页天然正确。
4. **`fake_repo_test.go`** `ListPosts` 镜像 hot 分支（:171 follow 后）：7 天窗口过滤 + `hotScore()` 辅助（赞+3×评论，从 `f.likes`/`f.comments` 现算，排除软删评论）在排序回调内调用（现排序 :181-186）。
   - 依据：项目测试纪律「测试只跨 seam」，fake 镜像 repo 语义是既有模式（fake 已镜像置顶+时间倒序，§2.2）；保证 service 单测能验证 hot 行为。

### 5.2 前端（2 文件）

1. **`frontend/src/api/content.ts:28`**：`tab?: 'all' | 'follow'` → `'all' | 'hot' | 'follow'`。
   - 依据：tab 是前端类型宽化，`listPosts` 走 `request` 无 requireAuth（§2.2），游客可调。
2. **`frontend/src/views/Feed.vue`**：SegTabs options 插 `{ key: 'hot', label: '热门' }`（全部/热门/关注，:20-21 之后）；`tab` ref 类型补 `'hot'`（:10）；空态文案按 tab 动态（热门→「暂无热门帖子」，其余保持「还没有帖子」，:29）；关注游客 LoginGuide 逻辑不变（:28）。
   - 依据：D2；`LoginGuide` 条件只匹配 `tab==='follow'`，热门不弹（游客可看热门）；复用 PostList/SegTabs（§2.2）。

### 5.3 测试（3 处，详见 §8）

- `service_test.go` 新增 `TestListFeed_Hot`（跨 fake seam）；`views/Feed.test.ts` 新建；`internal/httpapi/handler/content_test.go` 增 hot 端点集成用例（真 PG 兜底 LEFT JOIN SQL）。

## 6. 关键设计裁决（【决策】，含理由与备选）

### 6.1 热度分在 SQL 现算（LEFT JOIN 聚合），不在 service 内存算
- **问题**：赞/评论数不落库（§2.2 postViews:770/774）。热度排序若要分页正确，必须在「排序之后」才 offset/limit——若先取一页再内存算分排序，会拿到错误的一页。
- **定案【决策】**：热度分用 PostgreSQL 表达式在 `ListPosts` 的查询内算（`COALESCE` 聚合 + `gorm.Expr` 排序），`ListFeed` 只传 `q.Feed="hot"`。
- **理由**：单查询、分页正确、无额外往返；对调用方（handler/前端）完全透明，`ListFeed` 签名不变。
- **备选（不选）**：service 层取窗口内全部帖内存算分再分页——每请求拉全量窗口帖，随数据增长线性膨胀，且绕开「分页列表是独立查询形态」的项目纪律（SOP §5.3）。

### 6.2 热度分同分兜底：`created_at DESC`
- **定案【决策】**：热度分相等时按创建时间倒序（新帖优先），置于 `is_pinned DESC → 热度分 DESC → created_at DESC` 尾部。
- **理由**：热度分是离散整数，同分极常见；时间兜底保证排序确定性、分页稳定（否则同分帖在跨页时顺序漂移）。
- **注**：此为实现层定案（实现会话拟定，非用户 grilling 确认），已列 §9 Q4 待评审确认。

### 6.3 热度段与其他过滤叠加（board/tag/author）
- **定案【决策】**：`hot` 复用 `ListPosts` 既有过滤链（author/board/topic/tag），过滤与热度排序正交叠加（如 `?tab=hot&tag=ai` = 该标签下的热度流）。
- **理由**：零额外代码（同 query 路径）；语义自然（板块/标签页也能有热度视图）。
- **注**：同上为实现层定案，列 §9 Q4。

### 6.4 热度公式双处实现（SQL + fake 镜像）
- **问题**：fake repo 若要支撑 service 单测验证 hot 行为，必须镜像 SQL 的窗口+权重语义——公式出现两处。
- **定案【决策】**：接受双处实现（这是项目既有模式，fake 已镜像排序语义 §2.2）；SQL 正确性由 §8 集成测试（真 PG）兜底；fake 侧权重（×3）抽常量 `commentWeight` 提升可读。
- **理由**：换取 service 层可跨 seam 单测，符合「测试只跨 seam」纪律；真 PG 集成测试补足 SQL 验证。
- **备选（不选）**：只在集成层测 hot、fake 不镜像——service 单测无法覆盖 hot 行为（权重/窗口/置顶），回归保护不足。

### 6.5 置顶帖与 7 天窗口：窗口优先（F2 裁决）
- **问题**：D3 只说「置顶恒顶」，但未裁决置顶帖超出 7 天窗口时怎么办——超窗置顶帖会被 `WHERE created_at >= now()-7d` 整体剔除，导致「全部流置顶可见、热门流消失」，两段行为不一致。
- **定案【决策】**：**窗口优先**。`is_pinned` 仅作为结果集内的排序首键，不做窗口豁免；超窗置顶帖从热门段不出现，窗口内置顶帖恒顶。
- **理由**：用户已裁决方案 A（2026-08-25，评审意见书 F2）；7 天窗口是热门流的核心过滤语义，若置顶帖可豁免窗口，则管理很久以前的旧帖能长期占据热门，违背「让当下好内容浮上来」的目标。
- **备选（不选）**：置顶帖豁免窗口（超窗置顶仍排热门第一）——与「热门」产品语义冲突，且管理旧帖可长期霸榜。

## 7. 边界与不变量清单（含防护层）

| # | 不变量 | 防护层 | 依据 |
|---|---|---|---|
| 1 | 热门段游客可访问（不要求登录） | `ListFeed` hot 分支不加登录墙；`/posts` 在 reads 组（OptionalAuth） | §5.1 / §2.2 router:102 |
| 2 | 7 天窗口外帖子不出现 | `ListPosts` hot 分支 `WHERE created_at >= now()-7d`；fake 镜像同过滤 | §5.1 / D1 |
| 3 | **窗口内**置顶帖恒在热门段顶部；超窗置顶帖不出现 | `WHERE created_at >= now()-7d` 先过滤；剩余结果 `is_pinned DESC` 排首；fake 镜像同逻辑 | §5.1 / D3 / §6.5 |
| 4 | 热度分 = 赞×1 + 评论×3，评论软删不计 | LEFT JOIN 子查询 `deleted_at IS NULL`；fake `hotScore` 排除软删 | §5.1 / D1 |
| 5 | 分页正确（排序先于 offset/limit） | 排序在 `Limit`/`Offset` 之前构建（gorm_repo :106 后插排序） | §5.1 / §6.1 |
| 6 | 同分排序确定性、跨页稳定 | `created_at DESC` 尾键兜底 | §6.2 |
| 7 | all/follow 行为不变（回归） | switch 只加分支不删分支；既有 `TestListFeed_All/_Follow` 保持绿 | §8 |
| 8 | 计数口径与 `postViews` 一致（评论含回复、全部计入） | hot 子查询与 `CountCommentsByPost` 同为按 `post_id` 计数 | §5.1 / §2.2 postViews:774 |
| 9 | `frontend/components.d.ts` 不被污染提交 | 提交前 `git checkout -- frontend/components.d.ts`（SOP §4.5） | §8 |

## 8. 测试与验证计划

| 域 | 用例 | 覆盖 |
|---|---|---|
| service 单测（`service_test.go`，跨 fake seam） | `TestListFeed_Hot`：①权重排序（3 评论 > 1 赞，验证 评论×3 权重）；②窗口内置顶恒顶（低热度置顶帖仍第一）；③**超窗置顶帖不出现**；④7 天窗口剔除（seed 超窗 `CreatedAt` 不出现）；⑤分页（窗口内多帖翻页顺序稳定）；⑥游客（`Tab:"hot",ViewerID:0` 不报 `ErrAuthRequired`）；⑦同分按 `created_at` 兜底；⑧**hot + tag 过滤叠加**（`Tag` + `Feed=hot` 同时生效） | D1/D3/不变量 2-6、§6.3、§6.5 |
| handler 集成（`internal/httpapi/handler/content_test.go`，testcontainers 真 PG） | hot 端点 `GET /posts?tab=hot`：造帖+赞+评论 → 断言返回顺序=热度降序、**窗口内置顶帖第一、超窗置顶帖不出现**、窗口外剔除、游客 200、分页正确；**`?tab=hot&tag=ai` 叠加过滤** 正确 | §6.3 / §6.4 / §6.5 SQL 正确性兜底（fake 无法验 SQL） |
| 前端单测（`views/Feed.test.ts` 新建，仿 `Me.test.ts` mock 模式） | 三段渲染（labels=全部/热门/关注）；hot tab → `listPosts({tab:'hot',...})`；游客切 hot **无** LoginGuide；游客切 follow **有** LoginGuide（回归）；空态文案按 tab | D2 / Feed.vue 改动 |
| 回归 | 既有 `TestListFeed_All/_Follow`、`TestListFeedByTag`、前端既有单测保持绿 | 不变量 7 |
| 本地全量 | `cd backend && go build ./... && go vet ./... && go test ./...`；`cd frontend && npm run test:unit && npm run build`；提交前 `git checkout -- frontend/components.d.ts` | 门控 |
| 本地冒烟 | 切「热门」分段 → 高热度帖在前、置顶恒顶、窗口外帖不出现 | 端到端 |

## 9. 待评审焦点（Q1-Q5）

| # | 焦点 | 为什么值得盯 |
|---|---|---|
| Q1 | 热度公式双处实现（SQL + fake 镜像）的长期可维护性 | 权重（×3）与窗口（7d）在两个文件维护；已抽 fake 侧常量，但 SQL 侧仍是字面量——是否可接受，或是否应仅在集成层测 hot 以消除镜像 |
| Q2 | LEFT JOIN 聚合子查询在数据量增长下的性能 | 现有索引覆盖（§2.2），7 天窗口限行数；但热门每请求都全量聚合窗口——MVP 够用 vs 何时需物化列（§4 已排除物化） |
| Q3 | 窗口用服务器时钟 `time.Now()` 的测试脆弱性 | 集成测试（真 PG）里窗口是相对的（seed 相对 now）；若未来做数据回放/时区切换，窗口语义是否要改为 DB 时钟 |
| Q4 | §6.2/§6.3 实现层定案（同分兜底、过滤叠加）未经用户 grilling 确认 | 影响行为细节（同分顺序、板块页热度），非骨架级；是否需回补用户确认 |
| Q5 | 热门空态文案「暂无热门帖子」是产品文案，未用户确认 | 文案属于产品决策；实现先用合理默认，上线前是否需确认 |

## 10. 评审意见采纳记录

评审已完成（2026-08-25，评审意见书 `issue-61-hot-feed-设计文档-评审意见书.md`）：结论为**有条件通过**。本设计文档已按评审意见修订，未决执行项在代码实现中落地。

| 评审项 | 结论 | 采纳落地 |
|---|---|---|
| **F1 阻塞**：引用不存在的 `docs/handoffs/61-hot-feed.md` + 缺 `grilling-decisions` | 属实 | §1/§2.3/§4 引用已修正为 `docs/plans/issue-61-hot-feed.md`；补 `docs/handoffs/grilling-decisions/issue-61-hot-feed-decisions.md`；计划+设计文档+意见书随 PR 入库 |
| **F2 重要**：置顶帖 × 7 天窗口未裁决 | 用户已裁决方案 A（窗口优先） | 新增 §6.5；§7 不变量 3 改为「窗口内置顶帖恒顶」；§8 补超窗置顶剔除 + 窗内置置顶第一的测试 |
| **F3 重要**：7 天窗口语义未显式写明 | 属实 | §3 补「窗口 = 帖子创建时间窗口 + 全生命周期计数」；写入 grilling-decisions |
| **F4**：`Feed.vue:24` cast 未含 `'hot'` | 属实 | 实现时同步修改（代码实现清单 §5.2 已标注） |
| **F5**：hot + 过滤叠加无测试 | 属实 | §8 service/handler 测试各补 1 例叠加用例 |
| **F6**：物化触发条件未量化 | 建议采纳 | 决策记录写明量化口径；本地冒烟加 `EXPLAIN ANALYZE` |
| **F7**：「零命中」缺精确命令 | 属实 | §2.2 已补 `grep -rE '\bhot\b\|热门' backend/ frontend/src/` |
| **F8 重要**：hot 分支 JOIN 未防列遮蔽 | 属实 | §5.1 C3 已补 `Select("content.posts.*")` + 防笛卡尔扇出注释；§8 集成测试断言列不遮蔽 |
| **F9**：热度口径可博弈（自评论刷分） | 建议本票不动 | 记录为未来迭代候选，本票不实现 |
| **Q1**：双处实现取舍 | 保留双处，加注释 | fake 侧抽 `commentWeight`；gorm_repo 侧加注释指向 fake 常量 |
| **Q4/Q5**：实现层定案/空态文案未用户确认 | 不回补 grilling，标注即可 | PR 描述显式标注 §6.2/§6.3 为「实现层定案」；标注「暂无热门帖子」为产品默认文案 |

**推翻项**：无。全部评审发现经复核属实。
