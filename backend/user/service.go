package user

import (
	"context"
	"errors"
	"strings"
	"time"

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

// BanCmd 封禁命令。审计由 moderation 包负责（handler 层调用 moderation.RecordAction，#34）。
type BanCmd struct {
	OperatorID int64
	TargetID   int64
}

// UnbanCmd 解封命令（#34）。审计由 moderation 包负责。
type UnbanCmd struct {
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
	ErrUsernameTaken  = errors.New("user: 用户名已存在")
	ErrEmailTaken     = errors.New("user: 邮箱已存在")
	ErrInvalidInvite  = errors.New("user: 邀请码无效或已使用")
	ErrWeakPassword   = errors.New("user: 密码至少 8 位")
	ErrBadCredential  = errors.New("user: 用户名或密码错误")
	ErrBanned         = errors.New("user: 账号已被封禁")
	ErrNotFound       = errors.New("user: 目标不存在")
	ErrSelfFollow     = errors.New("user: 不能关注自己")
	ErrAlreadyFollow  = errors.New("user: 已关注该用户")
	ErrAuthRequired   = errors.New("user: 需要登录") // #23 收藏/关注列表游客
	ErrSelfBan        = errors.New("user: 不能封禁自己")
	ErrCannotBanAdmin = errors.New("user: 不能封禁管理员")
	ErrAlreadyBanned  = errors.New("user: 该用户已被封禁")
	ErrNotBanned      = errors.New("user: 该用户未被封禁")
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
	Unfollow(ctx context.Context, in FollowCmd) error
	Ban(ctx context.Context, in BanCmd) error
	// Unban 解封用户（#34）。幂等：未封则 ErrNotBanned。
	Unban(ctx context.Context, in UnbanCmd) error
	// IsActive 返回用户是否可执行写操作（存在且未被封禁；软删/不存在视为不活跃，供写拦截中间件）。
	IsActive(ctx context.Context, userID int64) (bool, error)
	GetUser(ctx context.Context, id int64) (UserView, error)
	// GetUserByUsername 按用户名查「存在且 active」的用户（#72 提及解析）。不存在/封禁/软删 → ErrNotFound。
	GetUserByUsername(ctx context.Context, username string) (UserView, error)
	// FollowedUserIDs 返回某用户关注的用户 id 列表（供 content 构造关注流，#4 进程内调用）。
	FollowedUserIDs(ctx context.Context, userID int64) ([]int64, error)
	// FollowsUser 判断 followerID 是否已关注 targetID（供 content 构造 viewer.following_author）。
	FollowsUser(ctx context.Context, followerID, targetID int64) (bool, error)
	// PublicProfile 返回用户公开资料（不含邮箱/角色等隐私字段）。
	PublicProfile(ctx context.Context, in PublicProfileCmd) (PublicProfileView, error)
	// ListFollowedUsers 返回我关注的用户，按关注时间倒序 + 分页（#23，需登录）。
	ListFollowedUsers(ctx context.Context, in ListFollowedUsersQuery) ([]UserFollowView, error)
}

// PublicProfileCmd 公开资料查询。
type PublicProfileCmd struct {
	TargetID int64
	ViewerID int64 // 0 = 游客（不附 Following）
}

// PublicProfileView 公开资料读模型。
type PublicProfileView struct {
	ID             int64
	Username       string
	AvatarURL      *string
	Bio            *string
	CreatedAt      time.Time
	Following      bool
	FollowerCount  int
	FollowingCount int
	Banned         bool // #34：当前是否被封禁（可见性门控在 handler：仅 admin/self 序列化）
}

// ListFollowedUsersQuery 我关注的用户列表（#23）。
type ListFollowedUsersQuery struct {
	ViewerID int64 // 0 = 游客
	Page     int
	PageSize int
}

// UserFollowView 关注列表的用户行（最小画像字段；刻意不复用 UserView/PublicProfileView——
// UserView 含 Email/Role 不可外泄，PublicProfileView 带多余计数）。
type UserFollowView struct {
	ID        int64             `json:"id"`
	Username  string            `json:"username"`
	AvatarURL *string           `json:"avatar_url"`
	Bio       *string           `json:"bio"`
	Viewer    *UserFollowViewer `json:"viewer,omitempty"`
}

// UserFollowViewer 关注行的登录态（恒 true：列表即"我关注的"）。
type UserFollowViewer struct {
	Following bool `json:"following"`
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

// Unfollow 取消单向关注；幂等（未关注也返回 nil，兼容 UI 防抖）。
func (s *service) Unfollow(ctx context.Context, in FollowCmd) error {
	return s.repo.DeleteFollow(ctx, in.FollowerID, in.TargetID)
}

// FollowedUserIDs 返回关注的用户 id 列表。
func (s *service) FollowedUserIDs(ctx context.Context, userID int64) ([]int64, error) {
	return s.repo.ListFollowedUserIDs(ctx, userID)
}

// FollowsUser 判断是否已关注目标用户。
func (s *service) FollowsUser(ctx context.Context, followerID, targetID int64) (bool, error) {
	return s.repo.FollowExists(ctx, followerID, targetID)
}

// PublicProfile 返回公开资料与计数（粉丝数/关注数）；ViewerID 非 0 时附 Following。
func (s *service) PublicProfile(ctx context.Context, in PublicProfileCmd) (PublicProfileView, error) {
	u, err := s.repo.GetUserByID(ctx, in.TargetID)
	if err != nil {
		return PublicProfileView{}, err
	}
	followers, err := s.repo.CountFollowers(ctx, in.TargetID)
	if err != nil {
		return PublicProfileView{}, err
	}
	following, err := s.repo.CountFollowing(ctx, in.TargetID)
	if err != nil {
		return PublicProfileView{}, err
	}
	v := PublicProfileView{
		ID:             u.ID,
		Username:       u.Username,
		AvatarURL:      u.AvatarURL,
		Bio:            u.Bio,
		CreatedAt:      u.CreatedAt,
		FollowerCount:  followers,
		FollowingCount: following,
		Banned:         u.Status == "banned",
	}
	if in.ViewerID != 0 {
		f, err := s.repo.FollowExists(ctx, in.ViewerID, in.TargetID)
		if err != nil {
			return PublicProfileView{}, err
		}
		v.Following = f
	}
	return v, nil
}

// normalizePage 统一分页归一化（与 content 包各持一份，包间不共享工具函数：#4 深模块）。
func normalizePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

// ListFollowedUsers 返回我关注的用户，按关注时间倒序 + 分页（#23）。
func (s *service) ListFollowedUsers(ctx context.Context, in ListFollowedUsersQuery) ([]UserFollowView, error) {
	if in.ViewerID == 0 {
		return nil, ErrAuthRequired
	}
	page, pageSize := normalizePage(in.Page, in.PageSize)
	us, err := s.repo.ListFollowedUsers(ctx, in.ViewerID, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, err
	}
	// make([]T,0,n)：空结果序列化为 "items":[] 而非 null（信封契约，评审 D2）
	views := make([]UserFollowView, 0, len(us))
	for _, u := range us {
		views = append(views, UserFollowView{
			ID:        u.ID,
			Username:  u.Username,
			AvatarURL: u.AvatarURL,
			Bio:       u.Bio,
			Viewer:    &UserFollowViewer{Following: true},
		})
	}
	return views, nil
}

// Ban 封禁用户（#34：存在性 → 自封防护 → 封 admin 防护 → 条件更新 active→banned）。
// 幂等：已封则 ErrAlreadyBanned（条件更新 RowsAffected=0 哨兵），不写审计、不产生重复审计。
// 审计动作（moderation_actions）由 handler 层调用 moderation.RecordAction 追加（状态先改、审计后写，§6.1）。
func (s *service) Ban(ctx context.Context, in BanCmd) error {
	target, err := s.repo.GetUserByID(ctx, in.TargetID)
	if err != nil {
		return err
	}
	if in.TargetID == in.OperatorID {
		return ErrSelfBan
	}
	if target.Role == "admin" {
		return ErrCannotBanAdmin
	}
	return s.repo.SetBanned(ctx, in.TargetID)
}

// Unban 解封用户（#34）：存在性 → 条件更新 banned→active；未封则 ErrNotBanned（幂等防重复审计）。
func (s *service) Unban(ctx context.Context, in UnbanCmd) error {
	if _, err := s.repo.GetUserByID(ctx, in.TargetID); err != nil {
		return err
	}
	return s.repo.SetActive(ctx, in.TargetID)
}

// IsActive 返回用户是否可执行写操作（存在且 status != banned）；软删/不存在视为不活跃（#34 fail-closed）。
func (s *service) IsActive(ctx context.Context, userID int64) (bool, error) {
	u, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return false, nil // 软删/不存在 → 拦截（安全）
		}
		return false, err
	}
	return u.Status != "banned", nil
}

// GetUser 按 id 返回用户读模型。
func (s *service) GetUser(ctx context.Context, id int64) (UserView, error) {
	u, err := s.repo.GetUserByID(ctx, id)
	if err != nil {
		return UserView{}, err
	}
	return toView(u), nil
}

// GetUserByUsername 按用户名查「存在且 active」的用户（#72 提及解析）。不存在/封禁/软删 → ErrNotFound，
// 与 IsActive 同 fail-closed 语义：content 据此决定 @username 是否渲染为链接。
func (s *service) GetUserByUsername(ctx context.Context, username string) (UserView, error) {
	u, err := s.repo.GetUserByUsername(ctx, username)
	if err != nil {
		return UserView{}, err
	}
	if u.Status != "active" {
		return UserView{}, ErrNotFound
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
