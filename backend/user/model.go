package user

import (
	"time"

	"gorm.io/gorm"
)

// User 映射 user.users（#5）。Role 存 member/moderator/admin；对外（API/JWT）由 mapRole 映射为 user/moderator/admin。
type User struct {
	ID           int64          `gorm:"primaryKey"`
	Email        string         `gorm:"uniqueIndex"`
	Username     string         `gorm:"uniqueIndex"`
	PasswordHash string         `json:"-"`
	AvatarURL    *string
	Bio          *string
	Role         string // member | moderator | admin
	Status       string // active | banned
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    gorm.DeletedAt // 软删（#5 D2：users 永不硬删）
}

func (User) TableName() string { return "user.users" }

// FollowUser 映射 user.follows_users（单向关注，#5 D4）。
type FollowUser struct {
	ID         int64 `gorm:"primaryKey"`
	FollowerID int64
	TargetID   int64
	CreatedAt  time.Time
}

func (FollowUser) TableName() string { return "user.follows_users" }

// InvitationCode 映射 user.invitation_codes（auth-flow §7，已确认）。
// 一次性、审计留痕：UsedBy 非空即视为已使用，记录不删除。
type InvitationCode struct {
	ID        int64 `gorm:"primaryKey"`
	Code      string
	CreatedBy int64
	UsedBy    *int64
	UsedAt    *time.Time
	CreatedAt time.Time
}

func (InvitationCode) TableName() string { return "user.invitation_codes" }
