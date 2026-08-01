package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"blog/internal/store"
)

// slug 生成用预编译正则（避免每次调用重新编译）。
var (
	reSpace       = regexp.MustCompile(`\s+`)            // 空白转连字符
	reNonSlugChar = regexp.MustCompile(`[^\p{L}\p{N}-]`) // 移除非 Unicode 字母/数字/连字符
	reMultiDash   = regexp.MustCompile(`-{2,}`)          // 折叠多个连字符
)

// generateSlug 从标题生成 URL 友好的 slug。
// 英文标题：转小写、空格转连字符、移除非字母数字；
// 中文标题：保留 Unicode 字母（\p{L} 含中文）和数字；生成结果为空时用时间戳兜底。
func generateSlug(title string) string {
	s := strings.ToLower(strings.TrimSpace(title))
	s = reSpace.ReplaceAllString(s, "-")
	s = reNonSlugChar.ReplaceAllString(s, "")
	s = reMultiDash.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = fmt.Sprintf("post-%d", time.Now().Unix())
	}
	return s
}

// ensureUniqueSlug 检查 slug 是否冲突，冲突时追加 -2/-3 后缀直到唯一。
func (h *Handler) ensureUniqueSlug(slug string) string {
	base := slug
	for n := 2; ; n++ {
		if existing, _ := h.store.GetBySlug(slug); existing == nil {
			return slug
		}
		slug = fmt.Sprintf("%s-%d", base, n)
	}
}

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
	// 文章级 SEO（Spec 42）
	MetaDescription string `json:"meta_description"`
	MetaKeywords    string `json:"meta_keywords"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
	Views           int64  `json:"views"`
	HasDraft        bool   `json:"has_draft"`
	// Draft 草稿头内容（整份编辑表单 JSON），仅详情接口且有草稿时返回。
	Draft json.RawMessage `json:"draft,omitempty"`
	// 多作者体系（Spec 38）：作者 ID
	Author string `json:"author"`
}

type posterJ struct {
	Headline string `json:"headline"`
	Topic    string `json:"topic"`
	Caption  string `json:"caption"`
	Color    string `json:"color"`
}

func toPostJSON(p *store.Post, views int64) *postJSON {
	pj := &postJSON{
		Slug:            p.Slug,
		Title:           p.Title,
		Excerpt:         p.Excerpt,
		Cover:           p.Cover,
		Category:        p.Category,
		Topic:           p.Topic,
		Tags:            p.Tags,
		ContentMD:       p.ContentMD,
		Status:          p.Status,
		Pinned:          p.Pinned,
		Password:        p.Password,
		MetaDescription: p.MetaDescription,
		MetaKeywords:    p.MetaKeywords,
		CreatedAt:       p.CreatedAt.Format(time.RFC3339),
		UpdatedAt:       p.UpdatedAt.Format(time.RFC3339),
		Views:           views,
		HasDraft:        strings.TrimSpace(p.DraftJSON) != "",
		Author:          p.Author,
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

	// 回收站专用列表
	if status == "trashed" {
		posts, err := h.store.ListTrashed()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"items": posts, "total": len(posts)})
		return
	}

	// 获取全部未删除文章后在内存筛选（SQLite 单文件博客数据量有限，无需复杂 SQL）
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
	pj := toPostJSON(p, views)
	if pj.HasDraft {
		pj.Draft = json.RawMessage(p.DraftJSON)
	}
	c.JSON(http.StatusOK, pj)
}

// createPost POST /api/v1/posts：创建文章。
func (h *Handler) createPost(c *gin.Context) {
	var req postRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误: " + err.Error()})
		return
	}
	// 空 slug 时从标题自动生成（Spec Task 6），并确保唯一；用户指定 slug 则检查冲突
	slugVal := strings.TrimSpace(req.Slug)
	if slugVal == "" {
		slugVal = h.ensureUniqueSlug(generateSlug(req.Title))
		req.Slug = slugVal
	} else if existing, _ := h.store.GetBySlug(slugVal); existing != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "slug 已存在: " + slugVal})
		return
	}

	contentHTML, err := h.posts.Render(req.ContentMD)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Markdown 渲染失败: " + err.Error()})
		return
	}

	p := reqToPost(&req, contentHTML)
	// 默认封面兜底：无封面时按 slug 生成稳定的 picsum seed 图
	if p.Cover == "" {
		p.Cover = defaultCover(req.Slug)
	}
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
	// 发布路径默认封面兜底
	if p.Status == store.StatusPublished && p.Cover == "" {
		p.Cover = defaultCover(newSlug)
	}

	if err := h.savePostWithSnapshot(existing, p, newSlug); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败: " + err.Error()})
		return
	}

	views, _ := h.store.GetViews(newSlug)
	c.JSON(http.StatusOK, toPostJSON(p, views))
}

// defaultCover 默认封面 URL：picsum.photos seed 模式，同 slug 永远返回同一张图。
// source.unsplash.com 已停用；picsum 图源为 Unsplash 授权图片，免 Key 免费商用。
func defaultCover(slug string) string {
	return fmt.Sprintf("https://picsum.photos/seed/%s/1200/675", slug)
}

// savePostDraft PUT /api/v1/posts/:slug/draft：保存草稿头（整份编辑表单）。
// 不渲染、不产 revision、不影响线上内容；列表小绿点据此显示。
func (h *Handler) savePostDraft(c *gin.Context) {
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
	b, err := json.Marshal(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "草稿序列化失败: " + err.Error()})
		return
	}
	if err := h.store.SaveDraft(slug, string(b)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存草稿失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "has_draft": true})
}

// publishPost POST /api/v1/posts/:slug/publish：应用草稿并发布。
// 请求体与 updatePost 相同（整份表单）；强制 status=published，成功后清空草稿头。
func (h *Handler) publishPost(c *gin.Context) {
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
	req.Status = store.StatusPublished

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
	if p.Cover == "" {
		p.Cover = defaultCover(newSlug)
	}

	if err := h.savePostWithSnapshot(existing, p, newSlug); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "发布失败: " + err.Error()})
		return
	}
	// 草稿已应用，清空草稿头（slug 变更时新记录本就无草稿，兼顾清理旧 slug）
	h.store.ClearDraft(newSlug)
	if newSlug != slug {
		h.store.ClearDraft(slug)
	}

	views, _ := h.store.GetViews(newSlug)
	h.notifySEOPublished(newSlug)
	c.JSON(http.StatusOK, toPostJSON(p, views))
}

// discardPostDraft DELETE /api/v1/posts/:slug/draft：放弃草稿头。
func (h *Handler) discardPostDraft(c *gin.Context) {
	slug := c.Param("slug")
	if err := h.store.ClearDraft(slug); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// savePostWithSnapshot 保存文章并在成功后把「保存前」的旧版本写入 revisions（Spec 22）。
// existing 为保存前查出的旧记录；p 为新内容（p.Slug 已设）；newSlug 为目标 slug。
// 内容无变化（content_md + title 均相同）时不产生空版本。
func (h *Handler) savePostWithSnapshot(existing, p *store.Post, newSlug string) error {
	if newSlug != existing.Slug {
		// slug 变更：先删旧再建新（保留 created_at）
		if err := h.store.Delete(existing.Slug); err != nil {
			return err
		}
		p.CreatedAt = existing.CreatedAt
		if p.References == "" {
			p.References = existing.References
		}
		if p.Poster == "" {
			p.Poster = existing.Poster
		}
		if _, err := h.store.Create(p); err != nil {
			return err
		}
		// 同步迁移版本归属
		h.store.UpdateRevisionSlug(existing.Slug, newSlug)
	} else {
		if err := h.store.Update(existing.Slug, p); err != nil {
			return err
		}
	}
	h.snapshotRevision(existing, p)
	return nil
}

// snapshotRevision 把保存前的旧版本写入 revisions；内容无变化则跳过。
func (h *Handler) snapshotRevision(old, neu *store.Post) {
	if old.ContentMD == neu.ContentMD && old.Title == neu.Title {
		return
	}
	h.store.CreateRevision(&store.Revision{
		PostSlug:  neu.Slug,
		Title:     old.Title,
		Excerpt:   old.Excerpt,
		ContentMD: old.ContentMD,
		Tags:      old.Tags,
		Category:  old.Category,
		Status:    old.Status,
	})
}

// deletePost DELETE /api/v1/posts/:slug：软删除文章（移入回收站）。
func (h *Handler) deletePost(c *gin.Context) {
	slug := c.Param("slug")
	if err := h.store.Delete(slug); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// restorePost POST /api/v1/posts/:slug/restore：从回收站恢复文章。
func (h *Handler) restorePost(c *gin.Context) {
	slug := c.Param("slug")
	if err := h.store.RestorePost(slug); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// purgePost DELETE /api/v1/posts/:slug/purge：彻底删除文章（物理删除）。
func (h *Handler) purgePost(c *gin.Context) {
	slug := c.Param("slug")
	if err := h.store.PurgePost(slug); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// 连带清理版本快照（Spec 22）
	h.store.DeleteRevisionsByPost(slug)
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
	// 文章级 SEO（Spec 42）
	MetaDescription string `json:"meta_description"`
	MetaKeywords    string `json:"meta_keywords"`
	// 多作者体系（Spec 38）：作者 ID
	Author string `json:"author"`
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
		// 文章级 SEO（Spec 42）：空串=未配置，回退站点级/现状链
		MetaDescription: strings.TrimSpace(req.MetaDescription),
		MetaKeywords:    strings.TrimSpace(req.MetaKeywords),
		// 多作者体系（Spec 38）
		Author: strings.TrimSpace(req.Author),
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
