package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"blog/internal/store"
)

// listWikidocs GET /api/v1/wiki/:project/docs：文档列表（含树结构）。
func (h *Handler) listWikidocs(c *gin.Context) {
	projectID := c.Param("project")
	docs, err := h.store.ListWikidocs(projectID)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	tree, err := h.store.BuildDocTree(projectID)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if docs == nil {
		docs = []*store.WikiDoc{}
	}
	if tree == nil {
		tree = []store.TreeNode{}
	}
	c.JSON(http.StatusOK, gin.H{"docs": docs, "tree": tree})
}

// getWikidoc GET /api/v1/wiki/:project/docs/:slug：单文档详情。
func (h *Handler) getWikidoc(c *gin.Context) {
	projectID := c.Param("project")
	slug := c.Param("slug")
	doc, err := h.store.GetWikidocBySlug(projectID, slug)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if doc == nil {
		Fail(c, http.StatusNotFound, "文档不存在")
		return
	}
	c.JSON(http.StatusOK, doc)
}

// createWikidoc POST /api/v1/wiki/:project/docs：创建文档。
func (h *Handler) createWikidoc(c *gin.Context) {
	projectID := c.Param("project")
	if strings.TrimSpace(projectID) == "" {
		Fail(c, http.StatusBadRequest, "project 不能为空")
		return
	}

	var req struct {
		Slug      string `json:"slug"`
		Title     string `json:"title"`
		Order     int    `json:"order"`
		Parent    int64  `json:"parent"`
		ContentMD string `json:"content_md"`
		Status    string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "请求格式错误")
		return
	}
	slug := strings.TrimSpace(req.Slug)
	if slug == "" {
		Fail(c, http.StatusBadRequest, "slug 不能为空")
		return
	}

	if existing, _ := h.store.GetWikidocBySlug(projectID, slug); existing != nil {
		Fail(c, http.StatusConflict, "slug 已存在")
		return
	}

	// parent 校验：父文档必须存在且属于当前项目，否则文档会从树中静默消失
	if req.Parent != 0 {
		parent, err := h.store.GetWikidocByID(req.Parent)
		if err != nil {
			Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		if parent == nil || parent.ProjectID != projectID {
			Fail(c, http.StatusBadRequest, "父文档不存在或不属于当前项目")
			return
		}
	}

	contentHTML, err := h.posts.Render(req.ContentMD)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "Markdown 渲染失败")
		return
	}

	status := req.Status
	if status != store.StatusDraft && status != store.StatusPublished {
		status = store.StatusDraft
	}

	doc := &store.WikiDoc{
		ProjectID:   projectID,
		Slug:        slug,
		Title:       strings.TrimSpace(req.Title),
		Order:       req.Order,
		Parent:      req.Parent,
		ContentMD:   req.ContentMD,
		ContentHTML: contentHTML,
		Status:      status,
	}
	if err := h.store.CreateWikidoc(doc); err != nil {
		Fail(c, http.StatusInternalServerError, "创建失败: " + err.Error())
		return
	}
	c.JSON(http.StatusCreated, gin.H{"ok": true, "id": doc.ID})
}

// updateWikidoc PUT /api/v1/wiki/:project/docs/:slug：更新文档。
func (h *Handler) updateWikidoc(c *gin.Context) {
	projectID := c.Param("project")
	slug := c.Param("slug")
	existing, err := h.store.GetWikidocBySlug(projectID, slug)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if existing == nil {
		Fail(c, http.StatusNotFound, "文档不存在")
		return
	}

	var req struct {
		Slug      string `json:"slug"`
		Title     string `json:"title"`
		Order     int    `json:"order"`
		Parent    int64  `json:"parent"`
		ContentMD string `json:"content_md"`
		Status    string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "请求格式错误")
		return
	}

	newSlug := strings.TrimSpace(req.Slug)
	if newSlug == "" {
		newSlug = slug
	}
	if newSlug != slug {
		if other, _ := h.store.GetWikidocBySlug(projectID, newSlug); other != nil {
			Fail(c, http.StatusConflict, "slug 已被占用")
			return
		}
	}

	// parent 校验：不能是自身，父文档必须存在且属于当前项目
	if req.Parent != 0 {
		if req.Parent == existing.ID {
			Fail(c, http.StatusBadRequest, "不能将文档设为自身的子文档")
			return
		}
		parent, err := h.store.GetWikidocByID(req.Parent)
		if err != nil {
			Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		if parent == nil || parent.ProjectID != projectID {
			Fail(c, http.StatusBadRequest, "父文档不存在或不属于当前项目")
			return
		}
		// 防循环：沿目标父文档的 parent 链向上遍历，链上出现当前文档即构成循环引用
		cur := parent
		for cur != nil && cur.Parent != 0 {
			if cur.Parent == existing.ID {
				Fail(c, http.StatusBadRequest, "不能将文档设为自身后代的子文档")
				return
			}
			next, err := h.store.GetWikidocByID(cur.Parent)
			if err != nil {
				Fail(c, http.StatusInternalServerError, err.Error())
				return
			}
			cur = next
		}
	}

	contentHTML, err := h.posts.Render(req.ContentMD)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "Markdown 渲染失败")
		return
	}

	status := req.Status
	if status != store.StatusDraft && status != store.StatusPublished {
		status = existing.Status
	}

	doc := &store.WikiDoc{
		ID:          existing.ID,
		ProjectID:   projectID,
		Slug:        newSlug,
		Title:       strings.TrimSpace(req.Title),
		Order:       req.Order,
		Parent:      req.Parent,
		ContentMD:   req.ContentMD,
		ContentHTML: contentHTML,
		Status:      status,
	}
	if err := h.store.UpdateWikidoc(doc); err != nil {
		Fail(c, http.StatusInternalServerError, "更新失败: " + err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// deleteWikidoc DELETE /api/v1/wiki/:project/docs/:slug：删除文档。
func (h *Handler) deleteWikidoc(c *gin.Context) {
	projectID := c.Param("project")
	slug := c.Param("slug")
	doc, err := h.store.GetWikidocBySlug(projectID, slug)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if doc == nil {
		Fail(c, http.StatusNotFound, "文档不存在")
		return
	}
	if err := h.store.DeleteWikidoc(doc.ID); err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
