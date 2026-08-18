package handler_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

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
