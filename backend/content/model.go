// Package content 是内容域（#4 服务候选边界）。本脚手架阶段提供模型与接口定义，实现见后续 ticket。
package content

import (
	"time"

	"gorm.io/gorm"
)

// 以下模型映射 content schema 各表（#5）：物理 FK 仅包内，跨 schema 引用（author_id 等）为逻辑外键。
// 故意不加 like_count/comment_count 冗余列（#5：50 人规模读侧 COUNT 零压力，单一事实来源）。

type Board struct {
	ID          int64 `gorm:"primaryKey"`
	Name        string
	Description *string
	ParentID    *int64 // 自引用物理FK，一行支持多级板块
	SortOrder   int
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   gorm.DeletedAt
}

func (Board) TableName() string { return "content.boards" }

type Topic struct {
	ID        int64 `gorm:"primaryKey"`
	BoardID   int64
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt
}

func (Topic) TableName() string { return "content.topics" }

type Post struct {
	ID         int64 `gorm:"primaryKey"`
	BoardID    int64
	TopicID    *int64
	AuthorID   int64 // 逻辑FK → user.users
	Title      string
	Content    string
	IsPinned   bool
	IsFeatured bool
	ViewCount  int
	CreatedAt  time.Time
	UpdatedAt  time.Time
	DeletedAt  gorm.DeletedAt
}

func (Post) TableName() string { return "content.posts" }

// Comment 邻接表评论（#5 D3）：ParentID 指向父评论，Floor 仅顶层 1..N。
type Comment struct {
	ID        int64 `gorm:"primaryKey"`
	PostID    int64
	AuthorID  int64 // 逻辑FK → user.users
	ParentID  *int64
	Floor     *int
	Content   string
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt // 软删留占位节点，回复链保留（#5 D3）
}

func (Comment) TableName() string { return "content.comments" }

type Like struct {
	ID         int64 `gorm:"primaryKey"`
	UserID     int64 // 逻辑FK → user.users
	TargetType string
	TargetID   int64
	CreatedAt  time.Time
}

func (Like) TableName() string { return "content.likes" }

type Favorite struct {
	ID        int64 `gorm:"primaryKey"`
	UserID    int64 // 逻辑FK → user.users
	PostID    int64
	CreatedAt time.Time
}

func (Favorite) TableName() string { return "content.favorites" }

// Tag 标签（#54）：name 为归一化小写（大小写不敏感，D2），UNIQUE。写一次、无软删。
type Tag struct {
	ID        int64 `gorm:"primaryKey"`
	Name      string
	CreatedAt time.Time
}

func (Tag) TableName() string { return "content.tags" }

// PostTag 帖子-标签关联（#54）：解析顺序 = id 升序（读模型保首次出现序）。无软删。
type PostTag struct {
	ID        int64 `gorm:"primaryKey"`
	TagID     int64
	PostID    int64
	CreatedAt time.Time
}

func (PostTag) TableName() string { return "content.post_tags" }

// Mention 提及（#72）：创建时落库（target_type: post|comment 多态，照 likes 先例）。
// mentioned_user_id 为跨包逻辑FK（只建索引不建约束）；mentioned_username 为创建时快照
// （对齐 users.username VARCHAR(64)），读侧零 join。
type Mention struct {
	ID                int64 `gorm:"primaryKey"`
	TargetType        string
	TargetID          int64
	MentionedUserID   int64  // 逻辑FK → user.users(id)
	MentionedUsername string // 创建时快照
	CreatedAt         time.Time
}

func (Mention) TableName() string { return "content.mentions" }

type FollowBoard struct {
	ID         int64 `gorm:"primaryKey"`
	FollowerID int64 // 逻辑FK → user.users
	BoardID    int64
	CreatedAt  time.Time
}

func (FollowBoard) TableName() string { return "content.follows_boards" }

type FollowTopic struct {
	ID         int64 `gorm:"primaryKey"`
	FollowerID int64 // 逻辑FK → user.users
	TopicID    int64
	CreatedAt  time.Time
}

func (FollowTopic) TableName() string { return "content.follows_topics" }
