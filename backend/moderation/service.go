// Package moderation 是治理域（#4 服务候选边界）。本文件实现举报闭环：举报 → 处理（删除/警告/忽略）
// → append 审计 → 双通知（report_result 给举报人 + report_handled 给被举报人，#53）。跨包依赖经
// seam（ContentGateway/UserGateway/Notifier）注入，不 import content/user/notify（S1，仿 content.UserProvider 先例）。
package moderation

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"
)

// ---- 处理动作与状态（对应 moderation_actions.action / reports.status 枚举） ----

// 处理动作（对应 moderation_actions.action 枚举，另含不落审计的 dismiss）。
const (
	ActionDismiss       = "dismiss"        // 忽略举报：仅改 report 状态，不追加审计（F2 裁决）
	ActionDeletePost    = "delete_post"    // 删除内容（帖子）→ 追加审计
	ActionDeleteComment = "delete_comment" // 删除内容（评论）→ 追加审计
	ActionWarn          = "warn"           // 警告用户 → 追加审计
	ActionBan           = "ban_user"       // 封禁用户 → 追加审计（仅 admin，§5.0；#34 RecordAction 已启用）
	ActionUnban         = "unban_user"     // 解封用户 → 追加审计（仅 admin；#34）
)

// report 状态（schema 0004 CHECK）。
const (
	StatusPending   = "pending"
	StatusResolved  = "resolved"
	StatusDismissed = "dismissed"
)

// ---- 命令 / 读模型 DTO ----

type CreateReportCmd struct {
	ReporterID int64
	TargetType string // user | post | comment
	TargetID   int64
	Reason     string // 六枚举之一（D4）
	Note       string // 举报人备注（O2：独立入 reporter_note 列，不拼进 reason）
}

type ReportView struct {
	ID               int64      `json:"id"`
	ReporterID       int64      `json:"reporter_id"`
	ReporterUsername string     `json:"reporter_username,omitempty"` // enrich（UserGateway）
	TargetType       string     `json:"target_type"`
	TargetID         int64      `json:"target_id"`
	TargetTitle      string     `json:"target_title,omitempty"` // enrich（帖子标题/评论摘要/用户名）
	Reason           string     `json:"reason"`
	ReporterNote     *string    `json:"reporter_note,omitempty"`
	Status           string     `json:"status"`
	HandlerID        *int64     `json:"handler_id,omitempty"`
	HandledAt        *time.Time `json:"handled_at,omitempty"`
	HandlingNote     *string    `json:"handling_note,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
}

// HandleReportCmd 处理举报（IA v2 §5.6 闭环）。
// 权限动作集（#9 §5.0）：moderator 动作集为 dismiss/delete_post/delete_comment/warn；admin 额外可用 ban_user。
// 服务端二次鉴权（F3：OperatorRole 从 JWT claims 注入）。
type HandleReportCmd struct {
	ReportID     int64
	HandlerID    int64
	OperatorRole string // user | moderator | admin（F3）
	Action       string // Action* 常量之一（含 #60 ban_user）
	Note         string // 处理备注（附加到 moderation_actions.reason，≤500，F5）
}

// ListReportsQuery 管理队列查询（S8）。
type ListReportsQuery struct {
	Status string // 空 = 全部；默认 pending 由 handler 层补
	Limit  int
	Offset int
}

// ListActionsQuery 审计日志只读查询（#60）。
type ListActionsQuery struct {
	TargetType  string // 空 = 不过滤
	TargetID    int64  // 0 = 不过滤
	Action      string // 空 = 不过滤
	ModeratorID int64  // 0 = 不过滤
	Limit       int
	Offset      int
}

// BanUserCmd 治理域经 UserGateway 调用 user 域封禁的最小命令（S1：moderation 不 import user）。
type BanUserCmd struct {
	OperatorID int64
	TargetID   int64
}

// RecordActionCmd 独立治理审计命令（#34：ban/unban 直接治理动作，独立于举报流程）。
// TargetType 恒 "user"；Reason 必填且 ≤500（F5，moderation_actions.reason VARCHAR(500)）。
type RecordActionCmd struct {
	ModeratorID int64
	Action      string // ActionBan | ActionUnban（validRecordAction 白名单）
	TargetID    int64
	Reason      string
}

// ---- 跨包网关 seam（S1：moderation 不 import content/user/notify） ----

// TargetRef 举报目标的最小解析结果。
type TargetRef struct {
	AuthorID int64
	Title    string // 帖子标题 / 评论摘要 / 空
}

// DeleteTargetCmd 删除内容命令（operator 身份用于 content 侧 canModerate 校验，O1-③）。
type DeleteTargetCmd struct {
	OperatorID   int64
	OperatorRole string
	TargetID     int64
}

// ContentGateway 供治理域读写内容。post/comment 不存在或已删 → ErrNotFound（adapter 映射 content 域错误）；
// DeletePost/DeleteComment 的「不存在/已删」由 adapter 映射为成功（O1-② 幂等重处理）。
type ContentGateway interface {
	ResolveTarget(ctx context.Context, targetType string, targetID int64) (TargetRef, error)
	DeletePost(ctx context.Context, in DeleteTargetCmd) error
	DeleteComment(ctx context.Context, in DeleteTargetCmd) error
}

// UserRef 治理域可见的最小用户画像。
type UserRef struct {
	ID       int64
	Username string
}

// UserGateway 供治理域读用户与执行封禁（S1： moderation 不 import user）。
type UserGateway interface {
	GetUserView(ctx context.Context, id int64) (UserRef, error)
	// BanUser 调 user 域封禁；错误由 adapter 翻译为 moderation 哨兵错误（ErrSelfBan/ErrCannotBanAdmin/ErrAlreadyBanned/ErrNotFound）。
	BanUser(ctx context.Context, in BanUserCmd) error
}

// NotificationCmd 镜像 notify.CreateNotificationCmd 的最小子集（快照由调用方算好，#4 D4）。
type NotificationCmd struct {
	RecipientID int64
	Type        string
	ActorID     *int64
	ActorName   *string
	TargetType  *string
	TargetID    *int64
	TargetTitle *string
}

// Notifier 供治理域发通知（#53：report_result 给举报人 + report_handled 给被举报人）。
type Notifier interface {
	CreateNotification(ctx context.Context, in NotificationCmd) error
}

// ---- 错误（调用方据此映射 HTTP 状态） ----

var (
	ErrNotFound         = errors.New("moderation: 举报或目标不存在")
	ErrDuplicatePending = errors.New("moderation: 同一举报者 pending 期内重复举报")
	ErrInvalidAction    = errors.New("moderation: 非法处理动作或权限越界")
	ErrRateLimited      = errors.New("moderation: 举报过于频繁，请稍后再试")
	// #60 弹窗封禁经 UserGateway 翻译 user 域错误（adapter 负责，不 import user）。
	ErrSelfBan        = errors.New("moderation: 不能封禁自己")
	ErrCannotBanAdmin = errors.New("moderation: 不能封禁管理员")
	ErrAlreadyBanned  = errors.New("moderation: 该用户已被封禁")
)

// ---- 举报频控（#53 D3：同一举报人时间窗口内 ≤N 次，全局跨目标） ----
const (
	reportWindow = 10 * time.Minute
	reportLimit  = 5
)

// ---- 白名单 ----

// validReasons 举报原因枚举（IA v2 §5.6：D4）。reason 列只存枚举（O2）。
var validReasons = map[string]bool{
	"垃圾广告": true, "违法违规": true, "人身攻击": true,
	"抄袭侵权": true, "引战灌水": true, "其他": true,
}

func validTargetType(t string) bool { return t == "user" || t == "post" || t == "comment" }

func canModerate(role string) bool { return role == "moderator" || role == "admin" }

// validRecordAction 独立审计白名单：仅 ban_user/unban_user（#34）。
func validRecordAction(a string) bool { return a == ActionBan || a == ActionUnban }

func statusFor(action string) string {
	if action == ActionDismiss {
		return StatusDismissed
	}
	return StatusResolved
}

// conclusionFor 通知给举报人的结论句（S3/D2；前端模板「举报结果：${target_title}」拼后半句）。
func conclusionFor(action string) string {
	switch action {
	case ActionDeletePost, ActionDeleteComment:
		return "内容已删除"
	case ActionWarn:
		return "已警告违规用户"
	case ActionBan:
		return "已封禁用户"
	default: // ActionDismiss
		return "未采取处理"
	}
}

// notePtr 空备注归一为 nil（reporter_note 可空列）。
func notePtr(note string) *string {
	note = strings.TrimSpace(note)
	if note == "" {
		return nil
	}
	return &note
}

// Service 是治理域的对外接口。
// 处理举报 = 改 report.status（resolved/dismissed）+ 若执行删/警告则追加一条 moderation_actions（审计留痕，#5 D6）。
type Service interface {
	CreateReport(ctx context.Context, in CreateReportCmd) (int64, error)
	HandleReport(ctx context.Context, in HandleReportCmd) error
	ListReports(ctx context.Context, in ListReportsQuery) ([]ReportView, error)
	CountReports(ctx context.Context, status string) (int64, error)
	// ListActions 审计日志只读查询（#60）。
	ListActions(ctx context.Context, in ListActionsQuery) ([]ModerationActionView, error)
	// RecordAction 追加独立治理审计（#34：ban/unban 直接动作，reason 必填 ≤500，TargetType 恒 user）。
	RecordAction(ctx context.Context, in RecordActionCmd) error
}

// ---- 实现 ----

type service struct {
	repo    Repo
	content ContentGateway
	users   UserGateway
	notify  Notifier
}

// NewService 构造 Service。网关 seam 由组合根（httpapi）装配。
func NewService(repo Repo, content ContentGateway, users UserGateway, notify Notifier) Service {
	return &service{repo: repo, content: content, users: users, notify: notify}
}

var _ Service = (*service)(nil)

// CreateReport 举报：校验原因枚举/目标类型 → 存在性 + 自举报拒绝（F7）→ pending 防重复 → 频控 → 落库。返回举报 id。
func (s *service) CreateReport(ctx context.Context, in CreateReportCmd) (int64, error) {
	if !validTargetType(in.TargetType) || !validReasons[in.Reason] {
		return 0, ErrInvalidAction
	}
	// IA §5.6：选「其他」时备注必填（前端也拦，后端兜底）
	if in.Reason == "其他" && strings.TrimSpace(in.Note) == "" {
		return 0, ErrInvalidAction
	}

	// 目标存在性 + 自举报拒绝（F7）。post/comment 经内容网关解析作者；user 直接比对并查存在。
	// #53：顺路快照被举报人 id，供处理后补发 report_handled（delete 路径幂等不 ResolveTarget）。
	var targetAuthorID *int64
	switch in.TargetType {
	case "post", "comment":
		ref, err := s.content.ResolveTarget(ctx, in.TargetType, in.TargetID)
		if err != nil {
			return 0, err // 目标不存在/已删 → ErrNotFound(404)
		}
		if ref.AuthorID == in.ReporterID {
			return 0, ErrInvalidAction // 不能举报自己的内容
		}
		targetAuthorID = &ref.AuthorID
	case "user":
		if in.TargetID == in.ReporterID {
			return 0, ErrInvalidAction
		}
		if _, err := s.users.GetUserView(ctx, in.TargetID); err != nil {
			return 0, err
		}
		targetAuthorID = &in.TargetID
	}

	// 防重复举报（uq_reports_pending 主防 + 此处预检双保险）：同目标 pending 重复的 409 语义优先于全局频控 429（#53 §6.4）
	dup, err := s.repo.PendingExists(ctx, in.ReporterID, in.TargetType, in.TargetID)
	if err != nil {
		return 0, err
	}
	if dup {
		return 0, ErrDuplicatePending
	}

	// 举报频控（#53 D3：全局窗口，同一举报人 reportWindow 内 ≤reportLimit 次）。软限（count+insert 非原子，
	// 并发可少量超发，MVP 接受；硬不变量仍是 uq_reports_pending）；不落库（D4）。
	n, err := s.repo.CountReportsSince(ctx, in.ReporterID, time.Now().Add(-reportWindow))
	if err != nil {
		return 0, err
	}
	if n >= reportLimit {
		return 0, ErrRateLimited
	}

	rep := &Report{
		ReporterID:     in.ReporterID,
		TargetType:     in.TargetType,
		TargetID:       in.TargetID,
		TargetAuthorID: targetAuthorID,
		Reason:         in.Reason,
		ReporterNote:   notePtr(in.Note),
		Status:         StatusPending,
	}
	if err := s.repo.CreateReport(ctx, rep); err != nil {
		return 0, err
	}
	return rep.ID, nil
}

// HandleReport 处理举报（O1 序列：网关副作用在事务外 → moderation 写单事务 → 提交后事务外发通知）。
func (s *service) HandleReport(ctx context.Context, in HandleReportCmd) error {
	// 角色门 + 动作门：收敛为单个 switch（Q1），只留一张白名单。
	switch in.Action {
	case ActionBan:
		if in.OperatorRole != "admin" {
			return ErrInvalidAction
		}
	case ActionDismiss, ActionDeletePost, ActionDeleteComment, ActionWarn:
		if !canModerate(in.OperatorRole) {
			return ErrInvalidAction
		}
	default:
		return ErrInvalidAction
	}
	if len([]rune(in.Note)) > 500 { // F5：moderation_actions.reason VARCHAR(500)，领域不变量兜底
		return ErrInvalidAction
	}
	r, err := s.repo.GetReportByID(ctx, in.ReportID)
	if err != nil {
		return err
	}
	if r.Status != StatusPending {
		return ErrInvalidAction // 已处理（条件更新再兜一层 F4）
	}
	// 动作与目标类型一致性
	switch in.Action {
	case ActionDeletePost:
		if r.TargetType != "post" {
			return ErrInvalidAction
		}
	case ActionDeleteComment:
		if r.TargetType != "comment" {
			return ErrInvalidAction
		}
	}

	// ---- 事务外：副作用（O1：不在持有 moderation 事务期间调用外部网关） ----
	// 审计目标：delete 记被举报内容；warn 记被举报内容作者（target_type=user）；ban 记被封用户（target_type=user）。
	auditType, auditID := r.TargetType, r.TargetID
	switch in.Action {
	case ActionDeletePost, ActionDeleteComment:
		// 不先 ResolveTarget：已删目标须能幂等重处理（adapter 把「不存在/已删」当成功，O1-②）
		cmd := DeleteTargetCmd{OperatorID: in.HandlerID, OperatorRole: in.OperatorRole, TargetID: r.TargetID}
		if in.Action == ActionDeletePost {
			err = s.content.DeletePost(ctx, cmd)
		} else {
			err = s.content.DeleteComment(ctx, cmd)
		}
		if err != nil {
			return err
		}
	case ActionWarn:
		ref, err := s.content.ResolveTarget(ctx, r.TargetType, r.TargetID)
		if err != nil {
			return err // 目标已不存在则无法归责到作者，保持 pending 由处理人改判
		}
		auditType, auditID = "user", ref.AuthorID
	case ActionBan:
		// F3：ban 原因必填；空 reason 会留下无原因审计，违反治理留痕意图。
		reason := strings.TrimSpace(in.Note)
		if reason == "" {
			return ErrInvalidAction
		}
		// Q3：ban 优先用 TargetAuthorID 快照（内容已删仍可定位作者），与 warn 的 ResolveTarget 刻意不同。
		var banTargetID int64
		switch r.TargetType {
		case "user":
			banTargetID = r.TargetID
		case "post", "comment":
			if r.TargetAuthorID != nil {
				banTargetID = *r.TargetAuthorID
			} else {
				ref, err := s.content.ResolveTarget(ctx, r.TargetType, r.TargetID)
				if err != nil {
					return err // 无快照且目标已删 → 无法定位作者，保持 pending
				}
				banTargetID = ref.AuthorID
			}
		}
		if err := s.users.BanUser(ctx, BanUserCmd{OperatorID: in.HandlerID, TargetID: banTargetID}); err != nil {
			return err // ErrSelfBan/ErrCannotBanAdmin/ErrAlreadyBanned/ErrNotFound 透传
		}
		auditType, auditID = "user", banTargetID
	}

	// ---- 单事务：append 审计 + 改状态（S2/F4） ----
	err = s.repo.Tx(ctx, func(tx Repo) error {
		if in.Action != ActionDismiss {
			reason := strings.TrimSpace(in.Note)
			if reason == "" {
				reason = r.Reason // 无备注时留原因作审计语义（ban 已在前置校验非空，不会走到这里）
			}
			if err := tx.AppendAction(ctx, &ModerationAction{
				ModeratorID: in.HandlerID,
				Action:      in.Action,
				TargetType:  auditType,
				TargetID:    auditID,
				Reason:      reason,
			}); err != nil {
				return err
			}
		}
		return tx.UpdateReportStatus(ctx, r.ID, statusFor(in.Action), in.HandlerID, in.Note)
	})
	if err != nil {
		return err
	}

	// ---- 提交后：通知举报人（report_result）+ 被举报人（report_handled，#53 D1/D2）。
	// 两条都在事务提交后发、各自吞错打日志不回滚（S3），互不阻塞。 ----
	s.notifyReportOutcome(ctx, r, in.Action)
	return nil
}

// notifyReportOutcome 提交后发双通知：举报人 report_result（所有动作含 dismiss 结论）+ 被举报人 report_handled（仅 delete/warn，#53 D2）。
func (s *service) notifyReportOutcome(ctx context.Context, r *Report, action string) {
	title := conclusionFor(action)
	if notifyErr := s.notify.CreateNotification(ctx, NotificationCmd{
		RecipientID: r.ReporterID,
		Type:        "report_result",
		TargetType:  &r.TargetType,
		TargetID:    &r.TargetID,
		TargetTitle: &title,
	}); notifyErr != nil {
		// S3：吞错但必须留日志，否则举报人无声丢失处理结论且无排查线索
		slog.Warn("report_result 通知发送失败", "report_id", r.ID, "recipient", r.ReporterID, "err", notifyErr)
	}

	// 被举报人视角：#53 D2 仅 delete/warn；dismiss 不发。
	if !notifiesReportedUser(action) {
		return
	}
	if r.TargetAuthorID == nil { // 存量 pending（0007 前建）无快照 → 跳过，不 panic
		slog.Warn("举报缺少 target_author_id，跳过被举报人通知", "report_id", r.ID, "action", action)
		return
	}
	// report_handled：target_title 承载整句文案（刻意语义复用，评审 F2）；TargetType/TargetID 随行下发供未来客户端筛选。
	authorTitle := handledConclusionFor(action, r.Reason)
	if notifyErr := s.notify.CreateNotification(ctx, NotificationCmd{
		RecipientID: *r.TargetAuthorID,
		Type:        "report_handled",
		TargetType:  &r.TargetType,
		TargetID:    &r.TargetID,
		TargetTitle: &authorTitle,
	}); notifyErr != nil {
		slog.Warn("report_handled 通知发送失败", "report_id", r.ID, "recipient", *r.TargetAuthorID, "err", notifyErr)
	}
}

// notifiesReportedUser #53 D2：哪些处理动作要通知被举报人（dismiss 不打扰；warn/ban 也通知）。
func notifiesReportedUser(action string) bool {
	switch action {
	case ActionDeletePost, ActionDeleteComment, ActionWarn, ActionBan:
		return true
	default:
		return false
	}
}

// handledConclusionFor 给被举报人的结论句（含原因枚举，作者视角；#53 D2，§6.2 不拼 reporter_note）。
func handledConclusionFor(action, reason string) string {
	switch action {
	case ActionDeletePost, ActionDeleteComment:
		return "你的内容因「" + reason + "」被举报，已删除"
	case ActionWarn:
		return "你因「" + reason + "」被举报，已警告"
	case ActionBan:
		return "你因「" + reason + "」被举报，已被封禁"
	default:
		// 防御：notifiesReportedUser 白名单外动作不调用本函数，但兜底不产生空标题通知（评审 Standards #1）
		return "你的内容已被处理"
	}
}

// ListReports 管理队列列表（S8），逐行 enrich 举报人用户名与目标标题（best-effort，F8-② 记录 N+1）。
func (s *service) ListReports(ctx context.Context, in ListReportsQuery) ([]ReportView, error) {
	reps, err := s.repo.ListReports(ctx, in.Status, in.Limit, in.Offset)
	if err != nil {
		return nil, err
	}
	views := make([]ReportView, 0, len(reps))
	for _, r := range reps {
		views = append(views, s.reportView(ctx, r))
	}
	return views, nil
}

func (s *service) reportView(ctx context.Context, r *Report) ReportView {
	v := ReportView{
		ID:           r.ID,
		ReporterID:   r.ReporterID,
		TargetType:   r.TargetType,
		TargetID:     r.TargetID,
		Reason:       r.Reason,
		ReporterNote: r.ReporterNote,
		Status:       r.Status,
		HandlerID:    r.HandlerID,
		HandledAt:    r.HandledAt,
		HandlingNote: r.HandlingNote,
		CreatedAt:    r.CreatedAt,
	}
	if u, err := s.users.GetUserView(ctx, r.ReporterID); err == nil {
		v.ReporterUsername = u.Username
	}
	if title := s.targetTitle(ctx, r); title != "" {
		v.TargetTitle = title
	}
	return v
}

func (s *service) targetTitle(ctx context.Context, r *Report) string {
	switch r.TargetType {
	case "user":
		if u, err := s.users.GetUserView(ctx, r.TargetID); err == nil {
			return u.Username
		}
	case "post", "comment":
		if ref, err := s.content.ResolveTarget(ctx, r.TargetType, r.TargetID); err == nil {
			return ref.Title
		}
	}
	return ""
}

// ListActions 审计日志只读查询（#60）：逐行 best-effort enrich 操作人用户名（仿 ListReports）。
func (s *service) ListActions(ctx context.Context, in ListActionsQuery) ([]ModerationActionView, error) {
	acts, err := s.repo.ListActions(ctx, in)
	if err != nil {
		return nil, err
	}
	views := make([]ModerationActionView, 0, len(acts))
	for _, a := range acts {
		v := ModerationActionView{
			ID:            a.ID,
			ModeratorID:   a.ModeratorID,
			Action:        a.Action,
			TargetType:    a.TargetType,
			TargetID:      a.TargetID,
			Reason:        a.Reason,
			CreatedAt:     a.CreatedAt,
		}
		if u, err := s.users.GetUserView(ctx, a.ModeratorID); err == nil {
			v.ModeratorUsername = u.Username
		}
		views = append(views, v)
	}
	return views, nil
}

// CountReports 队列计数（Me.vue 待处理徽章，S8）。
func (s *service) CountReports(ctx context.Context, status string) (int64, error) {
	return s.repo.CountReports(ctx, status)
}

// RecordAction 追加独立治理审计（#34：ban/unban 直接动作，独立于举报流程）。
// 校验：动作白名单 + TargetType 恒 user + reason 必填 ≤500（与 handler 预校验同源，评审 Q1 条件 3）。
func (s *service) RecordAction(ctx context.Context, in RecordActionCmd) error {
	if !validRecordAction(in.Action) || in.TargetID <= 0 {
		return ErrInvalidAction
	}
	reason := strings.TrimSpace(in.Reason)
	if reason == "" || len([]rune(reason)) > 500 {
		return ErrInvalidAction
	}
	return s.repo.AppendAction(ctx, &ModerationAction{
		ModeratorID: in.ModeratorID,
		Action:      in.Action,
		TargetType:  "user",
		TargetID:    in.TargetID,
		Reason:      reason,
	})
}
