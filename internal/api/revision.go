package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"blog/internal/store"
)

// revisionJSON 单版本全文响应模型。
type revisionJSON struct {
	ID        int64  `json:"id"`
	PostSlug  string `json:"post_slug"`
	Title     string `json:"title"`
	Excerpt   string `json:"excerpt"`
	ContentMD string `json:"content_md"`
	Tags      string `json:"tags"`
	Category  string `json:"category"`
	Status    string `json:"status"`
	WordCount int    `json:"word_count"`
	CreatedAt string `json:"created_at"`
}

func toRevisionJSON(r *store.Revision) *revisionJSON {
	return &revisionJSON{
		ID:        r.ID,
		PostSlug:  r.PostSlug,
		Title:     r.Title,
		Excerpt:   r.Excerpt,
		ContentMD: r.ContentMD,
		Tags:      r.Tags,
		Category:  r.Category,
		Status:    r.Status,
		WordCount: r.WordCount,
		CreatedAt: r.CreatedAt.Format(time.RFC3339),
	}
}

// listRevisions GET /api/v1/posts/:slug/revisions：版本列表（不含正文）。
func (h *Handler) listRevisions(c *gin.Context) {
	slug := c.Param("slug")
	metas, err := h.store.ListRevisions(slug)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if metas == nil {
		metas = []*store.RevisionMeta{}
	}
	c.JSON(http.StatusOK, gin.H{"items": metas, "total": len(metas)})
}

// getRevision GET /api/v1/revisions/:id：单版本全文。
func (h *Handler) getRevision(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的版本 id"})
		return
	}
	rev, err := h.store.GetRevision(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if rev == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "版本不存在"})
		return
	}
	c.JSON(http.StatusOK, toRevisionJSON(rev))
}

// restoreRevision POST /api/v1/revisions/:id/restore：回滚到该版本。
// 复用 savePostWithSnapshot 保存管线：重渲染 content_html，并把回滚前的当前内容存为新快照（链条不断）。
func (h *Handler) restoreRevision(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的版本 id"})
		return
	}
	rev, err := h.store.GetRevision(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if rev == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "版本不存在"})
		return
	}

	existing, err := h.store.GetBySlug(rev.PostSlug)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if existing == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "文章不存在: " + rev.PostSlug})
		return
	}

	contentHTML, err := h.posts.Render(rev.ContentMD)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Markdown 渲染失败: " + err.Error()})
		return
	}

	// 用版本内容覆盖正文相关字段，保留现有 slug 与其余元数据（cover/password/pinned/topic 等）
	p := &store.Post{
		Slug:        existing.Slug,
		Title:       rev.Title,
		Excerpt:     rev.Excerpt,
		Cover:       existing.Cover,
		Category:    rev.Category,
		Topic:       existing.Topic,
		Tags:        rev.Tags,
		ContentMD:   rev.ContentMD,
		ContentHTML: contentHTML,
		Status:      rev.Status,
		Pinned:      existing.Pinned,
		Password:    existing.Password,
		PublishAt:   existing.PublishAt,
		Poster:      existing.Poster,
		References:  existing.References,
	}

	if err := h.savePostWithSnapshot(existing, p, existing.Slug); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "回滚失败: " + err.Error()})
		return
	}

	views, _ := h.store.GetViews(existing.Slug)
	c.JSON(http.StatusOK, toPostJSON(p, views))
}
