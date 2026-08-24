# #54 标签系统（#hashtag）：正文内嵌标签 + 聚合页 — 设计文档（供评审）

> 文档用途：交付专业评审 agent 的评审对象。范围 = 背景 / 真值核对 / 决策记录 / 实现方案 / 不变量 / 验证。
> 溯源约定：**事实**标来源（代码 `file:line` / 服务器实查输出 / GitHub issue / grilling 用户确认）；**判断性裁决**单独标注【决策】并给出理由与备选，不冒充事实。
> 数据时点：2026-08-24（真值核对执行日；服务器实查 2026-08-24）。文档由实现会话产出，尚未实现任何代码。
> 评审状态：**有条件通过**（2026-08-24 独立评审，评审方以本地仓库 @ 0301601 + Node v24 / Go 正则实证复核）。评审意见书：`docs/plans/54-tag-system-设计文档-评审意见书.md`。F1–F9 已裁决**全部采纳**，采纳记录见 §10；**本版为实现基线**。

## §0 项目上下文（给零背景评审 agent，先读本节）

**这是什么**：AI 智联论坛——社团内部 AI 主题社区的移动端论坛 MVP。

- 前端：Vue 3 + TypeScript + Vite + Vant（PWA），纯静态部署（`frontend/`）。
- 后端：Go（Gin + GORM + PostgreSQL），单进程模块化单体（`backend/`）。
- 数据库：PostgreSQL 单实例、**多 schema**，每域一个：`user` / `content` / `notify` / `moderation`。

**后端架构约定（评审本设计必须理解）**：
- 4 个领域包，每包结构固定：`model.go`/`repo.go`/`gorm_repo.go`/`service.go`/`README.md`（`docs/workflow.md` §5）。
- 物理 FK 仅包内；跨 schema 引用（`author_id` 等）是逻辑 FK，建索引不建约束（`backend/content/model.go` 头注释）。
- **跨包调用禁止直接 import 他包**：调用方定义接口（seam），组合根装配（`docs/design-principles.md`）。
- 哨兵错误变量 + 调用方映射 HTTP 状态码；测试跨 seam（fake repo / testcontainers 真 PG）。
- 可查阅：`docs/workflow.md`（SOP）、`docs/impl/testing.md`、`backend/content/README.md`。

**角色与中间件**：
- `user.users.role` 存 `member | moderator | admin`；JWT claims 映射 `user | moderator | admin`。
- 中间件：`Auth`（纯验签）、`OptionalAuth`（有 token 才注入身份，游客放行）、`RequireRole`（`backend/internal/httpapi/middleware/auth.go:20/41/55`）。

**content 域现状（本票宿主）**：
- 板块（`content.boards`）→ 话题（`content.topics`）两级已实现；帖子 `content.posts` 带完整 `content` 正文（`backend/content/model.go`）。
- 列表查询 `ListPosts`（`backend/content/gorm_repo.go:84-112`）：`Where` 过滤 + `Order("is_pinned DESC").Order("created_at DESC")` + 有界 Limit/Offset；GORM 软删自动 scoping。
- 读模型批量组装 `postViews`（`backend/content/service.go:744-843`）：batch counts → 登录态 liked/faved → 作者名（缺省「已注销」）→ 板块名 → 话题名 → 循环拼 `PostView`。
- 列表 JOIN 先例（#23）：`ListFavoritedPosts` 显式 `Select("content.posts.*")` 防列遮蔽（`backend/content/gorm_repo.go:346-364`）。
- **无任何标签代码**（全库 grep 无 `Tag` 标识符，Plan agent 实证）——全新模块。

## §1 背景与目标（为什么做）

- 需求来源：GitHub **issue #54**「标签系统（#hashtag）：正文内嵌标签 + 聚合页」（OPEN，`wayfinder:task`）。方向已由用户选定（2026-08-24）。
- 交接依据：`docs/handoffs/54-tag-system.md`（自包含交接，§3 列出 5 项 grilling 开放决策）。
- **目标**：发帖正文中 `#关键词` 自动识别为可点击标签 → 点标签进入聚合页（该标签下帖子列表，分页）→ 帖子详情/列表中标签可点。实现路线 = **发帖时后端解析落库 + 渲染时前端高亮 + `/tag/:name` 聚合页**。

## §2 真值核对（数据来源，全部可复现）

### 2.1 服务器真值（2026-08-24 实查，SSH `ssh -p 2222 liyongquan@122.51.233.225 'cd ~/ai-forum && git log --oneline -1 && docker compose ps && curl -s http://localhost:8888/healthz'`）

| 项 | 实查结果 | 含义 |
|---|---|---|
| HEAD | `0301601 Merge pull request #52 from li-yongqvan/fix/deploy-ssh-port-2222` | 线上 = PR #52（SSH 端口迁移） |
| 容器 | `ai-forum-api Up (healthy)`、`ai-forum-postgres Up (healthy)`、`ai-forum-web Up` | 8888 容器化部署形态正常 |
| healthz | `{"status":"ok"}` | 服务正常 |

→ 与本机 `git log` 一致（本地 main = `0301601`），可安全在其上建分支。

### 2.2 数据库真值

- **生产 DB 未实查**（本票为**新建表**，无既有结构需核实；0007 迁移由应用启动自动应用）。若评审要求，可用 SOP 命令实查：`ssh -p 2222 ... 'cd ~/ai-forum && docker compose exec -T postgres psql -U forum forum'`。**「未复核（待复核）」**。
- 代码佐证：全库 grep 无 `Tag`/`PostTag` 模型、无标签表迁移（Plan agent 实证，`backend/content/`、`backend/migrations/` 0001-0006）。

### 2.3 代码真值（本机仓库实读/grep）

**content 域（`backend/content/`）**：
- `service.go:22-41` `PostView` 读模型：`ID/BoardID/BoardName/TopicID/TopicName/AuthorID/AuthorName/Title/Content/IsPinned/IsFeatured/ViewCount/LikeCount/CommentCount/FavoriteCount/Viewer/CreatedAt`——**无 `tags` 字段**。
- `service.go:110-118` `ListFeedQuery`：`Tab/ViewerID/AuthorID/BoardID/TopicID/Page/PageSize`——可加 `Tag *string`。
- `service.go:269-296` `CreatePost`：校验 board/topic → `repo.CreatePost(p)` → `postView`。**标签解析插入点**。
- `service.go:495-542` `ListFeed`：page/pageSize 归一 → `PostQuery` → `repo.ListPosts` → `postViews`。
- `service.go:744-843` `postViews` 批量组装：话题名富化在 `798-803`，循环拼装 `805-841`。**标签富化插入点**（batch `ListTagsByPostIDs` 后 `view.Tags = tagsByPost[p.ID]`）。
- `repo.go:6-16` `PostQuery`：`Feed/AuthorID/BoardID/TopicID/FollowedUserIDs/FollowedBoardIDs/FollowedTopicIDs/Offset/Limit`；`repo.go:19-73` `Repo` 接口（boards/topics/posts/comments/likes/favorites/follows 全组方法）。
- `gorm_repo.go:84-112` `ListPosts`：`Where` 过滤链 + `Order("is_pinned DESC").Order("created_at DESC")` + 有界 Limit/Offset。
- `gorm_repo.go:114-116` `DeletePost`：软删（`r.db.Delete(&Post{}, id)`）。
- `gorm_repo.go:346-364` `ListFavoritedPosts`：`Select("content.posts.*")` + `Joins` + `Order(f.created_at DESC)`——**JOIN 列表先例**。

**HTTP 层**：
- `internal/httpapi/handler/content.go:96-126` `ListPosts`：解析 page/page_size + 可选 `author_id/board_id/topic_id` → `ListFeedQuery` → `ListFeed` → `{"items", "page", "page_size"}`。**`?tag=` 在此加解析**。
- `internal/httpapi/router.go:98-105`：`reads` 组（OptionalAuth，游客可读）：`GET /posts`@102 等。→ **无需新路由**。
- `internal/httpapi/middleware/auth.go:41/55`：`OptionalAuth` / `Identity`（游客 id 0）。

**迁移机制**：
- `backend/migrations/`：0001-0006（0006 为 additive ALTER 格式范本）。下一文件 `0007_tags_schema.sql`。
- `backend/migrations/migrate.go:16/20`：`//go:embed *.sql` + `Run`（词典序、`;` 分隔、事务、幂等）。
- `cmd/api/main.go:41`：`migrations.Run(sqlDB)` 启动自动应用；`internal/testutil/db.go:83`：`SetupPG` 内同样跑 `migrations.Run`（**handler 集成测试自动应用 0007**）。

**测试基建**：
- `backend/content/fake_repo_test.go:14-31` `fakeRepo`：内存 map（boards/topics/posts/comments/likes/favs/followsB/followsT）+ 单调时钟。**实现 Repo 全接口——必须补两个新方法，否则包不编译**。
- `backend/internal/httpapi/handler/content_test.go:15-31` `postView` 测试结构体：无 `tags` 字段，需补。
- `backend/content/service_test.go`：表驱动 + fake，含 `TestListFeed_*` 列表页先例。

**前端（`frontend/src/`）**：
- `utils/md.ts`：手写正则渲染器。`esc()` 先转义（`5-11`，顺序 `&→&amp; <→&lt; >→&gt; "→&quot;`）；占位符 `pushBlock`（`27-30`）；规则顺序：围栏代码（`33-35`）→ 行内代码（`37`）→ 外链图片 `content-img`（`39-41`）→ `[text](url)`（`43-45`）→ 裸 URL（`47`）→ 粗体（`49`）→ 段落（`51-54`）；`plainText` 摘要（`61-70`）。
- `styles/global.css`：`button{min-height:44px}`@52；`.md`@70；`.md a{border-bottom:1px dashed var(--brand-2)}`@101-104；`.md .content-img{cursor:zoom-in}`@114。
- `components/PostCard.vue`：`.chips` 行@32-37 + `.chip` 样式@96-103；整卡 `@click="go"`（`24`）。
- `views/TopicDetail.vue`：聚合页模板——`fetcher` 闭包 route param（`15-16`）+ `PostList :fetcher :empty-title`（`54`），无独立 loading/error 包装。
- `components/PostList.vue`：props 仅 `{ fetcher:(page,pageSize)=>Promise<{items:Post[]}>, emptyTitle? }`。
- `router/index.ts`：hash 历史；routes@6-22；catch-all@21。
- `api/content.ts:27-34` `ListPostsParams`、`36-45` `listPosts`（URLSearchParams）；`55-57` `createPost`（payload 无 tags）。
- `api/types.ts:41-60` `Post`：无 `tags` 字段。
- `views/Write.vue:119-124`：提交 `{board_id, topic_id, title, content}`——**无 tags 字段，无需改**。
- `components/CommentTree.vue:81`：`.cbody` 用 `md(c.content)`——**评论正文标签免费生效**。
- `views/PostDetail.vue:355`：正文 `v-html="md(post.content)" @click="handleContentImageClick"`。

### 2.4 GitHub 状态

- issue **#54**（本票）OPEN，正文 = handoff（2026-08-24 `gh issue view 54`）。
- 地图 **#1** 已读：决策 21（#23 收藏列表）、#54 已登记 Roadmap「待取票」（`gh issue view 1`）。**本会话不 edit #1（统筹方更新）**。
- 本地 main = 远端 main = `0301601`（无超前/落后）。

## §3 Grilling 决策记录（7 项，用户确认 2026-08-24）

| 编号 | 决策问题 | 定案 | 依据 |
|---|---|---|---|
| D1 | 落库 vs 纯展示 | **落库**（新建 `content.tags` + `content.post_tags`） | 聚合页需真实数据/计数；用户确认（2026-08-24） |
| D2 | 标签规则 | **微博式**：`#`+连续中文/字母/数字/下划线 ≤30 字；大小写不敏感（归一化小写）；`#` 前须为行首/空白/标点 | 用户确认（2026-08-24） |
| D3 | 解析时机 | **发帖时后端解析落库 + 渲染时 md.ts 高亮**（前后端同规则） | 落库 D1 的必然；用户确认 |
| D4 | 列表卡片 | **PostCard 显示标签胶囊**（后端 `PostView.tags` 批量带出） | 贴合 ticket 目标 3「详情/列表中可点」；用户确认 |
| D5 | 聚合排序 | **置顶优先 + 发布时间倒序**（与现有列表一致） | 行为可预期，复用 `ListPosts` 排序；用户确认 |
| D6 | 存量回填 | **不回填**（标签只对之后新帖生效） | 帖子不可编辑、标签只发帖时解析；老帖 `#词` 照常渲染可点；用户确认 |
| D7 | 解析范围 | 仅正文 content，不含 title | ticket「发帖正文里 #关键词」；用户确认 |

**备选与为何不选**：D2 备选「宽松（到空白任意字符）」「严格（仅中文/字母/数字 ≤20 且纯数字不算）」——宽松易误判、严格过保守；微博式是用户明示方向。

## §4 范围收敛与明确不做

| 项 | 决策 | 依据 |
|---|---|---|
| 标签输入/选择器 | **不做**（正文直接打 `#词`） | ticket 明示「正文里 #关键词 自动识别」 |
| 帖子编辑 | **不做**（本项目无编辑功能，`Repo` 无 UpdatePost） | §2.3（gorm_repo.go 仅 Create/Delete） |
| 标签计数/热词榜/标签云 | **不做**（聚合页仅帖子列表） | ticket 范围；计数需额外查询 |
| 存量回填 | **不做** | D6 |
| 大小写敏感聚合 | **不做**（归一化小写） | D2 |
| 富文本/投票/附件/@提及 | **不做**（标签清单内独立项） | ticket 依赖节 |
| `Write.vue`/`CommentTree.vue` | **不改**（Write 无 payload 变更；评论标签经 md.ts 免费高亮但**只亮不聚**——评论不落库，见上方 F6 登记） | §2.3 |

**规格澄清（登记）**：grilling 选项文字称边界规则「避免误伤 #1楼、#ff0000」——实核（评审方实证）该规则**只能**挡 `C#`/`我在#1楼`/`C#和#AI` 这类 `#` 前是字母的内联误伤；`color:#ff0000`（冒号=边界）、行首 `#1楼`、`他说"#AI很重要"`（引号=边界）**仍会识别**为标签。**用户已确认采用字面规则**，行为以测试钉住（§8 已修正为实证值），此为本节登记项。若未来产品希望「引号内标签不识别」，需把 `"` 排除出边界类（超出字面规则范畴，须回用户确认，F2 附注）。

**评论标签「只亮不聚」（F6 登记）**：`CommentTree.vue:81` 走 `md(c.content)` → 评论里 `#词` 会渲染成可点锚点，但后端只在**发帖**时索引帖子，评论不落库 → 点评论标签进聚合页看不到该评论。取舍：评论标签**仅高亮不聚合**（MVP 接受；若让评论渲染关闭标签规则需给 `md()` 加选项，成本更高，不推荐）。

## §5 实现方案（每项给出依据）

### 5.1 后端（`backend/`）

1. **迁移 `migrations/0007_tags_schema.sql`**（新增）：`content.tags(id BIGSERIAL PK, name VARCHAR(60) NOT NULL UNIQUE, created_at)` + `content.post_tags(id, tag_id FK→tags ON DELETE CASCADE, post_id FK→posts ON DELETE CASCADE, created_at, UNIQUE(tag_id,post_id))` + `idx_post_tags_post`/`idx_post_tags_tag`。
   - 依据：物理 FK 仅包内（§2.3 头注释）；大小写不敏感由「存储归一化小写 + UNIQUE(name)」实现（D2）。
2. **`content/tags.go`**（新增）：`NormalizeTag`（trim+小写）；`escapeHTML`（同 md.ts esc 顺序）；`stripBlocks`（围栏代码→行内代码→外链图→`[text](url)`→裸 URL，各替换为空格）；`ParseTags`（escape → strip → **手动扫描镜像 JS 替换推进**，提取组2 归一化+去重保序）。
   - 依据：正则一致性三坑（§6.1）；剥离顺序镜像 md.ts（§2.3）。
3. **`content/model.go`**：`Tag{ID,Name,CreatedAt}`、`PostTag{ID,TagID,PostID,CreatedAt}`——均**无 DeletedAt/UpdatedAt**（写一次、无软删 → 子查询零 scope）。
   - 依据：与 `Topic`（含软删）刻意区分——post_tags 是纯关联，硬删即可（§6.2）。
4. **`content/repo.go`**：`PostQuery` 加 `Tag *string`；`Repo` 接口加 `ReplacePostTags(ctx, postID int64, tagNames []string) error`、`ListTagsByPostIDs(ctx, postIDs []int64) (map[int64][]string, error)`。
   - 依据：复用现有 `ListPosts` 排序/分页形态（置顶+时间倒序，§2.3 gorm_repo.go:84-112），tag 过滤作为 `PostQuery` 可选字段并入、**非新增独立分页列表方法**；`ReplacePostTags`/`ListTagsByPostIDs` 为写/读 seam 方法；子查询免 JOIN 规避列遮蔽（#23 先例，§2.3）。F9 修正。
5. **`content/gorm_repo.go`**：
   - `ListPosts`：`in.Tag != nil` 时 `Where("id IN (SELECT pt.post_id FROM content.post_tags pt JOIN content.tags t ON t.id = pt.tag_id WHERE t.name = ?)", *in.Tag)`——主查询无 JOIN，无需改 Select（规避 #23 列遮蔽）。
   - `ReplacePostTags`：`db.Transaction` 内 delete 旧 post_tags → 每 tag `clause.OnConflict{DoNothing:true}` upsert + 重查 id → 批量插 post_tags。
   - `ListTagsByPostIDs`：`Table("content.post_tags")` + JOIN tags + 限定 Select + `Where post_id IN ?` + `Order(pt.id ASC)` → group。
   - `DeletePost`：软删 + 硬删该帖 `PostTag` 同包 tx。
   - 依据：并发 upsert 竞态安全（§6.3）；子查询索引路径 `tags.name`(UNIQUE)→`post_tags.tag_id`→`post_tags.post_id`。
6. **`content/service.go`**：
   - `CreatePost`：`repo.CreatePost` 后 `tags := ParseTags(in.Content)`；非空则 `repo.ReplacePostTags`；**失败 `log.Printf` 不阻断发帖**（§6.4）。
   - `ListFeedQuery` 加 `Tag *string` → 透传 `PostQuery.Tag`。
   - `PostView` 加 `Tags []string json:"tags,omitempty"`；`postViews` 在话题名富化后 batch `ListTagsByPostIDs`，循环 `view.Tags = tagsByPost[p.ID]`（nil → omitempty 省略）。**读侧 best-effort（F7）**：`ListTagsByPostIDs` 失败仿 `topicNames` 的 `if err == nil` 处理，仅 tags 缺省、不拖垮整个列表。另注（评审 2.1 确认）：`postView`(735-741) 内部复用 `postViews` → 单帖/CreatePost 响应**自动获得** tags 富化，无需单帖分支。
   - 依据：富化插入点（§2.3 798-841）；所有列表一次带出（D4）。
7. **`handler/content.go`** `ListPosts`：`if v := c.Query("tag"); v != "" { if t := content.NormalizeTag(v); t != "" { q.Tag = &t } }`。复用 `GET /posts`。
   - 依据：reads 组游客可读（§2.3）；空/非法 tag 不过滤（§7）。
8. **`content/fake_repo_test.go`**：补 `tags/tagPosts` map + 两新方法 + 可选 `errReplaceTags` + `ListPosts` tag 过滤 + `DeletePost` 清 tagPosts。
   - 依据：fake 实现全接口（§2.3），否则包不编译。

### 5.2 前端（`frontend/src/`）

9. **`utils/md.ts`**：`const TAG_RE = /(^|[^\p{L}\p{N}_#])#([\p{L}\p{N}_]{1,30})/gu`；在裸 URL 规则后、粗体前插入：`s.replace(TAG_RE, (_m, lead, tag) => lead + pushBlock('<a class="tag" href="#/tag/' + encodeURIComponent(tag.toLowerCase()) + '">#' + tag + '</a>'))`；导出 `extractTags(src)`（esc → 5 步剥离 → mutation 式 `replace` 收集，返回 `' '`）。
   - 依据：占位符机制防二次处理（§2.3 md.ts 头注释）；`extractTags` 用 mutation 式保证与 `md()` 一致（§6.1）；锚点不加 `target="_blank"`（外链专属，hash 路由原生导航）。
10. **`styles/global.css`**：`.md a.tag`（`background: var(--brand-soft); color: var(--brand); border-bottom:none; cursor:pointer` 等胶囊样式）。
    - 依据：`.md a` 现有虚线下划线需覆盖（§2.3 global.css 101-104）。
11. **`api/types.ts`**：`Post` 加 `tags?: string[]`（可选，保住现有测试 mock）。
12. **`api/content.ts`**：`ListPostsParams` 加 `tag?: string`；`if (p.tag) qs.set('tag', p.tag)`。
13. **`router/index.ts`**：catch-all 前加 `{ path: '/tag/:name', component: () => import('../views/TagDetail.vue'), meta: { title: '标签' } }`。
14. **`views/TagDetail.vue`**（新增，克隆 TopicDetail 去关注）：`name = computed(() => String(route.params.name).trim().toLowerCase())`；`fetcher` 带空名守卫 `if (!name.value) return { items: [] }`（**`.trim()` 关键**：`%20` 解码为空格，trim 后触发守卫，防 `/tag/%20` 列出全部，F3）；头部 `# {{ name }}`；`<PostList :fetcher="fetcher" empty-title="还没有这个标签的帖子" />`。
    - 依据：TopicDetail 模板 + PostList 零改动（§2.3）；空名守卫防 `/tag/%20` 列出全部（§7）。
15. **`components/PostCard.vue`**：`.chips` 加 `<a v-for="t in post.tags ?? []" :key="t" class="chip tag" :href="'#/tag/' + encodeURIComponent(t)" @click.stop>#{{ t }}</a>`。
    - 依据：胶囊用 `<a>` 不用 `<button>`（全局 `button{min-height:44px}`@52 会撑爆；`<a>` 键盘可达 + hash 导航 + `@click.stop` 挡卡片 `go`，§6.5）。

### 5.3 测试（详 §8）

## §6 关键设计裁决（【决策】，含理由与备选）

### 6.1 正则一致性：两引擎语义天然对齐，无需「替换推进」镜像（F1/F2 修正）
- **问题**：前后端必须同一标签规则；Go RE2 无 lookbehind/lookahead，需用捕获组边界回放。
- **定案【决策】**（实测修正，评审方 Node v24 / Go 实证）：
  ① **两侧都基于原始输入做非重叠匹配**——JS `String.replace(全局正则, cb)` 与 Go `FindAllStringSubmatch` 语义一致：在原始串上从左到右找全部匹配，**替换文本不参与匹配、不存在「替换推进」**。`#a#b` 中第二个 `#` 前是字母 `a`（非边界）→ **两侧都只提取 `[a]`**。Go **用单次 `FindAllStringSubmatch`，不做切片推进**（切片推进会每次重建 `^` 串首锚点而**多**提取，是错误机制，方向与 JS 相反）。
  ② **无需为边界对称做 `escapeHTML`**：esc 只变换 `&<>"`，其实体以 `;` 结尾（`;` 是标点=边界），esc **不改变任何 `#` 的边界归属**——`他说"#AI很重要"` 引号前/`&quot;` 的 `;` 前都是边界，两侧都识别 `[ai很重要]`。esc 仅是前端 XSS 防护（§2.3 md.ts:5-11），后端解析跑**原始 content**。
  ③ 剥离的 markdown 构造（围栏代码→行内代码→外链图→链接→裸 URL）**替换为空格**，与前端占位符 `\x00N\x00` 边界等价（空格是边界；占位符内部数字从不贴 `#`）。
- **理由**：字面规则（D2）下两引擎**天然一致**；此前设计误判「JS 替换推进」，实测推翻并连带修正两处测试期望（§8）。
- **备选（不选）**：Go 切片推进「镜像替换」（实测制造分歧）；客户端传标签给后端（破坏后端校验、信任客户端）。
- **同步机制**：`tags.go` 顶部 TAG_RE 常量 + `md.ts` 注释**双向同步契约**（改一侧须改另一侧），两侧各放「与对侧保持同步」变更说明；同组用例双测作为 CI 断言（改规则时两侧各自跑红即暴露，Q3 裁决）。

### 6.2 post_tags 硬删 vs 软删
- **定案【决策】**：`post_tags` 与 `tags` **无 DeletedAt**，帖子软删后由 `DeletePost` 硬删关联行。
- **理由**：关联是派生元数据，无恢复需求；无软删 → 子查询零 scope，`tags.name` UNIQUE 不被软删行污染。
- **备选**：给 post_tags 加软删（保护历史）——无编辑/恢复功能，纯负担。

### 6.3 标签 upsert 并发安全
- **定案【决策】**：`ON CONFLICT DO NOTHING` + 重查 id，而非「先查后插」。
- **理由**：两个并发发帖同用新标签时，check-then-insert 会竞态撞 UNIQUE。
- **备选**：直接插不冲突处理（撞 UNIQUE 报错导致 ReplacePostTags 失败）——不可接受。
- **残余风险（F8）**：两并发发帖以**不同顺序**插入同组新标签时（A 先插 x 再 y、B 先插 y 再 x），唯一索引等待可形成 PG 死锁（40P01），一方 tx 被 abort → 该帖缺标签（被 §6.4 兜底，可接受）。**低成本缓解：批量插入前按标签名排序（稳定锁序）**。

### 6.4 标签写失败处理：log-and-continue
- **问题**：`repo.CreatePost` 提交后帖子已存在，二次调 `ReplacePostTags` 无法跨调用原子。
- **定案【决策】**：标签写失败 `log.Printf` 记录、**不阻断发帖**（标签是派生元数据）。
- **理由**：若 fail 请求，客户端收到「发帖失败」但帖子实际已存在 → 重试产生重复帖，危害远大于「帖子缺标签」。
- **备选（不选）**：新增 `CreatePostWithTags` 事务化（帖子+标签原子）——需改 `Repo` 接口 + fake，且现有 CreatePost 无 tx 先例；收益仅「DB 写失败时缺标签」这一罕见情形。
- **残余风险**：帖子存在但标签缺失的短暂不一致——列入 Q2。

### 6.5 PostCard 胶囊形态：`<a>` 而非 `<button>`/`<span>`
- **定案【决策】**：胶囊用 `<a :href="'#/tag/' + encodeURIComponent(t)" @click.stop>`。
- **理由**：全局 CSS `button{min-height:44px}`（§2.3 global.css:52）会把胶囊撑爆成整行触达；`<a>` 原生 hash 导航 + 键盘可达 + `@click.stop` 挡卡片 `go`。
- **备选**：`<span @click.stop>`（需 JS 导航，键盘不可达）；`<button>`（撞 44px 布局）。

### 6.6 大小写显示策略
- **定案【决策】**：**存储/查询/聚合 URL 一律归一化小写**；正文渲染保留原文大小写（`md()` 输出 `<a>#AI</a>` 但 href=`#/tag/ai`）；胶囊与聚合页头显示小写。
- **理由**：D2 大小写不敏感；存原大小写需 `UNIQUE(LOWER(name))` 表达式索引 + 全查询 LOWER 对比，复杂度不值。
- **残余观感**：正文 `#AI` vs 胶囊 `#ai` 大小写不一——列入 Q4。

## §7 边界与不变量清单（含防护层）

| # | 不变量 | 防护层 | 依据 |
|---|---|---|---|
| 1 | 前后端提取的标签集合完全一致 | 同正则 + Go escape-first + 手动扫描镜像 + 同组用例双测 | §6.1/§8 |
| 2 | `##ai` 不识别 | `#` 排除出边界类 + `#` 前非边界 | §2.3 TAG_RE |
| 3 | `#a#b` 只识别第一个（第二个 `#` 前是字母，非边界，D2 一致） | 单次 `FindAllStringSubmatch` / JS `replace` 原始串非重叠匹配 | §6.1/§8 |
| 4 | 代码块/行内码/链接 URL 内的 `#` 不识别 | 剥离规则先于标签规则（两侧同序） | §2.3 md.ts 33-47 |
| 5 | `C#`/`我在#1楼` 不识别 | 边界类排除字母/数字/下划线 | §2.3 TAG_RE |
| 6 | `#AI` 与 `#ai` 归同一聚合页 | 存储+查询均 `NormalizeTag` 小写 | D2/§5.1-7 |
| 7 | 同一帖子不重复关联同一标签 | `UNIQUE(tag_id, post_id)` | §5.1-1 |
| 8 | 并发创建同新标签不报错 | `ON CONFLICT DO NOTHING` | §6.3 |
| 9 | 标签写失败不阻断发帖 | `log.Printf` + 不返回错误 | §6.4 |
| 10 | 软删帖不出现于标签聚合 | 主查询 GORM `deleted_at IS NULL` scope | §2.3 gorm_repo |
| 11 | 删帖不留孤儿 post_tags | `DeletePost` 同 tx 硬删 | §6.2 |
| 12 | `/tag/%20`（空名）不列出全部 | TagDetail name `.trim()` + 空名守卫返回 `{items:[]}` | §5.2-14/F3 |
| 13 | `?tag=`/`?tag=%20`（空）不过滤返回全部；**不存在的标签 → 空列表（正常空态）** | handler 归一化后空不设过滤；非空则过滤（不存在即空态） | §5.1-7/F5 |
| 14 | 标签文本无 XSS | md.ts 先 `esc()`；href 恒为站内 `#/tag/...`；无 scheme | §2.3 md.ts 5-11 |
| 15 | 胶囊点击不触发卡片跳帖 | `@click.stop` | §5.2-15 |
| 16 | 超长 `#`+31字 前后端行为一致 | 量词 `{1,30}` 双侧截断一致 | §2.3 TAG_RE |
| 17 | 标签顺序 = 正文首次出现序 | `pt.id` 升序 = 解析插入序 | §5.1-5 |
| 18 | 引号/标点前 `#` 是标签（`他说"#AI很重要"`→`[ai很重要]`、`color:#ff0000`→`[ff0000]`） | 字面规则：`"`/`;`/`:` 均在边界类 | §6.1/§8 |

## §8 测试与验证计划（到用例级）

**后端单测 `content/tags_test.go`**（表驱动 ParseTags/NormalizeTag，**期望以评审实证为准，F1/F2 修正**）：中文/ASCII、大小写归一+去重保序、下划线数字、`C#和#AI`→`[]`（`和` 是字母）、`#1楼`→`[1楼]`、`color:#ff0000`→`[ff0000]`（钉住 §4 规格澄清）、`他说"#AI很重要"`→`[ai很重要]`（`"`/`;` 是边界，F2）、`##ai`→`[]`、`#a#b`→`[a]`（只识别首个，F1）、`` `#code` ``→`[]`、`[链接](https://x#frag)`→`[]`、`https://x.com#ai`→`[]`、`#`+35 字截 30、`# tag`→`[]`、`#😀`→`[]`、`\n#AI`→`[ai]`、`NormalizeTag(" AI ")`→`ai`。

**后端 service_test 扩展**：`TestCreatePostStoresTags`（`#AI #RAG`→view.Tags `[ai rag]`+fake 落库）；无标签 Tags=nil；`f.errReplaceTags` 置位→CreatePost 仍返回 view（§6.4）；`TestListFeedByTag`（置顶+最新序、软删排除、空→`[]` 非 nil）。

**后端 handler 集成 `content_test.go`（testcontainers）** `TestTagsFlow`：注册→发 `#RAG #Agent` 帖→201 带 `tags`；`?tag=rag` 命中；`?tag=AI` 大小写不敏感命中；无标签帖 `tags` 缺省；`?tag=nonexistent`→空；删帖后不再命中。**`postView` 测试结构体补 `Tags` 字段**。

**前端 `md.test.ts` 扩展**：`extractTags("今天 #AI #RAG")`→`["ai","rag"]`；`C#和#ai`→`[]`；`` `#code` ``→`[]`；`#a#b`→`["a"]`（只识别首个，与后端一致，F1）；`md("#AI")` 含 `<a class="tag" href="#/tag/ai">#AI</a>`；`md("C#")` 无锚点；`md("#<script>")` 转义无锚点（XSS）；`md("#a#b")` 单锚点；`md('他说"#AI"')` 含锚点 `#/tag/ai`（引号边界，F2）。

**前端 `views/TagDetail.test.ts`**（仿 PostDetail.test.ts vi.mock vue-router/vant/auth/api）：fetcher 传 `{tag:'ai',page,pageSize}`；头部 `# ai`；空名守卫；空态标题。

**可选 `components/PostCard.test.ts`**：胶囊点击 → `router.push('/tag/ai')` 且不触卡片 `go`。

**可复现命令**：
```
cd "C:\Users\liyongquan\ai-forum\backend" && go build ./... && go vet ./... && go test ./...
cd "C:\Users\liyongquan\ai-forum\frontend" && npm run test:unit && npm run build
git checkout -- frontend/components.d.ts   # build 改写，提交前还原
```
迁移自动验证：`testutil.SetupPG` 每测试容器跑 `migrations.Run`（§2.3）。本地浏览器冒烟：发 `#中文`/`#AI`/`#a#b` 帖 → 正文高亮可点 → 点标签进 `/tag/ai` → 聚合页出帖 → PostCard 胶囊 → 评论 `#词` 高亮。

## §9 待评审焦点（Q1-Q8，已评审裁决）

> 已由独立评审 agent 逐条裁决（2026-08-24，见评审意见书 §四），裁决**全部采纳**：Q1 可接受附条件（先修正 F2 测试期望）/ Q2 认可附 2 条精确化（F7 读侧 best-effort + F8 死锁文档化）/ Q3 按 F1 甲案改道（§6.1）/ Q4 认可 / Q5 认可 / Q6 确认无溢出（前提上限保持 30，上调需同步放宽 VARCHAR）/ Q7 认可附注（未来加恢复需重解析正文）/ Q8 认可。落地见 §10。

## §10 评审意见采纳记录（2026-08-24）

| 评审项 | 结论 | 采纳落地 |
|---|---|---|
| **F1**（阻塞）`#a#b` 机制方向相反 + 测试期望不可满足 + invariant 3 与 D2 矛盾 | 属实（Node/Go 实证） | 采纳甲案：Go `ParseTags` 改**单次 `FindAllStringSubmatch`**，删切片推进；§6.1②/不变量 3 改「只识别首个」；前后端 `#a#b` 测试期望改 `[a]`（§6.1/§7 #3/§8） |
| **F2**（阻塞）两处测试期望与字面规则相反 + 「esc 校正边界」前提不成立 | 属实 | 采纳：`他说"#AI很重要"`→`[ai很重要]`、`C#和#AI`→`[]` 改实证值；§6.1① 改写为「esc 仅 XSS 防护，边界归属不变」（§6.1/§8） |
| **F3**（重要）`/tag/%20` 空名守卫失效、invariant 12 落空 | 属实 | 采纳：TagDetail `name` 加 `.trim()` + `/tag/%20` 守卫测试（§5.2-14/§7 #12） |
| **F4**（重要）handoff untracked + grilling 决策记录未入库 | 属实 | 采纳：新建 `docs/handoffs/grilling-decisions/issue-54-tag-system-decisions.md`；handoff + 计划 + 设计文档 + 评审意见书随 PR 一并提交 |
| **F5**（建议）invariant 13 与实现矛盾 | 属实 | 采纳：invariant 13 改为「空 tag 不过滤；不存在标签→空列表」（§7 #13） |
| **F6**（建议）评论标签「只亮不聚」未登记 | 属实 | 采纳：§4 范围节登记取舍（§4） |
| **F7**（建议）读侧 tags enrich 失败行为未指定 | 属实 | 采纳：`postViews` 仿 `topicNames` best-effort，tags 缺省不拖垮列表（§5.1-6） |
| **F8**（建议）`ReplacePostTags` 多标签并发死锁窗口 | 属实 | 采纳：§6.3 文档化残余风险 + 批量插入前按标签名排序（稳定锁序）缓解（§6.3） |
| **F9**（建议）§5.1-4 依据引用不贴切 | 属实 | 采纳：依据改写为「复用 ListPosts 形态 + 子查询免 JOIN」（§5.1-4） |

**推翻项**：无。全部评审发现经独立复核属实（评审方本地仓库 @ 0301601 + Node v24 / Go 正则实证）。评审未复核项（生产服务器实查、用户确认溯源）均已如实标注并在开工时重查。

---

*本文档为实现基线。评审（2026-08-24 有条件通过）完成，可进入实现（分支 → 实现 → 测试全绿 → PR → 用户授权 → 合并 → 部署 → 服务器验证 → 回报统筹方更新地图 #1）。*
