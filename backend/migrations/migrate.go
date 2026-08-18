// Package migrations 以内嵌 SQL 文件的方式管理数据库迁移（#5 原生 DDL，GORM AutoMigrate 表达不了 CHECK/部分唯一索引）。
//
// 设计：文件按字典序作为迁移顺序，每个迁移在一个事务内逐条执行，结果记录到 schema_migrations 表。
// 应用启动时调用（#8：docker compose pull && up -d 即部署，无独立迁移步骤）；幂等，可反复调用。
package migrations

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

//go:embed *.sql
var files embed.FS

// Run 应用尚未执行的迁移。sqlDB 需是真实 PostgreSQL 连接。
func Run(sqlDB *sql.DB) error {
	names, err := list()
	if err != nil {
		return err
	}
	if _, err := sqlDB.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version VARCHAR(255) PRIMARY KEY,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`); err != nil {
		return fmt.Errorf("migrate: 初始化迁移表失败: %w", err)
	}
	applied, err := appliedSet(sqlDB)
	if err != nil {
		return err
	}
	for _, name := range names {
		if applied[name] {
			continue
		}
		body, err := files.ReadFile(name)
		if err != nil {
			return fmt.Errorf("migrate: 读取 %s 失败: %w", name, err)
		}
		tx, err := sqlDB.Begin()
		if err != nil {
			return fmt.Errorf("migrate: 开启事务失败: %w", err)
		}
		if err := applyStatements(tx, string(body)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migrate: 应用 %s 失败: %w", name, err)
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations(version) VALUES ($1)`, name); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migrate: 记录版本 %s 失败: %w", name, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("migrate: 提交 %s 失败: %w", name, err)
		}
	}
	return nil
}

// applyStatements 将迁移文件按分号拆分逐条执行。
// 迁移文件为纯 DDL（CHECK 内的单引号值不含分号），按 ";" 拆分安全。
func applyStatements(tx *sql.Tx, body string) error {
	for _, stmt := range strings.Split(body, ";") {
		s := strings.TrimSpace(stmt)
		if s == "" {
			continue
		}
		if _, err := tx.Exec(s); err != nil {
			return err
		}
	}
	return nil
}

func list() ([]string, error) {
	entries, err := fs.ReadDir(files, ".")
	if err != nil {
		return nil, fmt.Errorf("migrate: 读取嵌入目录失败: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names, nil
}

func appliedSet(sqlDB *sql.DB) (map[string]bool, error) {
	rows, err := sqlDB.Query(`SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("migrate: 读取已应用版本失败: %w", err)
	}
	defer func() { _ = rows.Close() }()
	set := map[string]bool{}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, fmt.Errorf("migrate: 扫描版本失败: %w", err)
		}
		set[v] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return set, nil
}
