package notify

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestCreateNotification(t *testing.T) {
	f := newFakeRepo()
	svc := NewService(f)

	actor := int64(2)
	name := "alice"
	if err := svc.CreateNotification(context.Background(), CreateNotificationCmd{
		RecipientID: 1, Type: "follow", ActorID: &actor, ActorName: &name, TargetType: strPtr("user"), TargetID: &actor,
	}); err != nil {
		t.Fatalf("CreateNotification err = %v", err)
	}
	if len(f.notifications) != 1 {
		t.Fatalf("通知数 = %d, want 1", len(f.notifications))
	}
	n := f.notifications[1]
	if n.RecipientID != 1 || n.Type != "follow" {
		t.Errorf("快照行 = %+v, want recipient=1 type=follow", n)
	}
	if n.ActorName == nil || *n.ActorName != "alice" || n.ActorID == nil || *n.ActorID != 2 {
		t.Errorf("actor 快照缺失: %+v", n)
	}
	if n.IsRead {
		t.Error("新建通知应为未读")
	}
}

func TestCreateNotificationInvalidType(t *testing.T) {
	f := newFakeRepo()
	svc := NewService(f)
	err := svc.CreateNotification(context.Background(), CreateNotificationCmd{RecipientID: 1, Type: "bogus"})
	if !errors.Is(err, ErrInvalidType) {
		t.Errorf("err = %v, want ErrInvalidType", err)
	}
	if len(f.notifications) != 0 {
		t.Error("非法类型不应写入")
	}
}

// #53：report_handled（被举报人视角）是合法类型，可落一行快照。
func TestCreateNotificationReportHandled(t *testing.T) {
	f := newFakeRepo()
	svc := NewService(f)
	title := "你的内容因「垃圾广告」被举报，已删除"
	targetID := int64(9)
	if err := svc.CreateNotification(context.Background(), CreateNotificationCmd{
		RecipientID: 100, Type: "report_handled",
		TargetType: strPtr("post"), TargetID: &targetID, TargetTitle: &title,
	}); err != nil {
		t.Fatalf("report_handled CreateNotification err = %v", err)
	}
	if len(f.notifications) != 1 {
		t.Fatalf("通知数 = %d, want 1", len(f.notifications))
	}
	n := f.notifications[1]
	if n.RecipientID != 100 || n.Type != "report_handled" || n.TargetTitle == nil || *n.TargetTitle != title {
		t.Errorf("快照行 = %+v", n)
	}
}

func TestListNotificationsPaginationAndScope(t *testing.T) {
	f := newFakeRepo()
	svc := NewService(f)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		actor := int64(i + 10)
		name := "u"
		if err := svc.CreateNotification(ctx, CreateNotificationCmd{RecipientID: 1, Type: "follow", ActorID: &actor, ActorName: &name}); err != nil {
			t.Fatal(err)
		}
	}
	actor := int64(99)
	name := "u2"
	if err := svc.CreateNotification(ctx, CreateNotificationCmd{RecipientID: 2, Type: "follow", ActorID: &actor, ActorName: &name}); err != nil {
		t.Fatal(err)
	}

	// 用户 1 第一页：新→旧，取 2 条
	views, err := svc.ListNotifications(ctx, ListQuery{UserID: 1, Page: 1, PageSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(views) != 2 {
		t.Fatalf("第一页 len = %d, want 2", len(views))
	}
	if views[0].ID != 3 || views[1].ID != 2 {
		t.Errorf("第一页顺序 = [%d %d], want [3 2]（新→旧）", views[0].ID, views[1].ID)
	}
	// 第二页剩 1 条
	views2, err := svc.ListNotifications(ctx, ListQuery{UserID: 1, Page: 2, PageSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(views2) != 1 || views2[0].ID != 1 {
		t.Errorf("第二页 = %+v, want id 1", views2)
	}
	// 用户 2 只见自己的（隔绝不泄露他人通知）
	views3, err := svc.ListNotifications(ctx, ListQuery{UserID: 2, Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(views3) != 1 || views3[0].ID != 4 {
		t.Errorf("用户 2 列表 = %+v, want id 4", views3)
	}
}

func TestUnreadOnlyFilter(t *testing.T) {
	f := newFakeRepo()
	svc := NewService(f)
	ctx := context.Background()
	for i := 0; i < 2; i++ {
		actor := int64(i + 1)
		name := "u"
		if err := svc.CreateNotification(ctx, CreateNotificationCmd{RecipientID: 1, Type: "follow", ActorID: &actor, ActorName: &name}); err != nil {
			t.Fatal(err)
		}
	}
	if err := svc.MarkRead(ctx, 1, 1); err != nil {
		t.Fatal(err)
	}
	views, err := svc.ListNotifications(ctx, ListQuery{UserID: 1, UnreadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(views) != 1 || views[0].ID != 2 {
		t.Errorf("unread 过滤 = %+v, want 仅 id 2", views)
	}
}

func TestMarkReadOwnership(t *testing.T) {
	f := newFakeRepo()
	svc := NewService(f)
	ctx := context.Background()
	actor := int64(2)
	name := "u"
	if err := svc.CreateNotification(ctx, CreateNotificationCmd{RecipientID: 1, Type: "follow", ActorID: &actor, ActorName: &name}); err != nil {
		t.Fatal(err)
	}
	// 本人已读成功
	if err := svc.MarkRead(ctx, 1, 1); err != nil {
		t.Errorf("本人已读 err = %v, want nil", err)
	}
	// 非本人 → ErrNotFound（归属隔离）
	if err := svc.MarkRead(ctx, 1, 999); !errors.Is(err, ErrNotFound) {
		t.Errorf("非本人已读 err = %v, want ErrNotFound", err)
	}
	// 不存在 → ErrNotFound
	if err := svc.MarkRead(ctx, 404, 1); !errors.Is(err, ErrNotFound) {
		t.Errorf("不存在已读 err = %v, want ErrNotFound", err)
	}
}

func TestMarkAllReadAndUnreadCount(t *testing.T) {
	f := newFakeRepo()
	svc := NewService(f)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		actor := int64(i + 1)
		name := "u"
		if err := svc.CreateNotification(ctx, CreateNotificationCmd{RecipientID: 1, Type: "follow", ActorID: &actor, ActorName: &name}); err != nil {
			t.Fatal(err)
		}
	}
	n, err := svc.UnreadCount(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Errorf("未读数 = %d, want 3", n)
	}
	if err := svc.MarkAllRead(ctx, 1); err != nil {
		t.Fatal(err)
	}
	n, _ = svc.UnreadCount(ctx, 1)
	if n != 0 {
		t.Errorf("全部已读后未读数 = %d, want 0", n)
	}
	// 幂等：再次全部已读不报错
	if err := svc.MarkAllRead(ctx, 1); err != nil {
		t.Errorf("重复全部已读 err = %v, want nil", err)
	}
}

// 同人反复触发不去重（#32 grilling：MVP 从简）——每条都落库。
func TestNoDedup(t *testing.T) {
	f := newFakeRepo()
	svc := NewService(f)
	ctx := context.Background()
	actor := int64(2)
	name := "u"
	for i := 0; i < 3; i++ {
		if err := svc.CreateNotification(ctx, CreateNotificationCmd{RecipientID: 1, Type: "follow", ActorID: &actor, ActorName: &name}); err != nil {
			t.Fatal(err)
		}
	}
	if len(f.notifications) != 3 {
		t.Errorf("同人反复触发应各写一条，实得 %d", len(f.notifications))
	}
}

// SendMessage 能力完整（MVP 未暴露 service 外部路由，仅验证 service 层可写可读）。
func TestSendMessageAndList(t *testing.T) {
	f := newFakeRepo()
	svc := NewService(f)
	ctx := context.Background()
	view, err := svc.SendMessage(ctx, SendMessageCmd{FromUserID: 2, ToUserID: 1, Content: "hi"})
	if err != nil {
		t.Fatal(err)
	}
	if view.Content != "hi" || view.FromUserID != 2 || view.ToUserID != 1 || view.ID == 0 {
		t.Errorf("SendMessage view = %+v, want 1 条 hi", view)
	}
	count, err := svc.UnreadMessageCount(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("未读私信数 = %d, want 1", count)
	}
}

func TestSendMessageValidation(t *testing.T) {
	f := newFakeRepo()
	svc := NewService(f)
	ctx := context.Background()

	cases := []struct {
		name    string
		cmd     SendMessageCmd
		wantErr error
	}{
		{"empty", SendMessageCmd{FromUserID: 1, ToUserID: 2, Content: "   "}, ErrEmptyMessage},
		{"self", SendMessageCmd{FromUserID: 1, ToUserID: 1, Content: "hi"}, ErrSelfMessage},
		{"too long", SendMessageCmd{FromUserID: 1, ToUserID: 2, Content: strings.Repeat("a", maxMessageLength+1)}, ErrMessageTooLong},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := svc.SendMessage(ctx, c.cmd)
			if !errors.Is(err, c.wantErr) {
				t.Errorf("err = %v, want %v", err, c.wantErr)
			}
		})
	}
}

func TestSendMessageRateLimit(t *testing.T) {
	old := sendInterval
	sendInterval = 500 * time.Millisecond
	t.Cleanup(func() { sendInterval = old })

	f := newFakeRepo()
	svc := NewService(f)
	ctx := context.Background()
	if _, err := svc.SendMessage(ctx, SendMessageCmd{FromUserID: 1, ToUserID: 2, Content: "a"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SendMessage(ctx, SendMessageCmd{FromUserID: 1, ToUserID: 2, Content: "b"}); !errors.Is(err, ErrRateLimited) {
		t.Errorf("second send err = %v, want ErrRateLimited", err)
	}
}

func TestListConversations(t *testing.T) {
	old := sendInterval
	sendInterval = 0
	t.Cleanup(func() { sendInterval = old })

	f := newFakeRepo()
	svc := NewService(f)
	ctx := context.Background()
	// 1 与 2、3 有会话；2 与 3 的消息不应出现在 1 的列表。
	mustSend(t, svc, ctx, 2, 1, "m1")
	mustSend(t, svc, ctx, 3, 1, "m2")
	mustSend(t, svc, ctx, 2, 1, "m3")
	mustSend(t, svc, ctx, 3, 2, "other") // 不相关

	cs, err := svc.ListConversations(ctx, ListQuery{UserID: 1, Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 2 {
		t.Fatalf("会话数 = %d, want 2", len(cs))
	}
	// 新→旧：最后一条是 from=2 to=1 "m3" → peer=2 排第一
	if cs[0].PeerID != 2 || cs[0].LastMessage.Content != "m3" || cs[0].UnreadCount != 2 {
		t.Errorf("第一会话 = %+v, want peer=2 last=m3 unread=2", cs[0])
	}
	if cs[1].PeerID != 3 || cs[1].UnreadCount != 1 {
		t.Errorf("第二会话 = %+v, want peer=3 unread=1", cs[1])
	}
}

func TestListConversationsPagination(t *testing.T) {
	old := sendInterval
	sendInterval = 0
	t.Cleanup(func() { sendInterval = old })

	f := newFakeRepo()
	svc := NewService(f)
	ctx := context.Background()
	mustSend(t, svc, ctx, 2, 1, "a")
	mustSend(t, svc, ctx, 3, 1, "b")
	mustSend(t, svc, ctx, 4, 1, "c")
	cs, err := svc.ListConversations(ctx, ListQuery{UserID: 1, Page: 1, PageSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 2 {
		t.Fatalf("第一页 = %d, want 2", len(cs))
	}
	cs2, err := svc.ListConversations(ctx, ListQuery{UserID: 1, Page: 2, PageSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(cs2) != 1 {
		t.Errorf("第二页 = %d, want 1", len(cs2))
	}
}

func TestListConversationMessagesScope(t *testing.T) {
	old := sendInterval
	sendInterval = 0
	t.Cleanup(func() { sendInterval = old })

	f := newFakeRepo()
	svc := NewService(f)
	ctx := context.Background()
	mustSend(t, svc, ctx, 2, 1, "a") // 相关
	mustSend(t, svc, ctx, 1, 2, "b") // 相关
	mustSend(t, svc, ctx, 3, 1, "c") // 不相关（peer 3）

	ms, err := svc.ListConversationMessages(ctx, 1, 2, ListQuery{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 2 {
		t.Fatalf("消息数 = %d, want 2", len(ms))
	}
	// 新→旧
	if ms[0].Content != "b" || ms[1].Content != "a" {
		t.Errorf("顺序 = [%s, %s], want [b, a]", ms[0].Content, ms[1].Content)
	}
}

func TestMarkConversationRead(t *testing.T) {
	old := sendInterval
	sendInterval = 0
	t.Cleanup(func() { sendInterval = old })

	f := newFakeRepo()
	svc := NewService(f)
	ctx := context.Background()
	mustSend(t, svc, ctx, 2, 1, "a")
	mustSend(t, svc, ctx, 2, 1, "b")
	mustSend(t, svc, ctx, 3, 1, "c")

	if err := svc.MarkConversationRead(ctx, 1, 2); err != nil {
		t.Fatal(err)
	}
	count, _ := svc.UnreadMessageCount(ctx, 1)
	if count != 1 {
		t.Errorf("未读数 = %d, want 1（仅 peer=3 剩余）", count)
	}
	// 幂等
	if err := svc.MarkConversationRead(ctx, 1, 2); err != nil {
		t.Errorf("重复已读 err = %v, want nil", err)
	}
}

func mustSend(t *testing.T, svc Service, ctx context.Context, from, to int64, content string) {
	t.Helper()
	if _, err := svc.SendMessage(ctx, SendMessageCmd{FromUserID: from, ToUserID: to, Content: content}); err != nil {
		t.Fatal(err)
	}
}

func strPtr(s string) *string { return &s }
