package moderation

import (
	"context"
	"time"
)

// Repo 是治理域持久化内部 seam（#7）。实现见 gorm_repo.go。
// moderation_actions 仅支持创建，不支持 update/delete（#5 D6：API 不暴露）。
type Repo interface {
	CreateReport(ctx context.Context, r *Report) error
	GetReportByID(ctx context.Context, id int64) (*Report, error)
	// UpdateReportStatus 条件更新：仅 status=pending 时生效（F4 并发双处理防护）；RowsAffected=0 → ErrNotFound。
	UpdateReportStatus(ctx context.Context, id int64, status string, handlerID int64, note string) error
	ListReports(ctx context.Context, status string, limit, offset int) ([]*Report, error)
	CountReports(ctx context.Context, status string) (int64, error)
	PendingExists(ctx context.Context, reporterID int64, targetType string, targetID int64) (bool, error)
	// CountReportsSince 统计某举报人在 since 之后发起的举报数（#53 频控窗口计数，全局跨目标、不区分状态）。
	CountReportsSince(ctx context.Context, reporterID int64, since time.Time) (int64, error)

	AppendAction(ctx context.Context, a *ModerationAction) error

	// Tx 提供事务边界（仿 user.Repo.Tx；HandleReport 审计+状态原子提交，#33 S2）。
	Tx(ctx context.Context, fn func(Repo) error) error
}
