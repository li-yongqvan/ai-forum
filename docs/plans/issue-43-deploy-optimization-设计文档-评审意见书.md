# 《#43 部署提速：服务器本地构建 — 设计文档》评审意见书

> **评审对象**：《#43 部署提速：服务器本地构建 — 设计文档（供评审）》（`docs/plans/issue-43-deploy-optimization-设计文档.md`，2026-08-24）
> **评审方式**：独立复核 —— 评审方以本地仓库 `li-yongqvan/ai-forum` @ `0301601`（main HEAD，已核实与服务器一致）+ 服务器 `122.51.233.225`（SSH -p 2222，2026-08-24 实查）为真值源，逐项核对文档引用，并对文档自带 Q1-Q8 待评审焦点逐条裁决。
> **评审结论**：**有条件通过**（0 阻塞；F1-F4 四项「重要」须在实现/预灌阶段消化，F5-F8 为建议）。

---

## 一、总体结论

设计方向正确且扎实：**服务器本地构建**直接命中跨洋 scp 物理瓶颈（§1），双模式 `DEPLOY_MODE` 应急通道（§6.2）是本方案「不牺牲稳定性」承诺的合理载体，`build 先于 up`（§6.1）把「构建失败站点不停摆」从口号落成了 compose 语义（成功才更新 tag、运行容器按 image id 引用）——这是全文论证质量最高的裁决。真值核对（§2）命令可复跑、未复核项诚实标注（§2.4 五项），证据纪律在本项目历史中属上乘。

须在动手前/实现期消化的四类问题（均非骨架性问题）：

1. **预灌清单有盲区**（F1）：`backend/Dockerfile:1` 的 `# syntax=docker/dockerfile:1` 要求 BuildKit frontend 镜像 `docker/dockerfile:1`——服务器 Docker Hub 被墙，若该 frontend 未随 daemon 捆绑、也未预灌，**首次 `docker compose build` 会在解析 syntax 时即失败**。预灌清单（golang/alpine）缺这一项。
2. **脚本与散文不一致**（F2）：§5.4/§6.4 声称「回退后再轮询 ≤30s」，但计划文档 §改动 4 的脚本在回退后**直接 `[ "$ok" -eq 1 ] || exit 1`，未再轮询**。二选一，必须对齐。
3. **scp 应急模式缺健康门**（F3）：scp 模式内联脚本 `load → up` 无健康轮询——应急路径会「deploy 绿但站点红」。至少应等待 healthy，或显式声明豁免。
4. **bootstrap 前置未列预灌**（F4）：bootstrap.sh 改显式 `docker compose build` 后，依赖 golang/alpine 预灌（+ F1 的 frontend），但执行清单只字未提——全新服务器首启必在 build 步失败。

总体评价：事实基础可信、框架可执行；以下条件满足后即可作为执行基线。

---

## 二、事实与证据复核

### 2.1 核实为真（抽样，全部命中）

| 文档主张 | 复核结果 |
|---|---|
| 服务器 HEAD = `0301601`，三容器 healthy，healthz 200 | ✅ `ssh -p 2222 ... 'git log --oneline -1 && docker compose ps && curl -s http://localhost:8888/healthz'` → 一致 |
| 服务器 4 核 / 3.7G 内存(可用 2.6G) / 23G / Docker 29.1.3 | ✅ `nproc`/`free -m`/`df -h ~`/`docker --version` 逐项命中 |
| 服务器无 golang/alpine builder 镜像、无 go/node | ✅ `docker images` 仅 3 个运行镜像；`which go` → no go binary |
| read-only deploy key 复用可行 | ✅ `~/.ssh/config` 有 `Host github.com / IdentityFile ~/.ssh/ai-forum-repo / IdentitiesOnly yes` |
| proxy.golang.org 不可达、goproxy.cn 200 | ✅ `curl --max-time 10 https://proxy.golang.org/` → `000 FAIL`；`https://goproxy.cn/` → `200 0.067s` |
| compose.yml api 无 `build:` 段 | ✅ `compose.yml:29-30` 仅 `image:` |
| bootstrap.sh 现为 `docker compose pull api` | ✅ `deploy/bootstrap.sh:47`（R1 属实，此为本设计 R1 硬阻塞修复的实证） |
| 前端服务器 dist 2.9M/320 累积、本机最新 583K/71 | ✅ `du -sh ~/ai-forum/frontend/dist` = 2.9M；本机 `frontend/dist` = 583K/71 |
| go.sum 完整（20844B）、go.mod `go 1.25.0` | ✅ `wc -c backend/go.sum` / `grep go backend/go.mod` |
| `.gitattributes` `deploy/*.sh eol=lf` | ✅ 已存在，新增 deploy-server.sh 自动 LF |
| `command_timeout` 是 appleboy/ssh-action 合法输入 | ✅ 第三方 action 输入 actionlint 不校验；官方支持 `command_timeout` |
| `# syntax=docker/dockerfile:1` 在 Dockerfile 首行 | ✅ `backend/Dockerfile:1`（构成 F1 的事实基础） |

### 2.2 不实 / 冲突

| 文档主张 | 问题 | 复核结果 |
|---|---|---|
| §1「已稳定零超时 **12 次（PR #38→#42）**」 | 归属错误 | 「连续 12 次零超时」出自 handoff §2（覆盖整个 A' 时期）；**PR #38→#42 是连续 5 次**（issue #1 决策 23 / handoff §7 声明）。12 次 ≠ #38→#42。 |

### 2.3 不可复核（与文档 §2.4 一致，诚实标注）

| 项 | 说明 |
|---|---|
| 跨洋 scp ~12KB/s / deploy ~18min | 来源 issue #43/handoff 自我报告，非独立实测；本票验证阶段以试点发版实测为准 |
| 本地→服务器 ~780KB/s | 来源 `docs/workflow.md` §9.2（预灌估时依据） |
| 本机 Docker Hub 实际 `docker pull` | 仅 `docker manifest inspect` 通过；实施期预灌第一步实测 |
| 服务器构建耗时/内存峰值 | merge 前 gated 试构建核实 |
| GOSUMDB=sum.golang.google.cn 可达 | goproxy.cn 已证；GOSUMDB 为边缘保险（go.sum 完整时默认不触发） |

---

## 三、决策清单评审（D1-D6）

| 决策 | 结论 | 评审意见 |
|---|---|---|
| D1 CI 角色：纯服务器构建 | **认可** | test job `go build ./...` 作编译门足够（后端零功能改动，编译门 = 编译正确性门）；删 GHCR push 的后果 R14 已如实披露。附 Q3 的版本漂移注记。 |
| D2 前端 gzip 后 scp | **认可** | 583K/71 文件（§2.1）坐实 tar ~250-400KB 非瓶颈；「清旧」是必需步而非可选优化。 |
| D3 GOPROXY 改 Dockerfile ARG | **认可** | proxy.golang.org 不可达实证支撑；默认不覆盖 CI（§6.6）语义干净。GOSUMDB 标未复核（§2.3），可接受（go.sum 完整）。 |
| D4 预灌 gated | **认可（附条件）** | 方向正确（merge 前必须，否则首次 build 拉 Docker Hub 失败）。**条件：预灌清单补 `docker/dockerfile:1` 或试构建确认捆绑 frontend**（F1）。 |
| D5 DEPLOY_MODE 双模式 | **认可** | 回退 = 改 variable + Re-run，不碰代码/merge，是应急场景的正确形态；A' 曾连续 12 次零超时是可信的成熟后备。附 F3（scp 模式补健康门）。 |
| D6 单 deploy job | **认可** | 前端 tar 非瓶颈（§2.1）使并行收益 ~1min、协调代价（版本错配 + 并发 group 语义）更高；取舍正确。附 F5 的时限预期校准。 |

---

## 四、文档自带待评审焦点裁决（Q1-Q8）

| # | 焦点 | 裁决 |
|---|---|---|
| Q1 | bootstrap 改显式 build 是否闭环 | **认可**：`deploy/bootstrap.sh:47` 已核实为 `pull api`，改 `build --build-arg` 是唯一闭环。**但** bootstrap 首启依赖 golang/alpine 预灌，执行清单必须补（F4）。 |
| Q2 | DEPLOY_MODE=scp 回退链路完整性 | **认可方向，附 1 条件**：前端 tar 统一走 /tmp、两模式共用解压逻辑，链路无断点。**条件**：scp 模式内联脚本补健康等待（F3），否则「应急」成了「无门部署」。 |
| Q3 | 镜像一致性漂移 | **认可（附注）**：服务器 golang:1.26-alpine 预灌一次即冻结（Docker Hub 被墙不会重拉），实际比 CI 每次拉 latest 更稳定；真正漂移是 CI 编译门 go 1.25 vs 产物 go 1.26，MVP 接受。建议预灌时记录 digest 入档（§5.6 已有，落实即可）。 |
| Q4 | GHCR 过期边界 | **认可**：纯服务器构建后 GHCR `:latest` 过期（R14）；服务器 rollback tag + git 源码可重建 = 灾难兜底已够。学习项目接受。 |
| Q5 | `exec -T` + `command_timeout` | **认可**：现 `deploy.yml:154` 无 `-T` 能跑是因 ssh-action 分配 PTY；统一 `-T` 在脚本/非交互上下文是正确且不破坏的改进。`command_timeout: 20m` 偏保守（见 F6）。 |
| Q6 | 健康轮询 60s 窗口 | **认可（附条件）**：api healthcheck `start_period 10s + interval 30s`，健康首次出现在 ~10-40s；60s 窗口对本应用（Go 二进制启动 ~1s、无慢迁移）足够。**条件**：试构建时实测一次「build→up→healthy」全链时点，若冷启动超窗口再调。 |
| Q7 | BuildKit GC 边界 | **认可**：`image prune -f` 不清 cache 是特性（cache 正是提速来源）；盘压 >5G 才 `builder prune --keep-storage 4G`，指令精确可执行。 |
| Q8 | PWA 旧哈希 404 瞬态 | **认可**：sw.js autoUpdate 自愈；「清旧」后旧 chunk 短暂 404 属已知取舍（#14 同源）。MVP 接受。 |

---

## 五、新发现问题（文档未覆盖，评审方补充）

| # | 级别 | 问题 | 要求 |
|---|---|---|---|
| F1 | **重要** | **BuildKit frontend 预灌盲区**：`backend/Dockerfile:1` `# syntax=docker/dockerfile:1` → 服务器 `docker compose build` 需解析 `docker/dockerfile:1` frontend。Docker Hub 被墙下，若该 frontend 未随 buildkit 捆绑、也未在本地缓存，**build 在解析 syntax 时即失败**（早于 golang 镜像解析）。 | 预灌清单（§5.6 / 执行清单）补 `docker/dockerfile:1`（`docker pull docker/dockerfile:1` + save/load），或在 gated 试构建中**显式验证 frontend 免拉**；两者取一，试构建必须记录该点。 |
| F2 | **重要** | **回退后再轮询的散文/脚本不一致**：§5.4 第 4 步与 §6.4 均称「回退后再轮询 ≤30s」，但计划文档 §改动 4 脚本在 rollback block 后直接 `[ "$ok" -eq 1 ] || { exit 1; }`——`ok` 仍为 0，**无再轮询**，直接判失败。 | 二选一：① 脚本补再轮询（rollback + up 后 `ok=0` 再 `for i in $(seq 1 15)`，healthy→ok=1，仍失败才 exit 1）——推荐，回退成功与否值得一次真实信号；② 若维持「回退后即 exit 1」，改 §5.4/§6.4 散文为「回退后部署判定失败（旧代码已恢复运行，CI 红反映新代码未上）」，消除不一致。 |
| F3 | **重要** | **scp 应急模式无健康门**：scp 模式内联脚本 `tar 解压 → docker load → up -d --force-recreate` 后**不轮询健康**——若 CI 构建的镜像坏，deploy 显示绿但 api unhealthy。 | scp 模式 `up` 后补健康等待（复用 deploy-server.sh 的轮询逻辑；超时即 exit 1，无需回退——它是最后手段）。若刻意豁免（应急优先速度），在文档显式声明「scp 模式跳过健康门，风险自担」。 |
| F4 | **重要** | **bootstrap 前置条件未列预灌**：bootstrap.sh 第 4 步改 `docker compose build` 后，依赖 golang:1.26-alpine + alpine:3.21（+ F1 的 frontend）预灌；v3.1 的 nginx/postgres 预灌在 §7 执行清单有显式步骤，**golang/alpine 未列**。 | docs/ops §7 执行清单补「预灌 golang:1.26-alpine + alpine:3.21 + docker/dockerfile:1（见 F1）」硬前置步 + bootstrap.sh 注释同步（防全新服务器首启在 build 步失败）。 |
| F5 | 建议 | **时限预期校准**：ticket「2-4min」vs 单 job + gzip 前端的现实 ~3-5min（前端 npm ci+build 1-2min + scp ~1min + 服务器 build 1-3min）。「2-4min」仅在最优情形成立。 | 验收口径改为「**deploy job ≤5min + 连续 3 次零超时**」，并把「18min→~4min」作为主成就宣示；若实测稳定 <4min 再收口号。 |
| F6 | 建议 | `command_timeout: 20m` 是 20min 挂起天花板，偏保守（build 仅 2-4min）；一旦 goproxy 卡死，CI 要干等 20min。 | 降为 `command_timeout: 10m`（冷机首建也有余量）；若试构建实测 >10min 再调。 |
| F7 | 建议 | **前端换装非原子**：`find -delete` + `cp -a` 中途失败会留半换装 dist（部分 chunk 404 至下个发版）。 | 接受（与现状 scp 非原子同源，#14 取舍）；在 deploy-server.sh 注释 + docs 记录即可，不引入 rsync/软链方案（改 bind mount 语义，过度设计）。 |
| F8 | 建议 | **文档引用小错**：§1「已稳定零超时 12 次（PR #38→#42）」归属错误。 | 改为「handoff 记录连续 12 次零超时」（#38→#42 是 5 次）；其余引用精确，不影响设计。 |

---

## 六、通过条件清单（执行前勾选）

- [ ] **F1**：预灌清单补 `docker/dockerfile:1`，或 gated 试构建显式验证 frontend 免拉（记录结果）
- [ ] **F2**：脚本与散文对齐——补回退后再轮询，或改散文表述
- [ ] **F3**：scp 应急模式补健康等待（或显式声明豁免 + 风险自担）
- [ ] **F4**：bootstrap 执行清单补 golang/alpine/frontend 预灌硬前置 + 注释同步
- [ ] **F5**：验收口径改为「deploy job ≤5min + 连续 3 次零超时」
- [ ] **F6**：`command_timeout` 降为 10m（试构建实测超限再调）
- [ ] **F8**：修正 §1「12 次（PR #38→#42）」归属
- [ ] 预灌 + gated 试构建：记录服务器构建耗时/内存（§2.4 未复核项核实）
- [ ] 合并后：docs/ops v3.2 + workflow.md §9 + lessons 同步（§5.6）；地图 #1 更新交给统筹方

## 七、结语

设计的骨架（build 先于 up 的安全模型、DEPLOY_MODE 应急通道、保 inode 前端换装、cache mount 提速）经得起复核，事实基础可信、未复核项诚实。所提问题集中在「预灌清单的完整」（F1/F4）与「脚本-散文一致性与应急路径的门控」（F2/F3），均为实现期低成本修复。按 §六清单消化后，可作为执行基线进入实现。

—— 评审方（独立复核：仓库 @ 0301601 + 服务器实查，2026-08-24）
