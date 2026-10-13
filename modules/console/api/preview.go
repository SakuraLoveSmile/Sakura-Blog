package api

import (
	"blog/internal/content"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"

	"blog/internal/post"
	"blog/internal/stellarv2"
	"github.com/gin-gonic/gin"
)

// previewMarkdown POST /api/v1/preview/markdown：把 Markdown 渲染成 HTML。
// 复用 post.Service 的 goldmark 管线（shortcode/KaTeX/Chroma 高亮），保证预览与前台渲染一致。
// preview_profile=component-library（Spec 63）时进入本地 Fixture 模式：
// GitHub/友链用确定性数据，rating/vote/friends/sites/md 端点改写为 /api/v1/preview/*，
// 不依赖公网、不写入真实数据；缺省保持原有渲染行为不变。
type StellarMarkdownPreviewRequest struct {
	StellarContentMetadata
	Markdown       string `json:"markdown"`
	PreviewProfile string `json:"preview_profile"`
	Title          string `json:"title"`
	Layout         string `json:"layout"`
	Language       string `json:"language"`
	Path           string `json:"path"`
	PreviewToken   string `json:"preview_token"`
}
type StellarMarkdownPreviewResponse struct {
	HTML         string `json:"html"`
	Document     string `json:"document"`
	PreviewToken string `json:"preview_token"`
}

func previewRequestURL(c *gin.Context) (string, error) {
	// Stellar middleware 已验证直接代理，仅消费它的协议决策，不信任原始转发头。
	scheme := c.GetString("blog.request_scheme")
	if scheme != "http" && scheme != "https" {
		scheme = "http"
		if c.Request.TLS != nil {
			scheme = "https"
		}
	}
	origin, err := url.Parse(scheme + "://" + c.Request.Host)
	if err != nil || origin.Hostname() == "" || origin.User != nil || origin.Path != "" || origin.RawQuery != "" || origin.Fragment != "" {
		return "", fmt.Errorf("无效的请求来源")
	}
	origin.Path = c.Request.URL.Path
	return origin.String(), nil
}

func (h *Handler) previewMarkdown(c *gin.Context) {
	var req StellarMarkdownPreviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误")
		return
	}
	if req.Layout == "" {
		req.Layout = "page"
	}
	switch req.Layout {
	case "page", "post", "wiki", "note":
	default:
		Fail(c, http.StatusBadRequest, "不支持的预览 layout")
		return
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_-]{16,128}$`).MatchString(req.PreviewToken) {
		Fail(c, http.StatusBadRequest, "无效的预览 token")
		return
	}
	var html string
	var err error
	if req.PreviewProfile == "component-library" {
		var parsed stellarv2.Result
		parsed, err = validateEditorMetadata(req.StellarContentMetadata, req.Layout, false)
		if err == nil {
			html, err = h.posts.RenderComponentPreviewStellarV2(req.Markdown, post.StellarRenderContext{Page: parsed.Value, Language: req.Language})
		}
	} else {
		html, err = h.renderEditorStellar(req.Markdown, req.StellarContentMetadata, req.Layout)
	}
	if err != nil {
		failEditorStellar(c, err)
		return
	}
	if h.renderStellarContentPreview == nil {
		Fail(c, http.StatusNotImplemented, "Stellar 2 文档预览尚未装配")
		return
	}
	requestURL, err := previewRequestURL(c)
	if err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	document, err := h.renderStellarContentPreview(content.StellarPreviewInput{StellarVersion: req.StellarVersion, StellarConfig: req.StellarConfig, BlogExtensions: req.BlogExtensions, HTML: html, Title: req.Title, Layout: req.Layout, Language: req.Language, Path: req.Path, RequestURL: requestURL, PreviewToken: req.PreviewToken})
	if err != nil {
		failEditorStellar(c, err)
		return
	}
	c.JSON(http.StatusOK, StellarMarkdownPreviewResponse{HTML: html, Document: document, PreviewToken: req.PreviewToken})
}

// previewSite POST /api/v1/preview/site：设置页「前端预览」所见即所得。
// 请求体 {sections: {分段名: 草稿JSON}, page: index|categories|tags}，
// 后端用草稿配置（不落库）渲染真实前台整页 HTML 返回，Console 塞入 iframe 展示。
func (h *Handler) previewSite(c *gin.Context) {
	var req struct {
		Sections map[string]json.RawMessage `json:"sections"`
		Page     string                     `json:"page"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误")
		return
	}
	if h.renderSitePreview == nil {
		Fail(c, http.StatusNotImplemented, "预览服务未装配")
		return
	}
	html, err := h.renderSitePreview(req.Sections, req.Page)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "渲染失败: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"html": html})
}
