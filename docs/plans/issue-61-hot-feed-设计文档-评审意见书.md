# #61 首页热门流 设计文档 评审意见书

> **评审对象**：《#61 首页热门流：内容发现新分段 — 设计文档（供评审）》（2026-08-25，`docs/plans/issue-61-hot-feed-设计文档.md`）
> **评审方式**：独立复核 —— 评审方以 worktree `ai-forum-61` @ `9cb5ec0`（main HEAD = origin/main，已核实）为真值源，逐项核对文档引用的代码行号、迁移、路由、测试基建、SOP，并核实 GitHub issue/PR 状态；对 Q1-Q5 五个开放点作出裁决。另吸收一份独立评审（2026-08-25）的发现，经对仓库独立复核确认后并入（列遮蔽 F8、热度博弈 F9、翻页漂移 Q3 注），并据其先例复核修正 F2 的建议方向。
> **评审结论**：**有条件通过**

---

## 一、总体结论

设计方向与既有架构（多 schema、跨包 seam、分页列表独立形态）高度一致，证据纪律扎实：§2.2 的代码引用行号基本全部精确，D1-D3 备选对比有理有据，SQL 现算热度分 + 排序后分页的技术论证（§6.1）是全文档质量最高的一条。决策清单 D1-D3 与设计裁决 6.1-6.4 全部认可或附条件认可。

但有一个**阻塞项**必须在动身前消化（F1）：文档在三个关键位置引用了一个**不存在的交接文件** `docs/handoffs/61-hot-feed.md`，实际承载来源是 `docs/plans/issue-61-hot-feed.md`（一份「计划」，非 handoff，节号为 §1-§7 而非 §0-§4），且两份 issue-61 文件均未提交（git untracked），grilling 决策（D1-D3）也未按 SOP §3.4 落档到 `grilling-decisions/`。这意味着「定案依据 / 红线 / PR#57 协调要求」当前只存在于未入库的本地文件——其他会话或 agent 无法接手复核，与文档自称的「全部可复现」不符。

另有**三个重要项**：置顶帖与 7 天窗口的相互作用（F2）**已由用户裁决为方案 A（窗口优先）**，需在文档与测试中钉死；「7 天窗口」的语义解释未显式写明（F3）；hot 分支 JOIN 未按仓库明规补 `Select("content.posts.*")` 防列遮蔽（F8）。

总体评价：事实基础可信、技术方案正确、可按其框架执行；补齐 F1、F3、F8 并落档 F2 裁决后即为可靠执行基线。

---

## 二、事实与证据复核

复核范围：文档 §0 架构约定、§2 真值核对全部条目、§3/§4 决策证据、§5 实现方案引用的代码行号、§7 不变量、§8 测试计划引用的测试基建。

### 核实为真（抽样 20+ 项，全部命中）

| 文档主张 | 复核结果 |
|---|---|
| 本地 main = origin/main = `9cb5ec0` | ✅ `git rev-parse HEAD` 与 `origin/main` 均 `9cb5ec0` |
| `Feed.vue:10` `ref<'all'\|'follow'>`；`:18-25` SegTabs 全部/关注；`:28` LoginGuide `tab==='follow' && !auth.isLoggedIn`；`:29` PostList `:key="tab"`；`:13` fetcher→`api.listPosts` | ✅ 逐行命中（Feed.vue:10/13/19-24/28/29） |
| `content.ts:26-35` `ListPostsParams`，`:28` `tab?: 'all'\|'follow'`；`listPosts` 无 requireAuth | ✅ 逐行命中（:27-35 / :28 / :37-47，GET 走 `request` 不带 requireAuth） |
| `service.go:112-121` `ListFeedQuery`（Tab :113 注释 `all|follow`）；`:504` `ListFeed`；`:524-550` switch（all :526 / follow 登录墙 :528-531 / default `ErrInvalidFeedTab` :548-549）；`:552` `repo.ListPosts`；`:556` `postViews` | ✅ 逐行命中 |
| `service.go:770/774/778` 现场 Count（赞/评论/收藏不落库） | ✅ `CountLikesByPost`/`CountCommentsByPost`/`CountFavoritesByPost` 均在 postViews 内逐页 Count |
| `repo.go:6-17` `PostQuery`（Feed :7 注释 `all|follow`）；`:30` `ListPosts` | ✅ 命中 |
| `gorm_repo.go:86-118` `ListPosts`：过滤 :88-105 → `Order("is_pinned DESC").Order("created_at DESC")` :106 → limit/offset :107-112；imports 无 `time` | ✅ 命中（imports = context/errors/sort/gorm/clause，**确实无 time**，计划补 import 成立） |
| `model.go` `Post`(:37-50)/`Comment`(:55-65)/`Like`(:69-77) | ✅ 命中；`Like` 无 `DeletedAt`（硬删，无需软删过滤），`Comment` 有 `DeletedAt` 软删 |
| handler `content.go:96-127` `ListPosts`；`:100` `Tab` 直通无白名单 | ✅ 命中（`Tab: c.DefaultQuery("tab","all")`） |
| `router.go:102` `reads.GET("/posts", ch.ListPosts)`；`:134` pin\|feature 在 mod 组 | ✅ 命中 |
| 迁移 0002 `:56` `idx_comments_post`；`:67` `idx_likes_target` | ✅ 逐字命中 |
| 热度聚合子查询可走现有索引 | ✅ 合理：likes `target_type='post'`+`target_id` 走 `idx_likes_target`；comments `post_id` 走 `idx_comments_post` |
| fake 已镜像「置顶+时间倒序」排序；follow 分支位置 | ✅ `fake_repo_test.go` `ListPosts` :153 起、follow :171、排序 :181-186；fake 有 `f.likes`/`f.comments` map（:313/:251），hot 分支可现算 hotScore |
| 评论计数口径与 `postViews` 一致（软删不计） | ✅ `CountCommentsByPost` 走 `Model(&Comment{})` → GORM 软删 scope 自动加 `deleted_at IS NULL`，与计划 hot 子查询显式过滤同口径（不变量 8 成立） |
| 前端组件 `PostList.vue`/`Empty.vue`/`SegTabs.vue` 均存在；`PostList` 有 `emptyTitle` prop（默认「还没有帖子」） | ✅ 均命中；`PostList.vue:17/:199`，按 tab 动态空态文案可行 |
| 'hot'/'热门' 零命中 | ✅ 按词边界 `grep -rE '\bhot\b|热门'` 零命中；子串语义下 `upload_test.go:57` 的 `photo.png` 是假命中（见 F7） |
| issue #61 OPEN + `wayfinder:task`；方向已定 | ✅ `gh issue view 61`：OPEN，label wayfinder:task，body 注明「方向已定 2026-08-25」 |
| 挂起 PR #57（术语表 docs） | ✅ PR #57 OPEN，仅新增 `docs/ubiquitous-language.md` |
| 并行 #59（notify，分支 feat/dm）、#60（moderation/ops） | ⚠️ 部分复核：issue #59/#60 OPEN、本地有 `feat/dm` 分支；「零文件重叠」未逐文件核验（不切换分支，见「不可复核」） |
| SOP §3.4 决策落档 `grilling-decisions/`；§4.5 `components.d.ts`；§5.3 分页列表独立形态；§12 #23 walkthrough | ✅ 逐条命中 `workflow.md`:63/:75/:93/:234 |
| 既有测试：`TestListFeed_All/_Follow/ByTag`；handler 集成 testcontainers 真 PG；`Me.test.ts` 存在 | ✅ `service_test.go`:296/:339/:110；`content_test.go` 多处 `testutil.SetupPG(t)`；`views/Me.test.ts` 存在 |
| 「4 个领域包每包结构固定：model.go/repo.go/gorm_repo.go/service.go/README.md」 | ✅ user/content/notify/moderation 四包均齐（`upload` 为文件存储无 schema，不属 DB 域，表述可接受） |
| `SegTabs.vue` options 形态 `{key,label}` | ✅ 命中（SegTabs.vue:2），插 `{key:'hot',label:'热门'}` 成立 |

### 不实 / 冲突

| 计划主张 | 复核结果 |
|---|---|
| **§1「交接依据：`docs/handoffs/61-hot-feed.md`（§0-§3，自包含交接）」** | ❌ **该文件不存在**。`docs/handoffs/` 实有 5 文件 + `grilling-decisions/`，无 61 条目，INDEX 亦未列。实际承载来源是 `docs/plans/issue-61-hot-feed.md`——它是**计划**而非 handoff，节号为 §1-§7，非文档所称 §0-§4 → **F1** |
| **§2.3「handoff §4 要求本票 PR 合并时搭 PR #57 一起合」** | ❌ 引用的 handoff §4 不存在。该要求实际在 `docs/plans/issue-61-hot-feed.md` §7（第 96 行）→ **F1** |
| **§4「notify/moderation 域、.github/workflows/ 不碰（handoff §4 红线）」** | ❌ 引用的「handoff §4 红线」无源。计划文档 §1 只有「与本票零文件重叠」的协同注，无 .github 红线表述 → **F1** |

### 不可复核

| 项 | 说明 |
|---|---|
| 服务器实查（§2.1：SSH 2222 输出、三容器 healthy、healthz） | ⚠️ 未复核（评审方无服务器通道）。本地 `main`=origin/main=`9cb5ec0` 与文档的线上基线主张相互印证，主张可信但未独立核到 |
| 2026-08-25 AskUserQuestion 用户 grilling 确认（D1-D3） | ⚠️ 本机无独立记录；仅存在于**未提交**的计划文档 §3 中（这正是 F1 的一部分） |
| #59/#60 分支文件清单、「零文件重叠」 | ⚠️ 未逐文件核验（worktree 隔离纪律，不切换分支）；按域隔离（content vs notify/moderation）判断合理 |

**评价**：代码/迁移/路由类引用行号命中率极高，SQL 现算论证、索引覆盖、软删口径、fake 镜像语义等关键事实全部属实。但文档自称「交接依据 / 红线可溯源」，其指向的文件却不存在且未入库——这是全文档证据链唯一的硬伤，恰好落在 D1-D3 定案与执行红线的最终依据上（F1）。

---

## 三、逐条评审

### 决策清单（D1-D3）

| 决策 | 结论 | 评审意见 |
|---|---|---|
| D1 热度公式：7 天窗口 + 赞×1 + 评论×3，降序，浏览量不参与 | **认可** | 备选（HN 式全时间+衰减、24h 快热）否因合理（幂/窗口语义与产品直觉）；与 grilling 记录一致。「7 天窗口」的**语义解释未显式写明**（窗口作用于帖子创建时间 vs 互动时间）——见 F3 |
| D2 入口形态：全部/热门/关注三分段，热门游客可见 | **认可** | 与 `router.go:102` reads 组（OptionalAuth）+ `ListFeed` hot 分支不加登录墙完全吻合，零路由改动成立 |
| D3 置顶恒顶；不做精华分段 | **认可方向，附条件** | 「置顶恒顶」与「7 天窗口剔除」两条不变量**未裁决相互作用**：窗口外置顶帖被整体剔除，「恒顶」在该帖上不成立，且与全部流（无窗口）行为不一致——见 F2 |

### 设计裁决（§6.1-6.4）

| 裁决 | 结论 | 评审意见 |
|---|---|---|
| 6.1 热度分 SQL 现算（LEFT JOIN 聚合），不在 service 内存算 | **认可** | 论证全文档最强：计数不落库 → 分页必须在排序后 → 单查询 + `gorm.Expr` 排序天然正确；备选「取全量内存算分再分页」违反 SOP §5.3 分页独立形态，否决有理 |
| 6.2 同分兜底 `created_at DESC` | **认可（附条件）** | 离散整数同分极常见，时间尾键保证确定性/跨页稳定，判断正确。条件：影响可见顺序，属实现层定案未用户确认——按 Q4 裁决标注即可 |
| 6.3 热度段与其他过滤正交叠加 | **认可（附条件）** | 同 query 路径零额外代码成立。条件：该卖点无测试覆盖（补一例 `tab=hot&tag=ai`，见 F5）；标注为未用户确认（Q4） |
| 6.4 热度公式双处实现（SQL + fake 镜像） | **认可** | 与项目既有模式一致（fake 本就镜像排序语义）；真 PG 集成测试兜底 SQL 正确性；fake 侧抽 `commentWeight` 常量提升可读。Q1 裁决见 §四 |

---

## 四、开放点裁决（Q1-Q5）

### Q1 热度公式双处实现的长期可维护性 —— **保留双处实现，不改为仅集成层测**

理由：SOP §7「测试只跨 seam」下，service 单测必须跨 fake repo 验证 hot 行为（权重/窗口/置顶），fake 镜像语义是本项目既有模式；SQL 侧正确性由真 PG 集成测试兜底，分工清晰。若改为「仅在集成层测 hot、fake 不镜像」，service 层对 hot 的回归保护归零，得不偿失。SQL 侧的 ×3/7d 字面量无法与 Go 共享常量，可接受，但建议**在 gorm_repo 侧加一行注释指向 fake 侧常量**，降低「改一边漏一边」的概率。

### Q2 LEFT JOIN 聚合子查询性能 —— **认可 MVP 够用，附 2 个可执行条件**

- 条件 1：本地冒烟步骤加一次 `EXPLAIN ANALYZE`（hot 查询），记录现状基线（行数级、执行时间）。
- 条件 2：决策记录写明**物化触发条件的量化口径**（如：likes/comments 合计行数超 N 万、或 hot 查询 p99 超 X ms），避免「何时物化」无限期悬置。
- 附注：子查询聚合的是**全量** likes/comments（全生命周期，非窗口限行），随数据量线性增长；索引支持分组但聚合是全表扫描。MVP 规模（50 人社团）完全可接受，两个条件只为防止无声退化。

### Q3 窗口用服务器时钟 `time.Now()` 的测试脆弱性 —— **认可，附 1 条测试纪律**

当前拓扑（Go + PG 同机、docker compose）无跨机时钟偏斜，「Go 进程 `time.Now()` vs PG `now()`」同机一致。未来若数据回放/分库分机，改 DB `now()` 是低风险迁移，无需现在做。**测试纪律**：seed 超窗/临窗帖时相对 `time.Now()` 留出边界余量（如 `±1s`），避免窗口边界测试 flaky。另记一已知性质（另一份评审补充，经复核属实）：offset 分页 + 移动窗口下，翻页途中窗口边界的帖可能跨页漂移——与全部流「新帖顶翻页」属同类已接受行为（§4 已排除游标分页），无需动作。

### Q4 §6.2/§6.3 实现层定案未经用户确认 —— **裁决：不回补 grilling，但必须显式标注**

§6.2 同分兜底（新帖优先）符合直觉、影响小；§6.3 过滤叠加仅经 URL 直访可见，非骨架级。两项都无需回用户 grilling。**条件**：在 PR 描述/决策记录中显式标注这两项为「实现层定案，未经用户确认」；若上线后用户对同分顺序有异议，改动是一行 Order，风险可控。

### Q5 空态文案「暂无热门帖子」未用户确认 —— **裁决：默认文案可用，标注即可**

产品文案属产品决策，但该文案只在「热门段且无帖子」时出现，影响面极小。用合理默认上线，PR 描述标注「产品文案，未用户确认」；若用户在意，上线前一行确认。

---

## 五、新发现问题（文档未覆盖，评审方补充）

| # | 级别 | 问题 | 要求 |
|---|---|---|---|
| F1 | **阻塞** | **交接依据三处引用指向不存在的文件**：§1/§2.3/§4 引用的 `docs/handoffs/61-hot-feed.md` 不在仓库（实际为 `docs/plans/issue-61-hot-feed.md`，且是计划非 handoff、节号 §1-§7 非 §0-§4）。同时该计划文件与本文档均**未提交**（git untracked），且缺 `grilling-decisions/issue-61-hot-feed-decisions.md`（SOP §3.4）。后果：D1-D3 定案依据、执行红线、PR#57 搭合要求只存在于未入库本地文件，其他会话/agent 无法接手复核，文档「全部可复现」声明对决策链不成立。 | ① 修正三处引用为 `docs/plans/issue-61-hot-feed.md` 并注明「计划」（若确要 handoff，则按 SOP 建真正的 handoff 文件）；② 补 `docs/handoffs/grilling-decisions/issue-61-hot-feed-decisions.md`（D1-D3 + 窗口语义解释 + §6.2/6.3 标注）；③ 计划 + 设计文档 + 评审意见书随本票 PR 一并提交入库。 |
| F2 | **重要** | **置顶帖与 7 天窗口相互作用未裁决**：`WHERE created_at >= now()-7d` 会把 8 天前被置顶的帖子从热门段**整体剔除**，D3「置顶恒顶」与不变量 3（「置顶帖恒在热门段顶部」）在该帖上不成立；且全部流无时间窗口，同一置顶帖在「全部」置顶可见、在「热门」消失，两段行为不一致。 | **用户已裁决：方案 A —— 窗口优先**。`is_pinned` 仅作为结果集内的排序键，不做成员豁免：hot 分支保持 `WHERE created_at >= now()-7d` 不变；超窗置顶帖不出现在热门段，窗口内置顶帖恒顶。落地三件事：① §6 新增裁决节「置顶 × 窗口」写明「窗口过滤优先、置顶在窗口内恒顶」；② §7 不变量 3 改写为「窗口内置顶帖恒顶」；③ §8 补测试用例：seed 8 天前置顶帖 → 断言热门段不出现，再 seed 窗口内置顶低热度帖 → 断言排第一。 |
| F3 | **重要** | **「7 天窗口」语义解释未显式写明**：当前实现 =「帖子创建时间在窗口内」+「全生命周期赞/评论计数」。若用户本意是「近 7 天的互动热度」（如 6 天前旧帖今天爆热），两种读法排序完全不同。D1 已用户确认，但文档从未显式写清读法。 | 在决策记录中显式写明该解释（一句即可）：`热度窗口=帖子创建时间 ∈ [now-7d, now]；计数=全生命周期赞/评论`。确认可与 F2 的用户回填合并为一次。 |
| F4 | 建议 | **`Feed.vue:24` 的 update 处理函数 cast 未列入改动清单**：`(v: string) => (tab = v as 'all' \| 'follow')` 在 §5.2 只提了「tab ref 类型补 'hot'（:10）」，未提这行。tab 宽化后该 cast 若不改为 `'all' \| 'hot' \| 'follow'`，运行时点「热门」后 `tab.value` 携带旧类型注解（TS 不报错但注解撒谎）。 | 改动清单补一行：`Feed.vue:24` cast 同步为 `(v: string) => (tab = v as 'all' \| 'hot' \| 'follow')`。 |
| F5 | 建议 | **§6.3 卖点（hot 与过滤正交叠加）无测试覆盖**：`?tab=hot&board_id=X` / `?tab=hot&tag=ai` 的叠加行为只有设计陈述，service/集成测试都未列该用例。 | 在 service 单测或 handler 集成测试补 1 例 hot+过滤叠加（如 `tab=hot&tag=ai`：窗口内该标签帖按热度排序）。成本极低。 |
| F6 | 建议 | **物化触发条件未量化**：Q2 已识性能风险，但「何时需物化计数列」无量化口径，可能无限期悬置。 | 决策记录写明触发条件（如 likes/comments 合计行数 > N 万 或 hot 查询 p99 > X ms 时立项物化）；本地冒烟加 `EXPLAIN ANALYZE` 记基线。 |
| F7 | 建议 | **「零命中」声明缺可复现命令**：文档写「全库 grep 'hot'/'热门' 零命中」，但字面子串语义下 `backend/internal/httpapi/handler/upload_test.go:57` 的 `photo.png` 含 "hot" 假命中。 | 文档附上精确命令 `grep -rE '\bhot\b|热门' backend/ frontend/src/`，避免后人复现时误判。 |
| F8 | **重要** | **hot 分支 JOIN 未按仓库既有惯例防「列遮蔽」**：仓库自己明文立规——GORM 默认 SELECT * 会把 JOIN 表同名列（id/created_at）覆盖主表字段，必须 `Select("content.posts.*")` 限定主表列（`gorm_repo.go:354-356` 原文；`ListFavoritedPosts:362`、`ListFollowedBoards:382` 均照办）。计划 §5.1 引入两个 LEFT JOIN 却未提 Select。当前子查询列名（target_id/like_cnt/post_id/cmt_cnt）与 posts 列无交集，**今天碰巧安全，但不受保护**，且违反仓库明规。（此条由另一份独立评审提出，本评审已对 gorm_repo.go 先例独立复核确认。） | ① hot 分支补 `Select("content.posts.*")`（一行，零风险）；② 子查询旁加防呆注释：「必须保持预聚合子查询形态；若改为直接 JOIN 原始 likes/comments 两表，赞/评论会笛卡尔扇出互乘、计数全错」；③ 集成测试断言返回帖 title/created_at 与 seed 一致，作列遮蔽回归网。 |
| F9 | 建议 | **热度口径可博弈**：评论 ×3 且自评论/楼中楼回复全部计入（与展示口径一致，不变量 8 已锁）→ 楼主可自评给自帖刷热度。50 人社区 MVP 可接受，不阻塞本票。（另一份独立评审提出，逻辑成立。） | 记录为未来迭代候选（排除自评 / 按去重评论者计数 / moderation 异常信号），本票不动。 |

---

## 六、通过条件清单（执行前勾选）

- [ ] **F1**：修正三处交接引用 → `docs/plans/issue-61-hot-feed.md`；补 `grilling-decisions/issue-61-hot-feed-decisions.md`；计划/设计文档/评审意见书随 PR 入库
- [ ] **F2**：按用户裁决 A 落地「窗口优先」：§6 写清「窗口过滤优先、置顶在窗口内恒顶」；§7 不变量 3 改「窗口内置顶帖恒顶」；补测试钉死（超窗置顶帖不出现在热门段、窗口内置顶帖排第一）
- [ ] **F3**：决策记录显式写明窗口语义 =「帖子创建时间窗口 + 全生命周期计数」
- [ ] **F4**：`Feed.vue:24` cast 同步含 `'hot'`
- [ ] **F5**：补 hot+过滤叠加测试（1 例）
- [ ] **F6**：决策记录写物化触发量化口径；冒烟加 `EXPLAIN ANALYZE`
- [ ] **F8**：hot 分支补 `Select("content.posts.*")` + 防扇出注释 + 列遮蔽回归断言
- [ ] **Q1 附带**：gorm_repo 侧加注释指向 fake 常量
- [ ] **Q4/Q5 附带**：PR 描述标注 §6.2/§6.3 实现层定案 + 「暂无热门帖子」产品文案未经用户确认

---

## 七、结语

本设计的骨架（SQL 现算热度分 + 排序后分页、跨 seam fake 镜像、集成测试兜底 SQL）经得起复核，代码级证据命中率在项目历史中属上乘；F1 是证据闭环的最后一公里，F3 与 F8 是语义与防护的两个钉死点，F2 已由用户裁决为方案 A（窗口优先）。本意见书已并入另一份独立评审的核实发现（列遮蔽 F8、热度博弈 F9、翻页漂移 Q3 注），并对该评审引用的仓库先例（`gorm_repo.go:354-356` 列遮蔽明规、`migration 0002:40` 容量注释、`fake_repo_test.go:84` withCreatedAt）独立复核确认属实。建议修复 F1/F3/F8 并落档 F2 后，以 §六清单作为本票 PR 自检表执行。

—— 评审方（独立复核：worktree `ai-forum-61` @ `9cb5ec0`，2026-08-25）
