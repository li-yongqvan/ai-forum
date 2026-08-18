package user

import "context"

// Repo 是 user 包访问持久化的内部 seam（#7：实现私有部件，不属于 Service 接口）。
// 调用方（httpapi）不感知 Repo；测试用内存 fake 跨 seam 验证 Service（testing.md §2.1）。
type Repo interface {
	CreateUser(ctx context.Context, u *User) error
	GetUserByUsername(ctx context.Context, username string) (*User, error)
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	GetUserByID(ctx context.Context, id int64) (*User, error)
	UpdateUserStatus(ctx context.Context, id int64, status string) error

	GetInvitationCode(ctx context.Context, code string) (*InvitationCode, error)
	MarkInvitationCodeUsed(ctx context.Context, id int64, usedBy int64) error

	CreateFollow(ctx context.Context, f *FollowUser) error
	DeleteFollow(ctx context.Context, followerID, targetID int64) error
	FollowExists(ctx context.Context, followerID, targetID int64) (bool, error)

	// Tx 在单事务内执行 fn，fn 收到事务绑定的 Repo。供跨表原子写（如注册：建用户 + 标记邀请码）。
	Tx(ctx context.Context, fn func(Repo) error) error
}
