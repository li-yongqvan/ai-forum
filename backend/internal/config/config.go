// Package config 从环境变量加载应用配置（#8：.env 即配置源，MVP 不引 viper）。
package config

import (
	"fmt"
	"os"
)

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
}

// Load 读取环境变量并校验必填项。
func Load() (Config, error) {
	cfg := Config{
		Env:         getenv("APP_ENV", "development"),
		Port:        getenv("APP_PORT", "8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		JWTSecret:   os.Getenv("JWT_SECRET"),
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
