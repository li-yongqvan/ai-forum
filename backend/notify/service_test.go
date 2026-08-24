package notify

import (
	"context"
	"errors"
	"testing"
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

// SendMessage 能力完整（MVP 未暴露路由，仅验证 service 层可写可读）。
func TestSendMessageAndList(t *testing.T) {
	f := newFakeRepo()
	svc := NewService(f)
	ctx := context.Background()
	if err := svc.SendMessage(ctx, SendMessageCmd{FromUserID: 2, ToUserID: 1, Content: "hi"}); err != nil {
		t.Fatal(err)
	}
	ms, err := svc.ListMessages(ctx, ListQuery{UserID: 1, Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 1 || ms[0].Content != "hi" || ms[0].FromUserID != 2 {
		t.Errorf("messages = %+v, want 1 条 hi", ms)
	}
}

func strPtr(s string) *string { return &s }
