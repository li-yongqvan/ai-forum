#!/usr/bin/env bash
# ai-forum「新代码在跑」部署核对（#74，防静默回退）
# 用法：bash deploy/verify-deployed.sh [期望api镜像ID] [容器名]
#   期望镜像 ID 非空 → 核对①运行容器镜像 == 期望（防回退到旧码；格式两侧统一去 sha256: 前缀后精确相等）
#   核对②schema_migrations 已应用集合 ⊇ 仓库 backend/migrations/*.sql 文件名集合（防 DB 停在旧版）
#   全部通过 exit 0；任一失败 exit 1（调用方 set -e 自然中止 deploy → CI 红）
# 前置条件：仓库已 git pull 到本次 HEAD（deploy.yml 先 pull）；schema_migrations 版本=文件名（0007×2 两条独立记录）
# ⚠️ 期望镜像 ID 必须在 up 前捕获（build 后 / load 后），回退分支会 retag :latest，up 后捕获即失真
# ⚠️ 测试：source 本文件（VERIFY_SOURCE_ONLY=1）后直接调用 verify_image / verify_migrations（第 3 参注入输入）
set -euo pipefail
APP_DIR="$HOME/ai-forum"

# 统一镜像 ID 格式：docker inspect -f '{{.Image}}' 与 docker images --no-trunc -q 均带 sha256: 前缀（2026-08-31 实查逐字相等），
# 去前缀后精确比对——即使某侧格式变化（如短 ID/无前缀）也不误判
strip_sha256_prefix() { echo "${1#sha256:}"; }

# 核对①：运行容器镜像 == 期望镜像（期望为空跳过）。$3 为测试注入的运行镜像（空=实查 docker inspect）
verify_image() { # $1=期望镜像ID $2=容器名 $3=运行镜像（测试注入）
  local expected="${1:-}" ct="${2:-ai-forum-api}" running="${3:-}"
  [ -n "$expected" ] || return 0
  if [ -z "$running" ]; then
    running="$(docker inspect -f '{{.Image}}' "$ct" 2>/dev/null || echo '')"
  fi
  if [ "$(strip_sha256_prefix "$running")" != "$(strip_sha256_prefix "$expected")" ]; then
    echo "VERIFY-FAIL: api 运行镜像 != 本次构建镜像（检测到回退）" >&2
    echo "  running   = ${running:-<无法读取>}" >&2
    echo "  expected  = $expected" >&2
    return 1
  fi
}

# 核对②：schema_migrations 已应用集合 ⊇ 仓库迁移文件集合（按完整文件名）。$1 为测试注入的已应用集（空=实查 psql）
verify_migrations() { # $1=已应用集（测试注入；空=psql 实查）
  local applied="${1:-}"
  if [ -z "$applied" ]; then
    # ⚠️ compose exec 吞 stdin → 必须 -T + 命令尾 < /dev/null；psql 失败 → set -e 中止（fail-closed）
    applied="$(docker compose exec -T postgres psql -U forum -d forum -tA -c "SELECT version FROM schema_migrations" < /dev/null)"
  fi
  local missing="" b f
  for f in backend/migrations/*.sql; do
    b="$(basename "$f")"
    if ! printf '%s\n' "$applied" | grep -qxF "$b"; then
      missing="$missing $b"
    fi
  done
  if [ -n "$missing" ]; then
    echo "VERIFY-FAIL: DB 缺迁移（新代码未真正落地）：${missing}" >&2
    return 1
  fi
}

# 主流程（被 source 供单测时跳过）
if [ "${VERIFY_SOURCE_ONLY:-0}" != "1" ]; then
  cd "$APP_DIR"
  verify_image "${1:-}" "${2:-ai-forum-api}" || exit 1
  verify_migrations || exit 1
  echo "VERIFY-OK: 新代码在跑（镜像一致，迁移齐全）"
fi
