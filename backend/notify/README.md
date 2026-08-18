# Package notify — 通知/私信域深模块接口（Interface README）

> 依据：#4 服务候选边界、#5 D5（notify 零依赖）、#7 深模块规范、IA v2 §5.4。
> **状态：接口骨架。** 实现（service/gorm_repo）留待后续 ticket。

## 范围

对外提供 `Service` 接口。**notify 是依赖图叶子**：只被其他包调用，自身从不读别的表。

## 不变量（Invariants）

- **通知快照自足**：`CreateNotificationCmd` 中的显示名/头像/标题等快照值由**调用方**（content/user/moderation）构造时算好传入；notify 收下即存，不 join、不补查（#4 D4 硬约束）。
- **`actor_id`/`target_id` 仅作客户端跳转锚点**，服务端不做外键校验。
- **私信不进通知表**：`messages` 自带 `is_read`；私信与会话首页靠 to_user_id 查询（IA v2 §5.4：id=对方用户 id，首次发起自动建会话）。

## 调用顺序约束（Ordering）

- `CreateNotification` 可由任何调用方在任何时机调用；无前置顺序。
- `MarkAllRead` 仅作用于指定 recipient。

## 错误模式（Errors）

| 哨兵错误 | 语义 | 建议 HTTP |
|---|---|---|
| `ErrNotFound` | 通知/私信不存在或不属于该用户 | 404 |

> 其余 error 为基础设施故障，调用方按 500 处理。通知异步写入失败采用「尽力而为 + 记日志」策略（#8：无 Redis，进程内 goroutine 异步），不阻塞主流程。

## 必需配置（Required config）

- `Repo` 依赖注入；真实实现需 schema `notify` 已迁移。

## 性能特征（Performance）

- 写为单行 INSERT；读为 recipient 维度索引查询（`idx_notifications_recipient`）。MVP 无积压风险；若队列积压触发升级条件再引消息队列（#8 技术债表）。
