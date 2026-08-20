package api

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"

	"blog/internal/comments"
	tmpl "blog/internal/settings"
	"blog/internal/store"
)

const (
	patPrefix = "pat_"
	patWindow = time.Minute
	patLimit  = 60
)

type publicContent struct {
	store *store.Store
	site  *atomic.Pointer[tmpl.Site]
	mu    sync.Mutex
	hits  map[int64]patHit
}

type patHit struct {
	n  int
	at time.Time
}

func NewPublicContent(st *store.Store, site *atomic.Pointer[tmpl.Site]) *publicContent {
	return &publicContent{store: st, site: site, hits: map[int64]patHit{}}
}

func (h *publicContent) Register(r *gin.Engine) error {
	pub := r.Group("/api/public/v1", h.patAuth())
	pub.GET("/posts", h.listPosts)
	pub.GET("/posts/:slug", h.getPost)
	pub.GET("/pages", h.listPages)
	pub.GET("/categories", h.listCategories)
	pub.GET("/tags", h.listTags)
	pub.GET("/comments", h.listComments)
	return nil
}

func (h *publicContent) patAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := strings.TrimSpace(c.GetHeader("Authorization"))
		if !strings.HasPrefix(auth, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "缺少 Bearer 令牌"})
			return
		}
		plain := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
		if !strings.HasPrefix(plain, patPrefix) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "非法令牌格式"})
			return
		}
		pat, err := h.store.GetPATByHash(store.HashToken(plain))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "令牌校验失败"})
			return
		}
		if pat == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "令牌无效或已吊销"})
			return
		}
		if !pat.ExpiresAt.IsZero() && time.Now().After(pat.ExpiresAt) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "令牌已过期"})
			return
		}
		if !h.allowPAT(pat.ID) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "请求过于频繁，请稍后再试"})
			return
		}
		h.store.TouchPATLastUsed(pat.ID)
		c.Set("tokenID", pat.ID)
		c.Next()
	}
}

func (h *publicContent) allowPAT(id int64) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	now := time.Now()
	for key, hit := range h.hits {
		if now.Sub(hit.at) >= patWindow {
			delete(h.hits, key)
		}
	}
	hit := h.hits[id]
	if hit.at.IsZero() || now.Sub(hit.at) >= patWindow {
		h.hits[id] = patHit{n: 1, at: now}
		return true
	}
	if hit.n >= patLimit {
		return false
	}
	hit.n++
	h.hits[id] = hit
	return true
}

type publicPostJSON struct {
	Slug              string   `json:"slug"`
	Title             string   `json:"title"`
	Excerpt           string   `json:"excerpt"`
	Cover             string   `json:"cover"`
	Category          string   `json:"category"`
	Categories        []string `json:"categories"`
	Tags              []string `json:"tags"`
	Authors           []string `json:"authors"`
	Author            string   `json:"author,omitempty"`
	ContentMD         string   `json:"content_md,omitempty"`
	ContentHTML       string   `json:"content_html,omitempty"`
	PasswordProtected bool     `json:"password_protected"`
	CreatedAt         string   `json:"created_at"`
	UpdatedAt         string   `json:"updated_at"`
}

func (h *publicContent) listPosts(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "10"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 50 {
		size = 10
	}
	total, err := h.store.CountPublished()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	posts, err := h.store.ListPublishedPaged(page, size)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	items := make([]*publicPostJSON, 0, len(posts))
	for _, post := range posts {
		items = append(items, publicPost(post, false))
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total, "page": page, "size": size})
}

func (h *publicContent) getPost(c *gin.Context) {
	post, err := h.store.GetPublishedBySlug(c.Param("slug"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if post == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "文章不存在"})
		return
	}
	c.JSON(http.StatusOK, publicPost(post, true))
}

func (h *publicContent) listPages(c *gin.Context) {
	pages, err := h.store.ListPublishedPages()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	items := make([]gin.H, 0, len(pages))
	for _, page := range pages {
		items = append(items, gin.H{"slug": page.Slug, "title": page.Title, "content_md": page.ContentMD, "content_html": page.ContentHTML, "created_at": page.CreatedAt.Format(time.RFC3339), "updated_at": page.UpdatedAt.Format(time.RFC3339)})
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": len(items)})
}

func (h *publicContent) listCategories(c *gin.Context) {
	tree, err := h.store.CategoryTree()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	flat := store.FlattenCategoryTree(tree)
	items := make([]gin.H, 0, len(flat))
	for _, n := range flat {
		items = append(items, gin.H{"id": n.ID, "name": n.Name, "path": n.Path, "parent_id": n.ParentID, "count": n.Count, "children": n.Children})
	}
	c.JSON(http.StatusOK, gin.H{"tree": tree, "items": items, "total": len(items)})
}
func (h *publicContent) listTags(c *gin.Context) { h.listTaxonomy(c, h.store.CountByTag) }
func (h *publicContent) listTaxonomy(c *gin.Context, fn func() (map[string]int, error)) {
	values, err := fn()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	items := make([]taxonomyItem, 0, len(values))
	for name, count := range values {
		items = append(items, taxonomyItem{Name: name, Count: count})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Count != items[j].Count {
			return items[i].Count > items[j].Count
		}
		return items[i].Name < items[j].Name
	})
	c.JSON(http.StatusOK, gin.H{"items": items, "total": len(items)})
}

type taxonomyItem struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func (h *publicContent) listComments(c *gin.Context) {
	pageKey := strings.TrimSpace(c.Query("page_key"))
	if pageKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "page_key 不能为空"})
		return
	}
	list, err := h.store.ListCommentsByPage(pageKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": "查询失败"})
		return
	}
	boosts, err := h.store.ListBoostsByPage(pageKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": "查询失败"})
		return
	}
	cfg := &tmpl.CommentsConfig{}
	if h.site != nil && h.site.Load() != nil {
		cfg = &h.site.Load().Comments
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "comments": comments.ToViewItemsWithOptions(list, cfg, comments.BoostViewOptions{Boosts: boosts})})
}

func publicPost(post *store.Post, detail bool) *publicPostJSON {
	protected := post.IsPasswordProtected()
	authors := append([]string(nil), post.Authors...)
	if len(authors) == 0 && strings.TrimSpace(post.Author) != "" {
		authors = []string{strings.TrimSpace(post.Author)}
	}
	item := &publicPostJSON{Slug: post.Slug, Title: post.Title, Excerpt: post.Excerpt, Cover: post.Cover, Category: post.Category, Categories: append([]string(nil), post.Categories...), Tags: splitTags(post.Tags), Authors: authors, Author: post.Author, PasswordProtected: protected, CreatedAt: post.CreatedAt.Format(time.RFC3339), UpdatedAt: post.UpdatedAt.Format(time.RFC3339)}
	if detail && !protected {
		item.ContentMD, item.ContentHTML = post.ContentMD, post.ContentHTML
	}
	return item
}

func splitTags(raw string) []string {
	var out []string
	for _, value := range strings.Split(raw, ",") {
		if value = strings.TrimSpace(value); value != "" {
			out = append(out, value)
		}
	}
	return out
}
