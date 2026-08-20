package api

import (
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"blog/internal/store"
)

// taxonomyItem 分类/标签列表项（名称 + 已发布文章数）。
type taxonomyItem struct {
	Name     string `json:"name"`
	Count    int    `json:"count"`
	Path     string `json:"path"`
	ID       int64  `json:"id"`
	ParentID int64  `json:"parent_id"`
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
	tree, err := h.store.CategoryTree()
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	nodes := store.FlattenCategoryTree(tree)
	items := make([]taxonomyItem, 0, len(nodes))
	for _, n := range nodes {
		items = append(items, taxonomyItem{Name: n.Name, Count: n.Count, Path: n.Path, ID: n.ID, ParentID: n.ParentID})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Path < items[j].Path })
	c.JSON(http.StatusOK, gin.H{"tree": tree, "items": items, "total": len(items)})
}

func categoryFailure(c *gin.Context, err error) {
	switch {
	case errors.Is(err, store.ErrCategoryNotFound):
		Fail(c, http.StatusNotFound, "分类不存在")
	case errors.Is(err, store.ErrCategoryConflict):
		Fail(c, http.StatusConflict, "分类存在文章、子分类或层级冲突")
	case errors.Is(err, store.ErrCategoryInvalid):
		Fail(c, http.StatusBadRequest, "分类名称或路径不合法")
	default:
		Fail(c, http.StatusInternalServerError, err.Error())
	}
}

// createCategory POST /api/v1/categories：创建顶级或子分类。
func (h *Handler) createCategory(c *gin.Context) {
	var req struct {
		Name     string `json:"name"`
		ParentID int64  `json:"parent_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "请求格式错误")
		return
	}
	n, err := h.store.CreateCategory(req.Name, req.ParentID)
	if err != nil {
		categoryFailure(c, err)
		return
	}
	c.JSON(http.StatusCreated, n)
}

// updateCategory PUT /api/v1/categories/:id：修改名称和父级。
// 非数字参数继续走旧的按名称重命名接口。
func (h *Handler) updateCategory(c *gin.Context) {
	param := c.Param("id")
	if id, err := strconv.ParseInt(param, 10, 64); err == nil && id > 0 {
		var req struct {
			Name     string `json:"name"`
			ParentID *int64 `json:"parent_id"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			Fail(c, http.StatusBadRequest, "请求格式错误")
			return
		}
		current, err := h.store.GetCategoryByID(id)
		if err != nil {
			categoryFailure(c, err)
			return
		}
		parentID := current.ParentID
		if req.ParentID != nil {
			parentID = *req.ParentID
		}
		n, err := h.store.UpdateCategory(id, req.Name, parentID)
		if err != nil {
			categoryFailure(c, err)
			return
		}
		c.JSON(http.StatusOK, n)
		return
	}
	h.renameCategory(c)
}

// deleteCategoryByID DELETE /api/v1/categories/:id：只允许删除空叶子。
func (h *Handler) deleteCategoryByID(c *gin.Context) {
	param := c.Param("id")
	if id, err := strconv.ParseInt(param, 10, 64); err == nil && id > 0 {
		if err := h.store.DeleteCategoryByID(id); err != nil {
			categoryFailure(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}
	// 旧客户端按名称删除，保留旧返回结构；新客户端使用数字 ID 获得安全策略。
	h.deleteCategory(c)
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
	if old == "" {
		old = c.Param("id")
	}
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
	name := c.Param("name")
	if name == "" {
		name = c.Param("id")
	}
	n, err := h.store.DeleteCategory(name)
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
