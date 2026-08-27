# #72 @用户提及（Mentions）设计文档 评审意见书

> **评审对象**：《#72 @用户提及（Mentions）：设计文档（供评审）》（`mvp-4-zany-marble-设计文档.md`，数据时点 2026-08-27）
> **评审方式**：独立复核 —— 评审方以本机仓库 `C:\Users\liyongquan\ai-forum` @ `bac163f`（HEAD 已核实、工作树干净）为真值源，逐项核对 §2.2/§2.3/§2.4 的代码与迁移引用，并运行 `go build ./...` 验证基线；用 `gh` 补核 GitHub 状态。
> **评审结论**：**有条件通过**

---

## 一、总体结论

方向正确，证据质量在本项目历史中属上乘。我逐项核对了文档 §2.2/§2.3/§2.4 的 40+ 条事实引用——**命中率极高**：全部代码行号、迁移内容、正则、装配顺序、调用点数量均核实为真，仅 1 处引用错位（F2，实质正确、行号指错）。文档对「未复核项」的诚实标注（服务器/线上 DB/issue 状态）属实，`gh` 补核后 issue #72 确实 OPEN、开放 PR = 0，`0003`（非 handoff 所写 `0004`）的迁移编号纠正经实查成立。§7 不变量清单、三态运行时推演、Q6 自曝的大小写语义边界，都是质量很高的部分。

但有一个**动手前必须解决**的问题和几个执行期会踩的点：

1. **一个阻塞项（F1）**：mention 通知的 actor 快照（`ActorName`/`ActorAvatar`）来源**全文未指定**，而 `CreatePostCmd`/`CreateCommentCmd` 只有 `AuthorID`。执行 agent 必须发明决策，且该决策直接决定用户可见文案（`「X」提到了你` vs `提到了你`）。
2. **若干执行前钉死项（F2–F5）**：一处引用错位会把字段加到错误的结构体上；`newContentService()` 的测试辅助函数改造波及 **47 处调用点**（文档未量化）；auth_test 装配顺序与 `fakeUsers` 的封禁状态建模均未交代。

总体评价：骨架（多态存储、后端驱动渲染、best-effort、seam 划分）经得起复核，值得按其框架执行；满足 §六 清单后即可作为执行基线。

---

## 二、事实与证据复核

复核范围：文档 §2.2（迁移真值）、§2.3（代码真值）全部条目 + §2.4（GitHub 状态）。本机仓库可直接核验。

### 2.1 核实为真（抽样 35+ 项，全部命中）

| 文档主张 | 复核结果 |
|---|---|
| HEAD = `bac163f`，工作树干净，remote = `li-yongqvan/ai-forum` | ✅ `git log --oneline -1` / `git status` / `git remote -v` 逐字一致 |
| 迁移目录 0001–0010 + 0007 撞车 ×2 | ✅ `ls` 与文档列出的 11 个文件逐一对应 |
| notify schema 实为 `0003`（handoff 误写 0004） | ✅ `0003_notify_schema.sql` 为 notify，`0004` 为 moderation——handoff 引用错误属实 |
| `tags.go:17` tagRE 正则逐字 | ✅ `(^|[^\p{L}\p{N}_#])#([\p{L}\p{N}_]{1,30})` 完全一致 |
| `tags.go:26-32` 五个剥离正则（围栏→行内→图→链接→裸URL） | ✅ 逐行核实 |
| `tags.go:36-54` ParseTags：先剥离后匹配、小写、去重、保序 | ✅ |
| `UserProvider` 仅 3 方法、无按 username 查询 | ✅ `service.go:211-216` |
| `NewService(repo, users)` 两参、struct 仅 `{repo, users}` | ✅ `service.go:256-264` |
| `CreatePost` 标签 best-effort 不阻断 | ✅ `service.go:298-303` |
| **`CreateComment` 未保留 fetch 到的 post** | ✅ `service.go:311` 为 `if _, err := s.repo.GetPostByID(...)` 丢弃结果 |
| `DeletePost`/`DeleteComment` 行号 | ✅ `service.go:390-411` |
| `postViews` 批量富化、tags 富化在 822-826 | ✅ `service.go:762-867`；**且实证 `postView`（单数，753）委托 `postViews`（754）**——批量富化天然覆盖 PostDetail 路径 |
| `buildTree`/`commentView` | ✅ `service.go:571-620 / 870-895` |
| `PostView` 已有 `Tags` | ✅ 实质正确，但位置在 `service.go:29`，**非** `model.go:23-43`（见 F2） |
| `Repo` 接口尾部 tags 方法 | ✅ `repo.go:76-77`（文档写 75-77，含注释行，实质一致） |
| `gormRepo.DeletePost` 事务内硬删 post_tags | ✅ `gorm_repo.go:135-143`，尤其 140-141 |
| `gormRepo.DeleteComment` 仅软删不清理关联 | ✅ `gorm_repo.go:203-205` |
| `ListTagsByPostIDs` | ✅ `gorm_repo.go:468` |
| fake 仓库结构与 tag 方法 | ✅ `fake_repo_test.go:14,570,589` |
| 测试辅助 `newContentService`/`fakeUsers` | ✅ `service_test.go:18-24` |
| notify `validTypes` 6 类型、无 `mention` | ✅ `service.go:119-121` |
| `CreateNotification` 校验后落快照行 | ✅ `service.go:137-151` |
| `Notification` 模型字段 | ✅ `model.go:8-21`；`CreateNotificationCmd` 含 ActorAvatar | ✅ `notify/service.go:16-25` |
| moderation `Notifier` seam + `NotificationCmd` | ✅ `moderation/service.go:138-152` |
| adapter 先例 | ✅ `moderation_adapter.go:126-148` |
| `follow.go` 通知吞错不回滚 | ✅ `follow.go:73-78` |
| user `Repo.GetUserByUsername`（gorm `WHERE username=?`、fake 按 map） | ✅ `repo.go:9` / `gorm_repo.go:27-33` |
| `Service` 内部用、未暴露到接口 | ✅ 接口 `service.go:88-107` 无；内部 `service.go:191/227` |
| `IsActive` 软删/不存在→false | ✅ `service.go:371-381`（fail-closed 属实） |
| `GetUser`/`toView` | ✅ `service.go:383-389` |
| `users.username` VARCHAR(64) UNIQUE、无大小写不敏感 | ✅ `0001_user_schema.sql:8`；FK 政策注释 `:2` |
| `main.go` contentSvc(line 48) 在 notifySvc(line 50) 前 | ✅ `cmd/api/main.go:47-50` |
| `userProviderAdapter` 3 方法 + `NewEngine` 7 参签名 | ✅ `router.go:22-45, 84` |
| **`content.NewService` 调用点恰 2 处** | ✅ grep 命中仅 `main.go:48` + `auth_test.go:36` |
| `content.likes` 多态先例 | ✅ `0002_content_schema.sql:59-67` |
| `0003:7` type 列级 CHECK | ✅ `type VARCHAR(16) NOT NULL CHECK (type IN (...))` |
| `0008` 全文 | ✅ 与文档引文逐字一致（DROP/ADD 两行） |
| 前端 `TAG_RE`/`md`/`extractTags` | ✅ `md.ts:28,35-74,81-100` |
| **`md()` 生产调用点恰 2 处** | ✅ grep 仅 `PostDetail.vue:355` + `CommentTree.vue:81`（余为 md.test.ts / 注释） |
| 路由 `/post/:id`、`/user/:id` | ✅ `router/index.ts:13,15` |
| `Post.tags?`、`CommentNode` 无 mentions、`NotificationType` 6 值、`AppNotification` | ✅ `types.ts:41-61,63-74,125,127` |
| `Notifications.vue` textFor/linkTo/onItemClick | ✅ `Notifications.vue:94-113,123-139,141-154` |
| 基线可编译 | ✅ `cd backend && go build ./...` **EXIT 0** |

### 2.2 不实 / 冲突（1 项）

| 文档主张 | 复核结果 |
|---|---|
| `PostView` 已有 `Tags`、`CommentView` 无 mentions —— `model.go:23-43 / 64-74` | ❌ **行号指错**：`model.go:23-43` = `Post` 领域结构体、`64-74` = `Comment` 领域结构体（均无 Tags）。`PostView` 实际在 `service.go:23-43`（`Tags` 在 `:29`）、`CommentView` 在 `service.go:64-74`。实质正确，引用错位（见 F2）。 |

### 2.3 不可复核（4 项，全部如实标注）

| 项 | 说明 |
|---|---|
| 线上服务器（`122.51.233.225:8888`）healthz / 容器 / HEAD | 评审方无 SSH 通道，**无法实测**。文档已标「待复核」，实现阶段按 SOP 落实（§8 上线验证）。 |
| 线上 DB：`notifications` CHECK 现状、`content` schema 表清单 | 无 psql 通道，**无法实测**。**风险低**：0012 迁移为幂等形态（`DROP CONSTRAINT IF EXISTS` + 全量 7 类型 `ADD`），即使线上枚举与假设不符也能正确收敛。 |
| 地图 issue #1「决策 29」（0007 撞车记录） | 未复核（旁证，与本票无关；0007 两文件存在已实证）。 |
| `frontend` 基线 `npm run test:unit && npm run build` | 未运行（评审方未跑前端构建；但后端 `go build` 已通过，前端改动为纯增量）。 |

---

## 三、逐条评审

### 3.1 Grilling 决策 D1–D7（§3）

| 决策 | 结论 | 评审意见 |
|---|---|---|
| D1 编辑不重解析 | **认可** | 实证 Service 接口无 `UpdatePost/UpdateComment`（`service.go:219-252` 无更新方法），MVP 无编辑，成本为零。Q1 的 `ReplaceMentions` 接缝留待未来即可。 |
| D2 大小写字面匹配 | **认可** | 实证 schema 无 CITEXT/大小写不敏感唯一（`0001:8`）、`GetUserByUsername` 为大小写敏感 `WHERE username=?`。端到端（解析→落库→渲染）行为一致。Q6 的边界（同一内容 @ 两个真实同名用户 → 通知两人）经推演成立。 |
| D3 不存在用户名纯文本 | **认可** | 「后端只落库有效用户 + 前端按 mentions 列表 gate」的设计我做了对抗验证：即使前后端正则出现漂移，未命中列表的 `@text` 一律保持纯文本，不会产生错误链接（见 Q3 裁决）。 |
| D4 去重 + 自我过滤 | **认可** | 表 `UNIQUE (target_type,target_id,mentioned_user_id)` + `processMentions` 按 id 去重双重保障；无并发竞态（单次创建进程内处理）。 |
| D5 `@`+1–30 中英数字下划线 | **认可** | 与 tagRE 同构（`tags.go:17`）；`users.username` 虽为 VARCHAR(64)，30 上限保守安全且与 #54 统一，认可。 |
| D6 软删清理 mentions | **认可（附条件）** | 照 `post_tags` 卫生先例（`gorm_repo.go:140-141`）正确。**条件**：`DeleteComment` 现为纯软删单语句（`gorm_repo.go:203-205`），追加清理须**改为事务内执行**（照 `DeletePost` 的事务形态），计划 §5.3.2 只写了「事务内追加」于 DeleteComment，需同步确认 DeleteComment 的 gorm 实现确实改造成事务。 |
| D7 通知锚点跳帖子详情 | **认可** | 与 `linkTo` 既有 like/comment/reply 分支一致（`Notifications.vue:129-132`）；mention 的 TargetID 恒为 post_id，路由存在（`router/index.ts:13`）。 |

### 3.2 关键设计裁决（§6）

| 裁决 | 结论 | 评审意见 |
|---|---|---|
| 6.1 一张多态表 | **认可** | `content.likes` 先例逐字核实（`0002:59-67`：`target_type CHECK ('post','comment')` + `UNIQUE(user_id,target_type,target_id)`）。mentions 完全同构。未来 `ReplaceMentions`（DeleteByTarget + Create）接缝够用。 |
| 6.2 `mentioned_username` 快照列 | **认可** | 实证 `user.Service` **无改名功能**，快照在 MVP 内永不漂移；快照省读侧 join。备注：对齐 `VARCHAR(64)` 正确。 |
| 6.3 后端驱动渲染 | **认可（重点认可）** | 这是全文档最扎实的一条。我实测该设计**天然免疫前后端正则漂移**（见 Q3），且 `/user/:id` 数字路由（`router/index.ts:15`）使 username→id 映射只能由后端提供，前端正则方案确实不可行。被 D7 否决的备选成立。 |
| 6.4 best-effort | **认可（附条件）** | 与 #54（`service.go:298-303`）+ follow（`follow.go:78`）双先例一致。**条件**：mentions 落库失败与通知发送失败**都必须 `slog.Warn` 留痕**（计划已写 slog.Warn ✓），尤其通知吞错——照 #33 评审的教训，不要复制 follow.go 无日志的旧瑕疵。 |
| 6.5 评论锚点统一帖子 | **认可** | `Notification` 无评论锚点字段（`model.go:15-16` 仅 TargetType/TargetID），评论级锚点需新增语义 + 前端 `revealComment` 联动链路，#47 的滚动定位能力存在但通知点击链路未接入。MVP 不做、登记已知缺口（§7 #9）正确。 |
| 6.6 `GetUserByUsername` 只返回 active | **认可（附建议）** | 语义定案正确（D2/D3 统一策略；区分「存在但封禁」会渲染指向封禁用户的链接，与 D3 冲突——理由成立）。**建议**：§5.6 的 adapter 对每个 mention 做 **2 次** user 域调用（`GetUserByUsername` + `IsActive`），可在 `user.Service.GetUserByUsername` 内合并为「存在且 active 才返回」，adapter 一次调用（见 F9）。 |

---

## 四、开放点裁决（Q1–Q9）

### Q1 存储形态 —— **认可**

`content.likes` 多态先例逐字核实，`(target_type,target_id)` + UNIQUE 一致。`ReplaceMentions` 接缝：UNIQUE 使 upsert/delete+insert 均可实现，D1 前提下够用。无异议。

### Q2 快照列 —— **认可**

当前无改名功能（实证 `user.Service` 接口无 rename），「用户名已变」的担忧在 MVP 内不成立；快照零 join 收益真实。保留快照。

### Q3 前端渲染耦合 / 漂移兜底 —— **认可（附 1 条件）**

「测试钉行为 + 后端列表 gate」双层是否足够——我做了对抗推演：

- **后端列表 gate 使漂移自愈**：即便前后端正则不一致，未命中 mentions 列表的 `@text` 前端一律纯文本，**不会产生错误链接**。例如 `#话题@alice`：后端 `@` 前是汉字（非边界）不落库；前端标签先被替换为 `\x00N\x00` 占位符后 `@` 前是 NUL（边界）会匹配——但因 'alice' 不在列表，保持纯文本，两端语义一致。**列表 gate 正是防漂移的关键，设计成立。**
- **残余漂移（接受并测试）**：内容 `<@alice` 或 `>@alice` 时，后端 `@` 前是 `<`/`>`（边界）→ 落库 + 通知；前端 esc 后为 `&lt;@alice`，`@` 前是字母 `t`（非边界）→ 不渲染链接。结果是「**通知已发但无可见链接**」。这与 #54 标签同类的既有接受边界（esc-first 模式），概率极低。**条件**：把该边界写入计划（§7 不变量清单补一行），并加一条 `md.test.ts` 断言（`<@alice` 不渲染链接）。

结论：双层防护足够，补 1 条断言 + 文档化残余边界即可。

### Q4 失败语义 best-effort —— **认可**

派生元数据不阻断主命令，与双先例一致（实证）。「通知可能丢」在 MVP 50 人规模可接受。条件同 §6.4（失败必须 Warn 日志）。

### Q5 评论通知锚点是否纳入 revealComment —— **裁决：不做（认可）**

`linkTo` 现有结构（`Notifications.vue:123-139`）无评论级锚点语义，接入 `revealComment` 需前端链路改造，MVP 收益低。已登记 §7 #9 已知缺口，正确。未来补做时注意：评论可能属于已被折叠的回复链，锚点需同时处理滚动 + 展开。

### Q6 大小写语义边界 —— **认可，与定案一致**

推演确认：`@alice` 与 `@Alice` 各查一次大小写敏感 `GetUserByUsername`；两个用户名都存在时通知两人、各落一条 mentions（不同 `mentioned_user_id`，UNIQUE 不冲突）。这正是 D2 的承诺边界，用户已确认。提示：**`@Alice` 而真实用户是 `alice` 时无人被通知、无链接**（D2 的字面匹配成本），已在 §7 #4 隐含，建议在 §3 D2 依据处补一句白话说明，避免用户误判为 bug。

### Q7 未复核项 —— **补核两项，另两项保持未复核**

- ✅ 评审方已用 `gh` 补核：**issue #72 存在且 OPEN**（"[Feature] @用户提及（Mentions）"）、**开放 PR = 0**。文档标「待复核」诚实且现状已确认。
- ⚠️ 线上服务器 / 线上 DB：无通道，保持未复核。**风险低**：0012 幂等迁移使线上 CHECK 未知不构成执行障碍（见 §2.3）。
- ⚠️ 地图「决策 29」：旁证，不影响。

### Q8 装配顺序 —— **认可，已实证无遗漏，附 1 条件**

`content.NewService` 调用点 grep 命中**恰好 2 处**（`main.go:48` + `auth_test.go:36`），无第三处，Q8 的担心不成立。`content.UserProvider` 实现者亦恰好 2 处（adapter + fakeUsers），都在计划覆盖内。**条件（F4）**：`auth_test.go` 的 `newEngineWithUploads` 中 notifySvc 在 `:38` 构造、晚于 contentSvc（`:36`）——改签名后同样需要**调整顺序**，不只是改签名一行。

### Q9 评论 title 获取 —— **认可，且担心可解除**

实证 `CreateComment` 用的是 `s.repo.GetPostByID`（`service.go:311`），**repo 层无浏览量副作用**（`IncrementViewCount` 只在 `GetPost` service 方法内，`service.go:479`）。Q9 担心的 `GetPost` 副作用不在此路径，保留 fetch 到的 post 取 `Title` 即可，无需改走 `GetPostMeta`。**注意**：fetched 的 post 若为软删（理论上此时 `ErrPostNotFound`），无副作用。

---

## 五、新发现问题（文档未覆盖，评审方补充）

| # | 级别 | 问题 | 要求 |
|---|---|---|---|
| F1 | **阻塞** | **mention 通知的 actor 快照来源未指定**。§5.3 定义 `MentionNotificationCmd{..., ActorName, ActorAvatar, ...}`，但 `CreatePostCmd`/`CreateCommentCmd` 只有 `AuthorID`（实证 `service.go:14-20, 57-62`），`processMentions` 拿什么填 ActorName/ActorAvatar 全文未说。执行 agent 必须发明：① `s.users.GetUserView(ctx, authorID)` 现查（多一次 provider 调用）；② 改 cmd 签名加 actor 字段（波及 httpapi handler 与 JWT claims 装配）；③ 留空（通知文案退化为「提到了你」无「谁」，用户不可知）。三者 UX 差异显著。 | §5.3 `processMentions` 流程显式写：**先一次 `s.users.GetUserView(ctx, authorID)` 取 Username/AvatarURL（postView 本来就要 fetch 作者，可复用该结果），填充全部 `MentionNotificationCmd` 的 ActorName/ActorAvatar**。若选②则同步列出 handler 改动点。 |
| F2 | **重要** | **引用错位：`PostView`/`CommentView` 在 `service.go` 不在 `model.go`**（§2.3 及 §5.3.1 均引错）。`model.go:23-43/64-74` 实为 `Post`/`Comment` **GORM 领域结构体**。照引用在 model.go 的 `Post`/`Comment` 上加 `Mentions` 字段会污染 GORM 表映射（多余列会触发 gorm 写入未知列错误）。 | 全部改为 `service.go:23-43`（PostView，Tags 在 `:29`）/ `service.go:64-74`（CommentView）。§5.3.1 明确：`Mention` 领域模型放 `model.go`（照 Tag 先例），`MentionView` + PostView/CommentView 字段放 `service.go` DTO 区。 |
| F3 | **重要** | **`GetUserByUsername` 错误分支未定**。§5.3 只写「不存在/非 active → 跳过」；DB 报错（非 not-found）分支未写，agent 需发明（返回 error 中止发帖？还是跳过？）。 | 显式写：**非 not-found 错误 → `slog.Warn` + 跳过该 username，不阻断**（与 §6.4 best-effort 一致）。 |
| F4 | **重要** | **`auth_test.go` 装配顺序**。§5.6 只说「同步 `content.NewService` 新签名」，但 `newEngineWithUploads` 中 notifySvc 构造在 `:38`、contentSvc 在 `:36`——注入 Notifier 后顺序必须同样调整，否则编译即证但测试构造缺依赖。 | 与 `main.go` 同改：`auth_test.go` 把 notifySvc 提前到 contentSvc 之前。 |
| F5 | **重要** | **测试辅助函数改造波及面未量化**。§5.3.4「`newContentService` 返回 `(*fakeRepo, Service, *fakeNotifier)` 或补 notifier 记录器」——前者会使 `service_test.go` 中 **47 处** `newContentService()` 调用点（grep 实测）全部编译失败；后者「补记录器」如何在不改 47 处的前提下让测试取到记录器，未说。另外 `fakeUsers`（`fake_repo_test.go:661`）无封禁状态建模，§8 的「封禁跳过」用例无处造态。 | 建议：`newServiceWith` 默认注入**非 nil** 的 `fakeNotifier`（包级记录器或结构体字段），保持 `newContentService()` 双返回值不变（47 处零改动），新增专门测试构造/访问器暴露记录器；`fakeUsers` 增加「按 username 返回 not-found」注入位（如 `errByName map[string]error`），使封禁/软删跳过可测。计划需写明此方案，不许留「实现时定」。 |
| F6 | 建议 | **`<@`/`>@` 残余漂移边界**（详见 Q3 裁决）：后端通知已发但前端不渲染链接。 | 接受 + 写入 §7 边界清单 + 一条 `md.test.ts` 断言。 |
| F7 | 建议 | **`linkTo` mention 分支缺 null 守卫**：`return \`/post/${n.target_id}\`` 与其他 case（`n.target_id != null` 才跳）不一致。mention 恒有 target_id，风险低。 | 补 `n.target_id != null ? \`/post/${n.target_id}\` : null`。 |
| F8 | 建议 | **feed 卡片摘要中的 `@username` 无链接**：`plainText`（`md.ts:103-112`）不经过 md()，卡片摘要里 `@alice` 以纯文本显示。MVP 可接受，但应文档化。 | §7 边界清单补一行：列表/摘要不渲染 mention 链接，详情页才渲染。 |
| F9 | 建议 | **每 mention 两次 user 域调用**：§5.6 adapter 先 `GetUserByUsername` 再 `IsActive`（N+1 翻倍）。 | 可在 `user.Service.GetUserByUsername` 内合并 active 过滤，adapter 单次调用；或保留现状，规模内均可，选一写明。 |
| F10 | 建议 | **`Mention` GORM 模型缺 `TableName()` 声明**：计划 §5.3.1 未显式列出，而代码库惯例是每个模型显式 `TableName()`（`model.go:24,35,...`）。照抄即可，仅提示。 | 实现时按惯例补 `func (Mention) TableName() string { return "content.mentions" }`。 |

---

## 六、通过条件清单（执行前勾选）

- [ ] **F1**：§5.3 `processMentions` 补 actor 快照来源（建议：复用 `GetUserView(ctx, authorID)` 一次，填全部 ActorName/ActorAvatar）
- [ ] **F2**：§2.3/§5.3.1 引用改为 `service.go:23-43/64-74`；明确 Mention 领域模型在 model.go、MentionView/字段在 service.go DTO 区
- [ ] **F3**：§5.3 补「非 not-found 错误 → Warn + 跳过」
- [ ] **F4**：`auth_test.go` notifySvc 提前到 contentSvc 前（与 main.go 同改）
- [ ] **F5**：测试辅助改造方案定案（保持 47 处调用点零改动 + fakeNotifier 记录器 + fakeUsers 注入 not-found 位）
- [ ] **Q3 附带**：`<@`/`>@` 边界写入 §7 + 一条 md.test 断言
- [ ] **D6 附带**：`gormRepo.DeleteComment` 改为事务内软删 + 清 mentions
- [ ] **§6.4 附带**：mentions 落库失败与通知失败均 `slog.Warn`（计划已含，落实现查）
- [ ] **F7/F8**：linkTo null 守卫 + 卡片摘要行为文档化（低成本，随实现带上）
- [ ] 合并后：SOP §10 更新地图 #1 + 关闭 issue #72（§10 落档）

---

## 七、结语

这份设计的骨架——多态存储、后端驱动渲染、best-effort、seam 划分——全部经得起对抗复核，证据行号在项目历史上属最扎实的一档；唯一事实问题是一处引用错位（F2），唯一需要补的承重墙是 actor 快照来源（F1）。补齐 F1 + 按 §六 勾选后，即可作为执行基线。建议以本意见书 §六 作为 PR 自检表。

—— 评审方（独立复核：ai-forum @ bac163f + gh @ issue #72，2026-08-27）

---

## 附：执行检查表（对抗协议 Pass 1 产出，可审计）

| # | 检查项 | 状态 |
|---|---|---|
| 1 | HEAD = `bac163f`、工作树干净、remote = li-yongqvan/ai-forum | ✅ `git log/status/remote -v` |
| 2 | 迁移目录 0001–0010 + 0007×2 | ✅ `ls` |
| 3 | `content.NewService` 调用点恰 2 处 | ✅ grep `main.go:48` + `auth_test.go:36` |
| 4 | `tags.go:17` tagRE、`:26-32` 五剥离、`:36-54` ParseTags | ✅ Read 逐行 |
| 5 | `UserProvider` 3 方法（service.go:211-216）、`NewService` 两参（256-264） | ✅ Read |
| 6 | CreatePost best-effort（298-303）、CreateComment 丢弃 post（311） | ✅ Read |
| 7 | DeletePost/DeleteComment（390-411） | ✅ Read |
| 8 | postViews 批量富化 + tags@822-826；postView@753 委托 | ✅ Read |
| 9 | buildTree@571-620、commentView@870 | ✅ Read |
| 10 | `PostView.Tags` 实际位置 | ✅ Read（发现引用错位，`service.go:29`） |
| 11 | repo.go:76-77 tags 方法；gorm_repo.go:135-143/203-205/468 | ✅ Read |
| 12 | fake_repo_test.go:14/570/589；service_test.go:18-24 | ✅ Read |
| 13 | notify validTypes（119-121）、CreateNotification（137-151）、模型（model.go:8-21）、Cmd（16-25） | ✅ Read |
| 14 | moderation Notifier seam（138-152）+ adapter（126-148） | ✅ Read |
| 15 | follow.go:73-78 吞错 | ✅ Read |
| 16 | user Repo.GetUserByUsername（repo.go:9 / gorm 27-33）；Service 接口未暴露（88-107）；IsActive（371-381）；GetUser（383-389） | ✅ Read |
| 17 | main.go 装配 contentSvc:48 < notifySvc:50 | ✅ Read |
| 18 | router.go adapter（22-45）+ NewEngine（84） | ✅ Read |
| 19 | 0002 likes 多态（59-67）、0003 CHECK（:7）、0008 全文、0001（:2/:8） | ✅ Read |
| 20 | md.ts TAG_RE:28 / md:35-74 / extractTags:81-100 | ✅ Read |
| 21 | md() 生产调用点恰 2 处（PostDetail:355 / CommentTree:81） | ✅ grep |
| 22 | router /post/:id:13 /user/:id:15；types.ts（41-61/63-74/125/127） | ✅ Read |
| 23 | Notifications.vue textFor:94-113 / linkTo:123-139 / onItemClick:141-154 | ✅ Read |
| 24 | 基线 `go build ./...` | ✅ **EXIT 0** |
| 25 | `content.UserProvider` 实现者恰 2（adapter+fakeUsers） | ✅ grep |
| 26 | `user.Service` 实现者仅 *service | ✅ grep |
| 27 | GitHub：issue #72 OPEN、开放 PR = 0 | ✅ `gh` |
| 28 | `newContentService()` 调用点 | ✅ grep 实测 **47 处**（发现 F5） |
| 29 | 线上服务器 / 线上 DB / 地图决策 29 | ⚠️ 无法实测（无 SSH/psql；旁证） |
