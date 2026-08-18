package content

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

// gormRepo 是 Repo 的 GORM 实现（实现细节，深模块内隐藏）。
type gormRepo struct {
	db *gorm.DB
}

// NewGormRepo 以真实连接构造 Repo。
func NewGormRepo(db *gorm.DB) Repo {
	return &gormRepo{db: db}
}

var _ Repo = (*gormRepo)(nil)

func mapNotFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return errNotFound
	}
	return err
}

var errNotFound = errors.New("content: 记录不存在")

// ---- boards / topics ----

func (r *gormRepo) GetBoardByID(ctx context.Context, id int64) (*Board, error) {
	var b Board
	if err := r.db.WithContext(ctx).First(&b, id).Error; err != nil {
		return nil, mapNotFound(err)
	}
	return &b, nil
}

func (r *gormRepo) GetTopicByID(ctx context.Context, id int64) (*Topic, error) {
	var t Topic
	if err := r.db.WithContext(ctx).First(&t, id).Error; err != nil {
		return nil, mapNotFound(err)
	}
	return &t, nil
}

func (r *gormRepo) ListBoards(ctx context.Context) ([]*Board, error) {
	var boards []*Board
	if err := r.db.WithContext(ctx).Order("sort_order ASC").Find(&boards).Error; err != nil {
		return nil, err
	}
	return boards, nil
}

func (r *gormRepo) ListTopics(ctx context.Context, boardID *int64) ([]*Topic, error) {
	q := r.db.WithContext(ctx).Model(&Topic{})
	if boardID != nil {
		q = q.Where("board_id = ?", *boardID)
	}
	var topics []*Topic
	if err := q.Order("id ASC").Find(&topics).Error; err != nil {
		return nil, err
	}
	return topics, nil
}

// ---- posts ----

func (r *gormRepo) CreatePost(ctx context.Context, p *Post) error {
	return r.db.WithContext(ctx).Create(p).Error
}

func (r *gormRepo) GetPostByID(ctx context.Context, id int64) (*Post, error) {
	var p Post
	if err := r.db.WithContext(ctx).First(&p, id).Error; err != nil {
		return nil, mapNotFound(err)
	}
	return &p, nil
}

// ListPosts 信息流/板块/话题/作者列表查询。排序：#9 §5.2「置顶优先 + 时间倒序」（无 pinned_at 列，置顶帖按创建时间排序）。
func (r *gormRepo) ListPosts(ctx context.Context, in PostQuery) ([]*Post, error) {
	q := r.db.WithContext(ctx).Model(&Post{})
	if in.AuthorID != nil {
		q = q.Where("author_id = ?", *in.AuthorID)
	}
	if in.BoardID != nil {
		q = q.Where("board_id = ?", *in.BoardID)
	}
	if in.TopicID != nil {
		q = q.Where("topic_id = ?", *in.TopicID)
	}
	if in.Feed == "follow" {
		// 关注来源聚合：作者/板块/话题任一命中；空集合 → IN (NULL) → 恒 false，正确返回空
		q = q.Where("author_id IN ? OR board_id IN ? OR topic_id IN ?",
			in.FollowedUserIDs, in.FollowedBoardIDs, in.FollowedTopicIDs)
	}
	q = q.Order("is_pinned DESC").Order("created_at DESC")
	if in.Limit > 0 {
		q = q.Limit(in.Limit)
	}
	if in.Offset > 0 {
		q = q.Offset(in.Offset)
	}
	var posts []*Post
	if err := q.Find(&posts).Error; err != nil {
		return nil, err
	}
	return posts, nil
}

func (r *gormRepo) DeletePost(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&Post{}, id).Error // 软删
}

func (r *gormRepo) CountPostsByAuthor(ctx context.Context, authorID int64) (int, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&Post{}).Where("author_id = ?", authorID).Count(&n).Error
	return int(n), err
}

func (r *gormRepo) UpdatePostPinned(ctx context.Context, id int64, pinned bool) error {
	return r.db.WithContext(ctx).Model(&Post{}).Where("id = ?", id).Update("is_pinned", pinned).Error
}

func (r *gormRepo) UpdatePostFeatured(ctx context.Context, id int64, featured bool) error {
	return r.db.WithContext(ctx).Model(&Post{}).Where("id = ?", id).Update("is_featured", featured).Error
}

func (r *gormRepo) IncrementViewCount(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Model(&Post{}).Where("id = ?", id).
		UpdateColumn("view_count", gorm.Expr("view_count + 1")).Error
}

// ---- comments ----

func (r *gormRepo) CreateComment(ctx context.Context, c *Comment) error {
	return r.db.WithContext(ctx).Create(c).Error
}

func (r *gormRepo) GetCommentByID(ctx context.Context, id int64) (*Comment, error) {
	var c Comment
	if err := r.db.WithContext(ctx).First(&c, id).Error; err != nil {
		return nil, mapNotFound(err)
	}
	return &c, nil
}

// GetMaxTopFloor 返回帖子顶层评论的最大楼层号（无则 0）。软删评论保留楼层，不回收。
func (r *gormRepo) GetMaxTopFloor(ctx context.Context, postID int64) (int, error) {
	var n int
	err := r.db.WithContext(ctx).
		Model(&Comment{}).
		Where("post_id = ?", postID).
		Select("COALESCE(MAX(floor), 0)").
		Scan(&n).Error
	return n, err
}

// ListCommentsByPost 一次查完整帖评论（含软删占位，#5 D3）。
func (r *gormRepo) ListCommentsByPost(ctx context.Context, postID int64) ([]*Comment, error) {
	var comments []*Comment
	// Unscoped：软删评论也要返回（占位节点 + 回复链）
	if err := r.db.WithContext(ctx).
		Unscoped().
		Where("post_id = ?", postID).
		Order("created_at ASC").
		Find(&comments).Error; err != nil {
		return nil, err
	}
	return comments, nil
}

func (r *gormRepo) DeleteComment(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&Comment{}, id).Error // 软删留占位
}

func (r *gormRepo) CountCommentsByPost(ctx context.Context, postIDs []int64) (map[int64]int, error) {
	var rows []countRow
	err := r.db.WithContext(ctx).
		Model(&Comment{}).
		Select("post_id AS id, COUNT(*) AS cnt").
		Where("post_id IN ?", postIDs).
		Group("post_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return toIDCount(rows), nil
}

// ---- likes ----

func (r *gormRepo) CreateLike(ctx context.Context, l *Like) error {
	return r.db.WithContext(ctx).Create(l).Error
}

func (r *gormRepo) DeleteLike(ctx context.Context, userID int64, targetType string, targetID int64) error {
	return r.db.WithContext(ctx).
		Where("user_id = ? AND target_type = ? AND target_id = ?", userID, targetType, targetID).
		Delete(&Like{}).Error
}

func (r *gormRepo) LikeExists(ctx context.Context, userID int64, targetType string, targetID int64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&Like{}).
		Where("user_id = ? AND target_type = ? AND target_id = ?", userID, targetType, targetID).
		Count(&count).Error
	return count > 0, err
}

func (r *gormRepo) CountLikesByPost(ctx context.Context, postIDs []int64) (map[int64]int, error) {
	var rows []countRow
	err := r.db.WithContext(ctx).
		Model(&Like{}).
		Select("target_id AS id, COUNT(*) AS cnt").
		Where("target_type = ? AND target_id IN ?", "post", postIDs).
		Group("target_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return toIDCount(rows), nil
}

func (r *gormRepo) ListLikedPostIDs(ctx context.Context, userID int64, postIDs []int64) ([]int64, error) {
	ids := make([]int64, 0)
	err := r.db.WithContext(ctx).
		Model(&Like{}).
		Where("user_id = ? AND target_type = ? AND target_id IN ?", userID, "post", postIDs).
		Pluck("target_id", &ids).Error
	return ids, err
}

// ---- favorites ----

func (r *gormRepo) CreateFavorite(ctx context.Context, f *Favorite) error {
	return r.db.WithContext(ctx).Create(f).Error
}

func (r *gormRepo) DeleteFavorite(ctx context.Context, userID, postID int64) error {
	return r.db.WithContext(ctx).
		Where("user_id = ? AND post_id = ?", userID, postID).
		Delete(&Favorite{}).Error
}

func (r *gormRepo) FavoriteExists(ctx context.Context, userID, postID int64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&Favorite{}).
		Where("user_id = ? AND post_id = ?", userID, postID).
		Count(&count).Error
	return count > 0, err
}

func (r *gormRepo) CountFavoritesByPost(ctx context.Context, postIDs []int64) (map[int64]int, error) {
	var rows []countRow
	err := r.db.WithContext(ctx).
		Model(&Favorite{}).
		Select("post_id AS id, COUNT(*) AS cnt").
		Where("post_id IN ?", postIDs).
		Group("post_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return toIDCount(rows), nil
}

func (r *gormRepo) ListFavedPostIDs(ctx context.Context, userID int64, postIDs []int64) ([]int64, error) {
	ids := make([]int64, 0)
	err := r.db.WithContext(ctx).
		Model(&Favorite{}).
		Where("user_id = ? AND post_id IN ?", userID, postIDs).
		Pluck("post_id", &ids).Error
	return ids, err
}

// ---- follows（board / topic） ----

func (r *gormRepo) CreateFollowBoard(ctx context.Context, f *FollowBoard) error {
	return r.db.WithContext(ctx).Create(f).Error
}

func (r *gormRepo) DeleteFollowBoard(ctx context.Context, followerID, boardID int64) error {
	return r.db.WithContext(ctx).
		Where("follower_id = ? AND board_id = ?", followerID, boardID).
		Delete(&FollowBoard{}).Error
}

func (r *gormRepo) FollowBoardExists(ctx context.Context, followerID, boardID int64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&FollowBoard{}).
		Where("follower_id = ? AND board_id = ?", followerID, boardID).
		Count(&count).Error
	return count > 0, err
}

func (r *gormRepo) ListFollowedBoardIDs(ctx context.Context, followerID int64) ([]int64, error) {
	ids := make([]int64, 0)
	err := r.db.WithContext(ctx).
		Model(&FollowBoard{}).
		Where("follower_id = ?", followerID).
		Pluck("board_id", &ids).Error
	return ids, err
}

func (r *gormRepo) CreateFollowTopic(ctx context.Context, f *FollowTopic) error {
	return r.db.WithContext(ctx).Create(f).Error
}

func (r *gormRepo) DeleteFollowTopic(ctx context.Context, followerID, topicID int64) error {
	return r.db.WithContext(ctx).
		Where("follower_id = ? AND topic_id = ?", followerID, topicID).
		Delete(&FollowTopic{}).Error
}

func (r *gormRepo) FollowTopicExists(ctx context.Context, followerID, topicID int64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&FollowTopic{}).
		Where("follower_id = ? AND topic_id = ?", followerID, topicID).
		Count(&count).Error
	return count > 0, err
}

func (r *gormRepo) ListFollowedTopicIDs(ctx context.Context, followerID int64) ([]int64, error) {
	ids := make([]int64, 0)
	err := r.db.WithContext(ctx).
		Model(&FollowTopic{}).
		Where("follower_id = ?", followerID).
		Pluck("topic_id", &ids).Error
	return ids, err
}

// ---- 辅助 ----

// countRow 是分组 COUNT 的统一扫描目标（id = 目标 id，cnt = 计数）。
type countRow struct {
	ID  int64
	Cnt int
}

func toIDCount(rows []countRow) map[int64]int {
	m := make(map[int64]int, len(rows))
	for _, r := range rows {
		m[r.ID] = r.Cnt
	}
	return m
}
