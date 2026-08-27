// Package notify 是通知与私信域（#4 服务候选边界；#5 D5：notify 零依赖、快照自足行）。
// 本脚手架阶段提供模型与接口定义，实现见后续 ticket。
package notify

import "time"

// Notification 映射 notify.notifications。快照值由调用方构造时算好传入，notify 收下即存、从不读别的表（#4 D4 硬约束）。
type Notification struct {
	ID          int64 `gorm:"primaryKey"`
	RecipientID int64
	Type        string // follow | like | comment | reply | report_result | report_handled | mention
	ActorID     *int64 // 仅客户端跳转，永不 join
	ActorName   *string
	ActorAvatar *string
	TargetType  *string
	TargetID    *int64
	TargetTitle *string
	IsRead      bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (Notification) TableName() string { return "notify.notifications" }

// Message 映射 notify.messages（MVP 不删）。
type Message struct {
	ID         int64 `gorm:"primaryKey"`
	FromUserID int64
	ToUserID   int64
	Content    string
	IsRead     bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (Message) TableName() string { return "notify.messages" }

// Conversation 是会话聚合读模型（repo 内部用，按对端分组）。
type Conversation struct {
	PeerID      int64
	LastMessage Message
	UnreadCount int64
}
