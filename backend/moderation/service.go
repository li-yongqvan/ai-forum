package moderation

import (
	"context"
	"time"
)

// 处理动作（对应 moderation_actions.action 枚举，另含不落审计的 dismiss）。
const (
	ActionDismiss       = "dismiss"        // 忽略举报：仅改 report 状态，不追加审计
	ActionDeletePost    = "delete_post"    // 删除内容（帖子）→ 追加审计
	ActionDeleteComment = "delete_comment" // 删除内容（评论）→ 追加审计
	ActionWarn          = "warn"           // 警告用户 → 追加审计
	ActionBan           = "ban_user"       // 封禁用户 → 追加审计（仅 admin，§5.0）
	ActionUnban         = "unban_user"     // 解封用户 → 追加审计（仅 admin，§5.0；下游依赖#1）
)

// ---- 命令 / 读模型 DTO ----

type CreateReportCmd struct {
	ReporterID int64
	TargetType string // user | post | comment
	TargetID   int64
	Reason     string
	Note       string
}

type ReportView struct {
	ID           int64      `json:"id"`
	ReporterID   int64      `json:"reporter_id"`
	TargetType   string     `json:"target_type"`
	TargetID     int64      `json:"target_id"`
	Reason       string     `json:"reason"`
	Status       string     `json:"status"`
	HandlerID    *int64     `json:"handler_id,omitempty"`
	HandledAt    *time.Time `json:"handled_at,omitempty"`
	HandlingNote *string    `json:"handling_note,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

// HandleReportCmd 处理举报（IA v2 §5.6 闭环）。
// 权限动作集（#9 §5.0）：moderator 动作集不含封禁；admin 全动作集——由调用方（httpapi）按角色拦截，服务端再校验一次。
type HandleReportCmd struct {
	ReportID  int64
	HandlerID int64
	Action    string // Action* 常量之一
	Note      string // 处理备注（附加到 moderation_actions.reason）
}

// Service 是治理域的对外接口。
// 处理举报 = 改 report.status（resolved）+ 若执行删/封/警告/解封则追加一条 moderation_actions（审计留痕，#5 D6）。
type Service interface {
	CreateReport(ctx context.Context, in CreateReportCmd) error
	HandleReport(ctx context.Context, in HandleReportCmd) error
	ListPendingReports(ctx context.Context, limit, offset int) ([]ReportView, error)
}
