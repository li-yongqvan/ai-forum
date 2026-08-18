package user

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/crypto/bcrypt"
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
		f.follows[[2]int64{follower.ID, target.ID}] = true
		svc := newService(f)
		if err := svc.Follow(context.Background(), FollowCmd{FollowerID: follower.ID, TargetID: target.ID}); !errors.Is(err, ErrAlreadyFollow) {
			t.Errorf("error = %v, want ErrAlreadyFollow", err)
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
