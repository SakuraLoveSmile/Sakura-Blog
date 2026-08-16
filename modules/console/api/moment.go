package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"blog/internal/store"
)

// momentJSON 瞬间 API 响应模型。
type momentJSON struct {
	ID         int64    `json:"id"`
	ContentMD  string   `json:"content_md"`
	Content    string   `json:"content"`
	Images     []string `json:"images"`
	Visibility string   `json:"visibility"`
	CreatedAt  string   `json:"created_at"`
}

func toMomentJSON(m *store.Moment) *momentJSON {
	imgs := store.DecodeMomentImages(m.Images)
	if imgs == nil {
		imgs = []string{}
	}
	return &momentJSON{
		ID:         m.ID,
		ContentMD:  m.ContentMD,
		Content:    m.ContentHTML,
		Images:     imgs,
		Visibility: m.Visibility,
		CreatedAt:  m.CreatedAt.Format(time.RFC3339),
	}
}

// listMoments GET /api/v1/moments：瞬间列表（分页）。
func (h *Handler) listMoments(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}

	total, err := h.store.CountMoments()
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	moments, err := h.store.ListMoments(page, size)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	items := make([]*momentJSON, 0, len(moments))
	for _, m := range moments {
		items = append(items, toMomentJSON(m))
	}

	c.JSON(http.StatusOK, gin.H{
		"items": items,
		"total": total,
		"page":  page,
		"size":  size,
	})
}

// createMoment POST /api/v1/moments：发布瞬间。
func (h *Handler) createMoment(c *gin.Context) {
	var req struct {
		ContentMD  string   `json:"content_md"`
		Images     []string `json:"images"`
		Visibility string   `json:"visibility"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "请求格式错误")
		return
	}
	// 内容或图片至少其一（纯图瞬间允许）
	if strings.TrimSpace(req.ContentMD) == "" && store.EncodeMomentImages(req.Images) == "" {
		Fail(c, http.StatusBadRequest, "内容不能为空")
		return
	}

	contentHTML, err := h.posts.Render(req.ContentMD)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "Markdown 渲染失败")
		return
	}

	m := &store.Moment{
		ContentMD:   req.ContentMD,
		ContentHTML: contentHTML,
		Images:      store.EncodeMomentImages(req.Images),
		Visibility:  req.Visibility,
	}
	if _, err := h.store.CreateMoment(m); err != nil {
		Fail(c, http.StatusInternalServerError, "发布失败: "+err.Error())
		return
	}
	c.JSON(http.StatusCreated, toMomentJSON(m))
}

// updateMoment PUT /api/v1/moments/:id：编辑瞬间（正文/图片/可见性）。
func (h *Handler) updateMoment(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "无效的 ID")
		return
	}
	var req struct {
		ContentMD  string   `json:"content_md"`
		Images     []string `json:"images"`
		Visibility string   `json:"visibility"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "请求格式错误")
		return
	}
	if strings.TrimSpace(req.ContentMD) == "" && store.EncodeMomentImages(req.Images) == "" {
		Fail(c, http.StatusBadRequest, "内容不能为空")
		return
	}
	existing, err := h.store.GetMoment(id)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if existing == nil {
		Fail(c, http.StatusNotFound, "瞬间不存在")
		return
	}
	contentHTML, err := h.posts.Render(req.ContentMD)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "Markdown 渲染失败")
		return
	}
	existing.ContentMD = req.ContentMD
	existing.ContentHTML = contentHTML
	existing.Images = store.EncodeMomentImages(req.Images)
	existing.Visibility = req.Visibility
	if err := h.store.UpdateMoment(existing); err != nil {
		Fail(c, http.StatusInternalServerError, "保存失败: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, toMomentJSON(existing))
}

// deleteMoment DELETE /api/v1/moments/:id：删除瞬间。
func (h *Handler) deleteMoment(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "无效的 ID")
		return
	}
	if err := h.store.DeleteMoment(id); err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
