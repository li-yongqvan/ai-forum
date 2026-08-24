# #43 部署提速：服务器本地构建 — 设计文档（供评审）

> 文档用途：交付专业评审 agent 的评审对象。范围 = 背景 / 真值核对 / 决策记录 / 实现方案 / 不变量 / 验证。
> 溯源约定：**事实**标来源（代码 `file:line` / 服务器实查输出 / GitHub issue / grilling 用户确认）；**判断性裁决**单独标注【决策】并给出理由与备选，不冒充事实。
> 数据时点：2026-08-24（真值核对执行日；服务器项注明实查日 2026-08-24）。
> 评审状态：**已评审（有条件通过，0 阻塞）**——评审意见书：`docs/plans/issue-43-deploy-optimization-设计文档-评审意见书.md`。F1-F8 决议见 §10（F2①/F3①/F5① 用户确认 2026-08-24），本版为实现基线。

## 0. 项目上下文（给零背景评审 agent，先读本节）

**这是什么**：AI 智联论坛——社团内部 AI 主题社区的移动端论坛 MVP，已上线 `http://122.51.233.225:8888/`。
- 前端：Vue 3 + TypeScript + Vite + Vant（PWA），纯静态部署（`frontend/`，CI 构建后 scp）。
- 后端：Go（Gin + GORM + PostgreSQL），单进程模块化单体（`backend/`，多阶段 Dockerfile 构建单二进制）。
- 部署：单服务器 Docker Compose all-in-one（web @8888 反代 + api + postgres），见 `docs/ops/deployment-v3-8888-container.md`（v3.1 基线）与 `compose.yml`。

**部署相关架构约束**（评审本设计必须理解）：
- 服务器 `122.51.233.225`（阿里云）**多人共享、`liyongquan` 无 sudo**（uid 1003，在 docker/nginx/devgroup 组；唯一 sudoer `ubuntu`）。docker 组成员等价 root（可跑 root 容器），但**不能改宿主配置文件**（如 `/etc/docker/daemon.json`）。
- **Docker Hub 被墙**（服务器实测 `registry-1.docker.io` 超时）；`ghcr.io`、`registry.cn-hangzhou.aliyuncs.com` 可达。基础镜像（nginx/postgres）已一次性 `docker save | ssh docker load` 预灌（v3.1，已执行）。
- repo 私有（`isPrivate: true`）；服务器用 **read-only deploy key**（`~/.ssh/ai-forum-repo`）git pull。
- SSH 端口 **2222**（sshd 迁移，PR #52，2026-08-23；默认 22 已不通）。CI 三处 SSH 步已补 `port: 2222`。
- 部署门控：`if: env.SSH_HOST != '' && vars.DEPLOY_ENABLED == 'true'`（**secrets 永不进 if**，SOP 红线）；job 级 `concurrency: deploy-prod`。
- 前端 dist 由 CI scp → `~/ai-forum/frontend/dist`（web 容器 bind mount `./frontend/dist:/usr/share/nginx/html:ro`）；`deploy/web.conf` 亦 bind mount（改后需 `nginx reload`）。

**CI 现状（A' 方案，本票要替换的基线）**：`.github/workflows/deploy.yml` deploy job 六步——前端 npm ci+build → buildx+GHCR login → build-push（`load:true`）→ `docker save|gzip`（~13MB）→ scp frontend+image → ssh 部署脚本（git pull → docker load → up --force-recreate → nginx reload → prune）。实测 ~18min，瓶颈 = GH runner→阿里云跨洋 scp ~12KB/s（issue #43 背景）。

**与地图/前序的关系**：本票 = GitHub **issue #43**（地图 #1 决策 23 登记的开放项「8/24 后·部署优化探索」）；#26（A' 方案）已关闭；handoff `docs/handoffs/43-deploy-optimization.md` 自包含交接。

## 1. 背景与目标（为什么做）

- 需求来源：GitHub **issue #43**「部署提速探索：服务器本地构建方案（8/24 后）」；地图 #1 决策 23「8/24 后·部署优化探索登记」（2026-08-21）。
- 交接依据：`docs/handoffs/43-deploy-optimization.md` §1-§3（目标 / 现状 A' / 开放决策清单）。
- **目标**：部署时间从 **~18min** 降到 **~4-5min**（评审 F5 决议：单 job 现实 ~3.5-5min，验收口径 ≤5min），不牺牲稳定性（保持零超时）。
- **首选方向**：服务器本地构建（服务器 `git pull` + `docker compose build`，省掉跨洋 13MB 镜像包）。v3.1 曾否决（私有 repo 需凭证 + 架空 CI），本票重估两个否决点（见 §2 真值）。

## 2. 真值核对（数据来源，全部可复现）

### 2.1 服务器真值（2026-08-24 实查，SSH `ssh -p 2222 liyongquan@122.51.233.225 '...'`）

| 项 | 实查命令 | 结果摘录 | 结论 |
|---|---|---|---|
| 线上版本 | `cd ~/ai-forum && git log --oneline -1` | `0301601 Merge pull request #52 ...` | 线上 = PR #52（SSH 2222 修复已生效） |
| 容器 | `docker compose ps` | 三容器 Up：api(healthy)/postgres(healthy)/web(Up) | 部署形态 8888 容器化 |
| 健康 | `curl -s -o /dev/null -w "%{http_code}" http://localhost:8888/healthz` | `200` | 服务正常 |
| 资源 | `nproc` / `free -m` / `df -h ~` | 4 核 / total 3723、available 2638 / 23G 可用 | **4 核 / 3.7G 内存(可用 2.6G) / 23G 磁盘**——Go 构建量级够，OOM 风险待试构建核实 |
| Docker | `docker --version` | `Docker version 29.1.3` | 支持 BuildKit cache mount（`# syntax=docker/dockerfile:1`） |
| 本地镜像 | `docker images` | 仅 `ghcr.io/li-yongqvan/ai-forum-api:latest`、`nginx:1.27-alpine`、`postgres:16-alpine` | **服务器无 golang/alpine builder 镜像 → 需预灌**（Docker Hub 被墙无法自拉） |
| go/node | `which go` | `no go binary` | 服务器本地构建 = 容器内构建（无需宿主工具链） |
| deploy key | `cat ~/.ssh/config` | `Host github.com / IdentityFile ~/.ssh/ai-forum-repo / IdentitiesOnly yes` | **read-only deploy key 复用可行**（grilling 项 1 解开） |
| **网络可达** | `curl --max-time 10 https://proxy.golang.org/` | `000 10.0s FAIL` | **proxy.golang.org 不可达** → go mod download 必失败 → 必须 GOPROXY 覆盖 |
| 网络可达 | `curl https://goproxy.cn/` | `200 0.067s` | **goproxy.cn 可达且快**（覆盖方案成立） |
| 网络可达 | `curl https://ghcr.io/` | `301` | GHCR 可达（但 ~2.6KB/s，不能作快速部署通道，见 §9 Q4） |
| 前端 dist | `du -sh ~/ai-forum/frontend/dist && find ... | wc -l` | `2.9M / 320 files` | **服务器 dist 是历年累积旧哈希**（scp 只增不删，#14 已知取舍） |

### 2.2 代码真值（2026-08-24 实查，本地仓库 grep/read）

| 项 | 证据 | 结论 |
|---|---|---|
| compose.yml api 无 build 段 | `compose.yml:29-30` 仅 `image: ghcr.io/li-yongqvan/ai-forum-api:latest`（无 `build:`） | `docker compose build api` 现无构建上下文 → **需补 `build: context: ./backend`** |
| Dockerfile builder | `backend/Dockerfile:4` `FROM golang:1.26-alpine AS build`；`:11` `FROM alpine:3.21` | 服务器需预灌这两个镜像；`# syntax=docker/dockerfile:1`（L1）已支持 cache mount |
| Dockerfile 无 GOPROXY/GOSUMDB ARG | `backend/Dockerfile` 全文无 `ARG GOPROXY` | **需加 ARG + ENV**（服务器默认 proxy.golang.org 不可达） |
| deploy.yml 现状 | `.github/workflows/deploy.yml:59-156` A' 六步 + 三处 `port: 2222` + 门控 `if: env.SSH_HOST != '' && vars.DEPLOY_ENABLED == 'true'` | 基线确认；重写保留门控 + concurrency |
| bootstrap.sh 现状 | `deploy/bootstrap.sh:47` `docker compose pull api` | **R1 硬阻塞属实**：加 `build:` 后 `pull api` 拉 GHCR 过期镜像；裸 `up -d` 缺镜像时用默认 GOPROXY 构建必失败 → 需改显式 build |
| 行尾卫生 | `.gitattributes`：`deploy/*.sh text eol=lf` | 新增 `deploy-server.sh` 自动 LF，不崩服务器 bash |
| go 版本 | `backend/go.mod` `go 1.25.0` | CI test job（setup-go 读 go.mod）用 1.25；服务器构建用 golang:1.26-alpine → 编译门(1.25) vs 产物(1.26) 漂移，见 §9 Q3 |
| go.sum 完整性 | `wc -c backend/go.sum` → `20844` | go.sum 含 indirect 完整提交 → `go mod download` 默认不触发 sumdb（GOSUMDB 是边缘保险） |
| workflow 校验 | `scripts/check-workflows.sh`（rhysd/actionlint:1.7.12 via Docker）；`.github/workflows/workflow-lint.yml` push 阻塞 | 改 deploy.yml 后可本地预验 + 分支 push 拦截 |
| 构建上下文卫生 | `backend/.dockerignore`：`.env` `.env.*` `.git` `*.log` | 服务器构建上下文不携带敏感文件 ✓ |

### 2.3 GitHub 状态（2026-08-24）

- 地图 **issue #1** 决策 23：「8/24 后·部署优化探索（服务器本地构建方向，预计 2-4min vs 现状 ~18min）登记为开放项——8/24 前求稳不动」。
- **issue #43** OPEN，labels `post-launch, wayfinder:task`，含 5 项第 0 步 grilling 清单。
- handoff `docs/handoffs/43-deploy-optimization.md` 自包含交接（§2 现状、§3 开放决策、§4 协调红线）。
- 本地 `main` = `0301601` 与服务器 HEAD 一致（`git log --oneline -1`，本机与服务器均确认）。

### 2.4 未复核项（诚实标注，绝不编造）

| 项 | 状态 | 说明 |
|---|---|---|
| 跨洋 scp ~12KB/s / deploy ~18min | 来源 handoff/issue #43 自我报告，**非独立实测** | 本票验证阶段以试点发版实测为准 |
| 本地→服务器 SSH ~780KB/s | 来源 `docs/workflow.md` §9.2 | 预灌估时依据（~400MB ≈ 10min） |
| 本机 Docker Hub 实际 `docker pull`（非 manifest） | 仅 `docker manifest inspect golang:1.26-alpine` 通过（2026-08-24） | 实施期预灌第一步实测；失败换镜像源 |
| 服务器构建实际耗时/内存峰值 | 未实测 | **merge 前试构建**（gating 步骤）核实 |
| GOSUMDB=sum.golang.google.cn 可达 | 未实测 | goproxy.cn 200 已证；GOSUMDB 为边缘保险，若挂则 go.sum 完整不触发 |

## 3. Grilling 决策记录（6 项，用户确认 2026-08-24）

| 编号 | 决策问题 | 定案 | 依据 |
|---|---|---|---|
| D1 | CI 角色：镜像构建放哪 | **纯服务器构建**：deploy job 默认删 docker build/GHCR push/save/scp；编译门由 test job `go build ./...` 承担 | 用户确认（2026-08-24）；v3.1 否决点「架空 CI」重估（§2.1/§2.3） |
| D2 | 前端 dist 怎么交付 | **gzip 后 scp**：CI tar.gz（~250-400KB）→ scp → 服务器解压（保 inode 清旧） | 用户确认（2026-08-24）；§2.2 前端 583K/71 文件非瓶颈 |
| D3 | proxy.golang.org 不可达怎么绕 | **改 backend/Dockerfile 加 `ARG GOPROXY/GOSUMDB` + ENV**；服务器 build 传 goproxy.cn | 用户确认（2026-08-24，批准最小范围例外）；§2.1 网络实测 |
| D4 | 构建基础镜像预灌 | **接受一次性预灌** golang:1.26-alpine + alpine:3.21（~400MB，~10min），**必须在 merge 前** | 用户确认（2026-08-24，gated：先出方案再执行）；§2.1 服务器无该镜像 |
| D5 | 服务器构建故障时怎么保 CI 能部署 | **双模式变量切换**：`DEPLOY_MODE` 仓库 variable（默认 `server`，应急 `scp`）；改变量 + Re-run 即切回 A'，无需改代码/merge | 用户确认（2026-08-24）；§6.2 【决策】 |
| D6 | deploy job 拆并行还是顺序 | **单 deploy job 顺序** | 用户确认（2026-08-24）；§6.5 【决策】 |

## 4. 范围收敛与明确不做

| 项 | 决策 | 依据 |
|---|---|---|
| 后端/前端功能代码 | **零改动** | 范围红线（handoff §4） |
| GHCR 冗余备份（CI 推镜像） | **不做**（纯服务器构建，D1） | 用户确认（2026-08-24）；GHCR→服务器拉取 ~2.6KB/s 不可作快速回退（§2.1） |
| 前端也服务器构建（预灌 node/npm） | **不做**（本轮） | 用户 grilling 未选；记录为后续提速方向 |
| PR 阶段 CI | **不做**（已知遗留 SOP §9.3） | 非本票 |
| 服务器本身 down（docker daemon/失联）的恢复 | **不做**（超出 CI 能力） | A' 也救不了；运维恢复范畴 |
| `backend/Dockerfile` 加 ARG + cache mount | **做**（已批准最小范围例外） | D3 + §5.1；构建配置非功能代码 |

## 5. 实现方案（每项给出依据）

### 5.1 `backend/Dockerfile`
- build 阶段加 `ARG GOPROXY=https://proxy.golang.org,direct` + `ENV GOPROXY=${GOPROXY}` + 同款 GOSUMDB（默认 sum.golang.org）；**go mod download 与 go build 加 BuildKit cache mount**（`--mount=type=cache,target=/go/pkg/mod` / `/root/.cache/go-build`）。
- 依据：D3（proxy.golang.org 不可达，§2.1）；cache mount = 提速核心（冷下 ~200MB modules 每跑重下直接吃掉收益，Plan agent 校验 R3）；默认不覆盖 CI（§6.6）。
- 运行时 stage（alpine:3.21 + adduser）不改。

### 5.2 `compose.yml`
- api 服务加 `build: context: ./backend`；`image: ghcr.io/li-yongqvan/ai-forum-api:latest` 不变（= compose 构建产物 tag）。
- 依据：§2.2（现无 build 段，`docker compose build api` 无上下文）；compose build 产物 tag = 运行时 tag，`up` 直接复用。

### 5.3 `deploy/bootstrap.sh`（R1 硬阻塞修复）
- 第 4 步 `docker compose pull api` → `docker compose build --build-arg GOPROXY=https://goproxy.cn,direct --build-arg GOSUMDB=sum.golang.google.cn api` + `docker compose up -d`。
- 依据：§2.2（`pull api` 拉 GHCR 过期 `:latest`；裸 `up -d` 缺镜像时默认 GOPROXY 构建失败）。

### 5.4 `deploy/deploy-server.sh`（新建，核心）
顺序与关键点（Plan agent 校验后的健壮版，完整脚本见计划文档）：
1. 前置校验：前端 tar 存在且 `tar -tzf` 可读（fail fast，不碰运行栈）。
2. build 前 `docker tag :latest :rollback`（guard 防缺失）+ **build 先于 up**。
3. `docker compose build --build-arg GOPROXY=... --build-arg GOSUMDB=... api`。
4. `up -d --force-recreate api` → **健康轮询 ≤60s**（if/fi 写法防 `set -e` 误杀；`{{if .State.Health}}` 兜底；unhealthy/超时 → 回退 `:rollback` + up，**回退后再轮询 ≤30s**，仍不健康才 `exit 1`——F2 ①决议：旧镜像理应恢复，若旧镜像也挂=更深层问题，值得报警）。
5. api healthy 后**前端换装**：staging 解压 → `chmod -R a+rX` → `find frontend/dist -mindepth 1 -delete`（清内容**保目录 inode**）→ `cp -a` 原地铺新。
6. `docker compose exec -T web nginx -t && nginx -s reload` → `docker image prune -f` → `docker compose ps`。
- 依据：D5（回退安全）；§6.1/§6.3/§6.4 【决策】；`-T` 防无 TTY（§9 Q5）。

### 5.5 `.github/workflows/deploy.yml`（deploy job 重写 + 双模式）
- 删：buildx/GHCR login/build-push/save-gzip/scp 镜像（移入 scp 模式条件步）；`permissions` 收窄 `contents: read`。
- 留：checkout / node / npm ci / `npm run build`。
- 前端上传（通用）：`tar -czf frontend-dist.tar.gz -C frontend/dist .` → scp `/tmp`（门控保留）。
- scp 模式专属（`if: env.SSH_HOST != '' && vars.DEPLOY_ENABLED == 'true' && vars.DEPLOY_MODE == 'scp'`）：buildx + docker build（`push:false, load:true`）→ save|gzip → scp `/tmp`。
- 部署步（通用，脚本按 `${{ vars.DEPLOY_MODE }}` 分叉）：`set -euo pipefail` → `git pull --ff-only origin main` → mkdir → scp 模式则 load + up + **健康等待 ≤60s**（F3 ①决议：应急路径也不出「deploy 绿站点红」）+ reload / server 模式则 `bash deploy/deploy-server.sh /tmp/ai-forum-frontend.tar.gz`（内含同款健康门）。`command_timeout: 10m`（F6 决议）。
- `on:` 加 `workflow_dispatch`；`command_timeout: 20m`（首次冷机构建余量）；`concurrency: group: deploy-prod` 保留。
- 依据：D1/D2/D5；SOP 门控红线（secrets 不进 if）；actionlint 逐条核对（§2.2）。

### 5.6 文档
- `docs/ops/deployment-v3-8888-container.md` → v3.2（§8 否决记录「服务器本地构建」改采纳 + 补新理由；§3.5/§4 bootstrap 改构建；§5 磁盘卫生补 `builder prune --keep-storage`；**§7 预灌清单补 golang/alpine/docker/dockerfile:1 + bootstrap 前置**——F1/F4）。
- `docs/workflow.md` §9.1/§9.2 重写 + 量化验收（**CI ≤5min**、连续 3 次零超时——F5）；§11 坑位表补 bind-mount inode / `set -e && break` / `exec -T`。
- `docs/ops/lessons-8888-deploy.md` 追加 #43 条目（含**前端换装非原子性**说明——F7）。

## 6. 关键设计裁决（【决策】,含理由与备选）

### 6.1 build 先于 up（回退安全）
- **问题**：服务器构建失败时，部署应如何表现？
- **定案【决策】**：`docker compose build` 在 `up` 之前；build 失败 `set -e` 中止，**旧容器照跑、站点不停摆**。
- **理由**：compose 只在成功时更新 tag、运行容器按 image id 引用 → 构建失败天然不影响运行态。这是「稳定性」的第一道保证。
- **备选（不选）**：先 up 再验证（坏镜像短暂驻留）；不适用——构建是本地操作，失败就该停在 build。

### 6.2 DEPLOY_MODE 双模式作为应急通道
- **问题**：服务器构建环境坏了（builder 镜像损坏/goproxy 不通/构建报错），如何保证 CI 仍能部署？
- **定案【决策】**：deploy.yml 保留 A' 步骤（构建/save/scp/load）但默认被 `DEPLOY_MODE=server` 关掉；翻 `DEPLOY_MODE=scp` + Actions Re-run 即恢复 A' 路径。
- **理由**：回退是「翻一个 variable + 重跑」，**不改代码、不 merge**——应急场景要的就是这个。A' 曾连续 12 次零超时，是可信的成熟路径。
- **备选（不选）**：独立 `deploy-fallback.yml` 工作流（两套工作流需同步维护）；纯 GHCR 备份（服务器拉 GHCR ~2.6KB/s，不可作快速回退，§2.1）。

### 6.3 前端换装保 inode
- **问题**：前端换装如何避免 web 容器（bind mount `frontend/dist`）挂到旧目录？
- **定案【决策】**：`find frontend/dist -mindepth 1 -delete`（清内容、**保目录 inode**）+ staging 解压后 `cp -a` 原地铺新；**严禁 `rm -rf dist && mv`**。
- **理由**：bind mount 绑定的是 inode；换目录 inode 后容器仍挂旧 inode，新文件永不生效（经典坑，Plan agent 校验 R5）。`cp -a` + `chmod a+rX` 保证 nginx worker（uid 101，other）可读。

### 6.4 健康轮询失败自动回退
- **问题**：新镜像被部署但 app 起不来（unhealthy 或一直 starting），怎么办？
- **定案【决策】**：up 后轮询 ≤60s；healthy 通过；unhealthy 或**超时**均回退 `:rollback` + `up --force-recreate`，再轮询 ≤30s 仍不健康才 `exit 1`。
- **理由**：broken 应用可能一直停留在 starting（不一定是 unhealthy），超时也需回退；rollback tag 在 build 前固化（§5.4），是回退源。

### 6.5 单 deploy job（顺序）而非拆并行
- **问题**：要不要把前端/后端拆两个 job 并行？
- **定案【决策】**：**单 deploy job 顺序执行**。
- **理由**：前端 tar ~250-400KB（§2.2），scp ~30-60s 不是瓶颈，并行收益 ~1min；拆两个 job 的版本错配窗口 + 并发 group 语义（同 run 同 group 互相排队/跨 push 交错）协调代价 > 收益。暖 cache 后单 job 可达 ~3-4min。
- **备选（不选）**：双 job 并行（实测超 4min 再拆，需用户接受协调代价）。

### 6.6 GOPROXY 默认不覆盖 CI
- **问题**：Dockerfile 默认 GOPROXY 写什么？
- **定案【决策】**：默认 `https://proxy.golang.org,direct`（GOSUMDB `sum.golang.org`），服务器 build-arg 覆盖为 goproxy.cn。
- **理由**：CI（GitHub runner）可正常访问 proxy.golang.org，默认不覆盖则 CI 行为零变化；仅服务器走 goproxy.cn（D3）。GOPROXY/GOSUMDB 不硬编码进 compose.yml，由部署脚本显式传（保持语义清晰）。

## 7. 边界与不变量清单（含防护层）

| # | 不变量 | 防护层 | 依据 |
|---|---|---|---|
| 1 | 构建失败站点不停摆 | build 先于 up；`set -e` 中止 | §6.1 |
| 2 | 旧镜像可回滚 | build 前 `docker tag :latest :rollback` + inspect guard | §5.4/§6.4 |
| 3 | 坏镜像不驻留 | 健康轮询 ≤60s + 自动回退 + 再轮询 ≤30s 兜底 | §6.4 |
| 4 | 前端换装不破坏 dist inode | `find -delete` 清内容 + staging + `cp -a` 原地铺 | §6.3 |
| 5 | 新前端不配坏 api | 前端换装挪到 api healthy 之后 | §5.4/§6.4 |
| 6 | 前端 tar 损坏 fail-fast | 脚本开头 `[ -s ]` + `tar -tzf` 校验 | §5.4 |
| 7 | nginx 可读新前端 | `chmod -R a+rX`（uid 101 other 读） | §5.4 |
| 8 | 服务器 repo 只 git 改 | SOP §4 红线（CI scp 只落 /tmp + frontend/dist，不写 repo 内文件） | SOP §4 |
| 9 | GOPROXY 服务器=cn、CI 默认不变 | Dockerfile 默认 + build-arg 覆盖分离 | §6.6 |
| 10 | 应急部署可用 | `DEPLOY_MODE=scp` 条件步 + Re-run 重读 vars | §6.2 |
| 11 | 两次 push 不撞部署 | `concurrency: group: deploy-prod, cancel-in-progress: false` | deploy.yml 现状保留 |
| 12 | actionlint 合规 | secrets 只落 job env；step if 只用 env./vars.；本地 + workflow-lint 双校验 | §2.2 |

## 8. 测试与验证计划

- **本地（push 前）**：
  - `bash scripts/check-workflows.sh`（actionlint，本地 Docker 可用，§2.2）。
  - `bash -n deploy/deploy-server.sh`（语法）。
  - `docker compose config`（compose.yml 校验）。
  - 本机 `docker compose build api` 试跑（验证 Dockerfile ARG/cache mount 不破坏 CI 默认路径）。
- **预灌 + 试构建（gated，merge 前）**：本地 `docker pull golang:1.26-alpine alpine:3.21`（不可达则镜像源 retag）→ save|gzip|scp|load → 服务器试构建计时 + `watch free -m`（§2.4 未复核项核实）。
- **试点发版**：1 个含代码改动 PR → merge → CI server 模式部署 → 服务器验证 `git log -1` / `docker compose ps` / `curl http://122.51.233.225:8888/healthz` / `/api/v1/posts` + **实测部署耗时**（目标 deploy job ≤5min，暖 cache 后应更快；验收口径=连续 3 次零超时 + ≤5min——F5）。
- **回退演练**（可选）：`docker tag :latest :rollback && up --force-recreate` 验证可回；设 `DEPLOY_MODE=scp` → Re-run → 验证 A' 路径仍通 → 恢复 `server`。

## 9. 待评审焦点（Q1-Q8）

> Q1-Q8 已在评审意见书 §四 逐条裁决（2026-08-24）：7 认可 + Q2/Q6 附条件。本表保留为原问题清单。

| # | 焦点 | 为什么值得盯 |
|---|---|---|
| Q1 | bootstrap.sh 改显式 build 是否闭环（R1） | §2.2 已核实属实；但 bootstrap 只在全新服务器首启时跑，合并后的回归面需评审确认无遗漏 |
| Q2 | DEPLOY_MODE=scp 回退链路完整性（含前端 tar 统一走 /tmp） | 应急通道是本设计「稳定性」核心（§6.2）；scp 模式脚本与 server 模式共享前端解压逻辑，需确认无断链 |
| Q3 | 镜像一致性漂移：服务器 golang:1.26-alpine（预灌固定）vs CI test job go 1.25 | 编译门 ≠ 产物门（§2.2）；MVP 接受，是否需 pin digest |
| Q4 | GHCR ~2.6KB/s 与「GHCR 最后镜像 = 灾难兜底」的边界 | 纯服务器构建后 GHCR `:latest` 过期（R14）；服务器 rollback tag 才是主回退，评审确认可接受 |
| Q5 | `docker compose exec -T` 的必要性与 `command_timeout: 20m` 余量 | 无 TTY 报错风险（Plan agent R9/R10）；CI 现状不加 -T 能跑因 ssh-action 分配 PTY，统一 -T 是否影响 appleboy/ssh-action 行为 |
| Q6 | 健康轮询 60s 是否足够（api healthcheck interval 30s + start_period 10s） | 首次健康检查最早 ~30s 出结果；60s 窗口含回退余量，需评审确认或建议调整 |
| Q7 | BuildKit cache mount 在服务器 23G 盘上的 GC 边界 | `image prune -f` 不清 buildkit cache；盘压 >5G 才 `builder prune --keep-storage 4G`（§5.6）；勿无脑 prune 删 cache |
| Q8 | 前端「清旧」是否引入新风险（PWA sw 缓存旧哈希 404 瞬态） | 换装后 sw.js 可能短暂引用已删旧 chunk；autoUpdate 自愈，MVP 接受（#14 已知取舍） |

## 10. 评审意见采纳记录（2026-08-24，评审完成 + 用户确认决议）

| 评审项 | 结论 | 采纳落地 |
|---|---|---|
| **F1** BuildKit frontend `docker/dockerfile:1` 预灌盲区 | 重要，属实 | §5.6 预灌清单补 `docker/dockerfile:1` + gated 试构建显式验证（用户确认） |
| **F2** 回退后再轮询散文/脚本不一致 | 重要，属实 | **选①**：脚本补回退后再轮询 ≤30s（§5.4/§6.4 已同步）——用户确认（2026-08-24） |
| **F3** scp 应急模式缺健康门 | 重要，属实 | **加健康门**：scp 模式 up 后健康等待 ≤60s（§5.5 已同步）——用户确认（2026-08-24） |
| **F4** bootstrap 前置未列预灌 | 重要，属实 | §5.6/§5.3 bootstrap 执行清单补 golang/alpine/frontend 预灌硬前置 |
| **F5** 时限预期校准 | 建议，采纳 | **接受 ~4-5min**：验收口径改「deploy job ≤5min + 连续 3 次零超时」（§1/§8 已同步）——用户确认（2026-08-24） |
| **F6** `command_timeout` 20m→10m | 建议，采纳 | §5.5 已改 10m（试构建实测超限再调） |
| **F7** 前端换装非原子性 | 建议，采纳 | 接受 + docs/lessons 记录（不引入 rsync/软链，§6.3 保持） |
| **F8** 「12 次(PR #38→#42)」归属 | 建议，属实 | 计划文档 Context 已改（12 次出自 handoff；#38→#42 为连续 5 次） |

**推翻项**：无。全部评审发现经独立复核属实；Q1-Q8 焦点全部裁决（7 认可 + Q2/Q6 附条件）。

---

*本文档为实现基线（待评审）。评审通过后按 SOP 进入实现（分支 → 实现 → 本地测试 → PR → 用户授权 → 合并 → CI 部署 → 服务器验证 → 回报统筹方更新地图 #1）。*
