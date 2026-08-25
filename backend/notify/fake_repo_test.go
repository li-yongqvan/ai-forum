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
	return slicePage(out, offset, limit), nil
}

// ---- 私信（#59） ----

func (f *fakeRepo) ListConversations(ctx context.Context, userID int64, offset, limit int) ([]Conversation, error) {
	type agg struct {
		lastID      int64
		unreadCount int64
	}
	m := make(map[int64]*agg)
	for _, msg := range f.messages {
		if msg.FromUserID != userID && msg.ToUserID != userID {
			continue
		}
		peer := msg.FromUserID
		if peer == userID {
			peer = msg.ToUserID
		}
		a, ok := m[peer]
		if !ok {
			a = &agg{}
			m[peer] = a
		}
		if msg.ID > a.lastID {
			a.lastID = msg.ID
		}
		if msg.ToUserID == userID && !msg.IsRead {
			a.unreadCount++
		}
	}
	type pair struct {
		peer   int64
		lastID int64
		unread int64
	}
	pairs := make([]pair, 0, len(m))
	for peer, a := range m {
		pairs = append(pairs, pair{peer, a.lastID, a.unreadCount})
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].lastID > pairs[j].lastID })
	pairs = slicePage(pairs, offset, limit)

	out := make([]Conversation, 0, len(pairs))
	for _, p := range pairs {
		out = append(out, Conversation{
			PeerID:      p.peer,
			LastMessage: *f.messages[p.lastID],
			UnreadCount: p.unread,
		})
	}
	return out, nil
}

func (f *fakeRepo) ListConversationMessages(ctx context.Context, userID, peerID int64, offset, limit int) ([]Message, error) {
	out := make([]Message, 0)
	for _, m := range f.messages {
		if (m.FromUserID == userID && m.ToUserID == peerID) || (m.FromUserID == peerID && m.ToUserID == userID) {
			out = append(out, *m)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return slicePage(out, offset, limit), nil
}

func (f *fakeRepo) MarkConversationRead(ctx context.Context, userID, peerID int64) error {
	for _, m := range f.messages {
		if m.ToUserID == userID && m.FromUserID == peerID && !m.IsRead {
			m.IsRead = true
		}
	}
	return nil
}

func (f *fakeRepo) UnreadMessageCount(ctx context.Context, userID int64) (int64, error) {
	var n int64
	for _, m := range f.messages {
		if m.ToUserID == userID && !m.IsRead {
			n++
		}
	}
	return n, nil
}

func slicePage[T any](s []T, offset, limit int) []T {
	if offset < 0 {
		offset = 0
	}
	if offset > len(s) {
		offset = len(s)
	}
	end := len(s)
	if limit > 0 && offset+limit < end {
		end = offset + limit
	}
	return s[offset:end]
}
