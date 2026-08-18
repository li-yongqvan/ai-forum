package upload_test

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/li-yongqvan/ai-forum/backend/upload"
)

// tinyPNG 生成 1×1 有效 PNG（真实编码，http.DetectContentType 会识别为 image/png）。
func tinyPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.SetRGBA(0, 0, color.RGBA{R: 255, G: 0, B: 0, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("编码测试 PNG 失败: %v", err)
	}
	return buf.Bytes()
}

// 主路径：合法 PNG 落盘成功，返回 /uploads/<随机名>.png，内容一致。
func TestStoreValidPNG(t *testing.T) {
	dir := t.TempDir()
	svc := upload.NewService(upload.Config{Dir: dir})

	data := tinyPNG(t)
	rel, err := svc.Store("照片.png", bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Store 失败: %v", err)
	}
	if !strings.HasPrefix(rel, "/uploads/") || !strings.HasSuffix(rel, ".png") {
		t.Errorf("rel = %q, want /uploads/<uuid>.png", rel)
	}
	// 文件名应为服务端生成的随机名，而非用户文件名
	if strings.Contains(rel, "照片") {
		t.Errorf("rel 不应包含用户文件名: %q", rel)
	}
	got, err := os.ReadFile(filepath.Join(dir, filepath.Base(rel)))
	if err != nil {
		t.Fatalf("读取落盘文件失败: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Error("落盘内容与上传不一致")
	}
	// 二次上传不覆盖（文件名唯一）
	rel2, err := svc.Store("another.png", bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Store 二次失败: %v", err)
	}
	if rel == rel2 {
		t.Errorf("两次上传文件名应不同: %q == %q", rel, rel2)
	}
}

// 扩展名白名单：非白名单（.txt/.svg/无扩展名）一律 ErrBadType。
func TestStoreExtensionWhitelist(t *testing.T) {
	dir := t.TempDir()
	svc := upload.NewService(upload.Config{Dir: dir})
	data := tinyPNG(t)

	for _, name := range []string{"a.txt", "b.svg", "noext", "c.webm", "d.mp4"} {
		if _, err := svc.Store(name, bytes.NewReader(data)); !errors.Is(err, upload.ErrBadType) {
			t.Errorf("Store(%q) = %v, want ErrBadType", name, err)
		}
	}
}

// MIME sniff 双校验：扩展名 .png 但内容非图片 → ErrBadType（防伪造扩展名）。
func TestStoreMIMEMismatch(t *testing.T) {
	dir := t.TempDir()
	svc := upload.NewService(upload.Config{Dir: dir})

	if _, err := svc.Store("fake.png", strings.NewReader("<?php echo 1; ?>")); !errors.Is(err, upload.ErrBadType) {
		t.Errorf("伪 .png = %v, want ErrBadType", err)
	}
	// jpg 扩展名配 png 内容 → 同样拦截
	if _, err := svc.Store("fake.jpg", bytes.NewReader(tinyPNG(t))); !errors.Is(err, upload.ErrBadType) {
		t.Errorf("伪 .jpg = %v, want ErrBadType", err)
	}
}

// 大小上限：超过默认 5MB → ErrTooLarge。
func TestStoreTooLarge(t *testing.T) {
	dir := t.TempDir()
	svc := upload.NewService(upload.Config{Dir: dir})

	big := bytes.Repeat([]byte{0x89, 0x50, 0x4e, 0x47}, 2<<20) // 8MB > 5MB
	if _, err := svc.Store("big.png", bytes.NewReader(big)); !errors.Is(err, upload.ErrTooLarge) {
		t.Errorf("超大文件 = %v, want ErrTooLarge", err)
	}
}

// 空文件 → ErrEmpty。
func TestStoreEmpty(t *testing.T) {
	dir := t.TempDir()
	svc := upload.NewService(upload.Config{Dir: dir})
	if _, err := svc.Store("a.png", strings.NewReader("")); !errors.Is(err, upload.ErrEmpty) {
		t.Errorf("空文件 = %v, want ErrEmpty", err)
	}
}

// 目录不存在时自动创建。
func TestStoreCreatesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "uploads") // 多级不存在目录
	svc := upload.NewService(upload.Config{Dir: dir})
	if _, err := svc.Store("a.png", bytes.NewReader(tinyPNG(t))); err != nil {
		t.Fatalf("Store 失败: %v", err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("存储目录应被自动创建: %v", err)
	}
}

// 防路径穿越（#12 D3）：恶意文件名只取白名单扩展名，落盘必在 Dir 内。
func TestStorePathTraversalRejected(t *testing.T) {
	dir := t.TempDir()
	svc := upload.NewService(upload.Config{Dir: dir})

	for _, name := range []string{"../../etc/passwd.png", `..\..\windows\evil.png`, "a.png/../../evil.png"} {
		rel, err := svc.Store(name, bytes.NewReader(tinyPNG(t)))
		if err != nil {
			t.Fatalf("Store(%q) 意外失败: %v", name, err)
		}
		base := filepath.Base(rel)
		if strings.Contains(base, "/") || strings.Contains(base, "\\") || strings.Contains(base, "..") {
			t.Errorf("Store(%q) 生成危险文件名: %q", name, base)
		}
		// 文件必须落在 Dir 内且可读
		got, err := os.ReadFile(filepath.Join(dir, base))
		if err != nil {
			t.Fatalf("Store(%q) 落盘位置错误: %v", name, err)
		}
		if len(got) == 0 {
			t.Error("落盘内容为空")
		}
		// 不得在目录外生成文件
		if _, err := os.Stat(filepath.Join(filepath.Dir(dir), base)); !os.IsNotExist(err) {
			t.Errorf("Store(%q) 在目录外留下了文件", name)
		}
	}
}

// 自定义上限生效（MaxBytes 配小值）。
func TestStoreCustomMaxBytes(t *testing.T) {
	dir := t.TempDir()
	svc := upload.NewService(upload.Config{Dir: dir, MaxBytes: 64})
	if svc.MaxBytes() != 64 {
		t.Errorf("MaxBytes() = %d, want 64", svc.MaxBytes())
	}
	if _, err := svc.Store("a.png", bytes.NewReader(tinyPNG(t))); err == nil {
		t.Error("超出 64B 上限应失败")
	}
}
