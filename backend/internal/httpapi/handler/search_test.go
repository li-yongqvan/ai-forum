package handler_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/li-yongqvan/ai-forum/backend/internal/testutil"
	"gorm.io/gorm"
)

// searchPost 是本文件用到的帖子视图切片（handler_test 既有的 postView 不含 title，
// 搜索用例的主断言对象正是标题，故本地声明而不改动既有夹具）。
type searchPost struct {
	ID         int64  `json:"id"`
	Title      string `json:"title"`
	AuthorName string `json:"author_name"`
	IsPinned   bool   `json:"is_pinned"`
}

// searchPage 是 GET /posts 的响应形状。total 用指针，并另附原始键集合，
// 用于钉死"非搜索态响应**不含** total 键"（设计文档 §7-2）——只断言 total==0 区分不了
// "键不存在"与"键存在且为 0"。
type searchPage struct {
	Items []searchPost `json:"items"`
	Total *int64       `json:"total"`
}

// searchGet 请求 /api/v1/posts?<query>，要求 200，返回解析结果 + 响应顶层键名（升序）。
func searchGet(t *testing.T, r *gin.Engine, query, token string) (searchPage, []string) {
	t.Helper()
	w := doJSON(t, r, http.MethodGet, "/api/v1/posts?"+query, nil, token)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /posts?%s = %d, body=%s", query, w.Code, w.Body.String())
	}
	var p searchPage
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatalf("解析响应失败: %v (%s)", err, w.Body.String())
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	keys := make([]string, 0, len(raw))
	for k := range raw {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return p, keys
}

func mustCreatePost(t *testing.T, r *gin.Engine, token, title, content string) searchPost {
	t.Helper()
	w := doJSON(t, r, http.MethodPost, "/api/v1/posts", map[string]any{
		"board_id": 1, "title": title, "content": content,
	}, token)
	if w.Code != http.StatusCreated {
		t.Fatalf("发帖 %q 失败: %d %s", title, w.Code, w.Body.String())
	}
	var p searchPost
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	return p
}

func titlesOf(p searchPage) []string {
	out := make([]string, 0, len(p.Items))
	for _, i := range p.Items {
		out = append(out, i.Title)
	}
	return out
}

// TestSearchPosts 是 #78 的检索语义验收面：设计文档 §8-1 的十三条中文用例，逐条跑在
// **testcontainers 真 PG（postgres:16-alpine，与生产同镜像）** 上。
//
// 为什么必须在真库：fake repo 的匹配是朴素 strings.Contains（大小写敏感、不认 ILIKE 转义），
// 与 `ILIKE '%词%'` 不等价 ⇒ 检索语义不得靠 fake 证明（设计文档 §5.1-9）。
// 一个容器跑完全部子例（子例开头清帖），避免每条各起一个 PG 容器。
func TestSearchPosts(t *testing.T) {
	gdb := testutil.SetupPG(t)
	r := newEngine(t, gdb)

	alice := registerUser(t, r, gdb, "alice", "alice@x.edu", "CODE-Q78A")
	bob := registerUser(t, r, gdb, "bob", "bob@x.edu", "CODE-Q78B")
	tok := alice.Token

	// 清帖：TRUNCATE ... CASCADE 连带 comments/favorites/post_tags（外键 RESTRICT，逐表删更啰嗦）。
	// 不动 boards/topics 种子数据与用户。
	reset := func() {
		t.Helper()
		if err := gdb.Exec(`TRUNCATE content.posts CASCADE`).Error; err != nil {
			t.Fatalf("清帖失败: %v", err)
		}
	}
	idOf := func(t *testing.T, title string) int64 {
		t.Helper()
		var id int64
		if err := gdb.Raw(`SELECT id FROM content.posts WHERE title = ?`, title).Scan(&id).Error; err != nil {
			t.Fatalf("按标题取 id 失败 (%s): %v", title, err)
		}
		return id
	}
	pin := func(t *testing.T, title string) {
		t.Helper()
		if err := gdb.Exec(`UPDATE content.posts SET is_pinned = true WHERE id = ?`, idOf(t, title)).Error; err != nil {
			t.Fatalf("置顶失败: %v", err)
		}
	}

	t.Run("1 中文子串召回", func(t *testing.T) {
		reset()
		mustCreatePost(t, r, tok, "周报", "本期 AI 智联论坛 全文搜索上线")
		mustCreatePost(t, r, tok, "无关帖", "正文里没有目标词")
		// 决定性一条：正文含"智联论坛"，搜"论坛"必须命中（to_tsvector('simple') 路线在此为 f）
		got, _ := searchGet(t, r, "q="+urlQuery("论坛"), tok)
		if len(got.Items) != 1 || got.Items[0].Title != "周报" {
			t.Errorf("命中 = %v, want [周报]", titlesOf(got))
		}
	})

	t.Run("2 标题命中优先且置顶不插队", func(t *testing.T) {
		reset()
		// c 更晚发布且被置顶，但只有正文命中；t 更早但标题命中 ⇒ 搜索态 t 必须排在前
		mustCreatePost(t, r, tok, "论坛 专题讨论", "正文无关")
		mustCreatePost(t, r, tok, "第3期公告", "正文里提到论坛一词")
		pin(t, "第3期公告")
		got, _ := searchGet(t, r, "q="+urlQuery("论坛"), tok)
		if len(got.Items) != 2 {
			t.Fatalf("命中 = %v, want 2 条", titlesOf(got))
		}
		if got.Items[0].Title != "论坛 专题讨论" {
			t.Errorf("搜索态首位 = %q, want 论坛 专题讨论（标题命中优先）", got.Items[0].Title)
		}
		if !got.Items[1].IsPinned {
			t.Error("第二条应为置顶帖（说明它被召回了，只是不插队）")
		}
		// 对照：非搜索态置顶仍优先 ⇒ 证明"跳过 is_pinned"只发生在搜索态
		plain, _ := searchGet(t, r, "", tok)
		if plain.Items[0].Title != "第3期公告" {
			t.Errorf("非搜索态首位 = %q, want 第3期公告（置顶优先未受影响）", plain.Items[0].Title)
		}
	})

	// 设计文档 §1 只说"标题命中排前"，§5.1-4 的伪码用 terms[0] 一个词判"标题命中"。
	// 实现取更严的口径：标题须含**全部**词才算标题命中（否则多词查询里"只碰巧带了第一个词"
	// 的标题会压过完整命中的标题）。这条用例把口径钉死——按 terms[0] 实现会翻成 B 在前。
	t.Run("2b 多词时标题命中优先按全部词判定", func(t *testing.T) {
		reset()
		full := mustCreatePost(t, r, tok, "论坛 搜索 上线", "正文无关") // 标题含全部两词（更早发布）
		part := mustCreatePost(t, r, tok, "论坛周刊", "正文提到搜索一词") // 标题只含首词（更晚发布）
		if full.ID == part.ID {
			t.Fatal("两条帖子 id 相同，用例数据不成立")
		}
		got, _ := searchGet(t, r, "q="+urlQuery("论坛 搜索"), tok)
		if len(got.Items) != 2 {
			t.Fatalf("命中 = %v, want 2 条", titlesOf(got))
		}
		if got.Items[0].Title != "论坛 搜索 上线" {
			t.Errorf("首位 = %q, want 论坛 搜索 上线（标题含全部词者优先，而非按 created_at 排在后的那条）", got.Items[0].Title)
		}
	})

	t.Run("3 多词 AND", func(t *testing.T) {
		reset()
		mustCreatePost(t, r, tok, "论坛公告", "正文含搜索")  // 两词分散在标题+正文 ⇒ 每词各自命中 ⇒ 命中
		mustCreatePost(t, r, tok, "只含论坛", "正文无目标词") // 只含其一 ⇒ 不命中
		got, _ := searchGet(t, r, "q="+urlQuery("论坛 搜索"), tok)
		if len(got.Items) != 1 || got.Items[0].Title != "论坛公告" {
			t.Errorf("命中 = %v, want [论坛公告]", titlesOf(got))
		}
	})

	t.Run("4 跨词跨句子串", func(t *testing.T) {
		reset()
		mustCreatePost(t, r, tok, "周报", "AI 智联论坛 全文搜索 测试")
		mustCreatePost(t, r, tok, "另一帖", "文不对题")
		// "文搜索"不是任何完整词，按子串语义必须命中（钉死"按子串而非按词"）
		got, _ := searchGet(t, r, "q="+urlQuery("文搜索"), tok)
		if len(got.Items) != 1 || got.Items[0].Title != "周报" {
			t.Errorf("命中 = %v, want [周报]", titlesOf(got))
		}
	})

	t.Run("5 大小写不敏感", func(t *testing.T) {
		reset()
		mustCreatePost(t, r, tok, "大写帖", "AI Agent 实践")
		mustCreatePost(t, r, tok, "小写帖", "ai agent 实践")
		for _, term := range []string{"ai", "AI", "Ai"} {
			got, _ := searchGet(t, r, "q="+urlQuery(term), tok)
			if len(got.Items) != 2 {
				t.Errorf("搜 %q 命中 %d 条, want 2（ILIKE 不区分大小写，X2）", term, len(got.Items))
			}
		}
	})

	t.Run("6 通配符按字面", func(t *testing.T) {
		reset()
		mustCreatePost(t, r, tok, "达成 100% 目标", "正文")
		mustCreatePost(t, r, tok, "达成 50% 目标", "正文")
		mustCreatePost(t, r, tok, "距今 50 年", "正文没有百分号")
		mustCreatePost(t, r, tok, "完全无关", "正文")

		got, _ := searchGet(t, r, "q="+urlQuery("100%"), tok)
		if len(got.Items) != 1 || got.Items[0].Title != "达成 100% 目标" {
			t.Errorf("搜 100%% 命中 = %v, want [达成 100%% 目标]", titlesOf(got))
		}
		// 决定性一条：若 % 未被转义，`%50%%` 会连带命中"距今 50 年"（% 当前缀通配）
		got2, _ := searchGet(t, r, "q="+urlQuery("50%"), tok)
		if len(got2.Items) != 1 || got2.Items[0].Title != "达成 50% 目标" {
			t.Errorf("搜 50%% 命中 = %v, want 仅 [达成 50%% 目标]（%% 须按字面，不作通配）", titlesOf(got2))
		}
	})

	t.Run("7 下划线按字面", func(t *testing.T) {
		reset()
		mustCreatePost(t, r, tok, "函数 a_b 的用法", "正文")
		mustCreatePost(t, r, tok, "函数 axb 的用法", "正文")
		got, _ := searchGet(t, r, "q="+urlQuery("a_b"), tok)
		if len(got.Items) != 1 || got.Items[0].Title != "函数 a_b 的用法" {
			t.Errorf("搜 a_b 命中 = %v, want 仅字面那条（_ 不作单字符通配）", titlesOf(got))
		}
	})

	t.Run("8 软删不召回", func(t *testing.T) {
		reset()
		keep := mustCreatePost(t, r, tok, "保留帖", "含论坛")
		del := mustCreatePost(t, r, tok, "删除帖", "也含论坛")
		if del.ID == keep.ID {
			t.Fatal("两条帖子 id 相同，夹具异常")
		}
		w := doJSON(t, r, http.MethodDelete, "/api/v1/posts/"+fmt.Sprint(del.ID), nil, tok)
		if w.Code != http.StatusOK {
			t.Fatalf("删帖 = %d, body=%s", w.Code, w.Body.String())
		}
		got, _ := searchGet(t, r, "q="+urlQuery("论坛"), tok)
		if len(got.Items) != 1 || got.Items[0].Title != "保留帖" {
			t.Errorf("命中 = %v, want [保留帖]（软删帖不召回，D5）", titlesOf(got))
		}
		if got.Total == nil || *got.Total != 1 {
			t.Errorf("total = %v, want 1（软删帖同样不计入）", got.Total)
		}
	})

	t.Run("9 封号作者仍召回且注销作者显示已注销", func(t *testing.T) {
		reset()
		mustCreatePost(t, r, tok, "alice 的帖", "含论坛")
		mustCreatePost(t, r, bob.Token, "bob 的帖", "也含论坛")

		// 封号（status=banned）：搜索不加任何治理谓词 ⇒ 照常召回（D5 维持现状）
		if err := gdb.Exec(`UPDATE "user".users SET status='banned' WHERE username='bob'`).Error; err != nil {
			t.Fatalf("封号失败: %v", err)
		}
		got, _ := searchGet(t, r, "q="+urlQuery("论坛"), tok)
		if len(got.Items) != 2 {
			t.Errorf("封号后命中 = %v, want 2 条（搜索不做治理过滤）", titlesOf(got))
		}

		// 作者软删（注销）：帖子仍召回，作者位降级"已注销"——与 Feed 同一 postViews 组装
		if err := gdb.Exec(`UPDATE "user".users SET deleted_at = now() WHERE username='bob'`).Error; err != nil {
			t.Fatalf("注销失败: %v", err)
		}
		if err := gdb.Exec(`UPDATE "user".users SET status='active' WHERE username='bob'`).Error; err != nil {
			t.Fatalf("复原状态失败: %v", err)
		}
		got2, _ := searchGet(t, r, "q="+urlQuery("论坛"), tok)
		if len(got2.Items) != 2 {
			t.Fatalf("注销后命中 = %v, want 2 条", titlesOf(got2))
		}
		var found bool
		for _, i := range got2.Items {
			if i.AuthorName == "已注销" {
				found = true
			}
		}
		if !found {
			t.Errorf("注销作者的帖子未降级显示已注销: %v", got2.Items)
		}
		resetBannedUser(t, gdb)
	})

	t.Run("10 total 与 items 同条件且不随翻页变化", func(t *testing.T) {
		reset()
		for _, s := range []string{"一", "二", "三"} {
			mustCreatePost(t, r, tok, "论坛"+s, "正文含搜索")
		}
		mustCreatePost(t, r, tok, "无关", "正文")
		p1, _ := searchGet(t, r, "q="+urlQuery("搜索")+"&page=1&page_size=2", tok)
		p2, _ := searchGet(t, r, "q="+urlQuery("搜索")+"&page=2&page_size=2", tok)
		if len(p1.Items) != 2 || len(p2.Items) != 1 {
			t.Fatalf("分页 items 数 = %d/%d, want 2/1", len(p1.Items), len(p2.Items))
		}
		if p1.Total == nil || *p1.Total != 3 || p2.Total == nil || *p2.Total != 3 {
			t.Errorf("total = %v/%v, want 均为 3（§7-3：与 items 同条件，且不随翻页变化）", p1.Total, p2.Total)
		}
	})

	t.Run("11 参数错误", func(t *testing.T) {
		reset()
		mustCreatePost(t, r, tok, "论坛帖", "正文")
		cases := []struct {
			name  string
			query string
		}{
			{"短于 2 码点", "q=" + urlQuery("论")},
			{"超过 64 码点", "q=" + urlQuery(strings.Repeat("论", 65))},
			{"超过 4 词", "q=" + urlQuery("a b c d e")},
			// 纯空白：§7-2 写"trim 后为空 ⇒ 走非搜索态"，6-3 写"越界一律拒绝、不静默"。
			// 二者在此冲突，取 6-3（分享一个 q=空格 的链接若静默变成全站 Feed，比报错更坏）；
			// 前端 Search.vue 的 invalidTip 已先拦住，这条钉的是绕过前端的直接请求。
			{"纯空白（§7-2/6-3 冲突取拒绝）", "q=" + urlQuery("   ")},
			// #81 热修：GBK percent-encoding 的非法 UTF-8 字节（如 q=%FF）此前直达 PG 报
			// SQLSTATE 22021 → 500；现由 NormalizeQuery 拦为 ErrInvalidQuery → 400。
			// %FF 是合法 percent-encoding（Decode 出 0xFF 字节），ParseQuery 不会拒收。
			{"非法 UTF-8 字节（#81：500→400）", "q=%FF%FE"},
		}
		for _, c := range cases {
			w := doJSON(t, r, http.MethodGet, "/api/v1/posts?"+c.query, nil, tok)
			if w.Code != http.StatusBadRequest {
				t.Errorf("%s: code = %d, want 400（body=%s）", c.name, w.Code, w.Body.String())
				continue
			}
			var body map[string]any
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatalf("%s: 解析错误体失败: %v", c.name, err)
			}
			// 既有错误体形状 = {"error": "参数不合法"}（respondError），不新造形状
			if body["error"] != "参数不合法" || len(body) != 1 {
				t.Errorf("%s: 错误体 = %v, want 仅 {error: 参数不合法}", c.name, body)
			}
		}
	})

	t.Run("12 游客可搜", func(t *testing.T) {
		reset()
		mustCreatePost(t, r, tok, "游客可见帖", "正文含论坛")
		got, keys := searchGet(t, r, "q="+urlQuery("论坛"), "")
		if len(got.Items) != 1 {
			t.Errorf("游客命中 = %v, want 1 条（D4 游客可搜）", titlesOf(got))
		}
		if got.Total == nil || *got.Total != 1 {
			t.Errorf("游客 total = %v, want 1", got.Total)
		}
		if !hasKey(keys, "total") {
			t.Errorf("搜索态响应键 = %v, want 含 total", keys)
		}
	})

	t.Run("13 非搜索态响应逐字节不变", func(t *testing.T) {
		reset()
		mustCreatePost(t, r, tok, "普通帖", "正文")
		mustCreatePost(t, r, tok, "置顶帖", "正文")
		pin(t, "置顶帖")
		for _, query := range []string{"", "tab=all", "page=1&page_size=20"} {
			got, keys := searchGet(t, r, query, tok)
			if hasKey(keys, "total") {
				t.Errorf("q=%q 响应键 = %v, want 不含 total（§7-2）", query, keys)
			}
			if got.Total != nil {
				t.Errorf("q=%q total = %v, want 键不存在", query, *got.Total)
			}
			if got.Items[0].Title != "置顶帖" {
				t.Errorf("q=%q 首位 = %q, want 置顶帖（排序仍含 is_pinned DESC）", query, got.Items[0].Title)
			}
		}
	})
}

// resetBannedUser 复原第 9 子例对被注销用户的改动，避免污染后续用例（同容器共享）。
func resetBannedUser(t *testing.T, gdb *gorm.DB) {
	t.Helper()
	if err := gdb.Exec(`UPDATE "user".users SET deleted_at = NULL WHERE username='bob'`).Error; err != nil {
		t.Fatalf("复原注销状态失败: %v", err)
	}
}

// hasKey 判断响应顶层键名升序切片是否含某键（键集合来自 json.RawMessage 解出的 map）。
func hasKey(keys []string, k string) bool {
	i := sort.SearchStrings(keys, k)
	return i < len(keys) && keys[i] == k
}

// urlQuery 把搜索词编码成 query 参数值。用 url.QueryEscape：空格 → +（服务端
// ParseQuery 还原为空格，与前端 URLSearchParams 的真实链路一致），% → %25、_ → %5F
// 等元字符按字面送达——正是"转义发生在服务端而非客户端"要证明的那一半。
func urlQuery(s string) string {
	return url.QueryEscape(s)
}
