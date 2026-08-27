# #72 @用户提及（Mentions）：设计文档（供评审）

> 文档用途：交付专业评审 agent（plan-review）的评审对象。范围 = 背景 / 真值核对 / 决策记录 / 实现方案 / 不变量 / 验证。
> 溯源约定：**事实**标来源（代码 `file:line` / 迁移文件实查 / GitHub issue / grilling 用户确认）；**判断性裁决**单独标注【决策】并给出理由与备选，不冒充事实。
> 数据时点：2026-08-27（真值核对执行日；服务器/线上 DB 项未实查，见 §2.1/§2.2 标注）。
> 评审状态：**有条件通过（2026-08-27 独立评审）**——1 阻塞项 F1 + 重要项 F2-F5 + 建议项 F6-F10 全部采纳落地（见 §10）。评审意见书：`mvp-4-zany-marble-设计文档-评审意见书.md`。**本版为实现基线**。

---

## §0 项目上下文（给零背景评审 agent，先读本节）

**这是什么**：ai-forum——社团内部 AI 主题社区的移动端论坛 MVP，已上线 `8888` 容器化部署。
- 前端：Vue 3 + TS + Vite + Vant（PWA），纯静态部署（`frontend/`）。
- 后端：Go（Gin + GORM + PostgreSQL），单进程模块化单体（`backend/`）。
- 数据库：PostgreSQL 单实例、**多 schema**，每域一个：`user` / `content` / `notify` / `moderation`。

**后端架构纪律（评审本设计必须理解）**：
- 4 个领域包（`user`/`content`/`notify`/`moderation`），每包固定 `model.go`/`repo.go`/`gorm_repo.go`/`service.go`。
- **跨包调用禁止直接 import 他包**：调用方在本包定义所需接口（seam），组合根（`httpapi`/`main`）装配实现。证据：`backend/content/service.go:211-216`（UserProvider seam）、`backend/moderation/service.go:138-152`（Notifier seam）、`backend/internal/httpapi/moderation_adapter.go:126-148`（adapter）。
- **FK 政策（`docs/design-principles.md` §7）**：包内物理 FK / 跨包逻辑外键——跨 schema 引用只建索引不建约束。证据：`backend/migrations/0002_content_schema.sql:30`（`posts.author_id` 逻辑 FK，仅 `CREATE INDEX`）。
- 哨兵错误变量 + 调用方映射 HTTP 状态；测试跨 seam（内存 fake + testcontainers 真 PG，`backend/internal/testutil/db.go`）。
- 可查阅：`docs/workflow.md`（SOP）、`docs/design-principles.md`、地图 issue #1。

**与本票相关的前序工作**：
- **#54 标签系统**（直接参考）：`#` 字面规则解析落库 → 前端高亮。`backend/content/tags.go`、`backend/migrations/0007_tags_schema.sql`、`frontend/src/utils/md.ts`。
- **#32 通知中心**（复用通知基础设施）：`backend/notify/`，快照自足行（#4 D4），`notify` 零依赖。
- **#34/#53**：`notify.notifications.type` 已含 `report_handled`（`backend/migrations/0008_notify_type_report_handled.sql`）；`user.Service.IsActive` 已存在（`backend/user/service.go:371`）。
- **#59/#61**：notify 域补齐私信（`notify.messages`）；首页热门流（`posts?tab=hot`）。

**角色与权限**：`user.users.role`（member/moderator/admin）+ `status`（active/banned）；写操作统一 `middleware.RequireActive` 拦截（`backend/internal/httpapi/router.go:139-140`）。本票通知接收人过滤复用 `IsActive`。

**术语速查**：
- mentions（提及）：`@username` 出现在帖子/评论正文，指向某用户。
- 快照字段：通知/关联表里创建时写死的最小画像（actor_name / mentioned_username），读侧不再 join。

---

## §1 背景与目标（为什么做）

- 需求来源：GitHub **issue #72**（2026-08-26 立项）。统筹方 2026-08-26 会话选项投票中，用户选择**优先实现 @用户提及**。
- 交接依据：`docs/handoffs/` 对应 handoff 文档（`#72 @用户提及`，Temp 目录）；SOP `docs/workflow.md`（取票→设计→评审→实现→测试→PR→部署→验证→地图更新）。
- **目标**：用户在帖子正文或评论正文输入 `@username` 后：
  1. 被提及用户收到一条通知；
  2. 正文中 `@username` 渲染为指向该用户主页的链接；
  3. 不存在的用户 / 被删除用户按统一策略处理（不通知、链接行为一致）。

---

## §2 真值核对（数据来源，全部可复现）

> 本设计文档产出于无服务器 SSH 通道的环境，**服务器与线上 DB 项未实查（待复核）**；代码真值为本机仓库 `C:\Users\liyongquan\ai-forum` 实查（HEAD `bac163f`，见 §2.4）。

### 2.1 服务器真值（未复核）

- **未复核（待复核）**：线上 `122.51.233.225:8888` 的 healthz / 容器状态 / HEAD。实现阶段按 SOP（`docs/workflow.md`）规定的只读命令实查。本文档不假设线上状态。

### 2.2 DB / 迁移真值

**本机 `backend/migrations/` 目录实查（2026-08-27）**：

```
0001_user_schema.sql
0002_content_schema.sql
0003_notify_schema.sql        ← notify schema 实为 0003
0004_moderation_schema.sql
0005_content_seed.sql
0006_reports_reporter_note.sql
0007_reports_target_author.sql
0007_tags_schema.sql           ← 编号撞车（#53/#54 并行，地图决策 29 已记录）
0008_notify_type_report_handled.sql
0009_moderation_actions_index.sql
0010_fix_user_intra_package_fks.sql
```

- ✅ **发现（handoff 引用错误）**：handoff 写「迁移：`backend/migrations/0004_notify_schema.sql`」，**实际 notify schema 是 `0003_notify_schema.sql`**（`0004` 是 moderation schema）。后续引用 notify 表结构应以 `0003` 为准。
- ✅ **迁移编号无冲突**：最新编号 `0010`。本设计新增 `0011`/`0012` 不与既有冲突（吸取 0007 撞车教训，已在本地目录核对）。

**`0003_notify_schema.sql` 关键行（实查）**：
```sql
-- line 7: notifications.type 为列级 CHECK（0008 已通过 DROP/ADD 追加 report_handled）
type VARCHAR(16) NOT NULL CHECK (type IN ('follow','like','comment','reply','report_result')),
```

**`0008_notify_type_report_handled.sql` 全文（实查）**——本设计新增 `mention` 类型的迁移照此写法：
```sql
ALTER TABLE notify.notifications DROP CONSTRAINT IF EXISTS notifications_type_check;
ALTER TABLE notify.notifications ADD CONSTRAINT notifications_type_check CHECK (type IN ('follow','like','comment','reply','report_result','report_handled'));
```

- **未复核（待复核）**：线上 DB 的 `notifications` 实际 CHECK 枚举、`content` schema 实际表清单。实现前按 SOP 用 psql 实查。

### 2.3 代码真值（本机仓库 grep）

#### content 域（`backend/content/`）

| 事实 | 证据 |
|---|---|
| 标签正则 `(^\|[^\p{L}\p{N}_#])#([\p{L}\p{N}_]{1,30})`，长度上限 30 | `tags.go:17` |
| 5 个剥离正则（围栏代码→行内码→图→链接→裸 URL），每处替换为单空格 | `tags.go:26-32` |
| `ParseTags`：先剥离后匹配，归一化小写、去重、保序 | `tags.go:36-54` |
| `CreatePost` 调 `ParseTags` + `repo.ReplacePostTags`；**写失败仅 `slog.Warn` 不阻断发帖**（best-effort 先例） | `service.go:298-303` |
| **`CreateComment` 不调 `ParseTags`**（标签目前仅帖子） | `service.go:307-341` |
| `UserProvider` 接口仅 `FollowedUserIDs/GetUserView/FollowsUser`，**无按 username 查询** | `service.go:212-216` |
| `service` struct 仅 `{repo, users}`；`NewService(repo, users)` 两参 | `service.go:256-264` |
| `DeletePost` 调 `repo.DeletePost`；`DeleteComment` 调 `repo.DeleteComment` | `service.go:390-411` |
| `postViews` 批量富化（含 #54 `tagsByPost` 批量查询 best-effort） | `service.go:762-867`（tags 富化 822-826） |
| `buildTree` / `commentView` 组装评论读模型 | `service.go:571-620 / 870-895` |
| `PostView` 已有 `Tags []string`；`CommentView` 无 mentions 字段 | **修正（评审 F2）**：`PostView` 在 `service.go:23-43`（`Tags` 在 `:29`）、`CommentView` 在 `service.go:64-74`——**不在 `model.go`**（`model.go:23-43/64-74` 实为 GORM 领域结构体 `Post`/`Comment`） |
| `Repo` 接口尾部已有 tags 方法（`ReplacePostTags`/`ListTagsByPostIDs`） | `repo.go:75-77` |
| `gormRepo.DeletePost` 事务内**顺带硬删 post_tags**（软删不触发 FK CASCADE，#54 卫生） | `gorm_repo.go:135-143`（尤其 140-141） |
| `gormRepo.DeleteComment` 仅软删，**不清理关联**（评论本无 tags） | `gorm_repo.go:203-205` |
| fake repo：`fakeRepo` 结构 + tag 方法（`ReplacePostTags` 570 / `ListTagsByPostIDs` 589） | `fake_repo_test.go:14,570,589` |
| 测试构造辅助 `newContentService() (*fakeRepo, Service)`；`fakeUsers{names: map[int64]string{1:"alice",...}}` | `service_test.go:18-24` |

#### notify 域（`backend/notify/`）

| 事实 | 证据 |
|---|---|
| `validTypes` 白名单含 6 类型，**无 `mention`** | `service.go:119-121` |
| `CreateNotification` 校验类型后落快照行 | `service.go:137-151` |
| `Notification` 模型：RecipientID/Type/ActorID/ActorName/ActorAvatar/TargetType/TargetID/TargetTitle/IsRead | `model.go:8-21` |
| 跨包通知 seam 先例：`moderation.Notifier` + `NotificationCmd`（快照由调用方算好） | `moderation/service.go:138-152` |
| adapter 先例：`notifierAdapter` 翻译 moderation→notify 命令 | `moderation_adapter.go:126-148` |
| handler 层跨包通知先例：`follow.go` `notifyFollow` 吞错不回滚关注 | `internal/httpapi/handler/follow.go:73-78` |

#### user 域（`backend/user/`）

| 事实 | 证据 |
|---|---|
| **`Repo` 已有 `GetUserByUsername`**（gorm 实现 `WHERE username = ?`，fake 实现按 map） | `repo.go:9` / `gorm_repo.go:27-33` / `fake_repo_test.go:74-79` |
| `Service` 内部已用 `GetUserByUsername`（注册查重 / 登录），但**未暴露到 Service 接口** | `service.go:191` / `service.go:227` |
| **`IsActive` 已存在**：软删/不存在返回 `false`（安全拦截） | `service.go:371-381` |
| `GetUser(ctx, id)` → `UserView` | `service.go:383-389` |
| `users.username`：`VARCHAR(64) UNIQUE NOT NULL`，**无大小写不敏感约束** | `migrations/0001_user_schema.sql:8` |

#### httpapi / 装配

| 事实 | 证据 |
|---|---|
| `userProviderAdapter` 实现 `content.UserProvider` 3 方法（`GetUserView` 映射 user.UserView→content.UserView） | `router.go:22-45` |
| `NewEngine` 签名：`(cfg, jwtMgr, userSvc, contentSvc, uploadSvc, notifySvc, moderationSvc)` | `router.go:84` |
| **`main.go` 装配顺序：`contentSvc`（line 48）在 `notifySvc`（line 50）之前** → 注入 Notifier 需调整顺序 | `main.go:47-50` |
| **`content.NewService` 全部调用点**：`main.go:48` + `auth_test.go:36`（grep 命中 2 处）→ 改构造器签名须同步两处 | `cmd/api/main.go:48` / `internal/httpapi/handler/auth_test.go:36` |

#### 前端（`frontend/`）

| 事实 | 证据 |
|---|---|
| `TAG_RE = /(^\|[^\p{L}\p{N}_#])#([\p{L}\p{N}_]{1,30})/gu`；`md(src)` 管道：转义→围栏→行内码→图→链接→裸 URL→标签→粗体→段落→还原占位 | `md.ts:28,35-74` |
| `extractTags` 剥离顺序（与 md() 同） | `md.ts:81-100` |
| **`md()` 生产调用点仅 2 处**：`PostDetail.vue:355`（`v-html="md(post.content)"`）、`CommentTree.vue:81`（`v-html="md(c.content)"`） | `PostDetail.vue:355` / `CommentTree.vue:81` |
| **用户主页路由 `/user/:id`（数字 ID）**；帖子详情 `/post/:id` | `router/index.ts:15,13` |
| `Post` 接口已有 `tags?: string[]`；`CommentNode` 无 mentions | `api/types.ts:41-59,63-74` |
| `NotificationType` 联合含 6 值，**无 `mention`** | `api/types.ts:125` |
| `AppNotification`：type/actor/target_* 字段 | `api/types.ts:127-137` |
| `Notifications.vue`：`textFor` 文案 / `linkTo` 锚点（like/comment/reply → `/post/:id`）/ `onItemClick` 点击+已读 | `Notifications.vue:94-113,123-139,141-154` |

### 2.4 GitHub 状态

- ✅ **本地实查**：`git log --oneline -1` = `bac163f`（main，工作树干净）——与 handoff 记录的 main HEAD 一致。
- ✅ issue #72 已存在（handoff 记录；本设计文档产出时未走 `gh` CLI，issue 状态标 **待复核**）。
- 开放 PR = 0（handoff 记录，待复核）。

---

## §3 Grilling 决策记录（7 项，用户确认）

| 编号 | 决策问题 | 定案 | 依据 |
|---|---|---|---|
| D1 | 编辑帖子/评论时是否重新解析 mentions | **MVP 不做**（编辑只改正文，不重触发通知/不更新 mentions 表） | 用户确认（2026-08-26/27）；handoff §5.1「可能做」项收敛 |
| D2 | 用户名大小写规则 | **按 DB 实际大小写字面匹配**（`@Alice` 只匹配 `alice` 或 `Alice` 中实际存在的那个；无大小写归一化） | 用户确认（2026-08-26/27）；现状 schema 无 CITEXT/大小写不敏感唯一（§2.3）。**白话补充（评审 Q6）**：`@Alice` 而真实用户是 `alice` 时**无人被通知、无链接**——这是 D2 字面匹配的固有成本，非 bug |
| D3 | 不存在的用户名渲染策略 | **保持纯文本，不转链接** | 用户确认（2026-08-26/27）；handoff 目标 3 |
| D4 | 同内容多次 @ 同一人 / 自己 @ 自己 | **去重：只发一条通知；自我过滤：不通知自己** | 用户确认（2026-08-26/27）；handoff §5.3「可能做」 |
| D5 | `@` 可识别的用户名语法 | **`@` + 1–30 个中文/字母/数字/下划线**（与 hashtag 规则一致，安全保守） | 用户确认（2026-08-26/27）；`users.username` 实为 VARCHAR(64)，但 30 上限与现有 tag 规则统一 |
| D6 | 帖子/评论软删时是否清理 mentions 关联 | **清理**（与 post_tags 卫生一致） | 用户确认（2026-08-26/27） |
| D7 | 通知点击锚点 / 用户主页路由 | **通知点击跳帖子详情 `/post/:id`；用户主页保持 `/user/:id`（数字 ID）** | 用户确认（2026-08-26/27）；`router/index.ts:13,15` 现状 |

---

## §4 范围收敛与明确不做

### 必须做（MVP）

- 帖子正文（`posts.content`）与评论正文（`comments.content`）的 `@username` 解析、落库、通知、渲染。
- 通知中心新增 `mention` 类型（复用 `notify.notifications` 基础设施）。
- 前端渲染：`@username` → 用户主页链接（仅后端下发的有效 mentions 列表）。
- 新增迁移：`content.mentions` 表 + `notify` 类型 CHECK 追加 `mention`。

### 明确不做（本次范围）

| 项 | 决策 | 依据 |
|---|---|---|
| 编辑重解析/撤销通知 | 不做 | D1 |
| 实时推送（WebSocket） | 不做，继续轮询/应用内通知中心 | handoff §2；#32 决策 |
| `@所有人` / `@板块管理员` 等群组提及 | 不做 | handoff §2 |
| 富文本编辑器内 @ 下拉提示 | 不做，纯文本解析 | handoff §2 |
| 用户主页路由改 `/user/:username` | 不做，保持 `/user/:id` | D7 |
| 标签系统改为大小写不敏感唯一 | 不做 | D2（保持现状） |
| 手机注册/OAuth/SSO | 不做 | handoff §2 |

### 与既有功能的关系

- 本票与「编辑功能」（当前**无** `UpdatePost`/`UpdateComment`，`service.go` 接口无更新方法）正交：MVP 无编辑，故 D1 成立无成本。若未来加编辑，须扩展 `ReplaceMentions`（见 §9 Q1）。

---

## §5 实现方案（每项给依据）

### 5.1 存储：一张多态关联表 `content.mentions` + 迁移 0011/0012

```sql
-- 0011_content_mentions.sql
CREATE TABLE content.mentions (
  id BIGSERIAL PRIMARY KEY,
  target_type VARCHAR(16) NOT NULL CHECK (target_type IN ('post','comment')),
  target_id BIGINT NOT NULL,                 -- 逻辑指向 content.posts/comments（同 schema 内联）
  mentioned_user_id BIGINT NOT NULL,         -- 逻辑FK → user.users(id)，只建索引不建约束
  mentioned_username VARCHAR(64) NOT NULL,   -- 创建时快照（对齐 users.username 长度）
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (target_type, target_id, mentioned_user_id)
);
CREATE INDEX idx_mentions_target ON content.mentions (target_type, target_id);
CREATE INDEX idx_mentions_user ON content.mentions (mentioned_user_id);
```

```sql
-- 0012_notify_type_mention.sql（照 0008 写法，见 §2.2）
ALTER TABLE notify.notifications DROP CONSTRAINT IF EXISTS notifications_type_check;
ALTER TABLE notify.notifications ADD CONSTRAINT notifications_type_check
  CHECK (type IN ('follow','like','comment','reply','report_result','report_handled','mention'));
```

- **依据**：多态 `(target_type, target_id)` 复用 `content.likes` 既有模式（`0002_content_schema.sql:59-67`，target_type CHECK IN ('post','comment')）；逻辑 FK 只建索引遵守 FK 政策（`0001_user_schema.sql:2` 注释）；迁移编号已对本地目录核对无冲突（§2.2）。

### 5.2 解析：`backend/content/mentions.go`（新建）

- 从 `tags.go` 抽取共享剥离逻辑为 `stripMarkdownForMatching(s)`（围栏代码→行内码→图→链接→裸 URL，各替换为单空格），`ParseTags` 与 `ParseMentions` 共用。
- **依据**：剥离顺序与前端 `md.ts:44-58` 一致（#54 评审 F1 已实证 Go/JS 语义一致，`tags.go:8-15`）。
- `ParseMentions(content string) []string`：在剥离后文本上匹配 `(^|[^\p{L}\p{N}_@])@([\p{L}\p{N}_]{1,30})`，去重、保首次出现序，**保留原文大小写**（不归一化，D2）。
- **依据**：边界 `[^\p{L}\p{N}_@]` 排除 `@@` 与 `a@b.com` 的 `@` 前字符为字母的情况（邮箱域名不会误识别——`b` 是字母非边界）；长度 30 对齐 D5；`tagRE` 同构 `tags.go:17`。

### 5.3 content 域改造（`backend/content/`）

1. **领域模型与 DTO 分区（评审 F2 修正）**：
   - `model.go`：新增 `Mention` GORM 模型（映射 `content.mentions`），**必须含 `func (Mention) TableName() string { return "content.mentions" }`**（照 `Tag`/`PostTag` 先例 `model.go:95,105`，评审 F10）。
   - `service.go` DTO 区：新增 `MentionView{UserID int64 json:"user_id"; Username string json:"username"}`；`PostView`（`service.go:23-43`）加 `Mentions []MentionView json:"mentions,omitempty"`、`CommentView`（`service.go:64-74`）加同字段。
   - **依据**：`PostView.Tags` 先例在 `service.go:29`；`omitempty` 保持「无则省略」契约；`Mentions` 若加在 `model.go` 的 `Post`/`Comment` 上会污染 GORM 表映射（评审 F2 提醒）。
2. **`repo.go` + `gorm_repo.go` + `fake_repo_test.go`**：新增
   - `CreateMentions(ctx, mentions []*Mention) error`（批量插入，`ON CONFLICT DO NOTHING` 幂等）。
   - `ListMentionsByTargets(ctx, targetType string, targetIDs []int64) (map[int64][]*Mention, error)`（批量富化用，照 `ListTagsByPostIDs` `gorm_repo.go:468`）。
   - `DeleteMentionsByTarget(ctx, targetType string, targetID int64) error`（软删清理，照 `DeletePost` 硬删 post_tags `gorm_repo.go:140-141`）。
   - **`DeleteComment` 改造为事务内软删 + 清 mentions**（评审 D6 附带）：现为单语句纯软删（`gorm_repo.go:203-205`），须改为照 `DeletePost` 的事务形态（`gorm_repo.go:135-143`）——`tx.Delete(&Comment{}, id)` + `tx.Where(...).Delete(&Mention{})` 同一事务。
   - 依据：接口形态照 `repo.go:75-77` tags 方法；fake 同步实现（`fake_repo_test.go:570-610` 照抄）。
3. **`service.go`**：
   - `UserProvider` 加 `GetUserByUsername(ctx, username string) (UserView, error)`（只返回 active 用户，见 §5.6）。
   - 新增 seam：`MentionNotificationCmd{RecipientID, ActorID, ActorName, ActorAvatar, TargetType string, TargetID, TargetTitle}` + `Notifier interface { NotifyMention(ctx, in MentionNotificationCmd) error }`（照 `moderation.Notifier` `moderation/service.go:138-152`）。
   - `NewService(repo, users, notifier)` 三参；struct 加 `notify Notifier`。
   - `CreatePost`：`repo.CreatePost` 后调用 `processMentions(ctx, authorID, content, "post", postID, postTitle)`。
   - `CreateComment`：`repo.CreateComment` 后调用 `processMentions(..., "comment", commentID, postTitle)`——当前 `CreateComment` 未保留 fetch 到的 post（`service.go:311`），需存下 `post.Title` 供通知标题。
   - `processMentions` 流程（评审 F1/F3 补齐）：
     - **actor 快照来源（F1）**：先一次 `s.users.GetUserView(ctx, authorID)` 取 author 的 `Username`/`AvatarURL`，填充全部 `MentionNotificationCmd` 的 ActorName/ActorAvatar。`CreatePostCmd`/`CreateCommentCmd` 仅含 AuthorID（`service.go:14-20,57-62`），快照字段必须现查；该结果可与 `postView` 的作者 fetch 复用（实现时在 `CreatePost`/`CreateComment` 提前 fetch 一次并传递）。
     - `ParseMentions` → 逐 username 调 `users.GetUserByUsername`：不存在/非 active → 跳过；**非 not-found DB 错误 → `slog.Warn` + 跳过该 username，不阻断**（F3，与 §6.4 best-effort 一致）。
     - 过滤 self → 按 `mentioned_user_id` 去重 → `repo.CreateMentions`（失败 `slog.Warn`）→ 逐条 `notify.NotifyMention`（失败 **必须 `slog.Warn`**——不复制 `follow.go:78` 无日志的旧瑕疵，评审 §6.4 附带）。
   - `DeletePost`/`DeleteComment`：追加 `repo.DeleteMentionsByTarget` 清理（D6）。
   - `postViews` 批量富化 `mentionsByPost`（照 tags 富化 `service.go:822-826`）；`buildTree`/`commentView` 批量富化评论 mentions。
   - 依据：best-effort 语义照 #54 先例 `service.go:298-303` + `follow.go:78`（吞错）；去重/自我过滤为 D4。
4. **`service_test.go` / `fake_repo_test.go`（评审 F5 定案）**：`newContentService()` 有 **47 处调用点**（评审实测，§2.3），**不得改双返回值**——新增 `newServiceWith(...)` 默认注入**非 nil** `fakeNotifier`（包级记录器或结构体字段），`newContentService()` 保持 `(*fakeRepo, Service)` 双返回值不变（**47 处零改动**）；新增测试构造/访问器暴露记录器断言；`fakeUsers` 加 `GetUserByUsername` + 注入位 `errByName map[string]error`（使「封禁/软删跳过」可测，评审 F5）。

### 5.4 notify 域（`backend/notify/`）

- `service.go:119-121` `validTypes` 加 `"mention": true`；`model.go:11` 注释更新类型清单。
- **依据**：`CreateNotification` 已通用（快照自足，#4 D4），无 repo 改动。

### 5.5 user 域（`backend/user/`）

- `Service` 加 `GetUserByUsername(ctx, username string) (UserView, error)`：调已有 `repo.GetUserByUsername`（`gorm_repo.go:27-33`）→ **`IsActive` 过滤**（存在且 active 才返回，复用 `service.go:371`）→ `toView` 映射。**评审 F9**：active 过滤下沉到此，adapter 对每个 mention 单次调用。
- **依据**：repo 方法已存在（§2.3），仅缺 Service 暴露；`toView` 复用（`service.go:383-389` GetUser 同款）；IsActive 已对软删/不存在返回 false（`service.go:374-376`）。

### 5.6 httpapi / 装配

- `router.go:22-45` `userProviderAdapter` 加 `GetUserByUsername`：调 `userSvc.GetUserByUsername`（§5.5，已含 active 过滤）→ 映射 `content.UserView`。**单次调用**（评审 F9）。
  - **依据**：D2/D3 语义（不存在/封禁/软删一律不落库不通知）；active 过滤已下沉 service 层（§5.5/§6.6）。
- 新建 `backend/internal/httpapi/content_adapter.go`：`mentionNotifierAdapter` 翻译 `content.MentionNotificationCmd` → `notify.CreateNotificationCmd`（照 `moderation_adapter.go:126-148`）。
- `main.go`：调整装配顺序 `notifySvc` 提前到 `contentSvc` 之前；`content.NewService(repo, NewUserProvider(userSvc), NewContentNotifier(notifySvc))`。
- `auth_test.go`：**同步 `content.NewService` 新签名 + notifySvc 同样提前到 contentSvc 之前**（评审 F4：`newEngineWithUploads` 中 notifySvc 在 `:38` 晚于 contentSvc `:36`——只改签名会缺 Notifier 依赖）。

### 5.7 前端

- **`md.ts`**：`md(src: string, mentions: MentionUser[] = [])`；`MENTION_RE = /(^|[^\p{L}\p{N}_@])@([\p{L}\p{N}_]{1,30})/gu`；在标签替换之后（`md.ts:61-63`）、粗体之前加入替换——仅当 `username` 命中 `mentions` 列表时才 `pushBlock(<a class="mention" href="#/user/${user_id}">@${username}</a>)`，未命中保持纯文本（D3）。
  - **依据**：占位符机制防二次处理（`md.ts:38-41`）；放粗体前使 `**@user**` 正确包 `strong`（`md.ts:64-65`）；放 URL 规则后防 `https://x.com/@user` 误伤。
  - **契约差异提示**：`md` 现有调用点传 1 参（§2.3 仅 2 处），加可选第二参向后兼容，既有调用不变。
- **`api/types.ts`**：`MentionUser{user_id:number; username:string}`；`Post`/`CommentNode` 加 `mentions?: MentionUser[]`；`NotificationType` 加 `'mention'`。
- **`PostDetail.vue:355` / `CommentTree.vue:81`**：`md(content)` → `md(content, mentions)`，各节点/回复透传 `c.mentions`。
- **`Notifications.vue`**：`textFor` 加 `case 'mention': return \`${who}在「${n.target_title||'帖子'}」中提到了你\``；`linkTo` 加 `case 'mention': return n.target_id != null ? \`/post/${n.target_id}\` : null`（**补 null 守卫**，与其他 case 一致，评审 F7；D7）。
  - **依据**：`textFor`/`linkTo` 结构已核（§2.3）；mention 的 TargetType/TargetID 由 5.6 通知命令填充（帖子/评论均指向帖子详情）。

---

## §6 关键设计裁决（【决策】，含理由与备选）

### 6.1 存储形态：一张多态表 vs 两张表（post_mentions/comment_mentions）

- **定案【决策】**：一张 `content.mentions`，`(target_type, target_id)` 多态。
- **理由**：`content.likes` 已用同一多态模式（`0002_content_schema.sql:59-67`）；避免两套几乎相同的 GORM/fake 代码；目标恒在 content schema 内，无跨包歧义。
- **备选（不选）**：`post_mentions` + `comment_mentions` 两表——类型更明确，但重复代码、查询/维护成本翻倍，MVP 收益低。

### 6.2 `mentioned_username` 快照列

- **定案【决策】**：`content.mentions` 存 `mentioned_username` 快照（创建时写入，对齐 `VARCHAR(64)`）。
- **理由**：读侧富化 mentions 列表时零 join 即可给出 username（前端渲染只需 user_id+username）；用户改名/软删不影响已落库提及的显示一致性。
- **备选（不选）**：只存 `mentioned_user_id`，读时 join `user.users`——省一列，但富化多一次 join，且用户名可能已变（软删用户 `GetUserView` 返回"已注销"）。

### 6.3 前端渲染由后端 mentions 列表驱动 vs 前端正则全部转链接

- **定案【决策】**：前端只把后端下发的 `mentions` 列表内 username 转链接；未命中保持纯文本。
- **理由**：D3（不存在用户纯文本）由「后端只落库有效用户」自然实现，前端无需知道全量用户表；路由是 `/user/:id`（数字），username→id 映射只能由后端提供。
- **备选（不选）**：前端正则把全部 `@xx` 转链接到 `/user/:xx`——路由是数字 id，`/user/:username` 会 404；且不存在用户无法区分。**被 D7 否决**。

### 6.4 mentions 落库/通知失败语义：best-effort

- **定案【决策】**：mentions 写入与通知发送均 best-effort——失败仅 `slog.Warn`，**不阻断**帖子/评论创建。**通知失败同样必须 `slog.Warn` 留痕**（评审 §6.4 附带：不复制 `follow.go:78` 无日志的旧瑕疵——#33 评审已有此教训）。
- **理由**：照 #54 标签先例（`service.go:298-303`）+ follow 通知先例（`follow.go:78` 吞错）；提及是派生元数据，不能让发帖失败。
- **备选（不选）**：mentions 失败回滚发帖——过度耦合，派生数据不应影响主命令原子性。

### 6.5 评论 mention 通知锚点：统一跳帖子详情

- **定案【决策】**：评论里的 mention，通知 `TargetType="post"`、`TargetID=post_id`、`TargetTitle=帖子标题`，点击跳 `/post/:id`（不定位到具体评论）。
- **理由**：D7 已定通知跳帖子详情；现有 `Notification` 无评论锚点字段，评论级锚点需新增 target 语义，MVP 不做。
- **备选（不选）**：`TargetType="comment"` + 前端锚点定位——需前端 `revealComment` 联动（#47 已有滚动定位能力，但通知点击链路未接入），MVP 收益低，留待后续。**登记为已知缺口（§7 #9）**。

### 6.6 `GetUserByUsername` 只返回 active 用户

- **定案【决策】**：`UserProvider.GetUserByUsername` 语义 =「存在且 active」；封禁/软删/不存在一律按不存在处理（跳过落库与通知）。
- **理由**：D2/D3 统一策略（不通知、不渲染链接）；`IsActive` 已对软删/不存在返回 false（`user/service.go:374-376`），复用即可。
- **备选（不选）**：区分「存在但封禁」（落库不通知）——会渲染指向封禁用户的链接，与 D3「被删除用户链接行为一致」冲突；MVP 简单一致优先。

---

## §7 边界与不变量清单

| # | 不变量 | 防护层 | 依据 |
|---|---|---|---|
| 1 | 提及只在创建时解析，编辑不重算 | MVP 无 `UpdatePost/Comment` 命令 | D1（§3）；`service.go` 接口无更新方法（§2.3） |
| 2 | 同内容多次 @ 同一人 → mentions 一条 + 通知一条 | `processMentions` 按 `mentioned_user_id` 去重；表 UNIQUE `(target_type,target_id,mentioned_user_id)` | D4；§5.3 |
| 3 | 自己 @ 自己不通知 | `processMentions` 过滤 `u.ID == authorID` | D4；§5.3 |
| 4 | 不存在/封禁/软删用户不落库不通知 | `GetUserByUsername` 只返回 active（IsActive 过滤） | D2/D3；§5.6/§6.6 |
| 5 | mentions 写失败/通知失败不阻断发帖 | `slog.Warn` + 继续（best-effort） | §6.4；`service.go:298-303` |
| 6 | 帖子/评论软删后 mentions 清理 | `DeletePost`/`DeleteComment` 事务内 `DeleteMentionsByTarget` | D6；`gorm_repo.go:140-141` 先例 |
| 7 | 前端只渲染后端下发的有效 mentions，其余 `@text` 纯文本 | `md()` 仅在 `mentions` 列表命中时转链接 | D3；§5.7/§6.3 |
| 8 | 跨包逻辑 FK，无物理约束 | `content.mentions.mentioned_user_id` 只建索引 | FK 政策；§5.1 |
| 9 | 评论 mention 通知不定位评论（已知缺口，MVP 接受） | 通知锚点统一帖子详情；实现时文档化 | §6.5 备选；D7 |
| 9a | `<@alice`/`>@alice` 残余漂移：后端通知已发但前端不渲染链接（esc-first 固有边界，与 #54 同类） | **接受** + 一条 `md.test.ts` 断言 + 本文档化（评审 F6/Q3） | §6.3/Q3；`md.ts:5-11` esc-first |
| 9b | feed 卡片/列表摘要不渲染 mention 链接（`plainText` 不经 `md()`） | **MVP 接受** + 文档化（评审 F8） | `md.ts:103-112` plainText |
| 10 | 通知中心运行时三态（推演） | 见下 | — |

**通知中心运行时三态推演（§7 #10 展开）**：
- **① 页面停留期间数据变化**：`Notifications.vue` 拉取后无轮询（现状 `load` 仅在挂载/滚动/切 tab 触发，`Notifications.vue:30-48`）。mention 通知到达后，停留页面的用户 badge（Tab 未读数）由 `notifyStore.refreshUnread()` 在挂载时刷新一次（`Notifications.vue:82`）；**新通知不会实时出现在已停留的列表**。与 follow 通知现状一致，属 #32 已知轮询现状，**MVP 接受**——本票不改变该行为。
- **② 挂载时序竞态**：`onItemClick` 先本地置已读再后台确认（`Notifications.vue:145-153`）——与 mention 通知无新增竞态（沿用现有通知读取链路）。无「挂载即已读」逻辑。
- **③ 退出再进**：重新 `load(true)` 拉取，mention 通知正常显示；无持久化状态依赖。
- **结论**：本票不引入新的运行时交互状态；通知中心实时性缺口语作为已知缺口登记（承接 #32 决策，不做 WebSocket）。

---

## §8 测试与验证计划

### 后端单测

| 文件 | 用例 |
|---|---|
| `backend/content/mentions_test.go`（新） | `ParseMentions`：边界（行首/空白/标点/`@@`/`a@b.com`）；代码/URL/链接内不识别；长度 >30 截断；去重保序；大小写保留 |
| `backend/content/service_test.go` | `CreatePost`/`CreateComment` 带 mentions：mentions 落库；去重；自我过滤；不存在/封禁跳过；写失败不阻断；`DeletePost`/`DeleteComment` 清理 mentions |
| `backend/notify/service_test.go` | `mention` 是合法类型（`TestCreateNotificationMention` 或等价） |
| `backend/user/service_test.go` | `GetUserByUsername` 暴露正确、映射 UserView |

### 后端集成（真 PG，`backend/internal/testutil/db.go`）

- `backend/internal/httpapi/handler/content_test.go`（或新增）：注册用户 A/B → A 发帖/评论含 `@B` → 断言 `content.mentions` 行 + `notify.notifications` type=mention 且 recipient=B；`@nonexistent` 与 self 不产生行/通知。

### 前端单测

- `frontend/src/utils/md.test.ts`：命中 mentions 渲染 `<a class="mention" href="#/user/{id}">`；未命中保持纯文本；代码/URL/邮箱不误识别；**`<@alice`/`>@alice` 不渲染链接**（评审 F6/Q3 附带）；既有 hashtag 用例回归。
- `frontend/src/views/PostDetail.test.ts` / `CommentTree.test.ts`：透传 mentions 后链接渲染。
- `frontend/src/views/Notifications.test.ts`：mention 文案 + 点击跳 `/post/:id`。

### 验证命令（本地全量）

```bash
cd backend && go build ./... && go vet ./... && go test ./...
cd frontend && npm run test:unit && npm run build
```

### 上线验证（部署后，按 SOP 走服务器只读通道——待复核项在此落实）

- 服务器 `git log --oneline -1` 与 PR HEAD 一致、容器 healthy、`/healthz` 200。
- 线上 DB：`content.mentions` 表存在；`notifications` CHECK 含 `mention`（psql 实查）。
- 冒烟：A 发帖 `@B` → B 收到 mention 通知；正文渲染链接；`@nonexistent` 纯文本。

---

## §9 待评审焦点（Q1-QN）

> **已评审裁决（2026-08-27，独立评审）**：Q1-Q9 全部认可（Q3 附 1 条件、Q8 附 F4 条件、Q6 附白话建议）。裁决详情见评审意见书 §四；落地见 §10。

- **Q1（存储形态）**：一张多态 `content.mentions` vs 两张表。已定【决策】一张（§6.1），请评审确认多态 CHECK `(post,comment)` 与 `content.likes` 先例的一致性、以及未来加「编辑重解析」时 `ReplaceMentions` 的可扩展性（D1 前提下 MVP 无 Replace，此接缝是否够）。
- **Q2（快照列）**：`mentioned_username` 快照是否必要？备选只存 id + 读时 join（§6.2）。请评审确认快照带来的「用户名已变但提及显示旧名」语义是否可接受。
- **Q3（前端渲染耦合）**：前端依赖后端下发的 `mentions` 列表决定哪些 `@text` 转链接。若后端解析与前端正则出现漂移（#54 已用同组单测钉行为），漏渲染风险如何兜底？请评审确认「测试钉行为」是否足够。
- **Q4（失败语义）**：mentions 落库/通知 best-effort 不阻断发帖（§6.4）。这是「通知可能丢」的权衡——评审是否认可 MVP 下 acceptable？
- **Q5（评论通知锚点）**：评论 mention 通知统一跳帖子详情，不定位到评论（§6.5 备选登记）。是否应纳入 MVP 的 `revealComment` 联动（#47 已有滚动定位能力）？——**本设计倾向不做，请评审把关**。
- **Q6（大小写语义）**：D2 字面匹配。`@alice` 与 `@Alice` 视为两个候选，各查一次 `GetUserByUsername`；同一人大小写不同写法会落两条 mentions？——**不**：去重按 `mentioned_user_id`（§7 #2），但若两个用户名都真实存在（`Alice`/`alice` 不同用户）会通知两人。请评审确认此语义边界。
- **Q7（未复核项）**：服务器 healthz / 线上 DB CHECK 现状 / issue #72 状态 / 开放 PR 数**均未复核（待复核）**——评审方如有通道可补核；实现阶段按 SOP 落实（§8 上线验证）。
- **Q8（装配顺序调整）**：`main.go` 需把 `notifySvc` 提前到 `contentSvc` 前（§5.6）。这是组合根重构，波及 `auth_test.go` 构造。请评审确认无其他 `content.NewService` 调用点遗漏（grep 已命中 2 处，§2.3）。
- **Q9（评论 title 获取）**：`CreateComment` 需保留 fetch 到的 post 以取标题（§5.3），当前 `service.go:311` 丢弃。这是对现有逻辑的小改动，请评审确认无副作用（如浏览计数的 GetPost 副作用规避——应复用 `GetPostMeta` 语义而非 `GetPost`）。

---

## §10 评审意见采纳记录（2026-08-27）

| 评审项 | 结论 | 采纳落地 |
|---|---|---|
| **F1** actor 快照来源未指定（阻塞） | 属实 | §5.3：`processMentions` 先一次 `GetUserView(ctx, authorID)` 取 Username/AvatarURL 填全部 `MentionNotificationCmd` |
| **F2** `PostView`/`CommentView` 引用错位 | 属实 | §2.3/§5.3.1 修正为 `service.go:23-43/64-74`；Mention 领域模型在 `model.go`、MentionView/字段在 `service.go` DTO 区 |
| **F3** `GetUserByUsername` 错误分支未定 | 属实 | §5.3：非 not-found 错误 → `slog.Warn` + 跳过，不阻断 |
| **F4** `auth_test.go` 装配顺序 | 属实 | §5.6：auth_test notifySvc 提前到 contentSvc 前（与 main.go 同改） |
| **F5** 测试辅助改造波及 47 处 | 属实 | §5.3.4：`newServiceWith` 默认注入非 nil `fakeNotifier`，`newContentService()` 双返回值不变（47 处零改动）；`fakeUsers` 加 `errByName` 注入位 |
| **F6 / Q3 附带** `<@`/`>@` 残余漂移 | 属实 | §7 #9a + §8 `md.test.ts` 断言 |
| **D6 附带** DeleteComment 事务化 | 属实 | §5.3.2：DeleteComment 改事务内软删 + 清 mentions |
| **§6.4 附带** 通知失败必须 Warn | 属实 | §6.4：不复制 `follow.go` 无日志旧瑕疵 |
| **F7** linkTo null 守卫 | 建议 | §5.7：`n.target_id != null ? ... : null` |
| **F8** 卡片摘要不渲染 | 建议 | §7 #9b 文档化 |
| **F9** 每 mention 两次 user 域调用 | 建议 | §5.5：active 过滤下沉 `user.Service.GetUserByUsername`，adapter 单次调用 |
| **F10** `Mention` 缺 `TableName()` | 建议 | §5.3.1：`func (Mention) TableName() string { return "content.mentions" }` 照惯例 |

**推翻项**：无。全部评审发现经独立复核属实；评审方未复核的 SSH/线上 DB 项在实现阶段按 SOP 落实（§8 上线验证）。

---

*本文档为实现基线。评审通过后：进入实现（分支→实现→测试→PR→用户授权合并→部署→服务器验证→回报统筹方更新地图 #1 + 关闭 #72），并将本文档落档到仓库 `docs/plans/72-mentions-设计文档.md`。*
