package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"blog/internal/store"
)

// wikiDocImportMaxItems 单次导入上限：与文章列表单页上限一致，避免一次请求占用过久。
const wikiDocImportMaxItems = 100

// WikiDocImportRequest 从「文章」导入到指定文库项目。
// Slugs 为文章 slug 列表（按选择顺序导入）；Parent 为目标父文档（0=根文档）；
// Status 为导入后的文档状态（空值按草稿处理，避免导入即公开）。
type WikiDocImportRequest struct {
	Slugs  []string `json:"slugs"`
	Parent int64    `json:"parent"`
	Status string   `json:"status"`
}

// WikiDocImportItem 单篇文章的导入结果。Result 取值 imported/skipped/failed，
// 与响应里的三个分桶一致；Reason 只对 skipped/failed 有意义。
type WikiDocImportItem struct {
	PostSlug string `json:"post_slug"`
	Slug     string `json:"slug"`
	Title    string `json:"title"`
	Result   string `json:"result"`
	Reason   string `json:"reason,omitempty"`
	DocID    int64  `json:"doc_id,omitempty"`
}

// WikiDocImportResponse 批量导入结果。单篇跳过/失败不回滚已导入的文档。
type WikiDocImportResponse struct {
	Imported []WikiDocImportItem `json:"imported"`
	Skipped  []WikiDocImportItem `json:"skipped"`
	Failed   []WikiDocImportItem `json:"failed"`
}

// importWikidocs POST /api/v1/wiki/:project/docs/import：把文章复制为当前项目的文库文档。
// 只读原文章：标题与 Markdown 正文生成新文档，文章级字段（封面、分类、标签、作者等）
// 不进入文库存量模型；已存在的同 Slug 文档一律跳过，不覆盖。
func (h *Handler) importWikidocs(c *gin.Context) {
	projectID := c.Param("project")
	if ok, err := h.wikiProjectAvailable(projectID); err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	} else if !ok {
		Fail(c, http.StatusNotFound, "Wiki 项目不存在")
		return
	}

	var req WikiDocImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "请求格式错误")
		return
	}

	status := strings.TrimSpace(req.Status)
	if status == "" {
		status = store.StatusDraft
	}
	if status != store.StatusDraft && status != store.StatusPublished {
		Fail(c, http.StatusBadRequest, "导入状态必须是 draft 或 published")
		return
	}

	// parent 校验与创建文档一致：父文档必须存在且属于当前项目，否则整单拒绝。
	if req.Parent != 0 {
		parent, err := h.store.GetWikidocByID(req.Parent)
		if err != nil {
			Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		if parent == nil || parent.ProjectID != projectID {
			Fail(c, http.StatusBadRequest, "父文档不存在或不属于当前项目")
			return
		}
	}

	slugs := normalizeImportSlugs(req.Slugs)
	if len(slugs) == 0 {
		Fail(c, http.StatusBadRequest, "请选择要导入的文章")
		return
	}
	if len(slugs) > wikiDocImportMaxItems {
		Fail(c, http.StatusBadRequest, "单次最多导入 100 篇文章")
		return
	}

	meta := StellarContentMetadata{StellarVersion: 2, StellarConfig: "{}", BlogExtensions: "{}"}
	resp := WikiDocImportResponse{
		Imported: []WikiDocImportItem{},
		Skipped:  []WikiDocImportItem{},
		Failed:   []WikiDocImportItem{},
	}

	for _, slug := range slugs {
		source, err := h.store.GetBySlug(slug)
		if err != nil {
			resp.Failed = append(resp.Failed, WikiDocImportItem{
				PostSlug: slug, Slug: slug, Result: "failed", Reason: "读取文章失败",
			})
			continue
		}
		if source == nil {
			resp.Failed = append(resp.Failed, WikiDocImportItem{
				PostSlug: slug, Slug: slug, Result: "failed", Reason: "文章不存在",
			})
			continue
		}
		if strings.TrimSpace(source.DeletedAt) != "" {
			resp.Failed = append(resp.Failed, WikiDocImportItem{
				PostSlug: slug, Slug: slug, Title: source.Title, Result: "failed", Reason: "文章在回收站，无法导入",
			})
			continue
		}
		title := strings.TrimSpace(source.Title)
		if title == "" {
			title = slug
		}
		if existing, err := h.store.GetWikidocBySlug(projectID, slug); err != nil {
			resp.Failed = append(resp.Failed, WikiDocImportItem{
				PostSlug: slug, Slug: slug, Title: title, Result: "failed", Reason: "检查同名文档失败",
			})
			continue
		} else if existing != nil {
			resp.Skipped = append(resp.Skipped, WikiDocImportItem{
				PostSlug: slug, Slug: slug, Title: title, Result: "skipped", Reason: "项目内已存在同名 Slug 文档",
			})
			continue
		}
		if strings.TrimSpace(source.ContentMD) == "" {
			resp.Failed = append(resp.Failed, WikiDocImportItem{
				PostSlug: slug, Slug: slug, Title: title, Result: "failed", Reason: "文章没有 Markdown 原文，无法导入",
			})
			continue
		}

		contentHTML, err := h.renderEditorStellar(source.ContentMD, meta, "wiki")
		if err != nil {
			resp.Failed = append(resp.Failed, WikiDocImportItem{
				PostSlug: slug, Slug: slug, Title: title, Result: "failed", Reason: "正文渲染失败: " + err.Error(),
			})
			continue
		}

		doc := &store.WikiDoc{
			StellarVersion: meta.StellarVersion, StellarConfig: meta.StellarConfig, BlogExtensions: meta.BlogExtensions,
			ProjectID:   projectID,
			Slug:        slug,
			Title:       title,
			Order:       0,
			Parent:      req.Parent,
			ContentMD:   source.ContentMD,
			ContentHTML: contentHTML,
			Status:      status,
		}
		if err := h.store.CreateWikidoc(doc); err != nil {
			resp.Failed = append(resp.Failed, WikiDocImportItem{
				PostSlug: slug, Slug: slug, Title: title, Result: "failed", Reason: "创建文档失败: " + err.Error(),
			})
			continue
		}
		resp.Imported = append(resp.Imported, WikiDocImportItem{
			PostSlug: slug, Slug: slug, Title: title, Result: "imported", DocID: doc.ID,
		})
	}

	// 只在确有写入时广播内容变更，避免空导入触发前台/搜索刷新。
	if len(resp.Imported) > 0 {
		h.publishContentChanged()
	}
	c.JSON(http.StatusOK, resp)
}

// normalizeImportSlugs 去空白、去空项并按首次出现顺序去重。
func normalizeImportSlugs(input []string) []string {
	seen := make(map[string]struct{}, len(input))
	out := make([]string, 0, len(input))
	for _, raw := range input {
		slug := strings.TrimSpace(raw)
		if slug == "" {
			continue
		}
		if _, ok := seen[slug]; ok {
			continue
		}
		seen[slug] = struct{}{}
		out = append(out, slug)
	}
	return out
}
