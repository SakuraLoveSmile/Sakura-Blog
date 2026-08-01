package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"blog/internal/store"
)

// parseIDParam 解析路径中的 :id 参数，非法时写 400 并返回 false。
func (h *Handler) parseIDParam(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 ID"})
		return 0, false
	}
	return id, true
}

// listNotebooks GET /api/v1/notebooks：列出所有笔记本（按 order, id 升序）。
func (h *Handler) listNotebooks(c *gin.Context) {
	notebooks, err := h.store.ListNotebooks()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if notebooks == nil {
		notebooks = []*store.Notebook{}
	}
	c.JSON(http.StatusOK, gin.H{"items": notebooks, "total": len(notebooks)})
}

// getNotebook GET /api/v1/notebooks/:id：单个笔记本详情。
func (h *Handler) getNotebook(c *gin.Context) {
	id, ok := h.parseIDParam(c)
	if !ok {
		return
	}
	nb, err := h.store.GetNotebook(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if nb == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "笔记本不存在"})
		return
	}
	c.JSON(http.StatusOK, nb)
}

// createNotebook POST /api/v1/notebooks：创建笔记本。
func (h *Handler) createNotebook(c *gin.Context) {
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Cover       string `json:"cover"`
		Order       int    `json:"order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name 不能为空"})
		return
	}
	if existing, _ := h.store.GetNotebookByName(name); existing != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "name 已存在"})
		return
	}

	nb := &store.Notebook{
		Name:        name,
		Description: req.Description,
		Cover:       req.Cover,
		Order:       req.Order,
	}
	if err := h.store.CreateNotebook(nb); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"ok": true, "id": nb.ID})
}

// updateNotebook PUT /api/v1/notebooks/:id：更新笔记本（name/description/cover/order）。
// 字段用指针接收以区分「未提供」与「传空值」：未提供的字段保持原值。
func (h *Handler) updateNotebook(c *gin.Context) {
	id, ok := h.parseIDParam(c)
	if !ok {
		return
	}
	existing, err := h.store.GetNotebook(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if existing == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "笔记本不存在"})
		return
	}

	var req struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		Cover       *string `json:"cover"`
		Order       *int    `json:"order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}

	nb := &store.Notebook{
		ID:          existing.ID,
		Name:        existing.Name,
		Description: existing.Description,
		Cover:       existing.Cover,
		Order:       existing.Order,
	}
	if req.Name != nil {
		newName := strings.TrimSpace(*req.Name)
		if newName == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "name 不能为空"})
			return
		}
		if newName != existing.Name {
			if other, _ := h.store.GetNotebookByName(newName); other != nil {
				c.JSON(http.StatusConflict, gin.H{"error": "name 已被占用"})
				return
			}
		}
		nb.Name = newName
	}
	if req.Description != nil {
		nb.Description = *req.Description
	}
	if req.Cover != nil {
		nb.Cover = *req.Cover
	}
	if req.Order != nil {
		nb.Order = *req.Order
	}

	if err := h.store.UpdateNotebook(nb); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// deleteNotebook DELETE /api/v1/notebooks/:id：删除笔记本（级联删除其下所有笔记）。
func (h *Handler) deleteNotebook(c *gin.Context) {
	id, ok := h.parseIDParam(c)
	if !ok {
		return
	}
	nb, err := h.store.GetNotebook(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if nb == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "笔记本不存在"})
		return
	}
	if err := h.store.DeleteNotebook(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// listNotes GET /api/v1/notebooks/:id/notes：分页列出笔记本下的笔记。
// page / per_page query 参数，默认 page=1, per_page=20。
func (h *Handler) listNotes(c *gin.Context) {
	notebookID, ok := h.parseIDParam(c)
	if !ok {
		return
	}
	nb, err := h.store.GetNotebook(notebookID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if nb == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "笔记本不存在"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}

	notes, total, err := h.store.ListNotesByNotebook(notebookID, perPage, (page-1)*perPage)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if notes == nil {
		notes = []*store.Note{}
	}
	c.JSON(http.StatusOK, gin.H{
		"notes":    notes,
		"total":    total,
		"page":     page,
		"per_page": perPage,
	})
}

// getNote GET /api/v1/notes/:id：单条笔记详情。
func (h *Handler) getNote(c *gin.Context) {
	id, ok := h.parseIDParam(c)
	if !ok {
		return
	}
	note, err := h.store.GetNote(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if note == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "笔记不存在"})
		return
	}
	c.JSON(http.StatusOK, note)
}

// createNote POST /api/v1/notebooks/:id/notes：在笔记本下创建笔记。
func (h *Handler) createNote(c *gin.Context) {
	notebookID, ok := h.parseIDParam(c)
	if !ok {
		return
	}
	nb, err := h.store.GetNotebook(notebookID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if nb == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "笔记本不存在"})
		return
	}

	var req struct {
		Title   string `json:"title"`
		Content string `json:"content"`
		Tags    string `json:"tags"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}
	if strings.TrimSpace(req.Title) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title 不能为空"})
		return
	}

	contentHTML, err := h.posts.Render(req.Content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Markdown 渲染失败"})
		return
	}

	note := &store.Note{
		NotebookID:  notebookID,
		Title:       strings.TrimSpace(req.Title),
		Content:     req.Content,
		ContentHTML: contentHTML,
		Tags:        req.Tags,
	}
	if err := h.store.CreateNote(note); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"ok": true, "id": note.ID})
}

// updateNote PUT /api/v1/notes/:id：更新笔记（title/content/tags/notebook_id）。
// content 变更时重新渲染 HTML；字段用指针接收以区分「未提供」与「传空值」。
func (h *Handler) updateNote(c *gin.Context) {
	id, ok := h.parseIDParam(c)
	if !ok {
		return
	}
	existing, err := h.store.GetNote(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if existing == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "笔记不存在"})
		return
	}

	var req struct {
		NotebookID *int64  `json:"notebook_id"`
		Title      *string `json:"title"`
		Content    *string `json:"content"`
		Tags       *string `json:"tags"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}

	note := &store.Note{
		ID:          existing.ID,
		NotebookID:  existing.NotebookID,
		Title:       existing.Title,
		Content:     existing.Content,
		ContentHTML: existing.ContentHTML,
		Tags:        existing.Tags,
	}
	if req.NotebookID != nil {
		if *req.NotebookID <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 notebook_id"})
			return
		}
		target, err := h.store.GetNotebook(*req.NotebookID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if target == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "目标笔记本不存在"})
			return
		}
		note.NotebookID = *req.NotebookID
	}
	if req.Title != nil {
		newTitle := strings.TrimSpace(*req.Title)
		if newTitle == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "title 不能为空"})
			return
		}
		note.Title = newTitle
	}
	if req.Content != nil {
		if *req.Content != existing.Content {
			contentHTML, err := h.posts.Render(*req.Content)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Markdown 渲染失败"})
				return
			}
			note.ContentHTML = contentHTML
		}
		note.Content = *req.Content
	}
	if req.Tags != nil {
		note.Tags = *req.Tags
	}

	if err := h.store.UpdateNote(note); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// deleteNote DELETE /api/v1/notes/:id：删除单条笔记。
func (h *Handler) deleteNote(c *gin.Context) {
	id, ok := h.parseIDParam(c)
	if !ok {
		return
	}
	note, err := h.store.GetNote(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if note == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "笔记不存在"})
		return
	}
	if err := h.store.DeleteNote(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// noteTags GET /api/v1/note-tags：返回所有笔记标签聚合列表（去重排序）。
func (h *Handler) noteTags(c *gin.Context) {
	tags, err := h.store.AllNoteTags()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if tags == nil {
		tags = []string{}
	}
	c.JSON(http.StatusOK, gin.H{"tags": tags})
}
