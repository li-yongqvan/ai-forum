# ai-forum MVP 部署与运维方案

> 本方案对应 issue #8 决议。目标：在单台已有 nginx 的服务器上，用 Docker Compose 跑起 AI 智联论坛 MVP。
>
> **重要前提**：MVP 阶段使用 `http://122.51.233.225/` 访问，无域名、无 HTTPS。该决策出于「当前重点是功能验证，安全债后续补」。若后续开放公测或处理真实用户数据，必须购买域名并启用 HTTPS。

---

## TL;DR

| 组件 | 方案 |
|---|---|
| 部署形态 | Docker Compose all-in-one |
| 反向代理 | 复用服务器现有 **nginx**（#2 研究推荐 Caddy，但目标服务器已装 nginx，MVP 复用以减少迁移风险） |
| 前端 | Vue 3 + Vite PWA 静态产物，由 nginx 直接托管 |
| 后端 | Go + Gin 单进程容器，监听 `:8080` |
| 数据库 | PostgreSQL 16 容器，数据卷持久化到宿主机 |
| 缓存/消息队列 | **不引入 Redis / RabbitMQ / Kafka**，异步用进程内 goroutine |
| CI/CD | GitHub Actions SSH 直连服务器，执行 `docker compose pull && up -d` |
| 备份 | 宿主机 cron 每日 `pg_dump` |
| 日志 | stdout/stderr + Docker `json-file` |
| 监控 | `/healthz` + 免费外部 uptime 监控或 cron curl |

---

## 1. 服务器结构

```text
┌─────────────────────────────────────────────────────────────┐
│  用户 / App                                                  │
│         http://122.51.233.225/                               │
└─────────────────────────┬───────────────────────────────────┘
                          ▼
┌─────────────────────────────────────────────────────────────┐
│  nginx（宿主机已安装）                                        │
│  • 80 端口监听                                               │
│  • /api/*  ──►  http://localhost:8080  (Go API)              │
│  • 其它路径 ──►  /var/www/ai-forum/  (前端静态文件)            │
└─────────────────────────┬───────────────────────────────────┘
                          ▼
┌─────────────────────────────────────────────────────────────┐
│  Docker Compose 网络                                          │
│  ┌─────────────────┐      ┌─────────────────────────────┐   │
│  │  ai-forum-api   │      │  ai-forum-postgres          │   │
│  │  Go + Gin       │◄────►│  PostgreSQL 16              │   │
│  │  port 8080      │      │  port 5432                  │   │
│  │  (容器)          │      │  数据卷 ./pgdata:/var/lib/...│  │
│  └─────────────────┘      └─────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

---

## 2. 前置条件

- 一台 Ubuntu 服务器，已安装 nginx（当前 `122.51.233.225` 已满足）。
- 服务器已安装 Docker + Docker Compose（v2）。
- GitHub 仓库已初始化（当前尚未初始化，后续替换占位符）。
- 服务器 SSH 私钥已配置到 GitHub Secrets：`SSH_HOST`、`SSH_USER`、`SSH_PRIVATE_KEY`。

---

## 3. 配置文件

### 3.1 `compose.yml`

放在仓库根目录。

```yaml
services:
  api:
    image: ghcr.io/li-yongqvan/ai-forum-api:latest
    container_name: ai-forum-api
    restart: unless-stopped
    env_file:
      - .env
    ports:
      - "127.0.0.1:8080:8080"
    networks:
      - ai-forum
    depends_on:
      - postgres
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:8080/healthz"]
      interval: 30s
      timeout: 5s
      retries: 3
      start_period: 10s

  postgres:
    image: postgres:16-alpine
    container_name: ai-forum-postgres
    restart: unless-stopped
    env_file:
      - .env
    volumes:
      - ./pgdata:/var/lib/postgresql/data
    networks:
      - ai-forum
    ports:
      - "127.0.0.1:5432:5432"

networks:
  ai-forum:
    driver: bridge
```

> 注意：`api` 只绑定 `127.0.0.1:8080`，避免直接暴露在公网；所有外部流量经 nginx 进入。

### 3.2 nginx 站点配置

路径：`/etc/nginx/sites-available/ai-forum`

```nginx
server {
    listen 80;
    server_name 122.51.233.225;

    root /var/www/ai-forum;
    index index.html;

    # 前端静态文件 + 前端路由回退
    location / {
        try_files $uri $uri/ /index.html;
    }

    # API 反向代理到本机 Go 容器
    location /api/ {
        proxy_pass http://127.0.0.1:8080/;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # 健康检查（可选，供外部监控使用）
    location /healthz {
        proxy_pass http://127.0.0.1:8080/healthz;
    }

    # 上传图片（#12 D1：nginx 直接托管，图片 GET 不经 Go）
    location /uploads/ {
        alias /opt/ai-forum/uploads/;
        expires 30d;
    }
}
```

启用配置：

```bash
sudo ln -s /etc/nginx/sites-available/ai-forum /etc/nginx/sites-enabled/
sudo nginx -t
sudo systemctl reload nginx
```

### 3.3 `.env.example`

仓库中提供示例，服务器上复制为 `.env` 并填写真实值。

```bash
# 应用
APP_ENV=production
APP_PORT=8080
DATABASE_URL=postgres://afuser:afpass@postgres:5432/aiforum?sslmode=disable

# PostgreSQL
POSTGRES_USER=afuser
POSTGRES_PASSWORD=afpass
POSTGRES_DB=aiforum

# JWT / 加密（请生成强随机字符串）
JWT_SECRET=change-me-in-production

# 图片上传（#12；以下为默认值，可省略）
UPLOADS_DIR=/opt/ai-forum/uploads
UPLOAD_MAX_BYTES=5242880
```

### 3.4 `backup.sh`

放在服务器 `/opt/ai-forum/backup.sh`，每日 cron 执行。

```bash
#!/bin/bash
set -euo pipefail

PROJECT_DIR="/opt/ai-forum"
BACKUP_DIR="/opt/ai-forum/backups"
DB_NAME="aiforum"
DB_USER="afuser"
DATE=$(date +%Y%m%d_%H%M%S)
FILE="${BACKUP_DIR}/aiforum_${DATE}.sql"

mkdir -p "${BACKUP_DIR}"
cd "${PROJECT_DIR}"

docker compose exec -T postgres pg_dump -U "${DB_USER}" "${DB_NAME}" > "${FILE}"
gzip "${FILE}"

# 保留最近 7 天
find "${BACKUP_DIR}" -type f -name '*.sql.gz' -mtime +7 -delete

echo "Backup done: ${FILE}.gz"
```

添加 cron：

```bash
chmod +x /opt/ai-forum/backup.sh
# 每天凌晨 3 点备份
(crontab -l 2>/dev/null; echo "0 3 * * * /opt/ai-forum/backup.sh >> /var/log/ai-forum-backup.log 2>&1") | crontab -
```

---

## 4. 首次部署步骤

1. 在服务器上准备目录：
   ```bash
   sudo mkdir -p /opt/ai-forum
   sudo chown $USER:$USER /opt/ai-forum
   # 上传目录：容器 app 用户 UID=1000 需要写权限（Dockerfile 已固定，#12）
   sudo mkdir -p /opt/ai-forum/uploads
   sudo chown 1000:1000 /opt/ai-forum/uploads
   cd /opt/ai-forum
   ```

2. 首次手动 clone 仓库（后续由 Actions 更新）：
   ```bash
   git clone https://github.com/li-yongqvan/ai-forum.git .
   ```

3. 配置环境变量：
   ```bash
   cp .env.example .env
   # 编辑 .env，修改 POSTGRES_PASSWORD 和 JWT_SECRET
   ```

4. 配置 nginx（见 3.2）。

5. 启动数据库和服务：
   ```bash
   docker compose up -d postgres
   # 等待数据库就绪
   docker compose up -d api
   ```

6. 部署前端静态文件到 nginx root：
   ```bash
   # 假设前端构建产物在仓库 frontend/dist
   sudo mkdir -p /var/www/ai-forum
   sudo cp -r frontend/dist/* /var/www/ai-forum/
   ```

7. 验证：
   ```bash
   curl http://122.51.233.225/healthz
   curl http://122.51.233.225/api/v1/posts   # 示例
   ```

---

## 5. CI/CD：GitHub Actions

### 5.1 `.github/workflows/deploy.yml`

```yaml
name: Build and Deploy

on:
  push:
    branches: [main]

jobs:
  build-and-deploy:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      packages: write

    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3

      - name: Login to GHCR
        uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Build and push API image
        uses: docker/build-push-action@v5
        with:
          context: ./backend
          push: true
          tags: |
            ghcr.io/li-yongqvan/ai-forum-api:latest
            ghcr.io/li-yongqvan/ai-forum-api:${{ github.sha }}
          cache-from: type=gha
          cache-to: type=gha,mode=max

      - name: Deploy to server
        uses: appleboy/ssh-action@v1.0.3
        with:
          host: ${{ secrets.SSH_HOST }}
          username: ${{ secrets.SSH_USER }}
          key: ${{ secrets.SSH_PRIVATE_KEY }}
          script: |
            set -e
            cd /opt/ai-forum
            git pull origin main
            docker compose pull api
            docker compose up -d api
            docker compose ps
            # 可选：重新部署前端静态文件
            sudo cp -r frontend/dist/* /var/www/ai-forum/
```

### 5.2 需要提前配置的 GitHub Secrets

| Secret | 值 |
|---|---|
| `SSH_HOST` | `122.51.233.225` |
| `SSH_USER` | 服务器登录用户名（如 `ubuntu`） |
| `SSH_PRIVATE_KEY` | 服务器私钥（建议专门生成一个 deploy key） |

---

## 6. 日志与监控

### 6.1 日志

- Go 应用日志输出到 stdout/stderr。
- Docker 默认 `json-file` 驱动收集：
  ```bash
  docker logs -f ai-forum-api
  docker logs -f ai-forum-postgres
  ```
- 可选：在 `compose.yml` 加 `logging` 限制防止占满磁盘：
  ```yaml
  logging:
    driver: "json-file"
    options:
      max-size: "10m"
      max-file: "3"
  ```

### 6.2 监控

- Go API 暴露 `/healthz` 接口，返回 200 OK 即健康。
- 最低成本监控方案：
  - 免费外部服务：UptimeRobot / Better Stack（原 Oh Dear）/ 阿里云拨测，定时访问 `http://122.51.233.225/healthz`。
  - 或服务器 cron：
    ```bash
    */5 * * * * curl -fsS http://localhost/healthz > /dev/null || echo "$(date) healthcheck failed" >> /var/log/ai-forum-health.log
    ```

---

## 7. 备份与恢复

### 7.1 备份

已配置 `backup.sh` + cron，每日凌晨 3 点生成 `.sql.gz`，保留 7 天。

### 7.2 恢复

```bash
cd /opt/ai-forum
docker compose stop api
# 清空前确认
docker compose exec postgres dropdb -U afuser aiforum || true
docker compose exec postgres createdb -U afuser aiforum
# 恢复最近一次备份
zcat backups/aiforum_YYYYMMDD_HHMMSS.sql.gz | docker compose exec -T postgres psql -U afuser aiforum
docker compose start api
```

---

## 8. 已知技术债与后续演进

| 当前决策 | 技术债 | 触发升级条件 |
|---|---|---|
| 无域名，纯 HTTP | 登录凭据明文传输 | 有真实用户 / 公测前必须购买域名 + certbot HTTPS |
| 复用 nginx 而非 Caddy | 需手动 certbot 续期 | 购买域名后，可继续使用 nginx+certbot；如想自动 HTTPS，可迁移到 Caddy |
| GitHub Actions SSH 部署 | SSH 私钥存于 GitHub Secrets | 团队扩大 / 多环境部署时，改为 webhook + watchtower 或 ArgoCD |
| 单服务器 | 无高可用 | 用户量增长 / 需要 99.9% SLA 时考虑多机 + LB |
| 无 Redis | 进程内 goroutine 异步 | 通知队列积压、需要削峰填谷时引入 Redis / 消息队列 |

---

## 9. 与 #2 研究结论的关系

- #2 推荐 **Caddy** 作为 greenfield 反向代理（自动 HTTPS、配置简单）。
- 本方案选择 **nginx** 是因为目标部署服务器 `122.51.233.225` 已预装 nginx 并占用 80 端口。为避免迁移风险和额外维护面，MVP 阶段复用 nginx。
- Caddy 方案仍保留为「新服务器 / 无 nginx」场景的推荐，未来购买域名后也可根据运维偏好切换。
