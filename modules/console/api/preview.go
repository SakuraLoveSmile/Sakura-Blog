package api

import (
	"encoding/json"
	"net/http"

	"blog/internal/post"
	"blog/internal/stellarv2"
	"github.com/gin-gonic/gin"
)

// previewMarkdown POST /api/v1/preview/markdown：把 Markdown 渲染成 HTML。
// 复用 post.Service 的 goldmark 管线（shortcode/KaTeX/Chroma 高亮），保证预览与前台渲染一致。
// preview_profile=component-library（Spec 63）时进入本地 Fixture 模式：
// GitHub/友链用确定性数据，rating/vote/friends/sites/md 端点改写为 /api/v1/preview/*，
// 不依赖公网、不写入真实数据；缺省保持原有渲染行为不变。
func (h *Handler) previewMarkdown(c *gin.Context) {
	var req struct {
		StellarContentMetadata
		Markdown       string `json:"markdown"`
		PreviewProfile string `json:"preview_profile"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误")
		return
	}
	var (
		html string
		err  error
	)
	if req.PreviewProfile == "component-library" {
		var parsed stellarv2.Result
		parsed, err = validateEditorMetadata(req.StellarContentMetadata, "page", false)
		if err == nil {
			html, err = h.posts.RenderComponentPreviewStellarV2(req.Markdown, post.StellarRenderContext{Page: parsed.Value})
		}
	} else {
		html, err = h.renderEditorStellar(req.Markdown, req.StellarContentMetadata, "page")
	}
	if err != nil {
		failEditorStellar(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"html": html})
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
