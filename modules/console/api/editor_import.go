package api

import (
	"net/http"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"

	"blog/internal/post"
)

// MarkdownImportRequest is intentionally JSON rather than multipart: the
// browser reads the local file and sends UTF-8 text, so no temporary server
// file is created and the endpoint remains strictly read-only.
type MarkdownImportRequest struct {
	Filename string `json:"filename"`
	Content  string `json:"content"`
	Kind     string `json:"kind"` // post or page
}

type MarkdownImportPoster struct {
	Headline string `json:"headline,omitempty"`
	Topic    string `json:"topic,omitempty"`
	Caption  string `json:"caption,omitempty"`
	Color    string `json:"color,omitempty"`
}

// MarkdownImportMetadata contains all recognized front matter. Keeping
// fields that a given editor cannot map lets the UI warn instead of silently
// dropping data.
type MarkdownImportMetadata struct {
	Slug            string                `json:"slug,omitempty"`
	Title           string                `json:"title,omitempty"`
	Date            string                `json:"date,omitempty"`
	Excerpt         string                `json:"excerpt,omitempty"`
	Cover           string                `json:"cover,omitempty"`
	Banner          string                `json:"banner,omitempty"`
	Category        string                `json:"category,omitempty"`
	Categories      []string              `json:"categories,omitempty"`
	Topic           string                `json:"topic,omitempty"`
	Tags            []string              `json:"tags,omitempty"`
	AILabel         string                `json:"ai_label,omitempty"`
	Poster          *MarkdownImportPoster `json:"poster,omitempty"`
	References      []string              `json:"references,omitempty"`
	Status          string                `json:"status,omitempty"`
	Pinned          bool                  `json:"pinned,omitempty"`
	Authors         []string              `json:"authors,omitempty"`
	Author          string                `json:"author,omitempty"`
	MetaDescription string                `json:"meta_description,omitempty"`
	MetaKeywords    string                `json:"meta_keywords,omitempty"`
}

// MarkdownImportResponse is the stable response envelope for both editors.
// ContentMD is the parsed body and is never rendered or rewritten.
type MarkdownImportResponse struct {
	ContentMD      string                 `json:"content_md"`
	Metadata       MarkdownImportMetadata `json:"metadata"`
	Fields         []string               `json:"fields"`
	UnknownFields  []string               `json:"unknown_fields"`
	UnmappedFields []string               `json:"unmapped_fields"`
	SlugSuggestion string                 `json:"slug_suggestion,omitempty"`
	StellarTags    []string               `json:"stellar_tags"`
}

var stellarImportTagRe = regexp.MustCompile(`\{%\s*([A-Za-z][A-Za-z0-9_-]*)\b`)

var unmappedImportFields = map[string]map[string]struct{}{
	"post": {
		"date": {}, "poster_headline": {}, "poster_topic": {},
		"poster_caption": {}, "poster_color": {}, "references": {},
	},
	"page": {
		"date": {}, "excerpt": {}, "description": {}, "cover": {}, "banner": {},
		"category": {}, "categories": {}, "topic": {}, "tags": {}, "ai_label": {},
		"poster_headline": {}, "poster_topic": {}, "poster_caption": {}, "poster_color": {},
		"references": {}, "pinned": {}, "authors": {}, "author": {},
		"meta_description": {}, "meta_keywords": {},
	},
}

func importMetadata(meta post.FileMeta) MarkdownImportMetadata {
	out := MarkdownImportMetadata{
		Slug: meta.Slug, Title: meta.Title, Date: meta.Date, Excerpt: meta.Excerpt,
		Cover: meta.Cover, Banner: meta.Banner, Category: meta.Category,
		Categories: append([]string(nil), meta.Categories...), Topic: meta.Topic,
		Tags: append([]string(nil), meta.Tags...), AILabel: meta.AILabel,
		References: append([]string(nil), meta.References...), Status: meta.Status,
		Pinned: meta.Pinned, Authors: append([]string(nil), meta.Authors...),
		Author: meta.Author, MetaDescription: meta.MetaDescription, MetaKeywords: meta.MetaKeywords,
	}
	if meta.PosterHeadline != "" || meta.PosterTopic != "" || meta.PosterCaption != "" || meta.PosterColor != "" {
		out.Poster = &MarkdownImportPoster{
			Headline: meta.PosterHeadline, Topic: meta.PosterTopic,
			Caption: meta.PosterCaption, Color: meta.PosterColor,
		}
	}
	return out
}

func importSlugSuggestion(filename string, meta post.FileMeta) string {
	if strings.TrimSpace(meta.Slug) != "" {
		return generateSlug(meta.Slug)
	}
	base := strings.TrimSpace(meta.Title)
	if base == "" {
		base = strings.TrimSuffix(filepath.Base(strings.TrimSpace(filename)), filepath.Ext(strings.TrimSpace(filename)))
	}
	if base == "" {
		return ""
	}
	return generateSlug(base)
}

func stellarImportTags(body string) []string {
	seen := make(map[string]struct{})
	var tags []string
	for _, match := range stellarImportTagRe.FindAllStringSubmatch(body, -1) {
		name := strings.ToLower(match[1])
		if strings.HasPrefix(name, "end") {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		tags = append(tags, name)
	}
	return tags
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

// importMarkdown POST /api/v1/editor/import/markdown parses an uploaded
// Markdown document in memory. It never writes a post, page, attachment, or
// temporary file; the editor decides whether to apply the returned snapshot.
func (h *Handler) importMarkdown(c *gin.Context) {
	var req MarkdownImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "请求格式错误")
		return
	}
	if req.Kind != "post" && req.Kind != "page" {
		Fail(c, http.StatusBadRequest, "kind 必须是 post 或 page")
		return
	}
	if h.maxUpload > 0 && int64(len(req.Content)) > h.maxUpload {
		Fail(c, http.StatusRequestEntityTooLarge, "Markdown 文件超过大小限制")
		return
	}
	if h.posts == nil {
		Fail(c, http.StatusInternalServerError, "导入服务未装配")
		return
	}
	result, err := h.posts.ParseImport([]byte(req.Content))
	if err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	unmappedSet := unmappedImportFields[req.Kind]
	unmapped := make([]string, 0)
	for _, field := range result.Fields {
		if _, ok := unmappedSet[field]; ok {
			unmapped = append(unmapped, field)
		}
	}
	sort.Strings(unmapped)
	c.JSON(http.StatusOK, MarkdownImportResponse{
		ContentMD: result.Body,
		Metadata:  importMetadata(result.Meta), Fields: nonNilStrings(result.Fields),
		UnknownFields: nonNilStrings(result.UnknownFields), UnmappedFields: nonNilStrings(unmapped),
		SlugSuggestion: importSlugSuggestion(req.Filename, result.Meta),
		StellarTags:    nonNilStrings(stellarImportTags(result.Body)),
	})
}
