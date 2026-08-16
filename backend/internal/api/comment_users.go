package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"blog/internal/store"
)

// 评论用户管理 API（Spec 71）：管理注册读者（role=reader）——
// 列表（含评论聚合）/ 封禁解封 / 删除 / 用户级评论批量处置。全部 admin-only（sys 组）。

// commentUserJSON 评论用户列表响应项（用户字段 + 评论聚合）。
type commentUserJSON struct {
	ID            int64  `json:"id"`
	Username      string `json:"username"`
	DisplayName   string `json:"display_name"`
	Email         string `json:"email"`
	Avatar        string `json:"avatar"`
	Role          string `json:"role"`
	Status        string `json:"status"`
	CreatedAt     string `json:"created_at"`
	CommentCount  int    `json:"comment_count"`
	LastCommentAt string `json:"last_comment_at"` // 空串=从未评论
}

func toCommentUserJSON(cu *store.CommentUser) *commentUserJSON {
	last := ""
	if !cu.LastCommentAt.IsZero() {
		last = cu.LastCommentAt.Format("2006-01-02 15:04:05")
	}
	return &commentUserJSON{
		ID:            cu.ID,
		Username:      cu.Username,
		DisplayName:   cu.DisplayName,
		Email:         cu.Email,
		Avatar:        cu.Avatar,
		Role:          cu.Role,
		Status:        cu.Status,
		CreatedAt:     cu.CreatedAt.Format("2006-01-02 15:04:05"),
		CommentCount:  cu.CommentCount,
		LastCommentAt: last,
	}
}

// listCommentUsers GET /api/v1/comment-users：注册读者分页列表（keyword 搜索 + status 筛选），
// 每项附评论数与最后评论时间；counts 返回 all/banned 全量计数（不受筛选影响）。
func (h *Handler) listCommentUsers(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	keyword := strings.TrimSpace(c.Query("keyword"))
	status := c.DefaultQuery("status", "")
	if !store.ValidUserStatus(status) {
		status = "" // 非法值回退全量
	}

	items, total, err := h.store.ListCommentUsers(page, size, keyword, status)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	// counts 基于 role=reader 全量（不受当前筛选影响），供页头统计卡展示。
	countBanned, err := h.store.CountCommentUsers(store.UserStatusBanned)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	countTotal, err := h.store.CountCommentUsers("")
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	out := make([]*commentUserJSON, 0, len(items))
	for _, cu := range items {
		out = append(out, toCommentUserJSON(cu))
	}
	c.JSON(http.StatusOK, gin.H{
		"items": out,
		"total": total,
		"page":  page,
		"size":  size,
		"counts": gin.H{
			"all":    countTotal,
			"banned": countBanned,
		},
	})
}

// setCommentUserStatus PUT /api/v1/comment-users/:id/status：封禁/解封评论用户。
// 仅管理 role=reader；封禁即时生效（读者中间件按匿名处理 + 登录拒绝）。
func (h *Handler) setCommentUserStatus(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		Fail(c, http.StatusBadRequest, "用户 ID 不合法")
		return
	}
	var req struct {
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "请求格式错误")
		return
	}
	if !store.ValidUserStatus(req.Status) {
		Fail(c, http.StatusBadRequest, "status 仅支持 active / banned")
		return
	}
	user, err := h.store.GetUserByID(id)
	if err != nil || user == nil {
		Fail(c, http.StatusNotFound, "用户不存在")
		return
	}
	if user.Role != store.RoleReader {
		Fail(c, http.StatusBadRequest, "仅支持管理注册读者（评论用户）")
		return
	}
	if err := h.store.UpdateUserStatus(id, req.Status); err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "id": id, "status": req.Status})
}

// deleteCommentUser DELETE /api/v1/comment-users/:id：删除评论用户。
// 仅 role=reader；用户名下评论保留（如需清理先走批量处置）。
func (h *Handler) deleteCommentUser(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		Fail(c, http.StatusBadRequest, "用户 ID 不合法")
		return
	}
	currentID := c.GetInt64("userID")
	if id == currentID {
		Fail(c, http.StatusBadRequest, "不能删除当前登录用户")
		return
	}
	user, err := h.store.GetUserByID(id)
	if err != nil || user == nil {
		Fail(c, http.StatusNotFound, "用户不存在")
		return
	}
	if user.Role != store.RoleReader {
		Fail(c, http.StatusBadRequest, "仅支持管理注册读者（评论用户）")
		return
	}
	if err := h.store.DeleteUser(id); err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// bulkCommentUserComments POST /api/v1/comment-users/:id/comments/bulk：
// 按用户批量处置其名下全部评论（approve / spam / delete），返回受影响条数。
func (h *Handler) bulkCommentUserComments(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		Fail(c, http.StatusBadRequest, "用户 ID 不合法")
		return
	}
	var req struct {
		Action string `json:"action"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "请求格式错误")
		return
	}
	switch req.Action {
	case "approve", "spam", "delete":
		// valid
	default:
		Fail(c, http.StatusBadRequest, "action 仅支持 approve / spam / delete")
		return
	}
	user, err := h.store.GetUserByID(id)
	if err != nil || user == nil {
		Fail(c, http.StatusNotFound, "用户不存在")
		return
	}
	if user.Role != store.RoleReader {
		Fail(c, http.StatusBadRequest, "仅支持管理注册读者（评论用户）")
		return
	}
	affected, err := h.store.BulkCommentsByUser(id, req.Action)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "affected": affected})
}
