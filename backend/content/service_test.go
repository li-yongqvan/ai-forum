package content

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"
)

func ctx() context.Context { return context.Background() }

// newContentService 预置一个带板块/话题/作者名的基础环境。
func newContentService() (*fakeRepo, Service) {
	f := newFakeRepo()
	f.seedBoard(1, "学习讨论")
	f.seedBoard(2, "项目实战")
	f.seedTopic(1, 1, "RAG")
	f.seedTopic(2, 2, "Agent")
	u := fakeUsers{names: map[int64]string{1: "alice", 2: "bob", 3: "mod", 4: "admin"}}
	return f, newServiceWith(f, u)
}

func TestCreatePost(t *testing.T) {
	t.Run("成功（含话题）", func(t *testing.T) {
		_, svc := newContentService()
		topicID := int64(1)
		view, err := svc.CreatePost(ctx(), CreatePostCmd{AuthorID: 1, BoardID: 1, TopicID: &topicID, Title: "标题", Content: "内容"})
		if err != nil {
			t.Fatalf("CreatePost() error = %v", err)
		}
		if view.BoardName != "学习讨论" || view.AuthorName != "alice" {
			t.Errorf("view = %+v, want 板块名/作者名", view)
		}
		if view.TopicName == nil || *view.TopicName != "RAG" {
			t.Errorf("TopicName = %v, want RAG", view.TopicName)
		}
	})

	t.Run("板块不存在", func(t *testing.T) {
		_, svc := newContentService()
		_, err := svc.CreatePost(ctx(), CreatePostCmd{AuthorID: 1, BoardID: 99, Title: "t", Content: "c"})
		if !errors.Is(err, ErrBoardNotFound) {
			t.Errorf("error = %v, want ErrBoardNotFound", err)
		}
	})

	t.Run("话题不属于该板块", func(t *testing.T) {
		_, svc := newContentService()
		topicID := int64(2) // 属于板块 2
		_, err := svc.CreatePost(ctx(), CreatePostCmd{AuthorID: 1, BoardID: 1, TopicID: &topicID, Title: "t", Content: "c"})
		if !errors.Is(err, ErrTopicNotInBoard) {
			t.Errorf("error = %v, want ErrTopicNotInBoard", err)
		}
	})

	t.Run("空标题", func(t *testing.T) {
		_, svc := newContentService()
		_, err := svc.CreatePost(ctx(), CreatePostCmd{AuthorID: 1, BoardID: 1, Title: "  ", Content: "c"})
		if !errors.Is(err, ErrContentEmpty) {
			t.Errorf("error = %v, want ErrContentEmpty", err)
		}
	})
}

func TestCreateComment(t *testing.T) {
	t.Run("顶层评论分配楼层", func(t *testing.T) {
		f, svc := newContentService()
		f.seedPost(1, 1, 1)
		c, err := svc.CreateComment(ctx(), CreateCommentCmd{PostID: 1, AuthorID: 2, Content: "评论1"})
		if err != nil {
			t.Fatalf("CreateComment() error = %v", err)
		}
		if c.Floor == nil || *c.Floor != 1 {
			t.Errorf("顶层评论 floor = %v, want 1", c.Floor)
		}
		if c.Content != "评论1" {
			t.Errorf("评论 content = %q, want 评论1", c.Content)
		}
		// 第二条顶层 → floor 2
		c2, err := svc.CreateComment(ctx(), CreateCommentCmd{PostID: 1, AuthorID: 2, Content: "评论2"})
		if err != nil || c2.Floor == nil || *c2.Floor != 2 {
			t.Errorf("第二条顶层 floor = %v, err = %v", c2.Floor, err)
		}
	})

	t.Run("回复不占楼层", func(t *testing.T) {
		f, svc := newContentService()
		f.seedPost(1, 1, 1)
		top, _ := svc.CreateComment(ctx(), CreateCommentCmd{PostID: 1, AuthorID: 2, Content: "顶层"})
		parent := top.ID
		reply, err := svc.CreateComment(ctx(), CreateCommentCmd{PostID: 1, AuthorID: 3, ParentID: &parent, Content: "回复"})
		if err != nil {
			t.Fatalf("CreateComment() error = %v", err)
		}
		if reply.Floor != nil {
			t.Errorf("回复 floor = %v, want nil", reply.Floor)
		}
	})

	t.Run("父评论不属于该帖子", func(t *testing.T) {
		f, svc := newContentService()
		f.seedPost(1, 1, 1)
		f.seedPost(2, 2, 2)
		c, _ := svc.CreateComment(ctx(), CreateCommentCmd{PostID: 2, AuthorID: 2, Content: "别的帖子的评论"})
		_, err := svc.CreateComment(ctx(), CreateCommentCmd{PostID: 1, AuthorID: 3, ParentID: &c.ID, Content: "错挂"})
		if !errors.Is(err, ErrParentNotInPost) {
			t.Errorf("error = %v, want ErrParentNotInPost", err)
		}
	})

	t.Run("帖子不存在", func(t *testing.T) {
		_, svc := newContentService()
		_, err := svc.CreateComment(ctx(), CreateCommentCmd{PostID: 99, AuthorID: 1, Content: "x"})
		if !errors.Is(err, ErrPostNotFound) {
			t.Errorf("error = %v, want ErrPostNotFound", err)
		}
	})
}

func TestLikeAndFavorite(t *testing.T) {
	t.Run("点赞成功与去重", func(t *testing.T) {
		f, svc := newContentService()
		f.seedPost(1, 1, 1)
		if err := svc.Like(ctx(), LikeCmd{UserID: 2, TargetType: "post", TargetID: 1}); err != nil {
			t.Fatalf("Like() error = %v", err)
		}
		if err := svc.Like(ctx(), LikeCmd{UserID: 2, TargetType: "post", TargetID: 1}); !errors.Is(err, ErrAlreadyLiked) {
			t.Errorf("重复点赞 error = %v, want ErrAlreadyLiked", err)
		}
	})

	t.Run("非法目标类型", func(t *testing.T) {
		_, svc := newContentService()
		err := svc.Like(ctx(), LikeCmd{UserID: 2, TargetType: "book", TargetID: 1})
		if !errors.Is(err, ErrInvalidTargetType) {
			t.Errorf("error = %v, want ErrInvalidTargetType", err)
		}
	})

	t.Run("点赞不存在的帖子", func(t *testing.T) {
		_, svc := newContentService()
		err := svc.Like(ctx(), LikeCmd{UserID: 2, TargetType: "post", TargetID: 99})
		if !errors.Is(err, ErrPostNotFound) {
			t.Errorf("error = %v, want ErrPostNotFound", err)
		}
	})

	t.Run("取消点赞幂等", func(t *testing.T) {
		_, svc := newContentService()
		if err := svc.Unlike(ctx(), LikeCmd{UserID: 2, TargetType: "post", TargetID: 1}); err != nil {
			t.Errorf("Unlike() 应幂等成功, error = %v", err)
		}
	})

	t.Run("收藏与取藏", func(t *testing.T) {
		f, svc := newContentService()
		f.seedPost(1, 1, 1)
		if err := svc.Favorite(ctx(), FavoriteCmd{UserID: 2, PostID: 1}); err != nil {
			t.Fatalf("Favorite() error = %v", err)
		}
		if err := svc.Favorite(ctx(), FavoriteCmd{UserID: 2, PostID: 1}); !errors.Is(err, ErrAlreadyFavorited) {
			t.Errorf("重复收藏 error = %v, want ErrAlreadyFavorited", err)
		}
		if err := svc.Unfavorite(ctx(), FavoriteCmd{UserID: 2, PostID: 1}); err != nil {
			t.Errorf("Unfavorite() error = %v", err)
		}
		if err := svc.Unfavorite(ctx(), FavoriteCmd{UserID: 2, PostID: 1}); err != nil {
			t.Errorf("Unfavorite() 幂等 error = %v", err)
		}
	})
}

func TestGetPost(t *testing.T) {
	f, svc := newContentService()
	f.seedPost(1, 1, 1, withCreatedAt(time.Now().Add(-time.Hour)))
	// 预置点赞/收藏数据
	_ = f.CreateLike(ctx(), &Like{UserID: 2, TargetType: "post", TargetID: 1})
	_ = f.CreateLike(ctx(), &Like{UserID: 3, TargetType: "post", TargetID: 1})
	_ = f.CreateFavorite(ctx(), &Favorite{UserID: 2, PostID: 1})

	view, err := svc.GetPost(ctx(), GetPostQuery{PostID: 1, ViewerID: 2})
	if err != nil {
		t.Fatalf("GetPost() error = %v", err)
	}
	if view.ViewCount != 1 {
		t.Errorf("view_count = %d, want 1（GET 自增）", view.ViewCount)
	}
	if view.LikeCount != 2 || view.FavoriteCount != 1 {
		t.Errorf("counts = like %d fav %d, want 2/1", view.LikeCount, view.FavoriteCount)
	}
	if view.Viewer == nil || !view.Viewer.Liked || !view.Viewer.Favorited {
		t.Errorf("viewer 2 应 liked/favorited 为真: %+v", view.Viewer)
	}

	// 游客视角：viewer 不附（nil）
	guest, err := svc.GetPost(ctx(), GetPostQuery{PostID: 1, ViewerID: 0})
	if err != nil {
		t.Fatal(err)
	}
	if guest.Viewer != nil {
		t.Error("游客 viewer 应为 nil")
	}
	if guest.ViewCount != 2 {
		t.Errorf("第二次 GET view_count = %d, want 2", guest.ViewCount)
	}
}

func TestListFeed_All(t *testing.T) {
	base := time.Now().Add(-48 * time.Hour)
	f, svc := newContentService()
	// p2 发布时间更晚，但 p1 置顶 → 置顶优先
	f.seedPost(1, 1, 1, withPinned(true), withCreatedAt(base))
	f.seedPost(2, 1, 1, withCreatedAt(base.Add(10*time.Hour)))
	f.seedPost(3, 2, 2, withTopic(2), withCreatedAt(base.Add(20*time.Hour)))

	t.Run("置顶优先 + 时间倒序", func(t *testing.T) {
		views, err := svc.ListFeed(ctx(), ListFeedQuery{Tab: "all", PageSize: 20})
		if err != nil {
			t.Fatalf("ListFeed() error = %v", err)
		}
		if len(views) != 3 {
			t.Fatalf("len = %d, want 3", len(views))
		}
		if views[0].ID != 1 || views[1].ID != 3 || views[2].ID != 2 {
			t.Errorf("排序 = [%d %d %d], want [1 3 2]", views[0].ID, views[1].ID, views[2].ID)
		}
	})

	t.Run("板块过滤", func(t *testing.T) {
		boardID := int64(2)
		views, err := svc.ListFeed(ctx(), ListFeedQuery{Tab: "all", BoardID: &boardID, PageSize: 20})
		if err != nil {
			t.Fatal(err)
		}
		if len(views) != 1 || views[0].ID != 3 {
			t.Errorf("板块2帖子 = %d 条, want [3]", len(views))
		}
	})

	t.Run("分页", func(t *testing.T) {
		views, err := svc.ListFeed(ctx(), ListFeedQuery{Tab: "all", Page: 2, PageSize: 2})
		if err != nil {
			t.Fatal(err)
		}
		if len(views) != 1 {
			t.Errorf("第2页 = %d 条, want 1", len(views))
		}
	})
}

func TestListFeed_Follow(t *testing.T) {
	base := time.Now().Add(-48 * time.Hour)
	f, _ := newContentService()
	// 关注来源：用户1（alice）、板块2、话题1(RAG)
	f.seedPost(1, 1, 1, withCreatedAt(base.Add(1*time.Hour)))          // alice 的帖（关注用户命中）
	f.seedPost(2, 2, 2, withTopic(2), withCreatedAt(base.Add(2*time.Hour)))  // bob 项目实战（关注板块命中）
	f.seedPost(3, 3, 1, withTopic(1), withCreatedAt(base.Add(3*time.Hour)))  // mod 学习讨论 RAG（关注话题命中）
	f.seedPost(4, 3, 1, withCreatedAt(base))                            // 无关帖（作者/板块/话题均未关注）

	fu := fakeUsers{
		followed: []int64{1}, // 关注用户 alice
		names:    map[int64]string{1: "alice", 2: "bob", 3: "mod"},
	}
	svc2 := newServiceWith(f, fu)
	_ = f.CreateFollowBoard(ctx(), &FollowBoard{FollowerID: 5, BoardID: 2})
	_ = f.CreateFollowTopic(ctx(), &FollowTopic{FollowerID: 5, TopicID: 1})

	views, err := svc2.ListFeed(ctx(), ListFeedQuery{Tab: "follow", ViewerID: 5, PageSize: 20})
	if err != nil {
		t.Fatalf("ListFeed(follow) error = %v", err)
	}
	if len(views) != 3 {
		t.Errorf("follow 流 = %d 条, want 3（1,2,3 命中）", len(views))
	}
	got := map[int64]bool{}
	for _, v := range views {
		got[v.ID] = true
	}
	if !got[1] || !got[2] || !got[3] {
		t.Errorf("follow 流命中 = %v, want 含 1,2,3", got)
	}
}

func TestListFeed_FollowGuestAndEmpty(t *testing.T) {
	f, svc := newContentService()

	t.Run("游客请求关注流 → ErrAuthRequired", func(t *testing.T) {
		_, err := svc.ListFeed(ctx(), ListFeedQuery{Tab: "follow", ViewerID: 0})
		if !errors.Is(err, ErrAuthRequired) {
			t.Errorf("error = %v, want ErrAuthRequired", err)
		}
	})

	t.Run("无关注 → 空列表", func(t *testing.T) {
		f.seedPost(1, 1, 1)
		views, err := svc.ListFeed(ctx(), ListFeedQuery{Tab: "follow", ViewerID: 5, PageSize: 20})
		if err != nil {
			t.Fatal(err)
		}
		if len(views) != 0 {
			t.Errorf("无关注 feed = %d 条, want 0", len(views))
		}
	})
}

func TestGetCommentTree(t *testing.T) {
	f, svc := newContentService()
	f.seedPost(1, 1, 1)
	// 顶层 c1(floor1)、c2(floor2)；c1 有回复 r1、r2；r2 被软删（占位保留）
	c1, _ := svc.CreateComment(ctx(), CreateCommentCmd{PostID: 1, AuthorID: 2, Content: "顶层1"})
	_, _ = svc.CreateComment(ctx(), CreateCommentCmd{PostID: 1, AuthorID: 3, Content: "顶层2"})
	r1, _ := svc.CreateComment(ctx(), CreateCommentCmd{PostID: 1, AuthorID: 1, ParentID: &c1.ID, Content: "回复1"})
	r2, _ := svc.CreateComment(ctx(), CreateCommentCmd{PostID: 1, AuthorID: 2, ParentID: &c1.ID, Content: "回复2"})
	_ = f.DeleteComment(ctx(), r2.ID)

	tree, err := svc.GetCommentTree(ctx(), 1)
	if err != nil {
		t.Fatalf("GetCommentTree() error = %v", err)
	}
	if len(tree.Comments) != 2 {
		t.Fatalf("顶层评论 = %d, want 2", len(tree.Comments))
	}
	if *tree.Comments[0].Floor != 1 || *tree.Comments[1].Floor != 2 {
		t.Errorf("顶层楼层顺序错误")
	}
	// c1 的回复
	replies := tree.Comments[0].Replies
	if len(replies) != 2 {
		t.Fatalf("c1 回复 = %d, want 2（含软删占位）", len(replies))
	}
	if !replies[1].Deleted {
		t.Error("r2 应标记 Deleted（软删占位）")
	}
	if replies[0].ID != r1.ID || replies[0].AuthorName != "alice" {
		t.Errorf("r1 = %+v", replies[0])
	}
	if tree.Comments[1].AuthorName != "mod" {
		t.Errorf("c2 author = %s, want mod", tree.Comments[1].AuthorName)
	}
}

// TestGetCommentTreeEmpty：空评论树契约回归（fix: 空树返回 [] 而非 nil → JSON "comments":[] 非 null）。
func TestGetCommentTreeEmpty(t *testing.T) {
	f, svc := newContentService()
	f.seedPost(1, 1, 1) // 无任何评论的帖子

	tree, err := svc.GetCommentTree(ctx(), 1)
	if err != nil {
		t.Fatalf("GetCommentTree() error = %v", err)
	}
	if tree.Comments == nil {
		t.Fatal("空评论树 Comments 应为非 nil 空切片（契约：JSON 序列化恒为 [] 而非 null）")
	}
	if len(tree.Comments) != 0 {
		t.Fatalf("空评论树 Comments = %d 条, want 0", len(tree.Comments))
	}
	raw, err := json.Marshal(tree)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if !bytes.Contains(raw, []byte(`"comments":[]`)) {
		t.Fatalf("空树 JSON = %s, 期望含 \"comments\":[]", raw)
	}
	if bytes.Contains(raw, []byte(`"comments":null`)) {
		t.Fatalf("空树 JSON = %s, 不得出现 \"comments\":null", raw)
	}
}

func TestDeletePermissions(t *testing.T) {
	f, svc := newContentService()
	f.seedPost(1, 1, 1) // alice 的帖
	c, _ := svc.CreateComment(ctx(), CreateCommentCmd{PostID: 1, AuthorID: 1, Content: "alice 评论"})

	t.Run("作者删除成功", func(t *testing.T) {
		if err := svc.DeletePost(ctx(), DeletePostCmd{OperatorID: 1, OperatorRole: "user", PostID: 1}); err != nil {
			t.Fatalf("作者删帖 error = %v", err)
		}
		// 已删除 → 再查 NotFound
		if err := svc.DeletePost(ctx(), DeletePostCmd{OperatorID: 1, OperatorRole: "user", PostID: 1}); !errors.Is(err, ErrPostNotFound) {
			t.Errorf("删后 error = %v, want ErrPostNotFound", err)
		}
	})

	t.Run("非作者 user 删除 → ErrForbidden", func(t *testing.T) {
		f.seedPost(2, 1, 1)
		err := svc.DeletePost(ctx(), DeletePostCmd{OperatorID: 2, OperatorRole: "user", PostID: 2})
		if !errors.Is(err, ErrForbidden) {
			t.Errorf("error = %v, want ErrForbidden", err)
		}
	})

	t.Run("moderator 可删他人帖", func(t *testing.T) {
		f.seedPost(3, 2, 1)
		if err := svc.DeletePost(ctx(), DeletePostCmd{OperatorID: 3, OperatorRole: "moderator", PostID: 3}); err != nil {
			t.Errorf("moderator 删帖 error = %v", err)
		}
	})

	t.Run("评论：作者删 / 非作者拦截", func(t *testing.T) {
		if err := svc.DeleteComment(ctx(), DeleteCommentCmd{OperatorID: 1, OperatorRole: "user", CommentID: c.ID}); err != nil {
			t.Errorf("作者删评论 error = %v", err)
		}
		// 再删 → NotFound
		if err := svc.DeleteComment(ctx(), DeleteCommentCmd{OperatorID: 1, OperatorRole: "user", CommentID: c.ID}); !errors.Is(err, ErrCommentNotFound) {
			t.Errorf("删后 error = %v, want ErrCommentNotFound", err)
		}
	})
}

func TestPinFeaturePermissions(t *testing.T) {
	f, svc := newContentService()
	f.seedPost(1, 1, 1)

	t.Run("user 置顶/精华 → ErrForbidden", func(t *testing.T) {
		err := svc.PinPost(ctx(), ToggleCmd{OperatorID: 2, OperatorRole: "user", PostID: 1})
		if !errors.Is(err, ErrForbidden) {
			t.Errorf("error = %v, want ErrForbidden", err)
		}
		err = svc.FeaturePost(ctx(), ToggleCmd{OperatorID: 2, OperatorRole: "user", PostID: 1})
		if !errors.Is(err, ErrForbidden) {
			t.Errorf("FeaturePost error = %v, want ErrForbidden", err)
		}
	})

	t.Run("moderator 置顶/精华成功并切换", func(t *testing.T) {
		if err := svc.PinPost(ctx(), ToggleCmd{OperatorID: 3, OperatorRole: "moderator", PostID: 1}); err != nil {
			t.Fatalf("PinPost() error = %v", err)
		}
		p, _ := f.GetPostByID(ctx(), 1)
		if !p.IsPinned {
			t.Error("置顶后 IsPinned 应为 true")
		}
		// 再次 → 取消置顶
		if err := svc.PinPost(ctx(), ToggleCmd{OperatorID: 3, OperatorRole: "moderator", PostID: 1}); err != nil {
			t.Fatal(err)
		}
		p, _ = f.GetPostByID(ctx(), 1)
		if p.IsPinned {
			t.Error("二次置顶应取消")
		}
		if err := svc.FeaturePost(ctx(), ToggleCmd{OperatorID: 4, OperatorRole: "admin", PostID: 1}); err != nil {
			t.Errorf("FeaturePost() error = %v", err)
		}
		p, _ = f.GetPostByID(ctx(), 1)
		if !p.IsFeatured {
			t.Error("精华后 IsFeatured 应为 true")
		}
		// 二次 → 取消精华（覆盖关闭分支）
		if err := svc.FeaturePost(ctx(), ToggleCmd{OperatorID: 4, OperatorRole: "admin", PostID: 1}); err != nil {
			t.Errorf("FeaturePost() 二次 error = %v", err)
		}
		p, _ = f.GetPostByID(ctx(), 1)
		if p.IsFeatured {
			t.Error("二次精华应取消")
		}
	})
}

func TestFollowBoardTopic(t *testing.T) {
	t.Run("关注板块：成功/重复/不存在", func(t *testing.T) {
		_, svc := newContentService()
		if err := svc.FollowBoard(ctx(), FollowBoardCmd{FollowerID: 2, BoardID: 1}); err != nil {
			t.Fatalf("FollowBoard() error = %v", err)
		}
		if err := svc.FollowBoard(ctx(), FollowBoardCmd{FollowerID: 2, BoardID: 1}); !errors.Is(err, ErrAlreadyFollowed) {
			t.Errorf("重复关注 error = %v, want ErrAlreadyFollowed", err)
		}
		if err := svc.FollowBoard(ctx(), FollowBoardCmd{FollowerID: 2, BoardID: 99}); !errors.Is(err, ErrBoardNotFound) {
			t.Errorf("关注不存在板块 error = %v, want ErrBoardNotFound", err)
		}
		// 取关幂等
		if err := svc.UnfollowBoard(ctx(), FollowBoardCmd{FollowerID: 2, BoardID: 1}); err != nil {
			t.Errorf("UnfollowBoard() error = %v", err)
		}
		if err := svc.UnfollowBoard(ctx(), FollowBoardCmd{FollowerID: 2, BoardID: 1}); err != nil {
			t.Errorf("UnfollowBoard() 幂等 error = %v", err)
		}
	})

	t.Run("关注话题", func(t *testing.T) {
		_, svc := newContentService()
		if err := svc.FollowTopic(ctx(), FollowTopicCmd{FollowerID: 2, TopicID: 1}); err != nil {
			t.Fatalf("FollowTopic() error = %v", err)
		}
		if err := svc.FollowTopic(ctx(), FollowTopicCmd{FollowerID: 2, TopicID: 99}); !errors.Is(err, ErrTopicNotFound) {
			t.Errorf("error = %v, want ErrTopicNotFound", err)
		}
		// 取关幂等
		if err := svc.UnfollowTopic(ctx(), FollowTopicCmd{FollowerID: 2, TopicID: 1}); err != nil {
			t.Errorf("UnfollowTopic() error = %v", err)
		}
		if err := svc.UnfollowTopic(ctx(), FollowTopicCmd{FollowerID: 2, TopicID: 1}); err != nil {
			t.Errorf("UnfollowTopic() 幂等 error = %v", err)
		}
	})
}

func TestListBoardsTopics(t *testing.T) {
	_, svc := newContentService()

	boards, err := svc.ListBoards(ctx(), 0)
	if err != nil {
		t.Fatalf("ListBoards() error = %v", err)
	}
	if len(boards) != 2 || boards[0].Name != "学习讨论" {
		t.Errorf("boards = %+v, want 2 个按 sort_order 排列", boards)
	}

	all, err := svc.ListTopics(ctx(), 0, nil)
	if err != nil || len(all) != 2 {
		t.Fatalf("ListTopics() = %d, err=%v", len(all), err)
	}
	boardID := int64(1)
	filtered, err := svc.ListTopics(ctx(), 0, &boardID)
	if err != nil || len(filtered) != 1 || filtered[0].Name != "RAG" {
		t.Errorf("按板块过滤 = %+v, err=%v", filtered, err)
	}
}

func TestCountPostsByAuthor(t *testing.T) {
	f, svc := newContentService()
	f.seedPost(1, 1, 1)
	f.seedPost(2, 1, 1)
	f.seedPost(3, 2, 1)

	n, err := svc.CountPostsByAuthor(ctx(), 1)
	if err != nil || n != 2 {
		t.Errorf("author 1 帖子数 = %d, err=%v, want 2", n, err)
	}
	n2, err := svc.CountPostsByAuthor(ctx(), 99)
	if err != nil || n2 != 0 {
		t.Errorf("不存在的作者帖子数 = %d, err=%v, want 0", n2, err)
	}
}

// ---- 收藏/关注列表（#23） ----

func TestListFavorites(t *testing.T) {
	base := time.Now().Add(-48 * time.Hour)

	t.Run("按收藏时间倒序 + viewer.favorited=true", func(t *testing.T) {
		f, svc := newContentService()
		f.seedPost(1, 1, 1)
		f.seedPost(2, 2, 1)
		f.seedPost(3, 3, 1)
		f.seedFavorite(5, 1, base)
		f.seedFavorite(5, 2, base.Add(1*time.Hour))
		f.seedFavorite(5, 3, base.Add(2*time.Hour))

		views, err := svc.ListFavorites(ctx(), ListFavoritesQuery{ViewerID: 5, PageSize: 20})
		if err != nil {
			t.Fatalf("ListFavorites() error = %v", err)
		}
		if len(views) != 3 || views[0].ID != 3 || views[1].ID != 2 || views[2].ID != 1 {
			t.Errorf("排序 = %d 条, want [3 2 1]（收藏时间倒序）", len(views))
		}
		for _, v := range views {
			if v.Viewer == nil || !v.Viewer.Favorited {
				t.Errorf("viewer.favorited 应为 true, got %+v", v.Viewer)
			}
		}
	})

	t.Run("游客 → ErrAuthRequired", func(t *testing.T) {
		_, svc := newContentService()
		if _, err := svc.ListFavorites(ctx(), ListFavoritesQuery{ViewerID: 0}); !errors.Is(err, ErrAuthRequired) {
			t.Errorf("error = %v, want ErrAuthRequired", err)
		}
	})

	t.Run("分页", func(t *testing.T) {
		f, svc := newContentService()
		f.seedPost(1, 1, 1)
		f.seedPost(2, 2, 1)
		f.seedPost(3, 3, 1)
		f.seedFavorite(5, 1, base)
		f.seedFavorite(5, 2, base.Add(1*time.Hour))
		f.seedFavorite(5, 3, base.Add(2*time.Hour))

		views, err := svc.ListFavorites(ctx(), ListFavoritesQuery{ViewerID: 5, Page: 3, PageSize: 1})
		if err != nil {
			t.Fatal(err)
		}
		if len(views) != 1 || views[0].ID != 1 {
			t.Errorf("第3页 = %d 条, want [1]（最旧收藏）", len(views))
		}
	})

	t.Run("空 → 非 nil 空切片", func(t *testing.T) {
		f, svc := newContentService()
		f.seedPost(1, 1, 1)
		views, err := svc.ListFavorites(ctx(), ListFavoritesQuery{ViewerID: 5, PageSize: 20})
		if err != nil {
			t.Fatal(err)
		}
		if views == nil || len(views) != 0 {
			t.Errorf("空收藏 views = %#v, want 非 nil 空切片（信封契约 items:[]）", views)
		}
	})

	t.Run("软删帖子不出现在收藏列表", func(t *testing.T) {
		f, svc := newContentService()
		f.seedPost(1, 1, 1)
		f.seedPost(2, 2, 1)
		f.seedFavorite(5, 1, base)
		f.seedFavorite(5, 2, base.Add(1*time.Hour))
		_ = f.DeletePost(ctx(), 1)

		views, err := svc.ListFavorites(ctx(), ListFavoritesQuery{ViewerID: 5, PageSize: 20})
		if err != nil {
			t.Fatal(err)
		}
		if len(views) != 1 || views[0].ID != 2 {
			t.Errorf("软删后收藏列表 = %d 条, want 仅 [2]", len(views))
		}
	})
}

func TestListFollowedBoards(t *testing.T) {
	t.Run("按关注时间倒序 + viewer.following=true", func(t *testing.T) {
		_, svc := newContentService()
		if err := svc.FollowBoard(ctx(), FollowBoardCmd{FollowerID: 5, BoardID: 1}); err != nil {
			t.Fatal(err)
		}
		if err := svc.FollowBoard(ctx(), FollowBoardCmd{FollowerID: 5, BoardID: 2}); err != nil {
			t.Fatal(err)
		}

		views, err := svc.ListFollowedBoards(ctx(), ListFollowedBoardsQuery{ViewerID: 5, PageSize: 20})
		if err != nil {
			t.Fatalf("ListFollowedBoards() error = %v", err)
		}
		if len(views) != 2 || views[0].ID != 2 || views[1].ID != 1 {
			t.Errorf("排序 = %d 条, want [2 1]（关注时间倒序）", len(views))
		}
		for _, v := range views {
			if v.Viewer == nil || !v.Viewer.Following {
				t.Errorf("viewer.following 应为 true, got %+v", v.Viewer)
			}
		}
	})

	t.Run("游客 → ErrAuthRequired", func(t *testing.T) {
		_, svc := newContentService()
		if _, err := svc.ListFollowedBoards(ctx(), ListFollowedBoardsQuery{ViewerID: 0}); !errors.Is(err, ErrAuthRequired) {
			t.Errorf("error = %v, want ErrAuthRequired", err)
		}
	})

	t.Run("分页", func(t *testing.T) {
		f, svc := newContentService()
		_ = f.CreateFollowBoard(ctx(), &FollowBoard{FollowerID: 5, BoardID: 1})
		_ = f.CreateFollowBoard(ctx(), &FollowBoard{FollowerID: 5, BoardID: 2})
		views, err := svc.ListFollowedBoards(ctx(), ListFollowedBoardsQuery{ViewerID: 5, Page: 2, PageSize: 1})
		if err != nil {
			t.Fatal(err)
		}
		if len(views) != 1 || views[0].ID != 1 {
			t.Errorf("第2页 = %d 条, want [1]", len(views))
		}
	})

	t.Run("空 → 非 nil 空切片", func(t *testing.T) {
		_, svc := newContentService()
		views, err := svc.ListFollowedBoards(ctx(), ListFollowedBoardsQuery{ViewerID: 5, PageSize: 20})
		if err != nil {
			t.Fatal(err)
		}
		if views == nil || len(views) != 0 {
			t.Errorf("空关注 views = %#v, want 非 nil 空切片", views)
		}
	})

	t.Run("软删板块不出现在关注列表", func(t *testing.T) {
		f, svc := newContentService()
		_ = f.CreateFollowBoard(ctx(), &FollowBoard{FollowerID: 5, BoardID: 1})
		_ = f.CreateFollowBoard(ctx(), &FollowBoard{FollowerID: 5, BoardID: 2})
		f.boards[1].DeletedAt = gorm.DeletedAt{Valid: true}

		views, err := svc.ListFollowedBoards(ctx(), ListFollowedBoardsQuery{ViewerID: 5, PageSize: 20})
		if err != nil {
			t.Fatal(err)
		}
		if len(views) != 1 || views[0].ID != 2 {
			t.Errorf("软删后关注板块 = %d 条, want 仅 [2]", len(views))
		}
	})
}

func TestListFollowedTopics(t *testing.T) {
	t.Run("按关注时间倒序 + viewer.following=true", func(t *testing.T) {
		_, svc := newContentService()
		if err := svc.FollowTopic(ctx(), FollowTopicCmd{FollowerID: 5, TopicID: 1}); err != nil {
			t.Fatal(err)
		}
		if err := svc.FollowTopic(ctx(), FollowTopicCmd{FollowerID: 5, TopicID: 2}); err != nil {
			t.Fatal(err)
		}

		views, err := svc.ListFollowedTopics(ctx(), ListFollowedTopicsQuery{ViewerID: 5, PageSize: 20})
		if err != nil {
			t.Fatalf("ListFollowedTopics() error = %v", err)
		}
		if len(views) != 2 || views[0].ID != 2 || views[1].ID != 1 {
			t.Errorf("排序 = %d 条, want [2 1]（关注时间倒序）", len(views))
		}
		for _, v := range views {
			if v.Viewer == nil || !v.Viewer.Following {
				t.Errorf("viewer.following 应为 true, got %+v", v.Viewer)
			}
			if v.BoardID == 0 {
				t.Errorf("topic 应带 board_id, got %+v", v)
			}
		}
	})

	t.Run("游客 → ErrAuthRequired", func(t *testing.T) {
		_, svc := newContentService()
		if _, err := svc.ListFollowedTopics(ctx(), ListFollowedTopicsQuery{ViewerID: 0}); !errors.Is(err, ErrAuthRequired) {
			t.Errorf("error = %v, want ErrAuthRequired", err)
		}
	})

	t.Run("空 → 非 nil 空切片", func(t *testing.T) {
		_, svc := newContentService()
		views, err := svc.ListFollowedTopics(ctx(), ListFollowedTopicsQuery{ViewerID: 5, PageSize: 20})
		if err != nil {
			t.Fatal(err)
		}
		if views == nil || len(views) != 0 {
			t.Errorf("空关注 views = %#v, want 非 nil 空切片", views)
		}
	})

	t.Run("软删话题不出现在关注列表", func(t *testing.T) {
		f, svc := newContentService()
		_ = f.CreateFollowTopic(ctx(), &FollowTopic{FollowerID: 5, TopicID: 1})
		_ = f.CreateFollowTopic(ctx(), &FollowTopic{FollowerID: 5, TopicID: 2})
		f.topics[1].DeletedAt = gorm.DeletedAt{Valid: true}

		views, err := svc.ListFollowedTopics(ctx(), ListFollowedTopicsQuery{ViewerID: 5, PageSize: 20})
		if err != nil {
			t.Fatal(err)
		}
		if len(views) != 1 || views[0].ID != 2 {
			t.Errorf("软删后关注话题 = %d 条, want 仅 [2]", len(views))
		}
	})
}
