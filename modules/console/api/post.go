package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
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
	StellarContentMetadata
	Slug        string   `json:"slug"`
	Title       string   `json:"title"`
	Excerpt     string   `json:"excerpt"`
	Banner      string   `json:"banner"`
	Cover       string   `json:"cover"`
	Category    string   `json:"category"`
	Categories  []string `json:"categories"`
	CategoryIDs []int64  `json:"category_ids,omitempty"`
	TopicID     int64    `json:"topic_id"`
	Topic       string   `json:"topic"`
	Tags        string   `json:"tags"`
	ContentMD   string   `json:"content_md"`
	Status      string   `json:"status"`
	Pinned      bool     `json:"pinned"`
	Password    string   `json:"password"`
	PublishAt   string   `json:"publish_at"`
	Poster      *posterJ `json:"poster,omitempty"`
	// 文章级 SEO（Spec 42）
	MetaDescription string `json:"meta_description"`
	MetaKeywords    string `json:"meta_keywords"`
	AILabel         string `json:"ai_label"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
	Views           int64  `json:"views"`
	HasDraft        bool   `json:"has_draft"`
	// AI 文章摘要（Spec 53）：后台预生成的导读摘要，空=无
	AISummary string `json:"ai_summary"`
	// Draft 草稿头内容（整份编辑表单 JSON），仅详情接口且有草稿时返回。
	Draft json.RawMessage `json:"draft,omitempty"`
	// 多作者体系：Authors 按选择顺序返回；Author 保留第一位作者，兼容旧客户端。
	Authors []string `json:"authors"`
	Author  string   `json:"author"`
}

type posterJ struct {
	Headline string `json:"headline"`
	Topic    string `json:"topic"`
	Caption  string `json:"caption"`
	Color    string `json:"color"`
}

func toPostJSON(p *store.Post, views int64) *postJSON {
	pj := &postJSON{
		StellarContentMetadata: StellarContentMetadata{p.StellarVersion, p.StellarConfig, p.BlogExtensions},
		Slug:                   p.Slug,
		Title:                  p.Title,
		Excerpt:                p.Excerpt,
		Banner:                 p.Banner,
		Cover:                  p.Cover,
		Category:               p.Category,
		Categories:             append([]string(nil), p.Categories...),
		TopicID:                p.TopicID,
		Topic:                  p.Topic,
		Tags:                   p.Tags,
		ContentMD:              p.ContentMD,
		Status:                 p.Status,
		Pinned:                 p.Pinned,
		Password:               p.Password,
		MetaDescription:        p.MetaDescription,
		MetaKeywords:           p.MetaKeywords,
		AILabel:                p.AILabel,
		CreatedAt:              p.CreatedAt.Format(time.RFC3339),
		UpdatedAt:              p.UpdatedAt.Format(time.RFC3339),
		Views:                  views,
		HasDraft:               strings.TrimSpace(p.DraftJSON) != "",
		Authors:                normalizedPostAuthors(p),
		Author:                 p.Author,
		AISummary:              p.AISummary,
	}
	if !p.PublishAt.IsZero() {
		pj.PublishAt = p.PublishAt.UTC().Format("2006-01-02T15:04")
	}
	if pd := store.DecodePoster(p.Poster); pd != nil {
		pj.Poster = &posterJ{Headline: pd.Headline, Topic: pd.Topic, Caption: pd.Caption, Color: pd.Color}
	}
	return pj
}

// postListJSON 文章列表响应模型。列表页不需要正文、阅读口令或草稿内容，
// 与详情用的 postJSON 分开，避免大字段随每次分页/搜索请求返回。
type postListJSON struct {
	StellarContentMetadata
	Slug            string   `json:"slug"`
	Title           string   `json:"title"`
	Excerpt         string   `json:"excerpt"`
	Banner          string   `json:"banner"`
	Cover           string   `json:"cover"`
	Category        string   `json:"category"`
	Categories      []string `json:"categories"`
	TopicID         int64    `json:"topic_id"`
	Topic           string   `json:"topic"`
	Tags            string   `json:"tags"`
	Status          string   `json:"status"`
	Pinned          bool     `json:"pinned"`
	PublishAt       string   `json:"publish_at"`
	Poster          *posterJ `json:"poster,omitempty"`
	MetaDescription string   `json:"meta_description"`
	MetaKeywords    string   `json:"meta_keywords"`
	AILabel         string   `json:"ai_label"`
	CreatedAt       string   `json:"created_at"`
	UpdatedAt       string   `json:"updated_at"`
	Views           int64    `json:"views"`
	HasDraft        bool     `json:"has_draft"`
	AISummary       string   `json:"ai_summary"`
	Authors         []string `json:"authors"`
	Author          string   `json:"author"`
}

func toPostListJSON(p *store.Post) *postListJSON {
	status := p.Status
	if p.DeletedAt != "" {
		status = "trashed"
	}
	pj := &postListJSON{
		StellarContentMetadata: StellarContentMetadata{p.StellarVersion, p.StellarConfig, p.BlogExtensions},
		Slug:                   p.Slug,
		Title:                  p.Title,
		Excerpt:                p.Excerpt,
		Banner:                 p.Banner,
		Cover:                  p.Cover,
		Category:               p.Category,
		Categories:             append([]string(nil), p.Categories...),
		TopicID:                p.TopicID,
		Topic:                  p.Topic,
		Tags:                   p.Tags,
		Status:                 status,
		Pinned:                 p.Pinned,
		MetaDescription:        p.MetaDescription,
		MetaKeywords:           p.MetaKeywords,
		AILabel:                p.AILabel,
		CreatedAt:              p.CreatedAt.Format(time.RFC3339),
		UpdatedAt:              p.UpdatedAt.Format(time.RFC3339),
		Views:                  p.Views,
		HasDraft:               strings.TrimSpace(p.DraftJSON) != "",
		AISummary:              p.AISummary,
		Authors:                normalizedPostAuthors(p),
		Author:                 p.Author,
	}
	if !p.PublishAt.IsZero() {
		pj.PublishAt = p.PublishAt.UTC().Format("2006-01-02T15:04")
	}
	if pd := store.DecodePoster(p.Poster); pd != nil {
		pj.Poster = &posterJ{Headline: pd.Headline, Topic: pd.Topic, Caption: pd.Caption, Color: pd.Color}
	}
	return pj
}

type postListResponse struct {
	Items []*postListJSON `json:"items"`
	Total int64           `json:"total"`
	Page  int             `json:"page"`
	Size  int             `json:"size"`
}

var (
	errTopicIDInvalid = errors.New("invalid topic_id")
	errTopicNotFound  = errors.New("topic not found")
)

// decodeTopicID distinguishes an omitted topic_id from an explicit null/0 so
// old clients can keep their existing association while new clients can clear it.
func decodeTopicID(raw json.RawMessage) (provided bool, id int64, err error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return false, 0, nil
	}
	if bytes.Equal(raw, []byte("null")) {
		return true, 0, nil
	}
	if err := json.Unmarshal(raw, &id); err != nil || id < 0 {
		return true, 0, errTopicIDInvalid
	}
	return true, id, nil
}

// 严格文章以 collection 原文为唯一归属真值；旧版本保留既有宿主关系语义。
func (h *Handler) normalizePostTopic(req *postRequest, fallbackID int64) error {
	provided, id, err := decodeTopicID(req.TopicID)
	if err != nil {
		return err
	}
	if req.StellarVersion == 2 {
		var raw struct {
			Collection *struct {
				Profile string `json:"profile"`
				ID      string `json:"id"`
			} `json:"collection"`
		}
		if err := json.Unmarshal([]byte(req.StellarConfig), &raw); err != nil {
			return errTopicMembershipConfig
		}
		canonicalID := int64(0)
		if raw.Collection != nil && raw.Collection.Profile == "topic" {
			topic, err := h.store.GetTopicBySlug(raw.Collection.ID)
			if err != nil {
				return err
			}
			if topic == nil {
				return errTopicMembershipMissing
			}
			canonicalID = topic.ID
		}
		if provided && id != canonicalID {
			if id > 0 {
				if topic, err := h.store.GetTopicByID(id); err != nil {
					return err
				} else if topic == nil {
					return errTopicNotFound
				}
			}
			return errTopicMembershipConflict
		}
		id = canonicalID
		provided = true
	}
	if !provided {
		id = fallbackID
		if id == 0 && strings.TrimSpace(req.Topic) != "" {
			if legacy, lookupErr := h.store.GetTopicByName(strings.TrimSpace(req.Topic)); lookupErr == nil && legacy != nil {
				id = legacy.ID
			}
		}
	}
	if id <= 0 {
		req.TopicID = json.RawMessage("0")
		req.Topic = ""
		return nil
	}
	topic, err := h.store.GetTopicByID(id)
	if err != nil {
		return err
	}
	if topic == nil {
		return errTopicNotFound
	}
	req.TopicID = json.RawMessage(strconv.FormatInt(topic.ID, 10))
	req.Topic = topic.Name
	return nil
}

var errTopicMembershipMissing = errors.New("strict topic collection missing")

var errTopicMembershipConfig = errors.New("invalid strict collection config")

var errTopicMembershipConflict = errors.New("topic_id conflicts with strict collection")

func failPostTopic(c *gin.Context, err error) bool {
	switch {
	case errors.Is(err, errTopicMembershipMissing):
		Fail(c, http.StatusBadRequest, "collection_not_found: 严格配置所属专栏不存在")
	case errors.Is(err, errTopicMembershipConfig):
		Fail(c, http.StatusBadRequest, "严格配置 JSON 原文无效")
	case errors.Is(err, errTopicMembershipConflict):
		Fail(c, http.StatusBadRequest, "topic_id 与严格配置 collection 归属冲突，请以原文为准")
	case errors.Is(err, errTopicIDInvalid):
		Fail(c, http.StatusBadRequest, "无效的 topic_id")
	case errors.Is(err, errTopicNotFound):
		Fail(c, http.StatusNotFound, "专栏不存在")
	default:
		Fail(c, http.StatusInternalServerError, "读取专栏失败: "+err.Error())
	}
	return true
}

// listPosts GET /api/v1/posts：文章列表（支持 status/category/tag/search 筛选 + 分页）。
func (h *Handler) listPosts(c *gin.Context) {
	status := c.Query("status")
	category := c.Query("category")
	tag := c.Query("tag")
	search := c.Query("search")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}

	// Spec 62：筛选、搜索与分页下推 SQL（ListFiltered 单次 COUNT + 单次分页查询，
	// 使用轻量 postListCols 投影，替代原「List() 全表 + 内存过滤 + 逐篇 GetViews」的 N+1 模式）。
	var authorID *int64
	if !contextHasPermission(c, store.PermissionContentManageAny) {
		uid := c.GetInt64("userID")
		authorID = &uid
	}
	posts, total, err := h.store.ListFilteredByAuthor(authorID, status, category, tag, search, page, size)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	items := make([]*postListJSON, 0, len(posts))
	for _, p := range posts {
		items = append(items, toPostListJSON(p))
	}

	c.JSON(http.StatusOK, postListResponse{Items: items, Total: total, Page: page, Size: size})
}

// getPost GET /api/v1/posts/:slug：文章详情。
func (h *Handler) getPost(c *gin.Context) {
	slug := c.Param("slug")
	if !h.requirePostOwnership(c, slug) {
		return
	}
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
	projectPostForm(&req)
	if err := h.normalizePostTopic(&req, 0); err != nil {
		failPostTopic(c, err)
		return
	}
	if err := h.normalizePostCategories(&req); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.normalizePostAuthorsRequest(c, &req, nil); err != nil {
		Fail(c, http.StatusBadRequest, "作者列表格式错误: "+err.Error())
		return
	}
	// 空 slug 时从标题自动生成（Spec Task 6），并确保唯一；用户指定 slug 则检查冲突
	slugVal := strings.TrimSpace(req.Slug)
	if slugVal == "" {
		slugVal = h.ensureUniqueSlug(generateSlug(req.Title))
		req.Slug = slugVal
	} else if strings.ContainsAny(slugVal, "/\\.") || slugVal != filepath.Base(filepath.FromSlash(slugVal)) {
		// 安全审计 2026-08-16：显式 slug 此前不校验字符集——含 ../ 的 slug 会随
		// OG 缓存写入（CachePath 直拼）落到缓存目录外。自动生成路径走 reNonSlugChar
		// 已天然安全，这里补用户显式指定路径的白名单校验。
		Fail(c, http.StatusBadRequest, "slug 只允许字母/数字/连字符，且不含路径分隔符")
		return
	} else if existing, _ := h.store.GetBySlug(slugVal); existing != nil {
		Fail(c, http.StatusConflict, "slug 已存在: "+slugVal)
		return
	}

	if err := h.checkEditorCollection(req.StellarContentMetadata, "post", ""); err != nil {
		failEditorStellar(c, err)
		return
	}
	contentHTML, err := h.renderEditorStellar(req.ContentMD, req.StellarContentMetadata, "post")
	if err != nil {
		failEditorStellar(c, err)
		return
	}

	p := reqToPost(&req, contentHTML)
	// 默认封面兜底：无封面时按 slug 生成稳定的 picsum seed 图
	if p.Cover == "" {
		p.Cover = defaultCover(req.Slug)
	}
	if _, err := h.store.Create(p); err != nil {
		if errors.Is(err, store.ErrPostTopicNotFound) {
			failPostTopic(c, errTopicNotFound)
			return
		}
		Fail(c, http.StatusInternalServerError, "创建失败: "+err.Error())
		return
	}
	h.publishContentChanged()
	c.JSON(http.StatusCreated, toPostJSON(p, 0))
}

// updatePost PUT /api/v1/posts/:slug：更新文章。
func (h *Handler) updatePost(c *gin.Context) {
	slug := c.Param("slug")
	if !h.requirePostOwnership(c, slug) {
		return
	}
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
	projectPostForm(&req)
	if err := h.normalizePostTopic(&req, existing.TopicID); err != nil {
		failPostTopic(c, err)
		return
	}
	if len(req.Categories) == 0 && len(req.CategoryIDs) == 0 && strings.TrimSpace(req.Category) == "" {
		req.Categories = append([]string(nil), existing.Categories...)
		req.Category = existing.Category
	}
	if err := h.normalizePostCategories(&req); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.normalizePostAuthorsRequest(c, &req, existing); err != nil {
		Fail(c, http.StatusBadRequest, "作者列表格式错误: "+err.Error())
		return
	}

	// slug 变更时检查唯一性
	newSlug := strings.TrimSpace(req.Slug)
	if newSlug == "" {
		newSlug = slug
	}
	// 安全审计 2026-08-16：显式 slug 补路径字符校验（防 OG 缓存写入穿越，见 createPost）。
	if strings.ContainsAny(newSlug, "/\\.") || newSlug != filepath.Base(filepath.FromSlash(newSlug)) {
		Fail(c, http.StatusBadRequest, "slug 只允许字母/数字/连字符，且不含路径分隔符")
		return
	}
	if newSlug != slug {
		if other, _ := h.store.GetBySlug(newSlug); other != nil {
			Fail(c, http.StatusConflict, "slug 已被占用: "+newSlug)
			return
		}
	}

	if err := h.checkEditorCollection(req.StellarContentMetadata, "post", ""); err != nil {
		failEditorStellar(c, err)
		return
	}
	contentHTML, err := h.renderEditorStellar(req.ContentMD, req.StellarContentMetadata, "post")
	if err != nil {
		failEditorStellar(c, err)
		return
	}

	p := reqToPost(&req, contentHTML)
	p.Slug = newSlug
	// 发布路径默认封面兜底
	if p.Status == store.StatusPublished && p.Cover == "" {
		p.Cover = defaultCover(newSlug)
	}

	if err := h.savePostWithSnapshot(existing, p, newSlug); err != nil {
		if errors.Is(err, store.ErrPostTopicNotFound) {
			failPostTopic(c, errTopicNotFound)
			return
		}
		if errors.Is(err, store.ErrPostSlugConflict) {
			Fail(c, http.StatusConflict, "slug 已被占用: "+newSlug)
			return
		}
		Fail(c, http.StatusInternalServerError, "更新失败: "+err.Error())
		return
	}

	// Spec 62：views 复用 existing 查询投影；slug 变更时新记录阅读量为 0（无需二次 GetViews）。
	views := existing.Views
	if newSlug != existing.Slug {
		views = 0
	}
	h.publishContentChanged()
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
	if !h.requirePostOwnership(c, slug) {
		return
	}
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
	projectPostForm(&req)
	if err := h.normalizePostTopic(&req, existing.TopicID); err != nil {
		failPostTopic(c, err)
		return
	}
	if err := h.normalizePostAuthorsRequest(c, &req, existing); err != nil {
		Fail(c, http.StatusBadRequest, "作者列表格式错误: "+err.Error())
		return
	}
	if err := h.checkEditorCollection(req.StellarContentMetadata, "post", ""); err != nil {
		failEditorStellar(c, err)
		return
	}
	if _, err := h.renderEditorStellar(req.ContentMD, req.StellarContentMetadata, "post"); err != nil {
		failEditorStellar(c, err)
		return
	}
	b, err := json.Marshal(req)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "草稿序列化失败: "+err.Error())
		return
	}
	if err := h.store.SaveDraft(slug, string(b)); err != nil {
		if errors.Is(err, store.ErrPostTopicNotFound) {
			failPostTopic(c, errTopicNotFound)
			return
		}
		Fail(c, http.StatusInternalServerError, "保存草稿失败: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "has_draft": true})
}

// publishPost POST /api/v1/posts/:slug/publish：应用草稿并发布。
// 请求体与 updatePost 相同（整份表单）；强制 status=published，成功后清空草稿头。
func (h *Handler) publishPost(c *gin.Context) {
	slug := c.Param("slug")
	if !h.requirePostOwnership(c, slug) {
		return
	}
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
	projectPostForm(&req)
	if err := h.normalizePostTopic(&req, existing.TopicID); err != nil {
		failPostTopic(c, err)
		return
	}
	if err := h.normalizePostAuthorsRequest(c, &req, existing); err != nil {
		Fail(c, http.StatusBadRequest, "作者列表格式错误: "+err.Error())
		return
	}
	req.Status = store.StatusPublished

	newSlug := strings.TrimSpace(req.Slug)
	if newSlug == "" {
		newSlug = slug
	}
	// 安全审计 2026-08-16：显式 slug 补路径字符校验（防 OG 缓存写入穿越，见 createPost）。
	if strings.ContainsAny(newSlug, "/\\.") || newSlug != filepath.Base(filepath.FromSlash(newSlug)) {
		Fail(c, http.StatusBadRequest, "slug 只允许字母/数字/连字符，且不含路径分隔符")
		return
	}
	if newSlug != slug {
		if other, _ := h.store.GetBySlug(newSlug); other != nil {
			Fail(c, http.StatusConflict, "slug 已被占用: "+newSlug)
			return
		}
	}

	if err := h.checkEditorCollection(req.StellarContentMetadata, "post", ""); err != nil {
		failEditorStellar(c, err)
		return
	}
	contentHTML, err := h.renderEditorStellar(req.ContentMD, req.StellarContentMetadata, "post")
	if err != nil {
		failEditorStellar(c, err)
		return
	}

	p := reqToPost(&req, contentHTML)
	p.Slug = newSlug
	if p.Cover == "" {
		p.Cover = defaultCover(newSlug)
	}

	if err := h.savePostWithSnapshot(existing, p, newSlug); err != nil {
		if errors.Is(err, store.ErrPostTopicNotFound) {
			failPostTopic(c, errTopicNotFound)
			return
		}
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
	h.publishContentChanged()
	c.JSON(http.StatusOK, toPostJSON(p, views))
}

// discardPostDraft DELETE /api/v1/posts/:slug/draft：放弃草稿头。
func (h *Handler) discardPostDraft(c *gin.Context) {
	slug := c.Param("slug")
	if !h.requirePostOwnership(c, slug) {
		return
	}
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
	// 存储层在事务内重新读取旧正文并完成文章更新、revision 写入和 slug 迁移。
	// existing 仍用于保持当前 API 的调用和响应语义，但不作为快照数据源。
	p.ID = existing.ID
	p.CreatedAt = existing.CreatedAt
	return h.store.UpdateSlugWithRevision(existing.Slug, newSlug, p)
}

// deletePost DELETE /api/v1/posts/:slug：软删除文章（移入回收站）。
func (h *Handler) deletePost(c *gin.Context) {
	slug := c.Param("slug")
	if !h.requirePostOwnership(c, slug) {
		return
	}
	if err := h.store.Delete(slug); err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	h.publishContentChanged()
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// restorePost POST /api/v1/posts/:slug/restore：从回收站恢复文章。
func (h *Handler) restorePost(c *gin.Context) {
	slug := c.Param("slug")
	if !h.requirePostOwnership(c, slug) {
		return
	}
	if err := h.store.RestorePost(slug); err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	h.publishContentChanged()
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// purgePost DELETE /api/v1/posts/:slug/purge：彻底删除文章（物理删除）。
func (h *Handler) purgePost(c *gin.Context) {
	slug := c.Param("slug")
	if !h.requirePostOwnership(c, slug) {
		return
	}
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
	if !h.requirePostOwnership(c, slug) {
		return
	}
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
	h.publishContentChanged()
	c.JSON(http.StatusOK, gin.H{"ok": true, "ai_summary": summary})
}

// --- 辅助 ---

type postRequest struct {
	StellarContentMetadata
	Slug        string          `json:"slug"`
	Title       string          `json:"title"`
	Excerpt     string          `json:"excerpt"`
	Banner      string          `json:"banner"`
	Cover       string          `json:"cover"`
	Category    string          `json:"category"`
	Categories  []string        `json:"categories"`
	CategoryIDs []int64         `json:"category_ids,omitempty"`
	TopicID     json.RawMessage `json:"topic_id,omitempty"`
	Topic       string          `json:"topic"`
	Tags        string          `json:"tags"`
	ContentMD   string          `json:"content_md"`
	Status      string          `json:"status"`
	Pinned      bool            `json:"pinned"`
	Password    string          `json:"password"`
	PublishAt   string          `json:"publish_at"`
	Poster      *struct {
		Headline string `json:"headline"`
		Topic    string `json:"topic"`
		Caption  string `json:"caption"`
		Color    string `json:"color"`
	} `json:"poster"`
	// 文章级 SEO（Spec 42）
	MetaDescription string `json:"meta_description"`
	MetaKeywords    string `json:"meta_keywords"`
	// 多作者体系：Authors 明确提交时替换完整作者列表；Author 为旧客户端兼容字段。
	Authors    json.RawMessage `json:"authors,omitempty"`
	Author     string          `json:"author"`
	authorRefs []string
	authorIDs  []int64
	// AI 成分标签档位（1.41 ai_label）：空=不渲染；空串提交时 store 层保留旧值
	AILabel string `json:"ai_label"`
}

func reqToPost(req *postRequest, contentHTML string) *store.Post {
	status := req.Status
	if status != store.StatusDraft && status != store.StatusPublished {
		status = store.StatusDraft
	}
	p := &store.Post{
		StellarVersion: req.StellarVersion, StellarConfig: req.StellarConfig, BlogExtensions: req.BlogExtensions,
		Slug:        strings.TrimSpace(req.Slug),
		Title:       strings.TrimSpace(req.Title),
		Excerpt:     strings.TrimSpace(req.Excerpt),
		Banner:      strings.TrimSpace(req.Banner),
		Cover:       strings.TrimSpace(req.Cover),
		Category:    strings.TrimSpace(req.Category),
		Categories:  append([]string(nil), req.Categories...),
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
		// 多作者体系（normalizePostAuthorsRequest 已在 handler 中统一填充）。
		Authors:   normalizeRequestAuthorRefs(req),
		AuthorIDs: append([]int64(nil), req.authorIDs...),
		Author:    strings.TrimSpace(req.Author),
		// AI 成分标签（1.41）：空串经 store NULLIF 保留旧值
		AILabel: strings.TrimSpace(req.AILabel),
	}
	if _, topicID, err := decodeTopicID(req.TopicID); err == nil {
		p.TopicID = topicID
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

// normalizePostCategories resolves category_ids to paths and keeps the legacy
// category field synchronized with the first submitted category.
func (h *Handler) normalizePostCategories(req *postRequest) error {
	if len(req.CategoryIDs) > 0 {
		paths := make([]string, 0, len(req.CategoryIDs))
		seen := map[int64]bool{}
		for _, id := range req.CategoryIDs {
			if id <= 0 || seen[id] {
				continue
			}
			seen[id] = true
			n, err := h.store.GetCategoryByID(id)
			if err != nil {
				return fmt.Errorf("分类不存在: %d", id)
			}
			paths = append(paths, n.Path)
		}
		req.Categories = paths
	}
	if len(req.Categories) == 0 && strings.TrimSpace(req.Category) != "" {
		req.Categories = []string{strings.TrimSpace(req.Category)}
	}
	for i, path := range req.Categories {
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) == 0 || strings.TrimSpace(path) == "" {
			return fmt.Errorf("分类路径不能为空")
		}
		for _, part := range parts {
			if strings.TrimSpace(part) == "" {
				return fmt.Errorf("分类路径不合法: %q", path)
			}
		}
		req.Categories[i] = strings.Trim(path, " /")
	}
	if len(req.Categories) > 0 {
		req.Category = req.Categories[0]
	}
	return nil
}

func normalizeRequestAuthorRefs(req *postRequest) []string {
	if len(req.authorRefs) > 0 {
		return append([]string(nil), req.authorRefs...)
	}
	var refs []string
	if len(bytes.TrimSpace(req.Authors)) > 0 {
		_ = json.Unmarshal(req.Authors, &refs)
	}
	return normalizeAuthorIDs(refs)
}

func normalizedPostAuthors(p *store.Post) []string {
	ids := normalizeAuthorIDs(p.Authors)
	if len(ids) == 0 && strings.TrimSpace(p.Author) != "" {
		ids = []string{strings.TrimSpace(p.Author)}
	}
	return ids
}

func normalizeAuthorIDs(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

// normalizePostAuthorsRequest 对管理员兼容 authors/author 请求；普通作者新建时绑定自己，
// 更新时保留已授权文章的完整署名关系，忽略客户端提供的作者替换。
func (h *Handler) normalizePostAuthorsRequest(c *gin.Context, req *postRequest, existing *store.Post) error {
	if !contextHasPermission(c, store.PermissionContentManageAny) {
		if existing == nil {
			return forceCurrentAuthor(c, req)
		}
		// 归属已由入口校验。保留数据库 ID，不能把数字用户名重新解释为另一个用户 ID。
		if !contextHasPermission(c, store.PermissionContentWrite) || len(existing.AuthorIDs) == 0 {
			return fmt.Errorf("cannot preserve authorized post authors")
		}
		req.authorIDs = append([]int64(nil), existing.AuthorIDs...)
		req.authorRefs = make([]string, 0, len(req.authorIDs))
		for _, id := range req.authorIDs {
			req.authorRefs = append(req.authorRefs, strconv.FormatInt(id, 10))
		}
		req.Author = req.authorRefs[0]
		data, err := json.Marshal(req.authorRefs)
		req.Authors = data
		return err
	}

	raw := bytes.TrimSpace(req.Authors)
	var ids []string
	provided := len(raw) > 0
	if provided {
		if bytes.Equal(raw, []byte("null")) {
			ids = []string{}
		} else if err := json.Unmarshal(raw, &ids); err != nil {
			return err
		}
	}
	if !provided {
		if legacy := strings.TrimSpace(req.Author); legacy != "" {
			ids = []string{legacy}
		} else if existing != nil {
			// 未请求修改署名时，管理员也保留稳定 ID，避免数字用户名歧义。
			for _, id := range existing.AuthorIDs {
				ids = append(ids, strconv.FormatInt(id, 10))
			}
		}
	}
	ids = normalizeAuthorIDs(ids)
	req.authorRefs = ids
	req.authorIDs = nil
	req.Author = ""
	if len(ids) > 0 {
		req.Author = ids[0]
	}
	b, err := json.Marshal(ids)
	if err != nil {
		return err
	}
	req.Authors = b
	return nil
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
