#!/usr/bin/env bash
# ai-forum api 健康等待（#43 共享辅助，deploy-server.sh 与 deploy.yml scp 应急模式共用）
# 用法：bash deploy/wait-healthy.sh [容器名] [超时秒数]
#   默认 ai-forum-api / 60s；healthy → exit 0；unhealthy 或超时 → exit 1
# ⚠️ 调用方若有 rollback 逻辑，用 `if bash deploy/wait-healthy.sh ...; then` 捕获（本脚本按 `set -e` 原则显式 exit，不吞）
set -euo pipefail
CT="${1:-ai-forum-api}"
TIMEOUT="${2:-60}"
attempts=$((TIMEOUT / 2))
for i in $(seq 1 "$attempts"); do
  st=$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}starting{{end}}' "$CT" 2>/dev/null || echo none)
  if [ "$st" = healthy ]; then exit 0; fi
  if [ "$st" = unhealthy ]; then echo "FATAL: $CT unhealthy" >&2; exit 1; fi
  sleep 2
done
echo "FATAL: $CT not healthy in ${TIMEOUT}s" >&2
exit 1
