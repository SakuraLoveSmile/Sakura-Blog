package api

import (
	"encoding/json"
	"fmt"
	"log"
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
	Banner    string   `json:"banner"`
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
	// AI 文章摘要（Spec 53）：后台预生成的导读摘要，空=无
	AISummary string `json:"ai_summary"`
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
		Banner:          p.Banner,
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
		AISummary:       p.AISummary,
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
			Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		c.JSON(http.StatusOK, gin.H{"items": posts, "total": len(posts)})
		return
	}

	// Spec 62：筛选与分页下推 SQL（ListFiltered 单次 COUNT + 单次分页查询，views 随 listCols 投影带出，
	// 替代原「List() 全表 + 内存过滤 + 逐篇 GetViews」的 N+1 模式）
	posts, total, err := h.store.ListFiltered(status, category, tag, page, size)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	items := make([]*postJSON, 0, len(posts))
	for _, p := range posts {
		items = append(items, toPostJSON(p, p.Views))
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
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if p == nil {
		Fail(c, http.StatusNotFound, "文章不存在")
		return
	}
	// Spec 62：views 已随 GetBySlug 的 listCols 投影带出，无需二次 GetViews 查询。
	pj := toPostJSON(p, p.Views)
	if pj.HasDraft {
		pj.Draft = json.RawMessage(p.DraftJSON)
	}
	c.JSON(http.StatusOK, pj)
}

// createPost POST /api/v1/posts：创建文章。
func (h *Handler) createPost(c *gin.Context) {
	var req postRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "请求格式错误: "+err.Error())
		return
	}
	// 空 slug 时从标题自动生成（Spec Task 6），并确保唯一；用户指定 slug 则检查冲突
	slugVal := strings.TrimSpace(req.Slug)
	if slugVal == "" {
		slugVal = h.ensureUniqueSlug(generateSlug(req.Title))
		req.Slug = slugVal
	} else if existing, _ := h.store.GetBySlug(slugVal); existing != nil {
		Fail(c, http.StatusConflict, "slug 已存在: "+slugVal)
		return
	}

	contentHTML, err := h.posts.Render(req.ContentMD)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "Markdown 渲染失败: "+err.Error())
		return
	}

	p := reqToPost(&req, contentHTML)
	// 默认封面兜底：无封面时按 slug 生成稳定的 picsum seed 图
	if p.Cover == "" {
		p.Cover = defaultCover(req.Slug)
	}
	if _, err := h.store.Create(p); err != nil {
		Fail(c, http.StatusInternalServerError, "创建失败: "+err.Error())
		return
	}
	c.JSON(http.StatusCreated, toPostJSON(p, 0))
}

// updatePost PUT /api/v1/posts/:slug：更新文章。
func (h *Handler) updatePost(c *gin.Context) {
	slug := c.Param("slug")
	existing, err := h.store.GetBySlug(slug)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if existing == nil {
		Fail(c, http.StatusNotFound, "文章不存在")
		return
	}

	var req postRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "请求格式错误: "+err.Error())
		return
	}

	// slug 变更时检查唯一性
	newSlug := strings.TrimSpace(req.Slug)
	if newSlug == "" {
		newSlug = slug
	}
	if newSlug != slug {
		if other, _ := h.store.GetBySlug(newSlug); other != nil {
			Fail(c, http.StatusConflict, "slug 已被占用: "+newSlug)
			return
		}
	}

	contentHTML, err := h.posts.Render(req.ContentMD)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "Markdown 渲染失败: "+err.Error())
		return
	}

	p := reqToPost(&req, contentHTML)
	p.Slug = newSlug
	// 发布路径默认封面兜底
	if p.Status == store.StatusPublished && p.Cover == "" {
		p.Cover = defaultCover(newSlug)
	}

	if err := h.savePostWithSnapshot(existing, p, newSlug); err != nil {
		Fail(c, http.StatusInternalServerError, "更新失败: "+err.Error())
		return
	}

	// Spec 62：views 复用 existing 查询投影；slug 变更时新记录阅读量为 0（无需二次 GetViews）。
	views := existing.Views
	if newSlug != existing.Slug {
		views = 0
	}
	c.JSON(http.StatusOK, toPostJSON(p, views))
}

// defaultCover 默认封面 URL：返回本地占位图，避免外部图片服务（picsum/unsplash）不可达导致控制台报 ERR_ABORTED。
// 历史上使用 picsum.photos seed 模式（https://picsum.photos/seed/{slug}/1200/675），
// 但该服务在部分网络环境下不稳定，统一回退到本地 icon-512 占位图；
// 如需美观的个性化封面，请通过 Console 编辑表单显式上传或填写可靠 CDN 地址。
func defaultCover(slug string) string {
	return "/assets/img/icon-512.png"
}

// savePostDraft PUT /api/v1/posts/:slug/draft：保存草稿头（整份编辑表单）。
// 不渲染、不产 revision、不影响线上内容；列表小绿点据此显示。
func (h *Handler) savePostDraft(c *gin.Context) {
	slug := c.Param("slug")
	existing, err := h.store.GetBySlug(slug)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if existing == nil {
		Fail(c, http.StatusNotFound, "文章不存在")
		return
	}

	var req postRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "请求格式错误: "+err.Error())
		return
	}
	b, err := json.Marshal(req)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "草稿序列化失败: "+err.Error())
		return
	}
	if err := h.store.SaveDraft(slug, string(b)); err != nil {
		Fail(c, http.StatusInternalServerError, "保存草稿失败: "+err.Error())
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
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if existing == nil {
		Fail(c, http.StatusNotFound, "文章不存在")
		return
	}

	var req postRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "请求格式错误: "+err.Error())
		return
	}
	req.Status = store.StatusPublished

	newSlug := strings.TrimSpace(req.Slug)
	if newSlug == "" {
		newSlug = slug
	}
	if newSlug != slug {
		if other, _ := h.store.GetBySlug(newSlug); other != nil {
			Fail(c, http.StatusConflict, "slug 已被占用: "+newSlug)
			return
		}
	}

	contentHTML, err := h.posts.Render(req.ContentMD)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "Markdown 渲染失败: "+err.Error())
		return
	}

	p := reqToPost(&req, contentHTML)
	p.Slug = newSlug
	if p.Cover == "" {
		p.Cover = defaultCover(newSlug)
	}

	if err := h.savePostWithSnapshot(existing, p, newSlug); err != nil {
		Fail(c, http.StatusInternalServerError, "发布失败: "+err.Error())
		return
	}
	// 草稿已应用，清空草稿头（slug 变更时新记录本就无草稿，兼顾清理旧 slug）
	if err := h.store.ClearDraft(newSlug); err != nil {
		log.Printf("clear draft %s: %v", newSlug, err)
	}
	if newSlug != slug {
		if err := h.store.ClearDraft(slug); err != nil {
			log.Printf("clear draft %s: %v", slug, err)
		}
	}

	// Spec 62：views 复用 existing 查询投影；slug 变更时新记录阅读量为 0（无需二次 GetViews）。
	views := existing.Views
	if newSlug != existing.Slug {
		views = 0
	}
	h.notifySEOPublished(newSlug)
	c.JSON(http.StatusOK, toPostJSON(p, views))
}

// discardPostDraft DELETE /api/v1/posts/:slug/draft：放弃草稿头。
func (h *Handler) discardPostDraft(c *gin.Context) {
	slug := c.Param("slug")
	if err := h.store.ClearDraft(slug); err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// savePostWithSnapshot 保存文章并在成功后把「保存前」的旧版本写入 revisions（Spec 22）。
// existing 为保存前查出的旧记录；p 为新内容（p.Slug 已设）；newSlug 为目标 slug。
// 内容无变化（content_md + title 均相同）时不产生空版本。
func (h *Handler) savePostWithSnapshot(existing, p *store.Post, newSlug string) error {
	// Spec 52：请求体不携带 topic_id（postRequest 无该字段），更新/重建时保留原专栏关联；
	// 关联变更由专门入口负责（届时放开）。
	p.TopicID = existing.TopicID
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
		if p.AISummary == "" {
			p.AISummary = existing.AISummary
		}
		if _, err := h.store.Create(p); err != nil {
			return err
		}
		// 同步迁移版本归属
		if err := h.store.UpdateRevisionSlug(existing.Slug, newSlug); err != nil {
			log.Printf("update revision slug %s -> %s: %v", existing.Slug, newSlug, err)
		}
	} else {
		if err := h.store.Update(existing.Slug, p); err != nil {
			return err
		}
	}
	h.snapshotRevision(existing, p)
	return nil
}

// snapshotRevision 把保存前的旧版本写入 revisions；内容无变化则跳过。
// 版本快照失败记日志（告警级：历史版本丢失不可自动恢复）。
func (h *Handler) snapshotRevision(old, neu *store.Post) {
	if old.ContentMD == neu.ContentMD && old.Title == neu.Title {
		return
	}
	if _, err := h.store.CreateRevision(&store.Revision{
		PostSlug:  neu.Slug,
		Title:     old.Title,
		Excerpt:   old.Excerpt,
		ContentMD: old.ContentMD,
		Tags:      old.Tags,
		Category:  old.Category,
		Status:    old.Status,
	}); err != nil {
		log.Printf("create revision for %s: %v", neu.Slug, err)
	}
}

// deletePost DELETE /api/v1/posts/:slug：软删除文章（移入回收站）。
func (h *Handler) deletePost(c *gin.Context) {
	slug := c.Param("slug")
	if err := h.store.Delete(slug); err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// restorePost POST /api/v1/posts/:slug/restore：从回收站恢复文章。
func (h *Handler) restorePost(c *gin.Context) {
	slug := c.Param("slug")
	if err := h.store.RestorePost(slug); err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// purgePost DELETE /api/v1/posts/:slug/purge：彻底删除文章（物理删除）。
func (h *Handler) purgePost(c *gin.Context) {
	slug := c.Param("slug")
	if err := h.store.PurgePost(slug); err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	// 连带清理版本快照（Spec 22）
	h.store.DeleteRevisionsByPost(slug)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// updateAISummary PUT /api/v1/posts/:slug/ai-summary：保存/清空 AI 摘要（Spec 53）。
// 请求体 {"summary": "..."}；传空串即清空。独立于主保存链路，避免常规保存误清。
func (h *Handler) updateAISummary(c *gin.Context) {
	slug := c.Param("slug")
	var req struct {
		Summary string `json:"summary"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "请求格式错误")
		return
	}
	summary := strings.TrimSpace(req.Summary)
	if runes := []rune(summary); len(runes) > 2000 {
		summary = string(runes[:2000])
	}
	if err := h.store.SetAISummary(slug, summary); err != nil {
		if strings.Contains(err.Error(), "no rows affected") {
			Fail(c, http.StatusNotFound, "文章不存在")
			return
		}
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "ai_summary": summary})
}

// --- 辅助 ---

type postRequest struct {
	Slug      string `json:"slug"`
	Title     string `json:"title"`
	Excerpt   string `json:"excerpt"`
	Banner    string `json:"banner"`
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
		Banner:      strings.TrimSpace(req.Banner),
		Cover:       strings.TrimSpace(req.Cover),
		Category:    strings.TrimSpace(req.Category),
		Topic:       strings.TrimSpace(req.Topic),
		Tags:        normalizeTags(req.Tags),
		ContentMD:   req.ContentMD,
		ContentHTML: contentHTML,
		Status:      status,
		Pinned:      req.Pinned,
		Password:    strings.TrimSpace(req.Password),
		PublishAt:   store.ParsePublishAtLocal(req.PublishAt),
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
