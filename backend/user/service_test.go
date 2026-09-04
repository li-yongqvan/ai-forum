package user

import (
	"context"
	"errors"
	"regexp"
	"strings"
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

// placeholderEmailRe 占位邮箱格式（#76 open 版）：u_<16字节32位小写hex>@local.invalid。
var placeholderEmailRe = regexp.MustCompile(`^u_[0-9a-f]{32}@local\.invalid$`)

// #76 open 版注册主路径：免码（不传邀请码）+ 占位邮箱 + 自动登录。
func TestRegister_Success(t *testing.T) {
	f := newFakeRepo()
	f.seedCode("ABCDEF", 1) // 预置一码：验证免码路径不消耗邀请码（不变量 #1）
	svc := newService(f)

	res, err := svc.Register(context.Background(), RegisterCmd{
		Username: "alice", Password: "secret123",
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
	// 免码注册不消耗/不触碰邀请码（不变量 #1）
	ic, err := f.GetInvitationCode(context.Background(), "ABCDEF")
	if err != nil || ic.UsedBy != nil {
		t.Errorf("免码注册不应消耗邀请码，err=%v usedBy=%v", err, ic.UsedBy)
	}
	// email 忽略入参：统一占位格式
	if !placeholderEmailRe.MatchString(res.User.Email) {
		t.Errorf("占位邮箱格式 = %q, want u_<32hex>@local.invalid", res.User.Email)
	}
}

// #76 open 版：占位邮箱唯一性——两次注册生成的占位值互不相同（随机 16hex + DB UNIQUE 兜底）。
func TestRegister_PlaceholderEmailUnique(t *testing.T) {
	f := newFakeRepo()
	svc := newService(f)
	r1, err := svc.Register(context.Background(), RegisterCmd{Username: "alice", Password: "secret123"})
	if err != nil {
		t.Fatalf("alice Register() error = %v", err)
	}
	r2, err := svc.Register(context.Background(), RegisterCmd{Username: "bob", Password: "secret123"})
	if err != nil {
		t.Fatalf("bob Register() error = %v", err)
	}
	if r1.User.Email == r2.User.Email {
		t.Errorf("两次注册占位邮箱不应相同: %q", r1.User.Email)
	}
}

// #76 open 版：占位邮箱罕见撞库（users_email_key 23505 → ErrEmailTaken）重试一次，
// 重试包住整个事务（评审 D2 条件②）。注入生成器首撞后空闲，应恰好调用 2 次且注册成功。
func TestRegister_PlaceholderEmailRetryOnCollision(t *testing.T) {
	f := newFakeRepo()
	f.seedUser("alice", "u_collide@local.invalid", "secret123")
	orig := newPlaceholderEmail
	calls := 0
	newPlaceholderEmail = func() (string, error) {
		calls++
		if calls == 1 {
			return "u_collide@local.invalid", nil // 首次撞库
		}
		return "u_fresh@local.invalid", nil
	}
	defer func() { newPlaceholderEmail = orig }()

	svc := newService(f)
	res, err := svc.Register(context.Background(), RegisterCmd{Username: "bob", Password: "secret123"})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if res.User.Email != "u_fresh@local.invalid" {
		t.Errorf("重试后 email = %q, want u_fresh@local.invalid", res.User.Email)
	}
	if calls != 2 {
		t.Errorf("生成器调用次数 = %d, want 2（重试恰好一次）", calls)
	}
}

// #76 open 版：撞库重试至多一次——连续两撞不再重试，ErrEmailTaken 上抛（handler 409）。
func TestRegister_PlaceholderEmailRetryOnceOnly(t *testing.T) {
	f := newFakeRepo()
	f.seedUser("alice", "u_collide@local.invalid", "secret123")
	orig := newPlaceholderEmail
	newPlaceholderEmail = func() (string, error) {
		return "u_collide@local.invalid", nil // 恒撞库
	}
	defer func() { newPlaceholderEmail = orig }()

	svc := newService(f)
	_, err := svc.Register(context.Background(), RegisterCmd{Username: "bob", Password: "secret123"})
	if !errors.Is(err, ErrEmailTaken) {
		t.Errorf("error = %v, want ErrEmailTaken（重试至多一次）", err)
	}
}

func TestRegister_WeakPassword(t *testing.T) {
	f := newFakeRepo()
	svc := newService(f)

	_, err := svc.Register(context.Background(), RegisterCmd{
		Username: "alice", Password: "short",
	})
	if !errors.Is(err, ErrWeakPassword) {
		t.Errorf("error = %v, want ErrWeakPassword", err)
	}
}

// #76 open 版改写（原 TestRegister_InvalidInvite）：免码注册——空/无效/已使用邀请码一律不拦截、
// 不消耗（断言改验新语义，评审 F2）。
func TestRegister_IgnoresInviteCode(t *testing.T) {
	t.Run("无效邀请码不再拒", func(t *testing.T) {
		f := newFakeRepo()
		svc := newService(f)
		_, err := svc.Register(context.Background(), RegisterCmd{
			Username: "alice", Password: "secret123", InviteCode: "NOPE",
		})
		if err != nil {
			t.Errorf("免码下无效邀请码不应拒, error = %v", err)
		}
	})

	t.Run("已使用的邀请码不再拒", func(t *testing.T) {
		f := newFakeRepo()
		ic := f.seedCode("ABCDEF", 1)
		usedBy := int64(99)
		ic.UsedBy = &usedBy
		svc := newService(f)
		_, err := svc.Register(context.Background(), RegisterCmd{
			Username: "alice", Password: "secret123", InviteCode: "ABCDEF",
		})
		if err != nil {
			t.Errorf("免码下已用邀请码不应拒, error = %v", err)
		}
	})

	t.Run("邀请码保持未消耗", func(t *testing.T) {
		f := newFakeRepo()
		f.seedCode("ABCDEF", 1)
		svc := newService(f)
		if _, err := svc.Register(context.Background(), RegisterCmd{
			Username: "alice", Password: "secret123", InviteCode: "ABCDEF",
		}); err != nil {
			t.Fatalf("Register() error = %v", err)
		}
		ic, err := f.GetInvitationCode(context.Background(), "ABCDEF")
		if err != nil || ic.UsedBy != nil {
			t.Errorf("免码注册不应消耗邀请码，err=%v usedBy=%v", err, ic.UsedBy)
		}
	})
}

// #76 改写（原 TestRegister_EmptyFields）：守卫拆分——trim 后空 username 仍拒（ErrEmptyUsername，
// handler 400，评审 F5④/不变量 #14）；email 忽略入参，空值放行。
func TestRegister_EmptyFields(t *testing.T) {
	t.Run("空用户名拒（ErrEmptyUsername）", func(t *testing.T) {
		f := newFakeRepo()
		svc := newService(f)
		_, err := svc.Register(context.Background(), RegisterCmd{
			Username: "", Password: "secret123",
		})
		if !errors.Is(err, ErrEmptyUsername) {
			t.Errorf("error = %v, want ErrEmptyUsername", err)
		}
	})

	t.Run("纯空白用户名拒（required 对空格放行，靠 service 守卫）", func(t *testing.T) {
		f := newFakeRepo()
		svc := newService(f)
		_, err := svc.Register(context.Background(), RegisterCmd{
			Username: "   ", Password: "secret123",
		})
		if !errors.Is(err, ErrEmptyUsername) {
			t.Errorf("error = %v, want ErrEmptyUsername", err)
		}
	})

	t.Run("空 email 放行（忽略入参生成占位）", func(t *testing.T) {
		f := newFakeRepo()
		svc := newService(f)
		res, err := svc.Register(context.Background(), RegisterCmd{
			Username: "alice", Email: "", Password: "secret123",
		})
		if err != nil {
			t.Fatalf("空 email 不应拒, error = %v", err)
		}
		if !placeholderEmailRe.MatchString(res.User.Email) {
			t.Errorf("应生成占位邮箱, got %q", res.User.Email)
		}
	})
}

// #76 改写（原 TestRegister_Duplicate）：用户名重复仍 ErrUsernameTaken（预查 + 23505 兜底双保险）；
// email 重复子测改为「忽略入参」——带已占用 email 注册应成功（占位邮箱生成）。
func TestRegister_Duplicate(t *testing.T) {
	t.Run("用户名重复", func(t *testing.T) {
		f := newFakeRepo()
		f.seedUser("alice", "alice@x.edu", "secret123")
		svc := newService(f)
		_, err := svc.Register(context.Background(), RegisterCmd{
			Username: "alice", Password: "secret123",
		})
		if !errors.Is(err, ErrUsernameTaken) {
			t.Errorf("error = %v, want ErrUsernameTaken", err)
		}
	})

	t.Run("email 重复忽略入参（不再 ErrEmailTaken）", func(t *testing.T) {
		f := newFakeRepo()
		f.seedUser("alice", "alice@x.edu", "secret123")
		svc := newService(f)
		res, err := svc.Register(context.Background(), RegisterCmd{
			Username: "bob", Email: "alice@x.edu", Password: "secret123",
		})
		if err != nil {
			t.Fatalf("email 忽略入参, error = %v", err)
		}
		if res.User.Email == "alice@x.edu" {
			t.Errorf("应生成占位邮箱而非复用入参, got %q", res.User.Email)
		}
	})

	t.Run("banned 用户也占用名字（判重口径）", func(t *testing.T) {
		f := newFakeRepo()
		u := f.seedUser("alice", "alice@x.edu", "secret123")
		u.Status = "banned"
		svc := newService(f)
		_, err := svc.Register(context.Background(), RegisterCmd{
			Username: "alice", Password: "secret123",
		})
		if !errors.Is(err, ErrUsernameTaken) {
			t.Errorf("error = %v, want ErrUsernameTaken（banned 占用名字）", err)
		}
	})
}

// #76 D4：建议名 seam——顺序取空闲、banned 占用、≤64 截断（不变量 #13）。
func TestSuggestNextUsername(t *testing.T) {
	t.Run("空闲名 → name_2", func(t *testing.T) {
		f := newFakeRepo()
		f.seedUser("alice", "a@x.edu", "secret123")
		svc := newService(f)
		sugg, err := svc.SuggestNextUsername(context.Background(), "alice")
		if err != nil || sugg != "alice_2" {
			t.Errorf("SuggestNextUsername() = %q, %v, want alice_2", sugg, err)
		}
	})

	t.Run("name_2 已占用 → name_3", func(t *testing.T) {
		f := newFakeRepo()
		f.seedUser("alice", "a@x.edu", "secret123")
		f.seedUser("alice_2", "a2@x.edu", "secret123")
		svc := newService(f)
		sugg, err := svc.SuggestNextUsername(context.Background(), "alice")
		if err != nil || sugg != "alice_3" {
			t.Errorf("SuggestNextUsername() = %q, %v, want alice_3", sugg, err)
		}
	})

	t.Run("banned 用户占用名字（探测口径与 Register 判重一致）", func(t *testing.T) {
		f := newFakeRepo()
		f.seedUser("alice", "a@x.edu", "secret123")
		a2 := f.seedUser("alice_2", "a2@x.edu", "secret123")
		a2.Status = "banned"
		svc := newService(f)
		sugg, err := svc.SuggestNextUsername(context.Background(), "alice")
		if err != nil || sugg != "alice_3" {
			t.Errorf("SuggestNextUsername() = %q, %v, want alice_3（banned 不放行）", sugg, err)
		}
	})

	t.Run("软删用户不占建议名", func(t *testing.T) {
		f := newFakeRepo()
		f.seedUser("alice", "a@x.edu", "secret123")
		a2 := f.seedUser("alice_2", "a2@x.edu", "secret123")
		a2.DeletedAt = gorm.DeletedAt{Valid: true}
		svc := newService(f)
		sugg, err := svc.SuggestNextUsername(context.Background(), "alice")
		if err != nil || sugg != "alice_2" {
			t.Errorf("SuggestNextUsername() = %q, %v, want alice_2（软删不占）", sugg, err)
		}
	})

	t.Run("64 字符原名截断后建议名 ≤64", func(t *testing.T) {
		f := newFakeRepo()
		long := strings.Repeat("a", 64)
		f.seedUser(long, "a@x.edu", "secret123")
		svc := newService(f)
		sugg, err := svc.SuggestNextUsername(context.Background(), long)
		if err != nil {
			t.Fatalf("SuggestNextUsername() error = %v", err)
		}
		if n := len(sugg); n > maxUsernameLen {
			t.Errorf("建议名长度 = %d, want ≤%d（防 22001→500）", n, maxUsernameLen)
		}
		if !strings.HasSuffix(sugg, "_2") {
			t.Errorf("建议名 = %q, want 以 _2 结尾", sugg)
		}
	})

	t.Run("中文名按字符截断 ≤64（PG VARCHAR 按字符计）", func(t *testing.T) {
		f := newFakeRepo()
		long := strings.Repeat("中", 70)
		f.seedUser(long, "a@x.edu", "secret123")
		svc := newService(f)
		sugg, err := svc.SuggestNextUsername(context.Background(), long)
		if err != nil {
			t.Fatalf("SuggestNextUsername() error = %v", err)
		}
		if n := len([]rune(sugg)); n > maxUsernameLen {
			t.Errorf("建议名字符数 = %d, want ≤%d", n, maxUsernameLen)
		}
	})

	t.Run("空名兜底 user 前缀", func(t *testing.T) {
		f := newFakeRepo()
		svc := newService(f)
		sugg, err := svc.SuggestNextUsername(context.Background(), "  ")
		if err != nil || sugg != "user_2" {
			t.Errorf("SuggestNextUsername(空) = %q, %v, want user_2", sugg, err)
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
		if err := svc.Ban(context.Background(), BanCmd{OperatorID: 9999, TargetID: target.ID}); err != nil {
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
		if err := svc.Ban(context.Background(), BanCmd{OperatorID: 9999, TargetID: 999}); !errors.Is(err, ErrNotFound) {
			t.Errorf("error = %v, want ErrNotFound", err)
		}
	})

	t.Run("不能封自己", func(t *testing.T) {
		f := newFakeRepo()
		admin := f.seedUser("root", "r@x.edu", "secret123")
		admin.Role = "admin"
		svc := newService(f)
		if err := svc.Ban(context.Background(), BanCmd{OperatorID: admin.ID, TargetID: admin.ID}); !errors.Is(err, ErrSelfBan) {
			t.Errorf("error = %v, want ErrSelfBan", err)
		}
	})

	t.Run("不能封 admin", func(t *testing.T) {
		f := newFakeRepo()
		admin := f.seedUser("root", "r@x.edu", "secret123")
		admin.Role = "admin"
		svc := newService(f)
		if err := svc.Ban(context.Background(), BanCmd{OperatorID: 9999, TargetID: admin.ID}); !errors.Is(err, ErrCannotBanAdmin) {
			t.Errorf("error = %v, want ErrCannotBanAdmin", err)
		}
	})

	t.Run("重复封禁 → ErrAlreadyBanned", func(t *testing.T) {
		f := newFakeRepo()
		target := f.seedUser("bob", "b@x.edu", "secret123")
		svc := newService(f)
		if err := svc.Ban(context.Background(), BanCmd{OperatorID: 9999, TargetID: target.ID}); err != nil {
			t.Fatalf("首次 Ban() error = %v", err)
		}
		if err := svc.Ban(context.Background(), BanCmd{OperatorID: 9999, TargetID: target.ID}); !errors.Is(err, ErrAlreadyBanned) {
			t.Errorf("重复 Ban error = %v, want ErrAlreadyBanned", err)
		}
	})
}

func TestUnban(t *testing.T) {
	t.Run("成功", func(t *testing.T) {
		f := newFakeRepo()
		target := f.seedUser("bob", "b@x.edu", "secret123")
		target.Status = "banned"
		svc := newService(f)
		if err := svc.Unban(context.Background(), UnbanCmd{OperatorID: 1, TargetID: target.ID}); err != nil {
			t.Fatalf("Unban() error = %v", err)
		}
		u, err := f.GetUserByID(context.Background(), target.ID)
		if err != nil || u.Status != "active" {
			t.Errorf("解封后 status = %q, err = %v", u.Status, err)
		}
	})

	t.Run("目标不存在", func(t *testing.T) {
		f := newFakeRepo()
		svc := newService(f)
		if err := svc.Unban(context.Background(), UnbanCmd{OperatorID: 1, TargetID: 999}); !errors.Is(err, ErrNotFound) {
			t.Errorf("error = %v, want ErrNotFound", err)
		}
	})

	t.Run("未封禁 → ErrNotBanned", func(t *testing.T) {
		f := newFakeRepo()
		target := f.seedUser("bob", "b@x.edu", "secret123")
		svc := newService(f)
		if err := svc.Unban(context.Background(), UnbanCmd{OperatorID: 1, TargetID: target.ID}); !errors.Is(err, ErrNotBanned) {
			t.Errorf("error = %v, want ErrNotBanned", err)
		}
	})
}

func TestIsActive(t *testing.T) {
	t.Run("active → true", func(t *testing.T) {
		f := newFakeRepo()
		target := f.seedUser("bob", "b@x.edu", "secret123")
		svc := newService(f)
		active, err := svc.IsActive(context.Background(), target.ID)
		if err != nil || !active {
			t.Errorf("IsActive = %v, err = %v, want true", active, err)
		}
	})

	t.Run("banned → false", func(t *testing.T) {
		f := newFakeRepo()
		target := f.seedUser("bob", "b@x.edu", "secret123")
		target.Status = "banned"
		svc := newService(f)
		active, err := svc.IsActive(context.Background(), target.ID)
		if err != nil || active {
			t.Errorf("IsActive = %v, err = %v, want false", active, err)
		}
	})

	t.Run("软删/不存在 → false", func(t *testing.T) {
		f := newFakeRepo()
		svc := newService(f)
		active, err := svc.IsActive(context.Background(), 999)
		if err != nil || active {
			t.Errorf("IsActive = %v, err = %v, want false", active, err)
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
