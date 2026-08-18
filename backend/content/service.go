package content

import (
	"context"
	"errors"
	"sort"
	"strings"
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

// PostView 帖子读模型（含计数与展示名；IsLiked/IsFaved 依查看者身份）。
type PostView struct {
	ID            int64     `json:"id"`
	BoardID       int64     `json:"board_id"`
	BoardName     string    `json:"board_name"`
	TopicID       *int64    `json:"topic_id"`
	TopicName     *string   `json:"topic_name"`
	AuthorID      int64     `json:"author_id"`
	AuthorName    string    `json:"author_name"`
	AuthorAvatar  *string   `json:"author_avatar"`
	Title         string    `json:"title"`
	Content       string    `json:"content"`
	IsPinned      bool      `json:"is_pinned"`
	IsFeatured    bool      `json:"is_featured"`
	ViewCount     int       `json:"view_count"`
	LikeCount     int       `json:"like_count"`
	CommentCount  int       `json:"comment_count"`
	FavoriteCount int       `json:"favorite_count"`
	IsLiked       bool      `json:"is_liked"`
	IsFaved       bool      `json:"is_faved"`
	CreatedAt     time.Time `json:"created_at"`
}

type CreateCommentCmd struct {
	PostID   int64
	AuthorID int64
	ParentID *int64
	Content  string
}

type CommentView struct {
	ID         int64     `json:"id"`
	PostID     int64     `json:"post_id"`
	AuthorID   int64     `json:"author_id"`
	AuthorName string    `json:"author_name"`
	ParentID   *int64    `json:"parent_id"`
	Floor      *int      `json:"floor"`
	Content    string    `json:"content"`
	Deleted    bool      `json:"deleted"` // 软删占位（回复链保留）
	CreatedAt  time.Time `json:"created_at"`
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

type DeletePostCmd struct {
	OperatorID   int64
	OperatorRole string // user | moderator | admin（来自 JWT，已边界映射）
	PostID       int64
}

type DeleteCommentCmd struct {
	OperatorID   int64
	OperatorRole string
	CommentID    int64
}

type ToggleCmd struct {
	OperatorID   int64
	OperatorRole string
	PostID       int64
}

type ListFeedQuery struct {
	Tab      string // all | follow
	ViewerID int64  // 0 = 游客
	BoardID  *int64
	TopicID  *int64
	Page     int
	PageSize int
}

type GetPostQuery struct {
	PostID   int64
	ViewerID int64 // 0 = 游客
}

type FollowBoardCmd struct {
	FollowerID int64
	BoardID    int64
}

type FollowTopicCmd struct {
	FollowerID int64
	TopicID    int64
}

type BoardView struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
}

type TopicView struct {
	ID      int64  `json:"id"`
	BoardID int64  `json:"board_id"`
	Name    string `json:"name"`
}

type CommentTreeView struct {
	PostID   int64         `json:"post_id"`
	Comments []CommentNode `json:"comments"`
}

type CommentNode struct {
	CommentView
	Replies []CommentNode `json:"replies,omitempty"`
}

// ---- 错误 ----

var (
	ErrBoardNotFound     = errors.New("content: 板块不存在")
	ErrTopicNotFound     = errors.New("content: 话题不存在")
	ErrTopicNotInBoard   = errors.New("content: 话题不属于该板块")
	ErrPostNotFound      = errors.New("content: 帖子不存在或已删除")
	ErrCommentNotFound   = errors.New("content: 评论不存在或已删除")
	ErrParentNotInPost   = errors.New("content: 父评论不属于该帖子")
	ErrAlreadyLiked      = errors.New("content: 已点赞")
	ErrAlreadyFavorited  = errors.New("content: 已收藏")
	ErrAlreadyFollowed   = errors.New("content: 已关注该板块/话题")
	ErrForbidden         = errors.New("content: 无权限执行该操作")
	ErrAuthRequired      = errors.New("content: 需要登录")
	ErrInvalidTargetType = errors.New("content: 目标类型非法")
	ErrInvalidFeedTab    = errors.New("content: 信息流 tab 非法")
	ErrContentEmpty      = errors.New("content: 内容不能为空")
)

// ---- Service 接口（粗粒度命令 + 读模型查询，#4/#7） ----

// UserView 内容域可见的最小作者画像（content 不 import user，见 UserProvider）。
type UserView struct {
	ID        int64
	Username  string
	AvatarURL *string
}

// UserProvider 供内容域读取关注与作者数据（#4 进程内调用；由 httpapi 装配层 adapter 包 user 服务实现）。
type UserProvider interface {
	FollowedUserIDs(ctx context.Context, userID int64) ([]int64, error)
	GetUserView(ctx context.Context, id int64) (UserView, error)
}

// Service 是内容域的对外接口。权限校验在命令内（#9 §5.0：接口独立鉴权，前端可见性只是体验层）。
type Service interface {
	CreatePost(ctx context.Context, in CreatePostCmd) (PostView, error)
	CreateComment(ctx context.Context, in CreateCommentCmd) (CommentView, error)
	Like(ctx context.Context, in LikeCmd) error
	Unlike(ctx context.Context, in LikeCmd) error
	Favorite(ctx context.Context, in FavoriteCmd) error
	Unfavorite(ctx context.Context, in FavoriteCmd) error

	GetPost(ctx context.Context, in GetPostQuery) (PostView, error)
	ListFeed(ctx context.Context, in ListFeedQuery) ([]PostView, error)
	GetCommentTree(ctx context.Context, postID int64) (CommentTreeView, error)
	ListBoards(ctx context.Context) ([]BoardView, error)
	ListTopics(ctx context.Context, boardID *int64) ([]TopicView, error)

	DeletePost(ctx context.Context, in DeletePostCmd) error
	DeleteComment(ctx context.Context, in DeleteCommentCmd) error
	PinPost(ctx context.Context, in ToggleCmd) error
	FeaturePost(ctx context.Context, in ToggleCmd) error

	FollowBoard(ctx context.Context, in FollowBoardCmd) error
	UnfollowBoard(ctx context.Context, in FollowBoardCmd) error
	FollowTopic(ctx context.Context, in FollowTopicCmd) error
	UnfollowTopic(ctx context.Context, in FollowTopicCmd) error
}

// ---- 实现 ----

type service struct {
	repo  Repo
	users UserProvider
}

// NewService 构造内容域服务。
func NewService(repo Repo, users UserProvider) Service {
	return &service{repo: repo, users: users}
}

var _ Service = (*service)(nil)

func canModerate(role string) bool { return role == "moderator" || role == "admin" }

// ---- 命令 ----

func (s *service) CreatePost(ctx context.Context, in CreatePostCmd) (PostView, error) {
	if strings.TrimSpace(in.Title) == "" || strings.TrimSpace(in.Content) == "" {
		return PostView{}, ErrContentEmpty
	}
	if _, err := s.repo.GetBoardByID(ctx, in.BoardID); err != nil {
		return PostView{}, ErrBoardNotFound
	}
	if in.TopicID != nil {
		t, err := s.repo.GetTopicByID(ctx, *in.TopicID)
		if err != nil {
			return PostView{}, ErrTopicNotFound
		}
		if t.BoardID != in.BoardID {
			return PostView{}, ErrTopicNotInBoard
		}
	}
	p := &Post{
		BoardID:  in.BoardID,
		TopicID:  in.TopicID,
		AuthorID: in.AuthorID,
		Title:    strings.TrimSpace(in.Title),
		Content:  in.Content,
	}
	if err := s.repo.CreatePost(ctx, p); err != nil {
		return PostView{}, err
	}
	return s.postView(ctx, p, in.AuthorID)
}

func (s *service) CreateComment(ctx context.Context, in CreateCommentCmd) (CommentView, error) {
	if strings.TrimSpace(in.Content) == "" {
		return CommentView{}, ErrContentEmpty
	}
	if _, err := s.repo.GetPostByID(ctx, in.PostID); err != nil {
		return CommentView{}, ErrPostNotFound
	}
	var (
		parentID *int64
		floor    *int
	)
	if in.ParentID != nil {
		parent, err := s.repo.GetCommentByID(ctx, *in.ParentID)
		if err != nil {
			return CommentView{}, ErrCommentNotFound
		}
		if parent.PostID != in.PostID {
			return CommentView{}, ErrParentNotInPost
		}
		parentID = in.ParentID
	} else {
		// 顶层评论分配楼层号（仅顶层计数，#5 D3）
		f, err := s.repo.GetMaxTopFloor(ctx, in.PostID)
		if err != nil {
			return CommentView{}, err
		}
		n := f + 1
		floor = &n
	}
	c := &Comment{PostID: in.PostID, AuthorID: in.AuthorID, ParentID: parentID, Floor: floor, Content: in.Content}
	if err := s.repo.CreateComment(ctx, c); err != nil {
		return CommentView{}, err
	}
	return s.commentView(ctx, c, nil), nil
}

func (s *service) Like(ctx context.Context, in LikeCmd) error {
	if in.TargetType != "post" && in.TargetType != "comment" {
		return ErrInvalidTargetType
	}
	if in.TargetType == "post" {
		if _, err := s.repo.GetPostByID(ctx, in.TargetID); err != nil {
			return ErrPostNotFound
		}
	} else {
		if _, err := s.repo.GetCommentByID(ctx, in.TargetID); err != nil {
			return ErrCommentNotFound
		}
	}
	exists, err := s.repo.LikeExists(ctx, in.UserID, in.TargetType, in.TargetID)
	if err != nil {
		return err
	}
	if exists {
		return ErrAlreadyLiked
	}
	return s.repo.CreateLike(ctx, &Like{UserID: in.UserID, TargetType: in.TargetType, TargetID: in.TargetID})
}

// Unlike 幂等（UI 防抖兼容）。
func (s *service) Unlike(ctx context.Context, in LikeCmd) error {
	return s.repo.DeleteLike(ctx, in.UserID, in.TargetType, in.TargetID)
}

func (s *service) Favorite(ctx context.Context, in FavoriteCmd) error {
	if _, err := s.repo.GetPostByID(ctx, in.PostID); err != nil {
		return ErrPostNotFound
	}
	exists, err := s.repo.FavoriteExists(ctx, in.UserID, in.PostID)
	if err != nil {
		return err
	}
	if exists {
		return ErrAlreadyFavorited
	}
	return s.repo.CreateFavorite(ctx, &Favorite{UserID: in.UserID, PostID: in.PostID})
}

// Unfavorite 幂等。
func (s *service) Unfavorite(ctx context.Context, in FavoriteCmd) error {
	return s.repo.DeleteFavorite(ctx, in.UserID, in.PostID)
}

func (s *service) DeletePost(ctx context.Context, in DeletePostCmd) error {
	p, err := s.repo.GetPostByID(ctx, in.PostID)
	if err != nil {
		return ErrPostNotFound
	}
	// 作者 或 moderator+（#9 §5.0）
	if p.AuthorID != in.OperatorID && !canModerate(in.OperatorRole) {
		return ErrForbidden
	}
	return s.repo.DeletePost(ctx, in.PostID)
}

func (s *service) DeleteComment(ctx context.Context, in DeleteCommentCmd) error {
	c, err := s.repo.GetCommentByID(ctx, in.CommentID)
	if err != nil {
		return ErrCommentNotFound
	}
	if c.AuthorID != in.OperatorID && !canModerate(in.OperatorRole) {
		return ErrForbidden
	}
	return s.repo.DeleteComment(ctx, in.CommentID)
}

func (s *service) PinPost(ctx context.Context, in ToggleCmd) error {
	if !canModerate(in.OperatorRole) {
		return ErrForbidden
	}
	p, err := s.repo.GetPostByID(ctx, in.PostID)
	if err != nil {
		return ErrPostNotFound
	}
	return s.repo.UpdatePostPinned(ctx, in.PostID, !p.IsPinned)
}

func (s *service) FeaturePost(ctx context.Context, in ToggleCmd) error {
	if !canModerate(in.OperatorRole) {
		return ErrForbidden
	}
	p, err := s.repo.GetPostByID(ctx, in.PostID)
	if err != nil {
		return ErrPostNotFound
	}
	return s.repo.UpdatePostFeatured(ctx, in.PostID, !p.IsFeatured)
}

func (s *service) FollowBoard(ctx context.Context, in FollowBoardCmd) error {
	if _, err := s.repo.GetBoardByID(ctx, in.BoardID); err != nil {
		return ErrBoardNotFound
	}
	exists, err := s.repo.FollowBoardExists(ctx, in.FollowerID, in.BoardID)
	if err != nil {
		return err
	}
	if exists {
		return ErrAlreadyFollowed
	}
	return s.repo.CreateFollowBoard(ctx, &FollowBoard{FollowerID: in.FollowerID, BoardID: in.BoardID})
}

func (s *service) UnfollowBoard(ctx context.Context, in FollowBoardCmd) error {
	return s.repo.DeleteFollowBoard(ctx, in.FollowerID, in.BoardID)
}

func (s *service) FollowTopic(ctx context.Context, in FollowTopicCmd) error {
	if _, err := s.repo.GetTopicByID(ctx, in.TopicID); err != nil {
		return ErrTopicNotFound
	}
	exists, err := s.repo.FollowTopicExists(ctx, in.FollowerID, in.TopicID)
	if err != nil {
		return err
	}
	if exists {
		return ErrAlreadyFollowed
	}
	return s.repo.CreateFollowTopic(ctx, &FollowTopic{FollowerID: in.FollowerID, TopicID: in.TopicID})
}

func (s *service) UnfollowTopic(ctx context.Context, in FollowTopicCmd) error {
	return s.repo.DeleteFollowTopic(ctx, in.FollowerID, in.TopicID)
}

// ---- 读侧 ----

func (s *service) GetPost(ctx context.Context, in GetPostQuery) (PostView, error) {
	p, err := s.repo.GetPostByID(ctx, in.PostID)
	if err != nil {
		return PostView{}, ErrPostNotFound
	}
	// 浏览量自增（#5 view_count；MVP：每次 GET 计数）
	if err := s.repo.IncrementViewCount(ctx, p.ID); err != nil {
		return PostView{}, err
	}
	p.ViewCount++
	return s.postView(ctx, p, in.ViewerID)
}

func (s *service) ListFeed(ctx context.Context, in ListFeedQuery) ([]PostView, error) {
	page := in.Page
	if page < 1 {
		page = 1
	}
	pageSize := in.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	q := PostQuery{BoardID: in.BoardID, TopicID: in.TopicID, Offset: (page - 1) * pageSize, Limit: pageSize}

	switch in.Tab {
	case "", "all":
		q.Feed = "all"
	case "follow":
		if in.ViewerID == 0 {
			return nil, ErrAuthRequired
		}
		q.Feed = "follow"
		// 关注来源聚合：#4 用户关注走 user 包，板块/话题关注走本包 repo
		uids, err := s.users.FollowedUserIDs(ctx, in.ViewerID)
		if err != nil {
			return nil, err
		}
		q.FollowedUserIDs = uids
		bids, err := s.repo.ListFollowedBoardIDs(ctx, in.ViewerID)
		if err != nil {
			return nil, err
		}
		q.FollowedBoardIDs = bids
		tids, err := s.repo.ListFollowedTopicIDs(ctx, in.ViewerID)
		if err != nil {
			return nil, err
		}
		q.FollowedTopicIDs = tids
	default:
		return nil, ErrInvalidFeedTab
	}

	posts, err := s.repo.ListPosts(ctx, q)
	if err != nil {
		return nil, err
	}
	return s.postViews(ctx, posts, in.ViewerID)
}

// GetCommentTree 一次查整帖评论 + 内存拼树（#5 D3：调用方不见树算法）。
func (s *service) GetCommentTree(ctx context.Context, postID int64) (CommentTreeView, error) {
	comments, err := s.repo.ListCommentsByPost(ctx, postID)
	if err != nil {
		return CommentTreeView{}, err
	}
	return s.buildTree(ctx, postID, comments), nil
}

func (s *service) buildTree(ctx context.Context, postID int64, comments []*Comment) CommentTreeView {
	// 作者名批量 enrich（不重复查）
	names := map[int64]string{}
	for _, c := range comments {
		if _, ok := names[c.AuthorID]; ok {
			continue
		}
		if u, err := s.users.GetUserView(ctx, c.AuthorID); err == nil {
			names[c.AuthorID] = u.Username
		}
	}
	byID := map[int64]*Comment{}
	for _, c := range comments {
		byID[c.ID] = c
	}
	children := map[int64][]*Comment{}
	var roots []*Comment
	for _, c := range comments {
		if c.ParentID != nil && byID[*c.ParentID] != nil {
			children[*c.ParentID] = append(children[*c.ParentID], c)
		} else {
			roots = append(roots, c)
		}
	}
	// 顶层按 floor 升序；回复按创建时间升序
	sort.Slice(roots, func(i, j int) bool { return floorLess(roots[i], roots[j]) })
	var build func(c *Comment) CommentNode
	build = func(c *Comment) CommentNode {
		node := CommentNode{CommentView: s.commentView(ctx, c, names)}
		replies := children[c.ID]
		// 回复按创建时间升序；同刻以 ID 决胜，保证确定性（#5 D3 邻接表无强序）
		sort.Slice(replies, func(i, j int) bool {
			if replies[i].CreatedAt.Equal(replies[j].CreatedAt) {
				return replies[i].ID < replies[j].ID
			}
			return replies[i].CreatedAt.Before(replies[j].CreatedAt)
		})
		for _, r := range replies {
			node.Replies = append(node.Replies, build(r))
		}
		return node
	}
	view := CommentTreeView{PostID: postID}
	for _, r := range roots {
		view.Comments = append(view.Comments, build(r))
	}
	return view
}

func floorLess(a, b *Comment) bool {
	if a.Floor == nil {
		return false
	}
	if b.Floor == nil {
		return true
	}
	return *a.Floor < *b.Floor
}

// ListBoards 返回板块列表（按 sort_order 升序）。
func (s *service) ListBoards(ctx context.Context) ([]BoardView, error) {
	boards, err := s.repo.ListBoards(ctx)
	if err != nil {
		return nil, err
	}
	views := make([]BoardView, 0, len(boards))
	for _, b := range boards {
		views = append(views, BoardView{ID: b.ID, Name: b.Name, Description: b.Description})
	}
	return views, nil
}

// ListTopics 返回话题列表；boardID 非空时按板块过滤。
func (s *service) ListTopics(ctx context.Context, boardID *int64) ([]TopicView, error) {
	topics, err := s.repo.ListTopics(ctx, boardID)
	if err != nil {
		return nil, err
	}
	views := make([]TopicView, 0, len(topics))
	for _, t := range topics {
		views = append(views, TopicView{ID: t.ID, BoardID: t.BoardID, Name: t.Name})
	}
	return views, nil
}

// ---- 读模型组装 ----

func (s *service) postView(ctx context.Context, p *Post, viewerID int64) (PostView, error) {
	views, err := s.postViews(ctx, []*Post{p}, viewerID)
	if err != nil {
		return PostView{}, err
	}
	return views[0], nil
}

// postViews 批量组装帖子读模型：#5 无冗余计数列，COUNT 走分组查询 + 一次作者/板块/话题 enrich。
func (s *service) postViews(ctx context.Context, posts []*Post, viewerID int64) ([]PostView, error) {
	if len(posts) == 0 {
		return []PostView{}, nil
	}
	postIDs := make([]int64, 0, len(posts))
	authorIDs := make([]int64, 0, len(posts))
	for _, p := range posts {
		postIDs = append(postIDs, p.ID)
		authorIDs = append(authorIDs, p.AuthorID)
	}

	likeCnt, err := s.repo.CountLikesByPost(ctx, postIDs)
	if err != nil {
		return nil, err
	}
	commentCnt, err := s.repo.CountCommentsByPost(ctx, postIDs)
	if err != nil {
		return nil, err
	}
	favCnt, err := s.repo.CountFavoritesByPost(ctx, postIDs)
	if err != nil {
		return nil, err
	}

	liked, faved := map[int64]bool{}, map[int64]bool{}
	if viewerID != 0 {
		lids, err := s.repo.ListLikedPostIDs(ctx, viewerID, postIDs)
		if err != nil {
			return nil, err
		}
		for _, id := range lids {
			liked[id] = true
		}
		fids, err := s.repo.ListFavedPostIDs(ctx, viewerID, postIDs)
		if err != nil {
			return nil, err
		}
		for _, id := range fids {
			faved[id] = true
		}
	}

	authorNames := map[int64]string{}
	for _, id := range uniqueIDs(authorIDs) {
		if u, err := s.users.GetUserView(ctx, id); err == nil {
			authorNames[id] = u.Username
		}
	}
	boardNames := map[int64]string{}
	if bs, err := s.repo.ListBoards(ctx); err == nil {
		for _, b := range bs {
			boardNames[b.ID] = b.Name
		}
	}
	topicNames := map[int64]string{}
	if ts, err := s.repo.ListTopics(ctx, nil); err == nil {
		for _, t := range ts {
			topicNames[t.ID] = t.Name
		}
	}

	views := make([]PostView, 0, len(posts))
	for _, p := range posts {
		view := PostView{
			ID:            p.ID,
			BoardID:       p.BoardID,
			BoardName:     boardNames[p.BoardID],
			TopicID:       p.TopicID,
			AuthorID:      p.AuthorID,
			AuthorName:    authorNames[p.AuthorID],
			Title:         p.Title,
			Content:       p.Content,
			IsPinned:      p.IsPinned,
			IsFeatured:    p.IsFeatured,
			ViewCount:     p.ViewCount,
			LikeCount:     likeCnt[p.ID],
			CommentCount:  commentCnt[p.ID],
			FavoriteCount: favCnt[p.ID],
			IsLiked:       liked[p.ID],
			IsFaved:       faved[p.ID],
			CreatedAt:     p.CreatedAt,
		}
		if p.TopicID != nil {
			if n, ok := topicNames[*p.TopicID]; ok {
				view.TopicName = &n
			}
		}
		if view.AuthorName == "" {
			view.AuthorName = "已注销"
		}
		views = append(views, view)
	}
	return views, nil
}

// commentView 组装评论读模型；names 为批量预取（nil 则逐条查）。
func (s *service) commentView(ctx context.Context, c *Comment, names map[int64]string) CommentView {
	v := CommentView{
		ID:        c.ID,
		PostID:    c.PostID,
		AuthorID:  c.AuthorID,
		ParentID:  c.ParentID,
		Floor:     c.Floor,
		Content:   c.Content,
		CreatedAt: c.CreatedAt,
	}
	name := names[c.AuthorID]
	if name == "" {
		if u, err := s.users.GetUserView(ctx, c.AuthorID); err == nil {
			name = u.Username
		}
	}
	if name == "" {
		name = "已注销"
	}
	v.AuthorName = name
	if c.DeletedAt.Valid {
		v.Deleted = true
		v.Content = ""
	}
	return v
}

func uniqueIDs(ids []int64) []int64 {
	seen := map[int64]bool{}
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}
