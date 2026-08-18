package handler_test

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/li-yongqvan/ai-forum/backend/internal/testutil"
)

// doUpload 构造 multipart POST /api/v1/uploads（字段 "file"）。
func doUpload(t *testing.T, r *gin.Engine, filename string, data []byte, token string) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("创建 multipart 字段失败: %v", err)
	}
	if _, err := fw.Write(data); err != nil {
		t.Fatalf("写入 multipart 失败: %v", err)
	}
	mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/uploads", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// 上传 → 返回 URL → 文件落盘 → 开发态静态可 GET（#12 主闭环）。
func TestUploadFlow(t *testing.T) {
	gdb := testutil.SetupPG(t)
	dir := t.TempDir()
	r := newEngineWithUploads(t, gdb, dir, 2048)
	alice := registerUser(t, r, gdb, "alice", "alice@x.edu", "CODE-UP1")

	// 未登录 → 401
	if w := doUpload(t, r, "a.png", tinyPNG(t), ""); w.Code != http.StatusUnauthorized {
		t.Errorf("游客上传 = %d, want 401", w.Code)
	}

	// 正常上传 → 201 + url 形如 http://<host>/uploads/<uuid>.png
	w := doUpload(t, r, "photo.png", tinyPNG(t), alice.Token)
	if w.Code != http.StatusCreated {
		t.Fatalf("上传 = %d, body=%s", w.Code, w.Body.String())
	}
	var res struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(res.URL, "http://") || !strings.Contains(res.URL, "/uploads/") || !strings.HasSuffix(res.URL, ".png") {
		t.Errorf("url = %q, want 形如 http://<host>/uploads/<uuid>.png", res.URL)
	}

	// 文件真实落盘且内容一致
	name := filepath.Base(res.URL)
	got, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("落盘文件缺失: %v", err)
	}
	if !bytes.Equal(got, tinyPNG(t)) {
		t.Error("落盘内容与上传不一致")
	}

	// 开发态 Go 托管 /uploads：GET 返回原图（生产由 nginx 接管，见 #8/deployment.md）
	wg := doJSON(t, r, http.MethodGet, "/uploads/"+name, nil, "")
	if wg.Code != http.StatusOK {
		t.Fatalf("GET 静态图片 = %d, want 200", wg.Code)
	}
	if !bytes.Equal(wg.Body.Bytes(), tinyPNG(t)) {
		t.Error("静态 GET 内容与落盘不一致")
	}
}

// 校验矩阵（#12 D2/D3）：类型白名单 + MIME sniff + 大小上限 + 缺字段。
func TestUploadValidation(t *testing.T) {
	gdb := testutil.SetupPG(t)
	r := newEngineWithUploads(t, gdb, t.TempDir(), 2048)
	alice := registerUser(t, r, gdb, "alice", "alice@x.edu", "CODE-UP2")

	// 非白名单扩展名 → 400
	if w := doUpload(t, r, "a.txt", []byte("hello"), alice.Token); w.Code != http.StatusBadRequest {
		t.Errorf(".txt = %d, want 400", w.Code)
	}
	// 伪扩展名：.png 内容非图片 → 400（MIME sniff 拦截）
	if w := doUpload(t, r, "fake.png", []byte("<?php echo 1; ?>"), alice.Token); w.Code != http.StatusBadRequest {
		t.Errorf("伪 .png = %d, want 400", w.Code)
	}
	// 超过大小上限（maxBytes=2048，传 3KB）→ 413
	big := bytes.Repeat([]byte{0x89, 0x50, 0x4e, 0x47}, 768) // 3072B
	if w := doUpload(t, r, "big.png", big, alice.Token); w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("超大文件 = %d, want 413", w.Code)
	}
	// 缺 file 字段 → 400
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/uploads", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+alice.Token)
	wr := httptest.NewRecorder()
	r.ServeHTTP(wr, req)
	if wr.Code != http.StatusBadRequest {
		t.Errorf("缺 file 字段 = %d, want 400", wr.Code)
	}
}

// tinyPNG 生成 1×1 有效 PNG（真实编码，http.DetectContentType 识别为 image/png）。
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
