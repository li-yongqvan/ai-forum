package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/li-yongqvan/ai-forum/backend/internal/httpapi/middleware"
	"github.com/li-yongqvan/ai-forum/backend/moderation"
)

// ModerationHandler 暴露治理域端点（#33：举报创建需登录；队列/处理需 mod 组）。
type ModerationHandler struct {
	svc moderation.Service
}

// NewModerationHandler 装配 ModerationHandler。
func NewModerationHandler(svc moderation.Service) *ModerationHandler {
	return &ModerationHandler{svc: svc}
}

// respondModerationError 将治理域哨兵错误映射为 HTTP 状态。
func respondModerationError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, moderation.ErrNotFound):
		respondError(c, http.StatusNotFound, "举报或目标不存在")
	case errors.Is(err, moderation.ErrDuplicatePending):
		respondError(c, http.StatusConflict, "你已举报过该内容，请等待处理")
	case errors.Is(err, moderation.ErrInvalidAction):
		respondError(c, http.StatusBadRequest, "操作不合法")
	default:
		respondError(c, http.StatusInternalServerError, "服务器内部错误")
	}
}

// CreateReport POST /api/v1/reports（登录墙；F7 存在性/自举报校验在服务层）
func (h *ModerationHandler) CreateReport(c *gin.Context) {
	claims, ok := middleware.Identity(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "需要登录")
		return
	}
	var req struct {
		TargetType string `json:"target_type" binding:"required"`
		TargetID   int64  `json:"target_id" binding:"required"`
		Reason     string `json:"reason" binding:"required"`
		Note       string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数不合法")
		return
	}
	id, err := h.svc.CreateReport(c.Request.Context(), moderation.CreateReportCmd{
		ReporterID: claims.UserID,
		TargetType: req.TargetType,
		TargetID:   req.TargetID,
		Reason:     req.Reason,
		Note:       req.Note,
	})
	if err != nil {
		respondModerationError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": id, "message": "ok"})
}

// ListReports GET /api/v1/moderation/reports?status=&page=&page_size=（mod 组）
func (h *ModerationHandler) ListReports(c *gin.Context) {
	status := c.DefaultQuery("status", "pending")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100 // O5：服务端钳制，防滥用打全表
	}
	views, err := h.svc.ListReports(c.Request.Context(), moderation.ListReportsQuery{
		Status: status,
		Limit:  pageSize,
		Offset: (page - 1) * pageSize,
	})
	if err != nil {
		respondModerationError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": views, "page": page, "page_size": pageSize})
}

// CountReports GET /api/v1/moderation/reports/count?status=（Me.vue 待处理徽章）
func (h *ModerationHandler) CountReports(c *gin.Context) {
	n, err := h.svc.CountReports(c.Request.Context(), c.DefaultQuery("status", "pending"))
	if err != nil {
		respondModerationError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"count": n})
}

// HandleReport POST /api/v1/moderation/reports/:id/handle（mod 组；服务端二次鉴权 D6/F3）
func (h *ModerationHandler) HandleReport(c *gin.Context) {
	claims, ok := middleware.Identity(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "需要登录")
		return
	}
	id, ok := pathID(c)
	if !ok {
		respondError(c, http.StatusBadRequest, "参数不合法")
		return
	}
	var req struct {
		Action string `json:"action" binding:"required"`
		Note   string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数不合法")
		return
	}
	if len([]rune(req.Note)) > 500 { // F5：moderation_actions.reason VARCHAR(500)
		respondError(c, http.StatusBadRequest, "备注不能超过 500 字")
		return
	}
	if err := h.svc.HandleReport(c.Request.Context(), moderation.HandleReportCmd{
		ReportID:     id,
		HandlerID:    claims.UserID,
		OperatorRole: claims.Role,
		Action:       req.Action,
		Note:         req.Note,
	}); err != nil {
		respondModerationError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}
