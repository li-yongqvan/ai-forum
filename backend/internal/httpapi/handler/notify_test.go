package handler_test

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/li-yongqvan/ai-forum/backend/internal/testutil"
	"gorm.io/gorm"
)

type notifView struct {
	ID        int64   `json:"id"`
	Type      string  `json:"type"`
	ActorID   *int64  `json:"actor_id"`
	ActorName *string `json:"actor_name"`
	IsRead    bool    `json:"is_read"`
}

type notifList struct {
	Items []notifView `json:"items"`
}

func registerNotifyUser(t *testing.T, r *gin.Engine, gdb *gorm.DB, username, code string) authResp {
	t.Helper()
	return registerUser(t, r, gdb, username, username+"@x.edu", code)
}

// 通知接口需登录（#9 登录墙：游客 401）。
func TestNotificationsRequireAuth(t *testing.T) {
	gdb := testutil.SetupPG(t)
	r := newEngine(t, gdb)

	for _, path := range []string{"/api/v1/notifications", "/api/v1/notifications/unread_count"} {
		w := doJSON(t, r, http.MethodGet, path, nil, "")
		if w.Code != http.StatusUnauthorized {
			t.Errorf("GET %s 无 token = %d, want 401", path, w.Code)
		}
	}
}

// 关注用户 → 被关注者收到 follow 通知（#32 主链路：仅关注生成通知）。
func TestFollowGeneratesNotification(t *testing.T) {
	gdb := testutil.SetupPG(t)
	r := newEngine(t, gdb)

	alice := registerNotifyUser(t, r, gdb, "alice", "NF1")
	bob := registerNotifyUser(t, r, gdb, "bob", "NF2")

	// bob 关注 alice
	w := doJSON(t, r, http.MethodPost, "/api/v1/follows", map[string]any{
		"target_type": "user", "target_id": alice.User.ID,
	}, bob.Token)
	if w.Code != http.StatusOK {
		t.Fatalf("follow = %d, body=%s", w.Code, w.Body.String())
	}

	// alice 未读数 = 1
	w2 := doJSON(t, r, http.MethodGet, "/api/v1/notifications/unread_count", nil, alice.Token)
	if w2.Code != http.StatusOK {
		t.Fatalf("unread_count = %d, body=%s", w2.Code, w2.Body.String())
	}
	var cnt struct{ Count int64 `json:"count"` }
	if err := json.Unmarshal(w2.Body.Bytes(), &cnt); err != nil {
		t.Fatal(err)
	}
	if cnt.Count != 1 {
		t.Errorf("未读数 = %d, want 1", cnt.Count)
	}

	// alice 列表含 1 条 follow 通知，actor = bob、未读
	w3 := doJSON(t, r, http.MethodGet, "/api/v1/notifications", nil, alice.Token)
	if w3.Code != http.StatusOK {
		t.Fatalf("notifications = %d, body=%s", w3.Code, w3.Body.String())
	}
	var list notifList
	if err := json.Unmarshal(w3.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 1 {
		t.Fatalf("通知数 = %d, want 1", len(list.Items))
	}
	it := list.Items[0]
	if it.Type != "follow" || it.ActorName == nil || *it.ActorName != "bob" || it.IsRead {
		t.Errorf("通知项 = %+v, want follow by bob 未读", it)
	}

	// bob 自己的列表为空（不泄露他人通知）
	w4 := doJSON(t, r, http.MethodGet, "/api/v1/notifications", nil, bob.Token)
	if w4.Code != http.StatusOK {
		t.Fatalf("bob notifications = %d", w4.Code)
	}
	var blist notifList
	if err := json.Unmarshal(w4.Body.Bytes(), &blist); err != nil {
		t.Fatal(err)
	}
	if len(blist.Items) != 0 {
		t.Errorf("bob 通知数 = %d, want 0", len(blist.Items))
	}
}

// 单条已读 + 全部已读 + 归属隔离（非本人已读 → 404）；重复关注不重复生成通知。
func TestMarkReadAndAll(t *testing.T) {
	gdb := testutil.SetupPG(t)
	r := newEngine(t, gdb)

	alice := registerNotifyUser(t, r, gdb, "alice", "NF3")
	bob := registerNotifyUser(t, r, gdb, "bob", "NF4")

	// 重复关注同一人第二次 → 409（且不生成第二条通知）
	w := doJSON(t, r, http.MethodPost, "/api/v1/follows", map[string]any{
		"target_type": "user", "target_id": alice.User.ID,
	}, bob.Token)
	if w.Code != http.StatusOK {
		t.Fatalf("follow = %d, body=%s", w.Code, w.Body.String())
	}
	wdup := doJSON(t, r, http.MethodPost, "/api/v1/follows", map[string]any{
		"target_type": "user", "target_id": alice.User.ID,
	}, bob.Token)
	if wdup.Code != http.StatusConflict {
		t.Errorf("重复 follow = %d, want 409", wdup.Code)
	}

	// 单条已读 id=1
	w2 := doJSON(t, r, http.MethodPost, "/api/v1/notifications/1/read", nil, alice.Token)
	if w2.Code != http.StatusOK {
		t.Fatalf("mark read = %d, body=%s", w2.Code, w2.Body.String())
	}
	// bob 标记 alice 的通知 → 404（归属隔离）
	w3 := doJSON(t, r, http.MethodPost, "/api/v1/notifications/1/read", nil, bob.Token)
	if w3.Code != http.StatusNotFound {
		t.Errorf("bob 标记他人通知 = %d, want 404", w3.Code)
	}
	// 未读数归零 + 列表 is_read=true
	var cnt struct{ Count int64 `json:"count"` }
	w4 := doJSON(t, r, http.MethodGet, "/api/v1/notifications/unread_count", nil, alice.Token)
	if err := json.Unmarshal(w4.Body.Bytes(), &cnt); err != nil {
		t.Fatal(err)
	}
	if cnt.Count != 0 {
		t.Errorf("单条已读后未读数 = %d, want 0", cnt.Count)
	}
	w5 := doJSON(t, r, http.MethodGet, "/api/v1/notifications", nil, alice.Token)
	var list notifList
	if err := json.Unmarshal(w5.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 1 || !list.Items[0].IsRead {
		t.Errorf("已读后列表 = %+v, want is_read=true", list.Items)
	}

	// 第三人关注 → 新增一条未读 → 全部已读
	carol := registerNotifyUser(t, r, gdb, "carol", "NF5")
	w7 := doJSON(t, r, http.MethodPost, "/api/v1/follows", map[string]any{
		"target_type": "user", "target_id": alice.User.ID,
	}, carol.Token)
	if w7.Code != http.StatusOK {
		t.Fatalf("carol follow = %d", w7.Code)
	}
	w8 := doJSON(t, r, http.MethodPost, "/api/v1/notifications/read-all", nil, alice.Token)
	if w8.Code != http.StatusOK {
		t.Fatalf("read-all = %d, body=%s", w8.Code, w8.Body.String())
	}
	w9 := doJSON(t, r, http.MethodGet, "/api/v1/notifications/unread_count", nil, alice.Token)
	if err := json.Unmarshal(w9.Body.Bytes(), &cnt); err != nil {
		t.Fatal(err)
	}
	if cnt.Count != 0 {
		t.Errorf("全部已读后未读数 = %d, want 0", cnt.Count)
	}
}

// ?unread=1 只回未读（分页过滤语义）。
func TestNotificationsUnreadFilter(t *testing.T) {
	gdb := testutil.SetupPG(t)
	r := newEngine(t, gdb)

	alice := registerNotifyUser(t, r, gdb, "alice", "NF6")
	bob := registerNotifyUser(t, r, gdb, "bob", "NF7")
	carol := registerNotifyUser(t, r, gdb, "carol", "NF8")

	for _, who := range []authResp{bob, carol} {
		w := doJSON(t, r, http.MethodPost, "/api/v1/follows", map[string]any{
			"target_type": "user", "target_id": alice.User.ID,
		}, who.Token)
		if w.Code != http.StatusOK {
			t.Fatalf("follow = %d", w.Code)
		}
	}
	// 已读 id=1，剩 id=2 未读
	w := doJSON(t, r, http.MethodPost, "/api/v1/notifications/1/read", nil, alice.Token)
	if w.Code != http.StatusOK {
		t.Fatalf("mark read = %d", w.Code)
	}
	w2 := doJSON(t, r, http.MethodGet, "/api/v1/notifications?unread=1", nil, alice.Token)
	var list notifList
	if err := json.Unmarshal(w2.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 1 || list.Items[0].ID != 2 {
		t.Errorf("unread 过滤 = %+v, want 仅 id 2", list.Items)
	}
}

// ---- 私信（#59） ----

type messageResp struct {
	ID         int64  `json:"id"`
	FromUserID int64  `json:"from_user_id"`
	ToUserID   int64  `json:"to_user_id"`
	Content    string `json:"content"`
	IsRead     bool   `json:"is_read"`
}

type messageList struct {
	Items []messageResp `json:"items"`
}

type conversationList struct {
	Items []struct {
		PeerID      int64       `json:"peer_id"`
		PeerName    string      `json:"peer_name"`
		LastMessage messageResp `json:"last_message"`
		UnreadCount int64       `json:"unread_count"`
	} `json:"items"`
}

// 私信接口需登录。
func TestMessagesRequireAuth(t *testing.T) {
	gdb := testutil.SetupPG(t)
	r := newEngine(t, gdb)

	for _, tc := range []struct {
		method, path string
	}{
		{http.MethodGet, "/api/v1/conversations"},
		{http.MethodGet, "/api/v1/conversations/1/messages"},
		{http.MethodPost, "/api/v1/messages"},
		{http.MethodPost, "/api/v1/conversations/1/read"},
	} {
		w := doJSON(t, r, tc.method, tc.path, nil, "")
		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s %s 无 token = %d, want 401", tc.method, tc.path, w.Code)
		}
	}
}

// A 给 B 发私信 → B 看到会话 + 未读；B 读会话 → 未读归零。
func TestDirectMessageFlow(t *testing.T) {
	gdb := testutil.SetupPG(t)
	r := newEngine(t, gdb)

	alice := registerNotifyUser(t, r, gdb, "alice", "DM1")
	bob := registerNotifyUser(t, r, gdb, "bob", "DM2")

	// alice 给 bob 发私信
	w := doJSON(t, r, http.MethodPost, "/api/v1/messages", map[string]any{
		"to_user_id": bob.User.ID,
		"content":    "hello bob",
	}, alice.Token)
	if w.Code != http.StatusOK {
		t.Fatalf("send message = %d, body=%s", w.Code, w.Body.String())
	}
	var sent messageResp
	if err := json.Unmarshal(w.Body.Bytes(), &sent); err != nil {
		t.Fatal(err)
	}
	if sent.Content != "hello bob" || sent.FromUserID != alice.User.ID || sent.ToUserID != bob.User.ID {
		t.Errorf("sent = %+v", sent)
	}

	// bob 未读数 = 1（合并通知+私信）
	var cnt struct{ Count int64 `json:"count"` }
	w2 := doJSON(t, r, http.MethodGet, "/api/v1/notifications/unread_count", nil, bob.Token)
	if err := json.Unmarshal(w2.Body.Bytes(), &cnt); err != nil {
		t.Fatal(err)
	}
	if cnt.Count != 1 {
		t.Errorf("bob 未读数 = %d, want 1", cnt.Count)
	}

	// bob 会话列表 1 条，未读 1
	w3 := doJSON(t, r, http.MethodGet, "/api/v1/conversations", nil, bob.Token)
	var clist conversationList
	if err := json.Unmarshal(w3.Body.Bytes(), &clist); err != nil {
		t.Fatal(err)
	}
	if len(clist.Items) != 1 || clist.Items[0].PeerID != alice.User.ID || clist.Items[0].UnreadCount != 1 {
		t.Errorf("bob 会话 = %+v, want 1 条来自 alice 未读", clist.Items)
	}

	// bob 读会话
	w4 := doJSON(t, r, http.MethodPost, "/api/v1/conversations/"+strconv.FormatInt(alice.User.ID, 10)+"/read", nil, bob.Token)
	if w4.Code != http.StatusOK {
		t.Fatalf("mark conversation read = %d, body=%s", w4.Code, w4.Body.String())
	}

	// 未读数归零
	w5 := doJSON(t, r, http.MethodGet, "/api/v1/notifications/unread_count", nil, bob.Token)
	if err := json.Unmarshal(w5.Body.Bytes(), &cnt); err != nil {
		t.Fatal(err)
	}
	if cnt.Count != 0 {
		t.Errorf("已读后未读数 = %d, want 0", cnt.Count)
	}
}

// 私信校验：自发言/接收人不存在/超长/限频。
func TestSendMessageValidation(t *testing.T) {
	gdb := testutil.SetupPG(t)
	r := newEngine(t, gdb)

	alice := registerNotifyUser(t, r, gdb, "alice", "DM3")
	bob := registerNotifyUser(t, r, gdb, "bob", "DM4")

	cases := []struct {
		name    string
		body    map[string]any
		token   string
		want    int
	}{
		{"self", map[string]any{"to_user_id": alice.User.ID, "content": "hi"}, alice.Token, http.StatusBadRequest},
		{"not found", map[string]any{"to_user_id": 99999, "content": "hi"}, alice.Token, http.StatusNotFound},
		{"empty", map[string]any{"to_user_id": bob.User.ID, "content": "   "}, alice.Token, http.StatusBadRequest},
		{"too long", map[string]any{"to_user_id": bob.User.ID, "content": string(make([]byte, 2001))}, alice.Token, http.StatusBadRequest},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := doJSON(t, r, http.MethodPost, "/api/v1/messages", c.body, c.token)
			if w.Code != c.want {
				t.Errorf("%s = %d, want %d", c.name, w.Code, c.want)
			}
		})
	}

	// 限频：对同一对端连续发两条，第二条 429
	w := doJSON(t, r, http.MethodPost, "/api/v1/messages", map[string]any{
		"to_user_id": bob.User.ID, "content": "a",
	}, alice.Token)
	if w.Code != http.StatusOK {
		t.Fatalf("first send = %d", w.Code)
	}
	w2 := doJSON(t, r, http.MethodPost, "/api/v1/messages", map[string]any{
		"to_user_id": bob.User.ID, "content": "b",
	}, alice.Token)
	if w2.Code != http.StatusTooManyRequests {
		t.Errorf("rate limit = %d, want 429", w2.Code)
	}
}
