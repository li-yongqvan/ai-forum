# Handoff · 项目推进交接（2026-08-20）

> 生成：2026-08-20 · 交接会话（Claude Code），基于 `launch-prep-8888.md` 的**服务器真值核对版**。
> **本文件是自包含交接**：给**零上下文**的下一会话读，读完即可上手，不依赖上一会话记忆。若内容与服务器/DB/GitHub 真值冲突，**以真值为准**。
> 前置：`docs/handoffs/launch-prep-8888.md`（上线前准备，内容仍有效，本文件为其**真值更新 + 续篇**）；`docs/handoffs/fix-8888-bugs.md`（旧交接，已完成可删）。

---

## 0. 零上下文启动块（新会话必读）

**这是什么**：AI 智联论坛——面向学院师生的 AI 主题社区（发帖/评论/点赞/收藏/关注/图片上传/通知/版主治理）。技术栈：**Go（Gin + GORM + PostgreSQL）后端 + Vue3 + TS + Vant（PWA）前端**。代码在本机 `C:\Users\liyongquan\ai-forum`；线上 `http://122.51.233.225:8888/`（容器 nginx@8888 → api → postgres）。服务器阿里云、用户 `liyongquan`（**无 sudo**）、repo `~/ai-forum`。仓库**无 CLAUDE.md**，本文件 + GitHub issue **#1**（地图 = 项目唯一状态机）就是你的上下文。

**新会话前 5 分钟**：
1. 读本文件 + `launch-prep-8888.md`；读 GitHub issue **#1**（决策日志 1-14 / Roadmap / 开放项）。
2. 连服务器核对真值：`ssh liyongquan@122.51.233.225 'cd ~/ai-forum && git log --oneline -1 && docker compose ps && ls ~/backups'`。
3. 向用户确认 §3 待决项。

**访问 / 命令速查**：
- 服务器：`ssh liyongquan@122.51.233.225`
- 生产 DB：`ssh ... 'cd ~/ai-forum && docker compose exec -T postgres psql -U forum forum'`，schema `"user"/"content"/"notify"/"moderation"`，db/user 均 `forum`
- 本地工具链：Go 1.26 / Node 24 / Docker Desktop / `gh`（**gh 一律前缀 `MSYS_NO_PATHCONV=1`**，失败重试 ≤4、换方式验证）
- 本地测试：`cd backend && go build ./... && go vet ./... && go test ./...`；`cd frontend && npm run test:unit && npm run build`
- 线上健康：`curl http://122.51.233.225:8888/healthz`、`/api/v1/posts`

**git 当前状态（2026-08-20 实测）**：
- 本地可能有并行会话残留分支；**动手前 `git fetch origin && git checkout main && git pull origin main`**。
- `origin/main` 最新 = **PR #25**（`docs/ops/lessons-8888-deploy.md` 经验清单）；#24/#22/#21 均已合入。

**协作流水线（每次改动都走）**：
`git checkout -b <type>/<slug>` → 实现 → 本地测试全绿 → conventional commit → push → `gh pr create` → **用户明确授权**后 `gh pr merge N --merge` → CI 自动部署 → **服务器侧验证真值** → 更新 issue #1。前端单测只在本地跑（未进 CI）。

---

## 1. 服务器真值（2026-08-20 核对，与 launch-prep 的差异点）

| 项 | 实测 | 说明 |
|---|---|---|
| 服务器 git HEAD | `b26607e` = PR #25 | 与 origin/main 同步 ✅ |
| 容器 | api **healthy** / postgres **healthy** / web up | ✅ |
| **备份 cron** | **`~/backups/forum-2026-08-20.sql.gz` 已存在** | 🆕 **昨晚 cron 真跑了**（pg_dump 备份生效） |
| 线上健康 | healthz **200**；`/api/v1/posts` 正常返回 | 有内容：帖子 id 到 9（author_id 5 = 人工测试账号） |
| 本地 api 镜像 | `ghcr.io/li-yongqvan/ai-forum-api:latest` | 手动 build 产物 |

**结论**：部署基线健康、备份闭环已生效；当前库里有**人工测试数据**（帖子到 id 9 + 5 测试账号），是否清空待 §3 决策。

## 2. 当前状态摘要（详版见地图 #1 决策日志 1-14）

- **线上已上线并验收**（#8 变体 8888 容器化，PR #17-20）；Bug A 无评论崩溃已修（#21）、测试数据已清理（#13，后又产生人工测试数据）、收藏/关注占位 tab 已隐藏（#24）、经验清单已入库（#25）。
- **首发前人工测试进行中**：5 测试账号（`tester_a/b/c/d` + `moderator_a`），**密码在测试手册**（§5，仓库外）；手册含全部测试场景。
- **首发后首批**（地图 Roadmap 顺序）：部署可靠性（#26）→ 前端单测进 CI → #23 收藏/关注列表 → 下一批。

## 3. Open decisions（向用户确认后执行）

1. **#26 部署可靠性方案**（首发后第一优先；阻塞每发版——**GHCR blob 下载极慢 ~573B/s**，CI 部署步曾超时）：
   - A. **阿里云 ACR**（lean）：服务器在阿里云，拉取快稳。需用户开通 ACR 个人版 + 提供命名空间/凭证 → CI 推送 ACR + compose 镜像源改 ACR。
   - B. GHCR 第三方代理：改动小，有信任权衡。
   - C. 维持 GHCR：不可取。
   - **临时 workaround**（服务器 CI 超时时）：本机 `docker build -t ghcr.io/li-yongqvan/ai-forum-api:latest .` → `docker save -o /tmp/api.tar` → `scp` → 服务器 `docker load` + `docker compose up -d --force-recreate api`。⚠️ 镜像名 **`li-yongqvan`**（qvan，勿打成 quan）。
2. **首发前是否清空人工测试数据**：A 清空（lean，同 #13 方式：psql 事务 + 快照）；B 保留作初始内容。
3. **首发时点 + 里程碑记录**：确定后在 issue #1 决策日志追加「首发上线」。

## 4. 已知坑（务必避免/注意）

- **GHCR 拉取慢**：CI 部署步可能超时 → 用 §3.1 的 workaround；根因排查走 #26。
- **gh 间歇 404/TLS**：`MSYS_NO_PATHCONV=1` + 重试 ≤4 + 换验证方式；服务器侧真值优先。
- **服务器 repo 卫生红线**：`~/ai-forum` 内文件只准 git 改动，**禁止 scp/手工写**（会挡 CI 的 git pull，踩过）。
- **issue #1 并行编辑风险**：改地图前先 `gh api repos/li-yongqvan/ai-forum/issues/1 --jq .body` 拉最新；`--body-file` 整段覆盖，别用旧快照盖新内容。
- `frontend/components.d.ts` 会被 build/测试改写 → 提交前 `git checkout -- frontend/components.d.ts`。
- lint 非阻断失败 1 项：`backend/internal/httpapi/handler/upload.go:46` `src.Close()` errcheck（遗留）。
- **HTTP 纯环境下 PWA SW 不注册**（已知技术债，#8 决策后果）。

## 5. Suggested skills

- 首发后首批/新功能：走正常 ticket → PR 流水线；`code-review`（合并前自审）、`tdd`（补单测）
- #26 部署可靠性调研：`research` + `grilling`（HITL 决策在新会话做）
- 部署排障/诊断：`diagnosing-bugs`
- 文档/交接类：`handoff`（本文件同款格式）

## 6. Artifacts（只引用，不重复）

- **状态机**：GitHub issue **#1**（决策日志 1-14 / 开放项 / Roadmap）；ticket **#21**（已解决）、**#23**（post-launch）、**#26**（部署可靠性，待决）
- **前序交接**：`docs/handoffs/launch-prep-8888.md`（上线前准备，内容仍有效）
- **文档**：`docs/ops/lessons-8888-deploy.md`（部署坑位清单，PR #25）；`docs/ops/acceptance-8888-results.md`（验收+清理记录）；`docs/ops/deployment-v3-8888-container.md`（基线 v3.1）
- **代码（上次改动）**：`backend/content/service.go:540`（空树 `[]`）、`frontend/src/views/PostDetail.vue`（`?? []`）、`Me.vue`（隐藏 tab）、`vite.config.ts`（Vant inline 单测基建）
- **测试手册（含账号密码，仓库外）**：`C:\Users\liyongquan\agent panel\pre-launch-manual-test.md`
- **服务器**：`122.51.233.225` / 用户 `liyongquan`（无 sudo）/ repo `~/ai-forum`；DB 见 §0

## 7. 安全注意（脱敏）

- 本文件不含任何 token/密码。测试账号密码只在测试手册（仓库外）。
- 服务器 `.env`（POSTGRES_PASSWORD / JWT_SECRET）只存服务器，勿外泄、勿进 git。
- 此前会话曾暴露 gh token → 建议用户已/再执行 `gh auth refresh` 轮换。

---

*本 handoff 为当票工作底稿（非持久化）；完成后由 INDEX 摘要 + issue 记录替代，可删除。*
