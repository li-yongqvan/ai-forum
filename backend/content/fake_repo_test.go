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
	followsB map[[2]int64]time.Time
	followsT map[[2]int64]time.Time

	// #54 标签：name→id + postID→有序标签名；errReplaceTags 供标签写失败用例
	tags           map[string]int64
	tagPosts       map[int64][]string
	errReplaceTags error

	// #72 提及：key "type:id:userid"；errCreateMentions 供「落库失败不阻断创建」用例
	mentions          map[string]*Mention
	errCreateMentions error

	// 单调递增关系时钟：关注/收藏时间严格递增，排序断言确定性成立（评审 §七.3）。
	followClock time.Time

	nextPostID    int64
	nextCommentID int64
	nextLikeID    int64
	nextFavID     int64
	nextTagID     int64
	nextMentionID int64
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		boards:      map[int64]*Board{},
		topics:      map[int64]*Topic{},
		posts:       map[int64]*Post{},
		comments:    map[int64]*Comment{},
		likes:       map[string]*Like{},
		favs:        map[string]*Favorite{},
		followsB:    map[[2]int64]time.Time{},
		followsT:    map[[2]int64]time.Time{},
		tags:        map[string]int64{},
		tagPosts:    map[int64][]string{},
		mentions:    map[string]*Mention{},
		followClock: time.Now(),
		nextPostID:  1, nextCommentID: 1, nextLikeID: 1, nextFavID: 1, nextTagID: 1, nextMentionID: 1,
	}
}

// nextFollowTime 返回严格递增的关系时间（每次调用 +1ms，保证同一测试内先后关系可排序）。
func (f *fakeRepo) nextFollowTime() time.Time {
	f.followClock = f.followClock.Add(time.Millisecond)
	return f.followClock
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
		if in.AuthorID != nil && p.AuthorID != *in.AuthorID {
			continue
		}
		if in.BoardID != nil && p.BoardID != *in.BoardID {
			continue
		}
		if in.TopicID != nil && (p.TopicID == nil || *p.TopicID != *in.TopicID) {
			continue
		}
		if in.Tag != nil && !stringContains(f.tagPosts[p.ID], *in.Tag) {
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
		if in.Feed == "hot" && p.CreatedAt.Before(time.Now().Add(-7*24*time.Hour)) {
			// #61 热门流：7 天窗口（镜像 gorm ListPosts 的 SQL 语义）
			continue
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].IsPinned != out[j].IsPinned {
			return out[i].IsPinned
		}
		if in.Feed == "hot" {
			si, sj := f.hotScore(out[i].ID), f.hotScore(out[j].ID)
			if si != sj {
				return si > sj
			}
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

// commentWeight 是 #61 热度公式中「评论」相对「赞」的权重。
// 注意：gorm_repo.go 的 SQL 表达式也硬编码了同一权重（×3），两边需同步修改。
const commentWeight = 3

// hotScore 计算帖子热度分：赞×1 + 评论×commentWeight（#61）。
// 与 gorm ListPosts 的 SQL 聚合同语义：likes 硬删、comments 排除软删。
func (f *fakeRepo) hotScore(postID int64) int {
	likes, comments := 0, 0
	for _, l := range f.likes {
		if l.TargetType == "post" && l.TargetID == postID {
			likes++
		}
	}
	for _, c := range f.comments {
		if c.PostID == postID && !c.DeletedAt.Valid {
			comments++
		}
	}
	return likes + commentWeight*comments
}

func (f *fakeRepo) DeletePost(ctx context.Context, id int64) error {
	p, ok := f.posts[id]
	if !ok {
		return errNotFound
	}
	p.DeletedAt = gorm.DeletedAt{Valid: true, Time: time.Now()}
	delete(f.tagPosts, id)                                     // #54 卫生：删帖清关联
	f.DeleteMentionsByTarget(context.Background(), "post", id) // #72 卫生：删帖清提及
	return nil
}

func (f *fakeRepo) CountPostsByAuthor(ctx context.Context, authorID int64) (int, error) {
	n := 0
	for _, p := range f.posts {
		if !p.DeletedAt.Valid && p.AuthorID == authorID {
			n++
		}
	}
	return n, nil
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
	f.DeleteMentionsByTarget(context.Background(), "comment", id) // #72 卫生：删评论清提及
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
	if fl.CreatedAt.IsZero() {
		fl.CreatedAt = f.nextFollowTime()
	}
	f.favs[likeKey("fav", fl.PostID, fl.UserID)] = fl
	return nil
}

// seedFavorite 以显式时间预置收藏（排序测试用确定性时间）。
func (f *fakeRepo) seedFavorite(userID, postID int64, at time.Time) {
	f.favs[likeKey("fav", postID, userID)] = &Favorite{ID: f.nextFavID, UserID: userID, PostID: postID, CreatedAt: at}
	f.nextFavID++
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
	f.followsB[[2]int64{fl.FollowerID, fl.BoardID}] = f.nextFollowTime()
	return nil
}

func (f *fakeRepo) DeleteFollowBoard(ctx context.Context, followerID, boardID int64) error {
	delete(f.followsB, [2]int64{followerID, boardID})
	return nil
}

func (f *fakeRepo) FollowBoardExists(ctx context.Context, followerID, boardID int64) (bool, error) {
	_, ok := f.followsB[[2]int64{followerID, boardID}]
	return ok, nil
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
	f.followsT[[2]int64{fl.FollowerID, fl.TopicID}] = f.nextFollowTime()
	return nil
}

func (f *fakeRepo) DeleteFollowTopic(ctx context.Context, followerID, topicID int64) error {
	delete(f.followsT, [2]int64{followerID, topicID})
	return nil
}

func (f *fakeRepo) FollowTopicExists(ctx context.Context, followerID, topicID int64) (bool, error) {
	_, ok := f.followsT[[2]int64{followerID, topicID}]
	return ok, nil
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

// ---- 收藏/关注列表（#23，fake 实现：跳过软删、按关系时间倒序 + id 决胜、分页） ----

func (f *fakeRepo) ListFavoritedPosts(ctx context.Context, userID int64, offset, limit int) ([]*Post, error) {
	type item struct {
		post *Post
		at   time.Time
		id   int64
	}
	items := make([]item, 0)
	for _, fl := range f.favs {
		if fl.UserID != userID {
			continue
		}
		p, ok := f.posts[fl.PostID]
		if !ok || p.DeletedAt.Valid {
			continue // 软删帖子不出现在收藏列表（对应 GORM 主表 deleted_at IS NULL）
		}
		items = append(items, item{post: p, at: fl.CreatedAt, id: fl.ID})
	}
	sort.Slice(items, func(i, j int) bool {
		if !items[i].at.Equal(items[j].at) {
			return items[i].at.After(items[j].at)
		}
		return items[i].id > items[j].id
	})
	out := make([]*Post, 0, len(items))
	for _, it := range slicePage(items, offset, limit) {
		out = append(out, it.post)
	}
	return out, nil
}

func (f *fakeRepo) ListFollowedBoards(ctx context.Context, followerID int64, offset, limit int) ([]*Board, error) {
	type item struct {
		board *Board
		at    time.Time
	}
	items := make([]item, 0)
	for k, at := range f.followsB {
		if k[0] != followerID {
			continue
		}
		b, ok := f.boards[k[1]]
		if !ok || b.DeletedAt.Valid {
			continue
		}
		items = append(items, item{board: b, at: at})
	}
	sort.Slice(items, func(i, j int) bool {
		if !items[i].at.Equal(items[j].at) {
			return items[i].at.After(items[j].at)
		}
		return items[i].board.ID > items[j].board.ID
	})
	out := make([]*Board, 0, len(items))
	for _, it := range slicePage(items, offset, limit) {
		out = append(out, it.board)
	}
	return out, nil
}

func (f *fakeRepo) ListFollowedTopics(ctx context.Context, followerID int64, offset, limit int) ([]*Topic, error) {
	type item struct {
		topic *Topic
		at    time.Time
	}
	items := make([]item, 0)
	for k, at := range f.followsT {
		if k[0] != followerID {
			continue
		}
		t, ok := f.topics[k[1]]
		if !ok || t.DeletedAt.Valid {
			continue
		}
		items = append(items, item{topic: t, at: at})
	}
	sort.Slice(items, func(i, j int) bool {
		if !items[i].at.Equal(items[j].at) {
			return items[i].at.After(items[j].at)
		}
		return items[i].topic.ID > items[j].topic.ID
	})
	out := make([]*Topic, 0, len(items))
	for _, it := range slicePage(items, offset, limit) {
		out = append(out, it.topic)
	}
	return out, nil
}

// ---- 标签（#54，fake 实现） ----

func (f *fakeRepo) ReplacePostTags(ctx context.Context, postID int64, tagNames []string) error {
	if f.errReplaceTags != nil {
		return f.errReplaceTags
	}
	// 去重保序（与 ParseTags 语义一致，防御性）
	seen := make(map[string]bool)
	names := make([]string, 0, len(tagNames))
	for _, n := range tagNames {
		if n == "" || seen[n] {
			continue
		}
		seen[n] = true
		names = append(names, n)
	}
	f.tagPosts[postID] = names
	f.ensureTagIDs(names)
	return nil
}

func (f *fakeRepo) ListTagsByPostIDs(ctx context.Context, postIDs []int64) (map[int64][]string, error) {
	out := make(map[int64][]string, len(postIDs))
	for _, pid := range postIDs {
		if tags, ok := f.tagPosts[pid]; ok {
			out[pid] = append([]string(nil), tags...)
		}
	}
	return out, nil
}

// seedPostTags 预置帖子标签（ListFeedByTag 等测试用），归一化小写。
func (f *fakeRepo) seedPostTags(postID int64, tags ...string) {
	names := make([]string, 0, len(tags))
	for _, n := range tags {
		names = append(names, NormalizeTag(n))
	}
	f.tagPosts[postID] = names
	f.ensureTagIDs(names)
}

// ensureTagIDs 为未登记标签分配 id（fake 幂等，去重分配循环）。
func (f *fakeRepo) ensureTagIDs(names []string) {
	for _, n := range names {
		if _, ok := f.tags[n]; !ok {
			f.tags[n] = f.nextTagID
			f.nextTagID++
		}
	}
}

// ---- 提及（#72，fake 实现） ----

// CreateMentions 批量插入提及关联（已存在 key → 跳过，模拟 ON CONFLICT DO NOTHING 幂等）。
func (f *fakeRepo) CreateMentions(ctx context.Context, mentions []*Mention) error {
	if f.errCreateMentions != nil {
		return f.errCreateMentions
	}
	for _, m := range mentions {
		key := mentionKey(m.TargetType, m.TargetID, m.MentionedUserID)
		if _, ok := f.mentions[key]; ok {
			continue
		}
		m.ID = f.nextMentionID
		f.nextMentionID++
		f.mentions[key] = m
	}
	return nil
}

func (f *fakeRepo) ListMentionsByTargets(ctx context.Context, targetType string, targetIDs []int64) (map[int64][]*Mention, error) {
	want := make(map[int64]bool, len(targetIDs))
	for _, id := range targetIDs {
		want[id] = true
	}
	out := make(map[int64][]*Mention)
	for _, m := range f.mentions {
		if m.TargetType != targetType || !want[m.TargetID] {
			continue
		}
		out[m.TargetID] = append(out[m.TargetID], m)
	}
	return out, nil
}

func (f *fakeRepo) DeleteMentionsByTarget(ctx context.Context, targetType string, targetID int64) error {
	for key, m := range f.mentions {
		if m.TargetType == targetType && m.TargetID == targetID {
			delete(f.mentions, key)
		}
	}
	return nil
}

func mentionKey(targetType string, targetID, userID int64) string {
	return targetType + ":" + strconv.FormatInt(targetID, 10) + ":" + strconv.FormatInt(userID, 10)
}

// slicePage 应用 offset/limit 分页（与 fake ListPosts 的内联分页语义一致）。
func slicePage[T any](items []T, offset, limit int) []T {
	start := offset
	if start > len(items) {
		start = len(items)
	}
	end := len(items)
	if limit > 0 && start+limit < len(items) {
		end = start + limit
	}
	return items[start:end]
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

func stringContains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
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
	byName   map[string]int64  // username → id（#72 提及解析；缺省时从 names 反查填充）
	banned   map[int64]bool    // #72 active 过滤：被封禁用户按不存在处理
	follows  map[[2]int64]bool // followerID→targetID 是否关注（默认 false）
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

func (u fakeUsers) GetUserByUsername(ctx context.Context, username string) (UserView, error) {
	id, ok := u.byName[username]
	if !ok {
		return UserView{}, ErrMentionedUserNotFound
	}
	if u.banned[id] {
		return UserView{}, ErrMentionedUserNotFound
	}
	return UserView{ID: id, Username: username}, nil
}

func (u fakeUsers) FollowsUser(ctx context.Context, followerID, targetID int64) (bool, error) {
	return u.follows[[2]int64{followerID, targetID}], nil
}

// fakeNotifier 记录提及通知命令，供断言通知次数与内容；err 非 nil 时模拟通知失败（best-effort 路径）。
type fakeNotifier struct {
	calls []MentionNotificationCmd
	err   error
}

func (f *fakeNotifier) NotifyMention(_ context.Context, in MentionNotificationCmd) error {
	if f.err != nil {
		return f.err
	}
	f.calls = append(f.calls, in)
	return nil
}

func newServiceWith(f *fakeRepo, u fakeUsers) Service {
	return newServiceWithN(f, u, &fakeNotifier{})
}

// newServiceWithN 允许注入自定义 Notifier（#72 通知断言用）。
func newServiceWithN(f *fakeRepo, u fakeUsers, n Notifier) Service {
	if u.names == nil {
		u.names = map[int64]string{}
	}
	if u.byName == nil {
		// 未显式指定 byName 时从 names 反查填充（默认全部存在且 active）
		u.byName = map[string]int64{}
		for id, name := range u.names {
			u.byName[name] = id
		}
	}
	return NewService(f, u, n)
}
