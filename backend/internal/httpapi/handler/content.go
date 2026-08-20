package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/li-yongqvan/ai-forum/backend/content"
	"github.com/li-yongqvan/ai-forum/backend/internal/httpapi/middleware"
)

// ContentHandler 暴露内容域端点（IA v2 §4 页面 → 实体映射）。
type ContentHandler struct {
	svc content.Service
}

// NewContentHandler 装配 ContentHandler。
func NewContentHandler(svc content.Service) *ContentHandler {
	return &ContentHandler{svc: svc}
}

// respondContentError 将内容域哨兵错误映射为 HTTP 状态（#9 §5.0 后端独立鉴权语义）。
func respondContentError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, content.ErrAuthRequired):
		respondError(c, http.StatusUnauthorized, "需要登录")
	case errors.Is(err, content.ErrForbidden):
		respondError(c, http.StatusForbidden, "权限不足")
	case errors.Is(err, content.ErrPostNotFound), errors.Is(err, content.ErrCommentNotFound),
		errors.Is(err, content.ErrBoardNotFound), errors.Is(err, content.ErrTopicNotFound):
		respondError(c, http.StatusNotFound, "内容不存在")
	case errors.Is(err, content.ErrAlreadyLiked), errors.Is(err, content.ErrAlreadyFavorited),
		errors.Is(err, content.ErrAlreadyFollowed):
		respondError(c, http.StatusConflict, "重复操作")
	case errors.Is(err, content.ErrInvalidTargetType), errors.Is(err, content.ErrInvalidFeedTab),
		errors.Is(err, content.ErrContentEmpty), errors.Is(err, content.ErrParentNotInPost),
		errors.Is(err, content.ErrTopicNotInBoard):
		respondError(c, http.StatusBadRequest, "参数不合法")
	default:
		respondError(c, http.StatusInternalServerError, "服务器内部错误")
	}
}

// viewerID 返回当前请求者 id；游客为 0。
func viewerID(c *gin.Context) int64 {
	claims, ok := middleware.Identity(c)
	if !ok {
		return 0
	}
	return claims.UserID
}

func pathID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return 0, false
	}
	return id, true
}

// ---- 板块 / 话题（公开） ----

// ListBoards GET /api/v1/boards
func (h *ContentHandler) ListBoards(c *gin.Context) {
	views, err := h.svc.ListBoards(c.Request.Context(), viewerID(c))
	if err != nil {
		respondContentError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": views})
}

// ListTopics GET /api/v1/topics?board_id=
func (h *ContentHandler) ListTopics(c *gin.Context) {
	var boardID *int64
	if v := c.Query("board_id"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			respondError(c, http.StatusBadRequest, "参数不合法")
			return
		}
		boardID = &id
	}
	views, err := h.svc.ListTopics(c.Request.Context(), viewerID(c), boardID)
	if err != nil {
		respondContentError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": views})
}

// ---- 帖子 ----

// ListPosts GET /api/v1/posts（信息流/板块/话题/作者过滤）
func (h *ContentHandler) ListPosts(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	q := content.ListFeedQuery{
		Tab:      c.DefaultQuery("tab", "all"),
		ViewerID: viewerID(c),
		Page:     page,
		PageSize: pageSize,
	}
	if v := c.Query("author_id"); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil {
			q.AuthorID = &id
		}
	}
	if v := c.Query("board_id"); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil {
			q.BoardID = &id
		}
	}
	if v := c.Query("topic_id"); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil {
			q.TopicID = &id
		}
	}
	views, err := h.svc.ListFeed(c.Request.Context(), q)
	if err != nil {
		respondContentError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": views, "page": page, "page_size": pageSize})
}

// GetPost GET /api/v1/posts/:id
func (h *ContentHandler) GetPost(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		respondError(c, http.StatusBadRequest, "参数不合法")
		return
	}
	view, err := h.svc.GetPost(c.Request.Context(), content.GetPostQuery{PostID: id, ViewerID: viewerID(c)})
	if err != nil {
		respondContentError(c, err)
		return
	}
	c.JSON(http.StatusOK, view)
}

type createPostReq struct {
	BoardID int64  `json:"board_id" binding:"required"`
	TopicID *int64 `json:"topic_id"`
	Title   string `json:"title" binding:"required"`
	Content string `json:"content" binding:"required"`
}

// CreatePost POST /api/v1/posts（发帖，#9 §5.0：操作需登录）
func (h *ContentHandler) CreatePost(c *gin.Context) {
	claims, ok := middleware.Identity(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "需要登录")
		return
	}
	var req createPostReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数不合法")
		return
	}
	view, err := h.svc.CreatePost(c.Request.Context(), content.CreatePostCmd{
		AuthorID: claims.UserID,
		BoardID:  req.BoardID,
		TopicID:  req.TopicID,
		Title:    req.Title,
		Content:  req.Content,
	})
	if err != nil {
		respondContentError(c, err)
		return
	}
	c.JSON(http.StatusCreated, view)
}

// DeletePost DELETE /api/v1/posts/:id
func (h *ContentHandler) DeletePost(c *gin.Context) {
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
	if err := h.svc.DeletePost(c.Request.Context(), content.DeletePostCmd{
		OperatorID: claims.UserID, OperatorRole: claims.Role, PostID: id,
	}); err != nil {
		respondContentError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

// PinPost POST /api/v1/posts/:id/pin（路由已 RequireRole(moderator, admin)，服务端再校验）
func (h *ContentHandler) PinPost(c *gin.Context) {
	h.togglePost(c, "pin")
}

// FeaturePost POST /api/v1/posts/:id/feature
func (h *ContentHandler) FeaturePost(c *gin.Context) {
	h.togglePost(c, "feature")
}

func (h *ContentHandler) togglePost(c *gin.Context, kind string) {
	claims, _ := middleware.Identity(c)
	id, ok := pathID(c)
	if !ok {
		respondError(c, http.StatusBadRequest, "参数不合法")
		return
	}
	cmd := content.ToggleCmd{OperatorID: claims.UserID, OperatorRole: claims.Role, PostID: id}
	var err error
	if kind == "pin" {
		err = h.svc.PinPost(c.Request.Context(), cmd)
	} else {
		err = h.svc.FeaturePost(c.Request.Context(), cmd)
	}
	if err != nil {
		respondContentError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

// ---- 评论 ----

// GetComments GET /api/v1/posts/:id/comments（评论树，#5 D3）
func (h *ContentHandler) GetComments(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		respondError(c, http.StatusBadRequest, "参数不合法")
		return
	}
	tree, err := h.svc.GetCommentTree(c.Request.Context(), id)
	if err != nil {
		respondContentError(c, err)
		return
	}
	c.JSON(http.StatusOK, tree)
}

type createCommentReq struct {
	PostID   int64  `json:"post_id" binding:"required"`
	ParentID *int64 `json:"parent_id"`
	Content  string `json:"content" binding:"required"`
}

// CreateComment POST /api/v1/comments
func (h *ContentHandler) CreateComment(c *gin.Context) {
	claims, ok := middleware.Identity(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "需要登录")
		return
	}
	var req createCommentReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数不合法")
		return
	}
	view, err := h.svc.CreateComment(c.Request.Context(), content.CreateCommentCmd{
		PostID:   req.PostID,
		AuthorID: claims.UserID,
		ParentID: req.ParentID,
		Content:  req.Content,
	})
	if err != nil {
		respondContentError(c, err)
		return
	}
	c.JSON(http.StatusCreated, view)
}

// DeleteComment DELETE /api/v1/comments/:id
func (h *ContentHandler) DeleteComment(c *gin.Context) {
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
	if err := h.svc.DeleteComment(c.Request.Context(), content.DeleteCommentCmd{
		OperatorID: claims.UserID, OperatorRole: claims.Role, CommentID: id,
	}); err != nil {
		respondContentError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

// ---- 点赞 / 收藏 ----

type likeReq struct {
	TargetType string `json:"target_type" binding:"required"`
	TargetID   int64  `json:"target_id" binding:"required"`
}

// Like POST /api/v1/likes
func (h *ContentHandler) Like(c *gin.Context) {
	claims, ok := middleware.Identity(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "需要登录")
		return
	}
	var req likeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数不合法")
		return
	}
	if err := h.svc.Like(c.Request.Context(), content.LikeCmd{UserID: claims.UserID, TargetType: req.TargetType, TargetID: req.TargetID}); err != nil {
		respondContentError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

// Unlike DELETE /api/v1/likes?target_type=&target_id=（取消用 query param，避免 DELETE 带体）
func (h *ContentHandler) Unlike(c *gin.Context) {
	claims, ok := middleware.Identity(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "需要登录")
		return
	}
	targetID, err := strconv.ParseInt(c.Query("target_id"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "参数不合法")
		return
	}
	if err := h.svc.Unlike(c.Request.Context(), content.LikeCmd{
		UserID: claims.UserID, TargetType: c.Query("target_type"), TargetID: targetID,
	}); err != nil {
		respondContentError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

type favoriteReq struct {
	PostID int64 `json:"post_id" binding:"required"`
}

// Favorite POST /api/v1/favorites
func (h *ContentHandler) Favorite(c *gin.Context) {
	claims, ok := middleware.Identity(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "需要登录")
		return
	}
	var req favoriteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "参数不合法")
		return
	}
	if err := h.svc.Favorite(c.Request.Context(), content.FavoriteCmd{UserID: claims.UserID, PostID: req.PostID}); err != nil {
		respondContentError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

// Unfavorite DELETE /api/v1/favorites?post_id=
func (h *ContentHandler) Unfavorite(c *gin.Context) {
	claims, ok := middleware.Identity(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "需要登录")
		return
	}
	postID, err := strconv.ParseInt(c.Query("post_id"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "参数不合法")
		return
	}
	if err := h.svc.Unfavorite(c.Request.Context(), content.FavoriteCmd{UserID: claims.UserID, PostID: postID}); err != nil {
		respondContentError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

// ListFavorites GET /api/v1/favorites（需登录；我收藏的帖子，按收藏时间倒序，#23）
func (h *ContentHandler) ListFavorites(c *gin.Context) {
	claims, ok := middleware.Identity(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "需要登录")
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	views, err := h.svc.ListFavorites(c.Request.Context(), content.ListFavoritesQuery{
		ViewerID: claims.UserID, Page: page, PageSize: pageSize,
	})
	if err != nil {
		respondContentError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": views, "page": page, "page_size": pageSize})
}
