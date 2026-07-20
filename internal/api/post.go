package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"blog/internal/store"
)

// postJSON 文章 API 响应模型。
type postJSON struct {
	Slug      string   `json:"slug"`
	Title     string   `json:"title"`
	Excerpt   string   `json:"excerpt"`
	Cover     string   `json:"cover"`
	Category  string   `json:"category"`
	Topic     string   `json:"topic"`
	Tags      string   `json:"tags"`
	ContentMD string   `json:"content_md"`
	Status    string   `json:"status"`
	Pinned    bool     `json:"pinned"`
	Password  string   `json:"password"`
	PublishAt string   `json:"publish_at"`
	Poster    *posterJ `json:"poster,omitempty"`
	CreatedAt string   `json:"created_at"`
	UpdatedAt string   `json:"updated_at"`
	Views     int64    `json:"views"`
}

type posterJ struct {
	Headline string `json:"headline"`
	Topic    string `json:"topic"`
	Caption  string `json:"caption"`
	Color    string `json:"color"`
}

func toPostJSON(p *store.Post, views int64) *postJSON {
	pj := &postJSON{
		Slug:      p.Slug,
		Title:     p.Title,
		Excerpt:   p.Excerpt,
		Cover:     p.Cover,
		Category:  p.Category,
		Topic:     p.Topic,
		Tags:      p.Tags,
		ContentMD: p.ContentMD,
		Status:    p.Status,
		Pinned:    p.Pinned,
		Password:  p.Password,
		CreatedAt: p.CreatedAt.Format(time.RFC3339),
		UpdatedAt: p.UpdatedAt.Format(time.RFC3339),
		Views:     views,
	}
	if !p.PublishAt.IsZero() {
		pj.PublishAt = p.PublishAt.UTC().Format("2006-01-02T15:04")
	}
	if pd := store.DecodePoster(p.Poster); pd != nil {
		pj.Poster = &posterJ{Headline: pd.Headline, Topic: pd.Topic, Caption: pd.Caption, Color: pd.Color}
	}
	return pj
}

// listPosts GET /api/v1/posts：文章列表（支持 status/category/tag 筛选 + 分页）。
func (h *Handler) listPosts(c *gin.Context) {
	status := c.Query("status")
	category := c.Query("category")
	tag := c.Query("tag")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}

	// 获取全部文章后在内存筛选（SQLite 单文件博客数据量有限，无需复杂 SQL）
	posts, err := h.store.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 筛选
	var filtered []*store.Post
	for _, p := range posts {
		if status != "" && p.Status != status {
			continue
		}
		if category != "" && p.Category != category {
			continue
		}
		if tag != "" && !containsTag(p.Tags, tag) {
			continue
		}
		filtered = append(filtered, p)
	}

	total := len(filtered)
	// 分页
	start := (page - 1) * size
	if start > total {
		start = total
	}
	end := start + size
	if end > total {
		end = total
	}
	paged := filtered[start:end]

	items := make([]*postJSON, 0, len(paged))
	for _, p := range paged {
		views, _ := h.store.GetViews(p.Slug)
		items = append(items, toPostJSON(p, views))
	}

	c.JSON(http.StatusOK, gin.H{
		"items": items,
		"total": total,
		"page":  page,
		"size":  size,
	})
}

// getPost GET /api/v1/posts/:slug：文章详情。
func (h *Handler) getPost(c *gin.Context) {
	slug := c.Param("slug")
	p, err := h.store.GetBySlug(slug)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if p == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "文章不存在"})
		return
	}
	views, _ := h.store.GetViews(slug)
	c.JSON(http.StatusOK, toPostJSON(p, views))
}

// createPost POST /api/v1/posts：创建文章。
func (h *Handler) createPost(c *gin.Context) {
	var req postRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误: " + err.Error()})
		return
	}
	if strings.TrimSpace(req.Slug) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "slug 不能为空"})
		return
	}

	// 检查 slug 唯一
	if existing, _ := h.store.GetBySlug(req.Slug); existing != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "slug 已存在: " + req.Slug})
		return
	}

	contentHTML, err := h.posts.Render(req.ContentMD)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Markdown 渲染失败: " + err.Error()})
		return
	}

	p := reqToPost(&req, contentHTML)
	if _, err := h.store.Create(p); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusCreated, toPostJSON(p, 0))
}

// updatePost PUT /api/v1/posts/:slug：更新文章。
func (h *Handler) updatePost(c *gin.Context) {
	slug := c.Param("slug")
	existing, err := h.store.GetBySlug(slug)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if existing == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "文章不存在"})
		return
	}

	var req postRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误: " + err.Error()})
		return
	}

	// slug 变更时检查唯一性
	newSlug := strings.TrimSpace(req.Slug)
	if newSlug == "" {
		newSlug = slug
	}
	if newSlug != slug {
		if other, _ := h.store.GetBySlug(newSlug); other != nil {
			c.JSON(http.StatusConflict, gin.H{"error": "slug 已被占用: " + newSlug})
			return
		}
	}

	contentHTML, err := h.posts.Render(req.ContentMD)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Markdown 渲染失败: " + err.Error()})
		return
	}

	p := reqToPost(&req, contentHTML)
	p.Slug = newSlug

	// slug 变更：先删旧再建新（保留 created_at）
	if newSlug != slug {
		if err := h.store.Delete(slug); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败: " + err.Error()})
			return
		}
		p.CreatedAt = existing.CreatedAt
		if p.References == "" {
			p.References = existing.References
		}
		if p.Poster == "" {
			p.Poster = existing.Poster
		}
		if _, err := h.store.Create(p); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败: " + err.Error()})
			return
		}
	} else {
		if err := h.store.Update(slug, p); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败: " + err.Error()})
			return
		}
	}

	views, _ := h.store.GetViews(newSlug)
	c.JSON(http.StatusOK, toPostJSON(p, views))
}

// deletePost DELETE /api/v1/posts/:slug：删除文章。
func (h *Handler) deletePost(c *gin.Context) {
	slug := c.Param("slug")
	if err := h.store.Delete(slug); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// --- 辅助 ---

type postRequest struct {
	Slug      string `json:"slug"`
	Title     string `json:"title"`
	Excerpt   string `json:"excerpt"`
	Cover     string `json:"cover"`
	Category  string `json:"category"`
	Topic     string `json:"topic"`
	Tags      string `json:"tags"`
	ContentMD string `json:"content_md"`
	Status    string `json:"status"`
	Pinned    bool   `json:"pinned"`
	Password  string `json:"password"`
	PublishAt string `json:"publish_at"`
	Poster    *struct {
		Headline string `json:"headline"`
		Topic    string `json:"topic"`
		Caption  string `json:"caption"`
		Color    string `json:"color"`
	} `json:"poster"`
}

func reqToPost(req *postRequest, contentHTML string) *store.Post {
	status := req.Status
	if status != store.StatusDraft && status != store.StatusPublished {
		status = store.StatusDraft
	}
	p := &store.Post{
		Slug:        strings.TrimSpace(req.Slug),
		Title:       strings.TrimSpace(req.Title),
		Excerpt:     strings.TrimSpace(req.Excerpt),
		Cover:       strings.TrimSpace(req.Cover),
		Category:    strings.TrimSpace(req.Category),
		Topic:       strings.TrimSpace(req.Topic),
		Tags:        normalizeTags(req.Tags),
		ContentMD:   req.ContentMD,
		ContentHTML: contentHTML,
		Status:      status,
		Pinned:      req.Pinned,
		Password:    strings.TrimSpace(req.Password),
		PublishAt:   parsePublishAt(req.PublishAt),
	}
	if req.Poster != nil {
		p.Poster = store.EncodePoster(store.PosterData{
			Headline: req.Poster.Headline,
			Topic:    req.Poster.Topic,
			Caption:  req.Poster.Caption,
			Color:    req.Poster.Color,
		})
	}
	return p
}

func normalizeTags(raw string) string {
	seen := map[string]bool{}
	var out []string
	for _, t := range strings.Split(raw, ",") {
		t = strings.TrimSpace(t)
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	return strings.Join(out, ",")
}

func parsePublishAt(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{"2006-01-02T15:04", "2006-01-02 15:04", time.RFC3339, "2006-01-02T15:04:05Z"} {
		if tm, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return tm.UTC()
		}
	}
	return time.Time{}
}

func containsTag(tags, target string) bool {
	for _, t := range strings.Split(tags, ",") {
		if strings.TrimSpace(t) == target {
			return true
		}
	}
	return false
}
