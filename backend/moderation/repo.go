package moderation

import "context"

// Repo 是治理域持久化内部 seam（#7）。实现见后续 ticket。
// moderation_actions 仅支持创建，不支持 update/delete（#5 D6：API 不暴露）。
type Repo interface {
	CreateReport(ctx context.Context, r *Report) error
	GetReportByID(ctx context.Context, id int64) (*Report, error)
	UpdateReportStatus(ctx context.Context, id int64, status string, handlerID int64, note string) error
	ListPendingReports(ctx context.Context, limit, offset int) ([]*Report, error)
	PendingExists(ctx context.Context, reporterID int64, targetType string, targetID int64) (bool, error)

	AppendAction(ctx context.Context, a *ModerationAction) error
}
