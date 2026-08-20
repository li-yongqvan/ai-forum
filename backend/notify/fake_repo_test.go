package notify

import (
	"context"
	"sort"
	"time"
)

// fakeRepo 是 Repo 的内存实现，仅用于本包测试（testing.md §2.1：in-memory fake 跨 Service 接口 seam 验证）。
type fakeRepo struct {
	notifications map[int64]*Notification
	messages      map[int64]*Message
	nextNID       int64
	nextMID       int64
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		notifications: map[int64]*Notification{},
		messages:      map[int64]*Message{},
		nextNID:       1,
		nextMID:       1,
	}
}

func (f *fakeRepo) CreateNotification(ctx context.Context, n *Notification) error {
	n.ID = f.nextNID
	f.nextNID++
	n.CreatedAt = time.Now()
	n.UpdatedAt = n.CreatedAt
	cp := *n
	f.notifications[n.ID] = &cp
	return nil
}

func (f *fakeRepo) CreateMessage(ctx context.Context, m *Message) error {
	m.ID = f.nextMID
	f.nextMID++
	m.CreatedAt = time.Now()
	m.UpdatedAt = m.CreatedAt
	cp := *m
	f.messages[m.ID] = &cp
	return nil
}

// MarkNotificationRead 模拟 GORM 归属过滤：不存在或非本人 → ErrNotFound。
func (f *fakeRepo) MarkNotificationRead(ctx context.Context, id, recipientID int64) error {
	n, ok := f.notifications[id]
	if !ok || n.RecipientID != recipientID {
		return ErrNotFound
	}
	n.IsRead = true
	return nil
}

func (f *fakeRepo) MarkAllNotificationsRead(ctx context.Context, recipientID int64) error {
	for _, n := range f.notifications {
		if n.RecipientID == recipientID && !n.IsRead {
			n.IsRead = true
		}
	}
	return nil
}

func (f *fakeRepo) MarkMessageRead(ctx context.Context, id, toUserID int64) error {
	m, ok := f.messages[id]
	if !ok || m.ToUserID != toUserID {
		return ErrNotFound
	}
	m.IsRead = true
	return nil
}

func (f *fakeRepo) UnreadCount(ctx context.Context, recipientID int64) (int64, error) {
	var n int64
	for _, x := range f.notifications {
		if x.RecipientID == recipientID && !x.IsRead {
			n++
		}
	}
	return n, nil
}

// ListNotifications 新→旧（id 单调即时间倒序，与 GORM Order("id DESC") 一致）。
func (f *fakeRepo) ListNotifications(ctx context.Context, recipientID int64, offset, limit int, unreadOnly bool) ([]Notification, error) {
	out := make([]Notification, 0)
	for _, n := range f.notifications {
		if n.RecipientID != recipientID {
			continue
		}
		if unreadOnly && n.IsRead {
			continue
		}
		out = append(out, *n)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	if offset > len(out) {
		offset = len(out)
	}
	if limit > 0 && offset+limit < len(out) {
		out = out[offset : offset+limit]
	} else if offset < len(out) {
		out = out[offset:]
	}
	return out, nil
}

func (f *fakeRepo) ListMessages(ctx context.Context, toUserID int64, offset, limit int) ([]Message, error) {
	out := make([]Message, 0)
	for _, m := range f.messages {
		if m.ToUserID != toUserID {
			continue
		}
		out = append(out, *m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	if offset > len(out) {
		offset = len(out)
	}
	if limit > 0 && offset+limit < len(out) {
		out = out[offset : offset+limit]
	} else if offset < len(out) {
		out = out[offset:]
	}
	return out, nil
}
