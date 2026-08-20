# Handoff · 上线前准备（launch-prep-8888）

> 生成：2026-08-20 · 交接会话（Claude Code）。
> **本文件是自包含交接**：给**零上下文**的下一会话读，读完即可上手，不依赖上一会话记忆。若内容与服务器/DB/GitHub 真值冲突，**以真值为准**。
> 前置：`docs/handoffs/fix-8888-bugs.md`（两个 bug 的旧交接，已全部完成，可删）。

---

## 0. 零上下文启动块（新会话必读）

**这是什么**：AI 智联论坛——面向学院师生的 AI 主题社区（发帖/评论/点赞/收藏/关注/图片上传/通知/版主治理）。技术栈：**Go（Gin + GORM + PostgreSQL）后端 + Vue3 + TS + Vant（PWA）前端**。代码在本机 `C:\Users\liyongquan\ai-forum`；线上 `http://122.51.233.225:8888/`（容器 nginx@8888 → api → postgres）。服务器阿里云、用户 `liyongquan`（**无 sudo**）、repo `~/ai-forum`。仓库**无 CLAUDE.md**，本文件 + GitHub issue #1 就是你的上下文。

**新会话前 5 分钟**：
1. 读本文件；读 GitHub issue **#1**（地图 = 项目唯一状态机：决策日志 / Not yet specified / Roadmap）。
2. 连服务器核对真值：`ssh liyongquan@122.51.233.225`（免密 key 已配）；`cd ~/ai-forum && git log --oneline -1 && docker compose ps`。
3. 向用户确认 §3 的待决项。

**访问 / 命令速查**：
- 服务器：`ssh liyongquan@122.51.233.225`
- 生产 DB（只读/写都走这条）：`ssh ... 'cd ~/ai-forum && docker compose exec -T postgres psql -U forum forum'`，schema 为 `"user"/"content"/"notify"/"moderation"`，db/user 均 `forum`
- 本地工具链：Go 1.26 / Node 24 / Docker Desktop / `gh`（gh 一律前缀 `MSYS_NO_PATHCONV=1`，失败重试 ≤4 次）
- 本地测试：`cd backend && go build ./... && go vet ./... && go test ./...`；`cd frontend && npm run test:unit && npm run build`
- 线上健康：`curl http://122.51.233.225:8888/api/v1/posts`、`/healthz`

**git 当前状态（2026-08-20 实测）**：
- 本地可能停在并行会话的残留分支（如 `docs/lessons-8888`）；**动手前先 `git checkout main && git pull origin main` 对齐远端**。
- `origin/main` 最新 = PR #25（`docs/ops/lessons-8888-deploy.md` 部署坑位清单）合并；PR #24（隐藏占位 tab）、#22、#21 均已合入。
- 本地 `main` 可能落后：`git fetch origin && git log origin/main..HEAD --oneline` 自查。

**协作流水线（每次改动都走）**：
`git checkout -b <type>/<slug>` → 实现 → 本地测试全绿 → conventional commit → push → `gh pr create` → **用户明确授权**后 `gh pr merge N --merge` → CI 自动部署（push main 触发）→ 服务器侧验证 → 更新 issue #1 决策日志。前端单测只在本地跑（未进 CI）。

---

## 1. Goal of next session

**主目标：推进 v1 首发 + 首发后首批。** 产出：
- 定下两个首发前待决（§3 决策 1、2），必要时执行；
- 首发后首批按地图 Roadmap 顺序：**部署可靠性（#26）→ 前端单测进 CI → #23 收藏/关注列表 → 下一批**；
- 首发时点确定后，在 issue #1 决策日志追加「首发上线」里程碑。

**Prompts to answer：**
1. 当前线上/代码/数据状态？→ 读本文件 + issue #1 + 服务器真值。
2. #26 部署可靠性选 A/B/C？→ 用户定（A 需 ACR 凭证）。
3. 首发前是否清空人工测试数据？→ 用户定。
4. 首发是否记里程碑？→ 是（惯例）。

## 2. State of play

**Done（已上线并验证）：**
- **Bug A 无评论详情崩溃（#21）**：后端 `backend/content/service.go:540` 空树返回 `[]` + 前端 `frontend/src/views/PostDetail.vue:39-40` `?? []`。回归测试：后端 `TestGetCommentTreeEmpty`、前端 `PostDetail.test.ts`（新建；`vite.config.ts` 加了 Vant inline 基建）。PR #21 合并部署，`GET /api/v1/posts/5/comments` → `"comments":[]`。
- **Bug B 测试数据清理（#13）**：生产库清空全部 smoke 数据（5 帖/评论/赞藏关注/用户 1、2/uploads 8 图）；保留 boards/topics 种子 + 邀请码。快照 `~/ai-forum-backups/ai-forum-pre-clean-2026-08-19.sql`。
- **#23 决策 B（v1 先藏）**：「我的」页收藏/关注占位 tab 已隐藏（`Me.vue`，PR #24 合并部署验证）。
- **地图 issue #1**：决策日志 1-14、开放项、`## Roadmap / 下一里程碑` 均已更新；#23 标 `post-launch`；#26 已开。PR #22（验收文档清理状态）已合并。
- **人工测试准备**：5 测试账号已建（`tester_a/b/c/d` + `moderator_a`），**密码在测试手册**（§5 路径，仓库外）；手册含全部测试场景，人工测试进行中。

**Blocking / 已知（重要）：**
- **GHCR 拉取极慢（~573B/s）**：CI 部署步 `docker compose pull api` 可能超时（PR #21 曾超时）。**Workaround**（服务器）：
  ```
  # 本机：cd backend && docker build -t ghcr.io/li-yongqvan/ai-forum-api:latest .
  #       docker save ghcr.io/li-yongqvan/ai-forum-api:latest -o /tmp/api.tar
  #       scp /tmp/api.tar liyongquan@122.51.233.225:/tmp/
  # 服务器：docker load -i /tmp/api.tar && cd ~/ai-forum && docker compose up -d --force-recreate api
  ```
  ⚠️ 镜像名是 `ghcr.io/li-yongqvan/ai-forum-api`（**qvan**，勿打成 quan）。根因排查见 #26。
- gh 间歇 404/TLS：`MSYS_NO_PATHCONV=1` + 重试 ≤4；服务器侧真值优先。
- **issue #1 有并行编辑风险**：改地图前先 `MSYS_NO_PATHCONV=1 gh api repos/li-yongqvan/ai-forum/issues/1 --jq .body` 拉最新；`--body-file` 是整段覆盖，别用旧快照盖新内容。
- `frontend/components.d.ts` 会被 build/测试改写（行尾噪音）→ 提交前 `git checkout -- frontend/components.d.ts`。
- lint 非阻断失败 1 项：`backend/internal/httpapi/handler/upload.go:46` `src.Close()` errcheck（遗留）。

## 3. Open decisions

1. **#26 部署可靠性方案**（首发后第一优先；阻塞每发版）：
   - A. **阿里云 ACR**（lean）：服务器在阿里云，拉取快且稳。落地需：用户开通 ACR 个人版 + 提供命名空间/凭证 → CI 推送 ACR（GitHub secrets）+ `compose.yml` 镜像源改 ACR。
   - B. GHCR 第三方代理：改动小，有信任权衡。
   - C. 维持 GHCR：不可取（每发版撞墙）。
2. **首发前是否清空人工测试数据**（当前 5 测试帖 + 5 账号 + 邀请码）：
   - A. 清空（lean）：首发从干净状态开始；执行方式同 #13（psql 事务 + 快照）。
   - B. 保留作初始内容。
3. **首发时点 + 里程碑记录**：确定后在 issue #1 决策日志追加「首发上线」。

## 4. Skills to use

部署排障/诊断：`diagnosing-bugs`；#26 调研：`research` + `grilling`（HITL 决策在新会话做）；新功能走正常 ticket→PR 流水线。

## 5. Artifacts（只引用，不重复）

- **状态机**：GitHub issue **#1**（决策日志 1-14 / 开放项 / Roadmap）；ticket **#21**（已解决）、**#23**（post-launch）、**#26**（ACR，待决）；PR **#21/#22/#24/#25**（已合入）。
- **文档**：`docs/ops/acceptance-8888-results.md`（清理已标记）；`docs/ops/deployment-v3-8888-container.md`（基线 v3.1）；`docs/ops/lessons-8888-deploy.md`（部署坑位清单，PR #25）。
- **代码（本次改动）**：`backend/content/service.go:540`、`service_test.go`（TestGetCommentTreeEmpty）、`frontend/src/views/PostDetail.vue:39-40`、`PostDetail.test.ts`、`Me.vue`、`vite.config.ts`。
- **测试手册（含账号密码，仓库外）**：`C:\Users\liyongquan\agent panel\pre-launch-manual-test.md`。
- **服务器**：`122.51.233.225` / 用户 `liyongquan`（无 sudo）/ repo `~/ai-forum`；DB 见 §0。

---

*本 handoff 为当票工作底稿（非持久化）；完成后由 INDEX 摘要 + issue 记录替代，可删除。*
