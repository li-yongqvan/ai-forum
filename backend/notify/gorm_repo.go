package notify

import (
	"context"

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

func (r *gormRepo) MarkMessageRead(ctx context.Context, id, toUserID int64) error {
	res := r.db.WithContext(ctx).
		Model(&Message{}).
		Where("id = ? AND to_user_id = ?", id, toUserID).
		Update("is_read", true)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
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

// ListMessages 分页倒序（新→旧）。
func (r *gormRepo) ListMessages(ctx context.Context, toUserID int64, offset, limit int) ([]Message, error) {
	var ms []Message
	err := r.db.WithContext(ctx).
		Model(&Message{}).
		Where("to_user_id = ?", toUserID).
		Order("id DESC").
		Limit(limit).Offset(offset).
		Find(&ms).Error
	return ms, err
}
