package user

import (
	"context"
	"errors"
)

// fakeRepo 是 Repo 的内存实现，仅用于本包测试（testing.md §2.1：in-memory fake 跨 Service 接口 seam 验证）。
type fakeRepo struct {
	byUsername map[string]*User
	byEmail    map[string]*User
	byID       map[int64]*User
	codes      map[string]*InvitationCode
	follows    map[[2]int64]bool
	nextID     int64
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		byUsername: map[string]*User{},
		byEmail:    map[string]*User{},
		byID:       map[int64]*User{},
		codes:      map[string]*InvitationCode{},
		follows:    map[[2]int64]bool{},
		nextID:     1,
	}
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

func (f *fakeRepo) UpdateUserStatus(ctx context.Context, id int64, status string) error {
	u, ok := f.byID[id]
	if !ok {
		return ErrNotFound
	}
	u.Status = status
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
	f.follows[[2]int64{fl.FollowerID, fl.TargetID}] = true
	return nil
}

func (f *fakeRepo) DeleteFollow(ctx context.Context, followerID, targetID int64) error {
	delete(f.follows, [2]int64{followerID, targetID})
	return nil
}

func (f *fakeRepo) FollowExists(ctx context.Context, followerID, targetID int64) (bool, error) {
	return f.follows[[2]int64{followerID, targetID}], nil
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

// fakeIssuer 是 TokenIssuer 的桩实现。
type fakeIssuer struct{ token string }

func (fi fakeIssuer) Issue(userID int64, username, role string) (string, error) {
	return fi.token, nil
}
