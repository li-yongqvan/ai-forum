# ai-forum #59 私信实现 — 关键设计决策（grilling decisions）

> 决策时间：2026-08-25  
> 问题域：ai-forum 私信（direct messages）功能实现  
> 适用范围：#59 实现者、#60/#61 相关同学、后续接手的维护者

## 1. 会话形态：按对端聚合（Conversation per peer）

**决策**：前端私信 tab 展示「会话列表」，而不是「我收到的全部消息列表」；每个会话 = 一个对端，会话内展示双方往来消息。

**理由**：
- 用户体验上，私信天然是 1:1 聊天；列表按对端聚合最贴近用户心智模型。
- 数据模型 `notify.messages` 有 `from_user_id`/`to_user_id`，按对端分组做聚合查询即可复用现有表，零迁移。
- 后端用单条 SQL（`CASE/GROUP BY/FILTER`）即可实现聚合，避免 N+1。

**边界与影响**：
- 首次从用户主页点击「私信」时，前端直接跳到 `/messages/:peerId`；会话列表是在「私信 tab」里按需懒加载的。
- 没有「删除会话/删除单条消息」功能（MVP 明确不做）。
- 未读数以接收方 `to_user_id=? AND is_read=false` 为准；发送方不感知已读状态（无双向回执）。

## 2. 未读：底部 badge 合并通知 + 私信

**决策**：`/notifications/unread_count` 返回「通知未读数 + 私信未读数」之和，底部 Tab 徽标只显示一个总数。

**理由**：
- 社区 App 底部通常只有一个「消息」入口，拆成两个徽标会占空间且和现有「通知」tab 冲突。
- 合并实现简单：通知未读来自 `notifications` 表，私信未读来自 `messages` 表，service 层相加后返回。
- 进入「通知中心」后，用户可在「通知」/「私信」两个 tab 内分别处理各自的未读；底部徽标在任意 tab 内清空对应类型时都会减少。

**边界与影响**：
- `MarkAllRead`（通知）不影响私信未读；`MarkConversationRead` 不影响通知未读；两者都会通过 `refreshUnread()` 重新拉合并后的总数。
- 后续若要拆徽章，需要改接口返回结构（如 `{ notifications, messages }`），前端再拆两个徽标。

## 3. 新私信不生成通知行

**决策**：收到新私信时只在私信 tab 和 badge 中体现，**不向 `notifications` 表插入通知记录**。

**理由**：
- 避免通知表混入私信事件，保持两种消息类型的语义清晰。
- 零额外迁移：`messages` 表已支持 `is_read`，不需要新表或新字段。
- 用户点开私信 tab / 进入会话即可消费未读，不需要通知行做二次入口。

**边界与影响**：
- 当前没有「push/websocket 实时提醒」；用户刷新或切换 tab 时才会看到新私信。
- 如果未来需要「收到私信时同时产生一条通知（带跳转）」，需要扩展通知类型并谨慎处理已读一致性（避免两边都已读状态不同步）。

## 4. 私信限制：不可删 + 限长 ≤2000 rune + 限频 1 条/秒（同一对端）

**决策**：
- MVP 不支持删除单条消息或删除会话。
- 内容长度上限 2000 rune（使用 `utf8.RuneCountInString`）。
- 限频：同一发送者对同一接收者，1 秒内只能发送 1 条；进程内 `map[from:to]time.Time` + `sync.Mutex` 实现。

**理由**：
- 删除功能涉及「双边可见性」和合规问题（骚扰证据留存），MVP 不引入这些复杂度。
- 2000 rune 约等于 1~2 条长微信消息，足够社区私信，同时防止单条消息过大。
- 1 条/秒/对端既能防简单刷屏，又不对正常聊天造成明显影响；按「对端」粒度而不是全局粒度，避免 A→B 被 A→C 的发送行为阻塞。

**边界与影响**：
- 限频是进程内内存实现，多实例部署时不跨进程共享；MVP 单实例部署可接受，后续若水平扩展需改为 Redis/shared store。
- 限频失败返回 `429 Too ManyRequests`，前端提示「发送太频繁，请稍后再试」。
- 给自己发私信在 service 层直接返回 400（`ErrSelfMessage`）。

## 5. 对端画像：handler 层通过 `PeerProvider` 装配，notify 零依赖

**决策**：`notify.Service` 只存 `from_user_id`/`to_user_id`，不查 `user.users`；会话列表返回的 `peer_name`/`peer_avatar` 由 `handler` 通过 `notify.PeerProvider` 接口从 `user.Service` 获取并合并。

**理由**：
- 遵守 #5 D5 硬约束：notify 包是依赖图叶子，不能 join 其他域的表。
- 复用现有 seam 模式（`content.UserProvider` 已有先例），降低架构不一致性。
- 未来 user 域的画像字段变化不会影响 notify。

**边界与影响**：
- `PeerProvider.GetPeerView` 查不到用户时返回 `ErrPeerNotFound`（handler 映射为 404）。
- 会话列表若有 N 个对端，会产生 N 次 `GetPeerView` 调用；当前用户量下可接受，后续可批量优化（需扩展 seam）。

## 6. 排序与分页

**决策**：
- 会话列表：按「最后一条消息 ID」降序（ID 单调递增 ≈ 时间序）。
- 聊天消息：按消息 ID 降序，前端首屏展示最新，底部「查看更早消息」prepend 旧消息。

**理由**：
- 代码库已有使用自增 ID 做时间序的约定（post/comment 等），避免再引入 `created_at` 排序的不确定性。
- 聊天从底部往上翻，prepend 旧消息符合常见 IM 交互。

---

## 变更文件速查

- `backend/notify/*.go`：repo、service、gorm_repo、fake_repo、service_test
- `backend/internal/httpapi/handler/notify.go`：handler
- `backend/internal/httpapi/router.go`：adapter + 路由
- `backend/internal/httpapi/handler/content.go`：`pathInt64` helper
- `backend/notify/README.md`：接口契约
- `frontend/src/api/{types,notify}.ts`：类型与 API 函数
- `frontend/src/views/{Notifications,Chat,UserProfile}.vue`
- `frontend/src/views/{Notifications,Chat,UserProfile}.test.ts`
- `frontend/src/router/index.ts` + `frontend/src/App.vue`
