package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"sort"

	"blog/internal/config"
	"blog/internal/content"
	"github.com/gin-gonic/gin"
)

type StellarWidgetCatalogItem struct {
	Name   string `json:"name"`
	Layout string `json:"layout"`
	Config string `json:"config"`
}
type StellarWidgetCatalogResponse struct {
	Items []StellarWidgetCatalogItem `json:"items"`
}

type StellarWidgetPreviewRequest struct {
	StellarContentMetadata
	WidgetConfig string `json:"widget_config"`
	Layout       string `json:"layout"`
	Path         string `json:"path"`
	Language     string `json:"language"`
	PreviewToken string `json:"preview_token"`
}

func (h *Handler) getStellarWidgetCatalog(c *gin.Context) {
	if _, err := config.LoadStellarV2(h.store); err != nil {
		failEditorStellar(c, err)
		return
	}
	if h.stellarWidgetCatalog == nil {
		Fail(c, http.StatusNotImplemented, "Stellar 2 Widget catalog尚未装配")
		return
	}
	catalog, err := h.stellarWidgetCatalog()
	if err != nil {
		failEditorStellar(c, err)
		return
	}
	names := make([]string, 0, len(catalog))
	for name := range catalog {
		names = append(names, name)
	}
	sort.Strings(names)
	items := make([]StellarWidgetCatalogItem, 0, len(names))
	for _, name := range names {
		raw, err := json.MarshalIndent(catalog[name], "", "  ")
		if err != nil {
			failEditorStellar(c, err)
			return
		}
		layout := ""
		if object, ok := catalog[name].(map[string]any); ok {
			layout, _ = object["layout"].(string)
		}
		items = append(items, StellarWidgetCatalogItem{Name: name, Layout: layout, Config: string(raw)})
	}
	c.JSON(http.StatusOK, StellarWidgetCatalogResponse{Items: items})
}

func (h *Handler) previewStellarWidget(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxSettingsBodyBytes)
	var request StellarWidgetPreviewRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误或超过 1 MiB")
		return
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_-]{16,128}$`).MatchString(request.PreviewToken) {
		Fail(c, http.StatusBadRequest, "无效的预览 token")
		return
	}
	if request.Layout == "" {
		request.Layout = "page"
	}
	switch request.Layout {
	case "page", "post", "wiki", "note":
	default:
		Fail(c, http.StatusBadRequest, "不支持的预览 layout")
		return
	}
	// 官方Widget定义开放；只检查JSON文法，语义交同一上游registry的warning/跳过规则。
	decoder := json.NewDecoder(bytes.NewBufferString(request.WidgetConfig))
	decoder.UseNumber()
	var candidate any
	if err := decoder.Decode(&candidate); err != nil {
		Fail(c, http.StatusBadRequest, "widget_config不是有效JSON")
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		Fail(c, http.StatusBadRequest, "widget_config有多余JSON数据")
		return
	}
	if _, err := validateEditorMetadata(request.StellarContentMetadata, request.Layout, false); err != nil {
		failEditorStellar(c, err)
		return
	}
	if h.renderStellarContentPreview == nil {
		Fail(c, http.StatusNotImplemented, "Stellar 2文档预览尚未装配")
		return
	}
	requestURL, err := previewRequestURL(c)
	if err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	output, err := h.renderStellarContentPreview(content.StellarPreviewInput{StellarVersion: request.StellarVersion, StellarConfig: request.StellarConfig, BlogExtensions: request.BlogExtensions, WidgetConfig: request.WidgetConfig, Layout: request.Layout, Path: request.Path, Language: request.Language, RequestURL: requestURL, PreviewToken: request.PreviewToken})
	if err != nil {
		failEditorStellar(c, err)
		return
	}
	c.JSON(http.StatusOK, StellarMarkdownPreviewResponse{Document: output.Document, PreviewToken: request.PreviewToken})
}
