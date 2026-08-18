package user

import (
	"context"
	"errors"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// ---- 命令 / 读模型 DTO（调用方只见这些） ----

// RegisterCmd 注册命令（auth-flow §3）：用户名、邮箱、密码、邀请码。
type RegisterCmd struct {
	Username   string
	Email      string
	Password   string
	InviteCode string
}

// LoginCmd 登录命令（auth-flow §2）：用户名 + 密码。
type LoginCmd struct {
	Username string
	Password string
}

// FollowCmd 单向关注命令（#5 D4）。
type FollowCmd struct {
	FollowerID int64
	TargetID   int64
}

// BanCmd 封禁命令。审计由 moderation 包负责（scaffold 阶段 moderation 未实现）。
type BanCmd struct {
	OperatorID int64
	TargetID   int64
}

// UserView 用户读模型，Role 为对外取值 user/moderator/admin。
type UserView struct {
	ID        int64   `json:"id"`
	Username  string  `json:"username"`
	Email     string  `json:"email"`
	Role      string  `json:"role"`
	AvatarURL *string `json:"avatar_url"`
}

// AuthResult 注册/登录成功结果（auth-flow §2：{ token, user }）。
type AuthResult struct {
	Token string   `json:"token"`
	User  UserView `json:"user"`
}

// ---- 错误（调用方据此映射 HTTP 状态） ----

var (
	ErrUsernameTaken = errors.New("user: 用户名已存在")
	ErrEmailTaken    = errors.New("user: 邮箱已存在")
	ErrInvalidInvite = errors.New("user: 邀请码无效或已使用")
	ErrWeakPassword  = errors.New("user: 密码至少 8 位")
	ErrBadCredential = errors.New("user: 用户名或密码错误")
	ErrBanned        = errors.New("user: 账号已被封禁")
	ErrNotFound      = errors.New("user: 目标不存在")
	ErrSelfFollow    = errors.New("user: 不能关注自己")
	ErrAlreadyFollow = errors.New("user: 已关注该用户")
)

// ---- Service 接口（粗粒度命令 + 读模型查询，#4/#7） ----

// TokenIssuer 签发访问 token 的能力（#7 接受依赖而非创建）。
type TokenIssuer interface {
	Issue(userID int64, username, role string) (string, error)
}

// Service 是 user 包的对外接口。调用方只跨此 seam，不接触实现内部。
type Service interface {
	Register(ctx context.Context, in RegisterCmd) (AuthResult, error)
	Login(ctx context.Context, in LoginCmd) (AuthResult, error)
	Follow(ctx context.Context, in FollowCmd) error
	Ban(ctx context.Context, in BanCmd) error
	GetUser(ctx context.Context, id int64) (UserView, error)
}

// ---- 实现 ----

const minPasswordLen = 8

type service struct {
	repo  Repo
	token TokenIssuer
}

// NewService 构造 Service。
func NewService(repo Repo, token TokenIssuer) Service {
	return &service{repo: repo, token: token}
}

var _ Service = (*service)(nil)

// Register 注册并自动登录（auth-flow §3：注册成功直接返回 token）。
// 建用户 + 标记邀请码在同一事务内完成，保证「邀请码一次性」的审计不变量（标记失败则整单回滚）。
func (s *service) Register(ctx context.Context, in RegisterCmd) (AuthResult, error) {
	username := strings.TrimSpace(in.Username)
	email := strings.TrimSpace(in.Email)
	if len(in.Password) < minPasswordLen {
		return AuthResult{}, ErrWeakPassword
	}
	if username == "" || email == "" {
		return AuthResult{}, errors.New("user: 用户名与邮箱必填")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return AuthResult{}, err
	}

	var created *User
	err = s.repo.Tx(ctx, func(repo Repo) error {
		// 邀请码准入：一次性；已用/不存在一律 ErrInvalidInvite
		code, err := repo.GetInvitationCode(ctx, strings.TrimSpace(in.InviteCode))
		if err != nil || code.UsedBy != nil {
			return ErrInvalidInvite
		}
		// 用户名/邮箱全局唯一
		if _, err := repo.GetUserByUsername(ctx, username); err == nil {
			return ErrUsernameTaken
		} else if !errors.Is(err, ErrNotFound) {
			return err
		}
		if _, err := repo.GetUserByEmail(ctx, email); err == nil {
			return ErrEmailTaken
		} else if !errors.Is(err, ErrNotFound) {
			return err
		}

		u := &User{
			Username:     username,
			Email:        email,
			PasswordHash: string(hash),
			Role:         "member",
			Status:       "active",
		}
		if err := repo.CreateUser(ctx, u); err != nil {
			return err
		}
		// 同一事务内标记邀请码已用（used_by + used_at，DB CHECK 兜底）
		if err := repo.MarkInvitationCodeUsed(ctx, code.ID, u.ID); err != nil {
			return err
		}
		created = u
		return nil
	})
	if err != nil {
		return AuthResult{}, err
	}
	return s.issueAuth(created)
}

// Login 校验用户名+密码（auth-flow §2：失败统一 ErrBadCredential，防枚举）。
func (s *service) Login(ctx context.Context, in LoginCmd) (AuthResult, error) {
	u, err := s.repo.GetUserByUsername(ctx, strings.TrimSpace(in.Username))
	if err != nil {
		return AuthResult{}, ErrBadCredential
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(in.Password)) != nil {
		return AuthResult{}, ErrBadCredential
	}
	if u.Status == "banned" {
		return AuthResult{}, ErrBanned
	}
	return s.issueAuth(u)
}

// Follow 建立单向关注；不能自关、不重复关注。
func (s *service) Follow(ctx context.Context, in FollowCmd) error {
	if in.FollowerID == in.TargetID {
		return ErrSelfFollow
	}
	if _, err := s.repo.GetUserByID(ctx, in.TargetID); err != nil {
		return ErrNotFound
	}
	exists, err := s.repo.FollowExists(ctx, in.FollowerID, in.TargetID)
	if err != nil {
		return err
	}
	if exists {
		return ErrAlreadyFollow
	}
	return s.repo.CreateFollow(ctx, &FollowUser{FollowerID: in.FollowerID, TargetID: in.TargetID})
}

// Ban 封禁用户。审计动作（moderation_actions）由 moderation 包在治理闭环中追加。
func (s *service) Ban(ctx context.Context, in BanCmd) error {
	if _, err := s.repo.GetUserByID(ctx, in.TargetID); err != nil {
		return ErrNotFound
	}
	return s.repo.UpdateUserStatus(ctx, in.TargetID, "banned")
}

// GetUser 按 id 返回用户读模型。
func (s *service) GetUser(ctx context.Context, id int64) (UserView, error) {
	u, err := s.repo.GetUserByID(ctx, id)
	if err != nil {
		return UserView{}, err
	}
	return toView(u), nil
}

func (s *service) issueAuth(u *User) (AuthResult, error) {
	role := mapRole(u.Role)
	token, err := s.token.Issue(u.ID, u.Username, role)
	if err != nil {
		return AuthResult{}, err
	}
	return AuthResult{Token: token, User: toView(u)}, nil
}

func toView(u *User) UserView {
	return UserView{
		ID:        u.ID,
		Username:  u.Username,
		Email:     u.Email,
		Role:      mapRole(u.Role),
		AvatarURL: u.AvatarURL,
	}
}

// mapRole 边界映射：#5 DB 存 member/moderator/admin；#9 权限矩阵对外为 guest/user/moderator/admin（guest 仅前端状态，永不入库、不入 token）。
func mapRole(dbRole string) string {
	if dbRole == "member" {
		return "user"
	}
	return dbRole
}
