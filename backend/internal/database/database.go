// Package database 负责 GORM 连接。
//
// 约定：schema 由迁移创建（见 backend/migrations），本包不设 search_path——
// 所有领域模型通过显式 TableName() 映射到 schema-qualified 表名（"user" 是 PG 保留字，显式表名最稳妥）。
package database

import (
	"fmt"

	"github.com/li-yongqvan/ai-forum/backend/internal/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Open 打开 GORM 连接并 ping 确认可用。
func Open(cfg config.Config) (*gorm.DB, error) {
	lvl := logger.Warn
	if cfg.Env == "development" {
		lvl = logger.Info
	}
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{
		Logger: logger.Default.LogMode(lvl),
	})
	if err != nil {
		return nil, fmt.Errorf("database: 连接失败: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("database: 获取底层连接失败: %w", err)
	}
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("database: ping 失败: %w", err)
	}
	return db, nil
}
