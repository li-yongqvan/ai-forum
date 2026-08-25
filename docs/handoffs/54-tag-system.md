# Handoff · #54 标签系统（#hashtag）：正文内嵌标签 + 聚合页

> 生成：2026-08-24 · 统筹会话（Claude Code）。
> **自包含交接**：给**零上下文**的下一会话读，读完即可上手。若与代码真值冲突，**以真值为准**。
> 前置：`docs/workflow.md`（SOP，**新会话必读**）；地图 issue #1；并行会话 **#43** + **#53**（见 §4）。

---

## 0. 零上下文启动块（新会话必读）

**这是什么**：AI 智联论坛——Go（Gin+GORM+PostgreSQL）后端 + Vue3+TS+Vant（PWA）前端。代码本机 `C:\Users\liyongquan\ai-forum`；线上 `http://122.51.233.225:8888/`。服务器阿里云、用户 `liyongquan`（**无 sudo**）、repo `~/ai-forum`。无 CLAUDE.md，本文件 + `docs/workflow.md` + issue #1 即上下文。

**新会话前 5 分钟**：
1. 读 `docs/workflow.md`（SOP）+ 本文件 + GitHub issue **#54**（本票正文）+ issue **#1**。
2. 连服务器核真值：`ssh -p 2222 liyongquan@122.51.233.225 'cd ~/ai-forum && git log --oneline -1 && docker compose ps && curl -s http://localhost:8888/healthz'`。
3. **探明 content 域**（§2 未知项）+ **先做第 0 步 grilling（§3）**（SOP §3）。

**命令速查**：gh 一律 `MSYS_NO_PATHCONV=1`（重试 ≤4）；本地 `cd backend && go build ./... && go vet ./... && go test ./...`；`cd frontend && npm run test:unit && npm run build`；生产 DB `ssh -p 2222 ... 'cd ~/ai-forum && docker compose exec -T postgres psql -U forum forum'`（schema `"content"`）。**服务器 SSH 端口 = 2222**。

**git 现状（2026-08-24）**：`main` = `0301601`。动手前 `git fetch origin && git checkout main && git pull origin main`。

**协作流水线**：分支 → 实现 → 本地测试全绿 → commit → push → `gh pr create` → **用户授权** → `gh pr merge N --merge` → CI 部署 → 服务器验证 → **地图更新交给统筹方**。

---

## 1. Goal of next session

**实现 #54 标签系统（#hashtag）**：发帖正文里 `#关键词` 自动识别为**可点击标签**，点标签进入**聚合页**（该标签下帖子列表）。

> 方向已由用户选定（微博式 #hashtag）。**落库 + 聚合页**为初拟（详见 §3 grilling 决策 1）。

## 2. State of play（部分已探明，部分待取票会话确认）

**已探明**：
- **content 域**：板块（board）/话题（topic）两级已有；`content.posts` 表；读模型组装（`postViews`，`backend/content/service.go`）。
- **markdown 渲染**：`frontend/src/utils/md.ts`（图片已加 `content-img` class，可参考其模式给标签加 class + 事件委托）。
- **组件复用**：`PostList.vue`（fetcher + emptyTitle 通用列表）、`SegTabs.vue`。

**待取票会话探明**：
- content 包 Repo/Service 接口结构（`repo.go`/`service.go`/`gorm_repo.go`），决定标签表/查询怎么加。
- 现有「发帖/看帖」链路（`Write.vue` 发帖、`PostDetail.vue` 渲染），标签解析插入点。
- 数据库迁移机制（`backend/migrations/000N_*.sql`，embed 自动跑）——若落库需新迁移。

## 3. Open decisions（第 0 步 grilling，本会话与用户定）

1. **落库 vs 纯展示**：聚合页需要真实数据 → 倾向**落库**（新建 `tags` 表 + 帖标签关联，如 `content.tags` + `post_tags`，可聚合/计数）；grilling 确认。
2. **标签规则**：`#` 后字符集（中文/字母/数字？）；长度上限；是否大小写不敏感；`#` 需前后空白/边界。
3. **聚合页形态**：路由（如 `/tag/:name`）+ `PostList` 复用；排序（最新/热度？）；空态。
4. **与话题（topic）关系**：互补（板块/话题是组织单位，标签是内容维度）——确认不冲突。
5. **解析时机**：发帖时落库解析（存关联）vs 渲染时实时高亮（纯展示部分）。

## 4. 协调

- **并行**：#43（CI/服务器）、#53（notify/moderation）与本票（content 域）**零文件重叠**，可并行。**worktree 隔离**必须用。
- **红线**：本票动 `backend/content` + 迁移 + 前端（`Write.vue`/`PostDetail.vue`/md.ts/新聚合页）；**不要碰** notify/moderation、`.github/workflows/`。
- **地图 #1 只由统筹方更新**——完成后回报统筹方（SOP §10）。

## 5. Skills to use

- `grilling`：§3 第 0 步（HITL）。
- `domain-modeling` / `codebase-design`：标签是内容域新概念，先对齐边界（深模块）。
- `tdd`：解析函数单测 + 聚合查询测试 + 前端组件测试。
- `code-review`：合并前自审。

## 6. Artifacts

- **ticket**：#54（本票正文，含背景/grilling/范围）。
- **SOP**：`docs/workflow.md`（§5 后端装订 / §6 前端装订 / §7 测试 / §8 PR / §10 地图）。
- **代码**：`backend/content/{repo,service,gorm_repo,model}.go`、`backend/migrations/`、`frontend/src/utils/md.ts`、`frontend/src/views/Write.vue`、`PostDetail.vue`、`frontend/src/api/`。
- **参考**：`docs/forum-wayfinder-input.md`（完整清单"标签"项）；`docs/impl/testing.md`；`docs/ops/lessons-8888-deploy.md`。

## 7. 安全注意

- 无 token/密码。gh 命令 `MSYS_NO_PATHCONV=1`。服务器 `.env` 勿外泄/进 git。

---

*handoff 为当票工作底稿（非持久化）；完成后由 INDEX + issue 记录替代。*
