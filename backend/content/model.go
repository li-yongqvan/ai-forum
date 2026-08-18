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
