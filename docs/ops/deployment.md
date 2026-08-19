# ai-forum 部署与运维

> 对应 issue #8 决策。当前线上形态为 **8888 容器化变体**（共享服务器无 sudo → 容器 nginx @8888），完整决策基线见 [deployment-v3-8888-container.md](./deployment-v3-8888-container.md)，本文件为运维速查。
>
> **重要前提**：MVP 阶段使用 `http://122.51.233.225:8888/`，无域名、无 HTTPS。后果：登录凭据明文传输（安全债）；PWA Service Worker 需 secure context（HTTPS/localhost）→ SW 不注册、离线缓存/安装提示失效，SPA 正常。开放公测前必须购买域名并启用 HTTPS（演进见 v3.1 §6）。

---

## TL;DR

| 组件 | 方案 |
|---|---|
| 部署形态 | Docker Compose all-in-one（**容器 nginx @8888**，共享服务器无 sudo 变体，并入 #8） |
| 反向代理 | **容器 nginx**（web 服务 `nginx:1.27-alpine`）监听 `:8888` → 反代 `api:8080`；无宿主 nginx、无 sudo |
| 前端 | Vue 3 + Vite PWA 静态产物，web 容器托管（挂载 `~/ai-forum/frontend/dist`） |
| 后端 | Go + Gin 单进程容器，监听 `:8080`（仅回环 `127.0.0.1:8080`） |
| 数据库 | PostgreSQL 16 容器，数据卷 `./pgdata`（官方镜像 entrypoint 自行 chown，无 sudo） |
| CI/CD | GitHub Actions：构建 API 镜像推 GHCR + 前端 scp `~/ai-forum/frontend/dist`；SSH 直连 `docker compose pull && up -d`；`DEPLOY_ENABLED` variable 门控上传/部署步 |
| 备份 | liyongquan 用户 cron：每日 `pg_dump` 保留 14 份 + 每周 uploads/.env tar 保留 8 份 |
| 日志 | Docker `json-file` 轮转（10m×3） |
| 监控 | `/healthz`（web 反代 api）+ 外部 uptime / cron curl |

---

## 1. 服务器结构

```text
┌────────────────────────────────────────────────────────────┐
│  用户 / App                                                  │
│        http://122.51.233.225:8888/                          │
└──────────────────────────┬─────────────────────────────────┘
                           ▼
┌────────────────────────────────────────────────────────────┐
│  Docker Compose 网络（bridge: ai-forum）                     │
│  ┌───────────────────────┐                                  │
│  │  web（容器 nginx）     │  • :8888 → 80                    │
│  │  nginx:1.27-alpine    │  • /api/ → proxy_pass api:8080    │
│  │  挂载 frontend/dist   │  • /uploads/ → alias              │
│  │       + uploads +     │  • limit_req login/register       │
│  │       web.conf        │  • SPA 回退                        │
│  └──────────┬────────────┘                                  │
│             │ 127.0.0.1:8080 仅回环                           │
│  ┌──────────▼────────────┐    ┌──────────────────────────┐  │
│  │  api（Go + Gin）      │◄──►│  postgres (PG 16)        │  │
│  │  ghcr.io/...:latest   │    │  127.0.0.1:5432          │  │
│  │  user: 1003:1005      │    │  ./pgdata                │  │
│  │  ./uploads:rw         │    │  pg_isready healthcheck  │  │
│  └───────────────────────┘    └──────────────────────────┘  │
└────────────────────────────────────────────────────────────┘
```

要点：
- **共享服务器无 sudo**（liyongquan uid 1003，唯一 sudoer 是 `ubuntu`）→ 容器 nginx @8888 变体，避开宿主 nginx（占 80）与 `/opt`/`/var/www` 的 root 权限。
- api 以 `user: ${APP_UID}:${APP_GID}`（=1003:1005）运行，uploads 归 liyongquan，免 chown。
- 基础镜像（nginx/postgres）已预灌服务器本地（Docker Hub 被墙），CI/bootstrap 仅 `pull api`（GHCR 可达）。
- 8888 端口外部 TCP 可达（安全组已放行）。

---

## 2. 基础设施清单

| 项 | 状态 | 说明 |
|---|---|---|
| 服务器 SSH（CI 用） | ✓ | Actions secrets `SSH_HOST/SSH_USER/SSH_PRIVATE_KEY`；公钥在服务器 `~/.ssh/authorized_keys` |
| repo read-only deploy key | ✓ | 服务器 `~/.ssh/ai-forum-repo` + `~/.ssh/config`（Host github.com）；GitHub 侧同名 deploy key（只读） |
| 基础镜像预灌 | ✓ | `nginx:1.27-alpine`、`postgres:16-alpine` 已在服务器（`docker save\|ssh docker load`） |
| GHCR api 镜像 | Public | `ghcr.io/li-yongqvan/ai-forum-api`（repo 保持私有，仅公开镜像包） |
| GitHub variable | `DEPLOY_ENABLED=false` | 服务器 bootstrap 完成后置 `true` |
| GitHub secrets | ✓ | `SSH_HOST`、`SSH_USER`、`SSH_PRIVATE_KEY` |

---

## 3. 配置

### 3.1 `compose.yml`

仓库根，三服务 `web`（容器 nginx @8888）+ `api` + `postgres`。与 v3.1 §3.1 一致（api 的 `user:` override、postgres `pg_isready` healthcheck、web/api 的 `depends_on: condition: service_healthy`）。以仓库实际文件为准。

### 3.2 `deploy/web.conf`（容器 nginx 配置）

web 容器挂载到 `/etc/nginx/conf.d/default.conf`。关键项：
- `proxy_pass http://api:8080;` **无尾斜杠**——Gin 全量挂 `/api/v1`，带尾斜杠会把 `/api/v1/x` 改写成 `/v1/x` → 全站 404。
- `limit_req_zone` 对 login/register 精确限流（10r/m，burst 20）——认证撞库防护。
- `client_max_body_size 20m`（图片上传）。
- `/uploads/` alias `/var/www/uploads/`（web 直托，图片 GET 不经 Go）。

### 3.3 `.env.example` / `.env`

`.env.example` 是唯一权威键清单，服务器 `.env` 由 bootstrap 以模板生成（含 `DATABASE_URL`、`APP_ENV=production`、`UPLOADS_DIR`、追加 `APP_UID/APP_GID`），并有必填键校验。缺 `DATABASE_URL` api 启动即挂（config.go L49）。

### 3.4 `deploy/bootstrap.sh`

服务器初始化一键脚本（进仓库）：GitHub SSH 自检 → SSH remote clone → 预建 `frontend/dist uploads ~/backups` → 模板生成 `.env` → `docker compose pull api` + `up -d`。幂等（`.env` 已存在则跳过）。**无 chown、无 sudo**。

---

## 4. 首次部署（共享服务器无 sudo）

1. 前置：read-only deploy key（§2）+ 基础镜像预灌 + GHCR api 镜像 Public + `DEPLOY_ENABLED=false`。
2. 把 `deploy/bootstrap.sh` 拷到服务器，执行：
   ```bash
   bash ~/bootstrap-8888.sh
   ```
3. 验证：
   ```bash
   docker compose ps          # 三容器 healthy
   curl http://122.51.233.225:8888/healthz
   ```
4. GitHub 仓库置 `DEPLOY_ENABLED=true`，push main 触发 CI 全链路部署（前端 scp + `up -d`）。

后续前端/后端更新：push main 自动部署（deploy.yml，`DEPLOY_ENABLED=true` 时上传/部署步生效）。

---

## 5. CI/CD：GitHub Actions

### 5.1 `.github/workflows/deploy.yml`

`push main` 触发。deploy job：
- **始终**：前端 npm ci + build（构建验证）；构建并推 API 镜像到 GHCR（`latest` + `sha` 双标签）。
- **门控**（`if: env.SSH_HOST != '' && vars.DEPLOY_ENABLED == 'true'`）：scp 前端到 `~/ai-forum/frontend/dist`；SSH 执行 `git pull → mkdir -p frontend/dist uploads → docker compose pull api → up -d → image prune`。
- job 级 `concurrency: group: deploy-prod` 防互撞。
- secrets 不进 `if`（三段式：secrets → job env → step if 判 env，见 docs/impl/testing.md §4.1）。

### 5.2 需配置的 GitHub Secrets / Variables

| 项 | 值 |
|---|---|
| Secret `SSH_HOST` | `122.51.233.225` |
| Secret `SSH_USER` | `liyongquan` |
| Secret `SSH_PRIVATE_KEY` | CI deploy key 私钥（公钥在服务器 `~/.ssh/authorized_keys`） |
| Variable `DEPLOY_ENABLED` | `false`（bootstrap 完成后 `true`） |

---

## 6. 日志与监控

### 6.1 日志

- 三服务均 `json-file` 轮转（10m×3）。`docker logs -f ai-forum-api` 等。
- `docker image prune -f` 由部署脚本定期执行。

### 6.2 监控

- `/healthz`（经 web 反代 api）返回 200 即健康。
- 免费外部监控（UptimeRobot / Better Stack / 阿里云拨测）访问 `http://122.51.233.225:8888/healthz`。

---

## 7. 备份与恢复

备份 cron（liyongquan 用户，`crontab -e`，见 v3.1 §5）：
- 每日 3 点 `pg_dump` → `~/backups/forum-$(date +%F).sql.gz`，保留 14 份。
- 每周日 4 点 tar `uploads + .env`（含 JWT_SECRET/DB 密码，恢复必需）→ `~/backups/uploads-*.tar.gz`，保留 8 份。
- 建议定期把 dump 拉离本机（本地下载或对象存储，当前为开放项）。

恢复：
```bash
cd ~/ai-forum
docker compose stop api
docker compose exec postgres dropdb -U forum forum || true
docker compose exec postgres createdb -U forum forum
zcat ~/backups/forum-YYYY-MM-DD.sql.gz | docker compose exec -T postgres psql -U forum forum
docker compose start api
```

---

## 8. 已知技术债与后续演进

| 当前决策 | 技术债 | 触发升级条件 |
|---|---|---|
| 无域名，纯 HTTP @8888 | 登录凭据明文；**PWA SW 不注册**（离线/安装失效，SPA 正常） | 开放公测/真实用户前必须购买域名 + HTTPS（可容器化 Caddy 自动签） |
| 容器 nginx @8888（80 被宿主 nginx 占用） | 端口非标准 | 请 ubuntu/opsadmin 配合：停 default 站点后宿主 nginx 反代 `127.0.0.1:8888`，或释放 80 给 web 容器（docker 绑 80 不需 sudo） |
| GitHub Actions SSH 部署 | SSH 私钥存 GitHub Secrets | 团队扩大/多环境时改 webhook + watchtower 或 ArgoCD |
| 单服务器 | 无高可用 | 用户量增长/需 SLA 时多机 + LB |
| 无 Redis | 进程内 goroutine 异步 | 通知队列积压时引入 Redis/消息队列 |

---

## 9. 决策演进

- **#8 变体**：原决策（宿主 nginx @80）因共享服务器无 sudo 不可行 → **容器 nginx @8888 变体**，并入 #8（不另开 ticket）。决策基线：`deployment-v3-8888-container.md`（v3.1）。
- 长期形态（v3.1 §6）：域名 + 443（Caddy 自动签 HTTPS）；80 的障碍仅是宿主 nginx 占用，迁回 80 为一次性 ops 配合。
