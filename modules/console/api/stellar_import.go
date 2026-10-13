package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"blog/internal/post"
	"github.com/gin-gonic/gin"
)

type StellarMarkdownImportRequest struct {
	Filename string `json:"filename"`
	Content  string `json:"content"`
	Kind     string `json:"kind"`
}

// 配置文本独立返回，JS 编辑器无需经过 Number 后再 stringify，开放袋大整数不丢失。
type StellarMarkdownImportResponse struct {
	FrontMatter    string   `json:"front_matter"`
	BlogExtensions string   `json:"blog_extensions"`
	ContentMD      string   `json:"content_md"`
	StellarTags    []string `json:"stellar_tags"`
}

type StellarMarkdownExportRequest struct {
	FrontMatter    string `json:"front_matter"`
	BlogExtensions string `json:"blog_extensions"`
	ContentMD      string `json:"content_md"`
}

type StellarMarkdownExportResponse struct {
	Content string `json:"content"`
}

func decodeStellarJSONObject(value string) (map[string]any, error) {
	decoder := json.NewDecoder(bytes.NewBufferString(value))
	decoder.UseNumber()
	var raw map[string]any
	if err := decoder.Decode(&raw); err != nil {
		return nil, err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, ErrValidation
	}
	if raw == nil {
		return nil, ErrValidation
	}
	return raw, nil
}

// 只读预览可在未迁移站点使用；保存仍须通过版本门禁与归属校验。
func (h *Handler) importStellarMarkdown(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxSettingsBodyBytes)
	var request StellarMarkdownImportRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		Fail(c, http.StatusBadRequest, "请求格式错误或超过 1 MiB")
		return
	}
	if request.Kind != "post" && request.Kind != "page" {
		Fail(c, http.StatusBadRequest, "kind 必须是 post 或 page")
		return
	}
	if h.maxUpload > 0 && int64(len(request.Content)) > h.maxUpload {
		Fail(c, http.StatusRequestEntityTooLarge, "Markdown 文件超过大小限制")
		return
	}
	document, err := post.ParseStellarDocument([]byte(request.Content), post.StellarDocumentOptions{Source: request.Filename})
	if err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if document.Raw == nil {
		document.Raw = map[string]any{}
	}
	if document.Blog == nil {
		document.Blog = map[string]any{}
	}
	raw, err := json.MarshalIndent(document.Raw, "", "  ")
	if err != nil {
		Fail(c, http.StatusInternalServerError, "无法编码配置")
		return
	}
	blog, err := json.MarshalIndent(document.Blog, "", "  ")
	if err != nil {
		Fail(c, http.StatusInternalServerError, "无法编码扩展配置")
		return
	}
	c.JSON(http.StatusOK, StellarMarkdownImportResponse{FrontMatter: string(raw), BlogExtensions: string(blog), ContentMD: document.Body, StellarTags: nonNilStrings(stellarImportTags(document.Body))})
}

func (h *Handler) exportStellarMarkdown(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxSettingsBodyBytes)
	var request StellarMarkdownExportRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		Fail(c, http.StatusBadRequest, "请求格式错误或超过 1 MiB")
		return
	}
	raw, err := decodeStellarJSONObject(request.FrontMatter)
	if err != nil {
		Fail(c, http.StatusBadRequest, "front_matter 必须是一个 JSON 对象")
		return
	}
	blog, err := decodeStellarJSONObject(request.BlogExtensions)
	if err != nil {
		Fail(c, http.StatusBadRequest, "blog_extensions 必须是一个 JSON 对象")
		return
	}
	exported, err := post.SerializeStellarDocument(post.StellarDocument{Raw: raw, Blog: blog, Body: request.ContentMD})
	if err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	c.JSON(http.StatusOK, StellarMarkdownExportResponse{Content: string(exported)})
}
