# #54 标签系统（#hashtag）：正文内嵌标签 + 聚合页

> 计划文档 · 2026-08-24 · 实现会话产出（grilling 决策已与用户逐项确认）。handoff：`docs/handoffs/54-tag-system.md`；ticket：issue #54；地图：#1。
> 状态：待用户审阅（升格可审计设计文档 + plan-review 评审后进入实现）。
>
> ⚠️ **更正（评审后）**：本计划文档「关键机制」中的**正则一致性 Pitfall A/B 已被评审实测推翻**（评审意见书 F1/F2：JS `String.replace` 与 Go `FindAllStringSubmatch` 天然一致，`#a#b`→`[a]`、`他说"#AI很重要"`→`[ai很重要]`、`C#和#AI`→`[]`；无需 escape 校正边界、无需切片推进）。**以 `54-tag-system-设计文档.md`（评审后有条件通过）为执行基线**，本文档对应段落仅作参考。

## Context（为什么做）

社区发帖目前只能用「板块 → 话题」两级组织内容，缺少更轻量的标签维度。用户选定微博式 **#hashtag**：正文里写 `#关键词` 自动识别为**可点击标签**，点标签进入**聚合页**（该标签下帖子列表，分页）。完整功能清单中「标签」为延后项，现补。

目标：
1. 发帖正文中 `#关键词` 自动识别为可点击标签；
2. 点标签 → 标签聚合页（该标签下帖子列表，分页）；
3. 帖子详情/列表中标签可点。

实现路线：**发帖时后端解析正文落库 → 渲染时前端高亮可点 → `/tag/:name` 聚合页**。规模中大（后端 content 域 + 迁移 + 前端），先设计后实现。

真值核对（2026-08-24）：服务器 `main`=`0301601`（与本地一致）、api/postgres healthy、healthz ok；`docs/workflow.md`（SOP）+ `docs/handoffs/54-tag-system.md` 已读；地图 #1 已读（#54 已登记 Roadmap「待取票」）。本会话不动地图 #1（统筹方更新）。

## 已与用户确认的决策（2026-08-24 grilling）

| # | 项 | 决策 |
|---|---|---|
| D1 | 落库 | **是**。新建 `content.tags` + `content.post_tags`（聚合页需真实数据） |
| D2 | 标签规则 | **微博式**：`#`+连续中文/字母/数字/下划线，≤30 字；大小写不敏感（归一化小写存储，`#AI`=`#ai`）；`#` 前须为行首/空白/标点才识别 |
| D3 | 解析时机 | **发帖时后端解析落库 + 渲染时 md.ts 高亮**（前后端共用同一规则） |
| D4 | 列表卡片 | **显示胶囊**：PostCard 加标签胶囊（后端 `PostView.tags` 批量带出） |
| D5 | 聚合排序 | **置顶优先 + 发布时间倒序**（与现有列表一致） |
| D6 | 存量回填 | **不回填**（标签只对之后新帖生效；存量正文 `#词` 照常渲染可点） |
| D7 | 解析范围 | 仅正文 content，不含 title |

**诚实澄清（边界规则的能力边界）**：D2 的边界规则能挡 `C#`、`我在#1楼`（`#` 前是字母）这类**内联**误伤；但 `color:#ff0000`（冒号是标点=边界）、行首 `#1楼` **仍会被识别**为标签。这与 grilling 选项描述「避免误伤 #1楼、#ff0000」不完全一致，但符合真实 hashtag 行为（微博同理）。采用字面规则并以该行为写测试钉住。

## 改动文件（范围红线）

- **后端**：`backend/migrations/0007_tags_schema.sql`（新增）；`backend/content/tags.go`（新增）；`backend/content/{model,repo,gorm_repo,service}.go`；`backend/internal/httpapi/handler/content.go`（仅 ListPosts 加 `?tag=` 解析）。
- **前端**：`frontend/src/utils/md.ts`（TAG_RE 渲染 + 导出 extractTags）；`frontend/src/styles/global.css`（`.md a.tag`）；`frontend/src/api/{content,types}.ts`；`frontend/src/router/index.ts`；`frontend/src/views/TagDetail.vue`（新增）；`frontend/src/components/PostCard.vue`。
- **测试**：后端 `tags_test.go`（新增）+ `service_test.go` 扩展 + `fake_repo_test.go` 补方法 + `content_test.go` 加 `TestTagsFlow`；前端 `md.test.ts` 扩展 + 新增 `TagDetail.test.ts`（可选 `PostCard.test.ts`）。
- **不碰**：`notify/moderation`、`.github/workflows/`、`Write.vue`（无 payload 变更）、`CommentTree.vue`（标签在评论里经 md.ts 免费生效）。

## 关键机制（已核实，实现依据）

**标签规则单一共享源**：后端 `tags.go` 与前端 `md.ts` 用**同一正则语义** `(^|[^\p{L}\p{N}_#])#([\p{L}\p{N}_]{1,30})`（RE2 无 lookbehind，用捕获组边界回放；JS 需 `u` 标志）。两侧以同一组用例断言行为一致。

**正则一致性三个坑位（Plan agent 实证，必须处理）**：
1. **Pitfall A——替换推进不对称**：JS `String.replace` 边替换边推进，`#a#b` 里第二个 `#b` 前面变成占位符 `\x00`（是边界）→ JS 提取**两个**；朴素 Go `FindAllStringSubmatch` 评估未变字符串 → 只提取**一个**。**解法**：Go 用**手动扫描**——循环 `FindStringSubmatchIndex`，提取组2，`s = s[loc[1]:]` 推进（重锚定 `^` 扮演占位符的边界角色）。
2. **Pitfall B——转义不对称**：JS 跑在 `esc(src)` 转义后的字符串上；`"#AI"`（引号前）在原始串里引号是边界、在转义串里 `#` 前是 `t`（`&quot;` 末尾）不是边界 → JS 不识别。**解法**：Go `ParseTags` 先做与 `md.ts esc()` 同顺序的 HTML 转义（`&→&amp; <→&lt; >→&gt; "→&quot;`）再解析。标签字符集不含 `&<>"`，转义不影响提取结果。
3. **Pitfall C——剥离块占位等价**：Go 把 markdown 构造（围栏代码/行内代码/外链图/链接/裸 URL）各 `.ReplaceAllString(s, " ")` 替换为**空格**，与前端占位符 `\x00N\x00` 边界等价（空格是边界；占位符内部数字从不贴 `#`）。已验证。

**md.ts 剥离顺序（Go 必须镜像）**：围栏代码（L33-35）→ 行内代码（L37）→ 外链图片（L39-41）→ `[text](url)`（L43-45）→ 裸 URL（L47），最后插入标签规则（L47 后、粗体 L49 前）。

**GORM 子查询规避列遮蔽**：`ListPosts` 加 tag 过滤用 `WHERE id IN (SELECT pt.post_id FROM content.post_tags pt JOIN content.tags t ON t.id = pt.tag_id WHERE t.name = ?)`——主查询无 JOIN，无需 `Select("content.posts.*")`（规避 #23 列遮蔽教训）；GORM 自动给主查询加 `deleted_at IS NULL`（软删帖自动排除）。子查询索引路径：`tags.name`(UNIQUE) → `post_tags.tag_id` → `post_tags.post_id`。

**并发安全的标签 upsert**：`ReplacePostTags` 事务内 delete 旧关联 → 每 tag `clause.OnConflict{DoNothing:true}` upsert + 重查 id（两个并发发帖同新标签不竞态）→ 批量插 post_tags。排序 = 解析顺序（post_tags.id 升序）→ 读模型保「首次出现序」。

**事务性与错误处理（建议 log-and-continue）**：`repo.CreatePost` 提交后帖子已存在，二次调 `ReplacePostTags` 无法原子——若失败 fail 请求会诱导客户端重发造成重复帖。故标签写失败 `log.Printf` 记录、不阻断发帖（标签是派生元数据）。`ReplacePostTags` 自身在 `db.Transaction` 内保证关联写原子。

**聚合页复用**：`TagDetail.vue` 克隆 `TopicDetail.vue`（去关注按钮）；`PostList.vue` props 仅 `{ fetcher, emptyTitle }`，零改动；hash 路由下 `<a href="#/tag/x">` 天然可点（无需 JS 事件委托，#50 图片模式不需复刻）。

**胶囊用 `<a>` 不用 `<button>`**：全局 CSS `button { min-height: 44px }` 会把胶囊撑爆；`<a>` 键盘可达 + hash 原生导航 + `@click.stop` 挡卡片 `go`。

## 后端改动

### B1 迁移 `backend/migrations/0007_tags_schema.sql`（新增，embed 自动跑）

```sql
-- 0007 标签 schema（#54）
-- 归一化小写 name 唯一（大小写不敏感）；post_tags 一对多，两表物理 FK
CREATE TABLE content.tags (
  id BIGSERIAL PRIMARY KEY,
  name VARCHAR(60) NOT NULL UNIQUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE content.post_tags (
  id BIGSERIAL PRIMARY KEY,
  tag_id BIGINT NOT NULL REFERENCES content.tags (id) ON DELETE CASCADE,
  post_id BIGINT NOT NULL REFERENCES content.posts (id) ON DELETE CASCADE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tag_id, post_id)
);
CREATE INDEX idx_post_tags_post ON content.post_tags (post_id);
CREATE INDEX idx_post_tags_tag ON content.post_tags (tag_id);
```

`migrate.go` 按文件名词典序跑，`;` 分隔，应用启动 `cmd/api/main.go:41` 与测试 `testutil.SetupPG` 均自动应用。

### B2 新建 `backend/content/tags.go`

- `var tagRE = regexp.MustCompile(...)`（上述规则，RE2）。
- `NormalizeTag(s) = strings.ToLower(strings.TrimSpace(s))`。
- `escapeHTML(s)`：`&→&amp; <→&lt; >→&gt; "→&quot;`（同 md.ts esc 顺序）。
- `stripBlocks(s)`：5 个剥离正则按序 `.ReplaceAllString(s, " ")`。
- `ParseTags(content) []string`：`escapeHTML` → `stripBlocks` → **手动扫描**（Pitfall A 镜像）→ 组2 归一化 + 去重保序。
- 注释注明「与 `frontend/src/utils/md.ts` TAG_RE/extractTags 保持同步（两侧单测钉行为）」。

### B3 `backend/content/model.go`

`Tag{ID, Name, CreatedAt}` → `content.tags`；`PostTag{ID, TagID, PostID, CreatedAt}` → `content.post_tags`。**均无 DeletedAt/UpdatedAt**（写一次、无软删 → 子查询零 scope）。

### B4 `backend/content/repo.go`

`PostQuery` 加 `Tag *string`；`Repo` 接口加 `ReplacePostTags(ctx, postID int64, tagNames []string) error`、`ListTagsByPostIDs(ctx, postIDs []int64) (map[int64][]string, error)`。

### B5 `backend/content/gorm_repo.go`

- `ListPosts`：`if in.Tag != nil { q = q.Where("id IN (SELECT ... WHERE t.name = ?)", *in.Tag) }`。
- `ReplacePostTags`：`db.Transaction` 内 delete 旧 post_tags → upsert（OnConflict DoNothing）+ 重查 → 批量插。
- `ListTagsByPostIDs`：`Table("content.post_tags")` + JOIN tags + 限定 Select + `WHERE post_id IN ?` + `Order(pt.id ASC)` → group 成 map。
- `DeletePost`：软删 + 硬删该帖 `PostTag` 同包 tx（卫生；软删不触发 FK CASCADE）。

### B6 `backend/content/service.go`

- `CreatePost`：`repo.CreatePost` 后 `tags := ParseTags(in.Content)`；非空则 `repo.ReplacePostTags(ctx, p.ID, tags)`；**失败 log.Printf 不阻断**。
- `ListFeedQuery` 加 `Tag *string` → 透传 `PostQuery.Tag`。
- `PostView` 加 `Tags []string json:"tags,omitempty"`；`postViews` 批量富化：`tagsByPost := repo.ListTagsByPostIDs(postIDs)`（失败随兄弟富化调用返回 err）→ `view.Tags = tagsByPost[p.ID]`。

### B7 `backend/internal/httpapi/handler/content.go`

`ListPosts` 在 topic_id 解析后加：`if v := c.Query("tag"); v != "" { if t := content.NormalizeTag(v); t != "" { q.Tag = &t } }`。复用现有 `GET /posts`，**不新增路由**。

### B8 `backend/content/fake_repo_test.go`

**必须**补 `ReplacePostTags`/`ListTagsByPostIDs`（fake 实现全部 Repo 接口，否则包不编译）+ `tags/tagPosts` 字段 + 可选 `errReplaceTags` + `ListPosts` 的 tag 过滤 + `DeletePost` 清 tagPosts。

## 前端改动

### F1 `frontend/src/utils/md.ts`

```ts
const TAG_RE = /(^|[^\p{L}\p{N}_#])#([\p{L}\p{N}_]{1,30})/gu
// 在裸链接规则后、粗体前插入：
s = s.replace(TAG_RE, (_m, lead: string, tag: string) =>
  lead + pushBlock(`<a class="tag" href="#/tag/${encodeURIComponent(tag.toLowerCase())}">#${tag}</a>`),
)
```

- 导出 `extractTags(src)`：`esc` → 同 5 步剥离 → **mutation 式** `s.replace(TAG_RE, cb)` 收集（返回 `' '`，**不能用 matchAll**，否则与 `md()` 在 `#a#b` 分叉）。
- 标签锚点**不加** `target="_blank"`（外链专属）；hash 路由下 `#/tag/x` 直接导航。

### F2 `frontend/src/styles/global.css`

`.md a.tag`：品牌软色胶囊（`background: var(--brand-soft); color: var(--brand)`）、`border-bottom: none`（覆盖 `.md a` 虚线）、`cursor: pointer`、圆角/内边距。

### F3 `frontend/src/api/types.ts`

`Post` 加 `tags?: string[]`（可选，保住 PostDetail.test.ts 的 mockPost）。

### F4 `frontend/src/api/content.ts`

`ListPostsParams` 加 `tag?: string`；`if (p.tag) qs.set('tag', p.tag)`。

### F5 `frontend/src/router/index.ts`

catch-all 前加 `{ path: '/tag/:name', component: () => import('../views/TagDetail.vue'), meta: { title: '标签' } }`。

### F6 新建 `frontend/src/views/TagDetail.vue`

克隆 TopicDetail 去关注按钮：`name = computed(() => String(route.params.name).toLowerCase())`；`fetcher = (page,pageSize) => api.listPosts({ tag: name.value, page, pageSize })`，**空名守卫** `if (!name.value) return { items: [] }`（防 `/tag/%20` 列出全部）；头部 `# {{ name }}`；`<PostList :fetcher="fetcher" empty-title="还没有这个标签的帖子" />`。

### F7 `frontend/src/components/PostCard.vue`

`.chips` 加：`<a v-for="t in post.tags ?? []" :key="t" class="chip tag" :href="'#/tag/' + encodeURIComponent(t)" @click.stop>#{{ t }}</a>`。

## 测试清单

**后端单测 `content/tags_test.go`**（表驱动 ParseTags/NormalizeTag）：
- 中文/ASCII：`今天心情好 #开心`→`[开心]`；`#AI #RAG #Agent`→`[ai rag agent]`。
- 大小写归一+去重保序：`#AI 然后 #ai`→`[ai]`。
- 下划线数字：`#deep_learning #RAG2`→`[deep_learning rag2]`。
- 边界：`C#和#AI`→`[ai]`；`#1楼`→`[1楼]`；`color:#ff0000`→`[ff0000]`（**钉住 D2 文档化行为**）。
- `##ai`→`[]`；**`#a#b`→`[a b]`（Pitfall A）**。
- 剥离：`` `#code` ``→`[]`；`[链接](https://x#frag)`→`[]`；`https://x.com#ai`→`[]`。
- 转义：**`他说"#AI很重要"`→`[]`（Pitfall B）**。
- 截断：`#`+35 个 a→截 30；`# tag`→`[]`；`#😀`→`[]`；`\n#AI`→`[ai]`。
- `NormalizeTag`：`" AI "`→`ai`，`""`→`""`。

**后端 service_test 扩展**：`TestCreatePostStoresTags`（`#AI #RAG` → view.Tags `[ai rag]` + fake 落库）；无标签 Tags=nil；`f.errReplaceTags` 置位 → CreatePost 仍返回 view（log-and-continue）；`TestListFeedByTag`（置顶+最新序）。

**后端 handler content_test（testcontainers）** `TestTagsFlow`：注册→发 `#RAG #Agent` 帖→201 带 `tags:["rag","agent"]`；`?tag=rag` 命中；`?tag=AI` 大小写不敏感命中；无标签帖 `tags` 缺省；`?tag=nonexistent`→`items:[]`；删帖后不再命中。**postView 测试结构体补 `Tags` 字段**。

**前端 md.test.ts 扩展**：`extractTags("今天 #AI #RAG")`→`["ai","rag"]`；`C#和#ai`→`["ai"]`；`` `#code` ``→`[]`；`#a#b`→`["a","b"]`；`md("#AI")` 含 `<a class="tag" href="#/tag/ai">#AI</a>`；`md("C#")` 无锚点；`md("#<script>")` 转义无锚点（XSS）；`#a#b` 双锚点。

**前端 views/TagDetail.test.ts**（仿 PostDetail.test.ts vi.mock vue-router/vant/auth/api）：fetcher 传 `{tag:'ai',page,pageSize}`；头部 `# ai`；空名守卫；空态标题。

**可选 components/PostCard.test.ts**：胶囊点击 → `router.push('/tag/ai')` 且不触发卡片 `go`。

## 边界情况（实现时对照）

- `#a#b` 相邻标签 → **两个都识别**（替换推进所致，Pitfall A 钉住）；`##ai` 不识别；单 `#` 语法无闭合 `#`。
- 大小写：存储/查询均归一化小写 → `#AI`=`#ai` 同一聚合页；正文渲染保留原文大小写，锚点 href/胶囊/页头显示小写。
- URL 片段 `#frag` 被剥离吞掉，不识别。
- 软删帖：主查询 scope 自动排除；DeletePost 硬清 post_tags（无恢复功能，可接受）。
- 空 tag：NormalizeTag 后为空则不设过滤；TagDetail 空名守卫返回空列表。
- 错误路径：标签写失败不阻断发帖；`?tag=garbage` 归一化后非法 → 不过滤。
- 评论正文经 md.ts 同样高亮（CommentTree 免费生效，不另改）。
- `**#ai**`：标签规则先于粗体 → 渲染为 `<strong><a>#ai</a></strong>`（占位符机制天然正确）。

## 验证（SOP §7/§9）

```
cd "C:\Users\liyongquan\ai-forum\backend"
go build ./... && go vet ./... && go test ./...   # handler 集成需 Docker testcontainers
cd "C:\Users\liyongquan\ai-forum\frontend"
npm run test:unit && npm run build
cd "C:\Users\liyongquan\ai-forum"
git checkout -- frontend/components.d.ts            # build 会改写，提交前还原
git status --porcelain                              # 只 git add 自己的文件
```

迁移自动验证：`testutil.SetupPG` 对每个测试容器跑 `migrations.Run`，0007 自动应用。

本地浏览器冒烟（`webapp-testing`/Playwright，390×844，不提交；本地 dev + 后端 8080 + 造数）：发含 `#中文`/`#AI`/`#a#b` 的帖子 → 详情正文高亮可点 → 点标签进 `/tag/ai` → 聚合页出帖子 → 首页 PostCard 显示胶囊 → 评论里 `#词` 也高亮 → 截屏存 `.smoketest/54/`。

## 协作流水线

worktree 隔离（`git worktree add ../ai-forum-54 feat/tag-system`）→ 分支 `feat/tag-system` → 实现（后端 B1-B8 → 前端 F1-F7）→ 本地测试全绿 → commit → push → `gh pr create`（`MSYS_NO_PATHCONV=1`，合并前 `/code-review`）→ **用户授权** → `gh pr merge --merge` → CI 部署 → 服务器验证（SSH 2222：HEAD / compose ps / healthz / 发带 # 帖测聚合）→ 地图 #1 回报统筹方。
