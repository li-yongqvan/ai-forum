# #43 部署提速：服务器本地构建（计划文档）

> 计划文档 · 2026-08-24 · 实现会话产出（决策已与用户逐项确认）。handoff：`docs/handoffs/43-deploy-optimization.md`。
> **评审状态**：待设计文档评审（见 `docs/plans/issue-43-deploy-optimization-设计文档.md`）。

## Context（为什么做）

ai-forum 部署走 A' 方案（CI 构建镜像 → docker save|gzip 13MB → 跨洋 scp → 服务器 load → up），handoff 记录连续 12 次零超时（#38→#42 为其中连续 5 次），但**卡在跨洋 scp ~12KB/s 物理瓶颈**，deploy job 实测 ~18min（issue #43 背景；地图 #1 决策 23 登记）。本票目标：降到 ~4-5min（评审 F5 决议，验收 ≤5min），不牺牲稳定性（保持零超时）。

首选方向 = **服务器本地构建**（服务器 git pull + docker compose build，省掉跨洋大包）。v3.1 曾否决（「私有 repo 需凭证 + 架空 CI」，`docs/ops/deployment-v3-8888-container.md` §8），本票重估两个否决点：
- **凭证**：read-only deploy key 已在服务器 `~/.ssh/ai-forum-repo`（github.com IdentityFile + IdentitiesOnly），git pull 一直用它，无需新凭证。
- **架空 CI**：test job 的 `go build ./...` 保留作编译门；CI 仍做前端构建验证 + 门控 + 触发部署，CI 没有被架空。

**前端实况（重要）**：服务器 `frontend/dist` 的 2.9MB/320 文件是**历年累积的旧哈希产物**（scp 只增不删，#14 已知取舍）；本机最新一次构建仅 **583K / 71 文件** → tar.gz 后 ~250-400KB，跨洋 scp ~30-60s，**不是瓶颈**（也因此「清旧」是必需步）。

## 已与用户确认的决策（2026-08-24 grilling + 回退）

1. **CI 角色 → 纯服务器构建**：deploy job 默认删 docker build/GHCR push/save/scp 镜像；编译门由 test job `go build ./...` 承担。
2. **前端交付 → gzip 后 scp**：CI tar.gz（~250-400KB）→ scp ~30-60s → 服务器解压（**保 inode 清旧**）；前端仍 CI 构建（构建验证）。
3. **GOPROXY → 改 backend/Dockerfile 加 ARG**（已批准最小范围例外）：proxy.golang.org 服务器不可达（实测超时）→ 加 `ARG GOPROXY/GOSUMDB` + ENV；服务器 build 传 `--build-arg GOPROXY=https://goproxy.cn,direct --build-arg GOSUMDB=sum.golang.google.cn`（实测 goproxy.cn 200/0.06s）。
4. **预灌 → gated**：一次性预灌 golang:1.26-alpine + alpine:3.21（~400MB，本地→服务器 ~10min），**必须在 merge 前完成**（merge 后首次 deploy 即 compose build）。
5. **回退 → 双模式变量切换**：`DEPLOY_MODE` 仓库 variable（默认 `server`，应急 `scp`）；改变量 + Actions Re-run 即切回 A'，无需改代码/merge。
6. **job 结构 → 单 deploy job 顺序**：前端 tar 已不是瓶颈，拆并行 job 的版本错配/并发协调代价 > 收益；暖 cache 后 ~3-4min。

## 改动文件（范围红线：只动 CI/服务器配置 + docs，外加已批准的 Dockerfile 例外）

| 文件 | 改动 |
|---|---|
| `compose.yml` | api 服务加 `build: context: ./backend`（`image:` 不变 = 构建产物 tag） |
| `backend/Dockerfile` | build 阶段加 `ARG GOPROXY/GOSUMDB` + ENV；**BuildKit cache mount**（go mod download + go build）——已批准最小范围例外 |
| `deploy/bootstrap.sh` | 第 4 步 `docker compose pull api` → 显式 `build --build-arg ...`（R1 硬阻塞：拉 GHCR 过期镜像 / 裸 up 默认 GOPROXY 构建失败） |
| `deploy/deploy-server.sh` | **新建**（服务器侧部署脚本，核心） |
| `.github/workflows/deploy.yml` | deploy job 重写 + DEPLOY_MODE 双模式 + `workflow_dispatch` |
| `docs/ops/deployment-v3-8888-container.md` | 升 v3.2（否决记录改采纳、CI/bootstrap、预灌、磁盘卫生） |
| `docs/workflow.md` §9、`docs/ops/lessons-8888-deploy.md` | 流程同步 + 新坑位 |

不碰后端/前端功能代码、`api/content.ts`、`api/types.ts`、vite 配置、其他 views。

## 关键机制

**deploy-server.sh 的健壮性骨架**（Plan agent 校验定稿）：
1. **build 先于 up**：`docker compose build` 失败 → `set -e` 中止 → 旧容器照跑（compose 只在成功时更新 tag、运行容器按 image id 引用）。站点绝不停摆。
2. **健康轮询 + 自动回退**：up 后 `docker inspect -f '{{if .State.Health}}...' ai-forum-api` 轮询 ≤60s；healthy 通过 / unhealthy 或超时 → `docker tag :rollback :latest && up --force-recreate` 回退旧镜像，**回退后再轮询 ≤30s**（F2① 决议：旧镜像理应恢复，若旧镜像也挂=更深层问题，值得报警）。⚠️ `set -e` 下禁用 `cmd && break`，必须 if/fi；inspect 失败 `|| echo none` 兜底。
3. **rollback tag 时机**：build 前 `docker tag :latest :rollback`（成功后旧镜像变 dangling 会被 prune）+ `docker image inspect` 守卫防缺失。
4. **前端换装保 inode**：`find frontend/dist -mindepth 1 -delete`（清内容保目录 inode）→ `cp -a` 原地铺新；严禁 `rm -rf dist && mv`（web 容器 bind mount 挂旧 inode 永不生效）。`chmod -R a+rX`（nginx worker uid 101 需 other 读）。**必须 api healthy 之后**（防「新前端 + 坏 api」错配）。
5. **BuildKit cache mount**：go mod download + go build 挂 `/go/pkg/mod` + `/root/.cache/go-build` → 越部署越快（冷下 ~200MB modules 直接吃掉提速收益）。
6. **GOSUMDB 保险**：go.sum 完整提交（21KB）默认不查 sumdb，但加 `sum.golang.google.cn` 防边缘挂起。

## 改动 1：`backend/Dockerfile`

```dockerfile
FROM golang:1.26-alpine AS build
ARG GOPROXY=https://proxy.golang.org,direct
ENV GOPROXY=${GOPROXY}
ARG GOSUMDB=sum.golang.org
ENV GOSUMDB=${GOSUMDB}
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api
```
默认 proxy.golang.org/sum.golang.org（CI/本地不受影响）；服务器 build-arg 覆盖。运行时 stage（alpine:3.21 + adduser）不动。

## 改动 2：`compose.yml`

api 服务加 `build: context: ./backend`，`image: ghcr.io/li-yongqvan/ai-forum-api:latest` 不变（= compose 构建产物 tag，`docker compose build api` 即构建并打此 tag）。`user:`/`env_file:`/`healthcheck:` 不动。GOPROXY/GOSUMDB 由部署脚本显式 `--build-arg` 传（不硬编码进 compose）。

## 改动 3：`deploy/bootstrap.sh`

```bash
docker compose build --build-arg GOPROXY=https://goproxy.cn,direct --build-arg GOSUMDB=sum.golang.google.cn api
docker compose up -d
```
替换原 `docker compose pull api`。原因：compose 加 `build:` 后，`pull api` 拉 GHCR 过期 `:latest`（不再更新）；裸 `up -d` 在镜像缺失时用默认 GOPROXY（服务器不可达）自动构建必失败。

## 改动 4：`deploy/deploy-server.sh`（新建）

```bash
#!/usr/bin/env bash
set -euo pipefail
APP_DIR="$HOME/ai-forum"; cd "$APP_DIR"
FRONTEND_TAR="${1:-/tmp/ai-forum-frontend.tar.gz}"
[ -s "$FRONTEND_TAR" ] || { echo "FATAL: tar missing"; exit 1; }
tar -tzf "$FRONTEND_TAR" >/dev/null || { echo "FATAL: tarball corrupt"; exit 1; }
IMG=ghcr.io/li-yongquan/ai-forum-api
if docker image inspect "$IMG:latest" >/dev/null 2>&1; then
  docker tag "$IMG:latest" "$IMG:rollback"
fi
docker compose build --build-arg GOPROXY=https://goproxy.cn,direct --build-arg GOSUMDB=sum.golang.google.cn api
docker compose up -d --force-recreate api
ok=0
for i in $(seq 1 30); do
  st=$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}starting{{end}}' ai-forum-api 2>/dev/null || echo none)
  if [ "$st" = healthy ]; then ok=1; break; fi
  if [ "$st" = unhealthy ]; then break; fi
  sleep 2
done
if [ "$ok" -ne 1 ]; then
  docker tag "$IMG:rollback" "$IMG:latest"
  docker compose up -d --force-recreate api
  # F2①：回退后再轮询 ≤30s，仍不健康才 exit 1（旧镜像理应恢复，若旧镜像也挂=更深层问题）
  ok=0
  for i in $(seq 1 15); do
    st=$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}starting{{end}}' ai-forum-api 2>/dev/null || echo none)
    if [ "$st" = healthy ]; then ok=1; break; fi
    sleep 2
  done
fi
[ "$ok" -eq 1 ] || { echo "FATAL: api not healthy after rollback"; exit 1; }
rm -rf /tmp/ai-forum-frontend.new && mkdir -p /tmp/ai-forum-frontend.new
tar -xzf "$FRONTEND_TAR" -C /tmp/ai-forum-frontend.new
chmod -R a+rX /tmp/ai-forum-frontend.new
find frontend/dist -mindepth 1 -delete
cp -a /tmp/ai-forum-frontend.new/. frontend/dist/
rm -rf /tmp/ai-forum-frontend.new
docker compose exec -T web nginx -t
docker compose exec -T web nginx -s reload
docker image prune -f
rm -f "$FRONTEND_TAR"
docker compose ps
```
`.gitattributes` 已有 `deploy/*.sh text eol=lf`，自动 LF。

## 改动 5：`.github/workflows/deploy.yml`

- **删除**：buildx setup / GHCR login / build-push-action / save|gzip / scp 镜像（移入 scp 模式条件步）；`permissions` 收窄为 `contents: read`。
- **保留**：checkout / node setup / npm ci / `npm run build`。
- **前端上传**（通用）：`tar -czf frontend-dist.tar.gz -C frontend/dist .` → scp 到 `/tmp`（门控 `env.SSH_HOST != '' && vars.DEPLOY_ENABLED == 'true'`）。
- **scp 模式专属**（`if: … && vars.DEPLOY_MODE == 'scp'`）：buildx + docker build（`push:false, load:true`）→ `docker save|gzip` → scp 到 `/tmp`。
- **部署步**（通用，脚本按 `${{ vars.DEPLOY_MODE }}` 分叉）：
  ```yaml
  uses: appleboy/ssh-action@v1.0.3
  with:
    port: 2222
    command_timeout: 10m      # F6：build 仅 2-4min，10m 余量足够；试构建实测超限再调
    script: |
      set -euo pipefail
      cd ~/ai-forum
      git pull --ff-only origin main
      mkdir -p frontend/dist uploads
      if [ "${{ vars.DEPLOY_MODE }}" = "scp" ]; then
        tar -xzf /tmp/ai-forum-frontend.tar.gz -C frontend/dist/
        docker load -i /tmp/api-image.tar.gz
        docker compose up -d --force-recreate api
        # F3①：应急路径也等健康（≤60s），不出「deploy 绿站点红」
        for i in $(seq 1 30); do
          st=$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}starting{{end}}' ai-forum-api 2>/dev/null || echo none)
          if [ "$st" = healthy ]; then break; fi
          if [ "$st" = unhealthy ]; then echo "FATAL: api unhealthy"; exit 1; fi
          sleep 2
        done
        [ "$st" = healthy ] || { echo "FATAL: api not healthy in 60s"; exit 1; }
        docker compose exec -T web nginx -t && docker compose exec -T web nginx -s reload
      else
        bash deploy/deploy-server.sh /tmp/ai-forum-frontend.tar.gz
      fi
      docker image prune -f
      docker compose ps
  ```
- **`on:` 加 `workflow_dispatch`**（应急手动触发）；`concurrency: group: deploy-prod, cancel-in-progress: false` 保留。
- actionlint 合规：`secrets` 只落 job `env`，step `if` 只用 `env.`/`vars.`；本地 `bash scripts/check-workflows.sh` + 分支 push workflow-lint 双把关。

## 预灌 bootstrap（gated，方案批准后执行；必须在 merge 前）

⚠️ 本机（中国）也可能拉不到 Docker Hub——先验证，失败换镜像源（`docker pull m.daocloud.io/docker.io/library/golang:1.26-alpine` 再 retag）。

```bash
docker pull golang:1.26-alpine && docker pull alpine:3.21 && docker pull docker/dockerfile:1   # F1：buildkit frontend，防 Docker Hub 被墙下 build 解析 syntax 失败
docker save golang:1.26-alpine | gzip -1 > golang-alpine.tar.gz
docker save alpine:3.21 | gzip -1 > alpine.tar.gz
docker save docker/dockerfile:1 | gzip -1 > dockerfile-frontend.tar.gz
scp -P 2222 golang-alpine.tar.gz alpine.tar.gz dockerfile-frontend.tar.gz liyongquan@122.51.233.225:/tmp/
ssh -p 2222 liyongquan@122.51.233.225 \
  'docker load -i /tmp/golang-alpine.tar.gz && docker load -i /tmp/alpine.tar.gz && docker load -i /tmp/dockerfile-frontend.tar.gz && rm -f /tmp/golang-alpine.tar.gz /tmp/alpine.tar.gz /tmp/dockerfile-frontend.tar.gz'
```
记录 digest（`docker image inspect golang:1.26-alpine --format '{{.RepoDigests}}'`）写进 docs。

**merge 前试构建**（grilling 项 3 真值）：在 PR 分支上，服务器 `cd ~/ai-forum && docker compose build --build-arg GOPROXY=https://goproxy.cn,direct --build-arg GOSUMDB=sum.golang.google.cn api`，实测耗时/内存峰值（watch `free -m`）。试构建不碰线上 api 容器。

## 边界情况（实现时对照）

服务器构建坏但服务器活着 → **DEPLOY_MODE=scp 应急通道**（改 variable + Re-run）；构建失败 → 旧容器照跑（build 先于 up）；坏镜像被部署 → 健康轮询 60s 拦截 + rollback tag 秒回滚；磁盘满 → build 失败旧容器照跑，`image/builder prune` 后重发；前端 tar 损坏 → 脚本开头 `tar -tzf` fail fast；服务器本身 down（docker daemon/失联）→ 超出 CI 能力，运维恢复（重启/`docker compose up -d`）。

## 验证（SOP §7/§9）

```
# 本地（push 前）
bash scripts/check-workflows.sh          # actionlint（本地 Docker 可用）
bash -n deploy/deploy-server.sh
docker compose config                     # compose.yml 校验
# 本机 docker compose build 试跑（验证 Dockerfile ARG/cache mount 不破坏 CI 默认路径）

# 预灌 + 试构建（gated）：见上，记录服务器构建耗时/内存

# 试点发版：1 个含代码改动的 PR → 合并 → CI server 模式部署
# 服务器验证：git HEAD / docker compose ps / curl http://122.51.233.225:8888/healthz / /api/v1/posts
# 实测部署耗时（目标 deploy job ≤5min，F5 决议；暖 cache 后续跑应更快）

# 回退演练（可选）：docker tag :latest :rollback && up --force-recreate 验证可回；
#                   设 DEPLOY_MODE=scp → Re-run → 验证 A' 路径仍通 → 恢复 server
```

分支：`git checkout -b chore/deploy-speedup`；动手前 `git status --porcelain` 查脏、`git fetch origin && git pull origin main`；若并行会话活跃用 `git worktree`。PR 走 SOP §8（`MSYS_NO_PATHCONV=1 gh`，合并前 `/code-review`，**用户授权后** merge）；地图 #1 更新交给统筹方，本会话不自行 edit。
