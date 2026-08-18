// Package config 从环境变量加载应用配置（#8：.env 即配置源，MVP 不引 viper）。
package config

import (
	"fmt"
	"os"
	"strconv"
)

// DefaultUploadBytes 单图片大小上限默认值（#12 D2：5MB，可经 UPLOAD_MAX_BYTES 覆盖）。
const DefaultUploadBytes = 5 << 20

// Config 是应用运行所需的最小配置集。
type Config struct {
	// Env 运行环境：development | production。
	Env string
	// Port 监听端口，默认 8080。
	Port string
	// DatabaseURL PostgreSQL 连接串，如 postgres://user:pass@host:5432/db?sslmode=disable。
	DatabaseURL string
	// JWTSecret 访问 token 签名密钥（生产须为强随机串，#8 .env.example）。
	JWTSecret string
	// UploadsDir 图片存储目录（#12 D1：生产 /opt/ai-forum/uploads，nginx 托管；
	// 开发默认 ./data/uploads 以便本机直接跑）。
	UploadsDir string
	// MaxUploadBytes 单图片大小上限（字节，#12 D2：默认 5MB）。
	MaxUploadBytes int64
}

// Load 读取环境变量并校验必填项。
func Load() (Config, error) {
	cfg := Config{
		Env:            getenv("APP_ENV", "development"),
		Port:           getenv("APP_PORT", "8080"),
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		JWTSecret:      os.Getenv("JWT_SECRET"),
		UploadsDir:     getenv("UPLOADS_DIR", ""),
		MaxUploadBytes: int64(getenvInt("UPLOAD_MAX_BYTES", DefaultUploadBytes)),
	}
	// 上传目录默认：生产按 #8 约定，开发用本地相对目录（Windows/容器内均可写）。
	if cfg.UploadsDir == "" {
		if cfg.Env == "production" {
			cfg.UploadsDir = "/opt/ai-forum/uploads"
		} else {
			cfg.UploadsDir = "./data/uploads"
		}
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("config: DATABASE_URL 未设置")
	}
	if cfg.JWTSecret == "" {
		return Config{}, fmt.Errorf("config: JWT_SECRET 未设置")
	}
	if cfg.Env != "development" && cfg.Env != "production" {
		return Config{}, fmt.Errorf("config: APP_ENV 非法值 %q（development|production）", cfg.Env)
	}
	return cfg, nil
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getenvInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}
