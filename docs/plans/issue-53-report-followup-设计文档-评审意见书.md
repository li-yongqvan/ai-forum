# Issue #53「举报闭环遗留收尾」设计文档 评审意见书

> **评审对象**：《#53 举报闭环遗留收尾：被举报人触达 + 举报频控 — 设计文档（供评审）》（2026-08-24）
> **评审方式**：独立复核 —— 本机仓库 `C:\Users\liyongquan\ai-forum` @ `0301601`（main HEAD）为真值源，逐项核对 `file:line` 引用；并**实际复现**服务器与线上 DB 实查（SSH 端口 2222、`docker compose exec` psql），对 7 个定向焦点（Q1–Q7）逐条裁决。
> **评审结论**：**有条件通过**

---

## 一、总体结论

方向正确、证据纪律在本项目历史中属最扎实一级。两大需求（被举报人触达 + 举报频控）与交接依据、grilling 定案、既有架构（快照自足 / seam / 迁移器约束）全部对齐；关键硬约束（`notifications_type_check` 约束名、迁移器 `;` 拆分、delete 路径不 ResolveTarget）经实查为真，设计对它们的处置（快照列、确定性约束名、禁 DO 块）全部站得住。

动身前需要消化的问题只有一类，且**无阻塞项**：

1. **一处文档同步遗漏（F1，重要）**：设计只把 `notify/README.md` 列入防漂移同步点，却漏了 `backend/moderation/README.md`——该文件有 3 处写死「report_result 只发举报人」的描述和错误表，本票落地后全部陈旧。这违背文档自己强调的「防文档漂移」原则，属同一类工作里的自我不一致。
2. 若干执行级细节（F2–F6，建议），均为低成本钉死项，不影响设计骨架。

总体评价：**事实基础可以全盘信任，按 §五 勾选清单执行即可作为落地基线。**

---

## 二、事实与证据复核

复核方式：本机仓库逐行核对 + **复现服务器/DB 实查**。无仓库不可核验项，故不做「不可复核」保留——除 §2.5「origin/main」与 GitHub issue 状态外全部亲手验证。

### 2.1 核实为真（全部命中）

| 计划主张 | 复核结果 |
|---|---|
| HEAD = `0301601`（PR #52） | ✅ 本地 HEAD、`origin/main`、服务器 `git log` 三处一致 |
| 部署形态 3 容器 + healthz ok | ✅ 复现 `docker compose ps`：api/postgres Up(healthy)、web Up；`curl healthz` → `{"status":"ok"}` |
| `notifications_type_check` 约束名 | ✅ 复现线上 DB 查询：`notifications_type_check | c`（另有 `notifications_pkey | p`） |
| `Report` 模型无被举报人字段（`moderation/model.go:8-21`） | ✅ 逐行核实，确无；`ReportView` 亦无 |
| `notifyReportResult` 只发举报人、无 actor（`service.go:354-367`） | ✅ `RecipientID: r.ReporterID`、`Type: report_result`，无 Actor 字段 |
| `conclusionFor` 三句结论（`:177-186`） | ✅ delete→「内容已删除」/ warn→「已警告违规用户」/ dismiss→「未采取处理」 |
| `HandleReport` O1 序列 + 提交后通知（`:275-351`） | ✅ 事务外副作用 → 单事务 → 提交后通知；S3 注释在 `:349` |
| delete 路径故意不 ResolveTarget（`:307-317`） | ✅ `:308` 注释「不先 ResolveTarget：已删目标须能幂等重处理」；adapter `:46-68` 把「不存在/已删」当成功 |
| `CreateReport` 已 ResolveTarget + 自举报拒绝（`:236-251`） | ✅ post/comment 分支 `ref.AuthorID`、user 分支比对 |
| 哨兵错误三件套（`:138-142`） | ✅ `ErrNotFound/ErrDuplicatePending/ErrInvalidAction` |
| 频控现状无时间窗口逻辑 | ✅ 全仓核实：仅 `uq_reports_pending` + `PendingExists`；`reports.created_at`（0004:16）与 `idx_reports_reporter`（0004:19）可支撑计数 |
| `respondModerationError` 无 429（`handler/moderation.go:28-39`） | ✅ 仅 404/409/400/500 |
| `CreateReport` handler 201 + Identity（`:69`, `:43-47`） | ✅ 属实 |
| 路由 `writers.POST("/reports")`（`router.go:128`）；mod 组三端点（`:137-139`） | ✅ 属实 |
| Notifier adapter 无 ActorAvatar（`moderation_adapter.go:120-131`）；`NotificationCmd` 最小子集（`service.go:120-129`） | ✅ 属实 |
| 迁移器 `;` 拆分（`migrate.go:64-75`）+ 每文件单事务（`:43-57`）+ 失败 fatal（`cmd/api/main.go:41-43`） | ✅ 属实 |
| `0003` type CHECK + VARCHAR(16)（`:7`）；`report_handled` 14 字符可装 | ✅ `0003:7` 原文；14 ≤ 16 |
| `0006` 纯 additive 先例 | ✅ `ADD COLUMN reporter_note TEXT;` 单语句 |
| 前端 `types.ts:124` 5 类联合 | ✅ 属实 |
| `Notifications.vue` textFor `:70-71`、linkTo default→null `:88-89`、头像 `|| '?'` `:146`、`:3` 注释 | ✅ 全部属实 |
| `Reports.vue:42` 警告文案 + `:35` 注释 | ✅ 属实 |
| `client.ts` ApiError 抛出 `:64-67`、429 不在 sessionLost `:57-58` | ✅ 429 正常透传为 `ApiError` |
| `ReportSheet.vue:66-69` toast 透出错误文案 | ✅ 属实，前端举报入口无需改成立 |
| 既有断言 `service_test.go:151/176/193` 均 `len(notify.calls)==1` | ✅ 逐行核实 |
| 前端旧文案断言 `Reports.test.ts:65/72` | ✅ 逐行核实 |
| 集成测试 helper `reportCode/handle/registerNotifyUser/makeModerator` 可复用 | ✅ 定义于 `moderation_test.go:42-48/:154`、`notify_test.go:25`、`content_test.go:66` |
| `testutil.SetupPG` 自动跑全量迁移 | ✅ `db.go:83-85` `migrations.Run(sqlDB)` → 0008 在测试容器真实执行，验证约束名成立 |
| fake repo 存指针（`fake_repo_test.go:43`） | ✅ `f.reports[r.ID] = r` |
| issue #53 OPEN；工作区仅 untracked 文档 | ✅ `gh issue view 53`：OPEN；`git status`：全部 `??` 无 `M` |
| 交接依据 handoff §3/§4 红线 + issue-33 grilling 决策（D33-2/O3/O4） | ✅ 两文件均在、内容吻合 |

### 2.2 不可复核项

无。服务器与 DB 真值均由评审方亲手复现（评审方式已注明），无需保留。

### 2.3 对「已核实」声明本身的核查

文档的溯源声明（事实标 `file:line`、判断标【决策】、数据时点标注）**在全部抽查项上成立**，未发现把「没核过」标成「已核过」的情况。§2.2 约束名的实查结果与 §5.1 迁移设计直接挂钩，是本项目罕见的「证据直达决策」闭环。

---

## 三、逐条评审（grilling 决策 + 关键设计裁决）

| 决策/选择 | 结论 | 评审意见 |
|---|---|---|
| D1 双发：保留 `report_result` 给举报人 + 补发 `report_handled` 给被举报人 | **认可** | 与 grilling 用户定案一致；两类型各配前端模板，视角不错乱。 |
| D2 被举报人收 delete/warn，dismiss 不收 | **认可** | dismiss = 不处理，通知作者反而制造无谓惊扰；举报人照旧收结论。语义自洽。 |
| D3 全局窗口 10min/5 次（跨目标） | **认可（附条件）** | 50 人规模够用。条件：窗口计数**含已 dismiss 的举报**是显式取舍，建议 §7 记为已知精度（见 F7）。 |
| D4 超限 429 不落库 | **认可** | 与「软限」配套成立；`respondModerationError` 加 429 映射唯一落点，正确。 |
| §6.1 新增 `report_handled` 类型 | **认可** | 复用 `report_result` 会让被举报人收到「举报结果：…」举报人视角文案，且会连带改举报人侧文案与既有断言（§2.4 已证实），blast radius 更大。命名 `report_handled` 与域内 `HandleReport`/`/handle` 动词一致，优于 `report_processed`。 |
| §6.2 只带原因枚举、不拼 `reporter_note` | **认可** | 自由文本可能含攻击性/身份线索，泄露给被举报人 = 隐私 + 升级风险；取舍正确。 |
| §6.3 不发 actor | **认可（附条件）** | 后端不引魔法字符串、不暴露 moderator，方向对。条件见 Q4 裁决（前端可用低成本替代「?」）。 |
| §6.4 检查顺序：目标校验 → 409 → 429 → 落库；软限 | **认可** | 重复举报给 409 比 429 更准确；并发超发 1–2 条在 MVP 可接受，硬不变量 `uq_reports_pending` 不动。 |
| §6.5 CreateReport 快照 `target_author_id` | **认可（全文档论证质量最高的一条）** | delete 路径不 ResolveTarget 是既有定案（幂等重处理），处理时现查作者必然不可靠；快照是唯一正解，且与 notify「快照自足」哲学一致，零额外查询。 |
| 迁移 0007（additive 可空列） | **认可** | 沿 0006 先例；GORM snake_case 自动映射 `target_author_id` 成立。 |
| 迁移 0008（DROP IF EXISTS + ADD 约束） | **认可** | 约束名已线上实查为 `notifications_type_check`；两 ALTER 被单事务包裹，ADD 失败 DROP 一并回滚；`;` 拆分安全（CHECK 内无半角分号）；testcontainers 测试在全新库重放 0008，构成有效守卫。 |

---

## 四、开放点裁决（Q1–Q7）

### Q1 新增 `report_handled` 类型 —— **裁决：成立**
备选「复用 report_result + 改前端模板」未被低估：它须同时承担 ① 改变现有举报人侧文案；② 改既有断言（§2.4 三处）；③ 通知列表无法用 type 区分两类受众——任一条都是不必要的新增成本。`report_handled` 命名维持。

### Q2 不拼 `reporter_note` —— **裁决：取舍合适**
隐私与升级风险 > 信息量收益。唯一补充：若未来要升级，应新增独立列快照（而非塞进 `target_title`），届时再议。

### Q3 频控参数 10min/5 次 + 软限 —— **裁决：MVP 合理**
用户已 grilling 确认；软限与「硬不变量仍是 uq_reports_pending」的表述准确。补充两点执行精度：① 6 连报测试中**第 6 次必须打在全新目标上**才能命中 429（打在已报目标会先被 409 拦截）——测试需准备第 6 个帖子；② 窗口计数含 dismissed 举报是显式取舍，建议写进 §7。

### Q4 头像显示「?」 —— **裁决：接受，但有一个更便宜的前端修复**
后端不发 actor 是对的（避免治理域魔法字符串 / 暴露 moderator）。但「?」在 UX 上读作「数据缺失」而非「系统通知」。建议**不动后端**，在前端 `Notifications.vue` 对 `report_handled` 渲染系统默认头像（如把 `'?'` 换成中性占位/图标，仅渲染层判断），一行成本消除歧义。若不愿加前端分支，接受「?」也可，但应在设计文档记明。

### Q5 存量 NULL 回填与否 —— **裁决：不回填正确**
存量 pending 报告的 post/comment 可能已删，join 不可靠；0006 先例为纯 additive；NULL → 跳过 + slog.Warn 诚实且可诊断。回填会引入不可逆的数据操作，MVP 不值。

### Q6 0008 约束名动态兜底 —— **裁决：不需要更强兜底**
约束名已线上实查、且 Postgres 对内联列级 CHECK 的自动命名 `<table>_<column>_check` 在全新库同样确定性成立——testcontainers 集成测试恰好同时验证线上与全新库两条路径。残留风险（假设未来该约束被手工改名，`DROP IF EXISTS` 空操作 + ADD 会建出**第二个**约束）属理论场景，且会被集成测试暴露，接受即可。

### Q7 warn 反转 #33 O4 是否符 #53 意图 —— **裁决：符合，且偏离处理得当**
补被举报人触达（含 warn）正是 #53 的立意；D2 已经用户 grilling（AskUserQuestion）确认，SOP §3 第 0 步流程合规；前端 `Reports.vue` 文案同步改为「警告（会通知被举报人）」使 UI 与行为一致。**建议补一步**：在 `issue-33-report-moderation-decisions.md` 的 D33-2/O4 处追加修订注记（本票反转，见 F5），避免未来读者以旧定案为准。

---

## 五、新发现问题（文档未覆盖，评审方补充）

| # | 级别 | 问题 | 要求 |
|---|---|---|---|
| **F1** | **重要** | **`backend/moderation/README.md` 未列入同步范围**：`§5.3` 只计划更新 notify README，但 moderation README 有 3 处随本票变得陈旧——`:8` 范围「`report_result` 通知（只发举报人，#33 D2）」、`:25` Ordering「提交后事务外发 `report_result` 通知」、`:30-35` 错误表（无 `ErrRateLimited`）。 | 将 `moderation/README.md` 纳入同步：范围/Ordering 描述改为双通知；错误表补 `ErrRateLimited → 429`；不变量补「被举报人通知仅 delete/warn」与频控软限说明。 |
| F2 | 建议 | `report_handled` 的 `target_title` 承载**整句文案**（语义过载）：该字段在 notify 域定义是「跳转锚点 + 标题」，此处被用作消息体；`TargetType`/`TargetID` 是否随通知下发也未写明。 | 在 §5.2/§6 明示这是刻意的语义复用；显式写明 report_handled 是否携带 `TargetType`/`TargetID`（建议带，供未来客户端筛选；前端 linkTo 已返回 null 无死链风险）。 |
| F3 | 建议 | §8 集成测试两处描述有歧义：①「warn→作者 2 条」需点明是**累计**（dismiss 0 → delete_post 1 → warn 2）而非单场景 2 条；②频控测试需**第 6 个全新帖子**才能命中 429（第 6 次打在旧目标会被 409 先拦）。 | 测试描述写明累计语义与第 6 个独立目标。 |
| F4 | 建议 | 「作者先自删 + 后续处理」边角：作者自删内容后，mod 处理该 pending 举报会触发 `delete_post` 审计 + 被举报人收到「你的内容已删除」通知——作者会困惑「我早删了」。这是 O1-② 已接受的 MVP 精度，如今第一次变成用户可见文案。 | §7 不变量补记此边角并接受；无需改逻辑。 |
| F5 | 建议 | `issue-33-report-moderation-decisions.md` 的 D33-2（只发举报人）与 O4（警告仅留痕）将被本票反转，文件将变陈旧，未来会话可能以旧定案为准。 | 随本票 PR 在该决策文件追加修订注记（D33-2/O4 被 #53 反转，指向本设计文档），或至少在地图 issue #1 决策日志注明。 |
| F6 | 建议 | 频控窗口对**已 dismiss 的举报**同样计数（§6.4 未明示）。用户在短窗口内连续举报多个真实违规会被 429 误伤。 | MVP 接受即可，但应在 §7 记为已知精度；如需收紧再按「排除 dismissed」或「分目标组」演进。 |

---

## 六、通过条件清单（执行前勾选）

- [ ] **F1**：`moderation/README.md` 同步（范围/Ordering/错误表 `ErrRateLimited`/新增不变量）
- [ ] **F2**：§5.2 明示 `report_handled` 的 `target_title` 语义复用；写明 `TargetType`/`TargetID` 是否下发
- [ ] **F3**：§8 测试描述修正（warn 累计语义、频控测试需第 6 个独立目标）
- [ ] **F4**：§7 补记「作者先自删 + 后处理」边角
- [ ] **F5**：issue-33 决策文件追加 D33-2/O4 反转注记
- [ ] **F6**：§7 记明频控窗口含 dismissed 的已知精度
- [ ] **Q4 附带**（可选）：前端 `report_handled` 渲染系统默认头像替代「?」（纯渲染层，不改后端）
- [ ] 常规：三处 `notify` 注释 + `types.ts` + `Notifications.vue:3` 注释同步；`git checkout -- frontend/components.d.ts`

---

## 七、结语

这是本项目事实纪律最扎实的一份设计文档：近 30 项 `file:line` 引用全中，服务器与线上 DB 实查可独立复现，最难核实的迁移约束名直达决策。所有关键设计裁决（快照作者、新通知类型、频控软限）都有充分依据，无阻塞项。唯一需要动手前补的是 F1 的 moderation README 同步——恰好是文档最在意的那类「文档漂移」，属于自己能举的例证漏在了自己身上。按 §六 勾选清单执行，本设计可直接作为落地基线。

—— 评审方（独立复核：本机仓库 @ 0301601 + 服务器/线上 DB 实查复现，2026-08-24）
