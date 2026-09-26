# issue #78 全文搜索（帖子关键词检索）— 设计文档

- **Ticket**: [#78](https://github.com/li-yongqvan/ai-forum/issues/78) · 标签 `wayfinder:grilling`
- **类型**: 设计票（本文件不含实现代码）
- **日期**: 2026-09-24
- **决策记录**: `docs/handoffs/grilling-decisions/issue-78-fulltext-search-decisions.md`（D1–D9 + X1–X11 全文，本文件 §3 为索引）
- **评审状态**: **未走独立评审**（用户拍板 X10，理由与对本仓库惯例的偏离声明见决策记录 X10）；残余风险自查见 §9；独立视角留实现票的 `/code-review` 门（SOP §8.3）
- **版本状态**: 本文件当前**仅存在于本地工作区磁盘，未 commit**（X11）。依据：`deploy.yml:13-15` 对 `push: branches:[main]` **无 `paths-ignore`** ⇒ 纯文档 commit 上 main 会触发一次完整生产发版。本文档将作为实现票的**第一个 commit** 随实现 PR 入库。

---

## 0. 项目上下文

AI 智联论坛 = 社团内部移动端论坛 MVP，Go + Vue PWA，**线上已跑**（`http://122.51.233.225:8888`）。状态权威 = 地图 issue #1（41 条决策日志）。协作仓库 `C:\Users\liyongquan\ai-forum`，`main` = `b9d64d5`。本票属地图**决策 41** 新开的「第二里程碑 · 社区功能完善」第一批；**新服务器迁移已搁置**，方案不得为换机器做任何让步。

方法论约束（SOP）：深模块词汇（`docs/design-principles.md` §1，禁 component/service/API/boundary 指代设计单元）、包边界=服务候选边界（§5）、FK 政策（§7）、域包五件套与"新增功能装订模式"（SOP §5）、前端装订模式（SOP §6）、部署铁律（决策 35/37：判落地看 **api 镜像 ID + `schema_migrations` + 冒烟**）。

---

## 1. 背景与目标

**用户视角**：在移动端输入搜索词，找到标题或正文含该词的**帖子**；结果按"标题命中排前 + 时间倒序"排列、可分页、显示命中条数；点入直达帖子详情。

**为什么现在做**：地图「功能全景」❌未实现清单第一项；用户 2026-09-23 拍板"先完善社区功能，先做全文搜索"。

**与既有裁决的冲突**：IA v2（#9 定稿）曾**明确取消搜索占位框**（`docs/ux/info-architecture.md:5`/`:58`/`:199`/`:228`），且文档里**未记录取消理由**（只登记"经用户拍板采纳"）。本票是对该裁决的修订，落地时须回写 IA（§10）。

---

## 2. 真值核对（服务器 / DB / 代码 / GitHub）

### 2.1 线上生产 PG：**未实测**（如实登记）

本会话经该机连 `122.51.233.225` 端口 22 与 2222，均在 **pre-auth 阶段**被拒：

```
kex_exchange_identification: Connection closed by remote host
Connection closed by 122.51.233.225 port 2222
```

- TCP 端口 `OPEN`、HTTP 侧 `:8888/` 与 `:8888/healthz` 均 **200** ⇒ 服务器存活，仅 SSH ingress 拒我们；
- 本机出口 IP = `207.246.66.205`（不在服务器 SSH 白名单）；两套 ssh 客户端（Git Bash OpenSSH 10.2p1 / Windows 原生）表现一致 ⇒ 排除客户端。
- **未读、未打印 `.env`**（handoff 红线）。

**影响评估：无。** 因 §3 的 D7 定为零迁移、零扩展、零索引，运行期不需要任何新 DB 权限，票面担心的 `CREATE EXTENSION` 权限问题成为伪问题。唯一残留不确定性 = 线上真实数据量是否仍约 26 帖；`ILIKE` 全扫在数万行内仍可接受，且本票方案不随规模改变。

### 2.2 替代复验路径：同镜像一次性容器（实测）

testcontainers 与生产用**同一镜像**（`backend/internal/testutil/db.go:47` vs `compose.yml:63-64` 均 `postgres:16-alpine`），故本机起一次性容器复验技术事实具备代表性。容器已 `docker rm -f`，未创建任何持久资源。

| # | 实测项 | SQL / 命令（要点） | 结果 |
|---|---|---|---|
| 1 | PG 小版本 | `SHOW server_version;` | **16.15** |
| 2 | 扩展可用性 | `SELECT name,default_version,installed_version FROM pg_available_extensions WHERE name IN (...)` | 存在：`pg_trgm 1.6`、`btree_gin 1.3`、`unaccent 1.1`（均 `installed=no`）。**不存在：`zhparser`、`pg_jieba`、`rum`、`pg_bigm`** |
| 3 | 内置中文分词配置 | `SELECT cfgname FROM pg_ts_config;` | 30 种语言配置，**无任何中文配置**，只有 `simple` |
| 4 | `simple` 对中文的切词 | `SELECT to_tsvector('simple','AI 智联论坛 全文搜索 测试');` | `'ai':1 '智联论坛':2 '全文搜索':3 '测试':4` —— **整串中文按空白切成单一 token** |
| 5 | 子串召回（决定性） | `... @@ to_tsquery('simple','论坛')` | **`f`（不命中）**；而 `to_tsquery('simple','智联论坛')` = `t` ⇒ 内置 FTS 路线在中文上召回为 0，**不是质量差** |
| 6 | `ILIKE` 中文子串 | `SELECT 'AI 智联论坛 全文搜索' ILIKE '%论坛%';` | **`t`** |
| 7 | 非超主能否建扩展 | `CREATE ROLE forum LOGIN;` → `CREATE DATABASE forum OWNER forum;` → 以 `forum` 执行 `CREATE EXTENSION pg_trgm;` | **成功**，`pg_extension.extversion = 1.6` ⇒ 票面"trusted extension 大概率可用"复验为真（但本票不用它） |
| 8 | 中文三元组 | `SELECT show_trgm('论坛');` | `{0x8b5e40,0x9b9616,0xfd5869}` —— trgm 按**字符**切，扩展本身对中文可工作 |
| 9 | trgm 模糊在中文上 | `similarity('全文搜索引擎','全文搜索')` / `word_similarity('论坛','AI 智联论坛 第3期')` | **0.500 / 0.333** ⇒ 阈值一放即引入不相干内容（噪声） |
| 10 | **trigram 索引是否被使用** | 300 行表 + 两个 `gin_trgm_ops` 索引 + `ANALYZE`，`SET enable_seqscan=off` 后 `EXPLAIN (COSTS OFF) SELECT id FROM p WHERE title ILIKE '%论坛%' AND deleted_at IS NULL ORDER BY id DESC LIMIT 20;` | **不走 trigram 索引**：`Limit → Index Scan Backward using p_pkey → Filter (title ~~* '%论坛%')` ⇒ 在该查询形态下索引**根本不被规划器选中** |

**对第 10 条的诚实限制**：合成数据 300 行且 100% 含"论坛"，命中率偏高天然偏向扫 PK；极低命中率场景规划器可能改选 GIN。但线上仅约 26 帖，且第 9 条另有一票否决 trgm 模糊的理由，故结论方向不变。

### 2.3 代码真值（本会话逐条读到 file:line）

- **没有任何检索代码**：`backend/` 全量 grep `LIKE|ILIKE|to_tsvector|pg_trgm|websearch_to|tsquery|FULLTEXT` → **零命中**（唯一命中是 `internal/database/database.go:3` 注释里的"不设 search_path"）。搜索是绿地。
- **列表接口形态**：`handler/content.go` 的 `ListPosts` 组装 `content.ListFeedQuery{Tab,ViewerID,Page,PageSize,AuthorID,BoardID,TopicID,Tag}`，响应 `c.JSON(200, gin.H{"items":views,"page":page,"page_size":pageSize})` —— **全仓库无 `total` 字段**（grep `total` 于 `content/model.go`+`service.go`+`handler/content.go` 零命中）。
- **`PostQuery`**（`content/repo.go:5-17`）：`Feed "all|hot|follow"` + `AuthorID/BoardID/TopicID *int64` + `Tag *string` + `FollowedIDs` + `Offset/Limit`。分页是 **offset/limit**，无游标。
- **`ListPosts` 实现**（`gorm_repo.go:87-133`）：逐条 `Where`（`:89-101`，tag 是子查询非 JOIN `:98-101`）；排序 `is_pinned DESC`（`:116`）→ 热度表达式（`:119`）→ `created_at DESC`（`:121`）→ `Limit/Offset`（`:122-127`）；软删排除靠 **GORM 默认 scope**（`:99` 注释）。
- **#61 热度**（`gorm_repo.go:107-120`）：**7 天窗口**（`:112`）+ 两个预聚合 LEFT JOIN（`:113-114`）+ `(赞×1 + 评论×3) DESC`（`:119`）；fake 里镜像常量 `commentWeight = 3`（`fake_repo_test.go:214-232`）。仅 `Feed=="hot"` 时用。
- **模型/列**：`Post{ID,BoardID,TopicID*,AuthorID,Title,Content,IsPinned,IsFeatured,ViewCount,CreatedAt,UpdatedAt,DeletedAt}`（`model.go:37-52`，表 `content.posts`）；`Comment{...,ParentID*,Floor*,Content,...,DeletedAt}`（`:55-67`）。**两表都只有 `deleted_at` 软删，无 `status` 列**；帖子正文列名是 **`content`** 不是 `body`（`migrations/0002_content_schema.sql:31-32,51`）。
- **`PostView` 携带全量 markdown 正文**（`content/service.go:23-44`；组装 `postViews` `:880-991`，含作者名 join `:922-927`、作者查不到降级"已注销" `:977-979`）；服务端不截断不剥离，前端自己 `plainText()` + CSS 两行截断（`PostCard.vue:47`、`:133-142`）。
- **`Service` 接口现有 24 个方法**（`content/service.go:246-279`）。
- **登录墙**：`router.go:134-141` 的 `reads` 组（`OptionalAuth`）已含 `/posts`、`/posts/:id`、`/posts/:id/comments`、`/users/:id`；游客 `viewerID=0`（`handler/content.go:45-52`）；只有 `tab=follow` 要求登录（`content/service.go:639-641`）。
- **迁移执行器语义**（handoff 断言，本会话逐行确认）：`migrate.go:43` `tx, err := sqlDB.Begin()` → `:47` `applyStatements` → `:51` 插 `schema_migrations` → `:55` `Commit()`；`:64-65` `strings.Split(body,";")` 朴素切句，`:62-63` 注释自称"迁移文件为纯 DDL ⇒ 安全"。⇒ **迁移内不能用 `CREATE INDEX CONCURRENTLY`，不能写 `DO $$…;…$$`**。既有 13 个迁移文件**全部纯 DDL，零 `CREATE EXTENSION` / 零函数 / 零 CONCURRENTLY 先例**。`schema_migrations.version` 存**文件名**（非整数），最大号 = **`0012_notify_type_mention.sql`**，且存在 `0007` 撞号前例（`0007_reports_target_author.sql` + `0007_tags_schema.sql`，地图决策 29）。`main.go` 启动即跑迁移，失败 `os.Exit(1)`。
- **可复用先例**：`content.ParseTags`（`tags.go:47-61`）/`ParseMentions`（`mentions.go:22-36`）；markdown 剥离器 `stripMarkdownForMatching`（`tags.go:34-43`）**是包内私有**，且 `:8-9` 明文纪律"Go/TS 两套正则改一侧须改另一侧，两侧以同组单测钉行为"；前端 `md.ts:120-129` 的 `plainText()` 语义**不同**（保留链接文字、代码块替换成 `[代码]`）。
- **前端**：路由 15 条（`router/index.ts:7-23`，`createWebHashHistory`），meta 只有 `{title, requireAuth?}`，**无 `scrollBehavior`、无 keep-alive、无查询态保持**；`PostList.vue:15-18` 契约 `fetcher(page,pageSize)→{items}`，`:3` 注释"fetcher 须稳定引用，参数变化由调用方 `:key` 重挂载"（Feed 用 `:key="tab"`）；无限滚动 = window scroll + 300px（`:50-58`）；`SegTabs.vue` `options/modelValue`；`Empty.vue` `icon?/title/desc?`；`App.vue:37-44` 全局 nav-bar **全站无一处使用 `#right` 槽**；`PostDetail.vue:289-307` 已有 `revealComment`（`scrollIntoView` + 1.6s 闪动）；无 debounce 工具（`Write.vue:27-40` 是内联 `setTimeout`）；`workbox` 只 precache 静态资源，无 `navigateFallback`。
- **GitHub**：open PR = 0；`main` = `origin/main` = `b9d64d5`；工作区唯一脏文件 = `frontend/components.d.ts`（SOP §4.5 构建产物）；worktree 仅主工作区（`ai-forum-74`/`-76` 均已清）。

---

## 3. Grilling 决策记录（索引，全文见决策记录）

| 编号 | 定案 |
|---|---|
| **D1** | 中文检索 = **纯 `ILIKE` 子串**，多词按空白切分后 **AND**；不用 `to_tsvector`（中文召回为 0，实测）、不用 `pg_trgm`（索引不被使用，实测）、不引外部依赖/分词 |
| **D2** | **仅帖子**（标题+正文）。评论搜索移出本票（用户裁决），建议挂账另票 |
| **D3** | 排序 = **标题命中优先 → `created_at DESC`**；`q` 非空时跳过 `is_pinned DESC`；不复用 #61 热度（7 天窗口会致老帖全 0 分退化）、不给分段切换 |
| **D4** | **游客可搜**（挂 `reads`/`OptionalAuth`，与全站读侧一致）+ 后端最小 2 字符校验；**不改 `web.conf` 加限流** |
| **D5** | 治理与全站**完全一致**：软删不召回（同一 GORM scope）、无占位；封号/注销作者内容照常召回显示"已注销"；不为搜索新增治理过滤 |
| **D6** | 接口 = **复用 `GET /api/v1/posts?q=`**（给 `PostQuery`/`ListFeedQuery` 加字段），不新建 `/search` 端点、不加 `type` 参数；照抄 #54 标签链路先例 |
| **D7** | **零迁移**（不建扩展、不建索引、不建表）。`schema_migrations` 上线后应仍停在 `0012_notify_type_mention.sql` |
| **D8** | 入口 = 首页 `SegTabs` 上方**入口壳** → 跳 **L2 页 `/search?q=`**；输入只发生在结果页；`q` 进 URL 即满足"返回保留查询词"；滚动位置恢复不做 |
| **D9** | 存量数据 = 无回填、无索引窗口期（D7 推论） |
| **X1** | `\` `%` `_` 转义后**按字面匹配**；`trim` 后 **≥2、≤64 字符**；**词数 ≤4**（超出拒绝，见 §6-3） |
| **X2** | 大小写**不区分**（`ILIKE` 语义），前端高亮两侧 `toLowerCase()` 对齐；英文大小写用例必进集成测试 |
| **X3** | 繁简**不做**（PG 无内置、镜像只有 `unaccent` 且不管汉字），登记未来触发条件 |
| **X4** | 高亮 = 前端纯函数 `utils/highlight.ts` 返回分段数组 + `v-for`/`<mark>`，**禁 `v-html`**（Vue 自动转义文本节点 = 零新增 XSS 面） |
| **X5** | 高亮载体 = 给 `PostCard.vue` 加**可选** `terms?: string[]`，**不新建搜索专用卡**（删除测试：两份卡片会悄悄长得不一样） |
| **X6** | 命中片段 = **前端**从 `PostView` 全量正文算（±40 字 + `…`），仅标题命中时回落现有两行截断；后端不加字段 |
| **X7** | 总条数 = **仅 `q` 非空时** `COUNT(*)`，响应新增可选 `total`；Feed/标签页零影响；`total` 不参与翻页 |
| **X8** | 详情页正文内定位 = **不做** |
| **X9** | 术语 = 「**搜索词（query）**」，本票**不动** `docs/ubiquitous-language.md`，词条登记于 §10 |
| **X10** | 交付 = 决策记录 + 本设计文档，**跳过独立评审**（对本仓库"文档+意见书成对"惯例的显式偏离） |
| **X11** | 票面 issue #78 由设计会话回写；**地图 issue #1 不动**；文档暂不 commit（避免纯文档触发生产发版） |

---

## 4. 范围收敛与明确不做

**in**：帖子标题+正文的关键词检索（多词 AND、不区分大小写、按字面匹配）；`/posts?q=` 接口扩展 + 搜索态 `total`；排序（标题命中优先）；游客可搜；软删口径复用；命中词高亮；命中上下文片段；结果页条数与空态；首页入口壳；`/search` L2 路由；IA 文档回写；`content/README.md` 补 `q` 语义。

**out（逐条给理由，防"顺手加"）**：

| 不做 | 理由 |
|---|---|
| 评论搜索 | 用户 D2 裁决砍掉；票面「目标」相应条目已回写 |
| 用户搜索/"找人" | 票面原 out；Roadmap 挂账 |
| 搜索建议、自动纠错、拼音、历史词、热词 | 票面原 out；无数据支撑其价值 |
| 繁简转换 | X3；须引映射表/第三方依赖，与 D1 方向相反 |
| 语义/向量检索、ES、`to_tsvector`、`pg_trgm`、任何扩展或索引 | D1/D7；§2.2 第 5、10 条实测否决 |
| 详情页正文内按词定位/滚动 | X8；`revealComment` 是为整条节点设计的，词内定位是另一件事 |
| 命中片段所在的二次"跳转定位" | 同上 |
| 结果总条数以外的统计（如各板块分布） | 无用户诉求 |
| 搜索端点独立限流（改 `web.conf`） | D4；属运维票，见 §9-1 |
| 存量冒烟数据清理（`[mention-smoke]` 等） | 地图挂账"清理另议"，与本票无关 |
| 换服务器/新机器适配 | 用户 2026-09-23 拍板迁移搁置 |
| HTTPS / 域名 / PWA 离线缓存 | 决策 8 已知技术债 |
| 软删帖占位（"该帖已删除"） | D5；属既有 IA 议题，若做应全站统一另票，不为搜索单开 |
| 封禁作者内容全站下沉 | D5；治理票 |
| 滚动位置恢复 / keep-alive | §2.3：项目无此机制，本票不引入 |

---

## 5. 实现方案（每项给依据）

> 交给实现票的落地清单。**零迁移、零 `router.go` 改动、零 `main.go` 改动**。

### 5.1 后端（SOP §5 装订模式，6 处）

1. **`backend/content/service.go`** — `ListFeedQuery` 加 **`Query string`**（原样字符串，未切词未转义）。
   - 依据：与既有 `Tag` 字段同层同位（`handler/content.go` 里 `if v := c.Query("tag")` → `q.Tag = &v`，注释明写"归一化/空值处理由 **service seam 层**兜底（评审 §5.1-7，避免双处策略）"）→ 搜索词的校验/切词/转义**同样必须放 service 层**，handler 只做透传（§6-1）。
2. **`backend/content/service.go`** — 新增导出函数 `NormalizeQuery(raw string) (terms []string, err error)`：`TrimSpace` → 长度 2..64 校验 → 按空白切分 → 词数校验 → 逐词转义（§6-2 顺序）→ 返回**可直接进 `ILIKE` pattern 的词**（不含 `%`，由 repo 层包）。`ListFeed` 内 `if q.Query != ""` 时调用，`err` 走既有哨兵错误风格（新增 `ErrInvalidQuery` 之类，命名对齐 `content` 包现有 `Err*` 变量）。
   - 依据：`NormalizeTag` 先例（`service.go:626-631` service 层兜底归一化）。
3. **`backend/content/repo.go`** — `PostQuery` 加 **`Terms []string`**（已归一化的词）。
4. **`backend/content/gorm_repo.go`** — `ListPosts` 内：
   - `for _, t := range q.Terms { db = db.Where("(title ILIKE ? OR content ILIKE ?)", "%"+t+"%", "%"+t+"%") }` —— **逐词一个 AND**，即 X1 的 AND 语义；参数化传值，不拼 SQL 字符串（防注入）。
   - 排序分支：`len(q.Terms) > 0` 时 **不加** `is_pinned DESC`，改为 `Order("(title ILIKE ?) DESC", "%"+terms[0]+"%")` 前置，再 `created_at DESC`；`len(q.Terms) == 0` 时排序构造**逐字节不变**（§7-2）。
   - 依据：`gorm_repo.go:116-121` 现有排序即 `Order()` 链式追加，分支点最小；软删排除不需要写（GORM 默认 scope，`:99`）。
5. **`backend/content/service.go`** — 新增 **`CountFeed(ctx, q ListFeedQuery) (int64, error)`**（与 `ListFeed` 复用**同一个** `PostQuery` 构造器）。签名/返回类型见 §6-4（为何不改进 `ListFeed` 的返回结构）。
6. **`backend/internal/httpapi/handler/content.go`** — `ListPosts` 加 `if v := c.Query("q"); v != "" { q.Query = v }`（与 `tag` 分支同构，**不做任何校验**）；`ListFeed` 成功后，仅当 `q.Query != ""` 时调 `CountFeed` 并在响应 map 里加 `"total"`。错误经 `respondContentError` 映射为 4xx（§6-3）。
7. **`backend/content/README.md`** — Interface README 补一段 `q` 的**行为契约**（不写签名，design-principles §3 规定 README 写不变量/错误模式/性能）：多词 AND、不区分大小写、`%`/`_` 按字面、2..64 字符、≤4 词、不做相关性打分、不做繁简、`total` 仅搜索态出现、性能特征（顺序扫描，随 `posts` 行数线性）。
8. **迁移/路由/装配**：**全部零改动**（D7；`/posts` 已在 `reads` 组 `router.go:134-141`）。
9. **测试**：`fake_repo_test.go` 加 `Terms` 过滤 + `CountFeed`；`content/service_test.go` 覆盖归一化（长度/词数/转义/大小写）；`handler/content_test.go`（testcontainers 真 PG）覆盖 §8-1 中文用例组。
   - ⚠ **fake 镜像风险**：`fake_repo_test.go:214-232` 已有"把真库语义用 Go 常量镜像一份"的先例（`commentWeight=3`）。fake 里的 `Terms` 匹配**只能是朴素 `strings.Contains`**，它与 `ILIKE` + 转义**不等价**（大小写、多字节）。因此**检索语义必须由集成测试钉死，service 单测只测"归一化 + 透传 + 分页"**（SOP §7"测试只跨 seam"）。

### 5.2 前端（SOP §6 装订模式，6 处）

1. **`frontend/src/utils/highlight.ts`**（新）— `splitByTerms(text: string, terms: string[]): {text: string, hit: boolean}[]`。两侧 `toLowerCase()` 定位（X2/X4）；**不产生 HTML 字符串**；纯函数、无 Vue 依赖、可直接单测。
2. **`frontend/src/components/PostCard.vue`** — 加可选 `terms?: string[]`（X5）。标题与摘要文本改用 `v-for` 渲染 `splitByTerms` 的分段，`hit` 段包 `<mark>`；`terms` 未传时**渲染结果与今天逐字节一致**（§7-5）。摘要另加可选 `snippetFrom?: string`（X6 的命中窗口文本），未传时走现有 `plainText()` 两行截断。
3. **`frontend/src/api/types.ts` + `api/content.ts`** — `PostList` 类型加 `total?: number`；`listPosts(params)` 支持 `q`（`URLSearchParams` 拼接，与现有 `tag` 同处）。
4. **`frontend/src/views/Search.vue`**（新，L2）— 顶部输入框（`van-search`，`v-model` 初值取 `route.query.q`）→ 回车/点击检索时 `router.replace({path:'/search', query:{q}})`；`<PostList :key="q" :fetcher="fetcher" />`，`fetcher` 内调 `api.listPosts({q, page, page_size})` 并把 `terms` 透传给卡片；`total` 显示"找到 N 条结果"；`items` 为空且 `total===0` → `<Empty :title="'没有找到「'+q+'」的相关帖子'" />`。
   - 依据：`PostList.vue:3` 契约要求"参数变化由调用方 `:key` 重挂载"，与 `Feed.vue:34` 的 `:key="tab"` 同一手法；标签页 `TagDetail.vue:9-13` 是"专用页复用 `listPosts`"的现成模板。
5. **`frontend/src/router/index.ts`** — 加 `{ path: '/search', ... meta: { title: '搜索' } }`；hash 路由，`q` 走 query（"返回保留查询词"即靠此，§2.3 已确认项目无其他查询态机制）。**不加 `requireAuth`**（D4）。
   - ⚠ **实现时须核对**：`App.vue:21-25` 的 L1/L2 判定是**白名单还是黑名单**——若白名单，新增 L2 路由必须显式登记才能隐藏 Tab、正确控制 FAB（`info-architecture.md:136` §5.1 规则）。此点本会话未读到定论，不假装已知。
6. **`frontend/src/views/Feed.vue`** — `SegTabs` **上方**加入口壳（静态样式，不可输入），点击 `router.push('/search')`（D8）。
7. **测试**：`utils/highlight.test.ts`（纯函数全分支：多词、重叠、大小写、词在首尾、无命中）；`views/Search.test.ts`（mock 模式照 `views/PostDetail.test.ts`：`vi.mock` vue-router / auth store / api）；`PostCard` 补两条 prop 分支用例。

---

## 6. 关键设计裁决（写文档时才浮出的四处，均给备选与不选理由）

### 6-1 搜索词的归一化放 service 层，不放 handler

- **问题**: `trim`/长度/切词/转义这套规则写在哪一层？
- **定案**:【**service 层**】`NormalizeQuery` 导出 + `ListFeed` 内调用；handler 只 `c.Query("q")` 透传。
- **理由**: `handler/content.go` 的 `tag` 分支已有一条明文注释——"归一化/空值处理由 service seam 层兜底（评审 §5.1-7，**避免双处策略**）"。同一层同一先例。
- **备选（不选）**: handler 里校验 → 与标签同事不同命，且未来若加"评论搜索"要复制一遍。

### 6-2 `ILIKE` 转义顺序：先 `\`，再 `%` 和 `_`

- **问题**: 三个字符都特殊，转义顺序错会二次转义。
- **定案**: `strings.ReplaceAll` 依次 `\`→`\\`、`%`→`\%`、`_`→`\_`；`pattern = "%"+t+"%"` 由 **repo 层**包装，`NormalizeQuery` 只交纯词。
- **理由**: 先转义反斜杠是唯一正确顺序（否则 `%`→`\%` 产生的 `\` 会被再转一次）。pattern 的 `%` 属"检索形态"而非"用户输入"，放 repo 层避免归一化函数被两侧 `%` 语义污染。
- **备选（不选）**: 用 `LIKE … ESCAPE '!'` 自定义转义符 → 能绕开反斜杠与 `standard_conforming_strings` 的纠缠，但 SQL 更啰嗦且要改 GORM 原生 SQL 写法；实现时若真撞上反斜杠行为异常，再切这条，并回填本节。

### 6-3 词数 >4 = 拒绝，不静默截断

- **问题**: 决策记录 X1 写的是"最多取前 4 词"，落地时到底静默丢弃还是报错？
- **定案**:【**拒绝**，返回 4xx 参数错误】长度 `<2`、`>64`、切词后 `>4` 三种一律拒绝，前端提示"最多 4 个搜索词"。
- **理由**: 静默丢词会产生"我明明打了 5 个词却搜不到"的不可解释结果；社团论坛输入 5 个词本就极罕见，报错的成本几乎不会命中真实用户。
- **备选（不选）**: 取前 4 + 结果页提示"仅按前 4 词检索" → 多一个提示态与一条文案，收益低于"直接拒绝"。
- **⚠ 这条与决策记录 X1 的表述有出入，已在决策记录与本文档同时标注，用户若有异议只需改本节。**

### 6-4 `total` 用新增 `CountFeed` 方法，不改 `ListFeed` 返回结构

- **问题**: 前端要条数，后端怎么给？
- **定案**:【新增 `CountFeed(ctx, q)`，handler 仅在 `q.Query != ""` 时调】
- **理由**: 把 `ListFeed` 改成返回 `PostPage{Items,Total}` 会波及**全部现有调用方**（Feed / 标签页 / 作者页 / 板块页 / 关注页…）+ `fake_repo_test.go` + 所有 service 单测，为一个只有搜索需要的字段做全站重构，违背最小 diff 与 SOP §1"小步提交"。
- **代价（诚实）**: `Service` 接口从 24 方法变 25 方法 → 接口面变大。这是 X7 定案的真实成本，不是零。
- **备选（不选）**: ① 改 `ListFeed` 返回结构（ripple 面过大）；② 搜索走独立端点 `/search`（与 D6 冲突，且那是更贵的 seam）；③ 不显示条数（用户已在 X 轮明确否决）。

### 6-5 高亮用分段数组渲染，不用 `v-html`

- **问题**: 见 X4。
- **定案**:【纯函数 + `v-for`/`<mark>`】**禁用 `v-html`**。
- **理由**: Vue 对文本节点自动转义，等于零新增 XSS 面；`v-html` 会把"这个字段是 HTML"变成永久约定，后端读模型或前端拼装都要手写转义（`md.ts:7-12` 已存在 `escapeHTML` 说明这条线踩过坑）。
- **不变量**: 前端定位词必须与后端 `ILIKE` 语义对齐（两侧 `toLowerCase()`），否则出现"后端命中、前端不高亮"（§7-6）。

---

## 7. 边界与不变量清单（实现与评审逐条核对）

1. **软删口径同源**：搜索态与非搜索态必须走同一个 `ListPosts`/同一个 GORM scope。不得为搜索写第二条查询路径。
2. **非搜索态行为逐字节不变**：`q` 缺失或 trim 后为空 ⇒ `Terms` 为空 ⇒ WHERE 不加、排序分支不进、`total` 不出现。此条必须有回归测试。
3. **`total` 与 `items` 必须同条件**：`CountFeed` 与 `ListFeed` 复用同一个 `PostQuery` 构造器；两者条件不一致 = 缺陷。
4. **参数化查询**：词值只经 GORM 占位符进 SQL，**禁止字符串拼接 SQL**（`%`/`_` 转义防的是语义误伤，不是注入；注入防线始终是参数化）。
5. **置顶仅非搜索态优先**：搜索态 `is_pinned` 不参与排序（D3）。若评审认为要保留置顶优先，只需删掉排序分支，不影响其他不变量。
6. **高亮与召回一致性**：`splitByTerms` 与后端 `ILIKE` 同样不区分 ASCII 大小写；不一致时**以召回为准**（多高亮或少高亮都不是错误，但不得因前端找不到词而丢弃该结果卡）。
7. **禁 `v-html`**（X4/6-5）。
8. **零迁移**：`backend/migrations/` 不得出现新文件。若实现会话认为需要索引，**必须停下来回报**，不得自行加迁移（D7 是用户裁决，且 §2.2-10 实测否决）。
9. **零路由/零装配改动**：`router.go`、`main.go` 不应出现在 diff 里。
10. **游客路径无 `Identity` 要求**（D4）。
11. **不改 `web.conf` / 不引 Redis / 不引缓存层**（D4/决策 4、8）。
12. **前端**：`components.d.ts` commit 前 `git checkout --`（SOP §4.5）；`/search` 不要求登录；`PostList` 的 `fetcher` 必须是稳定引用（`PostList.vue:3`），搜索词变化用 `:key` 重挂载而非重建 fetcher。
13. **不引入新依赖**（`go.mod`、`package.json` 应零变化）。
14. **术语**：文档与代码注释用"搜索词 / `q` / query"，**不得**称其为"关键词"（与标签 `#hashtag` 混淆，词表 `:63` 已 flagged）或"话题/标签"。

---

## 8. 测试与验证计划

### 8-1 后端集成测试（testcontainers 真 PG，`postgres:16-alpine`，与生产同镜像；本机 Docker 可用已确认）

必须有的**中文**用例（票面验收线）：

| 用例 | 断言 |
|---|---|
| 中文子串召回 | 正文含"智联论坛"，搜 `论坛` **命中**（钉死 D1 的核心：`to_tsvector` 路线在此为 `f`） |
| 标题命中优先 | 两条都命中，标题命中者排在前；且置顶帖（`is_pinned`）不插队 |
| 多词 AND | 搜 `论坛 搜索` → 两词都在才命中；只含其一不命中 |
| 跨词/跨句子串 | 搜 `文搜索`（非完整词）命中（钉死"按子串而非按词"语义） |
| 大小写不敏感 | 正文含 `AI`，搜 `ai` 命中（X2 强制项） |
| **通配符按字面** | 标题含 `100%`，搜 `%` **只命中含字面 `%` 的行**，且 `50%` 不作为前缀通配（X1） |
| 下划线按字面 | 搜 `a_b` 只命中字面 `a_b`，不命中 `axb` |
| 软删不召回 | `deleted_at` 非空的帖不出现在结果（D5） |
| 封号作者仍召回 | 作者 `status` 非 active 的帖**仍召回**、作者名降级"已注销"（D5 维持现状） |
| `total` 一致性 | `total == 全量匹配数`，且翻页不改变 `total`（§7-3） |
| 参数错误 | `<2`、`>64`、`>4 词` 三种返回 4xx 且错误体走既有 `respondContentError` 形状 |
| 游客可搜 | 不带 Bearer 请求 `/posts?q=` 返回 200（D4） |
| 非搜索态回归 | 无 `q` 时响应**不含** `total` 键、排序含 `is_pinned DESC`（§7-2） |

**门控**：`cd backend && go build ./... && go vet ./... && go test ./...` 全绿（SOP §7；handler 集成测试约 189s 属正常）。

### 8-2 前端

`cd frontend && npm run test:unit && npm run build` 全绿（`highlight.test.ts` 全分支 + `Search.test.ts` + `PostCard` 两 prop 分支）。

### 8-3 真机浏览器冒烟（票面验收线，不可用单测替代）

1. 游客（未登录）打开首页 → 点搜索入口 → 输入"论坛" → **结果 URL 带 `q`**、显示"找到 N 条结果"、卡片命中词有 `<mark>` 底色；
2. 翻到底（无限滚动）→ `total` 不变；
3. 点一条仅正文命中的帖 → 进详情 → **返回** → 查询词仍在输入框、结果仍在（验收线"返回保留查询词"）；**明确不要求滚动位置**；
4. 搜一个不存在的词 → 空态文案含所搜词；
5. 搜 `100%` → 结果为含字面 `100%` 的帖，非全部；
6. 登录态与非登录态搜索结果**条数一致**；
7. 首页 Feed 与标签页行为无变化（§7-2 回归）。

### 8-4 上线核对（铁律三件套，决策 35/37）

CI 全绿 **不等于**新代码在跑。必须逐条：服务器 `git rev-parse --short HEAD` = 合并 SHA → **api 镜像 ID 变化** → **`schema_migrations` 仍停在 `0012_notify_type_mention.sql`（零新增行 = D7 成立）** → `deploy/verify-deployed.sh` 通过 → `/healthz` 200 → 线上真实冒烟（同 8-3 的 1/3/5 三条）。

---

## 9. 残余风险与已知限制（代替评审意见书，逐条给"何时才需要处理"）

1. **搜索端点无独立限流**（D4 明确不改 `web.conf`）。触发条件：真出现刷接口。**处置路径**：另立运维票，注意 bind-mount 改后须 `nginx -t && nginx -s reload`（SOP §11）。
2. **`ILIKE` 是顺序扫描**，成本随 `posts` 行数线性。触发条件：帖子量到数万级且首屏可感知变慢。**处置路径**：那时再议 trigram 索引，但须先解 §2.2-10 那条"该查询形态下规划器不走 GIN"（可能要先接受"搜索态不按 `created_at` 排序"或改写为 `similarity` 语义，两者都改变召回）。
3. **变音符号不归一**（X2 已知限制）。触发条件：社团出现相关用例；解法是引归一化依赖 = 违背 D1。
4. **繁简不互认**（X3）。触发条件：出现繁体用户。
5. **`/posts` 语义变宽**：一个端点同时承担"Feed 列表"与"搜索结果"。缓解 = `content/README.md` 必须写清 `q` 契约（§5.1-7）；`Service` 接口 +1 方法（6-4 已认账）。
6. **未来加评论搜索时 `/posts` 会语义错位**（D6 已认账）。触发条件：评论搜索立票。
7. **前端高亮与后端召回的一致性只到 ASCII 大小写**（§7-6）。表现为极端字符下"命中但不高亮"，不影响正确性。
8. **线上 PG 未实测**（§2.1）。因零迁移，无运行期影响；地图收口时应补一条"下次能连服务器时顺手核对 `SELECT count(*) FROM content.posts`"。
9. **本票跳过了独立评审**（X10）。残余视角由实现票的 `/code-review` 门（SOP §8.3）补，届时 §7 清单与 §6 裁决是检查项。
10. **`App.vue` 的 L1/L2 判定方式未确认**（§5.2-5 ⚠）。实现时若为白名单而漏登记，表现为 `/search` 页底部 Tab 仍显示 = 违反 IA §5.1。

---

## 10. 收口事项（不在实现票内，登记以免丢失）

- **待并入 `docs/ubiquitous-language.md` 的词条**（X9：本票不改常驻词表，实现验收通过后并入）：
  - **搜索词（query）**：用户在搜索框输入、用于在帖子标题与正文中做子串匹配的字符串。别名要避免：**关键词、标签、话题、tag、hashtag**。
  - 与既有词条的显式区分：**标签**（正文内 `#hashtag`，组织内容）/ **话题**（发帖时所选，组织内容）/ **搜索词**（用户查询输入，不组织结构）——三者都常被中文叫成"关键词"，须在并表时写死。
- **待回写 `docs/ux/info-architecture.md`**：`:5`（定稿裁决"取消搜索占位框"）、`:58`（D1 页面清单"无搜索功能，亦不设占位框"）、`:199`（§7）、`:228`（§9 修订记录）四处改为实装条目，并把 `/search` 加进 §3 层级表与 §4 页面→实体表（L2、隐藏 Tab）。
- **待统筹方登记 issue #1**（本会话无权限、亦不越权改地图）：
  1. 本票决策落档 + 标签 `wayfinder:grilling` → `wayfinder:task`（需用户授权）；
  2. 新挂账候选：**评论搜索**（D2 移出）、**封禁内容全站下沉**（D5 另票）、**搜索端点限流**（§9-1 另票）。
- **本地分支/worktree**：本会话未建分支、未 commit、未 push、未动生产。`frontend/components.d.ts` 脏文件保留原状未动。

---

*本文件为 issue #78 的设计交付。所有事实断言均带 `file:line` 或 §2 实测输出；无出处处已显式标注为"未确认/待核对"。*
