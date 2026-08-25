package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/li-yongqvan/ai-forum/backend/internal/auth"
)

type fakeChecker struct {
	active bool
	err    error
}

func (f fakeChecker) IsActive(ctx context.Context, userID int64) (bool, error) { return f.active, f.err }

func runRequireActive(t *testing.T, checker ActiveChecker, withIdentity bool) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	if withIdentity {
		// 同包可访问 ctxKeyIdentity，模拟 Auth 注入身份
		r.Use(func(c *gin.Context) {
			c.Set(string(ctxKeyIdentity), &auth.Claims{UserID: 7, Username: "u", Role: "user"})
			c.Next()
		})
	}
	r.Use(RequireActive(checker))
	r.GET("/x", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))
	return w
}

func TestRequireActive(t *testing.T) {
	t.Run("无身份 → 401", func(t *testing.T) {
		w := runRequireActive(t, fakeChecker{active: true}, false)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", w.Code)
		}
	})

	t.Run("活跃用户 → next", func(t *testing.T) {
		w := runRequireActive(t, fakeChecker{active: true}, true)
		if w.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", w.Code)
		}
	})

	t.Run("被封用户 → 403 + code=account_banned", func(t *testing.T) {
		w := runRequireActive(t, fakeChecker{active: false}, true)
		if w.Code != http.StatusForbidden {
			t.Errorf("status = %d, want 403", w.Code)
		}
		if !strings.Contains(w.Body.String(), `"code":"account_banned"`) {
			t.Errorf("body = %q, want code=account_banned", w.Body.String())
		}
	})

	t.Run("查询失败 → 500（fail-closed）", func(t *testing.T) {
		w := runRequireActive(t, fakeChecker{err: errors.New("db down")}, true)
		if w.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want 500", w.Code)
		}
	})
}
