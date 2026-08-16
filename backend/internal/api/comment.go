package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"blog/internal/textutil"
	"blog/internal/comment"
	"blog/internal/notify"
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
		Content:  textutil.Truncate(textutil.StripHTML(cm.ContentHTML), 200),
		ParentID: cm.ParentID,
		RootID:   cm.RootID,
		Status:   cm.Status,
		IsAdmin:  cm.IsAdmin,
		Avatar:   comment.AvatarURL(gravatarMirror, cm.Email),
		Date:     cm.CreatedAt.Format("2006-01-02 15:04"),
	}
}

// listComments GET /api/v1/comments：评论列表（status 筛选 + keyword 搜索 + user_id 过滤 + 分页）。
// 响应附带 counts（各状态真实计数），供管理页 tab 徽标展示。
func (h *Handler) listComments(c *gin.Context) {
	filter := c.DefaultQuery("status", "")
	switch filter {
	case store.CommentPending, store.CommentApproved, store.CommentRejected, store.CommentSpam:
		// valid
	default:
		filter = ""
	}
	// Spec 62：keyword 后端 LIKE 搜索（nick/email/content），替代 console 本地过滤当前页
	keyword := strings.TrimSpace(c.Query("keyword"))
	// Spec 71：可选 user_id 过滤（评论用户详情页按用户查看评论）
	userID, _ := strconv.ParseInt(c.DefaultQuery("user_id", "0"), 10, 64)
	if userID < 0 {
		userID = 0
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}

	total, err := h.store.CountCommentsSearch(filter, keyword, userID)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	comments, err := h.store.ListCommentsSearch(filter, keyword, userID, page, size)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	mirror := h.site.Load().Comments.GravatarMirror
	// Spec 62：父评论昵称从逐条 GetComment 收敛为单条 IN 批量查询（GetCommentsByIDs）。
	parentIDs := make([]int64, 0, len(comments))
	for _, cm := range comments {
		if cm.ParentID != 0 {
			parentIDs = append(parentIDs, cm.ParentID)
		}
	}
	parents, err := h.store.GetCommentsByIDs(parentIDs)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	items := make([]*commentJSON, 0, len(comments))
	for _, cm := range comments {
		row := toCommentJSON(cm, mirror)
		// 被回复者昵称
		if p, ok := parents[cm.ParentID]; ok {
			row.ReplyNick = p.Nick
		}
		items = append(items, row)
	}

	counts, err := h.store.CountCommentStatuses()
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"items":  items,
		"total":  total,
		"page":   page,
		"size":   size,
		"counts": counts,
	})
}

// approveComment PUT /api/v1/comments/:id/approve：审核通过。
// 通过后触发 comment_approved 站内通知（异步，失败仅记日志）。
func (h *Handler) approveComment(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "无效的评论 ID")
		return
	}
	cm, err := h.store.GetComment(id)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if cm == nil {
		Fail(c, http.StatusNotFound, "评论不存在")
		return
	}
	if err := h.store.UpdateCommentStatus(id, store.CommentApproved); err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	// 站内通知（Wave 3b）：评论审核通过
	if h.notify != nil {
		content := cm.ContentMD
		if r := []rune(content); len(r) > 200 {
			content = string(r[:200]) + "…"
		}
		h.notify.Notify(notify.Event{
			Type:    "comment_approved",
			Title:   fmt.Sprintf("评论已通过：%s", cm.Nick),
			Content: content,
			Link:    cm.PageKey,
			Nick:    cm.Nick,
			Email:   cm.Email,
			IP:      cm.IP,
			PageKey: cm.PageKey,
			Status:  store.CommentApproved,
		})
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// statusComment PUT /api/v1/comments/:id/status：修改评论状态（拒绝 / 垃圾 / 恢复等）。
// 请求体 {"status": "approved|pending|rejected|spam"}；改为 approved 时同样触发通过通知。
func (h *Handler) statusComment(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "无效的评论 ID")
		return
	}
	var req struct {
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "请求格式错误")
		return
	}
	switch req.Status {
	case store.CommentPending, store.CommentApproved, store.CommentRejected, store.CommentSpam:
		// valid
	default:
		Fail(c, http.StatusBadRequest, "status 仅支持 pending / approved / rejected / spam")
		return
	}

	cm, err := h.store.GetComment(id)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if cm == nil {
		Fail(c, http.StatusNotFound, "评论不存在")
		return
	}
	if err := h.store.UpdateCommentStatus(id, req.Status); err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	// 恢复为已通过时补发通过通知，与 approve 端点行为一致
	if req.Status == store.CommentApproved && h.notify != nil {
		content := cm.ContentMD
		if r := []rune(content); len(r) > 200 {
			content = string(r[:200]) + "…"
		}
		h.notify.Notify(notify.Event{
			Type:    "comment_approved",
			Title:   fmt.Sprintf("评论已通过：%s", cm.Nick),
			Content: content,
			Link:    cm.PageKey,
			Nick:    cm.Nick,
			Email:   cm.Email,
			IP:      cm.IP,
			PageKey: cm.PageKey,
			Status:  store.CommentApproved,
		})
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// deleteComment DELETE /api/v1/comments/:id：删除评论（级联）。
func (h *Handler) deleteComment(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "无效的评论 ID")
		return
	}
	if err := h.store.DeleteCommentCascade(id); err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// maxBatchCommentIDs 单次批量操作的 ids 上限。
const maxBatchCommentIDs = 100

// batchComments POST /api/v1/comments/batch：批量通过/拒绝/标垃圾/删除。
// 事务内执行（store 批量方法内部保证原子性），返回 {succeeded, failed}；
// 混入不存在的 id 时其余照常成功，failed 计入未命中数。
// 注意：批量操作不触发站内/邮件通知（通知仅在单条通过 approve / status 与回复路径）。
func (h *Handler) batchComments(c *gin.Context) {
	var req struct {
		IDs    []int64 `json:"ids"`
		Action string  `json:"action"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "请求格式错误")
		return
	}
	if len(req.IDs) == 0 {
		Fail(c, http.StatusBadRequest, "ids 必填")
		return
	}
	if len(req.IDs) > maxBatchCommentIDs {
		Fail(c, http.StatusBadRequest, "单次最多操作 100 条")
		return
	}

	var succeeded int
	var err error
	switch req.Action {
	case "approve":
		succeeded, err = h.store.SetCommentsStatus(req.IDs, store.CommentApproved)
	case "reject":
		succeeded, err = h.store.SetCommentsStatus(req.IDs, store.CommentRejected)
	case "spam":
		succeeded, err = h.store.SetCommentsStatus(req.IDs, store.CommentSpam)
	case "delete":
		succeeded, err = h.store.DeleteComments(req.IDs)
	default:
		Fail(c, http.StatusBadRequest, "action 仅支持 approve / reject / spam / delete")
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
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
		Fail(c, http.StatusBadRequest, "请求格式错误")
		return
	}
	if req.ParentID == 0 || strings.TrimSpace(req.Content) == "" {
		Fail(c, http.StatusBadRequest, "parent_id 与 content 必填")
		return
	}

	parent, err := h.store.GetComment(req.ParentID)
	if err != nil || parent == nil {
		Fail(c, http.StatusBadRequest, "被回复的评论不存在")
		return
	}

	// 两层嵌套：回复子评论时挂到同一根下
	rootID := parent.ID
	if parent.RootID != 0 {
		rootID = parent.RootID
	}

	html, err := comment.Render(req.Content)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "渲染失败: "+err.Error())
		return
	}

	nick := h.site.Load().Comments.AdminBadge
	if nick == "" {
		nick = "博主"
	}
	cm := &store.Comment{
		PageKey:     parent.PageKey,
		Nick:        nick,
		Email:       h.site.Load().Comments.AdminEmail,
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
		Fail(c, http.StatusInternalServerError, "回复失败: "+err.Error())
		return
	}

	// 触发被回复者通知
	if h.comments != nil {
		h.comments.NotifyReply(cm, parent)
	}

	c.JSON(http.StatusCreated, toCommentJSON(cm, h.site.Load().Comments.GravatarMirror))
}
