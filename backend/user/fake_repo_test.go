package user

import (
	"context"
	"errors"
	"sort"
	"time"
)

// fakeRepo 是 Repo 的内存实现，仅用于本包测试（testing.md §2.1：in-memory fake 跨 Service 接口 seam 验证）。
type fakeRepo struct {
	byUsername map[string]*User
	byEmail    map[string]*User
	byID       map[int64]*User
	codes      map[string]*InvitationCode
	follows    map[[2]int64]time.Time
	nextID     int64
	// 单调递增关系时钟：关注时间严格递增，排序断言确定性成立（评审 §七.3）。
	followClock time.Time
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		byUsername:  map[string]*User{},
		byEmail:     map[string]*User{},
		byID:        map[int64]*User{},
		codes:       map[string]*InvitationCode{},
		follows:     map[[2]int64]time.Time{},
		nextID:      1,
		followClock: time.Now(),
	}
}

// nextFollowTime 返回严格递增的关注时间（每次调用 +1ms，保证同一测试内先后关系可排序）。
func (f *fakeRepo) nextFollowTime() time.Time {
	f.followClock = f.followClock.Add(time.Millisecond)
	return f.followClock
}

// seedFollow 以显式时间预置关注（排序测试用确定性时间）。
func (f *fakeRepo) seedFollow(followerID, targetID int64, at time.Time) {
	f.follows[[2]int64{followerID, targetID}] = at
}

// seedUser 预置用户（用于登录等场景）。
func (f *fakeRepo) seedUser(username, email, password string) *User {
	u := &User{ID: f.nextID, Username: username, Email: email, PasswordHash: hashPassword(password), Role: "member", Status: "active"}
	f.nextID++
	f.byUsername[u.Username] = u
	f.byEmail[u.Email] = u
	f.byID[u.ID] = u
	return u
}

// seedCode 预置未使用邀请码。
func (f *fakeRepo) seedCode(code string, createdBy int64) *InvitationCode {
	ic := &InvitationCode{ID: int64(len(f.codes)) + 1, Code: code, CreatedBy: createdBy}
	f.codes[code] = ic
	return ic
}

func (f *fakeRepo) CreateUser(ctx context.Context, u *User) error {
	if _, ok := f.byUsername[u.Username]; ok {
		return errors.New("duplicate username")
	}
	u.ID = f.nextID
	f.nextID++
	f.byUsername[u.Username] = u
	f.byEmail[u.Email] = u
	f.byID[u.ID] = u
	return nil
}

func (f *fakeRepo) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	if u, ok := f.byUsername[username]; ok {
		return u, nil
	}
	return nil, ErrNotFound
}

func (f *fakeRepo) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	if u, ok := f.byEmail[email]; ok {
		return u, nil
	}
	return nil, ErrNotFound
}

func (f *fakeRepo) GetUserByID(ctx context.Context, id int64) (*User, error) {
	if u, ok := f.byID[id]; ok {
		return u, nil
	}
	return nil, ErrNotFound
}

// SetBanned 模拟 gorm 条件更新 active→banned（RowsAffected=0 → ErrAlreadyBanned）。
func (f *fakeRepo) SetBanned(ctx context.Context, id int64) error {
	u, ok := f.byID[id]
	if !ok {
		return ErrNotFound
	}
	if u.Status != "active" {
		return ErrAlreadyBanned
	}
	u.Status = "banned"
	return nil
}

// SetActive 模拟 gorm 条件更新 banned→active（RowsAffected=0 → ErrNotBanned）。
func (f *fakeRepo) SetActive(ctx context.Context, id int64) error {
	u, ok := f.byID[id]
	if !ok {
		return ErrNotFound
	}
	if u.Status != "banned" {
		return ErrNotBanned
	}
	u.Status = "active"
	return nil
}

func (f *fakeRepo) GetInvitationCode(ctx context.Context, code string) (*InvitationCode, error) {
	if ic, ok := f.codes[code]; ok {
		return ic, nil
	}
	return nil, ErrNotFound
}

func (f *fakeRepo) MarkInvitationCodeUsed(ctx context.Context, id int64, usedBy int64) error {
	for _, ic := range f.codes {
		if ic.ID == id {
			if ic.UsedBy != nil {
				return ErrInvalidInvite // 已被标记（对应 gorm 实现 RowsAffected=0 语义）
			}
			ic.UsedBy = &usedBy
			return nil
		}
	}
	return ErrInvalidInvite
}

func (f *fakeRepo) Tx(ctx context.Context, fn func(Repo) error) error {
	return fn(f) // 内存 fake 无真实事务，直接透传
}

func (f *fakeRepo) CreateFollow(ctx context.Context, fl *FollowUser) error {
	f.follows[[2]int64{fl.FollowerID, fl.TargetID}] = f.nextFollowTime()
	return nil
}

func (f *fakeRepo) DeleteFollow(ctx context.Context, followerID, targetID int64) error {
	delete(f.follows, [2]int64{followerID, targetID})
	return nil
}

func (f *fakeRepo) FollowExists(ctx context.Context, followerID, targetID int64) (bool, error) {
	_, ok := f.follows[[2]int64{followerID, targetID}]
	return ok, nil
}

func (f *fakeRepo) ListFollowedUserIDs(ctx context.Context, followerID int64) ([]int64, error) {
	ids := make([]int64, 0)
	for k := range f.follows {
		if k[0] == followerID {
			ids = append(ids, k[1])
		}
	}
	return ids, nil
}

func (f *fakeRepo) CountFollowers(ctx context.Context, targetID int64) (int, error) {
	n := 0
	for k := range f.follows {
		if k[1] == targetID {
			n++
		}
	}
	return n, nil
}

func (f *fakeRepo) CountFollowing(ctx context.Context, followerID int64) (int, error) {
	n := 0
	for k := range f.follows {
		if k[0] == followerID {
			n++
		}
	}
	return n, nil
}

// ListFollowedUsers 我关注的用户，按关注时间倒序 + id 决胜、分页（fake：跳过软删用户）。
func (f *fakeRepo) ListFollowedUsers(ctx context.Context, followerID int64, offset, limit int) ([]*User, error) {
	type item struct {
		user *User
		at   time.Time
	}
	items := make([]item, 0)
	for k, at := range f.follows {
		if k[0] != followerID {
			continue
		}
		u, ok := f.byID[k[1]]
		if !ok || u.DeletedAt.Valid {
			continue
		}
		items = append(items, item{user: u, at: at})
	}
	// 按关注时间倒序；同刻以 target id 决胜
	sort.Slice(items, func(i, j int) bool {
		if !items[i].at.Equal(items[j].at) {
			return items[i].at.After(items[j].at)
		}
		return items[i].user.ID > items[j].user.ID
	})
	start := offset
	if start > len(items) {
		start = len(items)
	}
	end := len(items)
	if limit > 0 && start+limit < len(items) {
		end = start + limit
	}
	out := make([]*User, 0, len(items[start:end]))
	for _, it := range items[start:end] {
		out = append(out, it.user)
	}
	return out, nil
}

// fakeIssuer 是 TokenIssuer 的桩实现。
type fakeIssuer struct{ token string }

func (fi fakeIssuer) Issue(userID int64, username, role string) (string, error) {
	return fi.token, nil
}
