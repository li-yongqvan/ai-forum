package content

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"time"

	"gorm.io/gorm"
)

// fakeRepo 是 Repo 的内存实现，仅用于本包测试（testing.md §2.1：in-memory fake 跨 Service 接口 seam 验证）。
type fakeRepo struct {
	boards   map[int64]*Board
	topics   map[int64]*Topic
	posts    map[int64]*Post
	comments map[int64]*Comment
	likes    map[string]*Like // key: "type:id:user"
	favs     map[string]*Favorite
	followsB map[[2]int64]bool
	followsT map[[2]int64]bool

	nextPostID    int64
	nextCommentID int64
	nextLikeID    int64
	nextFavID     int64
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		boards:   map[int64]*Board{},
		topics:   map[int64]*Topic{},
		posts:    map[int64]*Post{},
		comments: map[int64]*Comment{},
		likes:    map[string]*Like{},
		favs:     map[string]*Favorite{},
		followsB: map[[2]int64]bool{},
		followsT: map[[2]int64]bool{},
		nextPostID: 1, nextCommentID: 1, nextLikeID: 1, nextFavID: 1,
	}
}

// ---- 预置辅助 ----

func (f *fakeRepo) seedBoard(id int64, name string) *Board {
	b := &Board{ID: id, Name: name, SortOrder: int(id)}
	f.boards[id] = b
	return b
}

func (f *fakeRepo) seedTopic(id, boardID int64, name string) *Topic {
	t := &Topic{ID: id, BoardID: boardID, Name: name}
	f.topics[id] = t
	return t
}

type postOpt func(*Post)

func withTopic(topicID int64) postOpt {
	return func(p *Post) { p.TopicID = &topicID }
}
func withPinned(b bool) postOpt {
	return func(p *Post) { p.IsPinned = b }
}
func withCreatedAt(t time.Time) postOpt {
	return func(p *Post) { p.CreatedAt = t }
}

func (f *fakeRepo) seedPost(id, authorID, boardID int64, opts ...postOpt) *Post {
	p := &Post{ID: id, AuthorID: authorID, BoardID: boardID, CreatedAt: time.Now()}
	for _, o := range opts {
		o(p)
	}
	f.posts[id] = p
	return p
}

// ---- boards / topics ----
func (f *fakeRepo) GetBoardByID(ctx context.Context, id int64) (*Board, error) {
	if b, ok := f.boards[id]; ok {
		return b, nil
	}
	return nil, errNotFound
}

func (f *fakeRepo) GetTopicByID(ctx context.Context, id int64) (*Topic, error) {
	if t, ok := f.topics[id]; ok {
		return t, nil
	}
	return nil, errNotFound
}

func (f *fakeRepo) ListBoards(ctx context.Context) ([]*Board, error) {
	out := make([]*Board, 0, len(f.boards))
	for _, b := range f.boards {
		out = append(out, b)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SortOrder < out[j].SortOrder })
	return out, nil
}

func (f *fakeRepo) ListTopics(ctx context.Context, boardID *int64) ([]*Topic, error) {
	out := make([]*Topic, 0)
	for _, t := range f.topics {
		if boardID != nil && t.BoardID != *boardID {
			continue
		}
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// ---- posts ----

func (f *fakeRepo) CreatePost(ctx context.Context, p *Post) error {
	p.ID = f.nextPostID
	f.nextPostID++
	p.CreatedAt = time.Now()
	f.posts[p.ID] = p
	return nil
}

func (f *fakeRepo) GetPostByID(ctx context.Context, id int64) (*Post, error) {
	p, ok := f.posts[id]
	if !ok || p.DeletedAt.Valid {
		return nil, errNotFound
	}
	// 返回拷贝，模拟 GORM 扫描（避免调用方本地自增污染存储对象）
	cp := *p
	return &cp, nil
}

func (f *fakeRepo) ListPosts(ctx context.Context, in PostQuery) ([]*Post, error) {
	out := make([]*Post, 0)
	for _, p := range f.posts {
		if p.DeletedAt.Valid {
			continue
		}
		if in.BoardID != nil && p.BoardID != *in.BoardID {
			continue
		}
		if in.TopicID != nil && (p.TopicID == nil || *p.TopicID != *in.TopicID) {
			continue
		}
		if in.Feed == "follow" {
			inUsers := contains(in.FollowedUserIDs, p.AuthorID)
			inBoards := contains(in.FollowedBoardIDs, p.BoardID)
			inTopics := p.TopicID != nil && contains(in.FollowedTopicIDs, *p.TopicID)
			if !inUsers && !inBoards && !inTopics {
				continue
			}
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].IsPinned != out[j].IsPinned {
			return out[i].IsPinned
		}
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	if in.Offset > len(out) {
		in.Offset = len(out)
	}
	if in.Limit > 0 && in.Offset+in.Limit < len(out) {
		out = out[in.Offset : in.Offset+in.Limit]
	} else if in.Offset < len(out) {
		out = out[in.Offset:]
	}
	return out, nil
}

func (f *fakeRepo) DeletePost(ctx context.Context, id int64) error {
	p, ok := f.posts[id]
	if !ok {
		return errNotFound
	}
	p.DeletedAt = gorm.DeletedAt{Valid: true, Time: time.Now()}
	return nil
}

func (f *fakeRepo) UpdatePostPinned(ctx context.Context, id int64, pinned bool) error {
	p, ok := f.posts[id]
	if !ok {
		return errNotFound
	}
	p.IsPinned = pinned
	return nil
}

func (f *fakeRepo) UpdatePostFeatured(ctx context.Context, id int64, featured bool) error {
	p, ok := f.posts[id]
	if !ok {
		return errNotFound
	}
	p.IsFeatured = featured
	return nil
}

func (f *fakeRepo) IncrementViewCount(ctx context.Context, id int64) error {
	p, ok := f.posts[id]
	if !ok {
		return errNotFound
	}
	p.ViewCount++
	return nil
}

// ---- comments ----

func (f *fakeRepo) CreateComment(ctx context.Context, c *Comment) error {
	c.ID = f.nextCommentID
	f.nextCommentID++
	c.CreatedAt = time.Now()
	f.comments[c.ID] = c
	return nil
}

func (f *fakeRepo) GetCommentByID(ctx context.Context, id int64) (*Comment, error) {
	c, ok := f.comments[id]
	if !ok || c.DeletedAt.Valid {
		return nil, errNotFound
	}
	return c, nil
}

func (f *fakeRepo) GetMaxTopFloor(ctx context.Context, postID int64) (int, error) {
	max := 0
	for _, c := range f.comments {
		if c.PostID == postID && c.Floor != nil && *c.Floor > max {
			max = *c.Floor
		}
	}
	return max, nil
}

func (f *fakeRepo) ListCommentsByPost(ctx context.Context, postID int64) ([]*Comment, error) {
	out := make([]*Comment, 0)
	for _, c := range f.comments {
		if c.PostID == postID {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

func (f *fakeRepo) DeleteComment(ctx context.Context, id int64) error {
	c, ok := f.comments[id]
	if !ok {
		return errNotFound
	}
	c.DeletedAt = gorm.DeletedAt{Valid: true, Time: time.Now()}
	return nil
}

func (f *fakeRepo) CountCommentsByPost(ctx context.Context, postIDs []int64) (map[int64]int, error) {
	m := map[int64]int{}
	for _, c := range f.comments {
		if c.DeletedAt.Valid || !contains(postIDs, c.PostID) {
			continue
		}
		m[c.PostID]++
	}
	return m, nil
}

// ---- likes ----

func likeKey(tt string, targetID, userID int64) string {
	return tt + ":" + itoa(targetID) + ":" + itoa(userID)
}

func (f *fakeRepo) CreateLike(ctx context.Context, l *Like) error {
	l.ID = f.nextLikeID
	f.nextLikeID++
	f.likes[likeKey(l.TargetType, l.TargetID, l.UserID)] = l
	return nil
}

func (f *fakeRepo) DeleteLike(ctx context.Context, userID int64, targetType string, targetID int64) error {
	delete(f.likes, likeKey(targetType, targetID, userID))
	return nil
}

func (f *fakeRepo) LikeExists(ctx context.Context, userID int64, targetType string, targetID int64) (bool, error) {
	_, ok := f.likes[likeKey(targetType, targetID, userID)]
	return ok, nil
}

func (f *fakeRepo) CountLikesByPost(ctx context.Context, postIDs []int64) (map[int64]int, error) {
	m := map[int64]int{}
	for _, l := range f.likes {
		if l.TargetType == "post" && contains(postIDs, l.TargetID) {
			m[l.TargetID]++
		}
	}
	return m, nil
}

func (f *fakeRepo) ListLikedPostIDs(ctx context.Context, userID int64, postIDs []int64) ([]int64, error) {
	out := make([]int64, 0)
	for _, l := range f.likes {
		if l.TargetType == "post" && l.UserID == userID && contains(postIDs, l.TargetID) {
			out = append(out, l.TargetID)
		}
	}
	return out, nil
}

// ---- favorites ----

func (f *fakeRepo) CreateFavorite(ctx context.Context, fl *Favorite) error {
	fl.ID = f.nextFavID
	f.nextFavID++
	f.favs[likeKey("fav", fl.PostID, fl.UserID)] = fl
	return nil
}

func (f *fakeRepo) DeleteFavorite(ctx context.Context, userID, postID int64) error {
	delete(f.favs, likeKey("fav", postID, userID))
	return nil
}

func (f *fakeRepo) FavoriteExists(ctx context.Context, userID, postID int64) (bool, error) {
	_, ok := f.favs[likeKey("fav", postID, userID)]
	return ok, nil
}

func (f *fakeRepo) CountFavoritesByPost(ctx context.Context, postIDs []int64) (map[int64]int, error) {
	m := map[int64]int{}
	for _, fl := range f.favs {
		if contains(postIDs, fl.PostID) {
			m[fl.PostID]++
		}
	}
	return m, nil
}

func (f *fakeRepo) ListFavedPostIDs(ctx context.Context, userID int64, postIDs []int64) ([]int64, error) {
	out := make([]int64, 0)
	for _, fl := range f.favs {
		if fl.UserID == userID && contains(postIDs, fl.PostID) {
			out = append(out, fl.PostID)
		}
	}
	return out, nil
}

// ---- follows（board / topic） ----

func (f *fakeRepo) CreateFollowBoard(ctx context.Context, fl *FollowBoard) error {
	f.followsB[[2]int64{fl.FollowerID, fl.BoardID}] = true
	return nil
}

func (f *fakeRepo) DeleteFollowBoard(ctx context.Context, followerID, boardID int64) error {
	delete(f.followsB, [2]int64{followerID, boardID})
	return nil
}

func (f *fakeRepo) FollowBoardExists(ctx context.Context, followerID, boardID int64) (bool, error) {
	return f.followsB[[2]int64{followerID, boardID}], nil
}

func (f *fakeRepo) ListFollowedBoardIDs(ctx context.Context, followerID int64) ([]int64, error) {
	out := make([]int64, 0)
	for k := range f.followsB {
		if k[0] == followerID {
			out = append(out, k[1])
		}
	}
	return out, nil
}

func (f *fakeRepo) CreateFollowTopic(ctx context.Context, fl *FollowTopic) error {
	f.followsT[[2]int64{fl.FollowerID, fl.TopicID}] = true
	return nil
}

func (f *fakeRepo) DeleteFollowTopic(ctx context.Context, followerID, topicID int64) error {
	delete(f.followsT, [2]int64{followerID, topicID})
	return nil
}

func (f *fakeRepo) FollowTopicExists(ctx context.Context, followerID, topicID int64) (bool, error) {
	return f.followsT[[2]int64{followerID, topicID}], nil
}

func (f *fakeRepo) ListFollowedTopicIDs(ctx context.Context, followerID int64) ([]int64, error) {
	out := make([]int64, 0)
	for k := range f.followsT {
		if k[0] == followerID {
			out = append(out, k[1])
		}
	}
	return out, nil
}

// ---- 辅助 ----

func contains(ids []int64, id int64) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}

func itoa(v int64) string { return strconv.FormatInt(v, 10) }

// fakeUsers 是 UserProvider 的内存实现。
type fakeUsers struct {
	followed []int64
	names    map[int64]string
}

func (u fakeUsers) FollowedUserIDs(ctx context.Context, userID int64) ([]int64, error) {
	return u.followed, nil
}

func (u fakeUsers) GetUserView(ctx context.Context, id int64) (UserView, error) {
	if n, ok := u.names[id]; ok {
		return UserView{ID: id, Username: n}, nil
	}
	return UserView{}, errors.New("fake: 用户不存在")
}

func newServiceWith(f *fakeRepo, u fakeUsers) Service {
	if u.names == nil {
		u.names = map[int64]string{}
	}
	return NewService(f, u)
}
