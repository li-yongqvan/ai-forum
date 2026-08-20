# Handoff · #23 收藏列表（post-launch 首批）

> 生成：2026-08-20 · 交接会话（Claude Code），本会话完成了 #26 整个 issue。
> **本文件是自包含交接**：给**零上下文**的下一会话读，读完即可上手，不依赖上一会话记忆。若内容与服务器/DB/GitHub 真值冲突，**以真值为准**。
> 前置：`docs/workflow.md`（功能开发 SOP，**新会话必读**）；地图 issue #1（决策日志 15-17）。

---

## 0. 零上下文启动块（新会话必读）

**这是什么**：AI 智联论坛——面向学院师生的 AI 主题社区。技术栈：**Go（Gin + GORM + PostgreSQL）后端 + Vue3 + TS + Vant（PWA）前端**。代码本机 `C:\Users\liyongquan\ai-forum`；线上 `http://122.51.233.225:8888/`（容器 nginx@8888 → api → postgres）。服务器阿里云、用户 `liyongquan`（**无 sudo**）、repo `~/ai-forum`。仓库**无 CLAUDE.md**，本文件 + `docs/workflow.md` + GitHub issue #1 就是你的上下文。

**新会话前 5 分钟**：
1. 读 `docs/workflow.md`（功能开发 SOP）+ 本文件 + GitHub issue **#1**（决策日志 15-17 / Roadmap）。
2. 连服务器核对真值：`ssh liyongquan@122.51.233.225 'cd ~/ai-forum && git log --oneline -1 && docker compose ps && curl -s http://localhost:8888/healthz'`。
3. 向用户确认 §3 待决项。

**访问 / 命令速查**：
- 服务器：`ssh liyongquan@122.51.233.225`
- 生产 DB：`ssh ... 'cd ~/ai-forum && docker compose exec -T postgres psql -U forum forum'`，schema `"user"/"content"/"notify"/"moderation"`，db/user 均 `forum`
- 本地工具链：Go 1.25.0（`backend/go.mod`）/ Node 24 / Docker Desktop / `gh`（**gh 一律前缀 `MSYS_NO_PATHCONV=1`**，失败重试 ≤4、换方式验证）
- 本地测试：`cd backend && go build ./... && go vet ./... && go test ./...`；`cd frontend && npm run test:unit && npm run build`
- workflow 校验：`bash scripts/check-workflows.sh`
- 线上健康：`curl http://122.51.233.225:8888/healthz`、`/api/v1/posts`

**git 当前状态（2026-08-20 实测）**：
- `origin/main` 最新 = **PR #28**（`623e64a`，#26 A' 部署落地）；PR #27 已合入；**PR #29 待合并**（验收线 docs，见 §3）。
- 动手前 `git fetch origin && git checkout main && git pull origin main`。

**协作流水线（每次改动都走）**：
`git checkout -b <type>/<slug>` → 实现 → 本地测试全绿 → conventional commit → push → `gh pr create` → **用户明确授权**后 `gh pr merge N --merge` → CI 自动部署 → **服务器侧验证真值** → 更新 issue #1。前端单测只在本地跑（未进 CI）。

---

## 1. Goal of next session

**主目标：实现 #23 收藏列表（8/24 交付窗口，稳定优先）。** 产出：
- 用户「我的」页加 **SegTabs「帖子 / 收藏」**，收藏 tab 显示**我收藏的帖子列表**（卡片式、可取消收藏、空态、分页）。
- **范围已定**：仅收藏。关注列表**拆为独立 ticket #27**（用户/板块/话题三子列表 + 新列表组件，8/24 后做），本票不碰。
- 按路线 A 实现（见 §2），8/24 前**求稳**，改动最小。

**Prompts to answer：**
1. 当前线上/代码/数据状态？→ 读本文件 + workflow.md + issue #1 + 服务器真值。
2. **PR #29 是否合并？**（待决，见 §3）→ 用户定。
3. #23 收藏列表是否按路线 A 实现？→ 本文件已定，若用户有异议再议。

---

## 2. State of play

**Done（上一会话 #26 已整体完成）：**
- **#26 部署可靠性 A' 落地**（PR #28，`623e64a`）：CI 镜像交付改为 `docker save|gzip`（48.5MB→13MB）→ scp → 服务器 `docker load`，**替代服务器 GHCR pull**（原 GHCR ~2.6KB/s 曾致 PR#21 两次部署超时）。保留 GHCR 推镜像作冗余；`compose.yml` 未改（tag 不变）。首验成功：HEAD 更新 / 容器 healthy / healthz 200。**部署步实测 ~18min**（GH runner→阿里云 scp 跨洋 ~12KB/s 为物理瓶颈，已接受为现状）。**验收线改为「连续 3 次发版零超时」**（workflow.md §9.2 已同步）。服务器本地构建方案（预计 2-4min）已向用户呈现，**用户决定 8/24 后探索**。
- **`docs/workflow.md` SOP 落地**（PR #27，`cbbe4d7`）：功能开发方法论与工作流 12 节，含 #23 walkthrough、CI 门控矩阵、ACR/A' 决策记录、地图更新约定。
- **issue #14 已关闭**（状态机收口，PR #15 早已合入但 issue 未关）。
- **地图 issue #1**：决策日志 15-17（首发里程碑 + 保留数据 / #26 A' 定案+首验+收尾）；开放项/Roadmap 已同步；Notes 加了 `docs/workflow.md` 固定引用。
- **基础设施推进意见**：已交用户（评审资料 `agent panel\ai-forum-infra-plan-review.md` + 外部评审报告）。根因「提交太慢」= CI 部署步慢 + 全流程周期长，已解到可用基线。

**In progress：**
- **PR #29**（`docs/workflow-acceptance-fix`，commit `de8a1b6`）：workflow.md 验收线调整（§2 排期 + §9.2 改为「连续 3 次零超时」）。**已 push 未合并**，见 §3。

**Blocking：** 无。

---

## 3. Open decisions

1. **PR #29 合并**（docs 验收线调整，纯文档低风险）：
   - A. 合并（lean，让 workflow.md 与地图一致；顺带累积一次「零超时」计数）→ `gh pr merge 29 --merge` + 服务器验证 + 地图记一笔。
   - B. 暂缓，随 #23 一起合。
   - **倾向 A**。需用户授权（HITL 红线：合并 PR）。
2. **#23 实现范围**：已定**仅收藏**（关注拆 #27）。若用户想收藏+关注一起做，需另议（关注需新组件，改动面翻倍）。
3. **前端单测进 CI**（稳定增强，Roadmap 第 2 项）：`deploy.yml` test job 加 `npm ci && npm run test:unit`（`continue-on-error: true` 过渡，**连续 5 run 全绿后下一票转阻塞**）。**是否并入 #23 或独立 PR** → 用户定（lean：独立小 PR）。

---

## 4. Skills to use (next session)

- `tdd`：#23 后端加测试（service_test + fake_repo）、前端 Me.test.ts。
- `code-review`：合并前自审（符合深模块规范？符合 #23 意图？）。
- `grilling`：若 #23 范围/路线有异议。
- `run` / `webapp-testing`：本地起服务验证收藏端到端。

---

## 5. Artifacts（只引用，不重复）

- **状态机**：GitHub issue **#1**（决策日志 15-17 / 开放项 / Roadmap）；ticket **#23**（收藏列表，post-launch）；**#27 待建**（关注列表，8/24 后）；**#26**（已完成）。
- **SOP**：`docs/workflow.md`（功能开发方法论与工作流，**新会话必读**）。
- **文档**：`docs/ops/lessons-8888-deploy.md`（部署坑位清单）；`docs/ops/deployment-v3-8888-container.md`（部署基线 v3.1）；`docs/impl/testing.md`（测试门控矩阵）；`docs/design-principles.md`（深模块规范）。
- **代码（本次会话改动）**：`.github/workflows/deploy.yml`（A' 镜像交付）；`docs/workflow.md`（SOP）；`README.md`（指针）。
- **#23 实现要点（路线 A，已探明，下一会话直接用）**：

  **后端 3 文件**（复用 `GET /posts?tab=favorites`，最小侵入）：
  - `backend/content/repo.go`：`PostQuery`（第 6-16 行）加 `FavedByUserID *int64`。
  - `backend/content/gorm_repo.go`：`ListPosts`（第 84-112 行）加 `if in.FavedByUserID != nil { q.Where("id IN (SELECT post_id FROM content.favorites WHERE user_id = ?)", *in.FavedByUserID) }`（仿 follow 分支）。
  - `backend/content/service.go`：`ListFeed`（第 440-487 行）加 `case "favorites":`——`ViewerID==0 → ErrAuthRequired`；`q.FavedByUserID = &in.ViewerID`。**Service 接口/handler/路由/错误映射都不用改**（`ErrInvalidFeedTab` 已兜底非法值）。
  - 关键事实：现有 `ListFavedPostIDs`（repo.go:56）需 `postIDs` 范围，**不能**列出全部收藏——所以走 `PostQuery.FavedByUserID` + 子查询过滤，而非复用该方法。`postViews`（service.go:617）自动组装读模型（liked/favorited/following_author）。

  **前端 3 文件**：
  - `frontend/src/api/content.ts`：`ListPostsParams.tab`（第 25 行）联合类型扩为 `'all' | 'follow' | 'favorites'`。
  - `frontend/src/views/Me.vue`：加 `<SegTabs>`（仿 `Feed.vue` 第 18-25 行）切「帖子/收藏」；收藏 tab fetcher = `api.listPosts({ tab: 'favorites', page, pageSize })`，PostList 用 `:key="tab"` 切换（约束：fetcher 参数变化需 :key 重挂载）；游客点收藏 tab → 复用 `LoginGuide` 引导模式。
  - `frontend/src/views/Me.test.ts`：新建，仿 `views/PostDetail.test.ts` mock 模式（mock 两个 tab 的 fetcher、切换逻辑）。
  - 复用零改动：`PostList.vue`（props: fetcher+emptyTitle）、`SegTabs.vue`（options+modelValue）、`Post`/`PostViewer`/`PostList` 类型（`api/types.ts`）。

  **通用检查**：
  - `go build/vet/test` 全绿；`npm run test:unit && npm run build` 全绿。
  - `git checkout -- frontend/components.d.ts`（build 改写噪音）。
  - 合并后服务器验证（§0 命令）+ 更新地图 #1（每里程碑即时更新，改前拉最新 body 防并行覆盖）。

- **测试手册（仓库外）**：`C:\Users\liyongquan\agent panel\pre-launch-manual-test.md`（含测试账号密码）。

---

## 6. 安全注意（脱敏）

- 本文件不含任何 token/密码。测试账号密码只在测试手册（仓库外）。
- 服务器 `.env`（POSTGRES_PASSWORD / JWT_SECRET）只存服务器，勿外泄、勿进 git。
- 此前会话曾暴露 gh token → 建议用户已/再执行 `gh auth refresh` 轮换。

---

*本 handoff 为当票工作底稿（非持久化）；完成后由 INDEX 摘要 + issue 记录替代，可删除。*
