package notify

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// ---- 命令 / 读模型 DTO ----

// CreateNotificationCmd 构造通知。快照字段（ActorName/ActorAvatar/TargetTitle）由调用方算好传入。
type CreateNotificationCmd struct {
	RecipientID int64
	Type        string // follow | like | comment | reply | report_result | report_handled
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

type ConversationView struct {
	PeerID      int64       `json:"peer_id"`
	PeerName    string      `json:"peer_name"`
	PeerAvatar  *string     `json:"peer_avatar,omitempty"`
	LastMessage MessageView `json:"last_message"`
	UnreadCount int64       `json:"unread_count"`
}

// PeerView 是 notify 域可见的最小对端画像（handler 层可再 enrich）。
type PeerView struct {
	ID       int64
	Username string
	Avatar   *string
}

// PeerProvider 供 notify handler 解析对端画像（notify 零依赖，不 import user）。
type PeerProvider interface {
	GetPeerView(ctx context.Context, id int64) (PeerView, error)
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

// Service 是通知域的对外接口。通知类型跳转锚点见 IA v2 §4（follow→用户主页，like/comment/reply→帖子详情评论锚点，report_result/report_handled→结果提示）。
type Service interface {
	CreateNotification(ctx context.Context, in CreateNotificationCmd) error
	MarkRead(ctx context.Context, id, recipientID int64) error
	MarkAllRead(ctx context.Context, recipientID int64) error
	ListNotifications(ctx context.Context, in ListQuery) ([]NotificationView, error)
	UnreadCount(ctx context.Context, recipientID int64) (int64, error)

	SendMessage(ctx context.Context, in SendMessageCmd) (MessageView, error)
	ListConversations(ctx context.Context, in ListQuery) ([]ConversationView, error)
	ListConversationMessages(ctx context.Context, userID, peerID int64, in ListQuery) ([]MessageView, error)
	MarkConversationRead(ctx context.Context, userID, peerID int64) error
	UnreadMessageCount(ctx context.Context, userID int64) (int64, error)
}

// ---- 错误（调用方据此映射 HTTP 状态） ----

var (
	ErrNotFound       = errors.New("notify: 通知不存在")
	ErrInvalidType    = errors.New("notify: 通知类型非法")
	ErrEmptyMessage   = errors.New("notify: 私信内容不能为空")
	ErrSelfMessage    = errors.New("notify: 不能给自己发私信")
	ErrMessageTooLong = errors.New("notify: 私信内容过长")
	ErrRateLimited    = errors.New("notify: 发送过于频繁，请稍后再试")
	ErrPeerNotFound   = errors.New("notify: 接收用户不存在")
)

// ---- 实现 ----

const (
	maxMessageLength    = 2000
	defaultSendInterval = 1 * time.Second
)

// sendInterval 可被测试覆写。
var sendInterval = defaultSendInterval

// validTypes 通知类型白名单（与 schema CHECK 一致：#53 加 report_handled，被举报人视角的举报处理通知）。
var validTypes = map[string]bool{
	"follow": true, "like": true, "comment": true, "reply": true, "report_result": true, "report_handled": true,
}

type service struct {
	repo    Repo
	rateMu  sync.Mutex
	rateMap map[string]time.Time
}

// NewService 构造 Service。
func NewService(repo Repo) Service {
	return &service{repo: repo, rateMap: make(map[string]time.Time)}
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

// SendMessage 发送私信并返回落库消息。
func (s *service) SendMessage(ctx context.Context, in SendMessageCmd) (MessageView, error) {
	content := strings.TrimSpace(in.Content)
	if content == "" {
		return MessageView{}, ErrEmptyMessage
	}
	if in.FromUserID == in.ToUserID {
		return MessageView{}, ErrSelfMessage
	}
	if utf8.RuneCountInString(content) > maxMessageLength {
		return MessageView{}, ErrMessageTooLong
	}
	key := rateKey(in.FromUserID, in.ToUserID)
	s.rateMu.Lock()
	if last, ok := s.rateMap[key]; ok && time.Since(last) < sendInterval {
		s.rateMu.Unlock()
		return MessageView{}, ErrRateLimited
	}
	s.rateMap[key] = time.Now()
	s.rateMu.Unlock()

	m := &Message{FromUserID: in.FromUserID, ToUserID: in.ToUserID, Content: content}
	if err := s.repo.CreateMessage(ctx, m); err != nil {
		return MessageView{}, err
	}
	return toMessageView(*m), nil
}

func rateKey(from, to int64) string {
	return strconv.FormatInt(from, 10) + ":" + strconv.FormatInt(to, 10)
}

func toMessageView(m Message) MessageView {
	return MessageView{
		ID:         m.ID,
		FromUserID: m.FromUserID,
		ToUserID:   m.ToUserID,
		Content:    m.Content,
		IsRead:     m.IsRead,
		CreatedAt:  m.CreatedAt,
	}
}

func (s *service) ListConversations(ctx context.Context, in ListQuery) ([]ConversationView, error) {
	offset, limit := paginate(in.Page, in.PageSize)
	cs, err := s.repo.ListConversations(ctx, in.UserID, offset, limit)
	if err != nil {
		return nil, err
	}
	views := make([]ConversationView, 0, len(cs))
	for _, c := range cs {
		views = append(views, ConversationView{
			PeerID:      c.PeerID,
			LastMessage: toMessageView(c.LastMessage),
			UnreadCount: c.UnreadCount,
		})
	}
	return views, nil
}

func (s *service) ListConversationMessages(ctx context.Context, userID, peerID int64, in ListQuery) ([]MessageView, error) {
	offset, limit := paginate(in.Page, in.PageSize)
	ms, err := s.repo.ListConversationMessages(ctx, userID, peerID, offset, limit)
	if err != nil {
		return nil, err
	}
	views := make([]MessageView, 0, len(ms))
	for _, m := range ms {
		views = append(views, toMessageView(m))
	}
	return views, nil
}

func (s *service) MarkConversationRead(ctx context.Context, userID, peerID int64) error {
	return s.repo.MarkConversationRead(ctx, userID, peerID)
}

func (s *service) UnreadMessageCount(ctx context.Context, userID int64) (int64, error) {
	return s.repo.UnreadMessageCount(ctx, userID)
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
