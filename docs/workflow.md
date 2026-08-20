# ai-forum 功能开发方法论与工作流（SOP）

> **定位**：后续所有功能添加的标准操作流程。**新会话必读**：本文件 + 地图 issue #1。
> **状态**：2026-08-20 定稿（v1），吸收方案评审修订。
> **真值优先**：若与服务器/DB/GitHub/仓库文件冲突，**以真值为准**。命令均已实测，改动流水线后再同步本文档。

---

## 0. 速查

**流水线一句话**：
```
取票(地图#1 Roadmap) → 设计(grilling/深模块) → 分支 → 实现 → 本地测试全绿
→ commit → push → gh pr create → 用户授权 → merge → CI部署 → 服务器验证 → 更新地图#1
```

**关键命令**（详见各节）：
- 服务器核对真值：`ssh liyongquan@122.51.233.225 'cd ~/ai-forum && git log --oneline -1 && docker compose ps'`
- 生产 DB：`ssh ... 'cd ~/ai-forum && docker compose exec -T postgres psql -U forum forum'`
- 本地后端测试：`cd backend && go build ./... && go vet ./... && go test ./...`
- 本地前端测试：`cd frontend && npm run test:unit && npm run build`
- 线上健康：`curl http://122.51.233.225:8888/healthz`
- workflow 校验：`bash scripts/check-workflows.sh`
- gh 一律前缀 `MSYS_NO_PATHCONV=1`；GitHub API 间歇 404/TLS，重试 ≤4、换验证方式、服务器侧真值优先。

**关键文件索引**：
- 状态机：GitHub issue **#1**（决策日志/开放项/Roadmap）
- 设计规范：`docs/design-principles.md`；测试约定：`docs/impl/testing.md`；部署坑位：`docs/ops/lessons-8888-deploy.md`；部署基线：`docs/ops/deployment-v3-8888-container.md`
- 代码：后端 `backend/content|user|upload|notify|moderation/`，组合根 `backend/internal/httpapi/router.go`；前端 `frontend/src/`（views/components/stores/api/utils）

---

## 1. 核心理念

1. **小步提交**：一个功能 = 一个 ticket = 一个 PR。小 diff 快审快合，避免大 PR 挤压周期。
2. **先稳管线，再堆功能**：部署/CI 管线不稳时，不排新功能（当前顺序见 §2）。
3. **深模块（Deep Module）**：接口小、实现厚。设计语境用 `codebase-design` 词表，禁用 component/service/API/boundary 指代设计单元（详见 `docs/design-principles.md`）。
4. **HITL 红线**：**合并 PR / 删生产数据 / 上生产 / 关闭 issue** 必须用户明确授权。人工授权等待是**有意保留的设计**，不优化（决策记录于地图 #1）。
5. **地图即状态机**：issue #1 是唯一真值，每里程碑即时更新（见 §10），不攒到收尾。

---

## 2. 功能来源与排期

- **唯一状态机 = 地图 issue #1** 的 Roadmap / 开放项。新功能先落 ticket（issue），再进 Roadmap。
- **优先级原则**：价值 × 学习目标（项目是学习导向，见地图 Destination）。
- **当前顺序**（首发后首批）：
  1. **#26 部署可靠性：镜像交付 CI 内 scp 镜像包（A' 方案，已落地）**（见 §9.2；不依赖任何 registry 账号；验收线 = 连续 3 次发版零超时）
  2. **前端单测进 CI**（`continue-on-error` 过渡 → 转阻塞，见 §7）
  3. **#23 收藏/关注列表**（见 §12 walkthrough）
  4. 下一批按「价值 × 学习目标」排，朝组合 B 微服务演进。
- **根因 2.3（本地全量测试慢）的显式表态**：开发中只测改动包（`go test ./<pkg>/...`），push 前全量；**不引入 pre-push 钩子**（避免本地环境差异阻塞提交，CI 是最终 gate）。

---

## 3. 设计阶段（动手前）

有开放决策时**先设计再实现**，避免返工：

1. **grilling**：开放决策/方案权衡 → HITL 对话定案，产出决策记录。
2. **domain-modeling**：涉及领域术语/边界时对齐通用语言。
3. **codebase-design**：接口设计（深模块三问：能否减少方法？能否简化参数？能否把更多复杂度藏进实现？）。
4. **决策落档**：`docs/handoffs/grilling-decisions/<issue>-<slug>-decisions.md`（参考现有 `issue-12-image-upload-decisions.md`）。

**产出**：ticket 上明确「方案 → 实现要点 → 影响面」，新会话据此接手。

---

## 4. 分支与提交

1. **动手前对齐**：`git fetch origin && git checkout main && git pull origin main`（本地可能有并行会话残留分支，先对齐远端）。
2. **建分支**：`git checkout -b <type>/<slug>`（type：`feat`/`fix`/`docs`/`chore`/`refactor`）。
3. **提交规范**：conventional commit，`<type>(<scope>): <subject>`，scope 用包/模块名（如 `fix(content)`、`docs(ops)`）。
4. **服务器 repo 卫生红线**：服务器 `~/ai-forum` 内文件**只准 git 改动，禁止 scp/手工写**（会挡 CI 的 git pull，踩过，见 lessons §1-B）。
5. **提交前检查**：`frontend/components.d.ts` 会被 build/测试改写 → `git checkout -- frontend/components.d.ts`。

---

## 5. 实现：后端

**域名包五件套**（参考 `backend/content/`，全部已实现）：
```
<包>/model.go        # 领域模型 + DTO
<包>/repo.go         # Repo 接口（seam，测试跨此）
<包>/gorm_repo.go    # GORM 实现
<包>/service.go      # Service 接口 + 实现（粗粒度命令 + 读模型查询）
<包>/README.md       # Interface README（外部调用方必读：不变量/顺序约束/错误模式/配置/性能，不写签名）
```

**新增功能装订模式**：
1. 判断归属包：用户→`user`，板块/话题/帖子/评论/赞藏→`content`，通知/私信→`notify`，举报/治理→`moderation`，图片→`upload`。
2. `service.go` 加**深接口**（命令 DTO + 读模型 View，如 `CreatePostCmd`/`PostView`），实现藏在 service；哨兵错误变量（如 `ErrPostNotFound`）。
3. `repo.go` + `gorm_repo.go` 加查询/写入方法；**分页列表是独立查询形态**（现有 `ListFavedPostIDs` 是"给定 postIDs 求交集"，不满足分页列表——新列表需求需加 offset/limit 方法，参考 #23 §12）。
4. **跨包调用经接口**：不直接 import 他包，定义接口（如 `content` 的 `UserProvider`）+ 组合根 adapter（`router.go` 装配）。
5. `internal/httpapi/handler/<pkg>.go` 加 handler：登录墙取 `middleware.Identity`（游客 id 0），错误映射走 `respondContentError`。
6. `internal/httpapi/router.go` 注册路由（前缀 `/api/v1`）：
   - `auth`（免登录）：`/auth/register` `/auth/login`
   - `authed`（Auth）：`/posts` `/comments` `/likes` `/favorites` `/follows` `/uploads` `/auth/me` `/auth/logout`
   - `reads`（OptionalAuth）：`/boards` `/topics` `/posts` `/posts/:id` `/posts/:id/comments` `/users/:id`
   - `mod`（Auth+RequireRole）：pin/feature
7. **迁移 SQL**：追加 `backend/migrations/000N_*.sql`（应用启动 embed 自动跑，参考 0001-0005）。
8. `main.go` 一般不动（除非新增包依赖装配）。

**跨包聚合在 handler 层做**（参考 `follow.go` 按 target_type 分派、`profile.go` 合并两包）。

---

## 6. 实现：前端

**目录**：`views/`（页面）`components/`（复用件）`stores/`（Pinia）`api/`（领域 API 函数 + 类型）`utils/`（纯函数）。

**新增功能装订模式**：
1. `api/<domain>.ts` 加 API 函数：走统一 `api/client.ts` 的 `request<T>(method, path, body, {requireAuth})`（统一 `/api/v1` 前缀 + Bearer + 401 归一）。
2. 类型集中在 `api/types.ts`（`Post`/`PostView`/`PostList` 等）。
3. 复用组件优先：`PostList.vue`（无限滚动通用列表，props 为 `fetcher`）、`SegTabs.vue`（分段 tab）、`Empty.vue`（空态）。
4. 单测同目录 `<Name>.test.ts`，mock 模式参考 `views/PostDetail.test.ts`（vi.mock vue-router/vant/auth store/api）。
5. 构建：`npm run build` = `vue-tsc -b && vite build`（类型检查 + 构建）。
6. **提交前** `git checkout -- frontend/components.d.ts`。

---

## 7. 测试

**门控矩阵**（详见 `docs/impl/testing.md`）：

| 层 | 方式 | CI 门控 | 覆盖目标 |
|---|---|---|---|
| 后端 service | 单测，fake repo（`<包>/fake_repo_test.go`） | **阻塞**（test job） | 成功+主要失败路径；核心模块 ≥70% |
| 后端 handler | 集成，testcontainers 真实 PG（`internal/testutil/db.go`，无 Docker 自动 skip） | **阻塞** | 关键 REST 端点 |
| 后端 lint | golangci | **非阻塞**（`continue-on-error`） | 已知遗留 `upload.go:46` errcheck |
| 前端单测 | vitest（`<Name>.test.ts`） | **当前不进 CI**；规划 `continue-on-error` 过渡 → **连续 5 run 全绿后下一票转阻塞** | 工具/composable/store；组件参照 PostDetail |
| E2E | Playwright | **不进 MVP CI**，本地跑 | 主流程冒烟 |
| workflow 自身 | actionlint（`workflow-lint.yml`，任何分支 push） | **阻塞** | 防止 CI 瘫痪 |

**写测试原则**：测试只跨 seam（service 接口），不测实现内部；执行「删除测试」（删掉模块后测试复杂度消失 = 模块浅）；单 Adapter 警示（大量 mock 私有实现 = seam 位置不对）。

**本地测试**（push 前全量）：
- 后端：`cd backend && go build ./... && go vet ./... && go test ./...`
- 前端：`cd frontend && npm run test:unit && npm run build`

---

## 8. PR 与合并

1. push 分支：`git push origin <branch>`。
2. 建 PR：`MSYS_NO_PATHCONV=1 gh pr create --base main --title "<type>(<scope>): <desc>" --body "..."`（gh 间歇 404，失败重试 ≤4；或 curl 完整 URL）。
3. **合并前自审**：`/code-review`（审查：符合规范？符合 ticket 意图？）—— 桌面/终端执行 skill。
4. **等待用户明确授权**，然后 `gh pr merge <N> --merge`（保持合并历史）。
5. **PR 阶段无 CI 是已知遗留**：`deploy.yml` 仅 `on: push: [main]`。规划项：加 `pull_request` 触发（test/lint 在 PR 跑、deploy job 级 `if: github.event_name == 'push' && github.ref == 'refs/heads/main'`、test/lint 加 `paths-ignore: ['docs/**','**.md']`），见 §9.3。落地前，合并进 main 的那次 CI 是首个反馈点。

---

## 9. 部署与验证

### 9.1 CI deploy job 流程（当前）
push main → `test`(阻塞) → `lint`(非阻塞) → `deploy`：
前端 `npm ci && npm run build` → scp `frontend/dist/*` 到服务器 `~/ai-forum/frontend/dist`（`strip_components: 2`）→ 推 GHCR 镜像（冗余备份）→ `docker save | gzip` 出镜像包 → scp 到服务器 `/tmp` → ssh 脚本：`git pull` → `mkdir -p frontend/dist uploads` → `docker load -i /tmp/api-image.tar.gz` → `up -d --force-recreate api` → `nginx -t && nginx -s reload` → `image prune` → `compose ps`。

**门控**：secrets → job env → step `if: env.SSH_HOST != '' && vars.DEPLOY_ENABLED == 'true'`。**secrets 永不进 if**（job/step 皆禁，白名单：job 级 `github, needs, vars, inputs`；step 级含 `env`），本地校验 `bash scripts/check-workflows.sh`。

### 9.2 镜像交付：CI 内 scp 镜像包（A' 方案，#26 定案）
**问题**：服务器→GHCR 下载 ~KB/s（实测 573B/s、2.6KB/s 两档），PR#21 曾两次部署超时。原定 GHCR→阿里云 ACR，但**共享服务器无法登录其阿里云账号**（ACR 需该账号开通），故改为 A'。

**A' 方案（已落地 deploy.yml）**：
- CI 构建镜像（build-push-action `load: true` 同时进 runner daemon）→ 推 GHCR 作冗余备份 → `docker save | gzip`（实测 48.5MB → **13MB**）→ `scp api-image.tar.gz` 到服务器 `/tmp` → 服务器 `docker load` → `compose up -d --force-recreate api`。
- **零外部依赖**：不碰 daemon、不需任何 registry 账号、不依赖 sudo；SSH 链路与前端 scp 同一条（已验证可达）。实测本机→服务器 SSH ~780KB/s，13MB ≈ 17s，比 GHCR 快约 300 倍。
- **compose.yml 无需改**：镜像 tag 保持 `ghcr.io/li-yongqvan/ai-forum-api:latest`，load 进去即被 compose 识别。
- **量化验收**（#26 关闭条件）：连续 3 次发版零超时（部署步实测 ~18min，瓶颈为 GH runner→阿里云 scp 跨洋带宽 ~12KB/s，已接受为现状，不再追求 ≤3min）。
- 注意事项：CI runner（美国）→阿里云 scp 带宽 ~12KB/s（本机→服务器 ~780KB/s，跨洋慢得多）；`appleboy/scp-action` 的 `source` 需相对 `$GITHUB_WORKSPACE`。

**不选其他路径的原因**：ACR 需服务器阿里云账号（不可得）；服务器本地构建需 golang 基础镜像（Docker Hub 被墙、服务器无该镜像）；改 daemon 镜像加速需 sudo（无 sudo）。

### 9.3 规划项：PR 阶段功能 CI（#26 后并入）
见 §8.5。目标：错误在合并前暴露，消掉「合并→修→再合并」返工。

### 9.4 部署后服务器验证
```
ssh liyongquan@122.51.233.225 'cd ~/ai-forum && git rev-parse --short HEAD && docker compose ps'
curl http://122.51.233.225:8888/healthz
curl http://122.51.233.225:8888/api/v1/posts
```
对照：HEAD = 预期合并 SHA、容器 healthy、healthz 200、API 正常。**CI run 状态优先看服务器落地效果**（lessons §2）。

### 9.5 备份
宿主机 cron：每晚 pg_dump 保留 14 + 每周 uploads/.env 保留 8（`~/backups/`）。**删生产数据前先快照**（同 #13 方式：psql 事务 + 快照）。

---

## 10. 地图更新（issue #1）

**每完成一个里程碑就更新**，不攒到收尾（用户明确在意及时性）。

1. **并行编辑防护**：改前先 `MSYS_NO_PATHCONV=1 gh api repos/li-yongqvan/ai-forum/issues/1 --jq .body` 拉最新；`--body-file` 是**整段覆盖**，别用旧快照盖新内容。
2. 更新对应段落：决策日志追加条目、Blocking 图、开放项、Roadmap、基础设施状态。
3. 核对 `updatedAt` 变化。

---

## 11. 已知坑位速查（详见 `docs/ops/lessons-8888-deploy.md`）

| 坑 | 对策 |
|---|---|
| Git Bash 下 gh 路径被 MSYS 改写 → 假 404 | 一律 `MSYS_NO_PATHCONV=1`；写操作 curl 完整 URL |
| gh 间歇 404/TLS | 重试 ≤4、换验证方式、服务器侧真值优先 |
| `ssh -T` 认证成功也退 1 | 先捕获输出再 grep（`... || true`） |
| `$host` 剥端口 → 上传 URL 丢 `:8888` | `proxy_set_header Host $http_host` |
| web.conf bind mount 不重建容器 | 改后 `nginx -t && nginx -s reload` |
| golangci v1/v2 配置混写 | 本地用同版 Docker 镜像预验证（`golangci-lint config verify`） |
| 服务器 repo 被 scp 污染 | 只 git 改，禁 scp/手工写 |
| Windows CRLF 崩服务器 bash | `.gitattributes` 强制 `*.sh`/`web.conf` 为 LF |
| `frontend/components.d.ts` 被改写 | 提交前 `git checkout --` |
| PWA 纯 HTTP 下 SW 不注册 | 已知技术债（#8 决策后果），SPA 正常 |
| 镜像名 `li-yongquan` vs `li-yongqvan` | 用 `li-yongqvan`（qvan） |

---

## 12. 新功能配方（Checklist）

> 以 **#23 收藏/关注列表**为 walkthrough（补 GET 收藏列表接口 + 接回「我的」页 tab）。

**后端**（6 处）：
- [ ] `backend/content/repo.go`：`Repo` 接口加分页列表方法（如 `ListFavoritePostIDs(ctx, userID, offset, limit)`）——现有 `ListFavedPostIDs` 是"给定 postIDs 求交集"，不满足分页，需新增。
- [ ] `backend/content/gorm_repo.go`：GORM 实现。
- [ ] `backend/content/service.go`：`Service` 接口加 `ListFavorited(ctx, viewerID, page, pageSize)`，复用 `postViews` 组装读模型。
- [ ] `backend/internal/httpapi/handler/content.go`：`ContentHandler` 加 handler（登录墙取 `middleware.Identity`，绑定 query 分页，仿 `ListPosts`）。
- [ ] `backend/internal/httpapi/router.go`：`authed` 组加 `GET /favorites`（与 POST/DELETE `/favorites` 同组）。
- [ ] 测试：`content/service_test.go` + `fake_repo_test.go` 补 fake 方法 + `TestListFavorited...`；handler 层 `content_test.go`（需 Docker）。

**前端**（4 处）：
- [ ] `frontend/src/api/content.ts`：加 `listFavorites(params)`（仿 `listPosts`，URLSearchParams 拼 query）。
- [ ] `frontend/src/api/types.ts`：如需新类型则补（`PostList` 可复用）。
- [ ] `frontend/src/views/Me.vue`：加 SegTabs 切换「帖子/收藏」（仿 `Feed.vue` 的 all/follow），收藏 tab 的 fetcher 调 `api.listFavorites`；复用 `PostList`。
- [ ] `frontend/src/views/Me.test.ts`：新建，仿 `PostDetail.test.ts` mock 模式。

**通用检查**：
- [ ] `go build/vet/test` 全绿；`npm run test:unit && npm run build` 全绿
- [ ] `git checkout -- frontend/components.d.ts`
- [ ] `MSYS_NO_PATHCONV=1 gh pr create` → 用户授权 → `gh pr merge <N> --merge`
- [ ] 服务器验证（§9.4）+ 更新地图 #1（§10）

---

*本文档为持久化 SOP。变更流水线（镜像交付 A' 落地/PR CI/前端单测转正）后，必须同步本文档对应节。*
