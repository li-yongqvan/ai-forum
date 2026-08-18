// Package upload 处理本地图片上传（#12：MVP 本地上传，无对象存储）。
// 职责：类型/大小双校验 + 磁盘落盘 + 生成对外可访问的相对路径。
// 边界（#12 D5）：图片是磁盘文件，post/comment 的 markdown 直接引用返回的 URL，
// 与 #5 数据模型无关，无 DB 表/迁移；缩略图、删除/管理页、配额均不做。
package upload

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// DefaultMaxBytes 单文件大小上限默认值（#12 D2：5MB，可经 UPLOAD_MAX_BYTES 覆盖）。
const DefaultMaxBytes int64 = 5 << 20

// Config 上传模块配置。
type Config struct {
	// Dir 存储目录（生产 /opt/ai-forum/uploads，nginx 托管；开发/测试本地目录）。
	Dir string
	// MaxBytes 单文件大小上限；<=0 时用 DefaultMaxBytes。
	MaxBytes int64
}

// 白名单：允许的扩展名 → 期望的 MIME（#12 D2：扩展名 + MIME sniff 双校验，防伪造扩展名）。
var allowedExt = map[string]string{
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".gif":  "image/gif",
	".webp": "image/webp",
}

// 哨兵错误（handler 映射 HTTP 状态码）。
var (
	ErrNoFile   = errors.New("upload: 未选择图片文件")
	ErrTooLarge = errors.New("upload: 图片超过大小上限")
	ErrBadType  = errors.New("upload: 仅支持 jpg/png/gif/webp 图片")
	ErrEmpty    = errors.New("upload: 文件内容为空")
)

// Service 是上传模块对外接口（#7 深模块：调用方只依赖本接口；无 DB，只有磁盘副作用）。
type Service interface {
	// Store 校验并保存一张图片，返回可访问的相对路径（如 /uploads/<uuid>.png）。
	// 文件名由服务端生成（UUID），不信任用户文件名（#12 D3：防路径穿越）。
	Store(filename string, r io.Reader) (string, error)
	// MaxBytes 返回生效的大小上限（handler 层 MaxBytesReader 提前拦截用）。
	MaxBytes() int64
}

type service struct {
	cfg Config
}

// NewService 装配上传服务（cfg.MaxBytes<=0 时落为 DefaultMaxBytes）。
func NewService(cfg Config) Service {
	if cfg.MaxBytes <= 0 {
		cfg.MaxBytes = DefaultMaxBytes
	}
	return &service{cfg: cfg}
}

func (s *service) MaxBytes() int64 { return s.cfg.MaxBytes }

func (s *service) Store(filename string, r io.Reader) (string, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	wantMIME, ok := allowedExt[ext]
	if !ok {
		return "", ErrBadType
	}
	// 读入内存并限制大小（handler 层已用 MaxBytesReader 提前拦截，此处双保险）。
	// 超限返回 ErrTooLarge 而非读到截断内容，避免落盘半截文件。
	data, err := io.ReadAll(io.LimitReader(r, s.cfg.MaxBytes+1))
	if err != nil {
		return "", fmt.Errorf("upload: 读取文件失败: %w", err)
	}
	if len(data) == 0 {
		return "", ErrEmpty
	}
	if int64(len(data)) > s.cfg.MaxBytes {
		return "", ErrTooLarge
	}
	// MIME sniff：与扩展名期望一致，防伪造扩展名（如 .png 实为脚本/文本）。
	if sniffed := http.DetectContentType(data); sniffed != wantMIME {
		return "", ErrBadType
	}
	if err := os.MkdirAll(s.cfg.Dir, 0o755); err != nil {
		return "", fmt.Errorf("upload: 创建存储目录失败: %w", err)
	}
	name := randHex(16) + ext
	if err := os.WriteFile(filepath.Join(s.cfg.Dir, name), data, 0o644); err != nil {
		return "", fmt.Errorf("upload: 写入文件失败: %w", err)
	}
	return "/uploads/" + name, nil
}

// randHex 生成 n 字节随机数的十六进制串（无额外依赖；失败 panic 与 uuid 库同策略）。
func randHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("upload: 随机数生成失败: %v", err))
	}
	return hex.EncodeToString(b)
}
