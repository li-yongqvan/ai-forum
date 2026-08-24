package content

import "context"

// PostQuery 列表查询参数（feed / 板块 / 话题 / 作者 / 标签 复用，分页统一 limit/offset）。
type PostQuery struct {
	Feed             string // all | follow
	AuthorID         *int64
	BoardID          *int64
	TopicID          *int64
	Tag              *string // #54 标签过滤：按归一化小写名匹配
	FollowedUserIDs  []int64 // follow 流：关注的用户
	FollowedBoardIDs []int64 // follow 流：关注的板块
	FollowedTopicIDs []int64 // follow 流：关注的话题
	Offset           int
	Limit            int
}

// Repo 是内容域持久化内部 seam（#7：实现私有部件，不属于 Service 接口）。测试用内存 fake 跨 seam 验证。
type Repo interface {
	// boards / topics
	GetBoardByID(ctx context.Context, id int64) (*Board, error)
	GetTopicByID(ctx context.Context, id int64) (*Topic, error)
	ListBoards(ctx context.Context) ([]*Board, error)
	ListTopics(ctx context.Context, boardID *int64) ([]*Topic, error)

	// posts
	CreatePost(ctx context.Context, p *Post) error
	GetPostByID(ctx context.Context, id int64) (*Post, error)
	ListPosts(ctx context.Context, in PostQuery) ([]*Post, error)
	CountPostsByAuthor(ctx context.Context, authorID int64) (int, error)
	DeletePost(ctx context.Context, id int64) error // 软删
	UpdatePostPinned(ctx context.Context, id int64, pinned bool) error
	UpdatePostFeatured(ctx context.Context, id int64, featured bool) error
	IncrementViewCount(ctx context.Context, id int64) error

	// comments（#5 D3 邻接表）
	CreateComment(ctx context.Context, c *Comment) error
	GetCommentByID(ctx context.Context, id int64) (*Comment, error)
	GetMaxTopFloor(ctx context.Context, postID int64) (int, error)
	ListCommentsByPost(ctx context.Context, postID int64) ([]*Comment, error)
	DeleteComment(ctx context.Context, id int64) error // 软删留占位
	CountCommentsByPost(ctx context.Context, postIDs []int64) (map[int64]int, error)

	// likes
	CreateLike(ctx context.Context, l *Like) error
	DeleteLike(ctx context.Context, userID int64, targetType string, targetID int64) error
	LikeExists(ctx context.Context, userID int64, targetType string, targetID int64) (bool, error)
	CountLikesByPost(ctx context.Context, postIDs []int64) (map[int64]int, error)
	ListLikedPostIDs(ctx context.Context, userID int64, postIDs []int64) ([]int64, error)

	// favorites
	CreateFavorite(ctx context.Context, f *Favorite) error
	DeleteFavorite(ctx context.Context, userID, postID int64) error
	FavoriteExists(ctx context.Context, userID, postID int64) (bool, error)
	CountFavoritesByPost(ctx context.Context, postIDs []int64) (map[int64]int, error)
	ListFavedPostIDs(ctx context.Context, userID int64, postIDs []int64) ([]int64, error)

	// follows（board / topic，单向，#5 D4）
	CreateFollowBoard(ctx context.Context, f *FollowBoard) error
	DeleteFollowBoard(ctx context.Context, followerID, boardID int64) error
	FollowBoardExists(ctx context.Context, followerID, boardID int64) (bool, error)
	ListFollowedBoardIDs(ctx context.Context, followerID int64) ([]int64, error)
	CreateFollowTopic(ctx context.Context, f *FollowTopic) error
	DeleteFollowTopic(ctx context.Context, followerID, topicID int64) error
	FollowTopicExists(ctx context.Context, followerID, topicID int64) (bool, error)
	ListFollowedTopicIDs(ctx context.Context, followerID int64) ([]int64, error)

	// 收藏/关注列表（#23：按关系时间倒序 + 分页；独立 JOIN 查询形态，
	// 非 ListFavedPostIDs 这类"给定 postIDs 求交集"的掩码辅助）
	ListFavoritedPosts(ctx context.Context, userID int64, offset, limit int) ([]*Post, error)
	ListFollowedBoards(ctx context.Context, followerID int64, offset, limit int) ([]*Board, error)
	ListFollowedTopics(ctx context.Context, followerID int64, offset, limit int) ([]*Topic, error)

	// tags（#54）
	ReplacePostTags(ctx context.Context, postID int64, tagNames []string) error
	ListTagsByPostIDs(ctx context.Context, postIDs []int64) (map[int64][]string, error)
}
