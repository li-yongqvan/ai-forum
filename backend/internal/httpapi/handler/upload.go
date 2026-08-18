package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/li-yongqvan/ai-forum/backend/upload"
)

// UploadHandler 暴露图片上传端点（#12：MVP 本地上传）。
type UploadHandler struct {
	svc upload.Service
}

// NewUploadHandler 装配 UploadHandler。
func NewUploadHandler(svc upload.Service) *UploadHandler {
	return &UploadHandler{svc: svc}
}

// multipartSlack multipart 包装开销余量：MaxBytesReader 限制的是整个请求体，
// 文件本体仍以 f.Size 精确校验（#12 D2：默认 5MB，config 可配）。
const multipartSlack = 1 << 20

// Upload POST /api/v1/uploads（需登录，挂在 authed 组）：multipart 字段 "file" →
// 校验落盘 → 返回可访问的绝对图片 URL。
func (h *UploadHandler) Upload(c *gin.Context) {
	max := h.svc.MaxBytes()
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, max+multipartSlack)

	f, err := c.FormFile("file")
	if err != nil {
		respondUploadError(c, err)
		return
	}
	if f.Size > max {
		respondError(c, http.StatusRequestEntityTooLarge, "图片过大，请压缩后重试")
		return
	}
	src, err := f.Open()
	if err != nil {
		respondError(c, http.StatusInternalServerError, "服务器内部错误")
		return
	}
	defer src.Close()

	rel, err := h.svc.Store(f.Filename, src)
	if err != nil {
		switch {
		case errors.Is(err, upload.ErrBadType):
			respondError(c, http.StatusBadRequest, "仅支持 jpg/png/gif/webp 图片")
		case errors.Is(err, upload.ErrTooLarge):
			respondError(c, http.StatusRequestEntityTooLarge, "图片过大，请压缩后重试")
		case errors.Is(err, upload.ErrEmpty):
			respondError(c, http.StatusBadRequest, "图片内容为空")
		default:
			respondError(c, http.StatusInternalServerError, "服务器内部错误")
		}
		return
	}
	c.JSON(http.StatusCreated, gin.H{"url": publicUploadURL(c, rel)})
}

// respondUploadError 处理 multipart 解析错误：请求体超限 → 413，其余（缺字段/非 multipart）→ 400。
func respondUploadError(c *gin.Context, err error) {
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) || strings.Contains(err.Error(), "request body too large") {
		respondError(c, http.StatusRequestEntityTooLarge, "图片过大，请压缩后重试")
		return
	}
	respondError(c, http.StatusBadRequest, "请选择图片文件")
}

// publicUploadURL 把相对路径拼成当前请求可达的绝对 URL（#12：md 渲染器只渲染 http(s)
// 绝对地址；开发经 Vite proxy、生产经 nginx 同源托管 /uploads，Host 由反代透传）。
func publicUploadURL(c *gin.Context, rel string) string {
	scheme := "http"
	if fwd := c.GetHeader("X-Forwarded-Proto"); fwd == "https" {
		scheme = "https"
	}
	return scheme + "://" + c.Request.Host + rel
}
