package moderation

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

// gormRepo 是 Repo 的 GORM 实现（#7 seam 的实现侧）。
type gormRepo struct {
	db *gorm.DB
}

// NewGormRepo 构造 Repo。
func NewGormRepo(db *gorm.DB) Repo { return &gormRepo{db: db} }

func (r *gormRepo) CreateReport(ctx context.Context, rep *Report) error {
	if err := r.db.WithContext(ctx).Create(rep).Error; err != nil {
		if isDuplicatePending(err) {
			return ErrDuplicatePending
		}
		return err
	}
	return nil
}

// isDuplicatePending 仅当唯一约束是 uq_reports_pending 时归为重复举报（S9：匹配约束名，不误吞其他唯一冲突）。
func isDuplicatePending(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505" && pgErr.ConstraintName == "uq_reports_pending"
	}
	return false
}

func (r *gormRepo) GetReportByID(ctx context.Context, id int64) (*Report, error) {
	var rep Report
	if err := r.db.WithContext(ctx).First(&rep, id).Error; err != nil {
		return nil, mapNotFound(err)
	}
	return &rep, nil
}

func mapNotFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	return err
}

// UpdateReportStatus 条件更新：仅 status=pending 时生效；RowsAffected=0 → ErrNotFound（F4 并发双处理防护）。
func (r *gormRepo) UpdateReportStatus(ctx context.Context, id int64, status string, handlerID int64, note string) error {
	res := r.db.WithContext(ctx).Model(&Report{}).
		Where("id = ? AND status = ?", id, StatusPending).
		Updates(map[string]any{
			"status":        status,
			"handler_id":    handlerID,
			"handled_at":    time.Now(),
			"handling_note": note,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *gormRepo) ListReports(ctx context.Context, status string, limit, offset int) ([]*Report, error) {
	q := r.db.WithContext(ctx).Model(&Report{}).Order("created_at DESC")
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if limit > 0 {
		q = q.Limit(limit)
	}
	if offset > 0 {
		q = q.Offset(offset)
	}
	var reps []*Report
	if err := q.Find(&reps).Error; err != nil {
		return nil, err
	}
	return reps, nil
}

func (r *gormRepo) CountReports(ctx context.Context, status string) (int64, error) {
	q := r.db.WithContext(ctx).Model(&Report{})
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var n int64
	if err := q.Count(&n).Error; err != nil {
		return 0, err
	}
	return n, nil
}

func (r *gormRepo) PendingExists(ctx context.Context, reporterID int64, targetType string, targetID int64) (bool, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&Report{}).
		Where("reporter_id = ? AND target_type = ? AND target_id = ? AND status = ?", reporterID, targetType, targetID, StatusPending).
		Count(&n).Error
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// AppendAction 追加审计行（append-only：仅 created_at 由 DB 默认，#5 D6）。
func (r *gormRepo) AppendAction(ctx context.Context, a *ModerationAction) error {
	return r.db.WithContext(ctx).Create(a).Error
}

// Tx 在单事务内执行 fn（S2：HandleReport 的审计+状态原子提交）。
func (r *gormRepo) Tx(ctx context.Context, fn func(Repo) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&gormRepo{db: tx})
	})
}
