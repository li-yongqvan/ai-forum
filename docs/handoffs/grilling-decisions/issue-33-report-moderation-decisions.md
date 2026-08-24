# Grilling #33 举报与处理闭环 — 决策记录

- **Ticket**: [#33 举报与处理闭环：补齐举报 API、管理队列与结果通知](https://github.com/li-yongqvan/ai-forum/issues/33)
- **类型**: grilling / HITL + 独立评审裁决吸收
- **日期**: 2026-08-20（grilling）· 2026-08-21（评审裁决，随本票 PR 入库）
- **参与**: Claude（#33 实现会话）+ @li-yongqvan + 独立评审方
- **产出**: 本票实现范围定稿；评审意见书裁决全部落地（见 §3）

---

## 目标

完成 #33 第 0 步 grilling（SOP §3），锁定举报闭环范围与语义，再据此实现；并经独立评审（意见书 2026-08-21）吸收裁决。

---

## 1. 用户定案（grilling 2026-08-20）

### D1 处理动作集 = 忽略 + 删帖/删评论 + 警告；封禁留 #34

- **问题**: 本票实现哪些处理动作？
- **选项**: A 忽略+删除+警告；B 只忽略+删除；C 全 IA 动作集含封禁。
- **决定**: **A**。`dismiss`(忽略) + `delete_post/delete_comment`(删除内容) + `warn`(警告)；`ban_user` 服务端拒绝（`validAction` 拦截），留 #34 封禁闭环。
- **理由**: IA §5.0「moderator 动作集不含封禁」；handoff §3 建议与 #34 划清；`user.Ban` 虽已实现但本期范围从紧。

### D2 结果通知 `report_result` 只发举报人

- **问题**: 处理结论通知给谁？（IA §5.6 写「通知给举报人」；handoff 初拟「被举报人」。）
- **决定**: **只通知举报人**（含结论句）。
- **理由**: 以 IA §5.6 原文为准（设计权威）；前端模板「举报结果：${target_title}」即举报人视角。被举报人触达本期不做（评审 O3 认可，未来补无需迁移，届时需回本决策重裁）。

### D3 前端只做帖子+评论举报入口

- **问题**: 是否本期加「举报用户」入口（用户主页）？
- **决定**: **本期不做**。API 仍支持 `target_type=user`（schema 已允许、服务端能力完整），前端入口留后续。
- **理由**: handoff §1 范围只写「发帖/评论内嵌入口」；用户举报与封禁联动留 #34。属对 IA §2 D4/§5.0 的**范围偏离**，合并后需在 IA/地图批注（SOP §10）。

## 2. IA/schema 既定（非开放，照做）

- **D4 举报原因枚举**：垃圾广告/违法违规/人身攻击/抄袭侵权/引战灌水/其他（选「其他」备注必填）。schema `reason TEXT` 无 CHECK → 应用层白名单（`validReasons`）。
- **D5 状态机**：`pending → resolved/dismissed`（schema CHECK + `uq_reports_pending` 防重复、结案可再举报）。
- **D6 处理人权限**：moderator 动作集不含封禁、admin 全动作集；前后端双重鉴权（`RequireRole` + 服务端按 `OperatorRole` 再校验）。

## 3. 独立评审裁决吸收（意见书 2026-08-21，有条件通过）

### 3.1 否决项
- **O2/S5 举报人备注存储——否决合成方案**，裁定新增迁移 `0006_reports_reporter_note.sql` 加 `reporter_note TEXT` 列，`reason` 只存枚举。理由：合成破坏枚举语义（筛选/统计/分组失效）、解析契约脆弱、「不新增迁移」是偏好非禁令。**编号修正**：意见书写的 `0005` 被 `content_seed` 占用，实为 **0006**。

### 3.2 阻塞项修复
- **F1 证据入库**：本文件随 #33 PR 入库，成为 D1/D2/D3 定案的可溯源记录（handoff `33-report-closure.md` 保持本地底稿，团队约定 `c23c1b3` 有意 untracked）。
- **F2 dismiss 审计三方冲突披露**：IA §5.6 字面「所有动作→append 审计」 vs README「dismiss 不落审计」 vs schema 枚举无 `dismiss`。**裁决**：采 README；dismiss 写 `reports.handler_id/handled_at/handling_note` 留痕（`UpdateReportStatus` 已承载）。合并后更新 IA 批注。

### 3.3 执行前钉死项（F3-F8、O1、S3/S7/S9 附带）
- **F3** `HandleReportCmd` 增 `OperatorRole`（JWT claims 注入）。
- **F4** `UpdateReportStatus` 条件更新 `WHERE status='pending'`，`RowsAffected=0` → 409/ErrNotFound（并发双处理防护）。
- **F5** `HandleReportCmd.Note` ≤500（`moderation_actions.reason VARCHAR(500)`），服务端校验 + 前端限长。
- **F6** Service 接口补 `ListReports/CountReports`（队列按 status 过滤）。
- **F7** 创建侧策略：自举报拒绝（service 一条判断）；对已删目标举报由存在性校验自然 404；频控 MVP 不做（记入已知缺口）。
- **O1 事务边界**：网关副作用在事务外执行（`ResolveTarget→删除→开事务→提交后通知`）；删除动作不先 ResolveTarget（保证已删目标可幂等重处理），adapter 把「不存在/已删」当成功，**显式接受**「作者先自删时审计归属处理人」的 MVP 精度；adapter 以处理人身份调用删除。
- **S3** 通知失败打日志（`slog.Warn`）不回滚（不复制 follow.go 无日志瑕疵）。
- **S7** 补全 API 面表（含创建端点 `POST /api/v1/reports`）。
- **S9** 23505 判定同时匹配 `ConstraintName == "uq_reports_pending"`。
- **O4** warn UI 写明「警告仅留痕，对方不会收到通知」；决策记录写明 #34 前瞻（`CountActionsByTarget` 聚合）。
- **O5** 队列 `pageSize` 服务端钳制 ≤100。
- **O3** 被举报人触达本期不做，可接受（路径提示已记录于 §3.1 D2）。
- **F8** `target_title` 结论句限长（VARCHAR(255) 内）；队列 enrich N+1 已记录；创建返回 201+举报 id；本意见书随决策记录归档。

---

## 4. 已知缺口（F7 显式记录，MVP 不做）

- 举报频控/滥用防护（`uq_reports_pending` 天然限「同举报者同目标 pending 期一条」）。
- 用户举报前端入口（API 支持，前端留 #34）。
- 被举报人触达（见 D2/O3）。
- warn 仅留痕、无用户侧状态变更（无 warn 状态字段）。

## 5. 后续行动

- [x] 后端：迁移 0006 + content 只读方法（S4）+ moderation 域（service/gorm_repo/handler/httpapi 装配）+ 单测/集成测试全绿
- [ ] 前端：api/report.ts + ReportSheet + PostList/PostDetail/CommentTree 接线 + Reports 队列页 + 路由 + Me 入口徽章 + vitest
- [ ] 自审 `/code-review` → PR（基 main）→ 用户授权合并 → CI 部署 → 服务器验证 → **回报统筹方更新地图 #1（本会话不自行 edit issue #1）**

---

## 6. 修订注记（#53，2026-08-24）

> **本文件 D2（§1）与 O4（§3.3）被 issue #53 反转**，以 #53 设计文档为准：
> `docs/plans/issue-53-report-followup-设计文档.md`（评审：有条件通过 0 阻塞，意见书同目录）。
>
> - **D2「report_result 只发举报人」→ 反转**：#53 D1 改为**举报人 + 被举报人都发**（被举报人收 `report_handled`，仅 delete_post/delete_comment/warn）。
> - **O4「warn 仅留痕、对方不会收到通知」→ 反转**：#53 D2 中 **warn 也通知被举报人**；前端 `Reports.vue` 警告按钮文案同步改为「警告（会通知被举报人）」。
> - 其余 D1/D3-D6 与 §4 缺口清单不变（举报频控作为 #53 新决策补入，见 #53 设计文档 §3 D3/D4）。
