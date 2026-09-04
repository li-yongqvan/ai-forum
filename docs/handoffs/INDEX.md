# Handoff 索引 · ai-forum Wayfinder Tickets

## 机制

每个 wayfinder ticket 在 `docs/handoffs/` 下有一份 `<编号>-<slug>.md` 的 handoff 文档，供**新会话**接手时直接读取。决策不依赖会话记忆，只依赖本文件 + GitHub issue。

**文档格式**：遵循 Matt Pocock 技能的 **handoff 5 段式结构**（[handoff_structure.md](https://raw.githubusercontent.com/alirezarezvani/claude-skills/refs/heads/main/engineering/handoff/skills/handoff/references/handoff_structure.md)）：
1. **Goal of next session** — 下一会话产出目标 + Prompts to answer（最重要）
2. **State of play** — Done / In progress / Blocking，具体到路径
3. **Open decisions** — 必须做的决策，每项含选项 + 当前倾向（lean）+ 依赖
4. **Skills to use (next session)** — 具体命名的技能
5. **Artifacts (reference only)** — 路径 + URL，只引用不重复

目标篇幅 ~50-100 行；不重复 PRD/issue/research 内容，只链接。反模式：handoff 比底层文档还长、无 artifact 引用、决策无选项/倾向、缺下一会话目标、路径过期。

**使用流程**：
1. 新会话开头读 `docs/handoffs/INDEX.md` 找到目标 ticket 的文档。
2. 读对应 `<编号>-<slug>.md`，按其 Goal 与 Open decisions 执行。
3. HITL ticket（grilling）决策在新会话完成；research 可后台 subagent 完成。
4. 完成后再按 wayfinder 规则 update 地图（#1）与关闭 ticket，并把该 ticket 的 handoff 文档标记为「已解决」。

## 活跃 ticket

> **#23 收藏/关注列表**：`docs/handoffs/23-favorites-list.md`（接通「我的」页收藏/关注 tab；范围 = 收藏 + 关注，旧底稿「仅收藏/拆 #27」已过时——#27 实为 SOP 落地 PR，不存在关注列表独立 ticket，以 ticket 正文与地图为准）。
> **#33 举报与处理闭环**：`docs/handoffs/33-report-closure.md`（举报 API + 管理队列 + `report_result` 通知 + 前端入口；依赖通知中心**已满足**；先做第 0 步 grilling 5 决策）。
> **前端单测进 CI（无 ticket，地图 Roadmap 第 2 项）**：`docs/handoffs/frontend-unit-tests-ci.md`（deploy.yml test job 挂 `npm run test:unit`，`continue-on-error` 过渡转阻塞）。
> **#76 9/13 一次性免码注册**：grilling 已定稿，决策记录见 [issue-76-temp-open-registration-decisions.md](grilling-decisions/issue-76-temp-open-registration-decisions.md)；设计文档 + 评审意见书在 `docs/plans/issue-76-temp-open-registration-设计文档*.md`（独立评审**有条件通过**，B1 已闭合 2026-09-05）。实现 handoff 待建——执行会话直接以设计文档为基线。

## 已解决

| # | Ticket | 解决时间 | 结论摘要 |
|---|---|---|---|
| #14 | [前端产物自动部署进 CI（阻断项 2）](https://github.com/li-yongqvan/ai-forum/issues/14) | 2026-08-18 | deploy job 新增前端构建（Node 22 + npm ci + build，**始终**，作 main 构建验证）+ **scp-action 直传 `/var/www/ai-forum`**（`if: env.SSH_HOST != ''` 门控，`strip_components: 2`）；未配 secrets 时前端照常构建、上传/部署步跳过、run 不红；deployment.md §4 step 6 改 chown 前置、§5.1 同步真实 3-job CI。actionlint + 前端 build 校验通过，**PR #15 已合入 main（2026-08-18，CI 全绿）**；配 SSH secrets + 服务器 `chown /var/www/ai-forum` 后真实生效。另随 PR 补齐此前滞留未合 main 的 actionlint 基建。详见 [issue #14](https://github.com/li-yongqvan/ai-forum/issues/14) |
| #12 | [图片上传（MVP 本地上传）](https://github.com/li-yongqvan/ai-forum/issues/12) | 2026-08-18 | 方案 A 本地上传闭环：后端 `POST /api/v1/uploads`（登录态，双校验/UUID 文件名/5MB）+ 新包 `backend/upload`；前端发帖/评论图片选择器 + 光标插入 + md 渲染；生产 nginx 托管 `/uploads`、开发 Go 静态。IA §7 升级「本地上传+外链」。`go test`/Vitest/Playwright 冒烟全过。分支 `feat/image-upload`。决策见 [issue-12-image-upload-decisions.md](grilling-decisions/issue-12-image-upload-decisions.md)，详见 [issue #12 resolution](https://github.com/li-yongqvan/ai-forum/issues/12#issuecomment-5324989160) |
| #8 | [部署与运维方案](https://github.com/li-yongqvan/ai-forum/issues/8) | 2026-08-18 | 单服务器 Docker Compose all-in-one；复用现有 nginx（目标服务器已预装，作为 #2 Caddy 推荐的条件分支）；PostgreSQL 容器化；无 Redis/消息队列；GitHub Actions SSH 直连部署；宿主机 cron pg_dump 备份；stdout + Docker json-file 日志；`/healthz` + 外部 uptime 监控；MVP 纯 HTTP on IP。产出 `docs/ops/deployment.md`，详见 issue #8 resolution comment |

| # | Ticket | 解决时间 | 结论摘要 |
|---|---|---|---|
| #6 | [移动端首页与帖子详情原型](https://github.com/li-yongqvan/ai-forum/issues/6) | 2026-08-18 | 以 `docs/ux/ia-prototype.html` 为基底，独立产出聚焦原型 `docs/ux/home-post-prototype.html`；覆盖首页信息流（分段/排序/分页/空态）与帖子详情（Markdown/评论树折叠/软删/锚点高亮/管理操作），角色与主题可在演示控制台切换。决议见 `docs/ux/home-post-prototype-resolution.md` |
| #2 | [Go 后端框架与微服务框架选型调研](https://github.com/li-yongqvan/ai-forum/issues/2) | 2026-08-17 | 组合 A（Gin 模块化单体 + GORM + Compose/Caddy）主推；组合 B（BFF + 2~3 gRPC 服务）学微服务。详见 `docs/research/go-backend-framework-selection.md` |
| #3 | [移动端前端技术方案调研](https://github.com/li-yongqvan/ai-forum/issues/3) | 2026-08-17 | Vue 3 + Vite + Vant PWA，REST(OpenAPI) ↔ Go，gRPC 仅服务间。详见 `docs/research/mobile-frontend-selection.md` |
| #7 | [深模块设计规范落地形式](https://github.com/li-yongqvan/ai-forum/issues/7) | 2026-08-17 | 采纳 codebase-design 词表 + 设计语境反义词禁令；Interface README 覆盖有外部调用方的模块；检查清单并入 code-review；包边界 = 服务候选边界，MVP 不建抽象层。详见 `docs/design-principles.md` |
| #4 | [MVP 服务拆分与模块边界](https://github.com/li-yongqvan/ai-forum/issues/4) | 2026-08-17 | 组合 A 模块化单体 + 4 包（user/content/notify/moderation）+ 粗粒度命令/读模型接口 + 进程内 goroutine 异步 + notify 叶子快照化 + 单进程多 schema 部署。决议见 issue #4 comment |
| #5 | [核心数据模型设计](https://github.com/li-yongqvan/ai-forum/issues/5) | 2026-08-17 | 14 表 / 4 schema 数据模型定案（50 人规模）：BIGSERIAL 主键、包内物理FK/包间逻辑外键、邻接表+floor 树状评论、关注单向、通知快照化、治理 append-only 审计。决议见 [#5 resolution comment](https://github.com/li-yongqvan/ai-forum/issues/5#issuecomment-5317256635) |
| #9 | [前端页面清单与信息架构](https://github.com/li-yongqvan/ai-forum/issues/9) | 2026-08-18 | 15 路由页+弹窗层、4 Tab（首页/板块/通知/我的）+ 全站 FAB、两级扁平层级（History API 导航）、治理闭环（举报内嵌+权限矩阵+举报处理+report_result 通知）、登录墙；定稿裁决：取消搜索占位、置顶/精华纳入 MVP。详见 `docs/ux/info-architecture.md`（v2）+ `docs/ux/ia-prototype.html`（Kimi Agent 版原型，裁定采纳） |

## 约定

- 新建 ticket 时，同步在「活跃 ticket」表加一行，文档标记「待建」；真正要处理时才写全文（避免空文件噪音）。
- ticket 关闭后从「活跃」移到「已解决」，在结论摘要留一行指向研究成果。
- 本索引不重复 GitHub issue 内容，只做指针与状态汇总。
