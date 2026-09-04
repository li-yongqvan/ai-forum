# issue #76 · 9/13 一次性免码注册 — 设计文档（供评审）

> 文档用途：交付专业评审 agent 的评审对象。范围 = 背景 / 真值核对 / 决策记录 / 实现方案 / 不变量 / 验证。
> 溯源约定：**事实**标来源（代码 `file:line` / DB 实查输出 / GitHub issue / grilling 用户确认）；**判断性裁决**单独标注【决策】并给出理由与备选，不冒充事实。
> 数据时点：2026-09-05（真值核对执行日；代码核对本机仓库 `C:\Users\liyongquan\ai-forum`；**未实连服务器/线上 DB**——设计基线期不需要，运行/演练见 §8）。
> 评审状态：**有条件通过（2026-09-05，独立复核）**，意见书 `issue-76-temp-open-registration-设计文档-评审意见书.md`。本版已回填采纳（§10）+ 事实修正；**B1 已执行（2026-09-05）**：issue #76 body/标题同步新模型 + 决策落档产品仓库（**独立文档 commit，先于 open commit**；落点 = 仓库既有 `docs/handoffs/grilling-decisions/` + `docs/plans/`，**未建 `docs/adr/`**——实查仓库无此约定，按实际调整，见 §10）。修订后为实现基线。

## 0. 项目上下文（给零背景评审 agent，先读本节）

**这是什么**：AI 智联论坛——社团 AI 主题社区移动端 MVP。本票 = 9/13 线下分享活动做**一次性免码注册**（数百人扫码注册、注册后长期可用，活动结束整体**回退**）。仓库 GitHub `li-yongqvan/ai-forum`，产品代码本机 `C:\Users\liyongquan\ai-forum`。

- **前端** `frontend/`：Vue 3 + TS + Vite + Vant（PWA）。路由 **hash 式**（`router/index.ts:1` import、`:5` `createWebHashHistory`），`/register` → `views/Register.vue`（`:7-8`）。
- **后端** `backend/`：Go（Gin + GORM + PostgreSQL）单进程模块化单体。入口 `cmd/api/main.go`，装配 config → database → migrate → 各域。
- **数据库**：PG 单实例多 schema（`user`/`content`/`notify`/`moderation`）。本票只涉及 `user.users`、`user.invitation_codes`。
- **部署**（`compose.yml`）：`web` nginx:1.27-alpine @8888（bind `deploy/web.conf`，`:6-17`）、`api`（`env_file: .env` `:39-40`，仅 `127.0.0.1:8080`，healthz wget `:52`）、`postgres`（`pgdata` 卷）。前端产物 CI scp 进 `frontend/dist`。发版由 CI `deploy.yml`（push main）驱动，含 `deploy/verify-deployed.sh` 镜像+迁移核对。
- **关键架构纪律**：领域包 4 个（`user` 等），跨包经 seam 由组合根装配（`main.go`）；哨兵错误 + handler 映射 HTTP 状态码；**迁移随应用启动执行、失败即 `os.Exit(1)`**（`main.go:41-44`）→ 发版会因启动即死触发自动回滚（issue #1 决策 35 教训）；**配置启动时只读一次**（`backend/internal/config/config.go` `Load`，`main.go:25`），无设置表 / 运行时翻转先例。
- **治理链**：封禁 admin-only（moderation 路由 + `user.Service.Ban`），users **永不硬删**（`user/model.go:21` `gorm.DeletedAt` 软删）。无批量删号工具。
- **术语速查**：本票用语（**免码注册 / open 版 / 原版 / 开放期 / 回退 / 占位邮箱 / 撞名 / 建议名 / 一键采用 / 当天零部署 / 紧急回退 / 回滚**）见 `agent panel/ai-forum/CONTEXT.md`（非 git，收口时并入产品仓库 `docs/ubiquitous-language.md`）；决策 ADR 原稿 `agent panel/ai-forum/docs/adr/0001`、`0002`（**定稿全文已收入产品仓库** `docs/handoffs/grilling-decisions/issue-76-temp-open-registration-decisions.md` §2）。**全文不设常驻开关。**

## 1. 背景与目标

- 需求来源：GitHub **issue #76**「9/13 活动·临时大规模注册（扫码免码开放注册）」（`gh issue view 76` 已核，OPEN，label `wayfinder:grilling`）。
- 交接依据：grilling（2026-09-04~05）逐项定稿 → 用户 2026-09-05 确认共识设计；domain-modeling 立语料 `CONTEXT.md` + ADR 0001/0002。
- **目标**：9/13 现场观众扫同一二维码 → 落地页只填 用户名+密码 → 注册即自动登录 → 正式 member 长期可用；活动结束**回退**恢复邀请码注册；数百人并发注册成功、撞名体验友好；9/13 当天零部署。
- **模型转变（偏离登记）**：issue #76 body 原草稿为「默认关闸门 `.env REGISTER_OPEN`、常驻可逆开关、9/10 功能冻结、换服务器待定」；grilling 改为「**一次性临时改动、整包单 commit、回退 = git revert，不做常驻开关**」，且**换服务器已确定为必做**（迁移先行）。→ issue #76 body 尚未同步，派发前需更新（见 §9 Q1）。

## 2. 真值核对（数据来源，全部可复现）

> 代码真值 = 本机仓库实读（2026-09-05）。服务器/线上 DB **未实连**（设计基线；open 版部署/演练时按 SOP 执行并回填）。GitHub 状态以 `gh` 实查为准（2026-09-04）。

### 2.1 服务器真值

- 本票**不改部署形态**（仍是 compose web@8888 / api / pg）。部署相关事实均来自仓库文件（可复核）：`compose.yml`（api `env_file: .env` `:39-40`；迁移失败即退出的启动序列 `main.go:41-44`；bind 的 `deploy/web.conf`）。运行/演练/回退命令见 §8，届时按既有 SOP 实查并回填 HEAD、镜像 ID、healthz。
- **结论**：无自造服务器事实；评审可按 `compose.yml` + `deploy/*.sh` 复核部署假设。

### 2.2 DB 真值

- 依据**迁移脚本**（本地仓库，未连线上库；线上最终以 `verify-deployed.sh` / psql 复核）：
  - `backend/migrations/0001_user_schema.sql:7` `email VARCHAR(128) UNIQUE NOT NULL`；`:8` `username VARCHAR(64) UNIQUE NOT NULL`；`:14` `created_at TIMESTAMPTZ NOT NULL DEFAULT now()`；`:16` `deleted_at`（软删）。
  - `:36-40` `invitation_codes`：`used_by BIGINT UNIQUE` + `CHECK (used_by IS NULL OR used_at IS NOT NULL)` → 邀请码**一次性由 DB 兜底**。
- **结论**：砍邮箱若走 DROP/NULL 需新迁移，与「迁移失败→启动即死→自动回滚」（`main.go:41-44`；issue #1 决策 35）冲突 → 支撑 ADR-0002「占位邮箱零迁移」方向属实。

### 2.3 代码真值（本机仓库 grep/read，2026-09-05）

**注册后端**（`backend/`）：
- `internal/httpapi/handler/auth.go:27-32` `registerReq` 四字段全 `binding:"required"`，email 另带 `,email` → 现状强约束 邮箱+邀请码。
- `auth.go:49-50` 撞名/邮箱占用统一 **409「用户名或邮箱已存在」**（同文案）；`:51-54` 邀请码/弱密码 400；`:55-56` 其余 → **500「服务器内部错误」**；`:60` 成功 201 返回 AuthResult。
- `user/service.go:154` `minPasswordLen = 8`；`:173-174` 密码不足→`ErrWeakPassword`；`:176-178` 空用户名/邮箱→error（open 版须去）；`:180` bcrypt；`:186-220` `Register` 单事务：`:188-191` 邀请码校验（不存在/已用→`ErrInvalidInvite`）、`:193-202` 用户名/邮箱唯一预查、`:204-213` `CreateUser`、`:215-217` `MarkInvitationCodeUsed`；`:224` `issueAuth` → **注册即自动登录**（返回 token）。
- `user/gorm_repo.go:23-25` `CreateUser = Create(u).Error` —— **无 OnConflict、无 23505 翻译** → 并发下撞 DB 唯一约束会原样冒泡到 handler 默认分支 500（此为新增映射的依据，见 §6.3）。
- `user/model.go:10-14` `Email`/`Username` 均 `uniqueIndex`；`:19-21` 时间戳+软删。`user/service.go:415-419` `toView` **把 Email 放进视图**（`:412` AuthResult.User、`:390/403` me 都带）→ 占位邮箱会随 API/`/auth/me` 返回。**F4 实核：占位邮箱在本人「设置」页展示**——`frontend/src/views/Settings.vue:49` `<van-cell :title="auth.user?.username" :label="auth.user?.email" />`，注册者本人可见 `u_<hex>@local.invalid`；不对**其他用户**展示。原稿"无页面展示/低危"与 ADR-0002「永不外发」措辞**不成立** → 修正见 §4 #12 / ADR-0002 措辞（已改）。
- 邮箱死字段（无发信/找回/通知用途）：只读全库扫描（2026-09-04，@main）无 SMTP/找回路径 → ✅ 属实（评审可 `grep -ri "smtp|ResetPassword|SendMail" backend/` 复核）。

**前端**（`frontend/`）：
- `views/Register.vue:39-42` 表单 用户名/邮箱/密码/邀请码 四字段；`:13-16` 四字段任一空→toast「请填写全部字段」；`:17-26` 提交。
- **纠正（设计稿原说法有误）**：`Register.vue:10` `const loading = ref(false)`，`:17` 提交置 true，`:25` finally 复位，`:45` `<van-button :loading="loading">` → **现状已有「请求中连点禁用」**（Vant loading 即 disabled）。原计划稿「加按钮禁用」是 no-op → 修正为：保留现状，可加 submit 顶部重入守卫封死极短双击窗（§6.3）；**真并发（双设备抢名）只能靠后端映射解决**。
- `api/auth.ts:8-14` `register(payload)` 载荷四字段全必填；`stores/auth.ts` 成功后存会话（自动登录）。
- 前端 **无 409 差异化分支**（错误只 `showToast(e.message)`，`Register.vue:22-24`）；`client.ts` 只透传 `{error}` 文案 → 撞名 UX 需要可机读的 409 标记（§6.2）。

**限流**（`deploy/web.conf`，本地实读）：
- `:2` `auth_limit` zone `rate=10r/m`；`:23-30` login `burst=20 nodelay`；`:31-38` register **同 zone** `burst=20 nodelay` → **login/register 共桶**，无法只放 register（拆 zone 依据，D5）。

### 2.4 GitHub 状态（`gh`，2026-09-04）

- issue #76 OPEN、label `wayfinder:grilling`、body 为旧框架草稿（见 §1 偏离登记）。
- issue #1（地图）已含决策 35（迁移失败→自动回滚）等；本地镜像 `wayfinder/.issue1-latest.md` 与 live 一致（统筹.md 2026-09-04 核对）。
- 本机产品仓库 = `C:\Users\liyongquan\ai-forum`。**F3 基座声明**：本地 main `f6b0600` 落后 `origin/main`（`5f62910`）3 提交（#74/#75 部署：新增 `deploy/verify-deployed.sh`、改 `deploy-server.sh` 加 nginx reload 等）；评审 diff 证实该 3 提交**未触碰** `backend/`、`frontend/`、`compose.yml`、`web.conf` → §2.3 代码真值对 `origin/main` **同样成立**；§8 引用的 `verify-deployed.sh` 等 deploy 事实以 `origin/main` 为准。**open commit 基座 = `origin/main`（先 `git pull`，记录 HEAD）**。

## 3. Grilling 决策记录（用户确认，可复核）

| 编号 | 决策问题 | 定案 | 依据 |
|---|---|---|---|
| D1 | 免码能力形态 | **一次性临时改动**，整包收成一个 commit，**回退 = `git revert` + 重发版**；**不做常驻开关/feature flag**（否掉 .env REGISTER_OPEN、DB 运行时翻转、共享活动暗号） | 用户确认 2026-09-05；ADR-0001；CONTEXT.md「闸门/开关」入废词 |
| D2 | 邮箱处理 | email 死字段**维持 NOT NULL UNIQUE、零迁移**；注册统一生成**占位邮箱** `u_<crypto/rand 16hex>@local.invalid`（罕见撞库重试一次）；事后识别只靠 created_at 窗口，**不打标不加列** | 用户确认 2026-09-05；ADR-0002；依据 §2.2/§2.3 |
| D3 | 密码策略 | **≥8 保留**（前后端零改动，长期账号安全基线不打折） | 用户确认 2026-09-05 |
| D4 | 撞名体验 | **409 + 最新建议名**（`name_2…`）；前端**一键采用** + **我自己改**退出路径，不强制 | 用户确认 2026-09-05（含「必须设退出键」） |
| D5 | 限流 | register **拆独立 zone 放宽**（60r/m、burst 120/IP）；login **保持严**（原 auth_limit 10r/m+burst20 不动） | 用户确认 2026-09-05；依据 §2.3 web.conf 共桶事实；备选=不拆（拒绝，Login 会被一起放水） |
| D6 | 陌生人/机器人策略 | **全开放免码**，不引入验证码/活动暗号；靠限流 + 事后 created_at 审计 + 单用户封禁清理 | 用户确认 2026-09-05；备选=URL 令牌（拒绝：同 URL 转发/扫码不可区分） |
| D7 | 演练环境 | **隔离 compose**（web+api+pg 同构、独立端口 + 独立 pg 卷）压完删栈，零污染线上 | 用户确认 2026-09-05（线上唯一 + 无批量删） |
| D8 | 并发/双击加固 | 现状 button `:loading` 已防连点；**后端把 username 23505→`ErrUsernameTaken`（走 409+建议名）**，防并发抢名裸 500 | 用户确认 2026-09-05（纠正前误判后并入）；依据 §2.3 CreateUser 无翻译 |
| D9 | 回退时机/操作 | **9/14 早上回退**；执行会话按 runbook 操作（敏感操作逐项授权）；当天零部署唯一例外=紧急回退 | 用户确认 2026-09-05 |

## 4. 范围收敛与明确不做

| 项 | 决策 | 依据 |
|---|---|---|
| 常驻开关 / feature flag / REGISTER_OPEN | **不做** | D1/ADR-0001 |
| 一人一码分发、图形验证码、活动暗号 | 不做 | D6 |
| email 列 DROP/NULL、新增迁移 | 不做（零迁移） | D2/ADR-0002、§2.2 |
| 批量删号/清窗工具 | 不做（预案可选项，垃圾号成批才写） | D6/§2.3 治理能力 |
| login 限流放宽 / 登录语义改动 | 不做 | D5/D3、§2.3 |
| 「必须扫码才可注册」门禁 | 不做（同 URL 无法区分扫码 vs 转发） | D6（§9 Q7） |
| QR 生成 | 延后到**迁移后**最终 URL（代码只依赖 `/register` 路径） | §1/§9 Q5 |
| **偏离登记**：issue #76 body 旧框架（常驻闸门/9/10 冻结/换服务器待定）与 grilling 新模型不符 | 以本 doc 为基线；**派发前更新 #76 body** | §1、§9 Q1 |
| 占位邮箱在本人「设置」页展示（F4） | 前端对 `@local.invalid` 隐藏该 label（或固定文案「占位邮箱」）；**后端不动** | F4/Q6 裁决、§2.3 `Settings.vue:49` |
| 决策/ADR 落档（B1②） | **已执行（2026-09-05）**：按产品仓库既有约定落 `docs/handoffs/grilling-decisions/issue-76-temp-open-registration-decisions.md`（ADR 0001/0002 全文收入其 §2）+ 本 doc/意见书入 `docs/plans/`；**独立文档 commit 先于 open commit**（9/14 revert 不删文档）。未建 `docs/adr/`——实查仓库无此约定，评审 B1 原话按仓库实际调整 | B1（评审意见书 §五） |

## 5. 实现方案（每项给依据）

### 5.1 后端 `backend/`（open 版一次性改动）

1. `internal/httpapi/handler/auth.go`：`registerReq` 的 `email`/`invite_code` **binding 标签整条去掉**（含 `,email`——仅去 `required` 会留空串仍触发 email 校验 → 400），username/password 保留 `required`。依据 D1/D2/F5⑤、§2.3。
2. `user/service.go` `Register`（事务内）：跳过 `GetInvitationCode`/`MarkInvitationCodeUsed`；email 忽略入参、统一生成占位邮箱（crypto/rand 16hex，罕见撞库重试一次——重试包住生成环节）。**保留 trim 后空 username → 400 守卫**（`required` 对 `"   "` 放行，不守卫会插空串撞 `users_username_key` → 被误判撞名）；仅去掉 email 空值条件。依据 D2/F5④、§2.3 `service.go:176-178/188-191`。
3. **建议名 seam** `svc.SuggestNextUsername(ctx, username)`（user 包方法，非 handler 计算）：探测口径与 Register 判重一致（`GetUserByUsername` → **banned 也占用名字**；软删不占）；原名 ≤ VARCHAR(64)，建议名生成时**截断保证 ≤64**（防 `name_2` 超长插库 22001→500）。依据 D4/F5①④、§2.3。
4. **23505 → 哨兵，下沉到 `gorm_repo.CreateUser` 适配层**（不在 service/handler——service/测试不沾 pgx 驱动）：`ConstraintName` 命中 `users_username_key` → `ErrUsernameTaken`、`users_email_key` → `ErrEmailTaken`、**其它/未知 23505 → 原样返回（宁 500 不误吞为 409）**。约束名存常量 + 单测覆盖（含 `ConstraintName` 为空样本；映射单测需真 PG `internal/testutil.SetupPG` 或 fake 注入 `*pgconn.PgError`）。依据 D8/Q2/F5③、§2.3 `gorm_repo.go:23-25`、`auth.go:49-50`。
   - 备选（不选）：`CreateUser` 失败后在事务内重查 username 判存在 → 无驱动依赖，但无法区分 email 冲突（占位碰撞已由生成重试兜底，基本到不了）→ 仍选约束名方案。
5. 撞名 409 载荷：handler 捕获 `ErrUsernameTaken` → 调 `svc.SuggestNextUsername(ctx, username)` 现算建议名 → 409 `{error, code:"username_taken", suggestion}`（`suggestion` **不进错误对象**，经 seam 方法取）。前端每次用**响应里最新 suggestion** 重渲染（Q4 铁律）。依据 D4/F5①/§6.2。

### 5.2 前端 `frontend/`（open 版一次性改动）

1. `views/Register.vue`：字段收敛为 用户名+密码；去掉四字段断言；载荷只带两字段。依据 D1/D2。
2. 撞名交互：捕获 409 且带标记 → 用户名下内联「`xxx` 已被占用，试试 `xxx_2`？」+ [就用这个名字]/[我自己改]。依据 D4。
3. 双击：**保留现状** `:loading`；可选在 submit 顶部加 `if (loading.value) return` 重入守卫封死极短双击窗。依据 D8/§2.3 现状已防。
4. `api/client.ts`：`ApiError` 增加 `code`/`suggestion` 字段（当前只抛 `{status,message}`，`client.ts:12-18/64-66`）。`api/auth.ts` + `stores/auth.ts` + `types.ts`：register 载荷 email/invite 收窄为可省——**`vue-tsc`（`npm run build` 前置、deploy 阻塞）会因载荷类型变化波及这三处，须同步放宽**，不能只改 `Register.vue`。依据 §2.3/F5⑥。

### 5.3 部署 `deploy/`（open 版，随 commit 回退一并撤销）

- `web.conf`：新增 `zone=register_limit:10m rate=60r/m`；`location = /api/v1/auth/register` 改用 `register_limit burst=120 nodelay`；login 保持 `auth_limit`。依据 D5/§2.3。该文件 bind-mount（`compose.yml:17`），随发版 git pull + `nginx -s reload`。

### 5.4 收尾（一次性、无代码残留）

- 记录 open 版上线 / 回退上线的精确时刻（部署日志）→ 只读查询 `SELECT … FROM user.users WHERE created_at BETWEEN …` 导出开放期清单存档；可疑号逐个 admin 封禁。依据 D2/D6/ADR-0002（email 无识别价值）。**F7 时区**：`created_at` 为 TIMESTAMPTZ（UTC 存储），runbook 查询显式带 UTC 偏移（如 `'…+00'`）并对齐部署日志同源时刻，避免窗口漂移。

## 6. 关键设计裁决（【决策】，含理由与备选）

### 6.1 「扫码才可注册」门禁 → 不做
- **问题**：是否把免码注册限制为「只有扫二维码进来的人」。
- **定案【决策】**：不做。open 版上线即开放，URL 未公开为唯一防线。
- **理由**：同一 URL 扫进 vs 转发给他人**技术上不可区分**；要卡「人必须在场」需现场一次性动态码/进网绑定，改动量完全不同，与 D6「最小改动」冲突。
- **备选（不选）**：URL 携带共享令牌（仅挡爬虫、挡不住转发）；现场动态码（改动大、超范围）。

### 6.2 撞名 409 的机读契约（不只改文案）
- **问题**：一键采用需要前端在 409 时走专用分支，而现有 `client.ts` 只透传 message。
- **定案【决策】**：409 响应带可机读标记（`code=username_taken`）+ `suggestion` 字段；前端按标记分支，其它错误仍走 toast。文案保持人类可读。**传输契约**：`suggestion` 由 service seam `SuggestNextUsername` 产出，handler 捕获 `ErrUsernameTaken` 后调用取得，**不塞进错误对象**；每次 409 现算、前端以响应最新值为准（Q4 铁律）。
- **理由**：只改文案，前端无法区分「撞名」与其它 409（现状用户名/邮箱同文案），一键采用 UX 无从落地。
- **备选（不选）**：前端对每次 409 盲试 `name_2`（会产生用户没选的名字）；忽略（退回纯 toast，放弃 D4 目标）。

### 6.3 并发抢名/双击的兜底分层
- **问题**：双击（同设备）与并发抢名（双设备同秒同热门名）都可能「先查后插」竞态。
- **定案【决策】**：①双击靠现状 `:loading`（+可加重入守卫）；②**真并发靠 DB UNIQUE + 23505→`ErrUsernameTaken`**（唯一硬保险）。**翻译位置【决策】**：下沉 `gorm_repo.CreateUser` 适配层（约束名匹配 `users_username_key`/`users_email_key`），不在 service（见 §5.1#4）。D8。
- **理由**：DB 唯一约束是事实层兜底（§2.3 `0001_user_schema.sql:8`）；把 23505 翻译成撞名 409，让输家拿到的是「建议名」而不是 500——`gorm_repo.go:23-25` 现无此翻译。
- **备选（不选）**：仅前端防（覆盖不了双设备）；忽略竞态（输家 500，与 D4 目标冲突）。

## 7. 边界与不变量清单（含防护层）

| # | 不变量 | 防护层 | 依据 |
|---|---|---|---|
| 1 | 免码注册不消耗/不触碰邀请码 | open 版 service 跳过 Get/MarkInvitationCode；DB 一次性约束（`used_by` UNIQUE+CHECK）仍在原版生效 | §5.1/D1、§2.2 |
| 2 | 占位邮箱唯一且不碰撞 | 随机 16hex + email UNIQUE + 罕见重试一次 | D2/ADR-0002、§2.3 |
| 3 | 并发同秒同名只有一个赢家、输家拿 409 而非 500 | DB username UNIQUE（先到先得）+ 23505→ErrUsernameTaken | §2.2/§2.3/D8 |
| 4 | 同设备双击不重复注册 | 现状 button `:loading`（Vant loading=disabled）+ 可选 submit 重入守卫 | §2.3 `Register.vue:10/17/25/45` |
| 5 | 拆 zone 不误伤 login 防线 | register 独立 zone；login 保持 `auth_limit` | D5/§2.3 |
| 6 | 弱密码仍被拒（≥8） | `minPasswordLen=8` 保留，handler 400 同文案 | D3/§2.3 |
| 7 | 登录枚举防护不回归 | login handler 未动，ErrBadCredential/ErrBanned 仍统一 401 | §2.3 `auth.go:76-84` |
| 8 | 回退可整体还原 | 整包单 commit + `git revert` + verify 三件套（含 web.conf 撤 zone、邀请码拦截冒烟） | D1/D9/§8 |
| 9 | 开放期账号可审计识别 | created_at 窗口 + 收尾只读清单；email 无识别价值（占位） | D2/ADR-0002/§5.4 |
| 10 | 9/13 当天零部署（唯一例外=紧急回退） | runbook 铁律；紧急回退 = 立即 git revert 重发版 | D9/§8 |
| 11 | 旧 PWA bundle 不误导现场 | **已知缺口（Q3 评审降级）**：`vite.config.ts` `registerType:'autoUpdate'` 且未配 navigateFallback → 对 `/register` 的导航请求走网络取新 bundle，老缓存主要残留于离线/极端场景 → 真机演练覆盖，**暂不版本化** | §1/§8、评审 Q3 |
| 12 | 占位邮箱不向本人展示伪值 | 前端 `Settings.vue` 对 `@local.invalid` 的 email label 隐藏（或固定文案「占位邮箱」） | F4/Q6、§4 |
| 13 | 建议名与 Register 判重口径一致、不超长 | `SuggestNextUsername` 用同一 repo 判重（banned 占用、软删不占）；原名 ≤64、建议名截断 | F5①、§5.1#3 |
| 14 | 空 username 不落库 | service 保留 trim 后空 → 400 守卫 | F5④、§5.1#2 |

## 8. 测试与验证计划

- **后端单测**（open 版新增/回归）：占位邮箱格式与唯一性（含撞库重试）；跳邀请码路径（空/无效码不再拒）；建议名助手（顺序取空闲、含 banned 占用、≤64 截断）；**23505→哨兵映射在 gorm_repo 适配层**（构造唯一冲突、`ConstraintName` 命中/为空样本）；空 username 仍 400；弱密码≥8 拒绝（回归）。
- **既有测试改写清单（F2，open commit 必含、随 revert 还原；断言改验新语义而非删除）**：`user/service_test.go` `TestRegister_Success:22`（不再断言邀请码已用）、`TestRegister_WeakPassword:50`、`TestRegister_InvalidInvite:63`（免码下不再拒无效码）、`TestRegister_EmptyFields:90`（拆：空 email 放行、空 username 仍 400）、`TestRegister_Duplicate:103`（email 重复子测改为「忽略入参」）；`handler/auth_test.go` `TestRegisterLoginMeLogoutFlow:124`、`TestRegisterValidation:182`（email 格式/邀请码必填 400 不再成立）。
- **handler/契约**：注册 400/409/500 映射回归；409 body 的 `code=username_taken`/`suggestion` 形状；`client.ts` ApiError 增 code/suggestion。
- **前端单测/类型**：Register.vue 两字段载荷与 409 一键采用/我自己改；loading 守卫不回归；**载荷类型收窄波及 `stores/auth.ts`/`api/auth.ts`/`types.ts`（vue-tsc = `npm run build` 前置、deploy 阻塞）**；Register.vue 现无单测、`stores/auth.test.ts` 不覆盖 register。
- **隔离 compose 演练**（`deploy/` 同构：web+api+pg、独立端口 + 独立 pg 卷，压完删栈，零污染线上）剧本：
  - 总量 ≈300 注册 / 峰值 ≈50·分⁻¹；
  - ~10% 故意撞名 → 一键采用全通；
  - 单 IP 同秒 120 连发 → register 拆 zone 无人误 503、login 仍严；
  - **F6 超限样本**：单 IP 同秒 150（>burst 120）→ 部分 503 符合预期，据实调 rate/burst；加「持续 60+r/m」样本看 refill；
  - **同一用户名双击 / 并发 2 连发** → 恰 1 个账号、恰 1 个 201、另一 409（非 500）；
  - **双设备同秒抢同名** → 赢家 201、输家 409+建议名、0 个 5xx；
  - **真机老 bundle 验证（Q3）**：此前访问过站点的真机扫码 → 确认拿到 open 版两字段页。
  - 通过线：0 个 5xx/500、自动登录全通。
- **可复现命令**：`cd backend && go build ./... && go vet ./... && go test ./...`；`cd frontend && npm run test:unit && npm run build`。
- **发版/回退验证**（按既有 SOP，实查并回填）：`verify-deployed.sh`（镜像 ID + schema_migrations + healthz）绿；回退后用一次性测试号注册被「需邀请码」拦下；`web.conf` 已回共享桶。

## 9. 待评审焦点（Q1–Q7，已评审裁决 2026-09-05）

> 裁决全文见《…评审意见书.md》§四。采纳落地见 §10。以下为裁决摘要：
- **Q1 → B1（阻塞，升级为派发硬门）**：不是开放问题。issue #76 body 需同步为新模型（一次性单 commit、不做常驻开关；换服务器必做、迁移先行），派发指令声明本 doc 权威基线。**已执行（2026-09-05）**：body/标题已同步，落档已推 main。
- **Q2 → 裁决：下沉 gorm_repo 适配层按 `ConstraintName` 匹配**（§5.1#4）；约束名常量 + 空样本单测。
- **Q3 → 裁决：值得做、不需版本化**：`vite.config.ts` autoUpdate + 无 navigateFallback → `/register` 导航走网络取新 bundle；保留真机演练即可（§7 #11 已降级）。
- **Q4 → 裁决：可接受 + 铁律**：每次 409 现算 suggestion、前端用响应最新值重渲染（§6.2）。
- **Q5 → 裁决：不阻塞设计**；落库已执行（2026-09-05）：实查仓库无 `docs/adr/` 约定 → 按既有 grilling-decisions 结构入库（ADR 0001/0002 全文收入其 §2），「编号对齐」随之消解。
- **Q6 → 裁决：前提有误（F4）已修正**：占位 email 本人 Settings 可见（`Settings.vue:49`）；处置 = 前端隐藏 `@local.invalid` label + ADR-0002 措辞改（§4 #12）。
- **Q7 → 裁决：站得住**：open 期**任何访问者**可注册（非仅扫码者）；runbook 写实灌号处置（created_at 清单 + 逐号封禁），不再当"可选项"一句带过。

## 10. 评审意见采纳记录（2026-09-05）

| 评审项 | 结论 | 采纳落地 |
|---|---|---|
| **B1**（阻塞）issue #76 body 旧框架 + 决策未入库 | 属实，接受 | **已执行（2026-09-05）**：① issue #76 body/标题同步新模型（body 声明本 doc 为权威基线）；② 决策落档 = 产品仓库 `docs/handoffs/grilling-decisions/issue-76-temp-open-registration-decisions.md`（含 ADR 全文）+ 本 doc/意见书入 `docs/plans/` + INDEX 挂账；独立文档 commit、先于 open commit（revert 不删文档） |
| **F2** 既有测试改写清单缺失 | 属实，接受 | §8 已补「既有测试改写清单」（service 5 + handler 2） |
| **F3** 基座落后 origin/main 3 提交 | 属实，接受 | §2.4 基座声明：open commit 基座 = origin/main（先 pull、记录 HEAD）；代码真值经 diff 对 origin/main 同样成立 |
| **F4** 占位邮箱本人 Settings 可见 | 属实，接受 | §2.3 修正 + §4 + §7 #12（前端隐藏 label）+ 本 ADR-0002 措辞改 |
| **F5** suggestion 传输契约未定义 | 属实，接受 | §5.1#1-#5 契约补丁（binding 整条去标签 / 空 username 守卫 / SuggestNextUsername seam / 23505 下沉 / 409 载荷）+ §5.2#4（vue-tsc 波及） |
| Q2 23505 识别位置 | 采纳 | §5.1#4 适配层按约束名匹配 + 常量/空样本单测 |
| Q3 409 契约扩散/PWA | 采纳（不需版本化） | §6.2 + §7 #11 降级 + §8 真机验证 |
| Q4 建议名并发 | 采纳（+铁律） | §6.2：每次现算、前端用最新值重渲染 |
| Q5 迁移/QR/ADR 编号 | 采纳（不阻塞） | §9 Q5；docs/adr 落档并入 B1 |
| Q6 占位 email | 采纳（F4 前提修正） | 同上 F4 |
| Q7 风险口径 | 采纳 | §9 Q7 + §5.4/runbook 写实灌号处置 |
| F6 演练参数卡上限 | 采纳 | §8 演练补 150 超限 + 持续 60r/m 样本，据实调 web.conf |
| F7 收尾查询时区 | 采纳 | §5.4 显式 UTC 偏移 |
| F8 路径精度 | 采纳 | §0 修正（config 全路径 / router hash :1/:5） |

**推翻项**：无。**随评审同步修正**：agent panel `docs/adr/0002` 措辞改（不对其他用户展示、本人侧可见，附勘误）；`CONTEXT.md` 占位邮箱词条同步。
