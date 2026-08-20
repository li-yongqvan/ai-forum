package notify

import "context"

// Repo 是通知域持久化内部 seam（#7）。实现见后续 ticket。
type Repo interface {
	CreateNotification(ctx context.Context, n *Notification) error
	CreateMessage(ctx context.Context, m *Message) error
	// MarkNotificationRead 标记单条已读；仅本人通知生效（recipient 归属过滤），不存在/非本人返回 ErrNotFound。
	MarkNotificationRead(ctx context.Context, id, recipientID int64) error
	MarkAllNotificationsRead(ctx context.Context, recipientID int64) error
	MarkMessageRead(ctx context.Context, id, toUserID int64) error
	UnreadCount(ctx context.Context, recipientID int64) (int64, error)
	// ListNotifications 返回某接收人的通知分页列表（新→旧）。unreadOnly 时仅取未读。
	ListNotifications(ctx context.Context, recipientID int64, offset, limit int, unreadOnly bool) ([]Notification, error)
	// ListMessages 返回某用户收到的私信分页列表（新→旧）。
	ListMessages(ctx context.Context, toUserID int64, offset, limit int) ([]Message, error)
}
