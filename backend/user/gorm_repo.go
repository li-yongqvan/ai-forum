package user

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

// gormRepo 是 Repo 的 GORM 实现（实现细节，深模块内隐藏）。
type gormRepo struct {
	db *gorm.DB
}

// NewGormRepo 以真实连接构造 Repo。
func NewGormRepo(db *gorm.DB) Repo {
	return &gormRepo{db: db}
}

var _ Repo = (*gormRepo)(nil)

func (r *gormRepo) CreateUser(ctx context.Context, u *User) error {
	return r.db.WithContext(ctx).Create(u).Error
}

func (r *gormRepo) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	var u User
	if err := r.db.WithContext(ctx).Where("username = ?", username).First(&u).Error; err != nil {
		return nil, mapNotFound(err)
	}
	return &u, nil
}

func (r *gormRepo) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	var u User
	if err := r.db.WithContext(ctx).Where("email = ?", email).First(&u).Error; err != nil {
		return nil, mapNotFound(err)
	}
	return &u, nil
}

func (r *gormRepo) GetUserByID(ctx context.Context, id int64) (*User, error) {
	var u User
	if err := r.db.WithContext(ctx).First(&u, id).Error; err != nil {
		return nil, mapNotFound(err)
	}
	return &u, nil
}

// SetBanned 条件更新 active→banned（#34：并发双 admin 同时 ban 时 RowsAffected=0 原子地保证仅一方成功）。
func (r *gormRepo) SetBanned(ctx context.Context, id int64) error {
	res := r.db.WithContext(ctx).
		Model(&User{}).
		Where("id = ? AND status = ?", id, "active").
		Update("status", "banned")
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrAlreadyBanned
	}
	return nil
}

// SetActive 条件更新 banned→active（#34）。
func (r *gormRepo) SetActive(ctx context.Context, id int64) error {
	res := r.db.WithContext(ctx).
		Model(&User{}).
		Where("id = ? AND status = ?", id, "banned").
		Update("status", "active")
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotBanned
	}
	return nil
}

func (r *gormRepo) GetInvitationCode(ctx context.Context, code string) (*InvitationCode, error) {
	var ic InvitationCode
	if err := r.db.WithContext(ctx).Where("code = ?", code).First(&ic).Error; err != nil {
		return nil, mapNotFound(err)
	}
	return &ic, nil
}

// MarkInvitationCodeUsed 一次性标记邀请码已用（used_by + used_at 同时写，满足 DB CHECK 约束）。
// 若已被并发标记（RowsAffected=0）返回 ErrInvalidInvite。
func (r *gormRepo) MarkInvitationCodeUsed(ctx context.Context, id int64, usedBy int64) error {
	res := r.db.WithContext(ctx).
		Model(&InvitationCode{}).
		Where("id = ? AND used_by IS NULL", id).
		Updates(map[string]any{"used_by": usedBy, "used_at": time.Now()})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrInvalidInvite
	}
	return nil
}

// Tx 在单事务内执行 fn，fn 收到事务绑定的 Repo（行为一致，供跨表原子写，如 Register）。
func (r *gormRepo) Tx(ctx context.Context, fn func(Repo) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&gormRepo{db: tx})
	})
}

func (r *gormRepo) CreateFollow(ctx context.Context, f *FollowUser) error {
	return r.db.WithContext(ctx).Create(f).Error
}

func (r *gormRepo) DeleteFollow(ctx context.Context, followerID, targetID int64) error {
	return r.db.WithContext(ctx).
		Where("follower_id = ? AND target_id = ?", followerID, targetID).
		Delete(&FollowUser{}).
		Error
}

func (r *gormRepo) FollowExists(ctx context.Context, followerID, targetID int64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&FollowUser{}).
		Where("follower_id = ? AND target_id = ?", followerID, targetID).
		Count(&count).Error
	return count > 0, err
}

func (r *gormRepo) ListFollowedUserIDs(ctx context.Context, followerID int64) ([]int64, error) {
	ids := make([]int64, 0)
	err := r.db.WithContext(ctx).
		Model(&FollowUser{}).
		Where("follower_id = ?", followerID).
		Pluck("target_id", &ids).Error
	return ids, err
}

func (r *gormRepo) CountFollowers(ctx context.Context, targetID int64) (int, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&FollowUser{}).Where("target_id = ?", targetID).Count(&n).Error
	return int(n), err
}

func (r *gormRepo) CountFollowing(ctx context.Context, followerID int64) (int, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&FollowUser{}).Where("follower_id = ?", followerID).Count(&n).Error
	return int(n), err
}

// ListFollowedUsers 我关注的用户，按关注时间倒序 + 分页（#23）。
// 显式 Select("user".users.*)：GORM 默认 SELECT * 会把 JOIN 表同名列（id/created_at）覆盖主表（列遮蔽）。
// "user" 是保留字，JOIN/Select 均需双引号限定。
func (r *gormRepo) ListFollowedUsers(ctx context.Context, followerID int64, offset, limit int) ([]*User, error) {
	var users []*User
	q := r.db.WithContext(ctx).
		Model(&User{}).
		Select(`"user".users.*`).
		Joins(`JOIN "user".follows_users fu ON fu.target_id = "user".users.id AND fu.follower_id = ?`, followerID).
		Order("fu.created_at DESC").
		Order("fu.id DESC") // 同刻决胜，确定性
	if limit > 0 {
		q = q.Limit(limit)
	}
	if offset > 0 {
		q = q.Offset(offset)
	}
	if err := q.Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

func mapNotFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	return err
}
