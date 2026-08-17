# Handoff · #7 深模块设计规范落地形式

> ✅ **已解决 2026-08-17** — Q1–Q5 已决策，草案产出：`docs/design-principles.md`。本节下方保留决策过程作历史记录。

> 按 Matt Pocock handoff 5 段式结构编写（Goal / State of play / Open decisions / Skills / Artifacts）。焦点：**design/plan**。

## Goal of next session

通过 grilling 决策「深模块（Deep Module）设计哲学如何落地为 ai-forum 团队规范」，产出简洁的 `docs/design-principles.md` 草案，并按 wayfinder 规则 resolve #7（resolution comment → 关闭 → 更新地图 #1 的 Decisions so far 与 frontier）。

Prompts to answer（逐条，用 /grilling）：
- 是否把 codebase-design 词汇（Module/Interface/Seam/Adapter/Depth/Locality/Leverage）定为项目通用术语，并执行反义词禁令（component/service/API/boundary）？
- 代码审查中用什么具体方法检查接口深度？
- 哪些层级的模块必须提供 README 说明其 Interface（调用者必须知道的事实）？
- 深模块原则与组合 A/B 微服务拆分如何配合？
- `docs/design-principles.md` 的形态（篇幅、结构、强制级别）？

## State of play

**Done:**
- 地图 #1 已建立，Notes 已声明「采用深模块设计哲学（Ousterhout）」并指向 codebase-design skill
- #2 后端选型已解决 → `docs/research/go-backend-framework-selection.md`（组合 A：Gin 模块化单体 / 组合 B：BFF + 2~3 gRPC 服务）
- #3 前端选型已解决 → `docs/research/mobile-frontend-selection.md`（Vue3+Vite+Vant PWA，REST ↔ Go）
- 本 handoff 文档 + `docs/handoffs/INDEX.md` 已建立

**In progress:**
- 无（#7 未认领，无 assignee）

**Blocking:**
- #7 无阻塞（并行 ticket）。注意 #4「MVP 服务拆分与模块边界」已解锁，但其决策独立于 #7；若先做 #4 会简化 Q4 的答案，但不应阻塞本 ticket

## Open decisions

- **Q1 词汇体系**：采纳 codebase-design 词表为通用语言？反义词禁令强制范围（PR 审查/注释/README）？落地载体（CLAUDE.md 一节 vs 仅 design-principles.md）？
  - *Lean*：采纳词表 + 反义词禁令，落地到 design-principles.md 并在地图 Notes 引用；暂不改 CLAUDE.md
  - *Deps*：无
- **Q2 深度检查**：审查时用什么方法判断「深」？（删除测试、接口是否薄、参数可否简化）
  - *Lean*：把 codebase-design 的「接口自问三问 + 删除测试」沉淀为 code-review 的默认检查项；不单独建工具
  - *Deps*：code-review skill（本地已有）
- **Q3 README 要求**：哪些层级必须写 Interface README？必填项是什么（不变量/错误模式/配置/性能，而非签名）？
  - *Lean*：仅要求「服务级（组合 B）/ 公开包级（组合 A）」提供 Interface README，模板来自 codebase-design 的 Interface 定义；代码注释（Godoc）不重复 README
  - *Deps*：Q1 词汇统一
- **Q4 深模块 × 微服务**：组合 A 的包边界 = 未来服务边界如何保证接口正确？组合 B 的服务接口如何用深模块取舍？「一个 Adapter = 假设性 seam」在 2~3 服务下意味着什么？
  - *Lean*：包边界即服务候选边界，按「深接口优先」设计；MVP 不引入服务间抽象层（seam 是假设性的）
  - *Deps*：#2 组合选择（已定）
- **Q5 草案形态**：篇幅、结构（术语表/原则/检查清单/README 要求/微服务配合）、强制级别
  - *Lean*：≤2 页；术语表 + 3~5 条原则 + 检查清单；「必须」与「建议」分层
  - *Deps*：Q1–Q4 结果

## Skills to use (next session)

- `/grilling` — 逐条决策 Q1–Q5（HITL，必须真人参与，不得自问自答）
- `/codebase-design` — 词汇与原则的唯一权威语料（本地：`~/.claude/skills/codebase-design/SKILL.md`，含 DEEPENING.md / DESIGN-IT-TWICE.md）
- `/domain-modeling` — 若 Q4 涉及领域边界讨论
- `/code-review` — Q2 检查项的落地载体参考

## Artifacts (reference only — do NOT duplicate)

- **Issue:** [#7 深模块设计规范落地形式](https://github.com/li-yongqvan/ai-forum/issues/7)；地图 [#1](https://github.com/li-yongqvan/ai-forum/issues/1)
- **Design principles 语料:** `~/.claude/skills/codebase-design/SKILL.md`（词表 + 深/浅模块图 + 拒绝的框架）
- **Research:** `docs/research/go-backend-framework-selection.md`（#2，组合 A/B）；`docs/research/mobile-frontend-selection.md`（#3）
- **Input doc:** `docs/forum-wayfinder-input.md`（项目最终形态与约束）
- **This handoff:** `docs/handoffs/07-deep-module-design.md`
- **分支/提交:** 无（仓库尚未初始化代码）
