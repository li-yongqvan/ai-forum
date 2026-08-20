package moderation

import (
	"context"
	"errors"
	"testing"
)

func ctx() context.Context { return context.Background() }

// newSvc 装配真实 service + fakes，并预置一条 pending 举报（举报人 101 → 作者 100 的 post 1）。
func newSvc(t *testing.T) (*service, *fakeRepo, *fakeContent, *fakeUsers, *fakeNotifier, *Report) {
	t.Helper()
	repo := newFakeRepo()
	content := newFakeContent()
	users := newFakeUsers()
	notify := newFakeNotifier()
	svc := NewService(repo, content, users, notify)

	users.add(100, "alice") // 作者
	users.add(101, "bob")   // 举报人
	users.add(102, "mod")   // 处理人
	content.addRef("post", 1, TargetRef{AuthorID: 100, Title: "举报帖"})
	content.addRef("comment", 2, TargetRef{AuthorID: 100, Title: "一条评论"})

	id, err := svc.CreateReport(ctx(), CreateReportCmd{
		ReporterID: 101, TargetType: "post", TargetID: 1, Reason: "垃圾广告", Note: "广告链接",
	})
	if err != nil {
		t.Fatalf("预置举报失败: %v", err)
	}
	rep, err := repo.GetReportByID(ctx(), id)
	if err != nil {
		t.Fatalf("取预置举报失败: %v", err)
	}
	return svc.(*service), repo, content, users, notify, rep
}

func TestCreateReportSuccess(t *testing.T) {
	repo := newFakeRepo()
	content := newFakeContent()
	users := newFakeUsers()
	notify := newFakeNotifier()
	svc := NewService(repo, content, users, notify)
	users.add(100, "alice")
	users.add(101, "bob")
	content.addRef("post", 1, TargetRef{AuthorID: 100, Title: "帖子"})

	id, err := svc.CreateReport(ctx(), CreateReportCmd{
		ReporterID: 101, TargetType: "post", TargetID: 1, Reason: "其他", Note: "补充说明",
	})
	if err != nil {
		t.Fatalf("CreateReport = %v", err)
	}
	if id == 0 {
		t.Error("应返回举报 id")
	}
	r, _ := repo.GetReportByID(ctx(), id)
	if r.Reason != "其他" { // O2：reason 只存枚举
		t.Errorf("reason = %q, want 枚举原值", r.Reason)
	}
	if r.ReporterNote == nil || *r.ReporterNote != "补充说明" { // O2：备注独立入 reporter_note
		t.Errorf("reporter_note = %v, want 补充说明", r.ReporterNote)
	}
	if r.Status != StatusPending {
		t.Errorf("status = %s, want pending", r.Status)
	}
}

func TestCreateReportValidations(t *testing.T) {
	_, _, content, users, notify, _ := newSvc(t)
	svc := NewService(newFakeRepo(), content, users, notify)

	base := CreateReportCmd{ReporterID: 101, TargetType: "post", TargetID: 1, Reason: "垃圾广告"}

	cases := []struct {
		name string
		mut  func(*CreateReportCmd)
		want error
	}{
		{"非法目标类型", func(c *CreateReportCmd) { c.TargetType = "channel" }, ErrInvalidAction},
		{"非法原因", func(c *CreateReportCmd) { c.Reason = "瞎编" }, ErrInvalidAction},
		{"其他无备注", func(c *CreateReportCmd) { c.Reason = "其他" }, ErrInvalidAction},
		{"自举报（作者=举报人）", func(c *CreateReportCmd) { c.ReporterID = 100 }, ErrInvalidAction},
		{"目标不存在", func(c *CreateReportCmd) { c.TargetID = 999 }, ErrNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := base
			tc.mut(&cmd)
			_, err := svc.CreateReport(ctx(), cmd)
			if !errors.Is(err, tc.want) {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestCreateReportUserSelfReport(t *testing.T) {
	repo := newFakeRepo()
	content := newFakeContent()
	users := newFakeUsers()
	notify := newFakeNotifier()
	svc := NewService(repo, content, users, notify)
	users.add(5, "me")

	// 举报自己 → ErrInvalidAction
	_, err := svc.CreateReport(ctx(), CreateReportCmd{ReporterID: 5, TargetType: "user", TargetID: 5, Reason: "垃圾广告"})
	if !errors.Is(err, ErrInvalidAction) {
		t.Errorf("自举报 user = %v, want ErrInvalidAction", err)
	}
	// user 目标不存在 → ErrNotFound
	_, err = svc.CreateReport(ctx(), CreateReportCmd{ReporterID: 5, TargetType: "user", TargetID: 6, Reason: "垃圾广告"})
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("user 目标不存在 = %v, want ErrNotFound", err)
	}
}

func TestCreateReportDuplicate(t *testing.T) {
	_, _, content, users, notify, _ := newSvc(t)
	svc := NewService(newFakeRepo(), content, users, notify)
	// 先建一条，再建同 target 同原因 → 重复
	if _, err := svc.CreateReport(ctx(), CreateReportCmd{ReporterID: 101, TargetType: "post", TargetID: 1, Reason: "垃圾广告"}); err != nil {
		t.Fatalf("首次 = %v", err)
	}
	_, err := svc.CreateReport(ctx(), CreateReportCmd{ReporterID: 101, TargetType: "post", TargetID: 1, Reason: "违法违规"})
	if !errors.Is(err, ErrDuplicatePending) {
		t.Errorf("重复 = %v, want ErrDuplicatePending", err)
	}
}

func TestHandleReportDismiss(t *testing.T) {
	_, repo, _, _, notify, rep := newSvc(t)
	svc := NewService(repo, newFakeContent(), newFakeUsers(), notify)

	if err := svc.HandleReport(ctx(), HandleReportCmd{ReportID: rep.ID, HandlerID: 102, OperatorRole: "moderator", Action: ActionDismiss, Note: "证据不足"}); err != nil {
		t.Fatalf("dismiss = %v", err)
	}
	r, _ := repo.GetReportByID(ctx(), rep.ID)
	if r.Status != StatusDismissed {
		t.Errorf("status = %s, want dismissed", r.Status)
	}
	if r.HandlerID == nil || *r.HandlerID != 102 || r.HandledAt == nil { // F2：dismiss 写处理人留痕
		t.Errorf("dismiss 应写 handler_id/handled_at, got %+v", r)
	}
	if len(repo.actions) != 0 {
		t.Errorf("dismiss 不应落审计, got %d", len(repo.actions))
	}
	if len(notify.calls) != 0 {
		t.Errorf("dismiss 不应发通知, got %d", len(notify.calls))
	}
}

func TestHandleReportDeletePost(t *testing.T) {
	_, repo, content, _, notify, rep := newSvc(t)
	svc := NewService(repo, content, newFakeUsers(), notify)

	if err := svc.HandleReport(ctx(), HandleReportCmd{ReportID: rep.ID, HandlerID: 102, OperatorRole: "moderator", Action: ActionDeletePost, Note: "违规删除"}); err != nil {
		t.Fatalf("delete_post = %v", err)
	}
	// 内容网关以处理人身份调用（O1-③）
	if len(content.deleted) != 1 || content.deleted[0].OperatorID != 102 || content.deleted[0].OperatorRole != "moderator" || content.deleted[0].TargetID != 1 {
		t.Errorf("DeletePost 调用 = %+v", content.deleted)
	}
	// 状态 resolved + 审计
	r, _ := repo.GetReportByID(ctx(), rep.ID)
	if r.Status != StatusResolved {
		t.Errorf("status = %s, want resolved", r.Status)
	}
	if len(repo.actions) != 1 || repo.actions[0].Action != ActionDeletePost || repo.actions[0].TargetType != "post" || repo.actions[0].TargetID != 1 {
		t.Errorf("审计 = %+v", repo.actions)
	}
	// 通知举报人（结论句）
	if len(notify.calls) != 1 || notify.calls[0].RecipientID != 101 || notify.calls[0].Type != "report_result" ||
		notify.calls[0].TargetTitle == nil || *notify.calls[0].TargetTitle != "内容已删除" {
		t.Errorf("通知 = %+v", notify.calls)
	}
}

func TestHandleReportWarnAttribsAuthor(t *testing.T) {
	_, repo, content, _, notify, rep := newSvc(t)
	svc := NewService(repo, content, newFakeUsers(), notify)

	if err := svc.HandleReport(ctx(), HandleReportCmd{ReportID: rep.ID, HandlerID: 102, OperatorRole: "admin", Action: ActionWarn}); err != nil {
		t.Fatalf("warn = %v", err)
	}
	// 审计记被举报内容作者（target_type=user）
	if len(repo.actions) != 1 || repo.actions[0].Action != ActionWarn || repo.actions[0].TargetType != "user" || repo.actions[0].TargetID != 100 {
		t.Errorf("warn 审计 = %+v", repo.actions)
	}
	if len(notify.calls) != 1 || notify.calls[0].TargetTitle == nil || *notify.calls[0].TargetTitle != "已警告违规用户" {
		t.Errorf("warn 通知 = %+v", notify.calls)
	}
}

func TestHandleReportFailures(t *testing.T) {
	svc, _, _, _, _, rep := newSvc(t)
	// 补一条 comment 目标举报，用于「delete_post 目标非 post」校验
	cid, err := svc.CreateReport(ctx(), CreateReportCmd{ReporterID: 101, TargetType: "comment", TargetID: 2, Reason: "垃圾广告"})
	if err != nil {
		t.Fatalf("建 comment 举报 = %v", err)
	}

	handle := func(cmd HandleReportCmd) error { return svc.HandleReport(ctx(), cmd) }

	cases := []struct {
		name string
		mut  func(*HandleReportCmd)
		want error
	}{
		{"非 mod 角色", func(c *HandleReportCmd) { c.OperatorRole = "user" }, ErrInvalidAction},
		{"封禁留 #34", func(c *HandleReportCmd) { c.Action = ActionBan }, ErrInvalidAction},
		{"非法动作", func(c *HandleReportCmd) { c.Action = "nuke" }, ErrInvalidAction},
		{"delete_post 目标非 post", func(c *HandleReportCmd) { c.Action = ActionDeletePost; c.ReportID = cid }, ErrInvalidAction},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := HandleReportCmd{ReportID: rep.ID, HandlerID: 102, OperatorRole: "moderator", Action: ActionDismiss}
			tc.mut(&cmd)
			if err := handle(cmd); !errors.Is(err, tc.want) {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
		})
	}

	// 已处理不可再处理（先 dismiss 再 dismiss → 已结案 ErrInvalidAction）
	if err := handle(HandleReportCmd{ReportID: rep.ID, HandlerID: 102, OperatorRole: "moderator", Action: ActionDismiss}); err != nil {
		t.Fatalf("首次 dismiss = %v", err)
	}
	if err := handle(HandleReportCmd{ReportID: rep.ID, HandlerID: 102, OperatorRole: "moderator", Action: ActionDismiss}); !errors.Is(err, ErrInvalidAction) {
		t.Errorf("重复处理 = %v, want ErrInvalidAction", err)
	}

	// 内容网关失败 → 传播、不落审计、状态保持 pending
	_, repo2, content2, _, notify2, rep2 := newSvc(t)
	content2.deleteErr = errors.New("db down")
	svc2 := NewService(repo2, content2, newFakeUsers(), notify2)
	if err := svc2.HandleReport(ctx(), HandleReportCmd{ReportID: rep2.ID, HandlerID: 102, OperatorRole: "moderator", Action: ActionDeletePost}); err == nil {
		t.Error("delete 网关失败应传播错误")
	}
	r2, _ := repo2.GetReportByID(ctx(), rep2.ID)
	if r2.Status != StatusPending {
		t.Errorf("网关失败后 status = %s, want pending（回滚）", r2.Status)
	}
	if len(notify2.calls) != 0 {
		t.Error("网关失败不应发通知")
	}
}

func TestListReportsEnrich(t *testing.T) {
	_, repo, content, users, notify, _ := newSvc(t)
	svc := NewService(repo, content, users, notify)

	// 再补一条已结案（dismiss）用于状态过滤
	content.addRef("comment", 2, TargetRef{AuthorID: 100, Title: "评论摘要"})
	id2, err := svc.CreateReport(ctx(), CreateReportCmd{ReporterID: 101, TargetType: "comment", TargetID: 2, Reason: "引战灌水"})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.HandleReport(ctx(), HandleReportCmd{ReportID: id2, HandlerID: 102, OperatorRole: "moderator", Action: ActionDismiss}); err != nil {
		t.Fatal(err)
	}

	// pending 过滤
	pending, err := svc.ListReports(ctx(), ListReportsQuery{Status: StatusPending, Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 || pending[0].TargetType != "post" || pending[0].TargetTitle != "举报帖" || pending[0].ReporterUsername != "bob" {
		t.Errorf("pending 列表 = %+v", pending)
	}

	// 全部（dismissed 也在）
	all, err := svc.ListReports(ctx(), ListReportsQuery{Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Errorf("全量 = %d, want 2", len(all))
	}

	// CountReports
	n, _ := svc.CountReports(ctx(), StatusPending)
	if n != 1 {
		t.Errorf("pending count = %d, want 1", n)
	}
}
