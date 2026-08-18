// Package auth 提供 JWT 访问 token 的签发与验证（auth-flow §1，已确认：7 天无 refresh token）。
//
// 它是横切能力（user 服务签发、httpapi 中间件验证），非领域模块；user 包通过 TokenIssuer 接口依赖它（#7 接受依赖而非创建）。
package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims 是访问 token 的载荷。
// Role 取值 user/moderator/admin——DB 存 member/moderator/admin，在签发边界将 member→user 映射（见 user 包 mapRole）。
type Claims struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// Manager 负责 token 签发与验证。
type Manager struct {
	secret []byte
	ttl    time.Duration
}

// NewManager 构造 Manager。ttl 为 0 时回退默认 7 天。
func NewManager(secret string, ttl time.Duration) *Manager {
	if ttl <= 0 {
		ttl = 7 * 24 * time.Hour
	}
	return &Manager{secret: []byte(secret), ttl: ttl}
}

// Issue 为已登录用户签发 token。
func (m *Manager) Issue(userID int64, username, role string) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:   userID,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(m.ttl)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
}

// Verify 校验签名与有效期，返回载荷。
func (m *Manager) Verify(token string) (*Claims, error) {
	claims := &Claims{}
	parsed, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("auth: 意外签名算法 %v", t.Header["alg"])
		}
		return m.secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("auth: 校验失败: %w", err)
	}
	if !parsed.Valid {
		return nil, errors.New("auth: token 无效")
	}
	return claims, nil
}
