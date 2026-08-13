package api

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"blog/internal/comment"
	"blog/internal/store"
	tmpl "blog/internal/template"
)

// PAT 常量。
const (
	patTokenPrefix     = "pat_"      // 令牌明文前缀（识别 + 防误用其它字符串）
	patScopeRead       = "read"      // 当前唯一允许的 scope（公开只读 API）
	patRateLimitPerMin = 60          // 每令牌每分钟最大请求数
	patRateWindow      = time.Minute // 限流窗口
)

// patWindow 单令牌限流窗口状态。
type patWindow struct {
	n int       // 窗口内请求数
	t time.Time // 窗口起点
}

// patAuth PAT 鉴权中间件：Authorization: Bearer pat_xxx → sha256 查表 → 过期/吊销校验 → 注入 tokenID。
// 与 apiAuth（cookie）完全分离：PAT 组无论站点密码是否配置都必须校验 Bearer。
func (h *Handler) patAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := strings.TrimSpace(c.GetHeader("Authorization"))
		if !strings.HasPrefix(auth, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "缺少 Bearer 令牌"})
			return
		}
		plain := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
		if !strings.HasPrefix(plain, patTokenPrefix) {
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
		if !h.patAllow(pat.ID) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "请求过于频繁，请稍后再试"})
			return
		}
		// 异步刷新最后使用时间：失败不阻断请求（并发吊销竞态下静默）
		h.store.TouchPATLastUsed(pat.ID)
		c.Set("tokenID", pat.ID)
		c.Next()
	}
}

// patAllow 每令牌 60 次/分钟窗口限流（仿登录限流的 map+mutex 模式），顺便清理过期条目防 map 膨胀。
func (h *Handler) patAllow(id int64) bool {
	h.patMu.Lock()
	defer h.patMu.Unlock()
	now := time.Now()
	for k, v := range h.patHits {
		if now.Sub(v.t) >= patRateWindow {
			delete(h.patHits, k)
		}
	}
	v, ok := h.patHits[id]
	if !ok || now.Sub(v.t) >= patRateWindow {
		h.patHits[id] = patWindow{n: 1, t: now}
		return true
	}
	if v.n >= patRateLimitPerMin {
		return false
	}
	v.n++
	h.patHits[id] = v
	return true
}

// --- 令牌管理（cookie authed 组） ---

// generatePAT 生成明文令牌：pat_ + 32 字节随机数 hex（明文仅创建时返回一次，DB 只存哈希）。
func generatePAT() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate pat: %w", err)
	}
	return patTokenPrefix + hex.EncodeToString(b), nil
}

// formatPATExpiry 时间格式化：零值返回空串（永不过期/从未使用）。
func formatPATExpiry(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// parsePATExpiry 解析过期时间（支持 RFC3339 / datetime-local / 日期），格式无效返回零值。
func parsePATExpiry(s string) time.Time {
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04", "2006-01-02 15:04", "2006-01-02"} {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t
		}
	}
	return time.Time{}
}

// listTokens GET /api/v1/tokens：令牌列表（脱敏：不含哈希，明文不可逆也无法展示）。
func (h *Handler) listTokens(c *gin.Context) {
	pats, err := h.store.ListPATs()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	items := make([]gin.H, 0, len(pats))
	for _, p := range pats {
		items = append(items, gin.H{
			"id":           p.ID,
			"name":         p.Name,
			"description":  p.Description,
			"scopes":       p.Scopes,
			"prefix":       patTokenPrefix + "****", // 脱敏展示：明文仅创建时返回一次
			"expires_at":   formatPATExpiry(p.ExpiresAt),
			"last_used_at": formatPATExpiry(p.LastUsedAt),
			"created_at":   p.CreatedAt.Format(time.RFC3339),
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": len(items)})
}

// createToken POST /api/v1/tokens：创建令牌。明文仅在本响应中返回一次，之后无法再获取。
func (h *Handler) createToken(c *gin.Context) {
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Scopes      string `json:"scopes"`
		ExpiresAt   string `json:"expires_at"` // 空=永不过期
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "令牌名称不能为空"})
		return
	}
	scopes := strings.TrimSpace(req.Scopes)
	if scopes == "" {
		scopes = patScopeRead
	}
	// 当前仅支持只读 scope：拦截写权限 scope，防止语义混淆
	for _, s := range strings.Split(scopes, ",") {
		if strings.TrimSpace(s) != patScopeRead {
			c.JSON(http.StatusBadRequest, gin.H{"error": "仅支持 read scope"})
			return
		}
	}
	var expires time.Time
	if s := strings.TrimSpace(req.ExpiresAt); s != "" {
		expires = parsePATExpiry(s)
		if expires.IsZero() {
			c.JSON(http.StatusBadRequest, gin.H{"error": "过期时间格式无效"})
			return
		}
	}
	plain, err := generatePAT()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	pat := &store.PAT{
		Name:        name,
		TokenHash:   store.HashToken(plain),
		Description: strings.TrimSpace(req.Description),
		Scopes:      scopes,
		ExpiresAt:   expires,
	}
	if err := h.store.CreatePAT(pat); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id":         pat.ID,
		"name":       pat.Name,
		"token":      plain, // 明文仅此一次返回
		"scopes":     pat.Scopes,
		"expires_at": formatPATExpiry(pat.ExpiresAt),
	})
}

// deleteToken DELETE /api/v1/tokens/:id：吊销令牌（立即失效，DELETE 幂等）。
func (h *Handler) deleteToken(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法令牌 id"})
		return
	}
	if err := h.store.DeletePAT(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// --- 公开只读 Content API（/api/public/v1，PAT 认证） ---

// pubPostJSON 公开只读文章响应模型（不含管理字段；密码保护文章不含正文）。
type pubPostJSON struct {
	Slug              string   `json:"slug"`
	Title             string   `json:"title"`
	Excerpt           string   `json:"excerpt"`
	Cover             string   `json:"cover"`
	Category          string   `json:"category"`
	Tags              []string `json:"tags"`
	ContentMD         string   `json:"content_md,omitempty"` // 密码保护文章不返回
	ContentHTML       string   `json:"content_html,omitempty"`
	PasswordProtected bool     `json:"password_protected"`
	CreatedAt         string   `json:"created_at"`
	UpdatedAt         string   `json:"updated_at"`
}

// splitTags 逗号分隔标签转切片（跳过空项）。
func splitTags(s string) []string {
	var out []string
	for _, t := range strings.Split(s, ",") {
		if t = strings.TrimSpace(t); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// toPubPostJSON 公开序列化：detail=true 时携带正文（密码保护文章除外）。
func toPubPostJSON(p *store.Post, detail bool) *pubPostJSON {
	protected := p.IsPasswordProtected()
	item := &pubPostJSON{
		Slug:              p.Slug,
		Title:             p.Title,
		Excerpt:           p.Excerpt,
		Cover:             p.Cover,
		Category:          p.Category,
		Tags:              splitTags(p.Tags),
		PasswordProtected: protected,
		CreatedAt:         p.CreatedAt.Format(time.RFC3339),
		UpdatedAt:         p.UpdatedAt.Format(time.RFC3339),
	}
	if detail && !protected {
		item.ContentMD = p.ContentMD
		item.ContentHTML = p.ContentHTML
	}
	return item
}

// pubListPosts GET /api/public/v1/posts：已发布文章分页列表（仅元信息，无正文）。
func (h *Handler) pubListPosts(c *gin.Context) {
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
	items := make([]*pubPostJSON, 0, len(posts))
	for _, p := range posts {
		items = append(items, toPubPostJSON(p, false))
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total, "page": page, "size": size})
}

// pubGetPost GET /api/public/v1/posts/:slug：单篇已发布文章。
// 密码保护文章不返回正文（content_md/content_html 置空），仅返回元信息 + password_protected 标记。
func (h *Handler) pubGetPost(c *gin.Context) {
	p, err := h.store.GetPublishedBySlug(c.Param("slug"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if p == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "文章不存在"})
		return
	}
	c.JSON(http.StatusOK, toPubPostJSON(p, true))
}

// pubListPages GET /api/public/v1/pages：已发布页面列表（页面无密码机制，含正文）。
func (h *Handler) pubListPages(c *gin.Context) {
	pages, err := h.store.ListPublishedPages()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	items := make([]gin.H, 0, len(pages))
	for _, p := range pages {
		items = append(items, gin.H{
			"slug":         p.Slug,
			"title":        p.Title,
			"content_md":   p.ContentMD,
			"content_html": p.ContentHTML,
			"created_at":   p.CreatedAt.Format(time.RFC3339),
			"updated_at":   p.UpdatedAt.Format(time.RFC3339),
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": len(items)})
}

// pubListCategories GET /api/public/v1/categories：分类及已发布文章数。
func (h *Handler) pubListCategories(c *gin.Context) {
	m, err := h.store.CountByCategory()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	items := sortedTaxonomy(m)
	c.JSON(http.StatusOK, gin.H{"items": items, "total": len(items)})
}

// pubListTags GET /api/public/v1/tags：标签及已发布文章数。
func (h *Handler) pubListTags(c *gin.Context) {
	m, err := h.store.CountByTag()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	items := sortedTaxonomy(m)
	c.JSON(http.StatusOK, gin.H{"items": items, "total": len(items)})
}

// pubListComments GET /api/public/v1/comments?page_key=：已发布评论树（复用前台评论渲染链路）。
func (h *Handler) pubListComments(c *gin.Context) {
	pageKey := strings.TrimSpace(c.Query("page_key"))
	if pageKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "page_key 不能为空"})
		return
	}
	comments, err := h.store.ListCommentsByPage(pageKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": "查询失败"})
		return
	}
	cfg := &tmpl.CommentsConfig{}
	if h.site.Load() != nil {
		cfg = &h.site.Load().Comments
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "comments": comment.ToViewItems(comments, cfg)})
}
