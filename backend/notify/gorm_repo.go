package notify

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

// gormRepo 是 Repo 的 GORM 实现（实现细节，深模块内隐藏）。
type gormRepo struct {
	db *gorm.DB
}

// NewGormRepo 以真实连接构造 Repo。
func NewGormRepo(db *gorm.DB) Repo {
	return &gormRepo{db: db}
}

var _ Repo = (*gormRepo)(nil)

// CreateNotification 写入一条通知快照行（#5 D5：快照自足，不读别的表）。
func (r *gormRepo) CreateNotification(ctx context.Context, n *Notification) error {
	return r.db.WithContext(ctx).Create(n).Error
}

func (r *gormRepo) CreateMessage(ctx context.Context, m *Message) error {
	return r.db.WithContext(ctx).Create(m).Error
}

// MarkNotificationRead 标记单条已读；非本人通知不生效（recipient 归属过滤），
// 0 行受影响 → ErrNotFound（深接口：调用方按哨兵错误处理，不关心 SQL）。
func (r *gormRepo) MarkNotificationRead(ctx context.Context, id, recipientID int64) error {
	res := r.db.WithContext(ctx).
		Model(&Notification{}).
		Where("id = ? AND recipient_id = ?", id, recipientID).
		Update("is_read", true)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *gormRepo) MarkAllNotificationsRead(ctx context.Context, recipientID int64) error {
	return r.db.WithContext(ctx).
		Model(&Notification{}).
		Where("recipient_id = ? AND is_read = ?", recipientID, false).
		Update("is_read", true).Error
}

// UnreadCount 未读通知数（底部 Tab badge）。
func (r *gormRepo) UnreadCount(ctx context.Context, recipientID int64) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).
		Model(&Notification{}).
		Where("recipient_id = ? AND is_read = ?", recipientID, false).
		Count(&n).Error
	return n, err
}

// ListNotifications 分页倒序（新→旧，id 单调即时间倒序）。unreadOnly 时只取未读。
func (r *gormRepo) ListNotifications(ctx context.Context, recipientID int64, offset, limit int, unreadOnly bool) ([]Notification, error) {
	q := r.db.WithContext(ctx).
		Model(&Notification{}).
		Where("recipient_id = ?", recipientID)
	if unreadOnly {
		q = q.Where("is_read = ?", false)
	}
	var ns []Notification
	err := q.Order("id DESC").Limit(limit).Offset(offset).Find(&ns).Error
	return ns, err
}

// ---- 私信（#59） ----

// conversationRow 是会话聚合 SQL 的临时结果。
type conversationRow struct {
	PeerID      int64 `gorm:"column:peer_id"`
	LastID      int64 `gorm:"column:last_id"`
	UnreadCount int64 `gorm:"column:unread_count"`
}

// ListConversations 按对端聚合返回会话列表（新→旧）。
// 采用 Raw SQL 避免 GORM 对别名分组/复杂 SELECT 的绑定不确定性（评审 F3）。
func (r *gormRepo) ListConversations(ctx context.Context, userID int64, offset, limit int) ([]Conversation, error) {
	var rows []conversationRow
	sql := `
SELECT CASE WHEN from_user_id = ? THEN to_user_id ELSE from_user_id END AS peer_id,
       MAX(id) AS last_id,
       COUNT(*) FILTER (WHERE to_user_id = ? AND is_read = false) AS unread_count
FROM notify.messages
WHERE from_user_id = ? OR to_user_id = ?
GROUP BY peer_id
ORDER BY last_id DESC
LIMIT ? OFFSET ?`
	if err := r.db.WithContext(ctx).Raw(sql, userID, userID, userID, userID, limit, offset).Scan(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return []Conversation{}, nil
	}
	lastIDs := make([]int64, len(rows))
	for i, r := range rows {
		lastIDs[i] = r.LastID
	}
	var lasts []Message
	if err := r.db.WithContext(ctx).
		Model(&Message{}).
		Where("id IN ?", lastIDs).
		Find(&lasts).Error; err != nil {
		return nil, err
	}
	lastByID := make(map[int64]Message, len(lasts))
	for _, m := range lasts {
		lastByID[m.ID] = m
	}

	out := make([]Conversation, 0, len(rows))
	for _, row := range rows {
		last, ok := lastByID[row.LastID]
		if !ok {
			return nil, fmt.Errorf("conversation last message %d not found", row.LastID)
		}
		out = append(out, Conversation{
			PeerID:      row.PeerID,
			LastMessage: last,
			UnreadCount: row.UnreadCount,
		})
	}
	return out, nil
}

// ListConversationMessages 返回双方往来消息（新→旧）。
func (r *gormRepo) ListConversationMessages(ctx context.Context, userID, peerID int64, offset, limit int) ([]Message, error) {
	var ms []Message
	err := r.db.WithContext(ctx).
		Model(&Message{}).
		Where("(from_user_id = ? AND to_user_id = ?) OR (from_user_id = ? AND to_user_id = ?)",
			userID, peerID, peerID, userID).
		Order("id DESC").
		Limit(limit).Offset(offset).
		Find(&ms).Error
	return ms, err
}

// MarkConversationRead 将该对端发给 userID 的入站消息全部标为已读（幂等）。
func (r *gormRepo) MarkConversationRead(ctx context.Context, userID, peerID int64) error {
	return r.db.WithContext(ctx).
		Model(&Message{}).
		Where("to_user_id = ? AND from_user_id = ? AND is_read = ?", userID, peerID, false).
		Update("is_read", true).Error
}

// UnreadMessageCount 返回 userID 的未读入站私信数。
func (r *gormRepo) UnreadMessageCount(ctx context.Context, userID int64) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).
		Model(&Message{}).
		Where("to_user_id = ? AND is_read = ?", userID, false).
		Count(&n).Error
	return n, err
}
