package content

import (
	"context"
	"errors"
	"sort"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

// ListPosts 信息流/板块/话题/作者列表查询。排序：默认 #9 §5.2「置顶优先 + 时间倒序」；Feed=hot 时 #61「置顶优先 + 热度降序（赞×1+评论×3）+ 时间倒序兜底」。
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
	if in.Tag != nil {
		// #54 标签过滤：子查询（主查询无 JOIN → 无列遮蔽；软删帖由主查询 deleted_at IS NULL scope 自动排除）
		q = q.Where("id IN (SELECT pt.post_id FROM content.post_tags pt JOIN content.tags t ON t.id = pt.tag_id WHERE t.name = ?)", *in.Tag)
	}
	if in.Feed == "follow" {
		// 关注来源聚合：作者/板块/话题任一命中；空集合 → IN (NULL) → 恒 false，正确返回空
		q = q.Where("author_id IN ? OR board_id IN ? OR topic_id IN ?",
			in.FollowedUserIDs, in.FollowedBoardIDs, in.FollowedTopicIDs)
	}
	if in.Feed == "hot" {
		// #61 热门流：7 天窗口，热度分 = 赞×1 + 评论×3（likes/comments 不落库，LEFT JOIN 现场聚合）；
		// 窗口内帖子随热度排序，置顶帖仍恒顶（与全部流行为一致）。评论软删由子查询 deleted_at IS NULL 排除。
		// 注意：子查询必须预聚合；若直接 JOIN 原始 likes/comments 两表，会因笛卡尔积导致计数互乘。
		q = q.Select("content.posts.*") // 限定主表列，防 JOIN 表列遮蔽（F8）
		q = q.Where("created_at >= ?", time.Now().Add(-7*24*time.Hour))
		q = q.Joins("LEFT JOIN (SELECT target_id AS pid, COUNT(*) AS like_cnt FROM content.likes WHERE target_type = 'post' GROUP BY target_id) hot_likes ON hot_likes.pid = content.posts.id")
		q = q.Joins("LEFT JOIN (SELECT post_id AS pid, COUNT(*) AS cmt_cnt FROM content.comments WHERE deleted_at IS NULL GROUP BY post_id) hot_comments ON hot_comments.pid = content.posts.id")
	}
	q = q.Order("is_pinned DESC")
	if in.Feed == "hot" {
		// 热度分降序；同分按时间倒序兜底。详见 fake_repo_test.go 的 commentWeight 常量。
		q = q.Order("(COALESCE(hot_likes.like_cnt, 0) + 3 * COALESCE(hot_comments.cmt_cnt, 0)) DESC")
	}
	q = q.Order("created_at DESC")
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
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&Post{}, id).Error; err != nil { // 软删
			return err
		}
		// 顺带硬删该帖 post_tags + mentions（软删不触发 FK CASCADE，#54/#72 卫生）
		if err := tx.Where("post_id = ?", id).Delete(&PostTag{}).Error; err != nil {
			return err
		}
		return tx.Where("target_type = ? AND target_id = ?", "post", id).Delete(&Mention{}).Error
	})
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

// DeleteComment 软删留占位 + 顺带硬删该评论 mentions（#72 卫生，照 DeletePost 事务形态）。
func (r *gormRepo) DeleteComment(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&Comment{}, id).Error; err != nil { // 软删留占位
			return err
		}
		return tx.Where("target_type = ? AND target_id = ?", "comment", id).Delete(&Mention{}).Error
	})
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

// ---- 收藏/关注列表（#23） ----

// 列表按关系时间倒序（favorites/follows_*.created_at），需 JOIN 关系表而非 PostQuery 子查询
// （PostQuery 只能按帖子 created_at 排序）。显式 Select(主表.*)：GORM 默认 SELECT * 会把
// JOIN 表同名列（id/created_at）覆盖主表字段（列遮蔽），必须限定主表列。

func (r *gormRepo) ListFavoritedPosts(ctx context.Context, userID int64, offset, limit int) ([]*Post, error) {
	var posts []*Post
	q := r.db.WithContext(ctx).
		Model(&Post{}).
		Select("content.posts.*").
		Joins("JOIN content.favorites f ON f.post_id = content.posts.id AND f.user_id = ?", userID).
		Order("f.created_at DESC").
		Order("f.id DESC") // 同刻决胜，确定性
	if limit > 0 {
		q = q.Limit(limit)
	}
	if offset > 0 {
		q = q.Offset(offset)
	}
	if err := q.Find(&posts).Error; err != nil {
		return nil, err
	}
	return posts, nil
}

func (r *gormRepo) ListFollowedBoards(ctx context.Context, followerID int64, offset, limit int) ([]*Board, error) {
	var boards []*Board
	q := r.db.WithContext(ctx).
		Model(&Board{}).
		Select("content.boards.*").
		Joins("JOIN content.follows_boards fb ON fb.board_id = content.boards.id AND fb.follower_id = ?", followerID).
		Order("fb.created_at DESC").
		Order("fb.id DESC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	if offset > 0 {
		q = q.Offset(offset)
	}
	if err := q.Find(&boards).Error; err != nil {
		return nil, err
	}
	return boards, nil
}

func (r *gormRepo) ListFollowedTopics(ctx context.Context, followerID int64, offset, limit int) ([]*Topic, error) {
	var topics []*Topic
	q := r.db.WithContext(ctx).
		Model(&Topic{}).
		Select("content.topics.*").
		Joins("JOIN content.follows_topics ft ON ft.topic_id = content.topics.id AND ft.follower_id = ?", followerID).
		Order("ft.created_at DESC").
		Order("ft.id DESC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	if offset > 0 {
		q = q.Offset(offset)
	}
	if err := q.Find(&topics).Error; err != nil {
		return nil, err
	}
	return topics, nil
}

// ---- 标签（#54） ----

// ReplacePostTags 事务内替换帖子标签关联：删旧关联 → upsert 标签（ON CONFLICT DO NOTHING，并发安全）→ 批量写关联。
func (r *gormRepo) ReplacePostTags(ctx context.Context, postID int64, tagNames []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("post_id = ?", postID).Delete(&PostTag{}).Error; err != nil {
			return err
		}
		if len(tagNames) == 0 {
			return nil
		}
		// 稳定锁序：按标签名排序 upsert（降低多标签并发插入的死锁窗口 F8）；post_tags 仍按原解析顺序写
		sorted := append([]string(nil), tagNames...)
		sort.Strings(sorted)
		idByName := make(map[string]int64, len(sorted))
		for _, name := range sorted {
			t := Tag{Name: name}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&t).Error; err != nil {
				return err
			}
			var existing Tag
			if err := tx.Where("name = ?", name).First(&existing).Error; err != nil {
				return err
			}
			idByName[name] = existing.ID
		}
		pts := make([]PostTag, 0, len(tagNames))
		for _, name := range tagNames {
			pts = append(pts, PostTag{TagID: idByName[name], PostID: postID})
		}
		return tx.Create(&pts).Error
	})
}

// ListTagsByPostIDs 批量取多帖标签（读模型富化，#54）。顺序 = post_tags.id 升序（正文首次出现序）。
func (r *gormRepo) ListTagsByPostIDs(ctx context.Context, postIDs []int64) (map[int64][]string, error) {
	if len(postIDs) == 0 {
		return map[int64][]string{}, nil
	}
	type tagRow struct {
		PostID int64
		Name   string
	}
	var rows []tagRow
	if err := r.db.WithContext(ctx).
		Table("content.post_tags").
		Select("content.post_tags.post_id, content.tags.name").
		Joins("JOIN content.tags ON content.tags.id = content.post_tags.tag_id").
		Where("content.post_tags.post_id IN ?", postIDs).
		Order("content.post_tags.id ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[int64][]string, len(rows))
	for _, row := range rows {
		out[row.PostID] = append(out[row.PostID], row.Name)
	}
	return out, nil
}

// ---- 提及（#72） ----

// CreateMentions 批量插入提及关联（ON CONFLICT DO NOTHING 幂等：#72 去重兜底，UNIQUE 冲突静默跳过）。
func (r *gormRepo) CreateMentions(ctx context.Context, mentions []*Mention) error {
	if len(mentions) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&mentions).Error
}

// ListMentionsByTargets 批量取目标（post 或 comment）的提及（读模型富化，#72）。
// 顺序 = id 升序（正文首次出现序，照 ListTagsByPostIDs）。
func (r *gormRepo) ListMentionsByTargets(ctx context.Context, targetType string, targetIDs []int64) (map[int64][]*Mention, error) {
	if len(targetIDs) == 0 {
		return map[int64][]*Mention{}, nil
	}
	var rows []*Mention
	if err := r.db.WithContext(ctx).
		Where("target_type = ? AND target_id IN ?", targetType, targetIDs).
		Order("id ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[int64][]*Mention, len(rows))
	for _, m := range rows {
		out[m.TargetID] = append(out[m.TargetID], m)
	}
	return out, nil
}

// DeleteMentionsByTarget 删除某目标（post/comment）的全部提及关联（软删清理，#72 D6）。
func (r *gormRepo) DeleteMentionsByTarget(ctx context.Context, targetType string, targetID int64) error {
	return r.db.WithContext(ctx).
		Where("target_type = ? AND target_id = ?", targetType, targetID).
		Delete(&Mention{}).Error
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
