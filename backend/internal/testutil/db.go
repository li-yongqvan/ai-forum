// Package testutil 提供集成测试共用的基础设施。
package testutil

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // 注册 database/sql 驱动名 "pgx"，供 wait.ForSQL 轮询
	"github.com/li-yongqvan/ai-forum/backend/migrations"
	"github.com/moby/moby/api/types/network"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// pgDSN 生成测试库连接串（keyword 格式，host/port 显式分离，避免 URL 解析歧义）。
func pgDSN(host string, port network.Port) string {
	return fmt.Sprintf("host=%s port=%s user=test password=test dbname=aiforum_test sslmode=disable", host, port.Port())
}

// SkipIfNoDocker 在本机无 Docker CLI 或 daemon 不可用时跳过集成测试。
// CI（ubuntu-latest）自带 Docker daemon，集成测试会真实运行；本机未启动 Docker Desktop 时优雅跳过而非失败。
func SkipIfNoDocker(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("未检测到 docker CLI，跳过集成测试")
	}
	out, err := exec.Command("docker", "version", "--format", "{{.Server.Version}}").Output()
	if err != nil || strings.TrimSpace(string(out)) == "" {
		t.Skipf("Docker daemon 不可用，跳过集成测试（%v）", err)
	}
}

// SetupPG 启动真实 PostgreSQL 测试容器（testing.md §2.1：testcontainers-go 真实 PG，已确认），跑迁移，返回连接。
// 用 wait.ForSQL 以真实数据库连接轮询就绪，规避 Docker Desktop 端口代理与就绪日志的竞态（首连 EOF）。
func SetupPG(t *testing.T) *gorm.DB {
	t.Helper()
	SkipIfNoDocker(t)
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "postgres:16-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "test",
			"POSTGRES_PASSWORD": "test",
			"POSTGRES_DB":       "aiforum_test",
		},
		WaitingFor: wait.ForSQL("5432/tcp", "pgx", pgDSN).
			WithStartupTimeout(60 * time.Second),
	}
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("testcontainers: 启动 postgres 失败: %v", err)
	}
	t.Cleanup(func() { _ = c.Terminate(ctx) })

	host, err := c.Host(ctx)
	if err != nil {
		t.Fatalf("testcontainers: 获取主机失败: %v", err)
	}
	mapped, err := c.MappedPort(ctx, "5432/tcp")
	if err != nil {
		t.Fatalf("testcontainers: 获取端口映射失败: %v", err)
	}

	gdb, err := gorm.Open(postgres.Open(pgDSN(host, mapped)), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm 连接失败: %v", err)
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		t.Fatalf("获取底层连接失败: %v", err)
	}
	if err := migrations.Run(sqlDB); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	return gdb
}
