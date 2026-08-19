# Handoff · 8888 容器化部署执行（共享服务器无 sudo，并入 #8）

> 状态：2026-08-19 待新会话接手执行。**决策已全部定稿**（v3.1 执行基线），本会话只需按 §执行清单落地，无需重新探查/再评审。
> 规范依据：handoff 5 段式 + wayfinder 机制（`docs/handoffs/INDEX.md`）。本工作**并入 #8，不另开 ticket**。

---

## 1. Goal of next session

把 ai-forum 部署到共享服务器 `122.51.233.225` 的 **8888 端口容器化方案** 从文档变成现实。目标态：`http://122.51.233.225:8888/` 返回 PWA，API/上传/健康检查全通，CI 每次 push main 自动部署前端 + API。

**执行清单（§执行清单）**——你（agent）能做 ①③④⑥⑦⑧，用户做 ②⑤：
- ① repo read-only deploy key（bootstrap 硬前置）
- ③ GitHub variable `DEPLOY_ENABLED`（先空/`false`）
- ④ **代码 PR**（把 v3.1 落成 repo 变更，见 §3）
- ⑥ 服务器跑 `bootstrap.sh`
- ⑦ `DEPLOY_ENABLED=true` + push main 全链路验证
- ⑧ 配备份 cron
- ② 轮换泄漏密码（**用户**）；⑤ GHCR 包改 Public（**用户**，见 §2.5）

**验收标准**：`curl http://122.51.233.225:8888/healthz` 200；`/` 返回 PWA；登录/注册/发帖/发图全通；未配 `DEPLOY_ENABLED=true` 时 CI 上传/部署步跳过、run 不红。

**Prompts to answer**：无重大待决。动手前先核对 §2 的「已验证事实」是否仍成立（服务器/仓库状态），以及 §4 的当前 git 状态（`compose.yml` 未提交）。

## 2. State of play

### 2.1 已完成（2026-08-19）
- **#14 前端 CI 部署已合 main**（PR #15）：deploy.yml 前端构建 + scp 上传 + 镜像推送 + SSH 部署；actionlint 基建已上线。
- **GitHub secrets 已配**：`SSH_HOST=122.51.233.225`、`SSH_USER=liyongquan`、`SSH_PRIVATE_KEY`（专用 deploy key `ai_forum_deploy_ed25519`，**已装入服务器** `~/.ssh/authorized_keys`，验证可免密登录）。
- **基础镜像已预灌服务器**：`nginx:1.27-alpine`（74.5MB）+ `postgres:16-alpine`（420MB）已通过 `docker save | ssh docker load` 装上（因 Docker Hub 被墙，见 2.4）。
- **v3.1 执行基线文档已定稿**：`C:\Users\liyongquan\Downloads\ai-forum-部署方案-v3（8888容器化·执行基线）.md`——**这是唯一权威执行文档，本 handoff 是对它的导航，不重复内容**。
- 服务器 SSH 权限规则已加（settings.json 放行 ssh/scp/plink 到 122.51.233.225）。

### 2.2 已验证事实（服务器，2026-08-19 实测）
- 共享多租户阿里云服务器；`liyongquan` **不在 sudoers**（uid **1003**，gid 1005），在 `docker`/`nginx`/`devgroup` 组；**唯一 sudoer 是 `ubuntu`（uid 1000）**，无其密码。
- `/var/www`、`/opt` 均 root 所有，无 ai-forum 目录；宿主 nginx 仅出厂 default 站点（占 80）。
- `docker compose v2.32.4` 可用（docker 组），当前无容器运行。
- 8888 端口外部 TCP 可达（安全组已放行）。
- **Docker Hub 被墙**（`registry-1.docker.io` 超时；`ghcr.io`、`registry.cn-hangzhou.aliyuncs.com` 可达 401）。
- ⚠️ 账号密码 `Liyongquan@123` 已明文出现在会话 → **立即轮换**（执行清单 ②，用户做）。

### 2.3 已验证事实（代码，2026-08-19 实测）
- **repo 私有**（`isPrivate: true`）→ 服务器 clone/pull 需 read-only deploy key（执行清单 ①）。
- `backend/internal/httpapi/router.go`：`api := r.Group("/api/v1")`（**全部 API 挂 `/api/v1`**）、`r.GET("/healthz")`（**根**）、**无路径改写中间件** → nginx `proxy_pass` 必须**不带尾斜杠**（`http://api:8080;`），否则 `/api/v1/x`→`/v1/x` 全站 404。生产 api **不托管 `/uploads`**（仅 dev `r.Static`）。
- `backend/internal/config/config.go`：`DATABASE_URL` **必填**（缺则 `config: DATABASE_URL 未设置` 退出）；`APP_ENV` 缺省 `development`；`UPLOADS_DIR` 缺省 `""`（上传挂）。→ 服务器 `.env` 必须以 `.env.example` 为模板覆盖全部键。
- `compose.yml`（main）：api + postgres 两服务；api 已有 healthcheck/`restart: unless-stopped`/json-file 轮转；postgres **缺 healthcheck**（本次补）。api 挂载 `./uploads:/opt/ai-forum/uploads`。
- `backend/Dockerfile`：多阶段构建，终态仅一个二进制，固定 UID 1000（非 root），alpine 3.21。
- 前端 `BASE = '/api/v1'`（相对路径）；PWA 在纯 HTTP 下 SW 不注册（离线/安装失效，SPA 正常）——已记入 v3.1 §6 技术债。

### 2.4 三个关键决策（v3.1 已定，勿改）
1. **uploads 权限 = compose `user:` override**：api 服务 `user: "${APP_UID:-1000}:${APP_GID:-1000}"`，`.env` 设 `APP_UID=$(id -u)`/`APP_GID=$(id -g)`（=1003:1005）。原因：docker-chown 会让 uploads 属主变 **ubuntu**（uid 1000）+ alpine 拉不到。uploads 目录 0755/文件 0644（web nginx worker 非 root 需读）。
2. **基础镜像来源 = 一次性 docker save/load 预灌**（已完成）+ bootstrap/CI 改 `docker compose pull api` + `up -d`（不用 `pull` 全部）。
3. **GHCR 包 Public**（用户做）：`仓库 → Packages → ai-forum-api → Package settings → Visibility → Public`；或 `! gh auth refresh -h github.com -s read:packages write:packages` 后 agent 用 API 改。repo 保持私有。

### 2.5 Blocking
- **无代码阻塞**。等待项：① deploy key 未配（agent 可做）、② 密码轮换（用户）、⑤ GHCR Public（用户）——其中 ⑤ 是 bootstrap（`pull api`）的前置。

## 3. 待办：代码 PR（执行清单 ④）——把 v3.1 落成 repo 变更

**当前 git 状态**：分支 `feat/web-8888`（自 origin/main），`compose.yml` 有未提交的 web 服务改动（**缺 v3.1 的 user-override**，需补）。

PR 内容（一次合入，actionlint 兜底）：
1. **`compose.yml`**：提交 web 服务（nginx:1.27-alpine @8888，挂 frontend/dist/uploads/web.conf）+ **api 加 `user:` override**（v3.1）+ postgres 加 `pg_isready` healthcheck + web/api 的 `depends_on: condition: service_healthy`。
2. **`deploy/web.conf`**（新建）：见 v3.1 §3.2——`limit_req_zone`（login/register 限流）、`client_max_body_size 20m`、`proxy_pass http://api:8080;` **无尾斜杠**、`/uploads/` alias、SPA 回退。
3. **`deploy/bootstrap.sh`**（新建）：v3.1 §4 版本——GitHub SSH 自检 → SSH remote clone → 预建 `frontend/dist uploads` → 以 `.env.example` 为模板生成 .env（含 DATABASE_URL/APP_ENV/UPLOADS_DIR/APP_UID/GID）+ 必填键校验 → `docker compose pull api` + `up -d`。**无 chown 步**。
4. **`.github/workflows/deploy.yml`**：job 级 `concurrency`；上传/部署步 `if: env.SSH_HOST != '' && vars.DEPLOY_ENABLED == 'true'`；scp target → `/home/liyongquan/ai-forum/frontend/dist`；部署脚本 `cd ~/ai-forum` + `mkdir -p frontend/dist uploads` + `docker compose pull api` + `up -d` + `image prune -f`；更新过时注释。
5. **`backend/.dockerignore`**（新建）：排除 `.env*` 等（卫生项）。
6. **`docs/ops/deployment.md`**：同步 8888 容器结构、首部署（`~/ai-forum` 无 sudo）、bootstrap 指引；#8 决策备注写「共享服务器无 sudo → 容器 nginx @8888 变体」。
7. **地图 #1 更新**（并入 #8）：追加变体决策 + 基础设施状态；可在 PR 合入后或同步做。

**校验**：`bash scripts/check-workflows.sh`（actionlint，Docker）；`docker compose config` 语法；改完可先本地 `cd frontend && npm run build` 确认前端产物。

## 4. Skills to use (next session)

- 无特殊技能必需。本地校验：`bash scripts/check-workflows.sh`、`docker compose config`。
- 若做服务器操作：SSH 到 `liyongquan@122.51.233.225`（settings.json 已放行）。
- 提交规范：沿用仓库现有 git 流（PR → main）。不要用 `secrets` 进 `if`（红线，见 `docs/impl/testing.md` §4.1）。

## 5. Artifacts (reference only)

- **权威执行文档**：`C:\Users\liyongquan\Downloads\ai-forum-部署方案-v3（8888容器化·执行基线）.md`（v3.1，含全部细节/否决记录/执行清单）
- repo 文件：`.github/workflows/deploy.yml`、`compose.yml`、`backend/Dockerfile`、`backend/internal/httpapi/router.go`、`backend/internal/config/config.go`、`.env.example`、`docs/ops/deployment.md`
- GitHub：issue #1（地图）、#8（部署决策）、#14（前端 CI 部署，已合）；PR #15（已合）
- 上一评估：独立 agent 对 8888 方案的完整评估（结论：方案成立，含 uploads/限流/PWA 等增量建议，已并入 v3.1）
- 计划文件：`C:\Users\liyongquan\.claude\plans\elegant-zooming-pillow.md`
