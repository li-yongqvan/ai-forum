# Package notify — 通知/私信域深模块接口（Interface README）

> 依据：#4 服务候选边界、#5 D5（notify 零依赖）、#7 深模块规范、IA v2 §5.4、#59 grilling 决策。
> **状态：已实现（service + gorm_repo 完整）。** 本文件是调用方契约，改动接口必须同步。

## 范围

对外提供 `Service` 接口。**notify 是依赖图叶子**：只被其他包调用，自身从不读别的表。

## 不变量（Invariants）

- **通知快照自足**：`CreateNotificationCmd` 中的显示名/头像/标题等快照值由**调用方**（content/user/moderation）构造时算好传入；notify 收下即存，不 join、不补查（#4 D4 硬约束）。
- **`actor_id`/`target_id` 仅作客户端跳转锚点**，服务端不做外键校验。
- **私信不进通知表**：`messages` 自带 `is_read`；私信与会话首页靠 to_user_id 查询（IA v2 §5.4：id=对方用户 id，首次发起自动建会话）。
- **归属隔离**：读/已读/未读全部按 `recipient_id`/`to_user_id` 过滤；操作他人通知/私信返回 `ErrNotFound`。
- **`is_read` 是接收方单边语义**：发送方看不到自己的消息是否被读，无双向回执（MVP 接受）。
- **底部 badge 合并语义**：`/notifications/unread_count` 返回「通知未读 + 私信未读」之和（#59 D2）。

## 调用顺序约束（Ordering）

- `CreateNotification` 可由任何调用方在任何时机调用；无前置顺序。
- `MarkAllRead` 仅作用于指定 recipient 的通知（不影响私信未读）。
- `SendMessage` 不校验对端是否存在；对端画像由 handler 层经 `PeerProvider` 解析（notify 零依赖）。

## 错误模式（Errors）

| 哨兵错误 | 语义 | 建议 HTTP | 产生位置 |
|---|---|---|---|
| `ErrNotFound` | 通知/私信不存在或不属于该用户 | 404 | notify repo |
| `ErrInvalidType` | `Type` 不在 6 类白名单（follow/like/comment/reply/report_result/report_handled） | 400 | notify service |
| `ErrEmptyMessage` | 私信内容 trim 后为空 | 400 | notify service |
| `ErrSelfMessage` | 不能给自己发私信 | 400 | notify service |
| `ErrMessageTooLong` | 私信内容超过 2000 rune | 400 | notify service |
| `ErrRateLimited` | 同一对端 1 秒内重复发送 | 429 | notify service |
| `ErrPeerNotFound` | 接收用户不存在（由 `PeerProvider` 转换 `user.ErrNotFound`） | 404 | httpapi adapter |

> 其余 error 为基础设施故障，调用方按 500 处理。**当前生成点为同步 best-effort**：调用方在成功动作后同步写通知、吞错不回滚主流程（如 follow 成功后通知失败不影响关注结果）；future 若积压可改进程内 goroutine 异步（#8：无 Redis，进程内）。

## 必需配置（Required config）

- `Repo` 依赖注入；真实实现需 schema `notify` 已迁移。
- `Service` 可选配合 `PeerProvider` seam 使用（#59 私信对端画像）。

## 性能特征（Performance）

- 写为单行 INSERT；读为 recipient 维度索引查询（`idx_notifications_recipient`、`idx_messages_from`、`idx_messages_to`）。MVP 无积压风险；若队列积压触发升级条件再引消息队列（#8 技术债表）。
- `ListConversations` 使用单条聚合 Raw SQL（Postgres `CASE` + `FILTER`），避免 N+1。

## 私信 API 契约（#59）

- `GET /api/v1/conversations` — 会话列表（对端聚合，新→旧）。
- `GET /api/v1/conversations/:peerID/messages` — 双方往来消息（新→旧）。
- `POST /api/v1/messages` — 发送私信；body `{ to_user_id, content }`；返回落库 `MessageView`。
- `POST /api/v1/conversations/:peerID/read` — 标记该对端会话全部已读（幂等）。
- `/notifications/unread_count` 合并返回通知+私信未读数。
