package content

import (
	"context"
	"time"
)

// ---- 命令 / 读模型 DTO ----

type CreatePostCmd struct {
	AuthorID int64
	BoardID  int64
	TopicID  *int64
	Title    string
	Content  string
}

type PostView struct {
	ID         int64     `json:"id"`
	BoardID    int64     `json:"board_id"`
	TopicID    *int64    `json:"topic_id"`
	AuthorID   int64     `json:"author_id"`
	Title      string    `json:"title"`
	Content    string    `json:"content"`
	IsPinned   bool      `json:"is_pinned"`
	IsFeatured bool      `json:"is_featured"`
	ViewCount  int       `json:"view_count"`
	CreatedAt  time.Time `json:"created_at"`
}

type CreateCommentCmd struct {
	PostID   int64
	AuthorID int64
	ParentID *int64
	Content  string
}

type CommentView struct {
	ID        int64     `json:"id"`
	PostID    int64     `json:"post_id"`
	AuthorID  int64     `json:"author_id"`
	ParentID  *int64    `json:"parent_id"`
	Floor     *int      `json:"floor"`
	Content   string    `json:"content"`
	Deleted   bool      `json:"deleted"` // 软删占位（回复链保留）
	CreatedAt time.Time `json:"created_at"`
}

type LikeCmd struct {
	UserID     int64
	TargetType string // post | comment
	TargetID   int64
}

type FavoriteCmd struct {
	UserID int64
	PostID int64
}

type ListFeedQuery struct {
	Feed     string // all | follow
	UserID   int64  // follow 流需要
	Page     int
	PageSize int
}

type CommentTreeView struct {
	PostID   int64         `json:"post_id"`
	Comments []CommentNode `json:"comments"`
}

type CommentNode struct {
	CommentView
	Replies []CommentNode `json:"replies,omitempty"`
}

type FollowBoardCmd struct {
	FollowerID int64
	BoardID    int64
}

type FollowTopicCmd struct {
	FollowerID int64
	TopicID    int64
}

// Service 是内容域的对外接口（粗粒度命令 + 读模型查询，#4/#7）。
// 排序：全部 = 置顶优先（按操作时间倒序）+ 其余按发布时间倒序；关注 = 关注来源（用户/板块/话题）按发布时间倒序（IA v2 §5.2）。
type Service interface {
	CreatePost(ctx context.Context, in CreatePostCmd) (PostView, error)
	CreateComment(ctx context.Context, in CreateCommentCmd) (CommentView, error)
	Like(ctx context.Context, in LikeCmd) error
	Favorite(ctx context.Context, in FavoriteCmd) error
	GetPost(ctx context.Context, id int64) (PostView, error)
	ListFeed(ctx context.Context, in ListFeedQuery) ([]PostView, error)
	GetCommentTree(ctx context.Context, postID int64) (CommentTreeView, error)
	FollowBoard(ctx context.Context, in FollowBoardCmd) error
	FollowTopic(ctx context.Context, in FollowTopicCmd) error
}
