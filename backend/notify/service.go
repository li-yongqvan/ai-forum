package notify

import (
	"context"
	"errors"
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
	UserID     int64
	Page       int
	PageSize   int
	UnreadOnly bool // 仅取未读（通知页 ?unread=1）
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

// ---- 错误（调用方据此映射 HTTP 状态） ----

var (
	ErrNotFound    = errors.New("notify: 通知不存在")
	ErrInvalidType = errors.New("notify: 通知类型非法")
)

// ---- 实现 ----

// validTypes 通知类型白名单（与 schema CHECK 一致）。
var validTypes = map[string]bool{
	"follow": true, "like": true, "comment": true, "reply": true, "report_result": true,
}

type service struct {
	repo Repo
}

// NewService 构造 Service。
func NewService(repo Repo) Service {
	return &service{repo: repo}
}

var _ Service = (*service)(nil)

// CreateNotification 校验类型后落一条快照行（快照字段由调用方算好传入，#4 D4）。
func (s *service) CreateNotification(ctx context.Context, in CreateNotificationCmd) error {
	if !validTypes[in.Type] {
		return ErrInvalidType
	}
	return s.repo.CreateNotification(ctx, &Notification{
		RecipientID: in.RecipientID,
		Type:        in.Type,
		ActorID:     in.ActorID,
		ActorName:   in.ActorName,
		ActorAvatar: in.ActorAvatar,
		TargetType:  in.TargetType,
		TargetID:    in.TargetID,
		TargetTitle: in.TargetTitle,
	})
}

// MarkRead 标记单条已读；仅本人通知（归属过滤在 repo 层，非本人 → ErrNotFound）。
func (s *service) MarkRead(ctx context.Context, id, recipientID int64) error {
	return s.repo.MarkNotificationRead(ctx, id, recipientID)
}

// MarkAllRead 全部已读（幂等）。
func (s *service) MarkAllRead(ctx context.Context, recipientID int64) error {
	return s.repo.MarkAllNotificationsRead(ctx, recipientID)
}

// ListNotifications 分页列表（新→旧）；UnreadOnly 时仅未读。
func (s *service) ListNotifications(ctx context.Context, in ListQuery) ([]NotificationView, error) {
	offset, limit := paginate(in.Page, in.PageSize)
	ns, err := s.repo.ListNotifications(ctx, in.UserID, offset, limit, in.UnreadOnly)
	if err != nil {
		return nil, err
	}
	views := make([]NotificationView, 0, len(ns))
	for _, n := range ns {
		views = append(views, NotificationView{
			ID:          n.ID,
			Type:        n.Type,
			ActorID:     n.ActorID,
			ActorName:   n.ActorName,
			TargetType:  n.TargetType,
			TargetID:    n.TargetID,
			TargetTitle: n.TargetTitle,
			IsRead:      n.IsRead,
			CreatedAt:   n.CreatedAt,
		})
	}
	return views, nil
}

// UnreadCount 未读通知数（底部 Tab badge）。
func (s *service) UnreadCount(ctx context.Context, recipientID int64) (int64, error) {
	return s.repo.UnreadCount(ctx, recipientID)
}

// SendMessage 发送私信（MVP 未暴露路由，能力完整供后续使用）。
func (s *service) SendMessage(ctx context.Context, in SendMessageCmd) error {
	return s.repo.CreateMessage(ctx, &Message{
		FromUserID: in.FromUserID,
		ToUserID:   in.ToUserID,
		Content:    in.Content,
	})
}

func (s *service) MarkMessageRead(ctx context.Context, id, toUserID int64) error {
	return s.repo.MarkMessageRead(ctx, id, toUserID)
}

func (s *service) ListMessages(ctx context.Context, in ListQuery) ([]MessageView, error) {
	offset, limit := paginate(in.Page, in.PageSize)
	ms, err := s.repo.ListMessages(ctx, in.UserID, offset, limit)
	if err != nil {
		return nil, err
	}
	views := make([]MessageView, 0, len(ms))
	for _, m := range ms {
		views = append(views, MessageView{
			ID:         m.ID,
			FromUserID: m.FromUserID,
			ToUserID:   m.ToUserID,
			Content:    m.Content,
			IsRead:     m.IsRead,
			CreatedAt:  m.CreatedAt,
		})
	}
	return views, nil
}

// paginate 将 page/pageSize 归一为 offset/limit（默认 1/20，上限 100 钳制）。
func paginate(page, pageSize int) (offset, limit int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return (page - 1) * pageSize, pageSize
}
