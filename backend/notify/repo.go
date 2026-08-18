package notify

import "context"

// Repo 是通知域持久化内部 seam（#7）。实现见后续 ticket。
type Repo interface {
	CreateNotification(ctx context.Context, n *Notification) error
	CreateMessage(ctx context.Context, m *Message) error
	MarkNotificationRead(ctx context.Context, id, recipientID int64) error
	MarkAllNotificationsRead(ctx context.Context, recipientID int64) error
	MarkMessageRead(ctx context.Context, id, toUserID int64) error
	UnreadCount(ctx context.Context, recipientID int64) (int64, error)
}
