package migrations_test

import (
	"testing"

	"github.com/li-yongqvan/ai-forum/backend/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUserIntraPackageForeignKeys 验证 0010 迁移补齐了 user schema 内的 4 处物理外键。
// 跨包引用仍保持逻辑外键，不在本测试范围内（#5）。
func TestUserIntraPackageForeignKeys(t *testing.T) {
	db := testutil.SetupPG(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)

	constraints := []struct {
		table      string
		constraint string
		column     string
	}{
		{"follows_users", "fk_follows_users_follower", "follower_id"},
		{"follows_users", "fk_follows_users_target", "target_id"},
		{"invitation_codes", "fk_invitation_codes_created_by", "created_by"},
		{"invitation_codes", "fk_invitation_codes_used_by", "used_by"},
	}

	for _, c := range constraints {
		var exists bool
		err := sqlDB.QueryRow(`
			SELECT EXISTS (
				SELECT 1 FROM information_schema.table_constraints
				WHERE constraint_schema = 'user'
				  AND table_name = $1
				  AND constraint_name = $2
				  AND constraint_type = 'FOREIGN KEY'
			)`, c.table, c.constraint).Scan(&exists)
		require.NoErrorf(t, err, "查询约束 %s 失败", c.constraint)
		require.Truef(t, exists, "约束 %s 应存在", c.constraint)
	}

	// 为合法数据准备一条用户记录。
	require.NoError(t, db.Exec(`
		INSERT INTO "user".users (id, email, username, password_hash)
		VALUES (999001, 'fk-test@example.com', 'fk-test', 'hash')
	`).Error)

	// follower_id 指向不存在的用户时应被拒绝。
	err = db.Exec(`INSERT INTO "user".follows_users (follower_id, target_id) VALUES (999999, 999001)`).Error
	assert.Error(t, err, "orphan follower_id 应违反外键约束")

	// target_id 指向不存在的用户时应被拒绝。
	err = db.Exec(`INSERT INTO "user".follows_users (follower_id, target_id) VALUES (999001, 999999)`).Error
	assert.Error(t, err, "orphan target_id 应违反外键约束")

	// created_by 指向不存在的用户时应被拒绝。
	err = db.Exec(`INSERT INTO "user".invitation_codes (code, created_by) VALUES ('TEST-CODE-001', 999999)`).Error
	assert.Error(t, err, "orphan created_by 应违反外键约束")

	// used_by 指向不存在的用户时应被拒绝（nullable 不影响 FK）。
	err = db.Exec(`INSERT INTO "user".invitation_codes (code, created_by, used_by, used_at) VALUES ('TEST-CODE-002', 999001, 999999, now())`).Error
	assert.Error(t, err, "orphan used_by 应违反外键约束")
}

// TestCrossPackageReferencesRemainLogical 确认跨包逻辑外键没有被误建成物理 FK。
func TestCrossPackageReferencesRemainLogical(t *testing.T) {
	db := testutil.SetupPG(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)

	logicalFKs := []struct {
		schema string
		table  string
		column string
	}{
		{"content", "posts", "author_id"},
		{"content", "comments", "author_id"},
		{"notify", "notifications", "recipient_id"},
		{"moderation", "reports", "reporter_id"},
	}

	for _, c := range logicalFKs {
		var exists bool
		err := sqlDB.QueryRow(`
			SELECT EXISTS (
				SELECT 1 FROM information_schema.table_constraints tc
				JOIN information_schema.key_column_usage kcu
				  ON tc.constraint_schema = kcu.constraint_schema
				 AND tc.constraint_name = kcu.constraint_name
				WHERE tc.constraint_schema = $1
				  AND tc.table_name = $2
				  AND kcu.column_name = $3
				  AND tc.constraint_type = 'FOREIGN KEY'
			)`, c.schema, c.table, c.column).Scan(&exists)
		require.NoErrorf(t, err, "查询 %s.%s.%s 外键失败", c.schema, c.table, c.column)
		assert.Falsef(t, exists, "%s.%s.%s 是跨包逻辑外键，不应存在物理 FK", c.schema, c.table, c.column)
	}
}
