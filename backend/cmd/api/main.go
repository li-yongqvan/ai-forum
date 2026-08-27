// Package main 是 ai-forum 后端（组合 A 模块化单体）入口。
// 装配顺序：config → database → migrate → auth → user → httpapi（#8 部署约定：迁移随应用启动）。
package main

import (
	"log/slog"
	"os"
	"time"

	"github.com/li-yongqvan/ai-forum/backend/content"
	"github.com/li-yongqvan/ai-forum/backend/internal/auth"
	"github.com/li-yongqvan/ai-forum/backend/internal/config"
	"github.com/li-yongqvan/ai-forum/backend/internal/database"
	"github.com/li-yongqvan/ai-forum/backend/internal/httpapi"
	"github.com/li-yongqvan/ai-forum/backend/migrations"
	"github.com/li-yongqvan/ai-forum/backend/moderation"
	"github.com/li-yongqvan/ai-forum/backend/notify"
	"github.com/li-yongqvan/ai-forum/backend/upload"
	"github.com/li-yongqvan/ai-forum/backend/user"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("配置加载失败", "err", err)
		os.Exit(1)
	}

	db, err := database.Open(cfg)
	if err != nil {
		logger.Error("数据库连接失败", "err", err)
		os.Exit(1)
	}
	sqlDB, err := db.DB()
	if err != nil {
		logger.Error("获取数据库连接失败", "err", err)
		os.Exit(1)
	}
	if err := migrations.Run(sqlDB); err != nil {
		logger.Error("数据库迁移失败", "err", err)
		os.Exit(1)
	}

	jwtMgr := auth.NewManager(cfg.JWTSecret, 7*24*time.Hour)
	userSvc := user.NewService(user.NewGormRepo(db), jwtMgr)
	// #72：content 需要 Notifier 包装 notifySvc，须先于 contentSvc 装配
	notifySvc := notify.NewService(notify.NewGormRepo(db))
	contentSvc := content.NewService(
		content.NewGormRepo(db),
		httpapi.NewUserProvider(userSvc),
		httpapi.NewContentNotifier(notifySvc),
	)
	uploadSvc := upload.NewService(upload.Config{Dir: cfg.UploadsDir, MaxBytes: cfg.MaxUploadBytes})
	moderationSvc := moderation.NewService(
		moderation.NewGormRepo(db),
		httpapi.NewContentGateway(contentSvc),
		httpapi.NewUserGateway(userSvc),
		httpapi.NewNotifier(notifySvc),
	)

	r := httpapi.NewEngine(cfg, jwtMgr, userSvc, contentSvc, uploadSvc, notifySvc, moderationSvc)
	logger.Info("服务启动", "port", cfg.Port, "env", cfg.Env)
	if err := r.Run(":" + cfg.Port); err != nil {
		logger.Error("服务退出", "err", err)
		os.Exit(1)
	}
}
