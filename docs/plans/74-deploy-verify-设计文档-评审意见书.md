# #74 deploy 静默回退防护 — 设计文档评审意见书

> **评审对象**：《#74 deploy 静默回退防护：部署后自动核对「新代码在跑」 — 设计文档（供评审）》`docs/plans/74-deploy-verify-设计文档.md`（2026-08-31）
> **评审方式**：独立对抗评审（prosecutor protocol Pass 0-5）。真值源 = 本机仓库 `C:\Users\liyongquan\ai-forum-74` @ `f6b0600`（worktree，已 pull 对齐远端）+ 生产服务器只读实测（SSH 2222，2026-08-31）+ 本地原型实测。
> **评审结论**：**有条件通过**（0 阻塞）。**采纳状态：F1 已定案补强（用户确认 2026-08-31），F2/F3/F5 已落地，设计文档 §10 已回填采纳记录；本意见书为最终版。**

---

## 一、总体结论

设计方向正确且证据纪律扎实：事故根因链与部署脚本盲区定位准确，方案 A（内置核对 + 共享脚本）被论证为天然覆盖双模式、零 CI 层改动即可红，符合 issue #74「只检测 + 标 FAIL、不自动修复」的目标。**真值核对部分经独立实测全部命中**——服务器真值、DB 迁移真值、镜像 ID 格式、脚本行号、调用方 grep、`.gitattributes`，无一失实；这在本项目历史里属上乘。

评审方额外完成了两项对抗性实测，均通过：

1. **镜像 ID 格式等价性**（设计 Q1 的核心疑云）——服务器实查 `docker images --no-trunc -q :latest` 与 `docker inspect -f '{{.Image}}'` **均为 `sha256:` 前缀 + 完整 64 位，逐字相等**。设计的 `norm()` 去前缀是双保险而非必需，误报风险低。
2. **核对逻辑可执行性**——按设计 §5.1 原型化 `verify-deployed.sh`（含测试注入钩子），5 场景 6 断言全过：快乐路径 exit 0、回退被抓 exit 1、迁移缺失点名 exit 1、DB 超集不误判 exit 0、空期望跳过镜像核对 exit 0。

**必须动身前消化的问题集中在两处（均低成本）**：① scp 应急模式的镜像核对形式性局限需要裁决（补 CI 侧传镜像 ID，或显式接受 §7 #8）；② 决策落档、测试计划的两处执行机制需要钉死。见 §四/§五。

---

## 二、事实与证据复核

### 2.1 核实为真（独立复核，全部命中）

| 计划主张 | 复核结果 |
|---|---|
| 服务器 HEAD `f6b0600`（#73） | ✅ SSH 实查 `git log --oneline -1` → `f6b0600 feat(mentions)... (#73)` |
| api/postgres/web 三容器 healthy | ✅ `docker compose ps` 实查 `Up 3 days (healthy)` |
| `docker inspect ai-forum-api -f '{{.Image}}'` = `sha256:c4bd...43d` | ✅ 实查，完整 64 位带 `sha256:` 前缀 |
| `:latest` = `c4bd545589e6`（3 days）、`:rollback` = `08dd6c4833f9`（5 days） | ✅ 实查（`:rollback` 全量 `sha256:08dd...c21` 已取，可作测试素材） |
| `schema_migrations` 13 条、含 `0007×2` | ✅ psql 实查 13 条，0001–0012 全部，两条 0007 独立 |
| `deploy-server.sh` 盲区在 L34-46（回退收敛 exit 0） | ✅ 全文 Read 核对，L46 `[ "$ok" -eq 1 ] || ... exit 1` 是唯一健康门，回退成功同样走到 |
| `deploy-server.sh` 插入点 L46 后 / L48 换装前 | ✅ 行号核对准确（L48-54 前端换装、L57-58 reload、L60-63 prune） |
| `wait-healthy.sh` 契约（默认 ai-forum-api/60s，healthy→0） | ✅ 全文 Read 核对 + `deploy.yml:154`/`deploy-server.sh:33,41` 调用点 |
| `migrate.go` 版本 = 文件名、`appliedSet` 按 `SELECT version` | ✅ `migrate.go:35-58/L93-111` 核对 |
| `main.go:41-43` 迁移失败 `os.Exit(1)` | ✅ grep `migrations.Run` 命中 L41，上下文 `os.Exit(1)` 属实 |
| `deploy.yml:148` `git pull --ff-only` 先于部署 | ✅ 实查，L148 在 L160 之前 |
| `deploy.yml` scp 内联分支无迁移核对（L150-158） | ✅ sed 实查 L150-161，确认 `wait-healthy || exit 1` 后直接换装，无迁移/镜像核对 |
| `deploy.yml:117` scp 构建 tag = `:latest` | ✅ 实查 build-push-action `tags: ghcr.io/li-yongqvan/ai-forum-api:latest` |
| `.gitattributes` `deploy/*.sh text eol=lf` | ✅ 实查，新增 `verify-deployed.sh` 自动 LF |
| `compose.yml` service `api` / container `ai-forum-api` | ✅ 实查 |
| **`docker images --no-trunc -q` 与 `{{.Image}}` 格式相等** | ✅ 服务器实查：两侧均 `sha256:` + 64 位，逐字相等（Q1 核心疑云解除） |
| **核对逻辑 5 场景行为**（设计 §8.1） | ✅ 本地原型实测 6/6 断言过（含回退被抓、迁移缺失点名、超集不误判） |

### 2.2 不实 / 冲突

**无。**

### 2.3 不可复核 / 无法实测

| 项 | 说明 |
|---|---|
| `scripts/check-workflows.sh`（§8.4 依赖） | ⚠️ **无法本地实测**：该脚本经 `docker run rhysd/actionlint` 执行（`check-workflows.sh:14`），本地 Docker daemon 未启动（2026-08-31 实测连接失败 `npipe...not found`）→ 见 F4 |
| 设计 §8.2 测试 2「临时塞不存在的迁移文件名」的具体机制 | ⚠️ **无法按原文执行**：verify 的期望集直接读仓库 `backend/migrations/*.sql`，「不建文件」就无法改变比对输入；在服务器仓库临时建文件违反「只准 git 改动」红线 → 见 F5 |
| 最终 deploy-server.sh / deploy.yml 落地形态 | 设计阶段尚未实现，无法执行验证；核心逻辑已由原型覆盖，落地后按 §8 回归 |

---

## 三、逐条评审

| 决策/选择 | 结论 | 评审意见 |
|---|---|---|
| D1 核对失败跳过前端换装、整站停旧码 | **认可** | 与「防新前端+旧 api 错配」（handoff §二）一致；用户确认（2026-08-31）已记录。落档见 F2。 |
| D2 抽共享 `verify-deployed.sh`、双模式都核 | **认可（附条件）** | 覆盖 scp 应急路径正确。条件：scp 模式镜像核对为形式性（见 Q3 裁决 + §7 #8），需补强或显式接受。 |
| D3 方案 A 为主、B 不做 | **认可** | 论证充分：A 经 `set -euo pipefail` ssh-action 步自然红 CI（`deploy.yml:146`），零 CI 层改动；B 多一跳 SSH 且时序分离。 |
| D4 不做自动修复/重试 | **认可** | 与 issue #74 目标原文一致。 |
| D5 不动既有迁移文件 | **认可** | 红线一致；本次核对不改迁移。 |
| D6 不做功能冒烟自动化 | **认可** | 与 #72 开放项一致；本票只保证「代码/DB 落地」不保证「功能正确」，已在 §7 #9 明示。 |
| §6.1 方案 A 裁决 | **认可** | 理由与备选成立（见 D3）。 |
| §6.2 捕获时机（build 后、up 前） | **认可** | 关键正确：回退分支 `deploy-server.sh:38` `docker tag rollback latest` 会污染 `:latest`，捕获必须早于任何 retag；存变量免疫后续 retag。 |
| §6.3 镜像 ID 全量对比 + 去前缀 | **认可** | 服务器实查格式逐字相等（§2.1），`norm()` 双保险覆盖版本差异；精确相等零歧义。 |
| §6.4 迁移期望集 = 仓库文件集 | **认可** | 关键洞察成立：期望集**独立于运行二进制**，旧二进制内嵌旧集 + 仓库新集 → 恰能暴露回退。前提（`deploy.yml:148` pull 先行 + `set -e`）已核。 |
| §6.5 失败语义（立即 exit 1 跳过换装） | **认可** | 版本自洽 + 可用性优先；文案两级前缀区分「deploy 失败但站点健康」vs「站点挂」（后者是 L46 `FATAL`）。 |
| §6.6 psql fail-closed | **认可** | 命令替换失败 + `set -euo pipefail` 自然中止，避免重演「吞错」；原型已验 psql 失败路径的行为方向。 |
| §6.7 失败信息可读性 | **认可（附注）** | `VERIFY-FAIL`/`DEPLOY-FAIL` 前缀 + 对账行输出 stderr 合理；建议 VERIFY-FAIL 文案再带一句「站点仍健康停旧码，无需人工救火」的指导语（见 Q4）。 |
| §7 不变量 1-10 | **认可** | #8（scp 形式性局限）诚实披露，是 Q3 裁决对象；#9（冒烟缺口）明示正确。 |

## 四、开放点裁决（设计文档 §9 自带 Q1-Q5，逐条回应）

### Q1（镜像核对误报风险）—— **裁决：低风险，放行**
- 已实证（§2.1）：捕获命令与运行查询**格式逐字相等**（`sha256:` + 64 位）；`up --force-recreate` 从 `:latest` 创建容器（compose 配置、`docker compose ps` 镜像列佐证）；捕获与 up 之间无任何 retag 路径（`deploy-server.sh` L28→L31 无中间 tag 操作）。`norm()` 兜底版本差异。
- 剩余风险仅为 BuildKit 多平台/构建系统怪癖（理论级），不值得为它加复杂度。

### Q2（新镜像 dangling 卫生）—— **裁决：接受**
- 核对失败 exit 1 时 `image prune`（L61）不执行 → 新构建镜像留 dangling 至下次成功部署清理。接受且**有益**（失败态保留镜像便于事后比对根因）；不碰运行栈。无需改动。

### Q3（scp 模式「load 陈旧镜像且无新迁移」漏检）—— **裁决：补强，成本 ~3 行（已采纳 2026-08-31）**
- 该场景真实存在（§7 #8）：`docker load` 的 tar 若来自旧 workflow run（旧 HEAD），镜像核对形式性通过、迁移核对也通过 → 绿但旧码。**这与 #74 存在的理由同类**，应急路径不应是例外。
- **补强**：CI scp build 步（`deploy.yml:110-124`）在 build+load 后取 `docker image inspect ghcr.io/li-yongqvan/ai-forum-api:latest --format '{{.Id}}'` → 经 `env` 传入 ssh-action → 内联脚本把该 ID 作为 verify 第一参数（替代「load 后读 `:latest`」）。约 3 行，堵死漏检。
- **保底**：若用户坚持最小改动，接受 §7 #8 记录 + 在 `deploy.yml` scp 分支注释中显式声明「scp 模式镜像核对形式性，漏检边界见 #74 设计文档 §7#8」，风险自担。

### Q4（失败语义可读性）—— **裁决：认可，附 1 条建议**
- 两级前缀已能区分「deploy 失败但站点健康」vs「站点挂」。建议 VERIFY-FAIL 文案尾部加一句运维指导（「站点健康停在旧码，无需救火；等待下次发版即可」），让 ssh-action 日志对值班者自解释。

### Q5（`git pull` 半成功旁路）—— **裁决：认可，无旁路**
- `git pull --ff-only` 失败即非零退出 → `set -euo pipefail`（`deploy.yml:146`）中止整个脚本 → job 红。唯一旁路是 pull 成功后仓库仍非预期 HEAD（如多 runner 并发写），已被 `concurrency: deploy-prod` 排除。护栏充分。

## 五、新发现问题（文档未覆盖，评审方补充）

| # | 级别 | 问题 | 要求 |
|---|---|---|---|
| F1 | 重要 | **scp 模式镜像核对形式性 → 漏检真实存在**（「旧 workflow 重跑 → load 旧镜像 → 绿但旧码」，与 #74 要抓的信号同类）。 | 采纳 §四 Q3 补强（CI 传构建镜像 ID，~3 行）；或用户显式接受 + 注释声明。二选一，写入通过条件。 |
| F2 | 重要 | **D1/D2 决策未落档**：SOP §3.4 约定决策归档 `docs/handoffs/grilling-decisions/`；当前仅设计文档 §3 内附决策原文+日期（可复核，非阻塞证据问题），但按项目惯例应收档，供后续会话/地图引用。 | 随本票 PR 新建 `docs/handoffs/grilling-decisions/issue-74-deploy-verify-decisions.md`（含 D1-D6）。 |
| F3 | 重要 | **`check-workflows.sh` 本地不可执行**：依赖 Docker（`docker run rhysd/actionlint`），本地 daemon 未启动（2026-08-31 实测）。§8.4「本地跑」在现环境会卡住。 | 测试计划改写：优先启动 Docker Desktop 后跑，或直接依赖 CI `workflow-lint`（push 触发、阻塞，SOP §7），或临时用 `npx actionlint` 类轻量校验。 |
| F4 | 建议 | **verify-deployed.sh 空期望静默跳过镜像核对**：调用方已守卫（`[ -n ]` fail-closed），但若被直接调用调试，会无声跳过①。 | 空期望时向 stderr 打一行 `WARN: 期望镜像为空，跳过镜像核对`，让调试期可见。 |
| F5 | 建议 | **deploy-server.sh 插入新节后注释编号错位**：现注释「4. 前端换装」「5. nginx」「6. 卫生」需重排（新增核对节占「4.」）。 | 实现时同步重排编号，防后续维护按错位注释改错区段。 |
| F6 | 建议 | **真实发版回归缺少「核对行」断言**：§8.3 只要求绿，未要求日志含 `VERIFY-OK`。 | 服务器验证（SOP §9.4）加一条：deploy 日志/复核含 `VERIFY-OK: 新代码在跑`，避免「核对没跑但绿」的假象。 |

## 六、通过条件清单（执行前勾选）

- [x] **F1 / Q3 裁决**：scp 模式补强（CI 传构建镜像 ID）——已采纳（用户确认 2026-08-31），落地见设计文档 §5.3
- [x] **F2**：新建 `docs/handoffs/grilling-decisions/issue-74-deploy-verify-decisions.md`（D1-D6）——待 PR 一并合入
- [x] **F3**：测试计划 §8.4 改写（本地 Docker 起不了时走 CI workflow-lint）
- [x] **F5**：实现时重排 deploy-server.sh 节注释编号（已重排 0-8）
- [x] **§8.2 测试 2 机制钉死**：迁移缺失分支用本地 source 单测覆盖，不在服务器仓库临时建文件
- [ ] 服务器只读对抗验证（§8.2 测试 1/3/4）执行并记录输出（实现后、合并前执行）
- [ ] 本地全量回归 + `git checkout -- frontend/components.d.ts`（如被改写）

## 七、结语

这是一份可以信赖其事实基础、值得按其框架执行的设计文档——评审方独立实测未发现任何失实主张，核心逻辑经原型验证成立。所提问题集中在「应急路径的最后一公里」（F1）、「决策落档」（F2）与「测试机制的两处执行细节」（F3/§8.2-测试2），均为低成本修复。**条件满足后即可作为实现基线**：改 `deploy-server.sh` → 新建 `verify-deployed.sh` → 改 `deploy.yml` → 按 §8 测试 → PR → 用户授权 → 合并 → 服务器验证。

—— 评审方（独立对抗评审：本机仓库 @ f6b0600 + 生产服务器只读实测 + 本地原型，2026-08-31）

---

## 附：执行检查表（对抗协议 Pass 1 产出）

> 标注 `✅ 实测通过 / ❌ 实测失败 / ⚠️ 无法实测（原因）`。

| # | 检查行 | 状态 | 证据 |
|---|---|---|---|
| 1 | SSH 服务器真值：HEAD / compose ps | ✅ 实测通过 | `git log --oneline -1`→`f6b0600`；三容器 `Up 3 days (healthy)`（2026-08-31） |
| 2 | psql：`SELECT version FROM schema_migrations` | ✅ 实测通过 | 13 条，0001–0012 全（含 0007×2）；exit 0 |
| 3 | `docker inspect -f '{{.Image}}'` 格式 | ✅ 实测通过 | `sha256:c4bd545589e69f...3d`（64 位 + 前缀） |
| 4 | `docker images --no-trunc -q :latest` 格式 | ✅ 实测通过 | `sha256:c4bd...3d` —— 与 #3 **逐字相等**（exit 0） |
| 5 | `docker images -q`（短格式对照） | ✅ 实测通过 | `c4bd545589e6`（12 位，无前缀）→ 印证必须 `--no-trunc` |
| 6 | `:rollback` 全量 ID（测试素材） | ✅ 实测通过 | `sha256:08dd6c4833f97dab56ed5ede1a57ebe815461fb5c28625d211d10466a6331c21` |
| 7 | 核对逻辑原型：快乐路径 / 回退被抓 / 迁移缺失点名 / DB 超集不误判 / 空期望跳过 | ✅ 实测通过 | `/tmp/verify-test` 5 场景 6 断言全过；`bash -n` 语法通过 |
| 8 | `scripts/check-workflows.sh`（Docker actionlint） | ⚠️ 无法实测 | 本地 Docker daemon 未启动（`npipe...not found`，2026-08-31）；脚本机制已核（`check-workflows.sh:14` docker run rhysd/actionlint:1.7.12） |
| 9 | deploy-server.sh / deploy.yml / migrate.go / main.go / .gitattributes / compose.yml 引用行号 | ✅ 实测核对 | Read/grep/sed 逐处命中（§2.1） |
| 10 | 数据流/引用完整性：期望镜像 ID 捕获→verify 消费、迁移期望集→DB 集比对 | ✅ 消费端核验 | 格式相等（#3/#4）+ 捕获时机（build 后 up 前、回退 retag 在其后 `deploy-server.sh:38`）+ 仓库集独立于运行二进制（§6.4）均成立；原型实测比对逻辑 |

**数据时点**：服务器/DB 实查 2026-08-31；本地仓库 @ `f6b0600`（worktree `ai-forum-74`，已 pull 对齐远端）。
