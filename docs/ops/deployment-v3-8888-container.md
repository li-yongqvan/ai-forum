# ai-forum 部署方案 v3：共享服务器（无 sudo）8888 容器化 —— 执行基线

> 版本沿革：v1（2026-08-18）待评估简报 → v2（2026-08-19）评审定稿 → **v3 修复 v2 的两个硬阻塞（代码实测确认），本版为执行基线**。
> 仓库：`github.com/li-yongqvan/ai-forum`（私有，`isPrivate: true` 已实测）。

## 0. 版本改动摘要

### v3 相对 v2（阻塞修复，2026-08-19）

| 项目 | v2 | v3 |
|---|---|---|
| 服务器 `.env` 生成 | 手写 4 键子集（POSTGRES_USER/DB/PASSWORD、JWT_SECRET） | **以 `.env.example` 为模板生成全部键 + 必填键校验**（补 DATABASE_URL/APP_ENV/UPLOADS_DIR）。缺 DATABASE_URL → api 启动即挂（`config.go` L35 读取、L49 报错退出）；APP_ENV 缺省 development（gin 非 release + dev 态 Go 静态 `/uploads`）；UPLOADS_DIR 缺省 `""`（上传功能挂） |
| 服务器 git 凭证 | 「repo 可见性」开放项 | 实测 `isPrivate: true`，匿名 clone / `git pull` 必失败 → **read-only deploy key**，bootstrap 硬前置，clone 改 SSH remote |
| uploads chown 必要性 | 「uid 若 = 1000 可免，非阻塞」 | 实测 `liyongquan` uid **1003** ≠ 1000 → chown 为**必需步**（措辞修正） |

### v3.1 追加修正（2026-08-19 实测追加）

| 项目 | v3 | v3.1 |
|---|---|---|
| **基础镜像来源** | 服务器 `docker compose pull` 拉 Docker Hub | **实测 Docker Hub 被墙**（`registry-1.docker.io` 超时；`ghcr.io`/阿里云 registry 可达）→ 一次性 `docker save \| ssh docker load` 预灌 nginx/postgres 到服务器（**已执行**）；bootstrap/CI 改 `docker compose pull api` + `up -d`（本地已有则用本地） |
| uploads 写权限 | docker 一次性 chown（需 alpine:3） | **换 compose `user:` override**（api 以部署用户 1003:1005 运行，uploads 归 liyongquan）——观感干净（不落 ubuntu 属主）+ 去掉 alpine 依赖（Docker Hub 被墙拉不到） |
| `.env` | 无 APP_UID/GID | 新增 `APP_UID=$(id -u)` / `APP_GID=$(id -g)` |

### v3.2 追加修正（#43 服务器本地构建，2026-08-24）

| 项目 | v3.1 | v3.2 |
|---|---|---|
| **api 镜像构建** | CI 构建 → `docker save\|gzip` → scp → 服务器 `docker load`（A' 方案，deploy ~18min 物理瓶颈） | **服务器本地构建**（`git pull` + `docker compose build api`，go mod 走 goproxy.cn；deploy job ~4-5min）；CI 不再构建/推 GHCR（编译门 = test job `go build ./...`） |
| **前端交付** | scp `frontend/dist/*` 直传（2.9MB 累积旧哈希） | **tar.gz（~300KB）→ scp /tmp → 服务器解压**（保 inode 清旧） |
| **GOPROXY/GOSUMDB** | 无（CI 构建） | Dockerfile 加 `ARG GOPROXY/GOSUMDB` + BuildKit cache mount；服务器 `--build-arg GOPROXY=https://goproxy.cn,direct --build-arg GOSUMDB=sum.golang.google.cn`（proxy.golang.org 被墙实测） |
| **构建基础镜像预灌** | 仅 nginx/postgres | 追加 golang:1.26-alpine + alpine:3.21 + `docker/dockerfile:1`（builder/frontend，见 §7 执行顺序） |
| **应急回退** | — | `DEPLOY_MODE` 仓库 variable：默认 `server`（服务器构建）/ 应急 `scp`（A' 老路：CI build+save\|gzip→scp→load+up+健康等待），改变量 + Actions Re-run 即切，无需改代码/merge |
| **验收口径** | CI 部署步 ≤3min（未达成，跨洋 scp 物理瓶颈，#26 改连续 3 次零超时） | **deploy job ≤5min + 连续 3 次零超时**（评审 F5 决议） |

> 设计评审：`docs/plans/issue-43-deploy-optimization-设计文档.md`（有条件通过，0 阻塞）+ 评审意见书同目录。

### v2 相对 v1（评审定稿，2026-08-19）

| 项目 | v1 | v2 |
|---|---|---|
| uploads 写权限 | setfacl，fallback 777 | docker 一次性 chown（setfacl 与 777 均否决，见 §3.3） |
| web.conf proxy_pass | 带尾斜杠 | 一律无尾斜杠（Gin 全量 `/api/v1` + 根 `/healthz`，已确认，必须原样透传） |
| web.conf | 无 body 大小配置 | `client_max_body_size 20m`（默认 1m 图片上传全量 413） |
| GHCR | 已选 Public，待复核 | 维持 Public（终态镜像仅含二进制）；补 `backend/.dockerignore` 卫生项 |
| CI deploy job | 仅 `SSH_HOST != ''` 门控 | 加 `vars.DEPLOY_ENABLED` 门控 + `concurrency` 组 + `docker image prune -f` |
| 服务器初始化 | 手工步骤散落 | 固化 `deploy/bootstrap.sh` 进仓库 |
| 运维闭环 | 开放问题 | 备份 cron、postgres healthcheck、日志轮转（main 已有） |

## 1. 背景与约束

- ai-forum（AI 智联论坛）：社团内部移动端论坛 MVP。Vue 3 + Vite + Vant PWA（静态产物 nginx 托管）；Go + Gin 模块化单体；PostgreSQL 16。前端 API 基址相对路径 `/api/v1`（同源反代）。
- 部署目标：单服务器 Docker Compose all-in-one（决策 #8）。无域名、纯 HTTP（MVP）。
- 约束：服务器 `122.51.233.225`（阿里云）多人共享；`liyongquan` 不在 sudoers（uid **1003**）；在 `docker`、`nginx`、`devgroup` 组；唯一 sudoer 是 `ubuntu`（无密码）。
- **关键认知**：docker 组成员等价于 root（daemon 以 root 运行，可启动 root 身份容器做任意文件操作）。「无人能 root chown」实际不成立，uploads 权限因此有干净解（§3.3）。docker 发布 80 端口也不需要 sudo，80 的唯一障碍是宿主 nginx 已占用（§6）。

## 2. 已验证事实

### 2.1 服务器（2026-08-18 实测）

- `docker compose v2.32.4` 可用，当前无容器运行；SSH 密钥认证可用（CI deploy key 已装 `~/.ssh/authorized_keys`）。
- `/var/www`、`/opt` 均 root 所有，无 ai-forum 目录；宿主 nginx 仅出厂 default 站点（占 80）。
- 8888 端口外部 TCP 可达（安全组已放行）。
- GHCR 镜像私有，服务器匿名 `docker pull` 报 `unauthorized` → 改 Public（§3.4）。
- **Docker Hub 被墙**（实测 `registry-1.docker.io` 连接超时；`ghcr.io`、`registry.cn-hangzhou.aliyuncs.com` 可达 401）→ 基础镜像（nginx/postgres/alpine）无法从 Docker Hub 拉取，需一次性 `docker save|ssh docker load` 预灌（v3.1，**已执行**）。
- `id liyongquan` → **uid 1003**（≠ 1000）；**uid 1000 = `ubuntu`**（唯一 sudoer）。
- ⚠️ 账号密码 `Liyongquan@123` 已明文泄漏 → **立即轮换**（执行清单第 2 项）。

### 2.2 仓库与代码实测（2026-08-19）

- **仓库私有**：`isPrivate: true`。服务器匿名 `git clone` / CI 部署步的 `git pull origin main` 均会卡住 → read-only deploy key 为硬前置（§7 第 1 步）。
- **`backend/internal/config/config.go`**：`DATABASE_URL` 必填（L35 `os.Getenv`，L49 `config: DATABASE_URL 未设置` 即退出）；`APP_ENV` 缺省 `development`（gin 非 release，dev 态开 Go 静态 `/uploads`）；`UPLOADS_DIR` 缺省 `""`（上传功能挂）。→ 服务器 `.env` 必须覆盖全部键，`.env.example` 是唯一权威键清单。
- `compose.yml`（main）：api + postgres 两服务（web 未合入）。api 已有 healthcheck、`restart: unless-stopped`、json-file 日志轮转（10m×3）✓。api 挂载 `./uploads:/opt/ai-forum/uploads`（注意容器内路径）。postgres 发布 `127.0.0.1:5432`（仅回环，可接受）；`./pgdata` bind mount（官方镜像 entrypoint 自行 chown，无需 sudo）；postgres **缺 healthcheck**（本次补）。
- `deploy.yml`（main）：镜像 `latest` + `sha` 双标签 ✓；scp 目标仍为 `/var/www/ai-forum`、部署 `cd /opt/ai-forum`（旧 #8 路径，本次改）；无 `concurrency`。
- `backend/Dockerfile`：多阶段构建，**终态镜像仅 COPY 一个二进制**，固定 UID 1000，alpine 3.21。构建上下文的敏感文件不进发布镜像层——GHCR Public 实际暴露面只有二进制。
- 仓库根无 `.dockerignore`，`backend/` 无 `.dockerignore`，`deploy/` 目录不存在（`web.conf`、`bootstrap.sh` 为本次新增）。

## 3. 定稿方案

### 3.1 架构与 compose.yml（目标态，全量）

```yaml
# ai-forum 部署（#8 变体：共享服务器无 sudo → 容器 nginx @8888，并入 #8）
services:
  web:
    image: nginx:1.27-alpine
    container_name: ai-forum-web
    restart: unless-stopped
    ports:
      - "8888:80"
    volumes:
      - ./frontend/dist:/usr/share/nginx/html:ro   # CI scp 目标
      - ./uploads:/var/www/uploads:ro              # /uploads 直托（#12）
      - ./deploy/web.conf:/etc/nginx/conf.d/default.conf:ro
    networks: [ai-forum]
    depends_on:
      api:
        condition: service_healthy
    logging: &default-logging
      driver: json-file
      options:
        max-size: "10m"
        max-file: "3"

  api:
    image: ghcr.io/li-yongqvan/ai-forum-api:latest
    container_name: ai-forum-api
    restart: unless-stopped
    env_file: [.env]
    user: "${APP_UID:-1000}:${APP_GID:-1000}"      # v3.1：以部署用户 uid/gid 运行，uploads 归 liyongquan（.env 设 APP_UID/APP_GID）
    volumes:
      - ./uploads:/opt/ai-forum/uploads            # api 写入（以 APP_UID 运行）
    ports:
      - "127.0.0.1:8080:8080"                      # 仅回环，外部经 web 反代
    networks: [ai-forum]
    depends_on:
      postgres:
        condition: service_healthy
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:8080/healthz"]
      interval: 30s
      timeout: 5s
      retries: 3
      start_period: 10s
    logging: *default-logging

  postgres:
    image: postgres:16-alpine
    container_name: ai-forum-postgres
    restart: unless-stopped
    env_file: [.env]
    volumes:
      - ./pgdata:/var/lib/postgresql/data          # entrypoint 自行 chown，无需 sudo
    networks: [ai-forum]
    ports:
      - "127.0.0.1:5432:5432"                      # 仅回环调试用，不需要可删
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U $$POSTGRES_USER"]
      interval: 10s
      timeout: 5s
      retries: 5
    logging: *default-logging

networks:
  ai-forum:
    driver: bridge
```

已知限制（MVP 接受）：nginx 启动时解析 `api` 主机名，api 单独重启换 IP 时需 `docker compose restart web`；`depends_on: service_healthy` 覆盖正常启动顺序。resolver + 变量写法可解但属过度设计，不引入。

### 3.2 deploy/web.conf（定稿）

```nginx
# 认证撞库防护（#8 安全）：login/register 无应用内限流 + 8888 公网可达 + 开放注册
limit_req_zone $binary_remote_addr zone=auth_limit:10m rate=10r/m;

server {
    listen 80;
    server_name _;
    client_max_body_size 20m;   # #12 图片上传；默认 1m 会全量 413

    root /usr/share/nginx/html;
    index index.html;

    # Gin 全量挂 /api/v1（已确认），无尾斜杠 = 原样透传；
    # 带尾斜杠会把 /api/v1/x 改写成 /v1/x → 全站 404
    location /api/ {
        proxy_pass http://api:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }

    # 仅对 login/register 精确限流（避免误伤 /auth/me 等高频读）
    location = /api/v1/auth/login {
        limit_req zone=auth_limit burst=20 nodelay;
        proxy_pass http://api:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }
    location = /api/v1/auth/register {
        limit_req zone=auth_limit burst=20 nodelay;
        proxy_pass http://api:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }

    location = /healthz {       # /healthz 在根（已确认）
        proxy_pass http://api:8080;
    }

    # #12：生产 api 不托管 /uploads（仅 dev r.Static，已确认），web 直托，图片 GET 不经 Go
    # 前提：APP_ENV=production（见 §4 .env），否则 dev 态 Go 也会挂 /uploads
    location /uploads/ {
        alias /var/www/uploads/;
    }

    location / {                # SPA 回退
        try_files $uri $uri/ /index.html;
    }
}
```

后续可选：按 vite-plugin-pwa 实际产物文件名给 sw 脚本加 `no-cache` 头；静态资源加缓存头。

### 3.3 uploads 写权限（定稿：compose `user:` override，v3.1 弃 docker-chown）

```yaml
# compose.yml api 服务（§3.1）：覆盖容器运行用户
user: "${APP_UID:-1000}:${APP_GID:-1000}"
```
```bash
# .env（bootstrap 自动生成）
APP_UID=1003
APP_GID=1005
```

- 原理：api 容器以 liyongquan 的 uid/gid（1003:1005）运行，uploads 目录由 bootstrap 预建即归 liyongquan → **免 chown 步**。宿主机 `ls -la ~/ai-forum/uploads` 属主显示 **`liyongquan liyongquan`**（v3 的 docker-chown 会显示 **`ubuntu ubuntu`**，因 uid 1000 = ubuntu）。
- 权限：目录 0755（bootstrap mkdir 默认）、文件 0644（服务端写入）——web 的 nginx worker（非 root，uid 101）以 other 身份可读 ✓。
- **为何弃 docker-chown（v3.1）**：① 观感——论坛图片落 ubuntu（唯一 sudoer）属主名下，多租户下易混淆、不移植；② **Docker Hub 被墙**，alpine:3 拉不到，chown 步实测直接失败。user-override 一并去掉 alpine 依赖。
- **setfacl 否决**：依赖 acl 包（无 sudo 装不上）与文件系统挂载支持，语义绕，方案太脆。
- **777 否决**：共享机上其他本地用户可任意写入/删除 `/uploads`；投放 `.html` 会被 nginx 按 text/html 同源直出 → 存储型 XSS + token 失窃链路，非理论风险。

### 3.4 GHCR Public（确认：维持）

- 实际暴露面：公开的是**镜像**而非源码仓库；package 可见性与 repo 独立，repo 保持私有。终态镜像仅含编译后二进制（§2.2），无源码、无配置层。
- 义务项：补 `backend/.dockerignore`（排除 `.env*` 等）——当前多阶段构建下不构成泄漏，纯卫生 + 防未来 Dockerfile 改动失误。
- 备选已评估不采用：只读 PAT + `docker login`（成本相同、保持私有，多 PAT 续期负担）；服务器本地构建镜像（私有 repo 需凭证 + 架空 CI）。

### 3.5 CI 变更（deploy.yml 改动点）

```yaml
# job 级新增
concurrency:
  group: deploy-prod
  cancel-in-progress: false

# deploy job 各上传/部署步 if 改为：
if: env.SSH_HOST != '' && vars.DEPLOY_ENABLED == 'true'

# scp 步：
target: /home/liyongquan/ai-forum/frontend/dist   # 原 /var/www/ai-forum
# （绝对路径：appleboy/scp-action 对 ~ 的展开不可靠）

# ssh 部署步 script 改为（git pull 走 SSH remote，依赖 §7 第 1 步的 deploy key）：
script: |
  set -e
  cd ~/ai-forum
  git pull origin main
  mkdir -p frontend/dist uploads   # 自愈：目录被误删时先以部署用户重建，防 docker 以 root 重建后 scp 写不进
  docker compose pull api          # v3.1：仅 api 走 GHCR（可达）；nginx/postgres 已预灌本地，up -d 用本地镜像
  docker compose up -d
  docker image prune -f
  docker compose ps
```

- `DEPLOY_ENABLED` 为仓库 variable：服务器 bootstrap 完成后置 `true`——解决「代码合了、服务器没跟上」的 CI 红污染空窗。
- `concurrency` 防两次 push 部署互撞。
- 删除/更新过时注释（「已 chown /var/www/ai-forum」等）。
- **v3.2 覆盖（#43）**：deploy job 双模式——默认 `DEPLOY_MODE=server`：前端 `npm ci+build` → `tar.gz` → scp → ssh `git pull --ff-only && bash deploy/deploy-server.sh`（服务器 `docker compose build --build-arg GOPROXY=goproxy.cn` → up → 健康轮询/回退 → 前端换装 → reload）；应急 `DEPLOY_MODE=scp`：追加 CI build+`save|gzip` → scp → 服务器 `load`+up+健康等待（A' 老路）。CI 不再推 GHCR（`permissions` 收窄 `contents: read`）。实现详见 `deploy/deploy-server.sh` 与 `docs/plans/issue-43-deploy-optimization-设计文档.md`。

## 4. 服务器 bootstrap（deploy/bootstrap.sh，进仓库）

```bash
#!/usr/bin/env bash
# ai-forum 服务器初始化：仅需 docker 组 + 家目录，全程无 sudo
# 硬前置：repo 私有，read-only deploy key 已装 ~/.ssh（执行清单第 1 步）
set -euo pipefail
APP_DIR="$HOME/ai-forum"

# 0. GitHub SSH 凭证自检（匿名 clone 私有 repo 必失败，先 fail fast）
ssh -T git@github.com 2>&1 | grep -q "successfully authenticated" \
  || { echo "FATAL: GitHub SSH 认证失败，先按执行清单第 1 步配 deploy key" >&2; exit 1; }

# 1. 代码（SSH remote；后续 CI 的 git pull 同路）
[ -d "$APP_DIR" ] || git clone git@github.com:li-yongqvan/ai-forum.git "$APP_DIR"
cd "$APP_DIR"

# 2. 预建挂载点：若不存在，docker 会以 root 自动创建，之后 scp/写入全部失败
mkdir -p frontend/dist uploads "$HOME/backups"

# 3. .env：以 .env.example 为模板（唯一权威键清单）就地填充，不手写子集
if [ ! -f .env ]; then
  PG_PASS=$(openssl rand -hex 32)
  JWT=$(openssl rand -hex 32)
  cp .env.example .env
  sed -i \
    -e "s|^POSTGRES_USER=.*|POSTGRES_USER=forum|" \
    -e "s|^POSTGRES_DB=.*|POSTGRES_DB=forum|" \
    -e "s|^POSTGRES_PASSWORD=.*|POSTGRES_PASSWORD=${PG_PASS}|" \
    -e "s|^DATABASE_URL=.*|DATABASE_URL=postgres://forum:${PG_PASS}@postgres:5432/forum?sslmode=disable|" \
    -e "s|^JWT_SECRET=.*|JWT_SECRET=${JWT}|" \
    -e "s|^APP_ENV=.*|APP_ENV=production|" \
    -e "s|^UPLOADS_DIR=.*|UPLOADS_DIR=/opt/ai-forum/uploads|" \
    .env
  chmod 600 .env
  # v3.1：compose `user:` override 所需的运行 uid/gid（部署用户 = liyongquan）
  printf '\nAPP_UID=%s\nAPP_GID=%s\n' "$(id -u)" "$(id -g)" >> .env
  # 必填键校验：模板键名漂移时 fail fast，不让 api 起来即挂（config.go L49）
  for k in DATABASE_URL POSTGRES_USER POSTGRES_DB POSTGRES_PASSWORD JWT_SECRET APP_ENV UPLOADS_DIR APP_UID APP_GID; do
    grep -q "^${k}=." .env || { echo "FATAL: .env 缺少有效 $k（对照 .env.example 键名）" >&2; exit 1; }
  done
fi

# 4. 拉起（v3.1：仅 api 走 GHCR 可达源；nginx/postgres 已预灌本地，见执行清单）
docker compose pull api
docker compose up -d
docker compose ps
```

要点：
- **v3.2 覆盖（#43）**：仓库内 `deploy/bootstrap.sh` 已把「4. 拉起」的 `docker compose pull api` 改为 `docker compose build --build-arg GOPROXY=https://goproxy.cn,direct --build-arg GOSUMDB=sum.golang.google.cn api`（compose 加 `build:` 段后 `pull` 会拉 GHCR 过期镜像、裸 `up` 会因默认 GOPROXY 不可达而失败）。**前置：golang:1.26-alpine + alpine:3.21 + docker/dockerfile:1 已预灌**（见 §7 执行顺序）。
- `DATABASE_URL` 与 `POSTGRES_USER/DB/PASSWORD` 同源生成，避免两处不一致；host 用 compose 服务名 `postgres`（`sslmode=disable` 为同机桥接网络内连接）。
- `APP_ENV=production`：gin release 模式 + 关闭 dev 态 Go 静态 `/uploads`（否则与 §3.2 的 web 直托语义冲突）。
- `UPLOADS_DIR=/opt/ai-forum/uploads`：与 compose 中 api 的容器内挂载路径一致（§2.2 实测）。
- `APP_UID`/`APP_GID`：由 `id -u`/`id -g` 生成（部署用户 = 1003:1005），供 compose `user:` override（§3.3）。
- 基础镜像（nginx/postgres）不在 bootstrap 内拉取：Docker Hub 被墙，已按执行清单预灌本地（§7）。
- `.env` 已存在则跳过生成（幂等，重跑不覆盖已轮换的密钥）。

## 5. 运维闭环（定稿）

- **备份**：`crontab -e` 用 liyongquan 自己的 cron（无需 sudo）：

```cron
# 每晚 3 点 pg_dump，保留 14 份
0 3 * * * cd ~/ai-forum && docker compose exec -T postgres pg_dump -U forum forum | gzip > ~/backups/forum-$(date +\%F).sql.gz && chmod 600 ~/backups/forum-*.sql.gz && ls -1t ~/backups/forum-*.sql.gz | tail -n +15 | xargs -r rm
# 每周日打包 uploads + .env（.env 含 JWT_SECRET/DB 密码，恢复必需），保留 8 份
0 4 * * 0 tar -czf ~/backups/uploads-$(date +\%F).tar.gz -C ~/ai-forum uploads .env && chmod 600 ~/backups/uploads-*.tar.gz && ls -1t ~/backups/uploads-*.tar.gz | tail -n +9 | xargs -r rm
```

  单点机器，建议再定期把 dump 拉离本机（本地下载或对象存储）。
- **日志**：compose 已配 json-file 轮转（10m×3，main 现状 ✓，web 同配置）。
- **健康检查**：api 已有 ✓；postgres 补 `pg_isready`；api/web 的 `depends_on` 均 `condition: service_healthy`。
- **重启恢复**：三服务均 `restart: unless-stopped`；宿主机重启后由 dockerd 自启拉起（首次验证一次 `docker compose ps`）。
- **磁盘卫生**：deploy 脚本尾部 `docker image prune -f`（§3.5）；v3.2 起服务器本地构建，BuildKit cache 是提速来源（`image prune -f` 不清它，属特性），**勿无脑 `builder prune`**——仅当磁盘占用 >5G 时 `docker builder prune --keep-storage 4G -f`（删 module cache 得不偿失）。

## 6. 与既有决策的关系及长期演进

- **#8**：原决策（宿主 nginx @80）因共享服务器无 sudo 不可行 → 本方案为条件分支变体（容器 nginx @8888），并入 #8 更新，不另开 ticket。
- **#14**：web root 从 `/var/www/ai-forum` 换为 `~/ai-forum/frontend/dist`，托管方从宿主 nginx 换为容器 nginx；CI 构建验证 + scp 上传主流程不变。
- **#12**：`/uploads` 由 web 容器 alias 同一宿主 `./uploads`（api 写入、web 读出）；生产 api 不托管静态上传（`APP_ENV=production` 保证）。
- **PWA 在纯 HTTP 下失效**：Service Worker 需 secure context（HTTPS 或 localhost），`http://122.51.233.225:8888/` 下 SW 不会注册 → 无离线缓存/安装提示/推送；SPA 本身正常。这是 #8「无域名纯 HTTP」决策的用户可见后果，随 HTTPS（长期演进 3）一并解决。
- **长期演进**（记入 #8 技术债备注）：
  1. 现状 8888 即正确形态；80 的障碍仅是宿主 nginx 占用（docker 绑 80 本身不需 sudo）。
  2. 迁回 80：请 ubuntu/opsadmin 配合一次——停 default 站点后宿主 nginx 反代 `127.0.0.1:8888`，或直接释放 80 给 web 容器。
  3. HTTPS 的前提是**域名**（纯 IP 拿不到可用的公网证书）；有域名后长期形态为域名 + 443（可容器化 Caddy 自动签）。

## 7. 执行顺序

1. **配 repo read-only deploy key**（bootstrap 硬前置；repo 实测私有）：

```bash
# 服务器（liyongquan）：
ssh-keygen -t ed25519 -f ~/.ssh/ai-forum-repo -N "" -C "ai-forum-repo-readonly"
cat ~/.ssh/ai-forum-repo.pub
# GitHub → repo Settings → Deploy keys → Add deploy key（不勾 Allow write access）
# 服务器 ~/.ssh/config 追加：
#   Host github.com
#     IdentityFile ~/.ssh/ai-forum-repo
#     IdentitiesOnly yes
```

2. **轮换泄漏密码** `Liyongquan@123`；`POSTGRES_PASSWORD`/`JWT_SECRET` 由 bootstrap 随机生成，天然不复用。
3. 仓库设置加 variable `DEPLOY_ENABLED`（留空或 `false`）。
4. PR 合入：compose 加 web 服务 + postgres healthcheck、`deploy/web.conf`、`deploy/bootstrap.sh`、deploy.yml 三处改动（§3.5）、`backend/.dockerignore`。
5. **GHCR package 改 Public**（GitHub 网页操作，repo 保持私有）——**必须在 bootstrap 之前**，否则 bootstrap 的 `docker compose pull` 拉私有 api 镜像报 `unauthorized`，`set -e` 直接退出。
6. 服务器跑 `bootstrap.sh`（自检 deploy key → clone → 预建目录 → 模板生成 .env + 校验 → 首拉起）。
   - **前置：基础镜像已预灌**——在能连 Docker Hub 的机器 `docker pull nginx:1.27-alpine postgres:16-alpine`，再 `docker save nginx:1.27-alpine postgres:16-alpine | ssh liyongquan@122.51.233.225 docker load`（**2026-08-19 已执行**，服务器本地已有，见 §2.1）。
   - **v3.2 追加前置（#43，2026-08-24）**：服务器本地构建还需预灌 `golang:1.26-alpine` + `alpine:3.21` + `docker/dockerfile:1`（builder/frontend；Docker Hub 被墙，本机若拉不到走镜像源 `docker pull m.daocloud.io/docker.io/library/golang:1.26-alpine` 再 retag）；并在 merge 前做一次**试构建**（`cd ~/ai-forum && docker compose build --build-arg GOPROXY=https://goproxy.cn,direct --build-arg GOSUMDB=sum.golang.google.cn api`，实测耗时/内存、确认 frontend 免拉）。
7. `DEPLOY_ENABLED` 置 `true`，push main 做全链路验证：`http://122.51.233.225:8888/healthz` + 前端 + 发图。
8. 配 §5 备份 cron。

**开放项**（唯一剩余）：备份异地存放方案（有对象存储/另一台机器再补）。

## 8. 否决记录（评审留存）

| 方案 | 否决理由 |
|---|---|
| uploads `chmod 777` | 多租户任意写 → 同源存储型 XSS 链路（§3.3） |
| uploads `setfacl` | 依赖 acl 包（无 sudo 装不上），语义绕，方案太脆 |
| `.env` 手写键子集 | 缺 DATABASE_URL api 启动即挂（config.go L49）；以 `.env.example` 为模板 + 校验替代 |
| 服务器 HTTPS 匿名 clone | repo 实测私有，必失败；read-only deploy key + SSH remote 替代 |
| GHCR 改只读 PAT | 与 Public 成本相同，多 PAT 续期负担；社团项目公开无所谓 |
| 服务器本地构建镜像 | **v3.1 否决 → v3.2 采纳（#43）**：①私有 repo 凭证已由现有 read-only deploy key 解决（§2.2）；②「架空 CI」重估——test job `go build` 保留编译门、CI 仍做前端构建验证+门控+触发部署，未被架空；③A' 跨洋 scp ~12KB/s 已成物理瓶颈（18min）。实施见 §0 v3.2 与 `docs/plans/issue-43-deploy-optimization-设计文档.md` |
| rootless docker | 已有 docker 组（root 等价），多此一举 |
| Gin 直托静态/+api 对外 | 推翻 #12/#14 的 nginx 托管设计，收益不成比例 |
| 宿主 nginx 反代 8888 | 改宿主配置需 sudo；列为长期演进（§6）而非现方案 |
| nginx resolver + 变量上游 | 可解 api 重启换 IP，但 MVP 过度设计；`restart web` 足够 |
| uploads docker 一次性 chown（v3） | 观感（uploads 属主 = ubuntu）+ Docker Hub 被墙 alpine 拉不到；换 compose `user:` override（§3.3） |
| 服务器直拉 Docker Hub 基础镜像 | Docker Hub 被墙（实测超时）；一次性 `docker save\|ssh docker load` 预灌 + `docker compose pull api` 替代（§3.5/§4） |
