package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"blog/internal/store"
)

// 专栏（Spec 52）：/api/v1/topics CRUD，对齐 notebook.go 的既有 API 风格与权限中间件。
// slug 与 name 双唯一：URL 用 slug，展示用 name；冲突统一 409；删除保护（有文章关联）403。

// listTopics GET /api/v1/topics：列出全部专栏（sort DESC + 创建时间线性），每项带文章总数角标。
func (h *Handler) listTopics(c *gin.Context) {
	topics, err := h.store.ListTopics()
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	type topicWithCount struct {
		*store.Topic
		PostCount int `json:"post_count"`
	}
	items := make([]topicWithCount, 0, len(topics))
	for _, t := range topics {
		n, err := h.store.CountAllByTopicID(t.ID)
		if err != nil {
			Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		items = append(items, topicWithCount{Topic: t, PostCount: n})
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": len(items)})
}

// createTopic POST /api/v1/topics：新建专栏。name 必填；slug 缺省由 name 生成。
// name/slug 任一冲突返回 409（双唯一约束，删除保护之外的写冲突主入口）。
func (h *Handler) createTopic(c *gin.Context) {
	var req struct {
		Name        string `json:"name"`
		Slug        string `json:"slug"`
		Description string `json:"description"`
		Icon        string `json:"icon"`
		Cover       string `json:"cover"`
		Sort        int    `json:"sort"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "请求格式错误")
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		Fail(c, http.StatusBadRequest, "name 不能为空")
		return
	}
	if existing, _ := h.store.GetTopicByName(name); existing != nil {
		Fail(c, http.StatusConflict, "name 已存在")
		return
	}
	slug := strings.TrimSpace(req.Slug)
	if slug == "" {
		slug = generateSlug(name)
	}
	if existing, _ := h.store.GetTopicBySlug(slug); existing != nil {
		Fail(c, http.StatusConflict, "slug 已被占用")
		return
	}

	t := &store.Topic{
		Name:        name,
		Slug:        slug,
		Description: req.Description,
		Icon:        req.Icon,
		Cover:       req.Cover,
		Sort:        req.Sort,
	}
	if err := h.store.CreateTopic(t); err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed: topics.") {
			Fail(c, http.StatusConflict, "name 或 slug 已被占用")
			return
		}
		Fail(c, http.StatusInternalServerError, "创建失败: "+err.Error())
		return
	}
	c.JSON(http.StatusCreated, gin.H{"ok": true, "id": t.ID})
}

// updateTopic PUT /api/v1/topics/:id：更新专栏（name/slug/description/icon/cover/sort）。
// 字段用指针接收以区分「未提供」与「传空值」：未提供的字段保持原值。
func (h *Handler) updateTopic(c *gin.Context) {
	id, ok := h.parseIDParam(c)
	if !ok {
		return
	}
	existing, err := h.store.GetTopicByID(id)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if existing == nil {
		Fail(c, http.StatusNotFound, "专栏不存在")
		return
	}

	var req struct {
		Name        *string `json:"name"`
		Slug        *string `json:"slug"`
		Description *string `json:"description"`
		Icon        *string `json:"icon"`
		Cover       *string `json:"cover"`
		Sort        *int    `json:"sort"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "请求格式错误")
		return
	}

	t := &store.Topic{
		ID:          existing.ID,
		Name:        existing.Name,
		Slug:        existing.Slug,
		Description: existing.Description,
		Icon:        existing.Icon,
		Cover:       existing.Cover,
		Sort:        existing.Sort,
	}
	if req.Name != nil {
		newName := strings.TrimSpace(*req.Name)
		if newName == "" {
			Fail(c, http.StatusBadRequest, "name 不能为空")
			return
		}
		if newName != existing.Name {
			if other, _ := h.store.GetTopicByName(newName); other != nil {
				Fail(c, http.StatusConflict, "name 已被占用")
				return
			}
		}
		t.Name = newName
	}
	if req.Slug != nil {
		newSlug := strings.TrimSpace(*req.Slug)
		if newSlug == "" {
			newSlug = generateSlug(t.Name)
		}
		if newSlug != existing.Slug {
			if other, _ := h.store.GetTopicBySlug(newSlug); other != nil {
				Fail(c, http.StatusConflict, "slug 已被占用")
				return
			}
		}
		t.Slug = newSlug
	}
	if req.Description != nil {
		t.Description = *req.Description
	}
	if req.Icon != nil {
		t.Icon = *req.Icon
	}
	if req.Cover != nil {
		t.Cover = *req.Cover
	}
	if req.Sort != nil {
		t.Sort = *req.Sort
	}

	if err := h.store.UpdateTopic(t); err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed: topics.") {
			Fail(c, http.StatusConflict, "name 或 slug 已被占用")
			return
		}
		Fail(c, http.StatusInternalServerError, "更新失败: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// deleteTopic DELETE /api/v1/topics/:id：删除专栏。专栏下仍有文章时返回 403（删除保护）。
func (h *Handler) deleteTopic(c *gin.Context) {
	id, ok := h.parseIDParam(c)
	if !ok {
		return
	}
	t, err := h.store.GetTopicByID(id)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if t == nil {
		Fail(c, http.StatusNotFound, "专栏不存在")
		return
	}
	if err := h.store.DeleteTopic(id); err != nil {
		if err == store.ErrTopicInUse {
			Fail(c, http.StatusForbidden, "专栏下仍有文章，请先解除关联")
			return
		}
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
