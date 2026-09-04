package user

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/li-yongqvan/ai-forum/backend/internal/testutil"
)

// #76 §8：23505→哨兵映射单测。纯函数样本覆盖约束名命中/为空/非唯一冲突（评审 Q2 附带）；
// 真实 PG 样本（testcontainers）覆盖 gorm 适配层端到端翻译。
func TestPgUniqueViolation(t *testing.T) {
	t.Run("命中 users_username_key", func(t *testing.T) {
		err := &pgconn.PgError{Code: "23505", ConstraintName: constraintUsernameKey}
		if got := pgUniqueViolation(err); got != constraintUsernameKey {
			t.Errorf("pgUniqueViolation() = %q, want %q", got, constraintUsernameKey)
		}
	})

	t.Run("命中 users_email_key", func(t *testing.T) {
		err := &pgconn.PgError{Code: "23505", ConstraintName: constraintEmailKey}
		if got := pgUniqueViolation(err); got != constraintEmailKey {
			t.Errorf("pgUniqueViolation() = %q, want %q", got, constraintEmailKey)
		}
	})

	t.Run("23505 但约束名为空 → 空（原样上抛路径，宁 500 不误吞）", func(t *testing.T) {
		err := &pgconn.PgError{Code: "23505", ConstraintName: ""}
		if got := pgUniqueViolation(err); got != "" {
			t.Errorf("pgUniqueViolation() = %q, want 空", got)
		}
	})

	t.Run("非 23505 → 空", func(t *testing.T) {
		err := &pgconn.PgError{Code: "42P01", ConstraintName: "whatever"}
		if got := pgUniqueViolation(err); got != "" {
			t.Errorf("pgUniqueViolation() = %q, want 空", got)
		}
	})

	t.Run("普通 error → 空", func(t *testing.T) {
		if got := pgUniqueViolation(errors.New("boom")); got != "" {
			t.Errorf("pgUniqueViolation() = %q, want 空", got)
		}
	})
}

// CreateUser 端到端：真实 PG 唯一冲突 → 哨兵（gorm 适配层按约束名匹配，#76 D8）。
func TestGormRepo_CreateUser_UniqueViolation(t *testing.T) {
	gdb := testutil.SetupPG(t)
	repo := NewGormRepo(gdb)
	ctx := context.Background()

	seed := func(username, email string) {
		t.Helper()
		if err := repo.CreateUser(ctx, &User{
			Username: username, Email: email, PasswordHash: "h", Role: "member", Status: "active",
		}); err != nil {
			t.Fatalf("seed %s 失败: %v", username, err)
		}
	}
	seed("alice", "u_alice@local.invalid")

	t.Run("无冲突 → nil", func(t *testing.T) {
		err := repo.CreateUser(ctx, &User{
			Username: "bob", Email: "u_bob@local.invalid", PasswordHash: "h", Role: "member", Status: "active",
		})
		if err != nil {
			t.Errorf("CreateUser() error = %v, want nil", err)
		}
	})

	t.Run("同用户名 → ErrUsernameTaken（并发抢名输家路径）", func(t *testing.T) {
		err := repo.CreateUser(ctx, &User{
			Username: "alice", Email: "u_other@local.invalid", PasswordHash: "h", Role: "member", Status: "active",
		})
		if !errors.Is(err, ErrUsernameTaken) {
			t.Errorf("error = %v, want ErrUsernameTaken", err)
		}
	})

	t.Run("同邮箱 → ErrEmailTaken（占位撞库兜底路径）", func(t *testing.T) {
		err := repo.CreateUser(ctx, &User{
			Username: "carol", Email: "u_alice@local.invalid", PasswordHash: "h", Role: "member", Status: "active",
		})
		if !errors.Is(err, ErrEmailTaken) {
			t.Errorf("error = %v, want ErrEmailTaken", err)
		}
	})
}
