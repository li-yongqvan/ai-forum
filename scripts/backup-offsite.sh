#!/usr/bin/env bash
# 异地备份派发脚本（#60）：把本地 ~/backups 快照复制到可配置目标。
# 设计原则：产生侧（现有 cron）不动，只做「派发 +  retention 修剪」；目标可插拔。
set -euo pipefail

DRY_RUN=0
if [[ "${1:-}" == "--dry-run" ]]; then
  DRY_RUN=1
fi
if [[ "${BACKUP_DRY_RUN:-0}" == "1" ]]; then
  DRY_RUN=1
fi

# 配置源：仓库外 $HOME/backup.conf（生产配置不放仓库）；环境变量优先，未设置时走默认值。
BACKUP_DEST="${BACKUP_DEST:-local://${HOME}/backups-offsite}"
BACKUP_RETENTION="${BACKUP_RETENTION:-14}"
BACKUP_SRC="${BACKUP_SRC:-${HOME}/backups}"
CONFIG_FILE="${BACKUP_CONFIG:-${HOME}/backup.conf}"
if [[ -f "$CONFIG_FILE" ]]; then
  # shellcheck source=/dev/null
  source "$CONFIG_FILE"
fi

log() { echo "[backup-offsite] $*"; }

if [[ ! -d "$BACKUP_SRC" ]]; then
  log "源目录不存在: $BACKUP_SRC（无备份可派发）"
  exit 0
fi

# 解析目标 scheme
scheme="${BACKUP_DEST%%://*}"
target="${BACKUP_DEST#*://}"

# 取最新快照文件清单（按 mtime 倒序）
mapfile -t files < <(find "$BACKUP_SRC" -maxdepth 1 -type f -printf '%T@ %p\n' 2>/dev/null | sort -rn | awk '{print $2}')
if [[ ${#files[@]} -eq 0 ]]; then
  log "源目录无文件: $BACKUP_SRC"
  exit 0
fi

case "$scheme" in
  local)
    mkdir -p "$target"
    for f in "${files[@]}"; do
      name=$(basename "$f")
      if [[ $DRY_RUN -eq 1 ]]; then
        log "[dry-run] 将复制: $f -> $target/$name"
      else
        cp -a "$f" "$target/$name"
        log "已复制: $name"
      fi
    done
    # retention 修剪：保留最新 $BACKUP_RETENTION 个，其余删除
    mapfile -t targets < <(find "$target" -maxdepth 1 -type f -printf '%T@ %p\n' 2>/dev/null | sort -rn | awk '{print $2}')
    if [[ ${#targets[@]} -gt $BACKUP_RETENTION ]]; then
      for old in "${targets[@]:$BACKUP_RETENTION}"; do
        if [[ $DRY_RUN -eq 1 ]]; then
          log "[dry-run] 将删除过期备份: $old"
        else
          rm -f "$old"
          log "已删除过期备份: $(basename "$old")"
        fi
      done
    fi
    ;;
  scp)
    log "scp:// 目标暂未实现（#60 占位），请改 local:// 或在 backup.conf 中配置具体方案"
    exit 1
    ;;
  oss)
    log "oss:// 目标预留（#60 占位），请改 local:// 或等后续实现"
    exit 1
    ;;
  *)
    log "未知目标 scheme: $scheme"
    exit 1
    ;;
esac

if [[ $DRY_RUN -eq 1 ]]; then
  log "dry-run 完成，未实际执行拷贝/删除"
else
  log "完成（目标: $BACKUP_DEST，保留: $BACKUP_RETENTION 份）"
fi
