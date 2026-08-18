package notify

import (
	"context"
	"time"
)

// ---- 命令 / 读模型 DTO ----

// CreateNotificationCmd 构造通知。快照字段（ActorName/ActorAvatar/TargetTitle）由调用方算好传入。
type CreateNotificationCmd struct {
	RecipientID int64
	Type        string // follow | like | comment | reply | report_result
	ActorID     *int64
	ActorName   *string
	ActorAvatar *string
	TargetType  *string
	TargetID    *int64
	TargetTitle *string
}

type NotificationView struct {
	ID          int64      `json:"id"`
	Type        string     `json:"type"`
	ActorID     *int64     `json:"actor_id,omitempty"`
	ActorName   *string    `json:"actor_name,omitempty"`
	TargetType  *string    `json:"target_type,omitempty"`
	TargetID    *int64     `json:"target_id,omitempty"`
	TargetTitle *string    `json:"target_title,omitempty"`
	IsRead      bool       `json:"is_read"`
	CreatedAt   time.Time  `json:"created_at"`
}

type MessageView struct {
	ID         int64     `json:"id"`
	FromUserID int64     `json:"from_user_id"`
	ToUserID   int64     `json:"to_user_id"`
	Content    string    `json:"content"`
	IsRead     bool      `json:"is_read"`
	CreatedAt  time.Time `json:"created_at"`
}

type SendMessageCmd struct {
	FromUserID int64
	ToUserID   int64
	Content    string
}

type ListQuery struct {
	UserID   int64
	Page     int
	PageSize int
}

// Service 是通知域的对外接口。通知类型跳转锚点见 IA v2 §4（follow→用户主页，like/comment/reply→帖子详情评论锚点，report_result→结果提示）。
type Service interface {
	CreateNotification(ctx context.Context, in CreateNotificationCmd) error
	MarkRead(ctx context.Context, id, recipientID int64) error
	MarkAllRead(ctx context.Context, recipientID int64) error
	ListNotifications(ctx context.Context, in ListQuery) ([]NotificationView, error)
	UnreadCount(ctx context.Context, recipientID int64) (int64, error)

	SendMessage(ctx context.Context, in SendMessageCmd) error
	MarkMessageRead(ctx context.Context, id, toUserID int64) error
	ListMessages(ctx context.Context, in ListQuery) ([]MessageView, error)
}
