package api

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"blog/internal/store"
)

// shareJSON 文章分享链接 API 响应模型。
type shareJSON struct {
	ID        int64  `json:"id"`
	Token     string `json:"token"`
	PostSlug  string `json:"post_slug"`
	PostTitle string `json:"post_title"` // 关联查文章标题
	ShareType string `json:"share_type"`
	ExpiresAt string `json:"expires_at"`
	CreatedAt string `json:"created_at"`
	URL       string `json:"url"` // 完整预览 URL（相对路径）
}

// shareRequest 创建分享链接的请求体。
type shareRequest struct {
	ShareType string `json:"share_type"` // "published" 或 "draft"，默认 "published"
	ExpiresAt string `json:"expires_at"` // RFC3339 过期时间，空串=永久有效
}

// toShareJSON 将 SharedPost 转为 API 响应模型，postTitle 为关联文章标题（可空）。
func toShareJSON(sp *store.SharedPost, postTitle string) *shareJSON {
	return &shareJSON{
		ID:        sp.ID,
		Token:     sp.Token,
		PostSlug:  sp.PostSlug,
		PostTitle: postTitle,
		ShareType: sp.ShareType,
		ExpiresAt: sp.ExpiresAt,
		CreatedAt: sp.CreatedAt.Format(time.RFC3339),
		URL:       fmt.Sprintf("/preview/shared-posts/%s", sp.Token),
	}
}

// listShares GET /api/v1/shares：列出全部分享链接（支持分页，与 listPosts 一致）。
// 关联查文章标题，构建 slug->title 映射后批量填充。
func (h *Handler) listShares(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}

	shares, err := h.store.ListShares()
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	// 构建 slug->title 映射（一次 List 取全部文章，避免逐条查 DB）
	titleMap := make(map[string]string)
	if posts, err := h.store.List(); err == nil {
		for _, p := range posts {
			titleMap[p.Slug] = p.Title
		}
	}

	total := len(shares)
	start := (page - 1) * size
	if start > total {
		start = total
	}
	end := start + size
	if end > total {
		end = total
	}
	paged := shares[start:end]

	items := make([]*shareJSON, 0, len(paged))
	for _, sp := range paged {
		items = append(items, toShareJSON(sp, titleMap[sp.PostSlug]))
	}

	c.JSON(http.StatusOK, gin.H{
		"items": items,
		"total": total,
		"page":  page,
		"size":  size,
	})
}

// listPostShares GET /api/v1/posts/:slug/shares：列出指定文章的分享链接。
func (h *Handler) listPostShares(c *gin.Context) {
	slug := c.Param("slug")

	// 验证文章存在
	p, err := h.store.GetBySlug(slug)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if p == nil {
		Fail(c, http.StatusNotFound, "文章不存在")
		return
	}

	shares, err := h.store.ListSharesBySlug(slug)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	items := make([]*shareJSON, 0, len(shares))
	for _, sp := range shares {
		items = append(items, toShareJSON(sp, p.Title))
	}

	c.JSON(http.StatusOK, gin.H{"items": items, "total": len(items)})
}

// createShare POST /api/v1/posts/:slug/shares：为指定文章创建分享链接。
// 请求体: {share_type: "published"|"draft", expires_at: ""}
func (h *Handler) createShare(c *gin.Context) {
	slug := c.Param("slug")

	// 验证文章存在
	p, err := h.store.GetBySlug(slug)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if p == nil {
		Fail(c, http.StatusNotFound, "文章不存在")
		return
	}

	var req shareRequest
	// body 可空（全部走默认值），不强制要求 ShouldBindJSON 成功
	_ = c.ShouldBindJSON(&req)

	shareType := req.ShareType
	if shareType == "" {
		shareType = "published"
	}
	if shareType != "published" && shareType != "draft" {
		Fail(c, http.StatusBadRequest, "share_type 只能是 published 或 draft")
		return
	}

	// 校验过期时间格式（非空时必须是合法 RFC3339）
	expiresAt := req.ExpiresAt
	if expiresAt != "" {
		if _, err := time.Parse(time.RFC3339, expiresAt); err != nil {
			Fail(c, http.StatusBadRequest, "expires_at 必须是 RFC3339 格式")
			return
		}
	}

	sp := &store.SharedPost{
		PostSlug:  slug,
		ShareType: shareType,
		ExpiresAt: expiresAt,
	}
	if _, err := h.store.CreateShare(sp); err != nil {
		Fail(c, http.StatusInternalServerError, "创建失败: " + err.Error())
		return
	}

	c.JSON(http.StatusCreated, toShareJSON(sp, p.Title))
}

// deleteShare DELETE /api/v1/shares/:id：删除分享链接。
func (h *Handler) deleteShare(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "无效的 id")
		return
	}

	if err := h.store.DeleteShare(id); err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}
