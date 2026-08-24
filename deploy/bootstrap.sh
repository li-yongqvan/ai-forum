#!/usr/bin/env bash
# ai-forum 服务器初始化：仅需 docker 组 + 家目录，全程无 sudo
# 硬前置：repo 私有，read-only deploy key 已装 ~/.ssh（执行清单第 1 步）
set -euo pipefail
APP_DIR="$HOME/ai-forum"

# 0. GitHub SSH 凭证自检（匿名 clone 私有 repo 必失败，先 fail fast）
# 注：ssh -T git@github.com 认证成功也返回 exit 1（GitHub 不提供 shell），pipefail 下直接管道会误判失败 → 先捕获输出再 grep
auth_out=$(ssh -T git@github.com 2>&1 || true)
echo "$auth_out" | grep -q "successfully authenticated" \
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

# 4. 拉起（v3.2：服务器本地构建 api——proxy.golang.org 被墙，go mod 走 goproxy.cn；nginx/postgres 已预灌本地）
#    前置：golang:1.26-alpine + alpine:3.21 + docker/dockerfile:1 已预灌（见执行清单）
docker compose build --build-arg GOPROXY=https://goproxy.cn,direct --build-arg GOSUMDB=sum.golang.google.cn api
docker compose up -d
docker compose ps
