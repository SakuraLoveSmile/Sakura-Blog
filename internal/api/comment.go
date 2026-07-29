package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"blog/internal/comment"
	"blog/internal/store"
)

// commentJSON 评论 API 响应模型。
type commentJSON struct {
	ID        int64  `json:"id"`
	PageKey   string `json:"page_key"`
	Nick      string `json:"nick"`
	Email     string `json:"email"`
	Content   string `json:"content"`
	ParentID  int64  `json:"parent_id"`
	RootID    int64  `json:"root_id"`
	Status    string `json:"status"`
	IsAdmin   bool   `json:"is_admin"`
	Avatar    string `json:"avatar"`
	ReplyNick string `json:"reply_nick,omitempty"`
	Date      string `json:"date"`
}

func toCommentJSON(cm *store.Comment, gravatarMirror string) *commentJSON {
	return &commentJSON{
		ID:       cm.ID,
		PageKey:  cm.PageKey,
		Nick:     cm.Nick,
		Email:    cm.Email,
		Content:  truncateText(stripHTML(cm.ContentHTML), 200),
		ParentID: cm.ParentID,
		RootID:   cm.RootID,
		Status:   cm.Status,
		IsAdmin:  cm.IsAdmin,
		Avatar:   comment.AvatarURL(gravatarMirror, cm.Email),
		Date:     cm.CreatedAt.Format("2006-01-02 15:04"),
	}
}

// listComments GET /api/v1/comments：评论列表（status 筛选 + 分页）。
func (h *Handler) listComments(c *gin.Context) {
	filter := c.DefaultQuery("status", "")
	switch filter {
	case store.CommentPending, store.CommentApproved:
		// valid
	default:
		filter = ""
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}

	total, err := h.store.CountComments(filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	comments, err := h.store.ListComments(filter, page, size)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	mirror := h.site.Comments.GravatarMirror
	items := make([]*commentJSON, 0, len(comments))
	for _, cm := range comments {
		row := toCommentJSON(cm, mirror)
		// 被回复者昵称
		if cm.ParentID != 0 {
			if parent, err := h.store.GetComment(cm.ParentID); err == nil && parent != nil {
				row.ReplyNick = parent.Nick
			}
		}
		items = append(items, row)
	}

	c.JSON(http.StatusOK, gin.H{
		"items": items,
		"total": total,
		"page":  page,
		"size":  size,
	})
}

// approveComment PUT /api/v1/comments/:id/approve：审核通过。
func (h *Handler) approveComment(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的评论 ID"})
		return
	}
	if err := h.store.UpdateCommentStatus(id, store.CommentApproved); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// deleteComment DELETE /api/v1/comments/:id：删除评论（级联）。
func (h *Handler) deleteComment(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的评论 ID"})
		return
	}
	if err := h.store.DeleteCommentCascade(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// maxBatchCommentIDs 单次批量操作的 ids 上限。
const maxBatchCommentIDs = 100

// batchComments POST /api/v1/comments/batch：批量通过/删除。
// 事务内执行（store 批量方法内部保证原子性），返回 {succeeded, failed}；
// 混入不存在的 id 时其余照常成功，failed 计入未命中数。
// 注意：批量通过不触发邮件通知（通知仅在单条回复路径 replyComment）。
func (h *Handler) batchComments(c *gin.Context) {
	var req struct {
		IDs    []int64 `json:"ids"`
		Action string  `json:"action"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}
	if len(req.IDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ids 必填"})
		return
	}
	if len(req.IDs) > maxBatchCommentIDs {
		c.JSON(http.StatusBadRequest, gin.H{"error": "单次最多操作 100 条"})
		return
	}

	var succeeded int
	var err error
	switch req.Action {
	case "approve":
		succeeded, err = h.store.ApproveComments(req.IDs)
	case "delete":
		succeeded, err = h.store.DeleteComments(req.IDs)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "action 仅支持 approve / delete"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"succeeded": succeeded, "failed": len(req.IDs) - succeeded})
}

// replyComment POST /api/v1/comments/reply：管理员回复。
func (h *Handler) replyComment(c *gin.Context) {
	var req struct {
		ParentID int64  `json:"parent_id"`
		Content  string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}
	if req.ParentID == 0 || strings.TrimSpace(req.Content) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "parent_id 与 content 必填"})
		return
	}

	parent, err := h.store.GetComment(req.ParentID)
	if err != nil || parent == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "被回复的评论不存在"})
		return
	}

	// 两层嵌套：回复子评论时挂到同一根下
	rootID := parent.ID
	if parent.RootID != 0 {
		rootID = parent.RootID
	}

	html, err := comment.Render(req.Content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "渲染失败: " + err.Error()})
		return
	}

	nick := h.site.Comments.AdminBadge
	if nick == "" {
		nick = "博主"
	}
	cm := &store.Comment{
		PageKey:     parent.PageKey,
		Nick:        nick,
		Email:       h.site.Comments.AdminEmail,
		ContentMD:   req.Content,
		ContentHTML: html,
		ParentID:    req.ParentID,
		RootID:      rootID,
		Status:      store.CommentApproved,
		IsAdmin:     true,
		IP:          c.ClientIP(),
		UA:          c.Request.UserAgent(),
	}
	if _, err := h.store.CreateComment(cm); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "回复失败: " + err.Error()})
		return
	}

	// 触发被回复者通知
	if h.comments != nil {
		h.comments.NotifyReply(cm, parent)
	}

	c.JSON(http.StatusCreated, toCommentJSON(cm, h.site.Comments.GravatarMirror))
}
