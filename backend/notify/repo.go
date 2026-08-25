package notify

import "context"

// Repo 是通知域持久化内部 seam（#7）。实现见后续 ticket。
type Repo interface {
	CreateNotification(ctx context.Context, n *Notification) error
	CreateMessage(ctx context.Context, m *Message) error
	// MarkNotificationRead 标记单条已读；仅本人通知生效（recipient 归属过滤），不存在/非本人返回 ErrNotFound。
	MarkNotificationRead(ctx context.Context, id, recipientID int64) error
	MarkAllNotificationsRead(ctx context.Context, recipientID int64) error
	UnreadCount(ctx context.Context, recipientID int64) (int64, error)
	// ListNotifications 返回某接收人的通知分页列表（新→旧）。unreadOnly 时仅取未读。
	ListNotifications(ctx context.Context, recipientID int64, offset, limit int, unreadOnly bool) ([]Notification, error)

	// ---- 私信会话（#59） ----
	// ListConversations 按对端聚合返回某用户的会话列表（新→旧）。
	ListConversations(ctx context.Context, userID int64, offset, limit int) ([]Conversation, error)
	// ListConversationMessages 返回某用户与某对端的双向消息（新→旧）。
	ListConversationMessages(ctx context.Context, userID, peerID int64, offset, limit int) ([]Message, error)
	// MarkConversationRead 将该对端发给 userID 的入站消息全部标为已读（幂等）。
	MarkConversationRead(ctx context.Context, userID, peerID int64) error
	// UnreadMessageCount 返回 userID 的未读入站私信数。
	UnreadMessageCount(ctx context.Context, userID int64) (int64, error)
}
