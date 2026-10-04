package content

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// ---- 命令 / 读模型 DTO ----

type CreatePostCmd struct {
	AuthorID int64
	BoardID  int64
	TopicID  *int64
	Title    string
	Content  string
}

// PostView 帖子读模型（含计数与展示名；viewer 字段提供赞/藏/关注初始态，v2 评审定稿形态）。
type PostView struct {
	ID            int64         `json:"id"`
	BoardID       int64         `json:"board_id"`
	BoardName     string        `json:"board_name"`
	TopicID       *int64        `json:"topic_id"`
	TopicName     *string       `json:"topic_name"`
	Tags          []string      `json:"tags,omitempty"`     // #54 正文标签（归一化小写；无则省略）
	Mentions      []MentionView `json:"mentions,omitempty"` // #72 提及（正文里有效 @username 列表，前端据此渲染链接）
	AuthorID      int64         `json:"author_id"`
	AuthorName    string        `json:"author_name"`
	AuthorAvatar  *string       `json:"author_avatar"`
	Title         string        `json:"title"`
	Content       string        `json:"content"`
	IsPinned      bool          `json:"is_pinned"`
	IsFeatured    bool          `json:"is_featured"`
	ViewCount     int           `json:"view_count"`
	LikeCount     int           `json:"like_count"`
	CommentCount  int           `json:"comment_count"`
	FavoriteCount int           `json:"favorite_count"`
	Viewer        *PostViewer   `json:"viewer,omitempty"` // 仅登录态附（含 liked/favorited/following_author）
	CreatedAt     time.Time     `json:"created_at"`
}

// PostViewer 登录态查看者对帖子的操作状态（前端渲染按钮初始态，避免逐项补请求）。
type PostViewer struct {
	Liked           bool `json:"liked"`
	Favorited       bool `json:"favorited"`
	FollowingAuthor bool `json:"following_author"`
}

// FollowViewer 板块/话题/用户粒度的关注初始态。
type FollowViewer struct {
	Following bool `json:"following"`
}

type CreateCommentCmd struct {
	PostID   int64
	AuthorID int64
	ParentID *int64
	Content  string
}

type CommentView struct {
	ID         int64         `json:"id"`
	PostID     int64         `json:"post_id"`
	AuthorID   int64         `json:"author_id"`
	AuthorName string        `json:"author_name"`
	ParentID   *int64        `json:"parent_id"`
	Floor      *int          `json:"floor"`
	Content    string        `json:"content"`
	Deleted    bool          `json:"deleted"`            // 软删占位（回复链保留）
	Mentions   []MentionView `json:"mentions,omitempty"` // #72 提及（有效 @username 列表）
	CreatedAt  time.Time     `json:"created_at"`
}

// MentionView 提及读模型（#72）：user_id 供跳转（/user/:id），username 供渲染文本。
type MentionView struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
}

// PostMetaView 帖子最小元数据（治理域 enrich/处理用，#33 S4：复用 repo.GetPostByID，
// 不触发 view_count 自增——GetPost 有浏览副作用，不能用于治理侧查询）。
type PostMetaView struct {
	AuthorID int64
	Title    string
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
	Tab      string // all | hot | follow（#61：hot = 热度流，公开，游客可见）
	ViewerID int64  // 0 = 游客
	AuthorID *int64 // 按作者过滤（用户主页/我的帖子）
	BoardID  *int64
	TopicID  *int64
	Tag      *string // #54 标签过滤（归一化小写名）
	Query    string  // #78 搜索词（原样字符串：未 trim、未切词、未转义；归一化在本 seam 层）
	Page     int
	PageSize int
}

// ListFavoritesQuery 我收藏的帖子列表（#23）。
type ListFavoritesQuery struct {
	ViewerID int64 // 0 = 游客
	Page     int
	PageSize int
}

// ListFollowedBoardsQuery 我关注的板块列表（#23）。
type ListFollowedBoardsQuery struct {
	ViewerID int64
	Page     int
	PageSize int
}

// ListFollowedTopicsQuery 我关注的话题列表（#23）。
type ListFollowedTopicsQuery struct {
	ViewerID int64
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
	ID          int64         `json:"id"`
	Name        string        `json:"name"`
	Description *string       `json:"description"`
	Viewer      *FollowViewer `json:"viewer,omitempty"`
}

type TopicView struct {
	ID      int64         `json:"id"`
	BoardID int64         `json:"board_id"`
	Name    string        `json:"name"`
	Viewer  *FollowViewer `json:"viewer,omitempty"`
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
	ErrBoardNotFound         = errors.New("content: 板块不存在")
	ErrTopicNotFound         = errors.New("content: 话题不存在")
	ErrTopicNotInBoard       = errors.New("content: 话题不属于该板块")
	ErrPostNotFound          = errors.New("content: 帖子不存在或已删除")
	ErrCommentNotFound       = errors.New("content: 评论不存在或已删除")
	ErrParentNotInPost       = errors.New("content: 父评论不属于该帖子")
	ErrAlreadyLiked          = errors.New("content: 已点赞")
	ErrAlreadyFavorited      = errors.New("content: 已收藏")
	ErrAlreadyFollowed       = errors.New("content: 已关注该板块/话题")
	ErrForbidden             = errors.New("content: 无权限执行该操作")
	ErrAuthRequired          = errors.New("content: 需要登录")
	ErrInvalidTargetType     = errors.New("content: 目标类型非法")
	ErrInvalidFeedTab        = errors.New("content: 信息流 tab 非法")
	ErrContentEmpty          = errors.New("content: 内容不能为空")
	ErrMentionedUserNotFound = errors.New("content: 被提及用户不存在或不可用")
	ErrInvalidQuery          = errors.New("content: 搜索词不合法") // #78：长度 2..64 码点、词数 ≤4，超出即拒绝（不静默截断）；#81：非法 UTF-8 同样归此哨兵
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
	FollowsUser(ctx context.Context, followerID, targetID int64) (bool, error)
	// GetUserByUsername 按用户名查「存在且 active」的用户（#72）。不存在/封禁/软删 → ErrMentionedUserNotFound。
	GetUserByUsername(ctx context.Context, username string) (UserView, error)
}

// MentionNotificationCmd 提及通知命令（快照字段由调用方算好，#4 D4；经 Notifier seam 到 notify 域）。
type MentionNotificationCmd struct {
	RecipientID int64
	ActorID     int64
	ActorName   string
	ActorAvatar *string
	TargetType  string // 恒 "post"（§6.5：评论提及也统一跳帖子详情）
	TargetID    int64  // 恒为 post_id（前端 /post/:id）
	TargetTitle string
}

// Notifier 供内容域发提及通知（S1：content 不 import notify；由 httpapi 装配层 adapter 实现）。
type Notifier interface {
	NotifyMention(ctx context.Context, in MentionNotificationCmd) error
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
	// GetPostMeta 返回帖子最小元数据（作者 + 标题），不触发浏览量自增（治理域 enrich/处理用，#33 S4）。
	GetPostMeta(ctx context.Context, id int64) (PostMetaView, error)
	// GetComment 按 id 返回单条评论读模型（治理域 enrich/处理用，#33 S4）。
	GetComment(ctx context.Context, id int64) (CommentView, error)
	ListFeed(ctx context.Context, in ListFeedQuery) ([]PostView, error)
	// CountFeed 返回与 ListFeed **同一筛选条件**下的帖子总数（#78 搜索结果条数）。
	// 仅搜索态需要；不参与翻页（分页仍由 ListFeed 的 offset/limit 决定）。
	CountFeed(ctx context.Context, in ListFeedQuery) (int64, error)
	GetCommentTree(ctx context.Context, postID int64) (CommentTreeView, error)
	ListBoards(ctx context.Context, viewerID int64) ([]BoardView, error)
	ListTopics(ctx context.Context, viewerID int64, boardID *int64) ([]TopicView, error)
	CountPostsByAuthor(ctx context.Context, authorID int64) (int, error)

	// 我的收藏/关注列表（#23，按关系时间倒序 + 分页，均需登录）
	ListFavorites(ctx context.Context, in ListFavoritesQuery) ([]PostView, error)
	ListFollowedBoards(ctx context.Context, in ListFollowedBoardsQuery) ([]BoardView, error)
	ListFollowedTopics(ctx context.Context, in ListFollowedTopicsQuery) ([]TopicView, error)

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
	repo   Repo
	users  UserProvider
	notify Notifier
}

// NewService 构造内容域服务（#72：注入 Notifier 供提及通知；content 经 seam 不 import notify）。
func NewService(repo Repo, users UserProvider, notifier Notifier) Service {
	return &service{repo: repo, users: users, notify: notifier}
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
	// #54 标签落库：标签为派生元数据，写失败仅记日志不阻断发帖（评审 §6.4）
	if tags := ParseTags(in.Content); len(tags) > 0 {
		if err := s.repo.ReplacePostTags(ctx, p.ID, tags); err != nil {
			slog.Warn("content: 帖子标签写入失败", "post_id", p.ID, "err", err)
		}
	}
	// #72 提及：解析落库 + 通知（best-effort，不阻断发帖）
	s.processMentions(ctx, p.AuthorID, p.Content, "post", p.ID, p.ID, p.Title)
	return s.postView(ctx, p, in.AuthorID)
}

func (s *service) CreateComment(ctx context.Context, in CreateCommentCmd) (CommentView, error) {
	if strings.TrimSpace(in.Content) == "" {
		return CommentView{}, ErrContentEmpty
	}
	// #72 评论提及需要父帖标题：保留 post（原实现丢弃）
	post, err := s.repo.GetPostByID(ctx, in.PostID)
	if err != nil {
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
	// #72 评论提及：mentions 表 target_id = 评论 id；通知统一跳父帖详情（§6.5）
	s.processMentions(ctx, c.AuthorID, c.Content, "comment", c.ID, post.ID, post.Title)
	return s.commentView(ctx, c, nil, nil), nil
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

// processMentions 解析提及并落库 + 发通知（#72）。best-effort：任何失败仅日志，不阻断主命令。
// 落库失败则跳过通知（无关联可发）；通知失败仅 Warn（照 likes 通知先例留痕）。
func (s *service) processMentions(ctx context.Context, authorID int64, content string, targetType string, targetID int64, notifyTargetID int64, targetTitle string) {
	names := ParseMentions(content)
	if len(names) == 0 {
		return
	}
	// actor 快照（评审 F1）：一次 GetUserView 取 author 的 Username/AvatarURL 填全部通知
	actor, err := s.users.GetUserView(ctx, authorID)
	if err != nil {
		slog.Warn("content: 提及通知 actor 读取失败", "author_id", authorID, "err", err)
		return
	}
	rows := make([]*Mention, 0, len(names))
	seen := make(map[int64]bool, len(names))
	for _, username := range names {
		u, err := s.users.GetUserByUsername(ctx, username)
		if err != nil {
			if errors.Is(err, ErrMentionedUserNotFound) {
				continue // 不存在/非 active → 跳过（D2/D3）
			}
			// 非 not-found DB 错误 → Warn + 跳过该 username（best-effort）
			slog.Warn("content: 提及用户解析失败", "username", username, "err", err)
			continue
		}
		if u.ID == authorID { // 自己 @ 自己不通知（D4）
			continue
		}
		if seen[u.ID] { // 同内容多次 @ 同一人只发一条（D4）
			continue
		}
		seen[u.ID] = true
		rows = append(rows, &Mention{
			TargetType:        targetType,
			TargetID:          targetID,
			MentionedUserID:   u.ID,
			MentionedUsername: username,
		})
	}
	if len(rows) == 0 {
		return
	}
	if err := s.repo.CreateMentions(ctx, rows); err != nil {
		slog.Warn("content: 提及关联写入失败", "target_type", targetType, "target_id", targetID, "err", err)
		return
	}
	for _, m := range rows {
		// 通知统一跳帖子详情（§6.5）：TargetType 恒 "post"，TargetID = notifyTargetID（post_id）
		if err := s.notify.NotifyMention(ctx, MentionNotificationCmd{
			RecipientID: m.MentionedUserID,
			ActorID:     authorID,
			ActorName:   actor.Username,
			ActorAvatar: actor.AvatarURL,
			TargetType:  "post",
			TargetID:    notifyTargetID,
			TargetTitle: targetTitle,
		}); err != nil {
			slog.Warn("content: 提及通知发送失败", "recipient", m.MentionedUserID, "err", err)
		}
	}
}

// toMentionViewsByTarget 把 repo 层 mentions 批量结果转成读模型（#72）。
func toMentionViewsByTarget(m map[int64][]*Mention) map[int64][]MentionView {
	out := make(map[int64][]MentionView, len(m))
	for targetID, rows := range m {
		views := make([]MentionView, 0, len(rows))
		for _, r := range rows {
			views = append(views, MentionView{UserID: r.MentionedUserID, Username: r.MentionedUsername})
		}
		out[targetID] = views
	}
	return out
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

// GetPostMeta 返回帖子最小元数据，不做浏览量自增（#33 S4：治理域 enrich 不能污染统计数据）。
func (s *service) GetPostMeta(ctx context.Context, id int64) (PostMetaView, error) {
	p, err := s.repo.GetPostByID(ctx, id)
	if err != nil {
		return PostMetaView{}, ErrPostNotFound
	}
	return PostMetaView{AuthorID: p.AuthorID, Title: p.Title}, nil
}

// GetComment 按 id 返回单条评论读模型（#33 S4：治理域 enrich 需按 comment id 读，现有 GetCommentTree 只按 post id 聚合）。
func (s *service) GetComment(ctx context.Context, id int64) (CommentView, error) {
	c, err := s.repo.GetCommentByID(ctx, id)
	if err != nil {
		return CommentView{}, ErrCommentNotFound
	}
	return s.commentView(ctx, c, nil, nil), nil
}

func (s *service) ListFeed(ctx context.Context, in ListFeedQuery) ([]PostView, error) {
	q, err := s.buildFeedQuery(ctx, in)
	if err != nil {
		return nil, err
	}
	posts, err := s.repo.ListPosts(ctx, q)
	if err != nil {
		return nil, err
	}
	return s.postViews(ctx, posts, in.ViewerID)
}

// CountFeed 返回与 ListFeed 同一筛选条件下的帖子总数（#78 搜索态结果条数）。
func (s *service) CountFeed(ctx context.Context, in ListFeedQuery) (int64, error) {
	q, err := s.buildFeedQuery(ctx, in)
	if err != nil {
		return 0, err
	}
	return s.repo.CountFeed(ctx, q)
}

// buildFeedQuery 把 ListFeedQuery 归一化为 repo 层的 PostQuery。ListFeed 与 CountFeed 共用此构造器，
// 使「结果条数」与「结果列表」恒为同条件（设计文档 §7-3）；两者仅差 offset/limit，计数忽略之。
func (s *service) buildFeedQuery(ctx context.Context, in ListFeedQuery) (PostQuery, error) {
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
	q := PostQuery{AuthorID: in.AuthorID, BoardID: in.BoardID, TopicID: in.TopicID, Offset: (page - 1) * pageSize, Limit: pageSize}
	if in.Tag != nil {
		// #54 标签归一化（大小写不敏感，D2）：seam 层兜底，不依赖调用方先归一化
		if t := NormalizeTag(*in.Tag); t != "" {
			q.Tag = &t
		}
	}
	if in.Query != "" {
		// #78 搜索词：校验/切词/转义同样在 seam 层兜底，handler 只透传（设计文档 §6-1，与 tag 同构）
		terms, err := NormalizeQuery(in.Query)
		if err != nil {
			return PostQuery{}, err
		}
		q.Terms = terms
	}

	switch in.Tab {
	case "", "all":
		q.Feed = "all"
	case "hot":
		// #61 热门流：公开、游客可见；排序在 repo 层（赞×1+评论×3，7 天窗口）
		q.Feed = "hot"
	case "follow":
		if in.ViewerID == 0 {
			return PostQuery{}, ErrAuthRequired
		}
		q.Feed = "follow"
		// 关注来源聚合：#4 用户关注走 user 包，板块/话题关注走本包 repo
		uids, err := s.users.FollowedUserIDs(ctx, in.ViewerID)
		if err != nil {
			return PostQuery{}, err
		}
		q.FollowedUserIDs = uids
		bids, err := s.repo.ListFollowedBoardIDs(ctx, in.ViewerID)
		if err != nil {
			return PostQuery{}, err
		}
		q.FollowedBoardIDs = bids
		tids, err := s.repo.ListFollowedTopicIDs(ctx, in.ViewerID)
		if err != nil {
			return PostQuery{}, err
		}
		q.FollowedTopicIDs = tids
	default:
		return PostQuery{}, ErrInvalidFeedTab
	}
	return q, nil
}

// ---- 搜索词归一化（#78）----

const (
	minQueryRunes = 2  // 少于 2 码点的搜索词召回噪声过大，一律拒绝（D4）
	maxQueryRunes = 64 // 搜索词总长度上限
	maxQueryWords = 4  // 空白切分后的词数上限（多词 AND）
)

// NormalizeQuery 归一化搜索词：trim → UTF-8 有效性 → 长度 2..64 码点 → 按空白切词 → 词数 ≤4 → 逐词转义
// ILIKE 通配符。返回的词可直接进 pattern，但**不含 `%`**——`%词%` 的包裹属检索形态，归 repo 层。
// 非法 UTF-8 / 长度 / 词数越界一律 ErrInvalidQuery，不静默截断（设计文档 §6-3；#81：非法字节直达 PG
// 会报 SQLSTATE 22021 冒泡成 500，须在此 seam 拦成 400——RuneCountInString 把非法字节按 RuneError
// 计 1 码点，长度校验拦不住如 "\xff\xff" 这类两字节输入）。
func NormalizeQuery(raw string) ([]string, error) {
	s := strings.TrimSpace(raw)
	if !utf8.ValidString(s) {
		return nil, ErrInvalidQuery
	}
	if n := utf8.RuneCountInString(s); n < minQueryRunes || n > maxQueryRunes {
		return nil, ErrInvalidQuery
	}
	words := strings.Fields(s)
	if len(words) > maxQueryWords {
		return nil, ErrInvalidQuery
	}
	terms := make([]string, 0, len(words))
	for _, w := range words {
		terms = append(terms, escapeLikeMeta(w))
	}
	return terms, nil
}

// escapeLikeMeta 转义 ILIKE 的通配符与转义符，让输入按字面匹配（X1）。
// 替换顺序不可调换：先转义 `\`，否则 `%`→`\%` 引入的 `\` 会被二次转义。
func escapeLikeMeta(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "%", `\%`)
	s = strings.ReplaceAll(s, "_", `\_`)
	return s
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
	// #72 评论提及批量 enrich（一次查询，不 N+1）
	commentIDs := make([]int64, 0, len(comments))
	for _, c := range comments {
		commentIDs = append(commentIDs, c.ID)
	}
	mentionsByComment := map[int64][]MentionView{}
	if mm, err := s.repo.ListMentionsByTargets(ctx, "comment", commentIDs); err == nil {
		mentionsByComment = toMentionViewsByTarget(mm)
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
		node := CommentNode{CommentView: s.commentView(ctx, c, names, mentionsByComment)}
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
	// 空树也初始化为空切片：JSON 序列化恒为 "comments":[] 而非 null（契约：comments 恒为数组，
	// 防前端对 null 直接 .reduce() 崩溃）
	view := CommentTreeView{PostID: postID, Comments: []CommentNode{}}
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

// ListBoards 返回板块列表（按 sort_order 升序）；登录态附 viewer.following。
func (s *service) ListBoards(ctx context.Context, viewerID int64) ([]BoardView, error) {
	boards, err := s.repo.ListBoards(ctx)
	if err != nil {
		return nil, err
	}
	views := make([]BoardView, 0, len(boards))
	for _, b := range boards {
		v := BoardView{ID: b.ID, Name: b.Name, Description: b.Description}
		if viewerID != 0 {
			following, err := s.repo.FollowBoardExists(ctx, viewerID, b.ID)
			if err != nil {
				return nil, err
			}
			v.Viewer = &FollowViewer{Following: following}
		}
		views = append(views, v)
	}
	return views, nil
}

// ListTopics 返回话题列表；boardID 非空时按板块过滤；登录态附 viewer.following。
func (s *service) ListTopics(ctx context.Context, viewerID int64, boardID *int64) ([]TopicView, error) {
	topics, err := s.repo.ListTopics(ctx, boardID)
	if err != nil {
		return nil, err
	}
	views := make([]TopicView, 0, len(topics))
	for _, t := range topics {
		v := TopicView{ID: t.ID, BoardID: t.BoardID, Name: t.Name}
		if viewerID != 0 {
			following, err := s.repo.FollowTopicExists(ctx, viewerID, t.ID)
			if err != nil {
				return nil, err
			}
			v.Viewer = &FollowViewer{Following: following}
		}
		views = append(views, v)
	}
	return views, nil
}

// CountPostsByAuthor 返回某作者的帖子数（未删除，供用户主页资料卡）。
func (s *service) CountPostsByAuthor(ctx context.Context, authorID int64) (int, error) {
	return s.repo.CountPostsByAuthor(ctx, authorID)
}

// normalizePage 统一分页归一化（与 ListFeed 内联逻辑一致；page 从 1 起，pageSize 默认 20、上限 100）。
func normalizePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

// ListFavorites 返回我收藏的帖子，按收藏时间倒序 + 分页（#23）。复用 postViews 读模型。
func (s *service) ListFavorites(ctx context.Context, in ListFavoritesQuery) ([]PostView, error) {
	if in.ViewerID == 0 {
		return nil, ErrAuthRequired
	}
	page, pageSize := normalizePage(in.Page, in.PageSize)
	posts, err := s.repo.ListFavoritedPosts(ctx, in.ViewerID, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, err
	}
	return s.postViews(ctx, posts, in.ViewerID)
}

// ListFollowedBoards 返回我关注的板块，按关注时间倒序 + 分页（#23）。
func (s *service) ListFollowedBoards(ctx context.Context, in ListFollowedBoardsQuery) ([]BoardView, error) {
	if in.ViewerID == 0 {
		return nil, ErrAuthRequired
	}
	page, pageSize := normalizePage(in.Page, in.PageSize)
	boards, err := s.repo.ListFollowedBoards(ctx, in.ViewerID, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, err
	}
	// make([]T,0,n)：空结果序列化为 "items":[] 而非 null（信封契约，评审 D2）
	views := make([]BoardView, 0, len(boards))
	for _, b := range boards {
		views = append(views, BoardView{
			ID:          b.ID,
			Name:        b.Name,
			Description: b.Description,
			Viewer:      &FollowViewer{Following: true},
		})
	}
	return views, nil
}

// ListFollowedTopics 返回我关注的话题，按关注时间倒序 + 分页（#23）。
func (s *service) ListFollowedTopics(ctx context.Context, in ListFollowedTopicsQuery) ([]TopicView, error) {
	if in.ViewerID == 0 {
		return nil, ErrAuthRequired
	}
	page, pageSize := normalizePage(in.Page, in.PageSize)
	topics, err := s.repo.ListFollowedTopics(ctx, in.ViewerID, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, err
	}
	views := make([]TopicView, 0, len(topics))
	for _, t := range topics {
		views = append(views, TopicView{
			ID:      t.ID,
			BoardID: t.BoardID,
			Name:    t.Name,
			Viewer:  &FollowViewer{Following: true},
		})
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
	// #54 标签富化（best-effort，评审 F7：tags 查询失败仅缺省，不拖垮列表）
	tagsByPost := map[int64][]string{}
	if tm, err := s.repo.ListTagsByPostIDs(ctx, postIDs); err == nil {
		tagsByPost = tm
	}
	// #72 提及富化（best-effort：查询失败仅缺省，前端按 mentions 列表 gate 渲染链接）
	mentionsByPost := map[int64][]MentionView{}
	if mm, err := s.repo.ListMentionsByTargets(ctx, "post", postIDs); err == nil {
		mentionsByPost = toMentionViewsByTarget(mm)
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
			CreatedAt:     p.CreatedAt,
		}
		view.Tags = tagsByPost[p.ID]
		view.Mentions = mentionsByPost[p.ID]
		if p.TopicID != nil {
			if n, ok := topicNames[*p.TopicID]; ok {
				view.TopicName = &n
			}
		}
		if view.AuthorName == "" {
			view.AuthorName = "已注销"
		}
		// 登录态附 viewer（赞/藏/关注作者初始态）
		if viewerID != 0 {
			following, err := s.users.FollowsUser(ctx, viewerID, p.AuthorID)
			if err != nil {
				return nil, err
			}
			view.Viewer = &PostViewer{Liked: liked[p.ID], Favorited: faved[p.ID], FollowingAuthor: following}
		}
		views = append(views, view)
	}
	return views, nil
}

// commentView 组装评论读模型；names 为批量预取（nil 则逐条查）。
func (s *service) commentView(ctx context.Context, c *Comment, names map[int64]string, mentions map[int64][]MentionView) CommentView {
	v := CommentView{
		ID:        c.ID,
		PostID:    c.PostID,
		AuthorID:  c.AuthorID,
		ParentID:  c.ParentID,
		Floor:     c.Floor,
		Content:   c.Content,
		CreatedAt: c.CreatedAt,
	}
	if mentions == nil {
		// 单条读取（CreateComment/GetComment）：单独查提及（#72；buildTree 走批量富化传非 nil map）
		if mm, err := s.repo.ListMentionsByTargets(ctx, "comment", []int64{c.ID}); err == nil {
			v.Mentions = toMentionViewsByTarget(mm)[c.ID]
		}
	} else {
		v.Mentions = mentions[c.ID] // 无该评论 → nil slice → omitempty 省略
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
