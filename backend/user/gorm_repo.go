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

func (r *gormRepo) UpdateUserStatus(ctx context.Context, id int64, status string) error {
	return r.db.WithContext(ctx).Model(&User{}).Where("id = ?", id).Update("status", status).Error
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

func mapNotFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	return err
}
