#!/usr/bin/env bash
# ai-forum 服务器部署脚本（#43 v3.2：服务器本地构建 api）
# 用法：bash deploy/deploy-server.sh [frontend-tar-path]（默认 /tmp/ai-forum-frontend.tar.gz，CI 上传位置）
# 安全模型：
#   - build 先于 up：构建失败 set -e 中止，旧容器按 image id 引用照跑（站点不停摆）
#   - 健康轮询 + 自动回退：up 后 ≤60s 等 healthy；unhealthy/超时 → 回退 rollback tag 镜像，再轮询 ≤30s
#   - 前端换装保 inode：find -delete 清内容 + cp -a 原地铺新（web 容器 bind mount frontend/dist）
#     ⚠️ 严禁 rm -rf frontend/dist && mv——bind mount 绑 inode，换目录后容器仍挂旧目录，新文件永不生效
#   - 换装必须 api healthy 之后：防「新前端 + 坏 api」版本错配
set -euo pipefail

APP_DIR="$HOME/ai-forum"
cd "$APP_DIR"
FRONTEND_TAR="${1:-/tmp/ai-forum-frontend.tar.gz}"
mkdir -p frontend/dist uploads   # 自愈：目录被误删时先以部署用户重建（防 docker 以 root 重建后写入失败）

# 0. 前置校验：前端 tar 存在且可读（fail fast，不碰运行栈）
[ -s "$FRONTEND_TAR" ] || { echo "FATAL: $FRONTEND_TAR missing" >&2; exit 1; }
tar -tzf "$FRONTEND_TAR" >/dev/null || { echo "FATAL: tarball corrupt" >&2; exit 1; }

# 1. 回退保险：build 前把当前 :latest 固化（build 成功会覆盖 :latest，旧镜像变 dangling 被 prune）
IMG=ghcr.io/li-yongqvan/ai-forum-api
if docker image inspect "$IMG:latest" >/dev/null 2>&1; then
  docker tag "$IMG:latest" "$IMG:rollback"
fi

# 2. 服务器本地构建（goproxy.cn；失败 set -e 中止，旧容器照跑）
docker compose build --build-arg GOPROXY=https://goproxy.cn,direct --build-arg GOSUMDB=sum.golang.google.cn api

# 3. 拉起 + 健康轮询（复用 deploy/wait-healthy.sh；unhealthy/超时 → 回退旧镜像）
docker compose up -d --force-recreate api
ok=0
if bash deploy/wait-healthy.sh ai-forum-api 60; then ok=1; fi
if [ "$ok" -ne 1 ]; then
  # 回退旧镜像（旧镜像之前一直 healthy，理应恢复）；回退后再轮询 ≤30s（F2①：旧镜像也挂=更深层问题，值得报警）
  # guard：:rollback 仅在 :latest 存在时创建（见上），消费前也需 inspect 守卫——防 set -e 下 tag 失败硬中断
  if docker image inspect "$IMG:rollback" >/dev/null 2>&1; then
    docker tag "$IMG:rollback" "$IMG:latest"
    docker compose up -d --force-recreate api
    ok=0
    if bash deploy/wait-healthy.sh ai-forum-api 30; then ok=1; fi
  else
    echo "WARN: no $IMG:rollback available to roll back to（api may be unhealthy）" >&2
  fi
fi
[ "$ok" -eq 1 ] || { echo "FATAL: api not healthy after rollback" >&2; exit 1; }

# 4. 前端换装（api healthy 之后；staging → 清旧内容 → 保 dist inode 原地铺新）
rm -rf /tmp/ai-forum-frontend.new && mkdir -p /tmp/ai-forum-frontend.new
tar -xzf "$FRONTEND_TAR" -C /tmp/ai-forum-frontend.new
chmod -R a+rX /tmp/ai-forum-frontend.new   # nginx worker uid 101（other）可读
find frontend/dist -mindepth 1 -delete     # 清旧内容（防哈希文件累积），保目录 inode
cp -a /tmp/ai-forum-frontend.new/. frontend/dist/
rm -rf /tmp/ai-forum-frontend.new

# 5. nginx 配置热载（web.conf 是 bind mount，只改文件不重建容器）
docker compose exec -T web nginx -t
docker compose exec -T web nginx -s reload

# 6. 卫生 + 收尾
docker image prune -f
rm -f "$FRONTEND_TAR"
docker compose ps
