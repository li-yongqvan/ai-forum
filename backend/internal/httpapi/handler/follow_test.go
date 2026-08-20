package handler_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/li-yongqvan/ai-forum/backend/internal/testutil"
)

type followUserRow struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Viewer   *struct {
		Following bool `json:"following"`
	} `json:"viewer"`
}

type followBoardRow struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Viewer *struct {
		Following bool `json:"following"`
	} `json:"viewer"`
}

type followTopicRow struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	BoardID int64  `json:"board_id"`
	Viewer  *struct {
		Following bool `json:"following"`
	} `json:"viewer"`
}

// TestFollowsList：GET /api/v1/follows?target_type=（#23）。真 PG 是 GORM JOIN + Select 列遮蔽
// （评审 D1）与软删过滤唯一能暴露的闸门，故断言**真实主键**而非仅顺序/长度。
func TestFollowsList(t *testing.T) {
	gdb := testutil.SetupPG(t)
	r := newEngine(t, gdb)

	alice := registerUser(t, r, gdb, "alice", "alice@x.edu", "CODE-FS1")
	bob := registerUser(t, r, gdb, "bob", "bob@x.edu", "CODE-FS2")
	carol := registerUser(t, r, gdb, "carol", "carol@x.edu", "CODE-FS3")
	dave := registerUser(t, r, gdb, "dave", "dave@x.edu", "CODE-FS4")

	follow := func(token, tt string, id int64) {
		t.Helper()
		if w := doJSON(t, r, http.MethodPost, "/api/v1/follows", map[string]any{"target_type": tt, "target_id": id}, token); w.Code != http.StatusOK {
			t.Fatalf("follow %s %d = %d, body=%s", tt, id, w.Code, w.Body.String())
		}
	}
	unfollow := func(token, tt string, id int64) {
		t.Helper()
		if w := doJSON(t, r, http.MethodDelete, fmt.Sprintf("/api/v1/follows?target_type=%s&target_id=%d", tt, id), nil, token); w.Code != http.StatusOK {
			t.Fatalf("unfollow %s %d = %d, body=%s", tt, id, w.Code, w.Body.String())
		}
	}
	// alice 依次关注 bob → carol（关注时间 bob < carol → 倒序 carol 在前）
	follow(alice.Token, "user", bob.User.ID)
	follow(alice.Token, "user", carol.User.ID)
	// 板块 1、话题 2（RAG，种子数据）
	follow(alice.Token, "board", 1)
	follow(alice.Token, "topic", 2)

	// 关注用户列表：真实主键 + 顺序 + viewer.following
	w := doJSON(t, r, http.MethodGet, "/api/v1/follows?target_type=user", nil, alice.Token)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /follows?target_type=user = %d, body=%s", w.Code, w.Body.String())
	}
	var users struct {
		Items []followUserRow `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &users); err != nil {
		t.Fatal(err)
	}
	if len(users.Items) != 2 || users.Items[0].ID != carol.User.ID || users.Items[1].ID != bob.User.ID {
		t.Errorf("关注用户 = %d 条, want [carol(%d) bob(%d)]（关注时间倒序，真实主键）", len(users.Items), carol.User.ID, bob.User.ID)
	}
	for _, it := range users.Items {
		if it.Viewer == nil || !it.Viewer.Following {
			t.Errorf("user 行 viewer.following 应为 true, got %+v", it.Viewer)
		}
	}

	// 关注板块/话题列表：真实主键
	wb := doJSON(t, r, http.MethodGet, "/api/v1/follows?target_type=board", nil, alice.Token)
	var boards struct {
		Items []followBoardRow `json:"items"`
	}
	if err := json.Unmarshal(wb.Body.Bytes(), &boards); err != nil {
		t.Fatal(err)
	}
	if len(boards.Items) != 1 || boards.Items[0].ID != 1 || boards.Items[0].Viewer == nil || !boards.Items[0].Viewer.Following {
		t.Errorf("关注板块 = %+v, want [板块1] viewer.following=true", boards.Items)
	}
	wt := doJSON(t, r, http.MethodGet, "/api/v1/follows?target_type=topic", nil, alice.Token)
	var topics struct {
		Items []followTopicRow `json:"items"`
	}
	if err := json.Unmarshal(wt.Body.Bytes(), &topics); err != nil {
		t.Fatal(err)
	}
	if len(topics.Items) != 1 || topics.Items[0].ID != 2 || topics.Items[0].Viewer == nil || !topics.Items[0].Viewer.Following {
		t.Errorf("关注话题 = %+v, want [话题2] viewer.following=true", topics.Items)
	}

	// 非法/缺失 target_type → 400
	if w := doJSON(t, r, http.MethodGet, "/api/v1/follows?target_type=bogus", nil, alice.Token); w.Code != http.StatusBadRequest {
		t.Errorf("bogus target_type = %d, want 400", w.Code)
	}
	if w := doJSON(t, r, http.MethodGet, "/api/v1/follows", nil, alice.Token); w.Code != http.StatusBadRequest {
		t.Errorf("缺失 target_type = %d, want 400", w.Code)
	}

	// 游客 → 401
	if w := doJSON(t, r, http.MethodGet, "/api/v1/follows?target_type=user", nil, ""); w.Code != http.StatusUnauthorized {
		t.Errorf("游客 GET /follows = %d, want 401", w.Code)
	}

	// 分页：target_type=user&page_size=1 → 最新关注 carol
	wp := doJSON(t, r, http.MethodGet, "/api/v1/follows?target_type=user&page_size=1", nil, alice.Token)
	var usersPage struct {
		Items []followUserRow `json:"items"`
	}
	_ = json.Unmarshal(wp.Body.Bytes(), &usersPage)
	if len(usersPage.Items) != 1 || usersPage.Items[0].ID != carol.User.ID {
		t.Errorf("page_size=1 = %d 条, want [carol]", len(usersPage.Items))
	}

	// 取关 bob → 关注用户列表只剩 carol
	unfollow(alice.Token, "user", bob.User.ID)
	wa := doJSON(t, r, http.MethodGet, "/api/v1/follows?target_type=user", nil, alice.Token)
	var usersAfter struct {
		Items []followUserRow `json:"items"`
	}
	_ = json.Unmarshal(wa.Body.Bytes(), &usersAfter)
	if len(usersAfter.Items) != 1 || usersAfter.Items[0].ID != carol.User.ID {
		t.Errorf("取关后关注用户 = %d 条, want 仅 [carol]", len(usersAfter.Items))
	}

	// 软删用户不出现在关注列表（评审 D5）
	if err := gdb.Exec(`UPDATE "user".users SET deleted_at = now() WHERE id = ?`, carol.User.ID).Error; err != nil {
		t.Fatalf("软删 carol 失败: %v", err)
	}
	ws := doJSON(t, r, http.MethodGet, "/api/v1/follows?target_type=user", nil, alice.Token)
	var usersSoft struct {
		Items []followUserRow `json:"items"`
	}
	_ = json.Unmarshal(ws.Body.Bytes(), &usersSoft)
	if len(usersSoft.Items) != 0 {
		t.Errorf("软删后关注用户 = %d 条, want 空", len(usersSoft.Items))
	}

	// 软删板块不出现在关注列表（评审 D5）
	if err := gdb.Exec(`UPDATE content.boards SET deleted_at = now() WHERE id = 1`).Error; err != nil {
		t.Fatalf("软删板块1 失败: %v", err)
	}
	wbs := doJSON(t, r, http.MethodGet, "/api/v1/follows?target_type=board", nil, alice.Token)
	var boardsSoft struct {
		Items []followBoardRow `json:"items"`
	}
	_ = json.Unmarshal(wbs.Body.Bytes(), &boardsSoft)
	if len(boardsSoft.Items) != 0 {
		t.Errorf("软删后关注板块 = %d 条, want 空", len(boardsSoft.Items))
	}

	// 空态信封：无任何关注的 dave → "items":[] 而非 null（评审 D2 协议层断言）
	we := doJSON(t, r, http.MethodGet, "/api/v1/follows?target_type=user", nil, dave.Token)
	if we.Code != http.StatusOK {
		t.Fatalf("dave GET /follows = %d", we.Code)
	}
	if !bytes.Contains(we.Body.Bytes(), []byte(`"items":[]`)) {
		t.Errorf("空关注 body = %s, want 含 \"items\":[]", we.Body.String())
	}
	if bytes.Contains(we.Body.Bytes(), []byte(`"items":null`)) {
		t.Errorf("空关注 body = %s, 不得出现 \"items\":null", we.Body.String())
	}
}
