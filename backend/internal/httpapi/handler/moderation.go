package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/li-yongqvan/ai-forum/backend/internal/httpapi/middleware"
	"github.com/li-yongqvan/ai-forum/backend/moderation"
	"github.com/li-yongqvan/ai-forum/backend/user"
)

// ModerationHandler 暴露治理域端点（#33：举报创建需登录；队列/处理需 mod 组；#34 ban/unban 聚合 user 域）。
type ModerationHandler struct {
	svc   moderation.Service
	users user.Service
}

// NewModerationHandler 装配 ModerationHandler（users 供 #34 封禁/解封跨包聚合）。
func NewModerationHandler(svc moderation.Service, users user.Service) *ModerationHandler {
	return &ModerationHandler{svc: svc, users: users}
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
	case errors.Is(err, moderation.ErrRateLimited):
		respondError(c, http.StatusTooManyRequests, "举报过于频繁，请稍后再试")
	case errors.Is(err, moderation.ErrSelfBan), errors.Is(err, moderation.ErrCannotBanAdmin):
		respondError(c, http.StatusBadRequest, err.Error())
	case errors.Is(err, moderation.ErrAlreadyBanned):
		respondError(c, http.StatusConflict, "该用户已被封禁")
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

// ListActions GET /api/v1/moderation/actions?target_type=&target_id=&action=&moderator_id=&page=&page_size=（mod 组，#60）
func (h *ModerationHandler) ListActions(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	targetID, _ := strconv.ParseInt(c.Query("target_id"), 10, 64)
	moderatorID, _ := strconv.ParseInt(c.Query("moderator_id"), 10, 64)
	views, err := h.svc.ListActions(c.Request.Context(), moderation.ListActionsQuery{
		TargetType:  c.Query("target_type"),
		TargetID:    targetID,
		Action:      c.Query("action"),
		ModeratorID: moderatorID,
		Limit:       pageSize,
		Offset:      (page - 1) * pageSize,
	})
	if err != nil {
		respondModerationError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": views, "page": page, "page_size": pageSize})
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

// respondAdminError 将治理操作的 user 域哨兵错误映射为 HTTP 状态（#34 ban/unban）。
func respondAdminError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, user.ErrNotFound):
		respondError(c, http.StatusNotFound, "用户不存在")
	case errors.Is(err, user.ErrSelfBan):
		respondError(c, http.StatusBadRequest, "不能封禁自己")
	case errors.Is(err, user.ErrCannotBanAdmin):
		respondError(c, http.StatusBadRequest, "不能封禁管理员")
	case errors.Is(err, user.ErrAlreadyBanned):
		respondError(c, http.StatusConflict, "该用户已被封禁")
	case errors.Is(err, user.ErrNotBanned):
		respondError(c, http.StatusConflict, "该用户未被封禁")
	default:
		respondError(c, http.StatusInternalServerError, "服务器内部错误")
	}
}

// adminBanReq 封禁/解封请求体（原因必填，≤500；与 RecordAction 校验同源，评审 Q1 条件 3）。
type adminBanReq struct {
	Reason string `json:"reason" binding:"required"`
}

// BanUser POST /api/v1/moderation/users/:id/ban（admin 组）。
// 状态先改（user.Ban 幂等条件更新）→ 审计后写（moderation.RecordAction）；审计失败 → 500 + status_changed:true（§6.1/F6）。
func (h *ModerationHandler) BanUser(c *gin.Context) {
	h.banUser(c, false)
}

// UnbanUser POST /api/v1/moderation/users/:id/unban（admin 组）。
func (h *ModerationHandler) UnbanUser(c *gin.Context) {
	h.banUser(c, true)
}

func (h *ModerationHandler) banUser(c *gin.Context, unban bool) {
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
	var req adminBanReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "请填写封禁原因")
		return
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" || len([]rune(reason)) > 500 { // 与 RecordAction 同源（trim + 非空 + ≤500 runes）
		respondError(c, http.StatusBadRequest, "原因必填且不能超过 500 字")
		return
	}
	ctx := c.Request.Context()

	// 1) 状态先改（user 域，条件更新幂等：#34 §6.1）
	var stateErr error
	if unban {
		stateErr = h.users.Unban(ctx, user.UnbanCmd{OperatorID: claims.UserID, TargetID: id})
	} else {
		stateErr = h.users.Ban(ctx, user.BanCmd{OperatorID: claims.UserID, TargetID: id})
	}
	if stateErr != nil {
		respondAdminError(c, stateErr)
		return
	}

	// 2) 审计后写（moderation 域）；失败不吞错（F6：500 + status_changed:true，供前端区分）
	action := moderation.ActionBan
	if unban {
		action = moderation.ActionUnban
	}
	if err := h.svc.RecordAction(ctx, moderation.RecordActionCmd{
		ModeratorID: claims.UserID,
		Action:      action,
		TargetID:    id,
		Reason:      reason,
	}); err != nil {
		slog.Error("治理审计写入失败", "operator", claims.UserID, "action", action, "target", id, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "账号状态已更新，但审计记录失败，请联系管理员", "status_changed": true})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}
