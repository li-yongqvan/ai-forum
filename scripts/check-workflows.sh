#!/usr/bin/env bash
# 本地 actionlint 校验（Docker，与 CI 同镜像同版本 rhysd/actionlint:1.7.12）
# 用法：仓库根目录运行  bash scripts/check-workflows.sh
set -euo pipefail

# Git Bash / MSYS 下把 POSIX 路径转成 Windows 路径，否则 Docker Desktop 无法挂载
repo_dir="$PWD"
case "$(uname)" in
  MINGW*|MSYS*|CYGWIN*) repo_dir="$(cygpath -w "$PWD")" ;;
esac

# MSYS_NO_PATHCONV=1：阻止 Git Bash 把容器内路径（-w /repo）错误转成宿主路径
export MSYS_NO_PATHCONV=1
exec docker run --rm -v "${repo_dir}:/repo" -w /repo rhysd/actionlint:1.7.12 -color
