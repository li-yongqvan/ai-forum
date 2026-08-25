package moderation

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
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
	// IA §5.6「所有动作→通知举报人」：dismiss 也发结论通知（含 dismiss 结论句）；#53 D2 dismiss 不通知被举报人
	if len(notify.calls) != 1 || notify.calls[0].RecipientID != 101 || notify.calls[0].Type != "report_result" ||
		notify.calls[0].TargetTitle == nil || *notify.calls[0].TargetTitle != "未采取处理" {
		t.Errorf("dismiss 应发 1 条 report_result 结论通知, got %+v", notify.calls)
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
	// 通知：举报人 report_result（结论句）+ 被举报人 report_handled（作者视角，含原因，#53 D1/D2）
	if len(notify.calls) != 2 {
		t.Fatalf("delete_post 应发 2 条通知, got %+v", notify.calls)
	}
	c0, c1 := notify.calls[0], notify.calls[1]
	if c0.RecipientID != 101 || c0.Type != "report_result" || c0.TargetTitle == nil || *c0.TargetTitle != "内容已删除" {
		t.Errorf("举报人通知 = %+v", c0)
	}
	if c1.RecipientID != 100 || c1.Type != "report_handled" || c1.TargetTitle == nil || *c1.TargetTitle != "你的内容因「垃圾广告」被举报，已删除" {
		t.Errorf("被举报人通知 = %+v", c1)
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
	// 通知：举报人 report_result + 被举报人 report_handled（#53 D2，warn 也通知，反转 O4）
	if len(notify.calls) != 2 {
		t.Fatalf("warn 应发 2 条通知, got %+v", notify.calls)
	}
	if notify.calls[0].RecipientID != 101 || notify.calls[0].TargetTitle == nil || *notify.calls[0].TargetTitle != "已警告违规用户" {
		t.Errorf("举报人通知 = %+v", notify.calls[0])
	}
	c1 := notify.calls[1]
	if c1.RecipientID != 100 || c1.Type != "report_handled" || c1.TargetTitle == nil || *c1.TargetTitle != "你因「垃圾广告」被举报，已警告" {
		t.Errorf("被举报人通知 = %+v", c1)
	}
}

func TestHandleReportDeleteCommentNotifiesAuthor(t *testing.T) {
	repo := newFakeRepo()
	content := newFakeContent()
	users := newFakeUsers()
	notify := newFakeNotifier()
	svc := NewService(repo, content, users, notify)
	users.add(100, "alice")
	users.add(101, "bob")
	content.addRef("comment", 2, TargetRef{AuthorID: 100, Title: "一条评论"})

	id, err := svc.CreateReport(ctx(), CreateReportCmd{ReporterID: 101, TargetType: "comment", TargetID: 2, Reason: "垃圾广告"})
	if err != nil {
		t.Fatalf("CreateReport = %v", err)
	}
	if err := svc.HandleReport(ctx(), HandleReportCmd{ReportID: id, HandlerID: 102, OperatorRole: "moderator", Action: ActionDeleteComment}); err != nil {
		t.Fatalf("delete_comment = %v", err)
	}
	if len(notify.calls) != 2 {
		t.Fatalf("delete_comment 应发 2 条通知, got %+v", notify.calls)
	}
	c1 := notify.calls[1]
	if c1.RecipientID != 100 || c1.Type != "report_handled" || c1.TargetTitle == nil || *c1.TargetTitle != "你的内容因「垃圾广告」被举报，已删除" {
		t.Errorf("被举报人通知 = %+v", c1)
	}
}

func TestHandleReportWarnUserTargetNotifiesUser(t *testing.T) {
	repo := newFakeRepo()
	content := newFakeContent()
	users := newFakeUsers()
	notify := newFakeNotifier()
	svc := NewService(repo, content, users, notify)
	users.add(100, "alice")
	users.add(101, "bob")
	users.add(102, "mod")
	// user 目标的 ResolveTarget 直接回显 targetID（与 adapter 一致），fake 需预置该 ref
	content.addRef("user", 100, TargetRef{AuthorID: 100, Title: ""})

	id, err := svc.CreateReport(ctx(), CreateReportCmd{ReporterID: 101, TargetType: "user", TargetID: 100, Reason: "人身攻击"})
	if err != nil {
		t.Fatalf("CreateReport = %v", err)
	}
	if err := svc.HandleReport(ctx(), HandleReportCmd{ReportID: id, HandlerID: 102, OperatorRole: "moderator", Action: ActionWarn}); err != nil {
		t.Fatalf("warn = %v", err)
	}
	if len(notify.calls) != 2 {
		t.Fatalf("warn user 目标应发 2 条通知, got %+v", notify.calls)
	}
	c1 := notify.calls[1]
	if c1.RecipientID != 100 || c1.Type != "report_handled" || c1.TargetTitle == nil || *c1.TargetTitle != "你因「人身攻击」被举报，已警告" {
		t.Errorf("被举报人通知 = %+v", c1)
	}
}

func TestHandleReportLegacyNilTargetAuthor(t *testing.T) {
	_, repo, content, _, notify, rep := newSvc(t)
	svc := NewService(repo, content, newFakeUsers(), notify)
	// 模拟 0007 之前建的存量举报：无 target_author_id 快照
	repo.reports[rep.ID].TargetAuthorID = nil

	if err := svc.HandleReport(ctx(), HandleReportCmd{ReportID: rep.ID, HandlerID: 102, OperatorRole: "moderator", Action: ActionDeletePost, Note: "违规删除"}); err != nil {
		t.Fatalf("delete_post = %v", err)
	}
	// 存量无快照 → 只发举报人通知，跳过被举报人（slog.Warn），不 panic
	if len(notify.calls) != 1 || notify.calls[0].Type != "report_result" || notify.calls[0].RecipientID != 101 {
		t.Errorf("存量无快照应只发 1 条举报人通知, got %+v", notify.calls)
	}
}

func TestCreateReportRateLimit(t *testing.T) {
	repo := newFakeRepo()
	content := newFakeContent()
	users := newFakeUsers()
	notify := newFakeNotifier()
	svc := NewService(repo, content, users, notify)
	users.add(100, "alice")
	users.add(101, "bob")
	for i := int64(1); i <= 6; i++ {
		content.addRef("post", i, TargetRef{AuthorID: 100, Title: "帖"})
	}
	create := func(targetID int64) error {
		_, err := svc.CreateReport(ctx(), CreateReportCmd{ReporterID: 101, TargetType: "post", TargetID: targetID, Reason: "垃圾广告"})
		return err
	}

	// 前 5 次成功（不同目标避开 fake 的 pending 去重）
	for i := int64(1); i <= reportLimit; i++ {
		if err := create(i); err != nil {
			t.Fatalf("第 %d 次 = %v, want 成功", i, err)
		}
	}
	// 第 6 次 → ErrRateLimited，且不落库
	if err := create(6); !errors.Is(err, ErrRateLimited) {
		t.Errorf("第 %d 次 = %v, want ErrRateLimited", reportLimit+1, err)
	}
	if len(repo.reports) != reportLimit {
		t.Errorf("落库条数 = %d, want %d（超限不落库）", len(repo.reports), reportLimit)
	}

	// 窗口过期：把已有举报 CreatedAt 改到窗口外 → 可再举报（含 dismissed 也计入，MVP 精度）
	now := time.Now()
	for _, r := range repo.reports {
		r.CreatedAt = now.Add(-reportWindow - time.Minute)
	}
	if err := create(6); err != nil {
		t.Errorf("窗口外重报 = %v, want 成功", err)
	}
	if len(repo.reports) != reportLimit+1 {
		t.Errorf("窗口外应落第 6 条, got %d 条", len(repo.reports))
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
		{" moderator 封禁拒绝", func(c *HandleReportCmd) { c.Action = ActionBan }, ErrInvalidAction},
		{"非法动作", func(c *HandleReportCmd) { c.Action = "nuke" }, ErrInvalidAction},
		{"delete_post 目标非 post", func(c *HandleReportCmd) { c.Action = ActionDeletePost; c.ReportID = cid }, ErrInvalidAction},
		{"备注超 500（F5）", func(c *HandleReportCmd) { c.Note = strings.Repeat("长", 501) }, ErrInvalidAction},
		{"ban 原因空", func(c *HandleReportCmd) { c.Action = ActionBan; c.OperatorRole = "admin"; c.Note = "" }, ErrInvalidAction},
		{"ban 原因仅空白", func(c *HandleReportCmd) { c.Action = ActionBan; c.OperatorRole = "admin"; c.Note = "  " }, ErrInvalidAction},
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

func TestRecordAction(t *testing.T) {
	newSvcFor := func(t *testing.T) *service {
		t.Helper()
		repo := newFakeRepo()
		return NewService(repo, newFakeContent(), newFakeUsers(), newFakeNotifier()).(*service)
	}

	t.Run("ban_user 成功", func(t *testing.T) {
		svc := newSvcFor(t)
		if err := svc.RecordAction(ctx(), RecordActionCmd{ModeratorID: 9, Action: ActionBan, TargetID: 42, Reason: "人身攻击"}); err != nil {
			t.Fatalf("RecordAction() error = %v", err)
		}
		actions := svc.repo.(*fakeRepo).actions
		if len(actions) != 1 {
			t.Fatalf("审计行数 = %d, want 1", len(actions))
		}
		a := actions[0]
		if a.Action != ActionBan || a.TargetType != "user" || a.TargetID != 42 || a.ModeratorID != 9 || a.Reason != "人身攻击" {
			t.Errorf("审计行 = %+v, want ban_user/user/42/moderator 9/reason 人身攻击", a)
		}
	})

	t.Run("unban_user 成功", func(t *testing.T) {
		svc := newSvcFor(t)
		if err := svc.RecordAction(ctx(), RecordActionCmd{ModeratorID: 9, Action: ActionUnban, TargetID: 42, Reason: "误封纠正"}); err != nil {
			t.Fatalf("RecordAction() error = %v", err)
		}
		if a := svc.repo.(*fakeRepo).actions[0]; a.Action != ActionUnban {
			t.Errorf("action = %q, want unban_user", a.Action)
		}
	})

	t.Run("reason 空 → ErrInvalidAction", func(t *testing.T) {
		svc := newSvcFor(t)
		err := svc.RecordAction(ctx(), RecordActionCmd{ModeratorID: 9, Action: ActionBan, TargetID: 42, Reason: "  "})
		if !errors.Is(err, ErrInvalidAction) {
			t.Errorf("error = %v, want ErrInvalidAction", err)
		}
	})

	t.Run("reason 超长 → ErrInvalidAction", func(t *testing.T) {
		svc := newSvcFor(t)
		err := svc.RecordAction(ctx(), RecordActionCmd{ModeratorID: 9, Action: ActionBan, TargetID: 42, Reason: strings.Repeat("长", 501)})
		if !errors.Is(err, ErrInvalidAction) {
			t.Errorf("error = %v, want ErrInvalidAction", err)
		}
	})

	t.Run("非白名单动作 → ErrInvalidAction", func(t *testing.T) {
		svc := newSvcFor(t)
		for _, bad := range []string{ActionDismiss, ActionWarn, ActionDeletePost, "nuke"} {
			if err := svc.RecordAction(ctx(), RecordActionCmd{ModeratorID: 9, Action: bad, TargetID: 42, Reason: "x"}); !errors.Is(err, ErrInvalidAction) {
				t.Errorf("action=%q error = %v, want ErrInvalidAction", bad, err)
			}
		}
	})

	t.Run("TargetID 非法 → ErrInvalidAction", func(t *testing.T) {
		svc := newSvcFor(t)
		if err := svc.RecordAction(ctx(), RecordActionCmd{ModeratorID: 9, Action: ActionBan, TargetID: 0, Reason: "x"}); !errors.Is(err, ErrInvalidAction) {
			t.Errorf("error = %v, want ErrInvalidAction", err)
		}
	})
}

// TestHandleReportBanUser 覆盖 #60 弹窗封禁：admin 成功、目标解析、通知、已封 409。
func TestHandleReportBanUser(t *testing.T) {
	t.Run("admin ban post 举报作者", func(t *testing.T) {
		_, repo, content, users, notify, rep := newSvc(t)
		svc := NewService(repo, content, users, notify)

		err := svc.HandleReport(ctx(), HandleReportCmd{
			ReportID: rep.ID, HandlerID: 102, OperatorRole: "admin", Action: ActionBan, Note: "严重违规",
		})
		if err != nil {
			t.Fatalf("ban = %v", err)
		}
		// 举报结案
		r, _ := repo.GetReportByID(ctx(), rep.ID)
		if r.Status != StatusResolved {
			t.Errorf("status = %s, want resolved", r.Status)
		}
		// 审计记被封用户
		if len(repo.actions) != 1 || repo.actions[0].Action != ActionBan || repo.actions[0].TargetType != "user" || repo.actions[0].TargetID != 100 {
			t.Errorf("审计 = %+v", repo.actions)
		}
		if repo.actions[0].Reason != "严重违规" {
			t.Errorf("reason = %q, want 严重违规", repo.actions[0].Reason)
		}
		// 双通知
		if len(notify.calls) != 2 {
			t.Fatalf("应发 2 条通知, got %+v", notify.calls)
		}
		if notify.calls[0].RecipientID != 101 || notify.calls[0].TargetTitle == nil || *notify.calls[0].TargetTitle != "已封禁用户" {
			t.Errorf("举报人通知 = %+v", notify.calls[0])
		}
		c1 := notify.calls[1]
		if c1.RecipientID != 100 || c1.Type != "report_handled" || c1.TargetTitle == nil || *c1.TargetTitle != "你因「垃圾广告」被举报，已被封禁" {
			t.Errorf("被举报人通知 = %+v", c1)
		}
	})

	t.Run("admin ban user 目标", func(t *testing.T) {
		repo := newFakeRepo()
		content := newFakeContent()
		users := newFakeUsers()
		notify := newFakeNotifier()
		svc := NewService(repo, content, users, notify)
		users.add(100, "alice")
		users.add(101, "bob")
		users.add(102, "admin")
		content.addRef("user", 100, TargetRef{AuthorID: 100, Title: ""})

		id, err := svc.CreateReport(ctx(), CreateReportCmd{ReporterID: 101, TargetType: "user", TargetID: 100, Reason: "违法违规"})
		if err != nil {
			t.Fatalf("CreateReport = %v", err)
		}
		if err := svc.HandleReport(ctx(), HandleReportCmd{ReportID: id, HandlerID: 102, OperatorRole: "admin", Action: ActionBan, Note: "违法"}); err != nil {
			t.Fatalf("ban = %v", err)
		}
		if len(repo.actions) != 1 || repo.actions[0].TargetID != 100 {
			t.Errorf("审计 = %+v", repo.actions)
		}
	})

	t.Run("moderator 拒绝 ban", func(t *testing.T) {
		svc, _, _, _, _, rep := newSvc(t)
		err := svc.HandleReport(ctx(), HandleReportCmd{ReportID: rep.ID, HandlerID: 102, OperatorRole: "moderator", Action: ActionBan, Note: "x"})
		if !errors.Is(err, ErrInvalidAction) {
			t.Errorf("err = %v, want ErrInvalidAction", err)
		}
	})

	t.Run("重复封禁 → ErrAlreadyBanned", func(t *testing.T) {
		_, repo, content, users, notify, rep := newSvc(t)
		svc := NewService(repo, content, users, notify)
		if err := svc.HandleReport(ctx(), HandleReportCmd{ReportID: rep.ID, HandlerID: 102, OperatorRole: "admin", Action: ActionBan, Note: "第一次"}); err != nil {
			t.Fatalf("首次 ban = %v", err)
		}
		// 再建一条同目标举报，再次 ban → 已封
		id2, err := svc.CreateReport(ctx(), CreateReportCmd{ReporterID: 101, TargetType: "post", TargetID: 1, Reason: "违法违规"})
		if err != nil {
			t.Fatalf("二次举报 = %v", err)
		}
		err = svc.HandleReport(ctx(), HandleReportCmd{ReportID: id2, HandlerID: 102, OperatorRole: "admin", Action: ActionBan, Note: "第二次"})
		if !errors.Is(err, ErrAlreadyBanned) {
			t.Errorf("err = %v, want ErrAlreadyBanned", err)
		}
	})

	t.Run("自封 → ErrSelfBan", func(t *testing.T) {
		_, repo, content, users, notify, rep := newSvc(t)
		svc := NewService(repo, content, users, notify)
		// 处理人即被举报作者（100）→ self-ban
		err := svc.HandleReport(ctx(), HandleReportCmd{ReportID: rep.ID, HandlerID: 100, OperatorRole: "admin", Action: ActionBan, Note: "自封"})
		if !errors.Is(err, ErrSelfBan) {
			t.Errorf("err = %v, want ErrSelfBan", err)
		}
	})
}

// TestListActions 覆盖审计只读查询：过滤、分页、enrich 用户名。
func TestListActions(t *testing.T) {
	repo := newFakeRepo()
	users := newFakeUsers()
	users.add(10, "modA")
	users.add(11, "modB")
	users.add(99, "ghost") // 不存在操作人，enrich 为空
	svc := NewService(repo, newFakeContent(), users, newFakeNotifier())

	// 直接 append 三条审计（绕过 RecordAction 以测试不同 moderator）
	now := time.Now()
	repo.actions = append(repo.actions,
		&ModerationAction{ID: 1, ModeratorID: 10, Action: ActionBan, TargetType: "user", TargetID: 100, Reason: "r1", CreatedAt: now.Add(-time.Hour)},
		&ModerationAction{ID: 2, ModeratorID: 11, Action: ActionDeletePost, TargetType: "post", TargetID: 5, Reason: "r2", CreatedAt: now},
		&ModerationAction{ID: 3, ModeratorID: 10, Action: ActionWarn, TargetType: "user", TargetID: 101, Reason: "r3", CreatedAt: now.Add(-2 * time.Hour)},
	)

	all, err := svc.ListActions(ctx(), ListActionsQuery{Limit: 10})
	if err != nil {
		t.Fatalf("ListActions = %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("all = %d, want 3", len(all))
	}
	// 倒序：id 2 → 1 → 3（按 created_at 倒序）
	if all[0].ID != 2 || all[1].ID != 1 || all[2].ID != 3 {
		t.Errorf("order = %v", []int64{all[0].ID, all[1].ID, all[2].ID})
	}
	if all[0].ModeratorUsername != "modB" || all[1].ModeratorUsername != "modA" {
		t.Errorf("usernames = %+v", []string{all[0].ModeratorUsername, all[1].ModeratorUsername})
	}

	byMod, err := svc.ListActions(ctx(), ListActionsQuery{ModeratorID: 10, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(byMod) != 2 {
		t.Errorf("byMod = %d, want 2", len(byMod))
	}

	byTarget, err := svc.ListActions(ctx(), ListActionsQuery{TargetType: "user", TargetID: 100, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(byTarget) != 1 || byTarget[0].ID != 1 {
		t.Errorf("byTarget = %+v", byTarget)
	}

	page, err := svc.ListActions(ctx(), ListActionsQuery{Limit: 2, Offset: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(page) != 2 || page[0].ID != 1 || page[1].ID != 3 {
		t.Errorf("page = %v", []int64{page[0].ID, page[1].ID})
	}
}
