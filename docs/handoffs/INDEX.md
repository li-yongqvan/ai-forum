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

| # | Ticket | 类型 | 阻塞 | Handoff 文档 | 状态 |
|---|---|---|---|---|---|
| #5 | [核心数据模型设计](https://github.com/li-yongqvan/ai-forum/issues/5) | grilling | 已解锁（#4 已解决） | 待建 | frontier |
| #8 | [部署与运维方案](https://github.com/li-yongqvan/ai-forum/issues/8) | grilling | 已解锁（#4 已解决） | 待建 | frontier |
| #6 | [移动端首页与帖子详情原型](https://github.com/li-yongqvan/ai-forum/issues/6) | prototype | 被 #5 阻塞 | 待建 | 阻塞 |

## 已解决

| # | Ticket | 解决时间 | 结论摘要 |
|---|---|---|---|
| #2 | [Go 后端框架与微服务框架选型调研](https://github.com/li-yongqvan/ai-forum/issues/2) | 2026-08-17 | 组合 A（Gin 模块化单体 + GORM + Compose/Caddy）主推；组合 B（BFF + 2~3 gRPC 服务）学微服务。详见 `docs/research/go-backend-framework-selection.md` |
| #3 | [移动端前端技术方案调研](https://github.com/li-yongqvan/ai-forum/issues/3) | 2026-08-17 | Vue 3 + Vite + Vant PWA，REST(OpenAPI) ↔ Go，gRPC 仅服务间。详见 `docs/research/mobile-frontend-selection.md` |
| #7 | [深模块设计规范落地形式](https://github.com/li-yongqvan/ai-forum/issues/7) | 2026-08-17 | 采纳 codebase-design 词表 + 设计语境反义词禁令；Interface README 覆盖有外部调用方的模块；检查清单并入 code-review；包边界 = 服务候选边界，MVP 不建抽象层。详见 `docs/design-principles.md` |
| #4 | [MVP 服务拆分与模块边界](https://github.com/li-yongqvan/ai-forum/issues/4) | 2026-08-17 | 组合 A 模块化单体 + 4 包（user/content/notify/moderation）+ 粗粒度命令/读模型接口 + 进程内 goroutine 异步 + notify 叶子快照化 + 单进程多 schema 部署。决议见 issue #4 comment |

## 约定

- 新建 ticket 时，同步在「活跃 ticket」表加一行，文档标记「待建」；真正要处理时才写全文（避免空文件噪音）。
- ticket 关闭后从「活跃」移到「已解决」，在结论摘要留一行指向研究成果。
- 本索引不重复 GitHub issue 内容，只做指针与状态汇总。
