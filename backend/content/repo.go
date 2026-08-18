package content

import "context"

// Repo 是内容域持久化内部 seam（#7：实现私有部件，不属于 Service 接口）。实现见后续 ticket。
type Repo interface {
	CreatePost(ctx context.Context, p *Post) error
	GetPostByID(ctx context.Context, id int64) (*Post, error)
	CreateComment(ctx context.Context, c *Comment) error
	GetCommentByID(ctx context.Context, id int64) (*Comment, error)
	// GetCommentTree 一次查完整帖评论、内存拼树（#5 D3：调用方不见树算法）。
	GetCommentTree(ctx context.Context, postID int64) ([]*Comment, error)

	CreateLike(ctx context.Context, l *Like) error
	DeleteLike(ctx context.Context, userID int64, targetType string, targetID int64) error
	LikeExists(ctx context.Context, userID int64, targetType string, targetID int64) (bool, error)

	CreateFavorite(ctx context.Context, f *Favorite) error
	DeleteFavorite(ctx context.Context, userID, postID int64) error
	FavoriteExists(ctx context.Context, userID, postID int64) (bool, error)

	CreateFollowBoard(ctx context.Context, f *FollowBoard) error
	CreateFollowTopic(ctx context.Context, f *FollowTopic) error
}
