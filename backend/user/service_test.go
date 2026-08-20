package user

import (
	"context"
	"errors"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func hashPassword(pw string) string {
	h, _ := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.MinCost)
	return string(h)
}

func newService(f *fakeRepo) Service {
	return NewService(f, fakeIssuer{token: "test-token"})
}

func TestRegister_Success(t *testing.T) {
	f := newFakeRepo()
	f.seedCode("ABCDEF", 1)
	svc := newService(f)

	res, err := svc.Register(context.Background(), RegisterCmd{
		Username: "alice", Email: "alice@x.edu", Password: "secret123", InviteCode: "ABCDEF",
	})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if res.Token != "test-token" {
		t.Errorf("Token = %q, want %q", res.Token, "test-token")
	}
	// 边界映射：DB 存 member，对外为 user（#5/#9 一致性修正）
	if res.User.Role != "user" {
		t.Errorf("Role = %q, want %q", res.User.Role, "user")
	}
	// 邀请码已标记使用（一次性）
	ic, err := f.GetInvitationCode(context.Background(), "ABCDEF")
	if err != nil || ic.UsedBy == nil {
		t.Errorf("邀请码应已标记使用，err=%v", err)
	}
	if *ic.UsedBy != res.User.ID {
		t.Errorf("UsedBy = %d, want %d", *ic.UsedBy, res.User.ID)
	}
}

func TestRegister_WeakPassword(t *testing.T) {
	f := newFakeRepo()
	f.seedCode("ABCDEF", 1)
	svc := newService(f)

	_, err := svc.Register(context.Background(), RegisterCmd{
		Username: "alice", Email: "alice@x.edu", Password: "short", InviteCode: "ABCDEF",
	})
	if !errors.Is(err, ErrWeakPassword) {
		t.Errorf("error = %v, want ErrWeakPassword", err)
	}
}

func TestRegister_InvalidInvite(t *testing.T) {
	t.Run("不存在的邀请码", func(t *testing.T) {
		f := newFakeRepo()
		svc := newService(f)
		_, err := svc.Register(context.Background(), RegisterCmd{
			Username: "alice", Email: "alice@x.edu", Password: "secret123", InviteCode: "NOPE",
		})
		if !errors.Is(err, ErrInvalidInvite) {
			t.Errorf("error = %v, want ErrInvalidInvite", err)
		}
	})

	t.Run("已使用的邀请码", func(t *testing.T) {
		f := newFakeRepo()
		ic := f.seedCode("ABCDEF", 1)
		usedBy := int64(99)
		ic.UsedBy = &usedBy
		svc := newService(f)
		_, err := svc.Register(context.Background(), RegisterCmd{
			Username: "alice", Email: "alice@x.edu", Password: "secret123", InviteCode: "ABCDEF",
		})
		if !errors.Is(err, ErrInvalidInvite) {
			t.Errorf("error = %v, want ErrInvalidInvite", err)
		}
	})
}

func TestRegister_EmptyFields(t *testing.T) {
	f := newFakeRepo()
	f.seedCode("ABCDEF", 1)
	svc := newService(f)

	_, err := svc.Register(context.Background(), RegisterCmd{
		Username: "", Email: "alice@x.edu", Password: "secret123", InviteCode: "ABCDEF",
	})
	if err == nil {
		t.Error("空用户名应报错")
	}
}

func TestRegister_Duplicate(t *testing.T) {
	t.Run("用户名重复", func(t *testing.T) {
		f := newFakeRepo()
		f.seedCode("ABCDEF", 1)
		f.seedUser("alice", "alice@x.edu", "secret123")
		svc := newService(f)
		_, err := svc.Register(context.Background(), RegisterCmd{
			Username: "alice", Email: "other@x.edu", Password: "secret123", InviteCode: "ABCDEF",
		})
		if !errors.Is(err, ErrUsernameTaken) {
			t.Errorf("error = %v, want ErrUsernameTaken", err)
		}
	})

	t.Run("邮箱重复", func(t *testing.T) {
		f := newFakeRepo()
		f.seedCode("ABCDEF", 1)
		f.seedUser("alice", "alice@x.edu", "secret123")
		svc := newService(f)
		_, err := svc.Register(context.Background(), RegisterCmd{
			Username: "bob", Email: "alice@x.edu", Password: "secret123", InviteCode: "ABCDEF",
		})
		if !errors.Is(err, ErrEmailTaken) {
			t.Errorf("error = %v, want ErrEmailTaken", err)
		}
	})
}

func TestLogin(t *testing.T) {
	t.Run("成功", func(t *testing.T) {
		f := newFakeRepo()
		f.seedUser("alice", "alice@x.edu", "secret123")
		svc := newService(f)
		res, err := svc.Login(context.Background(), LoginCmd{Username: "alice", Password: "secret123"})
		if err != nil {
			t.Fatalf("Login() error = %v", err)
		}
		if res.User.Username != "alice" || res.User.Role != "user" {
			t.Errorf("res.User = %+v", res.User)
		}
	})

	t.Run("密码错误统一 ErrBadCredential", func(t *testing.T) {
		f := newFakeRepo()
		f.seedUser("alice", "alice@x.edu", "secret123")
		svc := newService(f)
		_, err := svc.Login(context.Background(), LoginCmd{Username: "alice", Password: "wrongpass"})
		if !errors.Is(err, ErrBadCredential) {
			t.Errorf("error = %v, want ErrBadCredential", err)
		}
	})

	t.Run("用户不存在同样 ErrBadCredential（防枚举）", func(t *testing.T) {
		f := newFakeRepo()
		svc := newService(f)
		_, err := svc.Login(context.Background(), LoginCmd{Username: "ghost", Password: "whatever"})
		if !errors.Is(err, ErrBadCredential) {
			t.Errorf("error = %v, want ErrBadCredential", err)
		}
	})

	t.Run("封禁账号拒绝登录", func(t *testing.T) {
		f := newFakeRepo()
		u := f.seedUser("alice", "alice@x.edu", "secret123")
		u.Status = "banned"
		svc := newService(f)
		_, err := svc.Login(context.Background(), LoginCmd{Username: "alice", Password: "secret123"})
		if !errors.Is(err, ErrBanned) {
			t.Errorf("error = %v, want ErrBanned", err)
		}
	})

	t.Run("moderator/admin 角色透传（mapRole 非 member 直通）", func(t *testing.T) {
		f := newFakeRepo()
		u := f.seedUser("mod1", "mod1@x.edu", "secret123")
		u.Role = "moderator"
		svc := newService(f)
		res, err := svc.Login(context.Background(), LoginCmd{Username: "mod1", Password: "secret123"})
		if err != nil {
			t.Fatalf("Login() error = %v", err)
		}
		if res.User.Role != "moderator" {
			t.Errorf("Role = %q, want moderator", res.User.Role)
		}
	})
}

func TestFollow(t *testing.T) {
	t.Run("成功", func(t *testing.T) {
		f := newFakeRepo()
		follower := f.seedUser("alice", "a@x.edu", "secret123")
		target := f.seedUser("bob", "b@x.edu", "secret123")
		svc := newService(f)
		if err := svc.Follow(context.Background(), FollowCmd{FollowerID: follower.ID, TargetID: target.ID}); err != nil {
			t.Fatalf("Follow() error = %v", err)
		}
	})

	t.Run("不能关注自己", func(t *testing.T) {
		f := newFakeRepo()
		u := f.seedUser("alice", "a@x.edu", "secret123")
		svc := newService(f)
		if err := svc.Follow(context.Background(), FollowCmd{FollowerID: u.ID, TargetID: u.ID}); !errors.Is(err, ErrSelfFollow) {
			t.Errorf("error = %v, want ErrSelfFollow", err)
		}
	})

	t.Run("目标不存在", func(t *testing.T) {
		f := newFakeRepo()
		u := f.seedUser("alice", "a@x.edu", "secret123")
		svc := newService(f)
		if err := svc.Follow(context.Background(), FollowCmd{FollowerID: u.ID, TargetID: 999}); !errors.Is(err, ErrNotFound) {
			t.Errorf("error = %v, want ErrNotFound", err)
		}
	})

	t.Run("重复关注", func(t *testing.T) {
		f := newFakeRepo()
		follower := f.seedUser("alice", "a@x.edu", "secret123")
		target := f.seedUser("bob", "b@x.edu", "secret123")
		f.follows[[2]int64{follower.ID, target.ID}] = time.Now()
		svc := newService(f)
		if err := svc.Follow(context.Background(), FollowCmd{FollowerID: follower.ID, TargetID: target.ID}); !errors.Is(err, ErrAlreadyFollow) {
			t.Errorf("error = %v, want ErrAlreadyFollow", err)
		}
	})
}

func TestUnfollow(t *testing.T) {
	t.Run("取关成功", func(t *testing.T) {
		f := newFakeRepo()
		follower := f.seedUser("alice", "a@x.edu", "secret123")
		target := f.seedUser("bob", "b@x.edu", "secret123")
		f.follows[[2]int64{follower.ID, target.ID}] = time.Now()
		svc := newService(f)
		if err := svc.Unfollow(context.Background(), FollowCmd{FollowerID: follower.ID, TargetID: target.ID}); err != nil {
			t.Fatalf("Unfollow() error = %v", err)
		}
		ids, _ := svc.FollowedUserIDs(context.Background(), follower.ID)
		if len(ids) != 0 {
			t.Errorf("取关后关注列表 = %v, want 空", ids)
		}
	})

	t.Run("未关注也幂等成功", func(t *testing.T) {
		f := newFakeRepo()
		follower := f.seedUser("alice", "a@x.edu", "secret123")
		target := f.seedUser("bob", "b@x.edu", "secret123")
		svc := newService(f)
		if err := svc.Unfollow(context.Background(), FollowCmd{FollowerID: follower.ID, TargetID: target.ID}); err != nil {
			t.Errorf("Unfollow() 应幂等成功, error = %v", err)
		}
	})
}

func TestFollowedUserIDs(t *testing.T) {
	f := newFakeRepo()
	follower := f.seedUser("alice", "a@x.edu", "secret123")
	bob := f.seedUser("bob", "b@x.edu", "secret123")
	carol := f.seedUser("carol", "c@x.edu", "secret123")
	f.follows[[2]int64{follower.ID, bob.ID}] = time.Now()
	f.follows[[2]int64{follower.ID, carol.ID}] = time.Now()
	svc := newService(f)

	ids, err := svc.FollowedUserIDs(context.Background(), follower.ID)
	if err != nil {
		t.Fatalf("FollowedUserIDs() error = %v", err)
	}
	if len(ids) != 2 {
		t.Errorf("FollowedUserIDs() = %v, want 2 个", ids)
	}
}

func TestFollowsUser(t *testing.T) {
	f := newFakeRepo()
	alice := f.seedUser("alice", "a@x.edu", "secret123")
	bob := f.seedUser("bob", "b@x.edu", "secret123")
	f.follows[[2]int64{alice.ID, bob.ID}] = time.Now()
	svc := newService(f)

	if ok, err := svc.FollowsUser(context.Background(), alice.ID, bob.ID); err != nil || !ok {
		t.Errorf("已关注应 true, ok=%v err=%v", ok, err)
	}
	if ok, _ := svc.FollowsUser(context.Background(), bob.ID, alice.ID); ok {
		t.Error("未关注应 false")
	}
}

func TestPublicProfile(t *testing.T) {
	t.Run("资料与计数", func(t *testing.T) {
		f := newFakeRepo()
		alice := f.seedUser("alice", "a@x.edu", "secret123")
		bob := f.seedUser("bob", "b@x.edu", "secret123")
		carol := f.seedUser("carol", "c@x.edu", "secret123")
		f.follows[[2]int64{bob.ID, alice.ID}] = time.Now() // bob 关注 alice
		f.follows[[2]int64{carol.ID, alice.ID}] = time.Now()
		f.follows[[2]int64{alice.ID, bob.ID}] = time.Now() // alice 关注 bob
		svc := newService(f)

		p, err := svc.PublicProfile(context.Background(), PublicProfileCmd{TargetID: alice.ID, ViewerID: bob.ID})
		if err != nil {
			t.Fatalf("PublicProfile() error = %v", err)
		}
		if p.Username != "alice" || p.FollowerCount != 2 || p.FollowingCount != 1 {
			t.Errorf("profile = %+v, want followers=2 following=1", p)
		}
		if !p.Following {
			t.Error("bob 视角 Following 应为 true")
		}
	})

	t.Run("游客不附 Following", func(t *testing.T) {
		f := newFakeRepo()
		alice := f.seedUser("alice", "a@x.edu", "secret123")
		svc := newService(f)
		p, err := svc.PublicProfile(context.Background(), PublicProfileCmd{TargetID: alice.ID, ViewerID: 0})
		if err != nil {
			t.Fatal(err)
		}
		if p.Following {
			t.Error("游客 Following 应为 false")
		}
	})

	t.Run("不存在 → ErrNotFound", func(t *testing.T) {
		f := newFakeRepo()
		svc := newService(f)
		_, err := svc.PublicProfile(context.Background(), PublicProfileCmd{TargetID: 999})
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("error = %v, want ErrNotFound", err)
		}
	})
}

func TestBan(t *testing.T) {
	t.Run("成功", func(t *testing.T) {
		f := newFakeRepo()
		target := f.seedUser("bob", "b@x.edu", "secret123")
		svc := newService(f)
		if err := svc.Ban(context.Background(), BanCmd{OperatorID: 1, TargetID: target.ID}); err != nil {
			t.Fatalf("Ban() error = %v", err)
		}
		u, err := f.GetUserByID(context.Background(), target.ID)
		if err != nil || u.Status != "banned" {
			t.Errorf("封禁后 status = %q, err = %v", u.Status, err)
		}
	})

	t.Run("目标不存在", func(t *testing.T) {
		f := newFakeRepo()
		svc := newService(f)
		if err := svc.Ban(context.Background(), BanCmd{OperatorID: 1, TargetID: 999}); !errors.Is(err, ErrNotFound) {
			t.Errorf("error = %v, want ErrNotFound", err)
		}
	})
}

func TestGetUser(t *testing.T) {
	t.Run("命中", func(t *testing.T) {
		f := newFakeRepo()
		u := f.seedUser("alice", "a@x.edu", "secret123")
		svc := newService(f)
		view, err := svc.GetUser(context.Background(), u.ID)
		if err != nil {
			t.Fatalf("GetUser() error = %v", err)
		}
		if view.Username != "alice" {
			t.Errorf("Username = %q, want alice", view.Username)
		}
	})

	t.Run("不存在", func(t *testing.T) {
		f := newFakeRepo()
		svc := newService(f)
		_, err := svc.GetUser(context.Background(), 123)
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("error = %v, want ErrNotFound", err)
		}
	})
}

// ---- 我的关注用户列表（#23） ----

func TestListFollowedUsers(t *testing.T) {
	base := time.Now().Add(-48 * time.Hour)

	t.Run("按关注时间倒序 + viewer.following=true + 画像字段", func(t *testing.T) {
		f := newFakeRepo()
		alice := f.seedUser("alice", "a@x.edu", "secret123")
		bob := f.seedUser("bob", "b@x.edu", "secret123")
		carol := f.seedUser("carol", "c@x.edu", "secret123")
		avatar := "https://img/avatar.png"
		bio := "爱写代码"
		bob.AvatarURL = &avatar
		bob.Bio = &bio
		f.seedFollow(alice.ID, bob.ID, base)
		f.seedFollow(alice.ID, carol.ID, base.Add(1*time.Hour))
		svc := newService(f)

		views, err := svc.ListFollowedUsers(context.Background(), ListFollowedUsersQuery{ViewerID: alice.ID, PageSize: 20})
		if err != nil {
			t.Fatalf("ListFollowedUsers() error = %v", err)
		}
		if len(views) != 2 || views[0].Username != "carol" || views[1].Username != "bob" {
			t.Errorf("排序 = %d 条, want [carol bob]（关注时间倒序）", len(views))
		}
		if views[1].AvatarURL == nil || *views[1].AvatarURL != avatar || views[1].Bio == nil || *views[1].Bio != bio {
			t.Errorf("画像字段缺失: %+v", views[1])
		}
		for _, v := range views {
			if v.Viewer == nil || !v.Viewer.Following {
				t.Errorf("viewer.following 应为 true, got %+v", v.Viewer)
			}
		}
	})

	t.Run("游客 → ErrAuthRequired", func(t *testing.T) {
		f := newFakeRepo()
		svc := newService(f)
		if _, err := svc.ListFollowedUsers(context.Background(), ListFollowedUsersQuery{ViewerID: 0}); !errors.Is(err, ErrAuthRequired) {
			t.Errorf("error = %v, want ErrAuthRequired", err)
		}
	})

	t.Run("分页", func(t *testing.T) {
		f := newFakeRepo()
		alice := f.seedUser("alice", "a@x.edu", "secret123")
		bob := f.seedUser("bob", "b@x.edu", "secret123")
		carol := f.seedUser("carol", "c@x.edu", "secret123")
		f.seedFollow(alice.ID, bob.ID, base)
		f.seedFollow(alice.ID, carol.ID, base.Add(1*time.Hour))
		svc := newService(f)

		views, err := svc.ListFollowedUsers(context.Background(), ListFollowedUsersQuery{ViewerID: alice.ID, Page: 2, PageSize: 1})
		if err != nil {
			t.Fatal(err)
		}
		if len(views) != 1 || views[0].Username != "bob" {
			t.Errorf("第2页 = %d 条, want [bob]", len(views))
		}
	})

	t.Run("空 → 非 nil 空切片", func(t *testing.T) {
		f := newFakeRepo()
		f.seedUser("alice", "a@x.edu", "secret123")
		svc := newService(f)
		views, err := svc.ListFollowedUsers(context.Background(), ListFollowedUsersQuery{ViewerID: 1, PageSize: 20})
		if err != nil {
			t.Fatal(err)
		}
		if views == nil || len(views) != 0 {
			t.Errorf("空关注 views = %#v, want 非 nil 空切片", views)
		}
	})

	t.Run("软删用户不出现在关注列表", func(t *testing.T) {
		f := newFakeRepo()
		alice := f.seedUser("alice", "a@x.edu", "secret123")
		bob := f.seedUser("bob", "b@x.edu", "secret123")
		carol := f.seedUser("carol", "c@x.edu", "secret123")
		f.seedFollow(alice.ID, bob.ID, base)
		f.seedFollow(alice.ID, carol.ID, base.Add(1*time.Hour))
		bob.DeletedAt = gorm.DeletedAt{Valid: true}
		svc := newService(f)

		views, err := svc.ListFollowedUsers(context.Background(), ListFollowedUsersQuery{ViewerID: alice.ID, PageSize: 20})
		if err != nil {
			t.Fatal(err)
		}
		if len(views) != 1 || views[0].Username != "carol" {
			t.Errorf("软删后关注用户 = %d 条, want 仅 [carol]", len(views))
		}
	})
}
