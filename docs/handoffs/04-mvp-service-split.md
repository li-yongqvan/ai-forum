# Handoff · #4 MVP 服务拆分与模块边界

> **状态：已解决（2026-08-17）** — 本票 grilling 完成，D1–D5 决议见 [#4 resolution comment](https://github.com/li-yongqvan/ai-forum/issues/4#issuecomment-5316938952) 与 grilling 决策记录；地图 #1 已同步。
>
> 按 Matt Pocock handoff 5 段式结构编写。焦点：**design/plan**。

## Goal of next session

通过 grilling 决策 AI 智联论坛 MVP 的服务/模块边界，产出结论并记录到 #4 的 resolution comment（不要求产出独立文档；结论直接进地图 Decisions so far）。本决策**解锁 #5 数据模型**与 **#8 部署方案**，是后端后续一切的上游。

Prompts to answer（逐条，用 /grilling）：
- MVP 用组合 A（模块化单体）还是组合 B（BFF + 2~3 gRPC 服务）？是否 MVP 即上微服务？
- 按领域拆成几个模块/包？各自职责是什么？
- 每个模块的对外接口长什么样（深接口：小接口大实现）？
- 模块间通信方式（进程内 / HTTP / gRPC / 消息队列）？
- 哪些放同一进程与仓库、哪些必须独立部署？

## State of play

**Done:**
- #2 后端选型 → `docs/research/go-backend-framework-selection.md`（组合 A：Gin 模块化单体 + GORM + PostgreSQL 单库多 schema + Compose DNS + Caddy；组合 B：Gin BFF + 2~3 gRPC 服务）
- #7 深模块规范 → `docs/design-principles.md`（**§5「深模块 × 服务拆分」与 #4 直接相关**：包边界=服务候选边界；组合 B 服务暴露粗粒度深操作如 `CreatePost`；MVP 不建服务间抽象层）
- 地图 #1 已同步（#2/#3/#7 进 Decisions so far，frontier 仅剩 #4）
- 本 handoff 文档已建立

**In progress:**
- 无（#4 未认领）

**Blocking:**
- #4 无阻塞（#2 已解决即解锁）。**#4 阻塞 #5（数据模型）与 #8（部署方案）**——本 ticket 的形态决策决定它们的输入

## Open decisions

- **D1 架构形态：组合 A 还是组合 B？**（#4 的核心张力）
  - *选项*：A 模块化单体（单进程、包内拆边界，最快交付）；B BFF + 2~3 gRPC 服务（亲历微服务，教学价值高）；先 A 后 B（MVP 单体、预留服务候选边界，日后平滑拆分）
  - *Lean*：**组合 A**，与 #2 主推一致，且 #7 已定「MVP 不建服务间抽象层」；但用户学习导向允许为教学选 B——若社团有「微服务是考核目标」的硬要求则选 B，且严格控制服务数 ≤3
  - *Deps*：#2 组合结论、#7 design-principles §5
- **D2 模块/包划分**：按领域拆哪几个包？每个模块的职责边界（MVP 功能 → 模块映射）
  - *候选*：`user`（注册/登录/JWT/关注/封禁/角色）、`content`（帖子/评论/点赞/板块/话题）、`notify`（通知/私信 DM）、`moderation`（举报/治理）；或合并为 user/content/notify 三包（治理并入）
  - *Lean*：**4 包**（user / content / notify / moderation），因为治理前置是项目硬约束，独立包利于审计与封禁语义
  - *Deps*：D1；MVP 功能集（地图 Notes 已列）
- **D3 每模块的对外接口（深接口）**：公开接口暴露什么操作？如何保证「小接口大实现」？
  - *Lean*：每模块暴露粗粒度深操作（如 `CreatePost(ctx, cmd)` 而非 `CheckPermission→InsertPost→NotifyFollowers` 三连）；接口遵守 design-principles §3 Interface README 必填项
  - *Deps*：#7 §5、D1
- **D4 通信方式**：模块间/服务间如何通信？
  - *选项*：进程内直接调用（A）；gRPC（B 服务间）；是否引入异步/消息队列处理通知
  - *Lean*：MVP **进程内调用 + goroutine 异步通知**，不引入消息队列；若选 B，服务间 gRPC、BFF↔前端 REST（#3 已定）
  - *Deps*：#2、#3、D1
- **D5 部署边界**：单进程/单仓库部署，还是多服务独立部署？哪些「必须」独立？
  - *Lean*：MVP 单进程 + 单仓库 + Docker Compose 单服务；仅在需要独立扩缩容/独立发布/独立技术栈时才拆服务（MonolithFirst）
  - *Deps*：#8（部署 ticket 被本票阻塞，先定边界再细化）、D1

## Skills to use (next session)

- `/grilling` — 逐条决策 D1–D5（HITL，真人参与）
- `/codebase-design` — 深接口设计、seam/Adapter 语料（本地 `~/.claude/skills/codebase-design/SKILL.md`）
- `/domain-modeling` — 领域边界与有界上下文（D2/D3 核心）
- `/research` — 如遇需查证的通信/演进模式（可选，多数决策依赖本地语料）

## Artifacts (reference only — do NOT duplicate)

- **Issue:** [#4 MVP 服务拆分与模块边界](https://github.com/li-yongqvan/ai-forum/issues/4)；地图 [#1](https://github.com/li-yongqvan/ai-forum/issues/1)；上游已解决：[#2](https://github.com/li-yongqvan/ai-forum/issues/2)、[#7](https://github.com/li-yongqvan/ai-forum/issues/7)
- **Design principles:** `docs/design-principles.md`（#7 产物，§5 直接相关）
- **Research:** `docs/research/go-backend-framework-selection.md`（#2，组合 A/B 架构图）；`docs/research/mobile-frontend-selection.md`（#3，通信已定 REST↔Go）
- **Input doc:** `docs/forum-wayfinder-input.md`（MVP 功能与约束）
- **This handoff:** `docs/handoffs/04-mvp-service-split.md`
- **分支/提交:** 无（仓库尚未初始化代码）
