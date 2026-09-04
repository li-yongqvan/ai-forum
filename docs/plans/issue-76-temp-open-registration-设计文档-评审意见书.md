# Issue #76「9/13 一次性免码注册」设计文档 — 评审意见书

> **评审对象**：《issue-76-temp-open-registration-设计文档.md》（2026-09-05，待评审）
> **评审方式**：独立复核 —— 评审方以产品仓库 `C:\Users\liyongquan\ai-forum` 本地 HEAD `f6b0600` 与 GitHub `origin/main`（`5f62910`，本地落后 3 个提交）为真值源，逐项核对文档 file:line 引用；`gh` 实查 issue #76。**未实连服务器/线上 DB，未运行 go/npm 构建**（见 §二 执行检查表）。
> **评审结论**：**有条件通过**。

---

## 一、总体结论

方向与 D1–D9 的骨架（一次性单 commit + git revert、占位邮箱零迁移、拆 zone 放宽 register、撞名 409+建议名、真并发靠 DB 唯一约束）在既有架构内成立，且证据纪律（file:line + 原版/现版纠正 + 偏离登记 + 自留待评审焦点 Q1–Q7）是该项目历史中少见地扎实的一份。我逐行核对了 auth.go / service.go / gorm_repo.go / model.go / 0001_user_schema.sql / Register.vue / api/auth.ts / client.ts / web.conf / compose.yml / main.go，**几乎全部命中、行号精确**（详见 §二 核实为真表）。这正是本协议最强调的：计划文档把"事实"标了来源、把"裁决"单独标了【决策】、还把"原稿说法有误"的自我纠正写了进来——骨架可信。

但有 **1 个必须在派发前处置的 canonical 阻塞项** + **3 个动手前必须消化的问题**：

1. **B1（canonical 分叉）**：设计模型与 GitHub issue #76 body（仍是旧框架：默认关闸门 `.env REGISTER_OPEN` / 换服务器待定）互为反面，而支撑 D1–D9 的 ADR 0001/0002 + CONTEXT.md + grilling 共识**只存在于非 git 的 agent-panel 目录**，产品仓库 `docs/adr/` 尚不存在。文档把"同步 #76 body"列为待评审问题 Q1——这对一份要交付给"没有本对话的 agent"执行的 spec 来说，**必须是派发前的前置动作，不是开放问题**。
2. **F2**：open 版会改掉注册语义，导致一批**既有测试红**，而 `go test ./...` 是 deploy 的阻塞门——文档 §8 列了新增测试，却没列要改写的既有测试。
3. **F3（仓库基座不一致）**：文档以落后 `origin/main` 3 个提交（#74/#75 deploy）的本地 `f6b0600` 为"代码真值"，但 §8 依赖的 `verify-deployed.sh` / `deploy-server.sh` nginx-reload 流程恰恰只存在于 `origin/main`。
4. **F4（事实主张有误）**：Q6 的前提「占位邮箱无页面展示」是**错的**——`Settings.vue:49` 会把邮箱显示在本人设置页账号栏，ADR-0002「占位永不外发、无泄露面」被证伪。

总体评价：**事实基础可被信赖，决策框架正确**；以下条件满足后即可作为执行基线，其中 B1 是硬性的派发门。

---

## 二、事实与证据复核

> 复核通道：产品仓库本地 `f6b0600` 实读 + `git diff f6b0600 origin/main` 比对 + `gh issue view 76`。

### 2.1 核实为真（含 file:line）

| 文档主张 | 复核结果 |
|---|---|
| `auth.go:27-32` registerReq 四字段全 `required`，email 另带 `,email` | ✅ `auth.go:27-32` 逐字命中；四字段均 required，email 带 `binding:"required,email"` |
| `auth.go:49-50` 撞名/邮箱占用统一 409「用户名或邮箱已存在」；`:51-54` 400；`:55-56` 500；`:60` 201 AuthResult | ✅ 逐字命中（username/email 两错误合并同一 case） |
| `auth.go:76-84` ErrBadCredential/ErrBanned 统一 401（§7 #7 依据） | ✅ 逐字命中 |
| `service.go:154` `minPasswordLen=8` | ✅ |
| `service.go:173-174` 弱密码→`ErrWeakPassword` | ✅ |
| `service.go:176-178` 空 username/email→error | ✅（`:176-177` 判空+return，`:178` 收尾；判断标 open 版须去 email 条件 —— 与代码一致） |
| `service.go:186-220` Register 单事务；`:188-191` 邀请码校验；`:193-202` 唯一预查；`:204-213` CreateUser；`:215-217` MarkInvitationCodeUsed；`:224` issueAuth 自动登录 | ✅ 全部命中；注意预查在前、CreateUser 在后，真并发窗口真实存在（支撑 D8） |
| `gorm_repo.go:23-25` `CreateUser = Create(u).Error`，无 OnConflict、无 23505 翻译 | ✅ 逐字命中（`gorm_repo.go:23-25`，实际 Create 在 `:24`） |
| `model.go:10-14` Email/Username `uniqueIndex`；`:19-21` 时间戳+软删 | ✅ |
| `0001_user_schema.sql:7-8` email VARCHAR(128) UNIQUE NOT NULL / username VARCHAR(64) UNIQUE NOT NULL；`:14` created_at；`:16` deleted_at；`:36-40` used_by UNIQUE + CHECK | ✅ 逐字命中 |
| `service.go:415-419` toView 把 Email 放进视图；`:412` AuthResult.User；me 经 `:390/:403` 带 Email | ✅ Email 在 `:419`；toView 区间实际 `415-423`，Email 行号精确 |
| email 死字段：无 SMTP/找回/通知路径 | ✅ 复核命令 `grep -rinE "smtp|SendMail|ResetPassword|mail\.|发送邮件" backend/` 零命中 |
| `Register.vue:39-42` 四字段；`:13-16` 空→toast「请填写全部字段」；`:10/17/25/45` loading 现状 | ✅ 逐字命中；Vant `loading` 下按钮 disabled（防连点）成立 |
| `api/auth.ts:8-14` register 载荷四字段全必填 | ✅ 逐字命中 |
| 前端无 409 差异化分支，`client.ts` 只透传 message | ✅ `Register.vue:22-24` 仅 `showToast(e.message)`；`client.ts:64-66` 抛 `ApiError(status,msg)` 不含 code |
| `web.conf:2` auth_limit `rate=10r/m`；`:23-30` login、`:31-38` register 同 zone `burst=20 nodelay` | ✅ 逐字命中，**login/register 共桶属实**（支撑 D5 拆 zone 理由） |
| `compose.yml:39-40` api `env_file: .env`；`:17` web.conf bind-mount；`:45` api 仅 127.0.0.1:8080；`:52` healthz wget | ✅ 全部命中 |
| `main.go:25` config.Load 启动只读一次；`:41-44` 迁移失败 `os.Exit(1)` | ✅ 逐字命中（迁移随启动执行、失败即死属实；支撑"砍邮箱需新迁移→冲突"的 ADR-0002 理由） |
| 路由 hash 式、`/register`→Register.vue、public 无鉴权 | ✅ `router/index.ts` createWebHashHistory + `/register` 无 requireAuth |
| issue #76 OPEN、label `wayfinder:grilling`、body 为旧框架草稿 | ✅ `gh issue view 76` 实查（body 仍是「默认关闸门 REGISTER_OPEN / 9/10 冻结 / 换服务器待定」）——**偏离登记属实** |
| 机器可读 `code` 已有先例（403 `account_banned`） | ✅ `middleware/active.go:32` `c.AbortWithStatusJSON(403, gin.H{"error":…, "code":"account_banned"})` —— §6.2 可同构 |
| CI 阻塞门：`go test ./...` + `frontend-test` 前置 deploy | ✅ `deploy.yml` `test` job + `deploy needs:[test,frontend-test]` |
| 唯一 Register 调用方 = auth.go handler（改动无扩散） | ✅ grep 全仓 `.Register(` 仅 `auth.go:41` 一处 |

### 2.2 不实 / 冲突 / 需修正

| 文档主张 | 复核结果 |
|---|---|
| **§9 Q6 / §2.3**：「占位 email…无页面展示（低危）」 | ❌ **有页面展示**：`Settings.vue:49` `<van-cell :title="auth.user?.username" :label="auth.user?.email" />` —— open 注册用户在本人「设置」页账号栏会看到 `u_<hex>@local.invalid`。**Q6 的决策前提为假**（见 F4） |
| **ADR-0002**：「占位永不外发、无泄露面」 | ⚠️ 有页面/API 展示（本人侧）。不发往**其他用户**属实，但"永不外发"字面不成立 → 需改 ADR 措辞（见 F4） |
| **§0**：`config/config.go` Load | ⚠️ 实际包路径 `backend/internal/config/config.go`（文档全程写 `config/config.go`）。§0 可作包名简称读，但作为 spec 路径不精确（F8） |
| **§2.1/§8**：本仓库含 `deploy/verify-deployed.sh`、部署走含「镜像+迁移核对」的 deploy-server | ❌ 本地 `f6b0600` **无** `verify-deployed.sh`（#74 新增，仅 `origin/main`）；本地 `deploy-server.sh` 是 #74 前版本。文档把 origin/main 的 runbook 当作既有事实引用，却以本地落后仓库为"代码真值"——**自引用基座不一致**（见 F3） |

### 2.3 不可复核 / 未实测（如实标注）

| 项 | 说明 |
|---|---|
| 服务器 / 线上 DB | 本评审未实连（与文档 §2.1 声明一致；文档标注"运行/演练时按 SOP 实查回填"） |
| go/npm 构建与测试 | 本评审**未运行** `go build/test`、`npm test/build`（本地无 `frontend/node_modules`；文档 §8 将其列为执行期可复现命令，非"已实测"主张）。执行检查表相应行标 ⚠️ |
| nginx 限流行为（拆 zone 后 60r/m + burst120） | 无法本机实测，靠演练剧本验证；已核 zone 语法与 `location =` 精确匹配路径 |

---

## 三、逐条评审（D1–D9 + 实现方案各点）

### 3.1 Grilling 决策（D1–D9）

| 决策 | 结论 | 评审意见 |
|---|---|---|
| D1 一次性单 commit + git revert，不做常驻开关 | **认可（附条件）** | 方向正确且与 #74 部署 runbook（push→main 自动发版 + verify）自洽。**条件**：open commit 必须基于 `origin/main`（先 `git pull`）而非当前落后的本地 main；且派发前同步 issue #76 body（B1）——否则 canonical 源与设计互为反面 |
| D2 占位邮箱零迁移 | **认可（附条件）** | 零迁移规避「迁移失败→启动即死→自动回滚」的推理在代码层成立（`main.go:41-44` 属实）。**条件**：① 处理占位邮箱在 Settings 的展示（F4）；② 生成撞库重试一次的执行位置须写明（重试应包住整个 Tx，不能只在 Tx 外生成一次——见 F5）；③ 生成器长度校验：`u_`(2)+32hex+`@local.invalid`(13)=47 < VARCHAR(128) ✅ |
| D3 密码 ≥8 保留 | **认可** | 前后端零改动、与 D5 的 login 防枚举目标一致 |
| D4 撞名 409+建议名 | **认可（附条件）** | 需要 §6.2 的机读契约落地。**条件**：suggestion 的服务端来源/传输机制目前未定（F5）；建议名探测语义必须与 Register 重复判定一致（**封禁号也占用名字**），否则对 banned 用户名探测会误判可用→循环 409 |
| D5 register 拆 zone | **认可（附条件）** | 共桶事实核实属实、拆法正确（login 保留 auth_limit）。**条件**：burst=120 恰好卡在演练剧本「单 IP 同秒 120」上限——场地 WiFi 单出口 IP 若同秒 >120 会被 503；演练应加 1 档超限样本并据实调 burst/rate（F6） |
| D6 全开放免码、不做验证码/暗号 | **认可（风险已由用户承接）** | 技术裁决（同 URL 无法区分扫码/转发）成立。风险提示见 Q7 裁决：open 窗口 = **从 open commit 上线到 9/14 回退**，不止 9/13 演讲时段；期间任何打到 `/register` 的机器人（无需二维码）都可注册、账号永久、无批量清理。建议压缩窗口 + runbook 备好逐号封禁路径，但不推翻用户已确认的 D6 |
| D7 隔离 compose 演练 | **认可** | 独立端口 + 独立 pg 卷 + 压完删栈，零污染线上成立 |
| D8 并发/双击分层 | **认可（附条件）** | 分层（loading + DB UNIQUE + 23505 映射）正确，`gorm_repo.go:23-25` 无翻译属实。**条件**：23505→ErrUsernameTaken 的翻译位置与约束识别须钉死（Q2/F5）——建议下沉到 repo 层，避免 pgconn 类型泄漏进 user 域包 |
| D9 回退时机/runbook | **认可** | 9/14 早上回退 + 紧急回退唯一例外，与「当天零部署」自洽 |

### 3.2 关键裁决（§6）

| 裁决 | 结论 | 评审意见 |
|---|---|---|
| §6.1 不做扫码门禁 | **认可** | 理由成立；承接 D6 风险 |
| §6.2 409 带 code+suggestion | **认可（附条件）** | 现有 `respondError`（`response.go:6` 输出 `{error}`）不含 code；`middleware/active.go:32` 已提供 `{error, code}` 同构先例。**缺传输机制**：handler 拿不到 suggestion——必须由 service 产出（F5） |
| §6.3 真并发兜底分层 | **认可** | DB 唯一约束 + 翻译是正确的唯一硬保险 |

### 3.3 实现方案 §5.1/#5.2 逐条

- §5.1#1（去掉 email/invite required）：**认可，需精度**——不能只摘 `required` 留 `,email`：validator.v10 对空串跑 email 校验会 400。应把 email/invite 字段的 binding 标签**整条去掉**（或从 struct 删除；gin json decoder 忽略多余 body 键）。保留 username/password `required`。
- §5.1#2（跳过邀请码、email 忽略入参生成占位）：**认可，见 F5 的守卫建议**——服务层不能连"trim 后空 username"的守卫一起去掉：`binding:"required"` 对 `"   "` 放行，trim 后为空若不守卫会插空串 → 撞 `users_username_key` → 被翻译成"用户名已占用"+ 垃圾建议名。空用户名应仍走 400。
- §5.1#3/#4（建议名助手 + 23505 映射）：**认可方向，机制未钉死**——见 F5/Q2。
- §5.2#1-#4（前端字段收敛 + 409 分支 + 保留 loading + client 透传 code/suggestion）：**认可**。`Register.vue` 无单测、`stores/auth.test.ts` 不覆盖 register → vitest 不红；但 `vue-tsc`（`npm run build` 前置，deploy 阻塞）会因 register 载荷类型收窄波及 `stores/auth.ts` + `api/auth.ts` + `types.ts`，须同步放宽类型，文档未列（并入 F5 提醒）。
- §5.3（web.conf 拆 zone）：**认可**，见 D5 条件。
- §5.4（收尾审计）：**认可**——见 F7：TIMESTAMPTZ 查询必须带时区偏移，否则窗口漂移。

---

## 四、开放点裁决（Q1–Q7）

### Q1（issue #76 body 同步）—— **裁决：派发前的硬性前置，不是开放问题（B1）**

设计模型与 GitHub canonical 源互为反面，而 `gh` 实查确认 body 仍是旧框架、且注明"本地产品仓库落后 GitHub main…动工前先 git pull"。对一份交给**没有本对话**的 agent 执行的 spec，按 Pass 4 Q5/Q6 这是 **canonical 分叉**——必须处置：

1. 派发前更新 issue #76 body（或关闭旧 body 改挂一份指向本 doc 的摘要），至少改：**「默认关闸门 .env REGISTER_OPEN」→「一次性临时改动、单 commit、回退=git revert、不做常驻开关」**；**「换服务器待定」→「已定必做、迁移先行」**；保留 9/10 冻结、当天零部署。
2. **决策落档**：ADR 0001/0002 + 一份 issue-76 grilling 决策记录 + CONTEXT.md 目前只在**非 git 的 agent-panel 目录**；产品仓库 `docs/adr/` 尚不存在。应在产品仓库建 `docs/adr/` 把 0001/0002 按 Q5 对齐的真实序号入库（可随本票 PR 合入），否则第三方无法复核"用户确认 2026-09-05"。

### Q2（23505 约束识别）—— **裁决：方向对，落到 repo 层 + 钉死约束名**

- 无 GORM AutoMigrate（`migrations` 全原生 SQL，已核实），故唯一约束名 = 0001 迁移的内联 UNIQUE 自动命名：**`users_username_key` / `users_email_key`**（`invitation_codes` 的 `used_by` 为 `invitation_codes_used_by_key`）。比对 `*pgconn.PgError.ConstraintName` 是充分做法。
- **推荐下沉到 `gorm_repo.CreateUser`**（或 gorm_repo 内辅助）：user 域包当前**不 import pgconn/pgx**（已 grep 确认零命中）；在 `service.Register` 里做 `errors.As(&pgconn.PgError)` 会让域包依赖驱动类型，违反本项目跨包 seam 纪律（§0）。在 repo 层翻译成哨兵 `ErrUsernameTaken/ErrEmailTaken`，服务/测试都不沾驱动。
- 兜底规则：`ConstraintName` 命中 username→ErrUsernameTaken、命中 email→ErrEmailTaken；**其它/未知 23505 原样返回（宁可 500 不误吞为 409）**。约束名写死为常量 + 单测覆盖（含 ConstraintName 为空的样本）。
- 映射单测需真实 PG（`internal/testutil.SetupPG` 已有先例）触发真 23505，或 fake repo 注 `*pgconn.PgError`。

### Q3（409 契约扩散 / PWA 版本化）—— **裁决：值得做，不需版本化，靠演练验证**

- 扩散值得：`code`/`suggestion` 复用 `middleware/active.go` 先例，`client.ts` 本就 parse `data.code`，成本低；纯文案方案无法支撑 D4 的一键采用。
- PWA 老 bundle：`vite.config.ts` 已 `registerType:'autoUpdate'` 且**未配 navigateFallback/runtimeCaching** → 对 `/register` 的导航请求走网络取新 bundle，老缓存页面主要只在离线/极端场景残留；风险比 §7 #11 的担忧小。**保留真机演练**即可，不必版本化；若演练仍复现旧 4 字段页，加力手段是给 `index.html` 响应加 `no-store` 或 bump cache 名——先不做。

### Q4（并发点同一建议名）—— **裁决：可接受，补一条铁律**

低频现场下输家 409+新建议名是收敛链路，可接受。**铁律**：服务端每次 409 都要**重新**按当前 DB 算建议名，前端必须**用响应里最新 suggestion 重渲染**（不得复用上一次的），否则输家拿着已过期的 `name_2` 死循环。

### Q5（迁移/QR/ADR 编号）—— **裁决：不阻塞本 doc 设计**

确认不阻塞。但"ADR 编号对齐产品仓库 `docs/adr/`"目前指向**不存在的目录**——产品仓库无 `docs/adr/`；把建目录 + 入库并进 B1 的执行项。

### Q6（占位 email 随 API 返回）—— **裁决：前提有误，需改一处 UI + 改 ADR 措辞（F4）**

原前提"无页面展示"不成立。**推荐最小处置**：backend 不动（email 只回本人侧 API），前端 Settings 账号栏当 `email.endsWith('@local.invalid')` 时**不显示该 label**（或显示固定文案"占位邮箱"）；并修正 ADR-0002 措辞为"占位邮箱不对外部用户展示；仅本人 API/设置可见，UI 隐藏"。其它用户永不可见 → 无跨用户泄露，非阻塞。若选"接受展示"，请在本 doc 决策记录写明已知缺陷。

### Q7（数百人公开免码风险口径）—— **裁决：站得住，但把风险面写实**

结论站得住（用户已确认 D6）。但要把口径写实，避免执行/预案误以为防线是"二维码"：
- open 期间**任何访问者**（非仅扫码者）都能注册；`/register` 是公开已知路径、可被扫描器命中。
- 无验证码 + 占位邮箱 + 账号永久 + **无批量删/清工具**（文档 §4 自认"预案可选项"）→ 若被脚本灌号，唯一处置是逐号 admin 封禁。
- 缓解建议：把 open commit 的部署尽量贴近 9/13（压缩活动前暴露）；runbook 里写明灌号时的处置流程（created_at 窗口导清单 + 逐号封禁），而不是设计里当"可选项"一句带过。

---

## 五、新发现问题（文档未覆盖，评审方补充）

| # | 级别 | 问题 | 要求 |
|---|---|---|---|
| B1 | **阻塞** | **canonical 分叉 + 决策未入库**：GitHub issue #76 body 仍是与设计相反的旧模型（默认关闸门/换服务器待定）；ADR 0001/0002/CONTEXT/grilling 共识仅存于非 git 的 agent-panel，产品仓库无 `docs/adr/`。**一个没有本对话、只读仓库+issue 的执行 agent 会照旧模型实现**（§9 Q1 以"待评审问题"处理，力度不足）。 | ① 派发前同步 #76 body 到新模型（改点见 Q1 裁决）；② 产品仓库建 `docs/adr/`，将 0001/0002 + issue-76 grilling 决策记录（含用户确认日期与原文）入库，可随 open commit 或独立文档 PR 合入；③ 派发指令显式声明"本 doc 为权威基线，覆盖 issue body 矛盾处"。 |
| F2 | **重要（deploy 门）** | **既有测试改写清单缺失**：open 版改注册语义 → 以下测试红，而 `go test ./...` 是 deploy 阻塞门：`handler/auth_test.go` `TestRegisterLoginMeLogoutFlow`（复用码期望 400）、`TestRegisterValidation`（非法码 400 / 邮箱格式 400）；`user/service_test.go` `TestRegister_Success`（期望邀请码标记已用）、`TestRegister_InvalidInvite`、`TestRegister_Duplicate` 邮箱重复子测。§8 只列新增测试。 | 文档补一节「既有测试改写清单」（上列 + 文件），明确这些改动随 open commit 一起、回退时随 revert 还原；改写后的断言应验证**新语义**（免码成功、邀请码不被消耗、占位邮箱格式），而不是简单删测试。 |
| F3 | **重要** | **仓库基座自引用不一致**：本地 main `f6b0600` 落后 `origin/main` 3 个提交（#74/#75 deploy：新增 `verify-deployed.sh`、改 `deploy-server.sh` 加 nginx reload + 镜像/迁移核对）。文档 §8 引用 `verify-deployed.sh`/deploy-server reload 流程= **origin/main 内容**，但 §2.3/2.4 声明代码真值=本地 `f6b0600`（本地无该文件）。 | 文档补声明：open commit 在 `origin/main`（`git pull` 后，按 issue body 注）上开发与测试并记录 HEAD；本次真值核对仅 backend/frontend 有效（该 3 提交未触碰 `backend/`、`frontend/`、`compose.yml`、`web.conf`——已 diff 核实），deploy 相关事实以 origin/main 为准复核。 |
| F4 | **重要** | **Q6/ADR-0002 事实前提有误**：占位邮箱会显示在本人 Settings（`Settings.vue:49` 账号栏 label），"无页面展示 / 永不外发"不成立。 | 按 Q6 裁决落地：前端对 `@local.invalid` 隐藏 label（或固定文案）+ 修正 ADR-0002 措辞 + 本 doc 决策记录留痕。 |
| F5 | **重要** | **suggestion 的"从哪来、怎么传"未定义**：§6.2 说 handler 写 suggestion，但现有 `respondError{error}` 无通道；service 若返回哨兵错误则 suggestion 无处附着；建议名探测语义未与 Register 重复判定对齐（**banned 用户也占用名**）；未提 VARCHAR(64) 长度上限（原名 64 字符→`name_2` 超长插库报 22001→500）；空用户名守卫不能一起去掉。 | 给一份可执行契约：① 保留哨兵 `ErrUsernameTaken`，suggestion 不进错误对象——handler 捕获后调用新增 seam 方法 `svc.SuggestNextUsername(ctx, username)`（user 包内实现，探测用与 Register 相同的 repo 判重口径 = 任何非软删用户含 banned）；② 探测返回 `name`/`name_2`…并**截断到 ≤64**；③ 23505→哨兵翻译下沉 `gorm_repo.CreateUser`（见 Q2）；④ 服务层保留"trim 后空 username→400"守卫；⑤ `registerReq` 去掉 email/invite 的 binding 标签（整条，非仅 required）；⑥ 前端载荷类型收窄波及 `stores/auth.ts`/`api/auth.ts`/`types.ts`（vue-tsc 阻塞门）一并列进 §8。 |
| F6 | 建议 | register `burst=120` 卡在演练上限：场地单出口 IP 同秒 >120 → 503；且 60r/m≈1r/s 对「300 人 5 分钟内挤完」几乎零余量。 | 演练加 1 档超限（如 150 同秒）与"持续 60+ r/m"样本，据实调 rate/burst；结论回填决策记录（改参数=改 web.conf 行，仍在单 commit 内）。 |
| F7 | 建议 | 收尾审计用 `created_at BETWEEN`，但 created_at 为 TIMESTAMPTZ（UTC 存储）；runbook 若写本地时间无偏移会漂移、漏选/错选账号。 | runbook 查询**显式带 UTC 偏移**（如 `BETWEEN '2026-09-13 00:00:00+00' AND '2026-09-14 00:00:00+00'`），并记录 open 版上线/回退的精确时刻（部署日志时间戳对齐同一时区）。 |
| F8 | 建议 | 细节一致：`config/config.go` 实际在 `backend/internal/config/`；hash 路由声明 `router/index.ts:5` 实际 createWebHashHistory 在 `:1/:5`。 | 改精确路径/行号；不改变结论。 |

---

## 六、通过条件清单（执行前勾选）

- [ ] **B1**：issue #76 body 同步为新模型（REGISTER_OPEN→一次性单 commit/回退=revert；换服务器待定→必做迁移先行）；产品仓库建 `docs/adr/` 将 0001/0002 + issue-76 决策记录入库；派发指令声明本 doc 为权威基线
- [ ] **F3**：确认 open commit 基座 = `origin/main`（先 pull），记录 HEAD，deploy 事实以 origin/main 复核
- [ ] **F2**：§8 补既有测试改写清单（handler 2 处 + user service 3 处），随 open commit 改写、随 revert 还原
- [ ] **F4**：Settings 隐藏 `@local.invalid` label；ADR-0002 措辞修正
- [ ] **F5**：suggestion 契约落地（SuggestNextUsername 方法 + 探测口径含 banned + ≤64 截断 + 23505 下沉 gorm_repo + 空用户名 400 守卫 + registerReq binding 整条去标签 + 前端载荷类型收窄同步）
- [ ] **Q2 附带**：约束名常量 `users_username_key`/`users_email_key` + 单测覆盖（含 ConstraintName 为空样本）
- [ ] **Q4 附带**：409 每次重算 suggestion、前端重渲染最新值
- [ ] 演练补：超限样本（F6）+ 真机老 bundle 验证（Q3）+ 双设备同秒抢名 + login 仍严
- [ ] **F7 附带**：runbook 查询带 UTC 偏移并落档上线/回退时刻

---

## 七、结语

这是一份证据纪律极好、方向正确的设计：事实引用几乎逐字命中、自我纠错（loading 已防连点）、偏离登记（issue body 旧框架）、待评审焦点 Q1–Q7 全部点在要害。所提问题集中在三处"最后一公里"——**canonical 归档与基座对齐（B1/F3）、deploy 门前既有测试改写（F2）、409 suggestion 机制的传输契约（F5）**——均为低成本、一次性修复，不伤骨架。修复后，以 §六 清单作为派发/实现自检表执行即可。

—— 评审方（独立复核：产品仓库本地 `f6b0600` + `origin/main` `5f62910` diff 比对 + `gh` issue #76 实查；未实连服务器/线上 DB、未运行构建与测试。2026-09-05）

---

## 附：执行检查表（对抗协议 Pass 1 产出）

| 检查行 | 类别 | 状态 |
|---|---|---|
| `backend/.../auth.go:27-32/49-56/76-84` registerReq 与错误映射 | 代码 | ✅ 实测通过（实读 file:line 逐字命中） |
| `backend/user/service.go:154-224` Register 事务/预查/自动登录 | 代码 | ✅ 实测通过 |
| `backend/user/gorm_repo.go:23-25` CreateUser 无翻译 | 代码 | ✅ 实测通过（含"无 AutoMigrate、唯一约束名=SQL 内联命名"推断） |
| `backend/migrations/0001_user_schema.sql` 约束 | 代码 | ✅ 实测通过 |
| `Register.vue` / `api/auth.ts` / `client.ts` 现状 | 代码 | ✅ 实测通过 |
| `deploy/web.conf` 共桶事实 | 代码 | ✅ 实测通过（`compose.yml`、`web.conf` 在 f6b0600..origin/main 间零差异） |
| issue #76 状态与 body | 远端(GitHub) | ✅ 实测通过（`gh issue view 76`：OPEN、旧框架 body 属实） |
| `verify-deployed.sh` 存在于本地仓库 | 文件存在性 | ❌ 实测失败（本地 f6b0600 无此文件；仅 origin/main 有 → F3） |
| 仓库基座落后 | git | ✅ 实测：本地 main `f6b0600` 落后 origin/main 3 提交 |
| email 死字段 grep 复核 | 命令 | ✅ 实测通过（零命中） |
| `Settings.vue:49` 邮箱展示 | 数据流/消费方 | ✅ 实测：前置页面真实展示 → F4 |
| 409 code/suggestion 前端消费 | 数据流/消费方 | ✅ 代码核实（client.ts 现只透传 message、middleware/active.go 提供 code 先例）；前端逻辑未运行 |
| nginx 拆 zone 行为 | 命令 | ⚠️ 无法实测（本机无 nginx/线上；靠演练剧本） |
| `go build ./...` / `go test ./...` | 命令 | ⚠️ 无法实测（本评审未运行；文档 §8 列其为执行期命令） |
| `npm run test:unit` / `npm run build` | 命令 | ⚠️ 无法实测（本地无 `frontend/node_modules`） |
| 服务器 / 线上 DB | 远端 | ⚠️ 无法实测（无通道，与文档声明一致） |
