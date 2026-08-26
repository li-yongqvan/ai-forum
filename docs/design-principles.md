# 设计原则 · 深模块（Draft #7）

> 落地：#7 grilling 决策，2026-08-17。权威语料：`codebase-design` skill 与 Ousterhout《A Philosophy of Software Design》。范围：ai-forum 全栈（Go 后端 + Vue PWA）。
> 本文件是草案，随项目演进由后续 ticket 修订。

## 1. 术语表（通用语言 · 必须）

| 术语 | 定义 | 禁用词 |
|---|---|---|
| **Module** | 有接口与实现的东西，尺度无关：函数、类、包、跨层切片 | component / service / unit |
| **Interface** | 调用者必须知道的一切：签名 + 不变量、顺序约束、错误模式、必需配置、性能特征 | API（仅指类型面，太窄） |
| **Implementation** | 模块内部的代码主体 | — |
| **Adapter** | 在 seam 上满足某接口的具体物；描述角色而非实质 | — |
| **Depth** | 接口小、实现厚（复杂）：调用者每学习一单位接口，能撬动大量行为。深 = 简单接口 + 复杂实现；浅 = 接口 ≈ 实现复杂度（薄转发） | — |
| **Seam** | 可在不改该处代码的前提下改变行为的位置（Feathers） | boundary（DDD 语境超载） |
| **Leverage / Locality** | 深度带给调用者的杠杆 / 带给维护者的局部性：一份实现偿还 N 个调用方 + M 个测试；变更与缺陷集中一处 | — |

**反义词禁令（设计语境）**：在接口/设计讨论、审查、README、注释中，禁止用 component / service / API / boundary 指代设计单元。
service（部署单元）、API（传输协议名词）、boundary（DDD 有界上下文）在各自领域语义下不受影响。

## 2. 原则（必须）

1. **深度是接口的性质，不是实现的。** 深的模块内部可由小的、可 mock、可替换的部件组成——它们不属于接口。模块可有内部 seam（实现私有，供自身测试）与外部 seam（接口所在）。**实现复杂指接口后承载的行为量必须大（实现厚），不是内部代码允许混乱。**
2. **删除测试。** 删除该模块：复杂度随之消失 → pass-through（浅）；复杂度散布回 N 个调用方 → 它在赚位置（深）。
3. **接口即测试面。** 调用方与测试跨同一个 seam。若想测到接口之外，模块形状多半不对。
4. **一个 Adapter = 假设性 seam；两个 Adapter = 真实的。** 无真实变化就不引入 seam。
5. **可测试性三则。** 接受依赖而非创建；返回结果而非副作用；小接口面（少方法、简参数）。

## 3. Interface README（必须）

- **范围**：有外部调用方的模块 —— 组合 A 的公开包、组合 B 的 gRPC 服务。单一入口 / 无外部调用方的包不强制。
- **必填**：不变量、调用顺序约束、错误模式、必需配置、性能特征。**不写签名**。
- **分工**：README 管「调用者必须知道的事实」；Godoc 管签名与逐方法语义；两者不重复。

## 4. 审查检查清单（code-review 默认维度 · 必须对照）

- **接口三问**：能否减少方法？能否简化参数？能否把更多复杂度藏进实现？
- **删除测试**：删掉它，复杂度消失还是散布回调用方？
- **单 Adapter 警示**：只有一个具体实现的抽象接口 → 标记「假设性 seam，真的需要吗？」

## 5. 深模块 × 服务拆分

- **包边界 = 服务候选边界**：组合 A 中未来可能独立成服务的包，按深接口设计 —— 接口形状决定拆分难易。
- **组合 B 服务暴露粗粒度深操作**：如 `CreatePost` 而非 `CheckPermission → InsertPost → NotifyFollowers` 三个细调用。分支 / 事务 / 校验藏进服务实现，别让 BFF 承载业务流程。
- **MVP 不建服务间抽象层**：单一实现时 seam 是假设性的。包接口深 + 搬运（包 → 服务）即可完成拆分。

## 6. 强制级别

- **必须**：术语与反义词禁令（设计讨论 / 审查 / README / 注释）；Interface README（有外部调用方时）；审查检查清单。
- **建议**：README 中性能特征的完备性；内部 seam 的组织方式；README 篇幅上限。

## 7. 数据模型：包内物理 FK / 跨包逻辑外键（#5）

PostgreSQL schema 按业务域划分（`user`、`content`、`notify`、`moderation` 等）。外键策略：

- **包内物理 FK**：同一 schema 内的引用必须建 `FOREIGN KEY` 约束，由数据库保证引用完整性。
- **跨包逻辑外键**：跨 schema 的引用只建索引、不加约束，避免 schema 间循环依赖和强耦合；由业务代码与软删策略保证一致。

**理由**：`users` 表软删永不硬删，跨包逻辑外键不会失联；物理 FK 限制在同 schema 内，既享受数据库约束，又保留包边界独立演化的空间。

### 7.1 包内物理 FK 清单（截至 0010）

| schema | 表 | 列 | 指向 |
|---|---|---|---|
| `user` | `follows_users` | `follower_id` | `user.users(id)` |
| `user` | `follows_users` | `target_id` | `user.users(id)` |
| `user` | `invitation_codes` | `created_by` | `user.users(id)` |
| `user` | `invitation_codes` | `used_by` | `user.users(id)` |

### 7.2 跨包逻辑外键清单（保持索引，不加约束）

- `content`：`posts.author_id`、`comments.author_id`、`likes.user_id`、`favorites.user_id`。
- `user`：`follows_boards.follower_id`、`follows_topics.follower_id`。
- `notify`：`notifications.recipient_id`、`messages.from_user_id`、`messages.to_user_id`。
- `moderation`：`reports.reporter_id`、`reports.handler_id`、`moderation_actions.moderator_id`。

新增跨包引用时，遵循同一规则：建索引、写代码保证、不建 `FOREIGN KEY`。
