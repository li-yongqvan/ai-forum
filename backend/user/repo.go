package user

import "context"

// Repo 是 user 包访问持久化的内部 seam（#7：实现私有部件，不属于 Service 接口）。
// 调用方（httpapi）不感知 Repo；测试用内存 fake 跨 seam 验证 Service（testing.md §2.1）。
type Repo interface {
	CreateUser(ctx context.Context, u *User) error
	GetUserByUsername(ctx context.Context, username string) (*User, error)
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	GetUserByID(ctx context.Context, id int64) (*User, error)
	// SetBanned 条件更新 active→banned；RowsAffected=0 → ErrAlreadyBanned（幂等防重复审计，#34）。
	// 注意：RowsAffected=0 不区分「已封禁」与「不存在」——存在性由调用方先 GetUserByID 保证（评审 Q3 附）。
	SetBanned(ctx context.Context, id int64) error
	// SetActive 条件更新 banned→active；RowsAffected=0 → ErrNotBanned（#34）。
	SetActive(ctx context.Context, id int64) error

	GetInvitationCode(ctx context.Context, code string) (*InvitationCode, error)
	MarkInvitationCodeUsed(ctx context.Context, id int64, usedBy int64) error

	CreateFollow(ctx context.Context, f *FollowUser) error
	DeleteFollow(ctx context.Context, followerID, targetID int64) error
	FollowExists(ctx context.Context, followerID, targetID int64) (bool, error)
	ListFollowedUserIDs(ctx context.Context, followerID int64) ([]int64, error)
	CountFollowers(ctx context.Context, targetID int64) (int, error)
	CountFollowing(ctx context.Context, followerID int64) (int, error)
	// ListFollowedUsers 我关注的用户，按关注时间倒序 + 分页（#23，独立 JOIN 查询形态）。
	ListFollowedUsers(ctx context.Context, followerID int64, offset, limit int) ([]*User, error)

	// Tx 在单事务内执行 fn，fn 收到事务绑定的 Repo。供跨表原子写（如注册：建用户 + 标记邀请码）。
	Tx(ctx context.Context, fn func(Repo) error) error
}
