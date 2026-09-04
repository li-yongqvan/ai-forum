# Grilling #76 9/13 一次性免码注册 — 决策记录

- **Ticket**: [#76 9/13 活动·临时大规模注册（扫码免码开放注册）](https://github.com/li-yongqvan/ai-forum/issues/76)
- **类型**: grilling / HITL + 独立评审裁决吸收
- **日期**: 2026-09-04~05（grilling）· 2026-09-05（评审裁决吸收 + 落档，随本文档 commit 入库）
- **参与**: Claude（#76 设计/统筹会话）+ @li-yongqvan + 独立评审方
- **产出**: D1–D9 定案 + ADR 0001/0002（§2 全文）+ 可审计设计文档 `docs/plans/issue-76-temp-open-registration-设计文档.md`（独立评审**有条件通过**，意见书同目录）；评审意见全部落地（§3）

---

## 目标

9/13 线下分享：主讲结束后现场观众扫**同一个二维码**免码注册（只填 用户名+密码，注册即自动登录，账号长期有效），预计数百人并发；活动后**整体回退**恢复邀请码注册；**9/13 当天零部署**。

---

## 1. 用户定案（grilling 2026-09-05，AskUserQuestion 逐项确认）

### D1 形态 = 一次性临时改动、整包单 commit、回退 = git revert；不做常驻开关

- **问题**: 免码能力做成什么形态——常驻开关还是一次性改动？
- **选项**: A 常驻 `.env` 闸门（REGISTER_OPEN + 重启）；B DB/运行时翻转；C 共享活动暗号；D 一次性临时改动、单 commit、`git revert` 回退。
- **决定**: **D**。
- **理由**: 该能力只用一次、保留价值为零；常驻开关徒增维护面与攻击面，违背「改动单点可逆」纪律。「闸门/开关/REGISTER_OPEN」入废词，open 版回退后不存在。

### D2 邮箱 = 占位邮箱零迁移

- **问题**: `users.email` 是 `NOT NULL UNIQUE` 死字段（无发信/找回/通知用途），砍邮箱怎么处理？
- **选项**: A DROP/改 NULL（需新迁移）；B 前端隐藏但后端仍必填；C 后端统一生成占位邮箱、schema 不动。
- **决定**: **C**：`u_<crypto/rand 16hex>@local.invalid`（罕见撞库重试一次，重试包住生成环节）；事后识别只靠 `created_at` 窗口，不打标不加列。
- **理由**: 迁移失败会让新镜像启动即 `os.Exit(1)` 并触发部署自动回滚（issue #1 决策 35 教训）；零迁移 + 随机串天然满足唯一性。

### D3 密码 ≥8 保留

- **问题**: 现场体验 vs 安全基线？
- **决定**: **保留** `minPasswordLen=8`，前后端零改动。
- **理由**: 账号长期有效，安全基线不打折。

### D4 撞名 = 409 + 建议名 + 一键采用（必须留退出路径）

- **问题**: 数百人抢热门名，撞名体验怎么做？
- **选项**: A 纯文案报错；B 前端盲试后缀；C 后端现算建议名随 409 返回，前端一键采用 + 「我自己改」。
- **决定**: **C**（用户明确要求「必须设退出键」，不强制采纳）。
- **理由**: 纯文案无法支撑一键采用；盲试会产生用户没选的名字。

### D5 限流 = register 拆独立 zone 放宽，login 保持严

- **问题**: 现状 login/register 共享同一 nginx 桶（10r/m），数百人注册会被误伤，怎么放？
- **选项**: A 整体放宽；B register 拆独立 zone（60r/m、burst 120/IP），login 不动。
- **决定**: **B**。
- **理由**: 拆开才能只放 register；整体放宽 = Login 防枚举防线一起放水。

### D6 陌生人/机器人 = 全开放免码，不引入验证码/暗号

- **问题**: 数百人公开免码，要不要限「扫码才能注册」？
- **选项**: A URL 携带共享令牌；B 现场一次性动态码；C 全开放，靠限流 + 事后 created_at 审计 + 单用户封禁。
- **决定**: **C**（风险由用户承接；评审 Q7 裁决站得住）。
- **理由**: 同一 URL 扫码 vs 转发技术不可区分；动态码改动量超范围。**口径写实**：open 期间**任何访问者**可注册（非仅扫码者）；runbook 备灌号处置（created_at 清单 + 逐号封禁）。

### D7 演练 = 隔离 compose

- **决定**: web+api+pg 同构隔离栈（独立端口 + 独立 pg 卷），压完删栈，零污染线上。
- **理由**: 线上唯一、users 永不硬删、无批量清理工具。

### D8 并发/双击 = 前端 loading 已防连点（纠正误判）+ 后端 23505→哨兵

- **问题**: 双击/双设备同秒抢名，先查后插竞态怎么兜？
- **决定**: ①现状 `Register.vue` button `:loading` 已防连点（Vant loading=disabled；设计稿原判「无防护」有误，已纠正）；②真并发靠 DB UNIQUE + `gorm_repo.CreateUser` 适配层把 23505 按约束名（`users_username_key`/`users_email_key`）翻译成 `ErrUsernameTaken`/`ErrEmailTaken` → 输家拿 409+建议名而非 500；未知 23505 原样上抛（宁 500 不误吞）。
- **理由**: DB 唯一约束是唯一硬保险；翻译下沉 repo 层避免 pgconn 驱动类型泄漏进 user 域包。

### D9 回退 = 9/14 早上；当天零部署唯一例外 = 紧急回退

- **决定**: open 版最迟 **9/12 上线**泡 ≥1 天 → 9/13 活动 → **9/14 早上 `git revert` + 重发版**；9/13 全天不推代码，唯一例外是紧急回退（连续 5xx≥10/分或注册堆积失败 → 立即 revert）。

---

## 2. ADR 定稿（全文收录；原稿 agent panel `ai-forum/docs/adr/`，编号系 #76 语料期内部编号）

### ADR-0001 — 免码注册做成一次性临时改动，整包单 commit、回退 = git revert

9/13 线下活动需要数百人扫码**临时免码注册**一次；曾考虑常驻 `.env` 开关（REGISTER_OPEN）+ 重启、DB/运行时翻转标志、共享活动暗号等方式。决定：**不做任何常驻开关 / feature flag**——把「跳过邀请码校验 + 占位邮箱 + 前端只填用户名密码 + register 限流拆 zone」整包收成**一个 commit**，事件结束后（9/14 早上）`git revert` 整体还原、重新发版。

理由：该能力只使用一次、保留价值为零；常驻开关徒增维护面与攻击面，且违背「9/10 功能冻结、改动单点可逆」的纪律。发版失败自动退回 `:rollback` 旧镜像是另一套机制（issue #1 决策 35），与本决策的「回退」不是一回事。

### ADR-0002 — email 死字段保留 NOT NULL UNIQUE，免码注册统一生成占位邮箱（零迁移）

email 在系统内无发信/找回/通知用途，但 DB 列 `email VARCHAR(128) UNIQUE NOT NULL` 强约束（业务全链路均读该字段）。活动砍邮箱若直接 DROP 或改 NULL，需新增迁移；而迁移失败会让新镜像启动即 `os.Exit(1)` 并触发部署自动回滚（issue #1 决策 35 教训）。决定：**不动 schema**——免码注册由后端统一生成占位 `u_<随机16hex>@local.invalid` 存入 email 列。

理由：零迁移（规避历史「静默回退」风险）+ 天然满足唯一性。占位邮箱只出现在**本人侧**（随本人 API/`/auth/me` 返回、本人「设置」页账号栏展示，`frontend/src/views/Settings.vue:49`），不对**其他任何用户**展示或外发——跨用户泄露面为零。后果：这批账号 email 全是假的，**无法用 email 识别或找回**；事后批次识别一律靠开放期的 created_at 窗口，不新增打标列。

> **勘误（2026-09-05，评审 F4）**：本 ADR 原稿「占位永不外发、无泄露面」表述不精确——占位邮箱随本人 API 返回、且会在本人设置页展示；正确的边界是「不对外部用户展示」。措辞已改；产品随 #76 前端在本人侧隐藏该 label（不改变零迁移决策本身）。

---

## 3. 独立评审裁决吸收（意见书 2026-09-05，有条件通过）

评审对象 = `docs/plans/issue-76-temp-open-registration-设计文档.md`；逐条采纳明细以其 **§10 采纳记录表**（13 行，无推翻项）为准，本节只列要点：

- **B1（阻塞）canonical 分叉** → 已执行（即本 commit + issue body 同步）：issue #76 body/标题改新模型并声明设计文档为权威基线；决策落档（本文件）+ 设计文档/意见书入 `docs/plans/` + `docs/handoffs/INDEX.md` 挂账；**独立文档 commit 先于 open commit**（9/14 revert 不删文档）。
- **F2 既有测试改写清单**（`go test ./...` 是 deploy 阻塞门）→ 设计文档 §8 已列（service 5 + handler 2，断言改验新语义）。
- **F3 基座** → open commit 基于 `origin/main`（先 pull、记录 HEAD）；本地曾落后 3 提交（#74/#75 deploy，未触碰 backend/frontend/compose/web.conf）。
- **F4 占位邮箱本人可见** → 前端 Settings 隐藏 `@local.invalid` label + ADR-0002 勘误（上文 §2）。
- **F5 suggestion 传输契约** → `svc.SuggestNextUsername` seam（探测口径同 Register 判重、banned 占用、≤64 截断）+ 23505 下沉 gorm_repo + 409 `{error, code:"username_taken", suggestion}` + registerReq binding 整条去标签 + 空用户名 400 守卫 + 前端载荷类型收窄波及三文件。
- **F6/F7/F8、Q1–Q7** → 设计文档 §8（演练超限样本）/§5.4（UTC 偏移）/§0（路径精度）/§9（逐项裁决）。

---

## 4. 落档说明（对评审 B1「建 docs/adr/」的调整）

实查产品仓库**无** `docs/adr/` 目录与 ADR 编号序列；issue 决策的既有约定 = `docs/handoffs/grilling-decisions/<issue>-<slug>-decisions.md`（#8/#12/#32/#33/#34/#47/#50/#54/#59/#60/#61/#74 均如此）。故**不新建 `docs/adr/`**，ADR 0001/0002 定稿全文收入本文件 §2（编号系 #76 语料期内部编号，入库不另立序列）。

#76 术语词条（**免码注册 / open 版 / 原版 / 开放期 / 回退 / 占位邮箱 / 撞名 / 建议名 / 一键采用 / 当天零部署 / 紧急回退 / 回滚**，及废词「闸门/开关/REGISTER_OPEN」）暂存 `agent panel/ai-forum/CONTEXT.md`（非 git），收口时并入 `docs/ubiquitous-language.md`。

---

## 5. 下一步（实现会话入口）

- **权威基线** = `docs/plans/issue-76-temp-open-registration-设计文档.md`（§5 逐文件改动含依据、§7 不变量 14 条、§8 测试/隔离演练剧本）。
- 基于 `origin/main` 开发（先 pull、记录 HEAD）；`go test ./...` / vue-tsc（`npm run build` 前置）是 deploy 阻塞门，既有测试改写清单见设计文档 §8。
- 排期锚点：**换服务器已定必做、迁移先行**（≈9/10）→ open 版最迟 9/12 上线泡 ≥1 天 → 9/13 活动 → 9/14 早回退。QR 用迁移后最终 URL。

*本文件为 #76 执行基线的一部分；9/14 的回退（git revert open commit）不触碰本文件（独立文档 commit）。*
