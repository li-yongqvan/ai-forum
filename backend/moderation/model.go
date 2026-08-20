// Package moderation 是治理域（#4 服务候选边界）。本脚手架阶段提供模型与接口定义，实现见后续 ticket。
package moderation

import "time"

// Report 映射 moderation.reports。
// Status 为 pending/resolved/dismissed（一致性修正：以 IA v2 §5.6 为准，#5 原文 'open' 已废弃）。
type Report struct {
	ID           int64 `gorm:"primaryKey"`
	ReporterID   int64
	TargetType   string // user | post | comment
	TargetID     int64  // 跨 schema 多态、无 FK
	Reason       string // 六枚举之一（D4/O2：只存枚举，备注走 ReporterNote）
	ReporterNote *string // 举报人备注（0006：选「其他」时必填，IA §5.6）
	Status       string
	HandlerID    *int64
	HandledAt    *time.Time
	HandlingNote *string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (Report) TableName() string { return "moderation.reports" }

// ModerationAction 映射 moderation.moderation_actions（append-only 审计日志，仅 created_at 写死不可改）。
// Action 含 unban_user（下游依赖#1：解封必须留痕，#6/#9 遗留标注）。
type ModerationAction struct {
	ID          int64 `gorm:"primaryKey"`
	ModeratorID int64
	Action      string
	TargetType  string // user | post | comment
	TargetID    int64
	Reason      string
	CreatedAt   time.Time
}

func (ModerationAction) TableName() string { return "moderation.moderation_actions" }
