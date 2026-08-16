package api

import (
	"net/http"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
)

// taxonomyItem 分类/标签列表项（名称 + 已发布文章数）。
type taxonomyItem struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// sortedTaxonomy 把计数 map 转为按 count 降序、name 升序的列表。
func sortedTaxonomy(m map[string]int) []taxonomyItem {
	items := make([]taxonomyItem, 0, len(m))
	for name, count := range m {
		items = append(items, taxonomyItem{Name: name, Count: count})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Count != items[j].Count {
			return items[i].Count > items[j].Count
		}
		return items[i].Name < items[j].Name
	})
	return items
}

// listCategories GET /api/v1/categories：分类及已发布文章数。
func (h *Handler) listCategories(c *gin.Context) {
	m, err := h.store.CountByCategory()
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	items := sortedTaxonomy(m)
	c.JSON(http.StatusOK, gin.H{"items": items, "total": len(items)})
}

// listTags GET /api/v1/tags：标签及已发布文章数。
func (h *Handler) listTags(c *gin.Context) {
	m, err := h.store.CountByTag()
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	items := sortedTaxonomy(m)
	c.JSON(http.StatusOK, gin.H{"items": items, "total": len(items)})
}

// renameCategory PUT /api/v1/categories/:name：把分类 :name 重命名为 body.name（改所有文章，含草稿）。
func (h *Handler) renameCategory(c *gin.Context) {
	old := c.Param("name")
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "请求格式错误")
		return
	}
	newName := strings.TrimSpace(req.Name)
	if newName == "" {
		Fail(c, http.StatusBadRequest, "新分类名不能为空")
		return
	}
	n, err := h.store.RenameCategory(old, newName)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "affected": n})
}

// deleteCategory DELETE /api/v1/categories/:name：清空所有文章的该分类。
func (h *Handler) deleteCategory(c *gin.Context) {
	n, err := h.store.DeleteCategory(c.Param("name"))
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "affected": n})
}

// renameTag PUT /api/v1/tags/:name：把标签 :name 重命名为 body.name（CSV 逐项替换 + 规范化）。
func (h *Handler) renameTag(c *gin.Context) {
	old := c.Param("name")
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "请求格式错误")
		return
	}
	newName := strings.TrimSpace(req.Name)
	if newName == "" {
		Fail(c, http.StatusBadRequest, "新标签名不能为空")
		return
	}
	n, err := h.store.RenameTag(old, newName)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "affected": n})
}

// deleteTag DELETE /api/v1/tags/:name：从所有文章移除该标签。
func (h *Handler) deleteTag(c *gin.Context) {
	n, err := h.store.DeleteTag(c.Param("name"))
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "affected": n})
}

// mergeTags POST /api/v1/tags/merge：把 body.sources 合并到 body.target。
func (h *Handler) mergeTags(c *gin.Context) {
	var req struct {
		Sources []string `json:"sources"`
		Target  string   `json:"target"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "请求格式错误")
		return
	}
	target := strings.TrimSpace(req.Target)
	if target == "" || len(req.Sources) == 0 {
		Fail(c, http.StatusBadRequest, "目标标签与来源不能为空")
		return
	}
	n, err := h.store.MergeTags(req.Sources, target)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "affected": n})
}
