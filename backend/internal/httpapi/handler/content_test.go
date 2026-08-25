package handler_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/li-yongqvan/ai-forum/backend/internal/testutil"
	"gorm.io/gorm"
)

type postView struct {
	ID            int64   `json:"id"`
	BoardName     string  `json:"board_name"`
	TopicName     *string `json:"topic_name"`
	AuthorName    string  `json:"author_name"`
	AuthorID      int64   `json:"author_id"`
	ViewCount     int     `json:"view_count"`
	LikeCount     int     `json:"like_count"`
	CommentCount  int     `json:"comment_count"`
	FavoriteCount int     `json:"favorite_count"`
	Tags          []string `json:"tags"`
	Viewer        *struct {
		Liked           bool `json:"liked"`
		Favorited       bool `json:"favorited"`
		FollowingAuthor bool `json:"following_author"`
	} `json:"viewer"`
	IsPinned bool `json:"is_pinned"`
}

type commentView struct {
	ID    int64  `json:"id"`
	Floor *int   `json:"floor"`
}

type treeResp struct {
	PostID   int64        `json:"post_id"`
	Comments []treeNode   `json:"comments"`
}

type treeNode struct {
	ID      int64      `json:"id"`
	Floor   *int       `json:"floor"`
	Replies []treeNode `json:"replies"`
}

type listResp struct {
	Items []postView `json:"items"`
}

func registerUser(t *testing.T, r *gin.Engine, gdb *gorm.DB, username, email, code string) authResp {
	t.Helper()
	seedCode(t, gdb, code)
	w := doJSON(t, r, http.MethodPost, "/api/v1/auth/register", map[string]string{
		"username": username, "email": email, "password": "secret123", "invite_code": code,
	}, "")
	if w.Code != http.StatusCreated {
		t.Fatalf("register %s = %d, body=%s", username, w.Code, w.Body.String())
	}
	return decodeAuthResp(t, w)
}

// makeModerator 直接改库提权（MVP 无管理 UI；提权后重新登录拿 moderator token）。
func makeModerator(t *testing.T, gdb *gorm.DB, username string) {
	t.Helper()
	if err := gdb.Exec(`UPDATE "user".users SET role='moderator' WHERE username=?`, username).Error; err != nil {
		t.Fatalf("提权失败: %v", err)
	}
}

func login(t *testing.T, r *gin.Engine, username string) authResp {
	t.Helper()
	w := doJSON(t, r, http.MethodPost, "/api/v1/auth/login", map[string]string{
		"username": username, "password": "secret123",
	}, "")
	if w.Code != http.StatusOK {
		t.Fatalf("login %s = %d, body=%s", username, w.Code, w.Body.String())
	}
	return decodeAuthResp(t, w)
}

func createPost(t *testing.T, r *gin.Engine, token string, boardID int64, title string) postView {
	t.Helper()
	w := doJSON(t, r, http.MethodPost, "/api/v1/posts", map[string]any{
		"board_id": boardID, "title": title, "content": "正文",
	}, token)
	if w.Code != http.StatusCreated {
		t.Fatalf("create post = %d, body=%s", w.Code, w.Body.String())
	}
	var p postView
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatalf("解析帖子失败: %v", err)
	}
	return p
}

// 板块/话题种子数据来自迁移 0005。
func TestContentBoardsTopics(t *testing.T) {
	gdb := testutil.SetupPG(t)
	r := newEngine(t, gdb)

	w := doJSON(t, r, http.MethodGet, "/api/v1/boards", nil, "")
	var boards struct {
		Items []struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		} `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &boards); err != nil {
		t.Fatal(err)
	}
	if len(boards.Items) != 6 {
		t.Errorf("板块数 = %d, want 6", len(boards.Items))
	}
	if boards.Items[0].Name != "学习讨论" {
		t.Errorf("首板块 = %q, want 学习讨论", boards.Items[0].Name)
	}

	w2 := doJSON(t, r, http.MethodGet, "/api/v1/topics?board_id=1", nil, "")
	var topics struct {
		Items []struct {
			Name string `json:"name"`
		} `json:"items"`
	}
	if err := json.Unmarshal(w2.Body.Bytes(), &topics); err != nil {
		t.Fatal(err)
	}
	if len(topics.Items) != 4 {
		t.Errorf("板块1 话题数 = %d, want 4（LLM/RAG/微调/提示工程）", len(topics.Items))
	}
}

// 主流程：发帖 → feed → 详情 → 评论 → 点赞/收藏 → 关注板块 → 关注流。
func TestContentFlow(t *testing.T) {
	gdb := testutil.SetupPG(t)
	r := newEngine(t, gdb)

	alice := registerUser(t, r, gdb, "alice", "alice@x.edu", "CODE-C1")
	token := alice.Token

	// 发帖（板块1 + 话题2=RAG）
	w := doJSON(t, r, http.MethodPost, "/api/v1/posts", map[string]any{
		"board_id": 1, "topic_id": 2, "title": "RAG 实践踩坑", "content": "正文",
	}, token)
	if w.Code != http.StatusCreated {
		t.Fatalf("create post = %d, body=%s", w.Code, w.Body.String())
	}
	var created postView
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.BoardName != "学习讨论" || created.AuthorName != "alice" {
		t.Errorf("post enrich = %+v", created)
	}
	if created.TopicName == nil || *created.TopicName != "RAG" {
		t.Errorf("topic_name = %v, want RAG", created.TopicName)
	}

	// feed（游客可看）
	w2 := doJSON(t, r, http.MethodGet, "/api/v1/posts?tab=all", nil, "")
	var feed listResp
	if err := json.Unmarshal(w2.Body.Bytes(), &feed); err != nil {
		t.Fatal(err)
	}
	if len(feed.Items) == 0 || feed.Items[0].ID != created.ID {
		t.Errorf("feed 首位应为新帖, items=%d", len(feed.Items))
	}

	// 详情（登录态看 is_liked/is_faved）
	w3 := doJSON(t, r, http.MethodGet, fmt.Sprintf("/api/v1/posts/%d", created.ID), nil, token)
	var detail postView
	if err := json.Unmarshal(w3.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	if detail.ViewCount != 1 {
		t.Errorf("view_count = %d, want 1", detail.ViewCount)
	}

	// 评论（顶层 floor 1）
	w4 := doJSON(t, r, http.MethodPost, "/api/v1/comments", map[string]any{
		"post_id": created.ID, "content": "沙发",
	}, token)
	if w4.Code != http.StatusCreated {
		t.Fatalf("comment = %d, body=%s", w4.Code, w4.Body.String())
	}
	var cmt commentView
	if err := json.Unmarshal(w4.Body.Bytes(), &cmt); err != nil {
		t.Fatal(err)
	}
	if cmt.Floor == nil || *cmt.Floor != 1 {
		t.Errorf("floor = %v, want 1", cmt.Floor)
	}

	// 评论树（游客可看）
	w5 := doJSON(t, r, http.MethodGet, fmt.Sprintf("/api/v1/posts/%d/comments", created.ID), nil, "")
	var tree treeResp
	if err := json.Unmarshal(w5.Body.Bytes(), &tree); err != nil {
		t.Fatal(err)
	}
	if len(tree.Comments) != 1 || tree.Comments[0].ID != cmt.ID {
		t.Errorf("评论树 = %+v", tree)
	}

	// 点赞 → 重复 409 → 详情 is_liked=true → 取消
	if w := doJSON(t, r, http.MethodPost, "/api/v1/likes", map[string]any{"target_type": "post", "target_id": created.ID}, token); w.Code != http.StatusOK {
		t.Fatalf("like = %d", w.Code)
	}
	if w := doJSON(t, r, http.MethodPost, "/api/v1/likes", map[string]any{"target_type": "post", "target_id": created.ID}, token); w.Code != http.StatusConflict {
		t.Errorf("dup like = %d, want 409", w.Code)
	}
	w6 := doJSON(t, r, http.MethodGet, fmt.Sprintf("/api/v1/posts/%d", created.ID), nil, token)
	var liked postView
	_ = json.Unmarshal(w6.Body.Bytes(), &liked)
	if liked.Viewer == nil || !liked.Viewer.Liked || liked.LikeCount != 1 {
		t.Errorf("点赞后 viewer.liked = %+v, like_count = %d", liked.Viewer, liked.LikeCount)
	}
	if w := doJSON(t, r, http.MethodDelete, fmt.Sprintf("/api/v1/likes?target_type=post&target_id=%d", created.ID), nil, token); w.Code != http.StatusOK {
		t.Errorf("unlike = %d", w.Code)
	}

	// 收藏 / 取藏
	if w := doJSON(t, r, http.MethodPost, "/api/v1/favorites", map[string]any{"post_id": created.ID}, token); w.Code != http.StatusOK {
		t.Fatalf("favorite = %d", w.Code)
	}
	if w := doJSON(t, r, http.MethodDelete, fmt.Sprintf("/api/v1/favorites?post_id=%d", created.ID), nil, token); w.Code != http.StatusOK {
		t.Errorf("unfavorite = %d", w.Code)
	}

	// 关注板块 1 → 关注流应含该帖
	if w := doJSON(t, r, http.MethodPost, "/api/v1/follows", map[string]any{"target_type": "board", "target_id": 1}, token); w.Code != http.StatusOK {
		t.Fatalf("follow board = %d", w.Code)
	}
	wf := doJSON(t, r, http.MethodGet, "/api/v1/posts?tab=follow", nil, token)
	var followFeed listResp
	if err := json.Unmarshal(wf.Body.Bytes(), &followFeed); err != nil {
		t.Fatal(err)
	}
	if len(followFeed.Items) != 1 || followFeed.Items[0].ID != created.ID {
		t.Errorf("关注流应含该帖, items=%d", len(followFeed.Items))
	}
	// 取关 → 关注流空
	if w := doJSON(t, r, http.MethodDelete, "/api/v1/follows?target_type=board&target_id=1", nil, token); w.Code != http.StatusOK {
		t.Errorf("unfollow = %d", w.Code)
	}
	wf2 := doJSON(t, r, http.MethodGet, "/api/v1/posts?tab=follow", nil, token)
	var followFeed2 listResp
	_ = json.Unmarshal(wf2.Body.Bytes(), &followFeed2)
	if len(followFeed2.Items) != 0 {
		t.Errorf("取关后关注流 = %d, want 0", len(followFeed2.Items))
	}

	// 游客请求关注流 → 401
	if w := doJSON(t, r, http.MethodGet, "/api/v1/posts?tab=follow", nil, ""); w.Code != http.StatusUnauthorized {
		t.Errorf("游客 follow = %d, want 401", w.Code)
	}
}

// TestTagsFlow #54 标签端到端：发带 # 帖 → tags 落库返回 → 按 tag 聚合（游客可读）→ 大小写不敏感 → 空 tag 不过滤 → 删帖剔除。
func TestTagsFlow(t *testing.T) {
	gdb := testutil.SetupPG(t)
	r := newEngine(t, gdb)

	alice := registerUser(t, r, gdb, "alice", "alice@x.edu", "CODE-C1")
	token := alice.Token

	// 发帖：正文带 #RAG 和 #Agent（大小写混合）
	w := doJSON(t, r, http.MethodPost, "/api/v1/posts", map[string]any{
		"board_id": 1, "title": "标签系统", "content": "今天学习 #RAG 和 #Agent",
	}, token)
	if w.Code != http.StatusCreated {
		t.Fatalf("create post = %d, body=%s", w.Code, w.Body.String())
	}
	var created postView
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(created.Tags, []string{"rag", "agent"}) {
		t.Errorf("create tags = %v, want [rag agent]", created.Tags)
	}

	// 无标签帖：tags 字段缺省（omitempty）
	wNo := doJSON(t, r, http.MethodPost, "/api/v1/posts", map[string]any{
		"board_id": 1, "title": "无标签", "content": "普通正文",
	}, token)
	if wNo.Code != http.StatusCreated {
		t.Fatalf("create no-tag post = %d", wNo.Code)
	}
	var noTag postView
	_ = json.Unmarshal(wNo.Body.Bytes(), &noTag)
	if len(noTag.Tags) != 0 {
		t.Errorf("无标签帖 tags = %v, want 空", noTag.Tags)
	}

	// 按标签聚合（游客可读）
	wTag := doJSON(t, r, http.MethodGet, "/api/v1/posts?tag=rag", nil, "")
	var feed listResp
	if err := json.Unmarshal(wTag.Body.Bytes(), &feed); err != nil {
		t.Fatal(err)
	}
	if len(feed.Items) != 1 || feed.Items[0].ID != created.ID {
		t.Fatalf("tag=rag items = %d, want 仅带 rag 的帖", len(feed.Items))
	}
	if !reflect.DeepEqual(feed.Items[0].Tags, []string{"rag", "agent"}) {
		t.Errorf("列表 tags = %v, want [rag agent]", feed.Items[0].Tags)
	}

	// 大小写不敏感：?tag=RAG 命中同一帖
	wTag2 := doJSON(t, r, http.MethodGet, "/api/v1/posts?tag=RAG", nil, "")
	var feed2 listResp
	_ = json.Unmarshal(wTag2.Body.Bytes(), &feed2)
	if len(feed2.Items) != 1 || feed2.Items[0].ID != created.ID {
		t.Errorf("tag=RAG items = %d, want 1（大小写不敏感）", len(feed2.Items))
	}

	// 不存在的标签 → 空列表（正常空态）
	wNone := doJSON(t, r, http.MethodGet, "/api/v1/posts?tag=nonexistent", nil, "")
	var feedNone listResp
	_ = json.Unmarshal(wNone.Body.Bytes(), &feedNone)
	if len(feedNone.Items) != 0 {
		t.Errorf("tag=nonexistent items = %d, want 0", len(feedNone.Items))
	}

	// 空 tag（?tag=%20 归一化后空）→ 不过滤，返回全部（评审 F3/F5）
	wSpace := doJSON(t, r, http.MethodGet, "/api/v1/posts?tag=%20", nil, "")
	var feedSpace listResp
	_ = json.Unmarshal(wSpace.Body.Bytes(), &feedSpace)
	if len(feedSpace.Items) != 2 {
		t.Errorf("tag=%%20 items = %d, want 2（空 tag 不过滤）", len(feedSpace.Items))
	}

	// 删帖 → 标签聚合不再含该帖
	if w := doJSON(t, r, http.MethodDelete, fmt.Sprintf("/api/v1/posts/%d", created.ID), nil, token); w.Code != http.StatusOK {
		t.Fatalf("delete post = %d", w.Code)
	}
	wAfter := doJSON(t, r, http.MethodGet, "/api/v1/posts?tag=rag", nil, "")
	var feedAfter listResp
	_ = json.Unmarshal(wAfter.Body.Bytes(), &feedAfter)
	if len(feedAfter.Items) != 0 {
		t.Errorf("删帖后 tag=rag items = %d, want 0", len(feedAfter.Items))
	}
}

// 权限矩阵（#9 §5.0）：作者删自己 / user 删他人 403 / user 置顶 403 / moderator 置顶 200。
func TestContentPermissions(t *testing.T) {
	gdb := testutil.SetupPG(t)
	r := newEngine(t, gdb)

	alice := registerUser(t, r, gdb, "alice", "alice@x.edu", "CODE-P1")
	_ = registerUser(t, r, gdb, "bob", "bob@x.edu", "CODE-P2")
	makeModerator(t, gdb, "bob")
	mod := login(t, r, "bob")
	carol := registerUser(t, r, gdb, "carol", "carol@x.edu", "CODE-P3")

	p := createPost(t, r, alice.Token, 1, "alice 帖")

	// user 删他人帖 → 403
	if w := doJSON(t, r, http.MethodDelete, fmt.Sprintf("/api/v1/posts/%d", p.ID), nil, carol.Token); w.Code != http.StatusForbidden {
		t.Errorf("user 删他人帖 = %d, want 403", w.Code)
	}
	// user 置顶 → 403
	if w := doJSON(t, r, http.MethodPost, fmt.Sprintf("/api/v1/posts/%d/pin", p.ID), nil, carol.Token); w.Code != http.StatusForbidden {
		t.Errorf("user 置顶 = %d, want 403", w.Code)
	}
	// moderator 置顶 → 200，且 is_pinned 翻转
	if w := doJSON(t, r, http.MethodPost, fmt.Sprintf("/api/v1/posts/%d/pin", p.ID), nil, mod.Token); w.Code != http.StatusOK {
		t.Fatalf("mod 置顶 = %d", w.Code)
	}
	gd := doJSON(t, r, http.MethodGet, fmt.Sprintf("/api/v1/posts/%d", p.ID), nil, "")
	var pd postView
	_ = json.Unmarshal(gd.Body.Bytes(), &pd)
	if !pd.IsPinned {
		t.Error("置顶后 is_pinned 应为 true")
	}
	// moderator 删他人帖 → 200
	if w := doJSON(t, r, http.MethodDelete, fmt.Sprintf("/api/v1/posts/%d", p.ID), nil, mod.Token); w.Code != http.StatusOK {
		t.Errorf("mod 删他人帖 = %d, want 200", w.Code)
	}
	// 作者删自己帖 → 200
	p2 := createPost(t, r, alice.Token, 1, "alice 帖2")
	if w := doJSON(t, r, http.MethodDelete, fmt.Sprintf("/api/v1/posts/%d", p2.ID), nil, alice.Token); w.Code != http.StatusOK {
		t.Errorf("作者删帖 = %d, want 200", w.Code)
	}
}

// 新后端能力（v2 §4）：author_id 过滤 + viewer 字段（赞/藏/关注作者）。
func TestAuthorFilterAndViewer(t *testing.T) {
	gdb := testutil.SetupPG(t)
	r := newEngine(t, gdb)

	alice := registerUser(t, r, gdb, "alice", "alice@x.edu", "CODE-V1")
	bob := registerUser(t, r, gdb, "bob", "bob@x.edu", "CODE-V2")

	p1 := createPost(t, r, alice.Token, 1, "alice 帖")
	p2 := createPost(t, r, bob.Token, 1, "bob 帖")

	// author_id 过滤：只出该作者的帖
	w := doJSON(t, r, http.MethodGet, fmt.Sprintf("/api/v1/posts?author_id=%d", alice.User.ID), nil, "")
	var feed listResp
	if err := json.Unmarshal(w.Body.Bytes(), &feed); err != nil {
		t.Fatal(err)
	}
	if len(feed.Items) != 1 || feed.Items[0].ID != p1.ID {
		t.Errorf("author 过滤 = %d 条, want 仅 p1", len(feed.Items))
	}

	// alice 点赞 p1 + 关注 bob
	if w := doJSON(t, r, http.MethodPost, "/api/v1/likes", map[string]any{"target_type": "post", "target_id": p1.ID}, alice.Token); w.Code != http.StatusOK {
		t.Fatalf("like = %d", w.Code)
	}
	if w := doJSON(t, r, http.MethodPost, "/api/v1/follows", map[string]any{"target_type": "user", "target_id": bob.User.ID}, alice.Token); w.Code != http.StatusOK {
		t.Fatalf("follow = %d", w.Code)
	}

	// 列表带 alice token：p1.viewer.liked=true；p2.viewer.following_author=true（关注 bob）
	w2 := doJSON(t, r, http.MethodGet, "/api/v1/posts", nil, alice.Token)
	var feed2 listResp
	if err := json.Unmarshal(w2.Body.Bytes(), &feed2); err != nil {
		t.Fatal(err)
	}
	for _, item := range feed2.Items {
		switch item.ID {
		case p1.ID:
			if item.Viewer == nil || !item.Viewer.Liked {
				t.Errorf("p1 viewer = %+v, want liked=true", item.Viewer)
			}
		case p2.ID:
			if item.Viewer == nil || !item.Viewer.FollowingAuthor {
				t.Errorf("p2 viewer = %+v, want following_author=true", item.Viewer)
			}
		}
	}

	// 游客列表：viewer 为 nil
	w3 := doJSON(t, r, http.MethodGet, "/api/v1/posts", nil, "")
	var feed3 listResp
	_ = json.Unmarshal(w3.Body.Bytes(), &feed3)
	for _, item := range feed3.Items {
		if item.Viewer != nil {
			t.Errorf("游客列表 viewer 应为 nil, got %+v", item.Viewer)
		}
	}
}

// 新后端能力（v2 §4）：GET /users/:id 公开资料 + viewer.following。
func TestUserProfileEndpoint(t *testing.T) {
	gdb := testutil.SetupPG(t)
	r := newEngine(t, gdb)

	alice := registerUser(t, r, gdb, "alice", "alice@x.edu", "CODE-U1")
	bob := registerUser(t, r, gdb, "bob", "bob@x.edu", "CODE-U2")

	// bob 关注 alice；alice 发 2 帖
	if w := doJSON(t, r, http.MethodPost, "/api/v1/follows", map[string]any{"target_type": "user", "target_id": alice.User.ID}, bob.Token); w.Code != http.StatusOK {
		t.Fatalf("bob follow alice = %d", w.Code)
	}
	createPost(t, r, alice.Token, 1, "a1")
	createPost(t, r, alice.Token, 1, "a2")

	// bob 视角看 alice 主页
	w := doJSON(t, r, http.MethodGet, fmt.Sprintf("/api/v1/users/%d", alice.User.ID), nil, bob.Token)
	if w.Code != http.StatusOK {
		t.Fatalf("profile = %d, body=%s", w.Code, w.Body.String())
	}
	var prof struct {
		Username       string `json:"username"`
		PostCount      int    `json:"post_count"`
		FollowerCount  int    `json:"follower_count"`
		FollowingCount int    `json:"following_count"`
		Viewer         *struct {
			Following bool `json:"following"`
		} `json:"viewer"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &prof); err != nil {
		t.Fatal(err)
	}
	if prof.Username != "alice" || prof.PostCount != 2 || prof.FollowerCount != 1 || prof.FollowingCount != 0 {
		t.Errorf("profile = %+v, want username=alice posts=2 followers=1 following=0", prof)
	}
	if prof.Viewer == nil || !prof.Viewer.Following {
		t.Errorf("bob 视角 viewer.following 应为 true, got %+v", prof.Viewer)
	}

	// 游客视角：viewer 为 nil
	wg := doJSON(t, r, http.MethodGet, fmt.Sprintf("/api/v1/users/%d", alice.User.ID), nil, "")
	var profG struct {
		Viewer *struct {
			Following bool `json:"following"`
		} `json:"viewer"`
	}
	_ = json.Unmarshal(wg.Body.Bytes(), &profG)
	if profG.Viewer != nil {
		t.Error("游客 profile viewer 应为 nil")
	}

	// 不存在 → 404
	if w := doJSON(t, r, http.MethodGet, "/api/v1/users/9999", nil, ""); w.Code != http.StatusNotFound {
		t.Errorf("不存在用户 = %d, want 404", w.Code)
	}
}

// TestFavoritesList：GET /api/v1/favorites（#23）。真 PG 是 GORM JOIN + Select 列遮蔽（评审 D1）
// 与软删过滤唯一能暴露的闸门，故断言**真实主键**而非仅顺序/长度。
func TestFavoritesList(t *testing.T) {
	gdb := testutil.SetupPG(t)
	r := newEngine(t, gdb)

	alice := registerUser(t, r, gdb, "alice", "alice@x.edu", "CODE-FL1")
	bob := registerUser(t, r, gdb, "bob", "bob@x.edu", "CODE-FL2")

	p1 := createPost(t, r, alice.Token, 1, "alice 帖1")
	p2 := createPost(t, r, alice.Token, 1, "alice 帖2")

	// 收藏 p1 再收藏 p2 → 列表按收藏时间倒序：p2 在前
	fav := func(postID int64) {
		t.Helper()
		if w := doJSON(t, r, http.MethodPost, "/api/v1/favorites", map[string]any{"post_id": postID}, alice.Token); w.Code != http.StatusOK {
			t.Fatalf("favorite p%d = %d, body=%s", postID, w.Code, w.Body.String())
		}
	}
	fav(p1.ID)
	fav(p2.ID)

	// 完整列表：真实主键 + 顺序 + viewer.favorited
	w := doJSON(t, r, http.MethodGet, "/api/v1/favorites", nil, alice.Token)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /favorites = %d, body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Items    []postView `json:"items"`
		Page     int        `json:"page"`
		PageSize int        `json:"page_size"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Items) != 2 || resp.Items[0].ID != p2.ID || resp.Items[1].ID != p1.ID {
		t.Errorf("收藏列表 = %d 条, want [p2(%d) p1(%d)]（收藏时间倒序，真实主键）", len(resp.Items), p2.ID, p1.ID)
	}
	for _, it := range resp.Items {
		if it.Viewer == nil || !it.Viewer.Favorited {
			t.Errorf("items viewer.favorited 应为 true, got %+v", it.Viewer)
		}
	}

	// 分页
	w1 := doJSON(t, r, http.MethodGet, "/api/v1/favorites?page_size=1", nil, alice.Token)
	var page1 struct {
		Items []postView `json:"items"`
	}
	_ = json.Unmarshal(w1.Body.Bytes(), &page1)
	if len(page1.Items) != 1 || page1.Items[0].ID != p2.ID {
		t.Errorf("page_size=1 = %d 条, want [p2]", len(page1.Items))
	}
	w2 := doJSON(t, r, http.MethodGet, "/api/v1/favorites?page=2&page_size=1", nil, alice.Token)
	var page2 struct {
		Items []postView `json:"items"`
	}
	_ = json.Unmarshal(w2.Body.Bytes(), &page2)
	if len(page2.Items) != 1 || page2.Items[0].ID != p1.ID {
		t.Errorf("page2/page_size1 = %d 条, want [p1]", len(page2.Items))
	}

	// 取藏 p2 → 列表只剩 p1
	if w := doJSON(t, r, http.MethodDelete, fmt.Sprintf("/api/v1/favorites?post_id=%d", p2.ID), nil, alice.Token); w.Code != http.StatusOK {
		t.Fatalf("unfavorite p2 = %d", w.Code)
	}
	w3 := doJSON(t, r, http.MethodGet, "/api/v1/favorites", nil, alice.Token)
	var after struct {
		Items []postView `json:"items"`
	}
	_ = json.Unmarshal(w3.Body.Bytes(), &after)
	if len(after.Items) != 1 || after.Items[0].ID != p1.ID {
		t.Errorf("取藏后 = %d 条, want [p1]", len(after.Items))
	}

	// 空态信封：无收藏的 bob → "items":[] 而非 null（评审 D2 协议层断言）
	w4 := doJSON(t, r, http.MethodGet, "/api/v1/favorites", nil, bob.Token)
	if w4.Code != http.StatusOK {
		t.Fatalf("bob GET /favorites = %d", w4.Code)
	}
	if !bytes.Contains(w4.Body.Bytes(), []byte(`"items":[]`)) {
		t.Errorf("空收藏 body = %s, want 含 \"items\":[]", w4.Body.String())
	}
	if bytes.Contains(w4.Body.Bytes(), []byte(`"items":null`)) {
		t.Errorf("空收藏 body = %s, 不得出现 \"items\":null", w4.Body.String())
	}

	// 游客 → 401
	if w := doJSON(t, r, http.MethodGet, "/api/v1/favorites", nil, ""); w.Code != http.StatusUnauthorized {
		t.Errorf("游客 GET /favorites = %d, want 401", w.Code)
	}
}

// TestListPostsHot 验证 #61 热门流 SQL 排序、置顶×窗口、列遮蔽防护、过滤叠加。
func TestListPostsHot(t *testing.T) {
	gdb := testutil.SetupPG(t)
	r := newEngine(t, gdb)

	alice := registerUser(t, r, gdb, "alice", "alice@x.edu", "CODE-H1")
	registerUser(t, r, gdb, "bob", "bob@x.edu", "CODE-H2")
	makeModerator(t, gdb, "bob")
	mod := login(t, r, "bob")

	now := time.Now()

	// p1: 1 赞（1 分）
	p1 := createPost(t, r, alice.Token, 1, "p1 like")
	if w := doJSON(t, r, http.MethodPost, "/api/v1/likes", map[string]any{"target_type": "post", "target_id": p1.ID}, alice.Token); w.Code != http.StatusOK {
		t.Fatalf("like p1 = %d", w.Code)
	}

	// p2: 3 评论（9 分），热度最高
	p2 := createPost(t, r, alice.Token, 1, "p2 comments")
	for i := 0; i < 3; i++ {
		if w := doJSON(t, r, http.MethodPost, "/api/v1/comments", map[string]any{"post_id": p2.ID, "content": fmt.Sprintf("c%d", i)}, alice.Token); w.Code != http.StatusCreated {
			t.Fatalf("comment p2 = %d", w.Code)
		}
	}

	// p3: 0 互动，窗口内置顶 → 应排第一
	p3 := createPost(t, r, alice.Token, 1, "p3 pinned")
	if w := doJSON(t, r, http.MethodPost, fmt.Sprintf("/api/v1/posts/%d/pin", p3.ID), nil, mod.Token); w.Code != http.StatusOK {
		t.Fatalf("pin p3 = %d", w.Code)
	}

	// p4: 8 天前 + 1 评论 → 应被窗口剔除
	p4 := createPost(t, r, alice.Token, 1, "p4 old")
	if w := doJSON(t, r, http.MethodPost, "/api/v1/comments", map[string]any{"post_id": p4.ID, "content": "old"}, alice.Token); w.Code != http.StatusCreated {
		t.Fatalf("comment p4 = %d", w.Code)
	}
	if err := gdb.Exec(`UPDATE content.posts SET created_at = ? WHERE id = ?`, now.Add(-8*24*time.Hour), p4.ID).Error; err != nil {
		t.Fatalf("backdate p4: %v", err)
	}

	// p5: 10 天前 + 置顶 + 高热度 → 验证置顶不豁免窗口（F2 方案 A）
	p5 := createPost(t, r, alice.Token, 1, "p5 pinned old")
	if w := doJSON(t, r, http.MethodPost, "/api/v1/likes", map[string]any{"target_type": "post", "target_id": p5.ID}, alice.Token); w.Code != http.StatusOK {
		t.Fatalf("like p5 = %d", w.Code)
	}
	for i := 0; i < 5; i++ {
		if w := doJSON(t, r, http.MethodPost, "/api/v1/comments", map[string]any{"post_id": p5.ID, "content": fmt.Sprintf("c%d", i)}, alice.Token); w.Code != http.StatusCreated {
			t.Fatalf("comment p5 = %d", w.Code)
		}
	}
	if w := doJSON(t, r, http.MethodPost, fmt.Sprintf("/api/v1/posts/%d/pin", p5.ID), nil, mod.Token); w.Code != http.StatusOK {
		t.Fatalf("pin p5 = %d", w.Code)
	}
	if err := gdb.Exec(`UPDATE content.posts SET created_at = ? WHERE id = ?`, now.Add(-10*24*time.Hour), p5.ID).Error; err != nil {
		t.Fatalf("backdate p5: %v", err)
	}

	// p6: 带 #ai 标签 + 1 赞，用于 hot+tag 叠加
	w6 := doJSON(t, r, http.MethodPost, "/api/v1/posts", map[string]any{"board_id": 1, "title": "p6 tag", "content": "聊聊 #ai"}, alice.Token)
	if w6.Code != http.StatusCreated {
		t.Fatalf("create p6 = %d, body=%s", w6.Code, w6.Body.String())
	}
	var p6 postView
	_ = json.Unmarshal(w6.Body.Bytes(), &p6)
	if w := doJSON(t, r, http.MethodPost, "/api/v1/likes", map[string]any{"target_type": "post", "target_id": p6.ID}, alice.Token); w.Code != http.StatusOK {
		t.Fatalf("like p6 = %d", w.Code)
	}

	// 验证热门段顺序（游客访问）
	w := doJSON(t, r, http.MethodGet, "/api/v1/posts?tab=hot", nil, "")
	if w.Code != http.StatusOK {
		t.Fatalf("GET hot = %d, body=%s", w.Code, w.Body.String())
	}
	var hot listResp
	if err := json.Unmarshal(w.Body.Bytes(), &hot); err != nil {
		t.Fatal(err)
	}
	wantIDs := []int64{p3.ID, p2.ID, p6.ID, p1.ID}
	gotIDs := make([]int64, len(hot.Items))
	for i, v := range hot.Items {
		gotIDs[i] = v.ID
	}
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Errorf("hot 顺序 = %v, want %v", gotIDs, wantIDs)
	}

	// 列遮蔽回归：断言返回字段来自 posts 表，未被 JOIN 列覆盖
	for _, v := range hot.Items {
		if v.ID == p1.ID && v.AuthorName != "alice" {
			t.Errorf("p1 列遮蔽: author=%q", v.AuthorName)
		}
		if v.ID == p3.ID && !v.IsPinned {
			t.Errorf("p3 列遮蔽: is_pinned=%v", v.IsPinned)
		}
	}

	// 超窗帖 p4/p5 不出现
	for _, v := range hot.Items {
		if v.ID == p4.ID || v.ID == p5.ID {
			t.Errorf("超窗帖 %d 不应出现在热门段", v.ID)
		}
	}

	// hot + tag 叠加
	w2 := doJSON(t, r, http.MethodGet, "/api/v1/posts?tab=hot&tag=ai", nil, "")
	if w2.Code != http.StatusOK {
		t.Fatalf("GET hot+tag = %d, body=%s", w2.Code, w2.Body.String())
	}
	var hotTag listResp
	if err := json.Unmarshal(w2.Body.Bytes(), &hotTag); err != nil {
		t.Fatal(err)
	}
	if len(hotTag.Items) != 1 || hotTag.Items[0].ID != p6.ID {
		t.Errorf("hot+tag = %+v, want [p6]", hotTag.Items)
	}
}
