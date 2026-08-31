# #74 deploy 静默回退防护：部署后自动核对「新代码在跑」 — 设计文档（供评审）

> 文档用途：交付专业评审 agent 的评审对象。范围 = 背景 / 真值核对 / 决策记录 / 实现方案 / 不变量 / 验证。
> 溯源约定：每个结论标注来源。**事实**标来源（代码 `file:line` / DB 实查输出 / GitHub issue / grilling 用户确认）；**判断性裁决**单独标注【决策】并给出理由与备选，不冒充事实。
> 数据时点：2026-08-31（真值核对执行日；服务器/DB 项均为 2026-08-31 实查）。
> 评审状态：**有条件通过**（2026-08-31 独立评审完成，0 阻塞；F1/F2/F3/F5 条件已采纳，见 §10）。评审意见书：`docs/plans/74-deploy-verify-设计文档-评审意见书.md`。

## §0 项目上下文（给零背景评审 agent，先读本节）

**这是什么**：AI 智联论坛——社团内部 AI 主题社区的移动端论坛 MVP，生产已上线。

- **后端**：Go（Gin + GORM + PostgreSQL），单进程模块化单体（`backend/`）。迁移用 `backend/migrations/migrate.go` 内嵌 SQL。
- **前端**：Vue 3 + TypeScript + Vite + Vant（PWA），纯静态构建产物（`frontend/`）。
- **部署形态**：单台共享服务器（122.51.233.225，无 sudo，ssh 端口 2222）docker compose 三容器：`web`（nginx @8888）/ `api` / `postgres`。compose 文件为 **`compose.yml`**，api service 名 `api`（`compose.yml:7/35/65`）。镜像 `ghcr.io/li-yongqvan/ai-forum-api:latest`。
- **本票完全不涉及后端业务代码、前端代码、DB schema 改动**——只改部署脚本（`deploy/*.sh`）与 CI workflow（`.github/workflows/deploy.yml`）。

**评审必须理解的部署关键纪律**（来源：`docs/workflow.md` §11、`docs/ops/lessons-8888-deploy.md`）：
- 服务器 `~/ai-forum` 内文件**只准 git 改动**，禁 scp/手工写（会挡 CI 的 `git pull`）。
- `.gitattributes` 强制 `deploy/*.sh text eol=lf`（CRLF 会让服务器 bash 崩 `\r: command not found`）。
- `docker compose exec` **吞 stdin** → 一律 `-T` + 命令尾 `< /dev/null`。
- `schema_migrations` 的版本 = **迁移文件名**（`0007_reports_target_author.sql` 与 `0007_tags_schema.sql` 是两条独立记录），比对**必须按文件名**，不能按数字前缀。
- 部署脚本按 `set -euo pipefail` 原则显式 exit，调用方用 `if` 捕获，勿吞。
- 判断「新代码落地」的三指标：① api 镜像 ID/时间 ② `schema_migrations` 版本 ③ 功能冒烟（人工）。**本票自动化①+②，③明确不做**。

## §1 背景与目标

- 需求来源：GitHub **issue #74**「[Ops] deploy 后自动核对『新代码在跑』防静默回退」（2026-08-28 立项，OPEN）。
- 交接依据：`agent panel/handoff-74-deploy-verify.md`（统筹方→执行方，含 §五 8 个待设计问题，本文档逐条作答见附录 A）。
- 事故根因链（issue #74 原文 + 地图 issue #1 决策 35）：`user.users` id 1/2 被硬删 → `invitation_codes` 16 行孤儿引用 → #69 迁移 0010 加物理 FK 校验失败 → **新镜像启动即死**（`cmd/api/main.go:41-43` 迁移失败 → `os.Exit(1)`）→ `deploy-server.sh` 健康轮询失败**自动回退旧镜像** → 旧镜像无 0010，DB 停在 0009 → **deploy check-run 仍绿**。
- 缺口本质：`deploy-server.sh` 把「回退到健康旧镜像」当成功（exit 0），**静默吞掉「新代码没落地」**。
- **目标**：部署完成后自动核对「新代码在跑」——api 容器当前运行镜像 == 本次构建产物 + `schema_migrations` 已应用本次 HEAD 的全部迁移文件；不匹配则本次 deploy 标 FAIL（CI 红）。**只做检测 + 标 FAIL，不做自动修复**（issue #74「候选方案」段 + handoff §二）。

## §2 真值核对（数据来源，全部可复现）

> 以下服务器/DB 项均为 **2026-08-31 实查**（SSH 端口 2222，命令见各表）。

### 2.1 服务器真值（SSH：`cd ~/ai-forum && git log --oneline -1 && docker compose ps`）

| 项 | 实查结果（2026-08-31） | 含义 |
|---|---|---|
| HEAD | `f6b0600 feat(mentions): 帖子/评论 @用户提及... (#73)` | 线上 = #72 事故修复后的最新代码 |
| api 容器 | `Up 3 days (healthy)`，镜像列 `ghcr.io/li-yongqvan/ai-forum-api:latest` | 当前运行态健康 |
| postgres / web | 均 `Up 3 days (healthy)` | 部署形态 8888 容器化 |
| api 运行镜像 | `docker inspect ai-forum-api -f '{{.Image}}'` → `sha256:c4bd545589e69f779a413d6749c287e8a98c66badec8ac821aa82a967f7ba43d` | **运行容器镜像 = 完整 64 位 sha256**（`docker images --no-trunc -q` 同格式，可直接比） |
| `:latest` 标签 | `c4bd545589e6`（3 days ago） | 与运行镜像一致 → 当前无回退 |
| `:rollback` 标签 | `08dd6c4833f9`（5 days ago） | **旧静默回退时代镜像仍在**，可作核对逻辑的对照素材（只读使用，不真造回退） |

→ 结论：当前服务器是**健康基线**（新代码在跑、镜像自洽、无回退）。这既验证了核对逻辑的「期望通过」输入，也确认 `rollback` 镜像可作为测试素材。

### 2.2 DB 真值（SSH + psql）

命令：`docker compose exec -T postgres psql -U forum -d forum -tA -c "SELECT version FROM schema_migrations" < /dev/null`

实查输出（13 条，0001–0012 全部）：
```
0001_user_schema.sql
0002_content_schema.sql
0003_notify_schema.sql
0004_moderation_schema.sql
0005_content_seed.sql
0006_reports_reporter_note.sql
0007_reports_target_author.sql
0007_tags_schema.sql
0008_notify_type_report_handled.sql
0009_moderation_actions_index.sql
0010_fix_user_intra_package_fks.sql
0011_content_mentions.sql
0012_notify_type_mention.sql
```

→ 结论：DB 已应用全部 13 个迁移文件（含 **0007×2 两条独立记录**），与仓库 `backend/migrations/*.sql` 集合完全一致（见 2.3）。当前 DB 真值 = 「期望通过」输入；同时证明「比文件名、不比数字前缀」的必要性。

### 2.3 代码真值（本机仓库 `C:\Users\liyongquan\ai-forum-74`，HEAD f6b0600）

**`deploy/deploy-server.sh`（63 行，本次改动主体）**：
- `L10` `set -euo pipefail`。
- `L22-25` **build 前**把当前 `:latest` 固化 tag 成 `:rollback`（仅当 `:latest` 存在时，`docker image inspect` 守卫）。
- `L28` `docker compose build --build-arg GOPROXY=... api`（服务器本地构建）——**成功后 `:latest` = 新镜像**。
- `L31` `docker compose up -d --force-recreate api`。
- `L32-33` `wait-healthy.sh ai-forum-api 60` 首轮健康轮询。
- `L34-45` 首轮 unhealthy/超时 → 回退分支：`docker tag $IMG:rollback $IMG:latest` → 再 up → `wait-healthy.sh ai-forum-api 30` 二轮。
- `L46` `[ "$ok" -eq 1 ] || { echo "FATAL: api not healthy after rollback" >&2; exit 1; }` —— **回退成功也走到这里且 exit 0**，这就是静默盲区：健康 ≠ 新代码。
- `L48-54` 前端换装（保 dist inode：`find -delete` + staging `cp -a`）。
- `L57-58` nginx `-t && -s reload`；`L60-63` prune + 删 tar + `compose ps`。

**`deploy/wait-healthy.sh`**（共享辅助，契约）：
- 用法 `bash deploy/wait-healthy.sh [容器名] [超时秒数]`，默认 `ai-forum-api`/60s；healthy→exit 0，unhealthy/超时→exit 1。**调用方必须用 `if ... then` 捕获**（脚本头注释 + `docs/plans/issue-43-deploy-optimization-设计文档-评审意见书.md` F3 决议）。

**`backend/migrations/migrate.go`（迁移机制，核对的「期望集」来源）**：
- `L16-17` `//go:embed *.sql` 内嵌全部迁移文件；`list()` 按**文件名字典序**（`L77-91`）。
- `L35-58` 逐条应用：已应用（`appliedSet`，`L93-111`）跳过，否则事务内执行 + `INSERT INTO schema_migrations(version)`。**版本 = 文件名**（`0007×2` 是两条）。
- `L25-30` 建表 `schema_migrations(version VARCHAR(255) PRIMARY KEY, applied_at)`。
- `backend/cmd/api/main.go:41-43`：`migrations.Run(sqlDB)` 失败 → `slog` + `os.Exit(1)` → **迁移失败 = 容器死**（这是回退被触发的根因通道）。

**`.github/workflows/deploy.yml` deploy job**：
- `L148` ssh 脚本内 `git pull --ff-only origin main`（先 pull：保证新 `deploy-server.sh`/`verify-deployed.sh` 就位）——**部署时服务器仓库已在本次 HEAD**，这是「期望迁移集 = 仓库文件集」的成立前提（见 §6.4）。
- `L150-158` **scp 应急分支**：`docker load` → up → `wait-healthy.sh 60 || exit 1` → `tar -xzf` 换装 → `nginx -t && reload` → prune → ps。**无回退、无静默**，但**无迁移核对**（本票 D2 补齐）。
- `L160` **server 分支**：`bash deploy/deploy-server.sh /tmp/ai-forum-frontend.tar.gz`（内含 wait-healthy + prune + ps）。
- **关键**：deploy-server.sh / 内联脚本非零退出 → ssh-action 步（`set -euo pipefail`）失败 → **deploy job 自然 FAIL**（无需改 CI 层即可标红，来源：handoff §4.2 + `deploy.yml:145-161`）。

**调用方 grep 证据**（改脚本前确认无隐藏调用方）：
- `wait-healthy.sh` 调用方：`deploy/deploy-server.sh:33,41`、`.github/workflows/deploy.yml:154`。均保留不变。
- `deploy-server.sh` 调用方：`.github/workflows/deploy.yml:160`（仅此一处）。新增 verify 调用只在脚本内部，不影响该调用面。
- `.gitattributes`：`deploy/*.sh text eol=lf` 已存在 → 新增 `deploy/verify-deployed.sh` **自动 LF**，无行尾风险。

### 2.4 GitHub 状态（2026-08-31）

- issue #74：**OPEN**（正文见 §1）。本地 main `f6b0600` == 服务器 HEAD == `origin/main`（三处一致）。
- 本票分支 `feat/74-deploy-verify`：新建于 worktree `C:\Users\liyongquan\ai-forum-74`，仅本设计文档未提交。开工前已 `git status --porcelain` 确认主仓库干净。

## §3 Grilling 决策记录

| 编号 | 决策问题 | 定案 | 依据 |
|---|---|---|---|
| D1 | 核对失败（检测到回退/迁移缺失）时前端换装怎么处理？ | **跳过前端换装，整站停旧码，deploy 红**（不换装、不 reload） | 用户确认（2026-08-31，handoff §五 Q5 访谈）；理由：避免「新前端 + 旧 api」接口错配（handoff §二 警示）；站点整体版本自洽、可用性优先 |
| D2 | scp 应急模式（DEPLOY_MODE=scp）是否覆盖「新代码在跑」核对？ | **抽共享脚本 `deploy/verify-deployed.sh`，两模式都调用** | 用户确认（2026-08-31，handoff §五 Q6 访谈）；理由：应急路径当前无迁移核对（§2.3），成本低（一脚本 + deploy.yml 一行）；scp 模式镜像核对为形式性（见 §7 #8） |
| D3 | 方案 A（deploy-server.sh 内置核对）vs 方案 B（CI 独立 check 步） | **方案 A 为主，B 不做** | handoff §二「CI 标 FAIL」段 + §4.2 已论证 A 天然覆盖双模式、非零退出自动红 CI；B 显式但重复造轮子。A 达成全部目标（§6.1【决策】） |
| D4 | 是否做自动修复/自动重试？ | **不做**（本次只做检测 + 标 FAIL） | handoff §二「明确不做」+ issue #74 目标原文 |
| D5 | 是否动既有迁移文件（0001–0012）？ | **不动** | handoff §八 红线；本次只加核对不改迁移 |
| D6 | 是否做功能级冒烟自动化？ | **不做**（功能冒烟仍靠人工） | handoff §二「明确不做」；#72 开放项 |

> 决策落档：D1/D2 为本次访谈新增，已附决策原文（问题+定案+弃选理由）于本表，后续会话可复核；D3-D6 源于 handoff 已定方向（2026-08-28 统筹方/用户确认）。

## §4 范围收敛与明确不做

| 项 | 决策 | 依据 |
|---|---|---|
| 新建 `deploy/verify-deployed.sh` | 做（核心） | §5.1；D2 |
| `deploy/deploy-server.sh` 内置核对 | 做 | §5.2；D3 |
| `deploy.yml` scp 分支补核对 | 做 | §5.3；D2 |
| 自动修复 / 自动重试 | 不做 | D4 |
| 功能冒烟自动化 | 不做 | D6 |
| 部署架构改动 / 新第三方依赖 / 新工具 | 不做 | handoff §八 红线 |
| 既有迁移文件改动 / DB 数据改动 | 不做 | D5；红线「不删生产数据」 |
| 后端业务代码 / 前端代码改动 | 不做（零代码改动，纯 Ops） | 本票范围界定（§0） |

**跨票并行隔离声明**：当前无已知并行票改动 `deploy/` 或 `deploy.yml`（本地 worktree 隔离 + 开工前 `git status --porcelain` 干净，§2.4）。若评审期间出现并行会话碰 `deploy.yml`/`deploy/*.sh`，需按 SOP §4.1 rebase 对齐后再合并——本声明仅核了 `deploy/` 与 `.github/workflows/deploy.yml` 两个共享装配点（`verify-deployed.sh` 无外部调用面）。

## §5 实现方案（每项给出依据）

### 5.1 新建 `deploy/verify-deployed.sh`（核心，D2 的共享逻辑）

**契约**：
```
用法：bash deploy/verify-deployed.sh [期望api镜像ID] [容器名]
  - 期望镜像 ID 非空 → 核对①：运行容器镜像 == 期望（防回退到旧码）
  - 核对②：schema_migrations 已应用集合 ⊇ 仓库 backend/migrations/*.sql 文件名集合（防 DB 停在旧版）
  - 全部通过 exit 0；任一失败 exit 1（调用方 set -e 自然中止 deploy，CI 红）
前置条件：仓库已 git pull 到本次 HEAD（deploy.yml L148 保证）；cwd 可 cd 到 $HOME/ai-forum
```

**核对①（镜像）**：`docker inspect -f '{{.Image}}' "$CT"` 取运行镜像；与传入期望 ID 比对前**统一去 `sha256:` 前缀**后精确相等。两者格式均为完整 64 位（§2.1 实查 `sha256:c4bd...`；`docker images --no-trunc -q` 同格式），**不用短 ID 前缀匹配**（§6.3）。
- 期望为空 → 跳过镜像核对（防御性：捕获失败时 fail-closed 由调用方守卫，见 §5.2/5.3）。

**核对②（迁移）**：一次 psql 取已应用集（`docker compose exec -T postgres psql -U forum -d forum -tA -c "SELECT version FROM schema_migrations" < /dev/null`，必须 `-T` + `< /dev/null`，SOP §11），与 `backend/migrations/*.sql` basename 集合逐一 `grep -qxF` 比对，**只查缺失**（DB 多出的记录不判失败）。
- psql 查询失败 → `set -euo pipefail` 下命令替换失败即中止 → **fail-closed**（§6.6）。
- 比对按**完整文件名**（`0007×2` 两条独立记录），不按数字前缀（§2.2/2.3）。

**失败输出（可读告警，§6.7）**：明确打印 `VERIFY-FAIL` + 原因 + 关键对账数据（运行镜像 vs 期望镜像；缺失迁移文件名清单），让 CI 日志一眼定位。

**依据**：D2（共享脚本）；镜像格式事实 §2.1；迁移机制 §2.3；`wait-healthy.sh` 同款「显式 exit 不吞」约定（§2.3）。

### 5.2 `deploy/deploy-server.sh` 改动（两处插入，依据 D1/D3）

1. **捕获期望镜像**（`L28` build 之后、`L31` up 之前插入）：
   ```bash
   # 捕获期望镜像：build 成功后、up 前（回退分支会 retag :latest，必须在 up 前抓）
   EXPECTED_IMAGE="$(docker images --no-trunc -q "$IMG:latest")"
   [ -n "$EXPECTED_IMAGE" ] || { echo "FATAL: 无法捕获 $IMG:latest 镜像 ID（构建产物缺失）" >&2; exit 1; }
   ```
   - 依据：捕获时机必须在**任何 retag 之前**（回退分支 `L38` `docker tag rollback latest` 会污染 `:latest`，§6.2）；fail-closed 守卫防空值静默跳过镜像核对。
2. **核对调用**（`L46` 健康确定之后、`L48` 前端换装之前插入）：
   ```bash
   # 核对「新代码在跑」（#74 防静默回退：健康 ≠ 新代码；失败=已回退/迁移缺失 → 跳过换装 + exit 1）
   if ! bash deploy/verify-deployed.sh "$EXPECTED_IMAGE"; then
     echo "DEPLOY-FAIL: 新代码未真正上线。站点健康停在旧码；本次 deploy 标记 FAIL（CI 红）。" >&2
     exit 1
   fi
   ```
   - 依据：插入点恰好区分「新镜像健康在跑」（核对通过 → 继续换装）vs「已回退到旧镜像」（核对失败 → **跳过换装** exit 1，D1）。原 `L48-54` 前端换装、`L57-58` reload 仅在核对通过后执行。

### 5.3 `.github/workflows/deploy.yml` scp 分支改动（D2 + F1 补强）

1. **CI 构建步捕获期望镜像 ID**（「Save and compress API image」步，`deploy.yml:120-124`，加 `id: api-image`）：build+load 后 `docker image inspect ghcr.io/li-yongqvan/ai-forum-api:latest --format '{{.Id}}'` → 写入 `GITHUB_OUTPUT`（`steps.api-image.outputs.image_id`）。
2. **scp 内联脚本**（`L154` `wait-healthy` 之后）插入核对，期望镜像 ID 由 CI 传入：
```bash
bash deploy/wait-healthy.sh ai-forum-api 60 || exit 1
bash deploy/verify-deployed.sh "${{ steps.api-image.outputs.image_id }}" || exit 1
```
- 依据：期望镜像 ID 由 **CI 构建侧捕获**（而非服务器 load 后读 `:latest`）——彻底堵死「load 到旧镜像 / 重跑旧 workflow」时镜像核对形式性通过的漏检（F1，用户确认 2026-08-31 补强）；`|| exit 1` 与既有 `wait-healthy` 行同风格（`deploy.yml:154`）。
- 改动后跑 `bash scripts/check-workflows.sh` 校验 workflow（SOP §0；本地 Docker daemon 未启动时依赖 CI `workflow-lint` 兜底，见 §8.4）。

## §6 关键设计裁决（【决策】，含理由与备选）

### 6.1 方案 A vs 方案 B（D3 展开）
- **问题**：核对逻辑放哪——deploy-server.sh 内置（A）还是 CI 加独立 check 步（B）？
- **定案【决策】**:方案 A。`deploy-server.sh` 内置 + 抽共享 `verify-deployed.sh`（D2 同时覆盖 scp 内联脚本）。
- **理由**:A 天然覆盖两种 DEPLOY_MODE；核对位置就在回退逻辑旁边，时序最直观；非零退出经 ssh-action 步（`set -euo pipefail`）**自动红 CI**，零 CI 层改动（§2.3）。
- **备选（不选）**:方案 B（CI 独立 check job 部署后 SSH 查服务器）——显式但重复造轮子、多一跳 SSH、且「查到的真值」与「脚本内时序」分离更易出错；B 无法比 A 提供更多保证。

### 6.2 期望镜像捕获时机
- **问题**:何时捕获「本次新构建镜像」才可靠？
- **定案【决策】**：**build 成功后、up 前**（server 模式服务器捕获 §5.2；scp 模式由 CI 构建步捕获后传参 §5.3/F1），存变量贯穿脚本。
- **理由**:回退分支 `docker tag rollback latest`（`deploy-server.sh:38`）会**改掉 `:latest` 指向**，任何在 up 之后捕获的实现都会把「旧镜像」误当期望。变量在捕获点后不再读取 `:latest`，天然免疫 retag。
- **备选（不选）**:up 后捕获（会取到回退后的 `:latest` → 镜像核对恒真、静默退化）；用 build 输出日志解析（脆弱、依赖 BuildKit 输出格式）。

### 6.3 镜像 ID 对比格式
- **问题**:`docker inspect -f '{{.Image}}'`（完整 64 位）vs `docker images -q`（默认短 12 位）如何对齐？
- **定案【决策】**：**统一完整 ID**——捕获用 `docker images --no-trunc -q`（完整 64 位），对比前对两侧 `strip sha256:` 前缀后**精确相等**。
- **理由**:格式对齐（§2.1 实查两侧均为 `sha256:<64hex>`）零歧义；短 ID 前缀匹配在镜像多时理论可撞（极小概率但没必要冒）。
- **备选（不选）**:短 ID 前缀匹配（省一字符但引入碰撞语义）；sed 正则提取（过度加工）。

### 6.4 迁移期望集来源
- **问题**:期望迁移集用仓库 `backend/migrations/*.sql`（部署时 HEAD）还是 api 镜像内嵌集合？
- **定案【决策】**：**仓库文件集合**。
- **理由**:① 部署时服务器已 `git pull --ff-only` 到本次 HEAD（`deploy.yml:148`），仓库集合 == 本次意图集合；② 期望集**独立于运行二进制**——若已回退到旧镜像，旧镜像内嵌集合是旧的（小），仓库集合是新 HEAD（大），正好用「新期望 ⊄ 旧实际」暴露缺口——**这正是本票要抓的信号**；③ 无容器依赖、纯文件系统读取，最简单可审计。
- **备选（不选）**:镜像内嵌集合（`docker run --rm <img> ls /migrations` 之类）——需额外容器调用、且当镜像恰好是旧码时期望集同步变小、核对失真。

### 6.5 核对失败语义
- **问题**:核对失败（回退/迁移缺失）对前端换装与退出码的语义？
- **定案【决策】**：**立即 `exit 1`，跳过前端换装与 nginx reload**（D1）。
- **理由**:整站停在「旧前端+旧 api」= 版本自洽、可用性优先；「新前端+旧 api」错配 = 交接单点名的隐患（handoff §二）。deploy 红（CI FAIL）与站点健康可并存——文案区分「deploy 失败但站点健康停旧码」vs「站点也挂了」。
- **备选（不选）**:先换装再 exit 1（制造错配态）；核对失败不退出仅 WARN（回到静默盲区）。

### 6.6 psql 核对失败 fail-closed
- **问题**:迁移核对查询失败（DB 不可达/psql 报错）怎么处理？
- **定案【决策】**：**fail-closed**——psql 失败即脚本中止、deploy 红。
- **理由**:核对查询失败 = 无法证明新代码在跑 = 按未通过处理；静默吞掉会重演「绿但没落地」。`set -euo pipefail` 下命令替换失败自然中止，无需额外分支。
- **备选（不选）**:`|| true` 吞错（重演静默盲区）。

### 6.7 失败信息可读性
- **定案【决策】**：所有核对失败以 `VERIFY-FAIL`/`DEPLOY-FAIL` 前缀 + 具体对账行（运行镜像/期望镜像/缺失迁移清单）输出到 **stderr**，调用方（ssh-action）会把 stderr 打进 CI 日志 → 一眼定位是「镜像回退」还是「迁移缺失」。
- **理由**:handoff §五 Q8「可读告警」要求；stderr 随非零退出保留。

## §7 边界与不变量清单

| # | 不变量 | 防护层 | 依据 |
|---|---|---|---|
| 1 | 回退后站点健康停在旧码，但本次 deploy 必红 | `verify-deployed.sh` 非零退出 → ssh-action 步失败 → deploy job FAIL（`deploy.yml:145-161`） | §6.5 / §2.3 |
| 2 | 前端换装与 nginx reload 仅在「新 api 健康 + 核对通过」后发生 | 核对调用插在 `deploy-server.sh` L46 后、L48 换装前；核对失败 `exit 1` 截断后续 | §5.2 / D1 |
| 3 | 期望镜像捕获不被回退 retag 污染 | 捕获点在 build 后、up 前；之后只用变量不再读 `:latest` | §6.2 / `deploy-server.sh:38` |
| 4 | 期望镜像捕获失败时 fail-closed | `[ -n "$EXPECTED_IMAGE" ] || exit 1` 守卫 | §5.2 |
| 5 | 期望迁移集 = 仓库文件集（部署时已 pull 到新 HEAD） | `deploy.yml:148` `git pull --ff-only` 先行；`set -euo pipefail` 下 pull 失败即止 | §6.4 |
| 6 | 迁移比对按文件名（`0007×2` 两条），DB 多出记录不判失败 | `grep -qxF "$b"`（完整文件名精确行匹配）；只查缺失方向 | §2.2 / §2.3 |
| 7 | psql 查询失败 fail-closed | 命令替换失败 + `set -euo pipefail` 自然中止 | §6.6 |
| 8 | scp 模式镜像核对不退化：期望镜像 ID 由 CI 构建步捕获传入（非 load 后读 `:latest`），旧 workflow/旧 tar 重跑必被镜像或迁移核对拦下 | `steps.api-image.outputs.image_id` 传入 verify（F1 补强，用户确认 2026-08-31） | §5.3 / F1 |
| 9 | （已知缺口）功能冒烟仍靠人工 | 明确不做自动化（D6）；本票只保证「代码/DB 落地」，不保证「功能正确」 | D6 |
| 10 | 服务器 `~/ai-forum` 文件只经 git 改动 | 新脚本随 PR 合并进仓库，不 scp 写入；`.gitattributes` 强制 LF | §2.3 / SOP §11 |

## §8 测试与验证计划

> 目标：证明「核对逻辑能抓到静默回退」且「正常发版不误报」。**不造坏迁移、不删生产数据、不真回退生产容器**（红线，handoff §八）——用只读对抗输入达成。

### 8.1 本地 shell 逻辑自测（无 Docker 依赖的部分）
- `verify-deployed.sh` 已按「可测结构」实现：`verify_image()`/`verify_migrations()` 函数化 + 第 3 参/第 1 参测试注入 + `VERIFY_SOURCE_ONLY=1` 可 source（生产路径无旁路 env）。本地喂假值断言：① 相同镜像 ID → exit 0；② 不同镜像 ID（模拟回退）→ exit 1 并打印对账；③ 迁移集合含全部期望 → exit 0；④ 缺一个文件名 → exit 1 并点名；⑤ DB 超集（多一条）不判失败 → exit 0；⑥ 空已应用集 → exit 1。**评审实测 7/7 断言全过（2026-08-31）**。
- 依据：函数化 + 注入参数避免给脚本留「可被环境变量放行」的旁路后门；纯 bash 断言脱离 Docker 可跑（`cd` 到仓库根以取 `backend/migrations/*.sql`）。

### 8.2 服务器只读对抗验证（关键，2026-08-31 基线可用）
> 全部只读：不改容器、不动 DB、不重排生产栈。在**当前健康基线**（§2.1/2.2）上执行：
1. **镜像核对「抓回退」**：以 `rollback` 镜像短 ID 作为「期望」参数（实际运行是 `c4bd...`）跑 `verify-deployed.sh` → 应 `exit 1` 并打印 running≠expected。→ 证明：若真回退到旧镜像，核对必红。
2. **迁移核对「抓缺失」**：**本地 source 单测覆盖**（§8.1：注入缺 0010 的已应用集 → `exit 1` 并点名，评审实测通过）；**服务器端不做「临时建文件」注入**（违反「服务器 repo 只准 git 改动」红线）。→ 证明：DB 停在旧版时核对必红。
3. **不误报**：以当前真实期望（运行镜像 `sha256:c4bd...` + 仓库 13 个迁移文件）跑 → 应 `exit 0`（`VERIFY-OK`）。→ 证明：正常发版不误报。
4. **psql fail-closed**：故意用错 DB 名（`-d wrongdb`，临时参数注入）→ 应非零退出。→ 证明：DB 查询失败按失败处理。

### 8.3 真实发版回归（合并后，CI 触发）
- 正常发版：deploy job 应绿，日志含 `VERIFY-OK: 新代码在跑`（**核对确实跑了**，不是「核对没跑但绿」）；服务器 HEAD/镜像/迁移三对齐（SOP §9.4）。
- 若未来某次真发生回退：核对应使 deploy job 红（本次不人为制造）。

### 8.4 workflow 校验
- 改 `deploy.yml` 后跑 `bash scripts/check-workflows.sh`（SOP §0，actionlint Docker 校验）——**注意**：该脚本依赖本地 Docker daemon 运行中（`check-workflows.sh:14`）；daemon 未启动时（2026-08-31 实测）依赖 CI `workflow-lint` job（push 触发、阻塞，SOP §7）作为兜底，或临时改用轻量 YAML 解析自检（本票已用 Python `yaml.safe_load` 通过）。

### 8.5 本地全量（无代码改动，仅确认不受影响）
- `cd backend && go build ./... && go vet ./... && go test ./...`（确认零代码改动不破坏）；前端不动。

## §9 待评审焦点（Q1-QN）

> 给评审 agent 的定向问题清单——作者最想让对方盯的点。

- **Q1（镜像核对误报风险）**：`docker compose build` 成功后 `docker images --no-trunc -q "$IMG:latest"` 是否**恒等于** `up --force-recreate` 创建容器的 `{{.Image}}`？有无 BuildKit 多阶段/标签别名场景会使两者不一致 → 健康新代码被误判失败？作者认为无（build 即 retag `:latest`，up 即从 `:latest` 创建），但这是**最不该误报**的点，请独立核验。
- **Q2（新镜像 dangling 卫生）**：核对失败 exit 1 时，`docker image prune -f`（原 L61）不会执行 → 新构建镜像留作 dangling，直到下次成功部署的 prune 清理。是否接受？（作者：接受，失败态留下镜像反而便于事后比对，且不碰运行栈。）
- **Q3（scp 模式漏检边界）**：~~§7 #8 已知缺口~~ **已裁决：补强**（用户确认 2026-08-31）——CI 构建步捕获镜像 ID 传入 verify（§5.3/F1），漏检关闭。留档备查。
- **Q4（失败语义可读性）**：`DEPLOY-FAIL`/`VERIFY-FAIL` 两级前缀能否让运维在 CI 日志一眼区分「deploy 失败但站点健康停旧码」vs「站点也挂了」（后者是 L46 的 `FATAL: api not healthy after rollback`）？文案是否需再收敛措辞？
- **Q5（仓库文件集作为期望的时效边界）**：期望迁移集依赖「部署时服务器 git pull 已到新 HEAD」（`deploy.yml:148`）。若该 pull 被 `set -e` 之外的方式半成功（如 `--ff-only` 在非干净工作区失败但被忽略）→ 期望集少算 → 迁移核对失真。`--ff-only` + `set -euo pipefail` 是否已足够护栏？作者认为够（pull 失败即中止整个 ssh 脚本），请评审确认无旁路。

## §10 评审意见采纳记录（2026-08-31）

| 评审项 | 结论 | 采纳落地 |
|---|---|---|
| Q1 镜像核对误报风险 | 认可（低风险） | 服务器实查两侧格式逐字相等；维持 `norm()` 双保险（§6.3） |
| Q2 新镜像 dangling 卫生 | 认可 | 失败态保留镜像便于事后比对；维持现状 |
| Q3 scp 漏检边界 | 裁决：补强（用户确认） | CI 构建步捕获镜像 ID 传入 verify（§5.3）；§7 #8 已更新 |
| Q4 失败语义可读性 | 认可 + 建议 | `VERIFY-FAIL`/`DEPLOY-FAIL` 文案含运维指导句（§6.7/§5.2） |
| Q5 git pull 护栏 | 认可（无旁路） | 维持 `--ff-only` + `set -euo pipefail` |
| F1 scp 镜像核对形式性 | 重要 → 补强 | 见 Q3；用户确认 2026-08-31 |
| F2 决策落档 | 重要 | 新建 `docs/handoffs/grilling-decisions/issue-74-deploy-verify-decisions.md`（D1-D6，随 PR） |
| F3 check-workflows 需 Docker | 重要 | §8.4 改写：daemon 未起时走 CI `workflow-lint`/轻量解析兜底 |
| F4 空期望静默跳过 | 建议 | verify-deployed.sh 空期望跳过 + 调用方守卫已注释明示 |
| F5 节注释重排 | 建议 | deploy-server.sh 已重排为 0-8（§5.2 落地） |
| F6 真实发版断言 VERIFY-OK | 建议 | §8.3 增加「核对行非空」断言 |

**推翻项**：无。评审方未复核的 SSH/线上 DB 项均已在 §2 实查；F2（决策落档）随 PR 一并合入。

---

## 附录 A：交接单 §五 8 问 → 本文档作答索引

| 交接单问题 | 结论 | 出处 |
|---|---|---|
| Q1 核对位置 A vs B | **A**（deploy-server.sh 内置 + 共享脚本），B 不做 | §3 D3 + §6.1 |
| Q2「新镜像」捕获时机 | **build 成功后、up 前**（scp 为 load 后、up 前），存变量免疫 retag | §6.2 |
| Q3 镜像 ID 对比格式 | 完整 64 位（`--no-trunc`）+ 去 `sha256:` 前缀精确相等 | §6.3 |
| Q4 迁移期望集来源 | **仓库文件集合**（部署时已 pull 到新 HEAD） | §6.4 |
| Q5 失败语义 + 前端换装联动 | 核对失败 → **跳过换装 + exit 1**（D1）；文案区分「deploy 失败但站点健康停旧码」vs「站点挂」 | §3 D1 + §6.5 + §9 Q4 |
| Q6 scp 应急模式 | **抽共享 `verify-deployed.sh`，两模式都核**（D2）；scp 镜像核对形式性局限见 §7 #8 | §3 D2 + §5.3 |
| Q7 测试策略 | 本地 shell 自测 + 服务器**只读对抗**验证（错期望/缺迁移/不误报/psql fail-closed）+ 真实发版回归；不造坏迁移不删数据 | §8 |
| Q8 可读告警 | `VERIFY-FAIL`/`DEPLOY-FAIL` 前缀 + 对账数据（运行/期望镜像、缺失迁移清单）输出 stderr | §6.7 |

---

*本文档为实现基线。待 `plan-review` 评审通过后进入实现（改 deploy-server.sh → 新建 verify-deployed.sh → 改 deploy.yml → 测试 → PR → 用户授权 → 合并 → 服务器验证 → 回报统筹方更新地图 #1）。*
