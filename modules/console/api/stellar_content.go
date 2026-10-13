package api

import (
	"blog/internal/content"
	"fmt"
	"net/http"

	"blog/internal/config"
	"blog/internal/post"
	"blog/internal/stellarv2"
	"github.com/gin-gonic/gin"
)

// StellarContentMetadata 保留原始 JSON 文本，不经过 JavaScript Number。
type StellarContentMetadata struct {
	StellarVersion int    `json:"stellar_version"`
	StellarConfig  string `json:"stellar_config"`
	BlogExtensions string `json:"blog_extensions"`
}

type StellarContentValidationRequest struct {
	StellarContentMetadata
	Profile    string `json:"profile"`
	Collection bool   `json:"collection"`
}

type StellarContentValidationResponse struct {
	Valid  bool                   `json:"valid"`
	Issues []StellarSettingsIssue `json:"issues"`
}

func validateEditorMetadata(meta StellarContentMetadata, profile string, collection bool) (stellarv2.Result, error) {
	if meta.StellarVersion != 2 {
		return stellarv2.Result{}, fmt.Errorf("stellar_version 必须为 2；旧内容须先审查迁移预览")
	}
	raw, err := decodeStellarJSONObject(meta.StellarConfig)
	if err != nil {
		return stellarv2.Result{}, fmt.Errorf("stellar_config 必须是 JSON 对象: %w", err)
	}
	if _, err := decodeStellarJSONObject(meta.BlogExtensions); err != nil {
		return stellarv2.Result{}, fmt.Errorf("blog_extensions 必须是独立 JSON 对象: %w", err)
	}
	options := stellarv2.Options{Source: "editor.stellar_config"}
	var parsed stellarv2.Result
	if collection {
		parsed, err = stellarv2.ParseCollection(raw, options)
	} else {
		parsed, err = stellarv2.ParsePage(raw, options)
	}
	if err != nil {
		return parsed, err
	}
	var consumed stellarv2.Result
	if collection {
		consumed, err = stellarv2.ValidateCollectionProfile(parsed.Value, options.Source, profile)
	} else {
		consumed = stellarv2.ValidatePageProfile(parsed.Value, options.Source, profile)
	}
	consumed.Issues = append(parsed.Issues, consumed.Issues...)
	return consumed, err
}

func (h *Handler) validateStellarContent(c *gin.Context) {
	var req StellarContentValidationRequest
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxSettingsBodyBytes)
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "请求格式错误或超过 1 MiB")
		return
	}
	switch req.Profile {
	case "post", "page", "wiki", "topic", "note", "notebook":
	default:
		Fail(c, http.StatusBadRequest, "未知 profile")
		return
	}
	result, err := validateEditorMetadata(req.StellarContentMetadata, req.Profile, req.Collection)
	if err != nil {
		stellarSettingsError(c, err, http.StatusBadRequest)
		return
	}
	c.JSON(http.StatusOK, StellarContentValidationResponse{Valid: true, Issues: stellarSettingsIssues(result.Issues)})
}

// 所有保存和预览使用同一严格正文渲染器，仅传公开 data 袋。
func (h *Handler) renderEditorStellar(body string, meta StellarContentMetadata, profile string) (string, error) {
	parsed, err := validateEditorMetadata(meta, profile, false)
	if err != nil {
		return "", err
	}
	loaded, err := config.LoadStellarV2(h.store)
	if err != nil {
		return "", err
	}
	language := content.ResolveStellarLanguage(parsed.Value, loaded.Blog)
	data, _ := loaded.Blog["data"].(map[string]any)
	return h.posts.RenderStellarV2WithContext(body, post.StellarRenderContext{Theme: loaded.Theme, Data: data, Page: parsed.Value, Language: language, BlogExtensions: true})
}

func failEditorStellar(c *gin.Context, err error) {
	stellarSettingsError(c, err, http.StatusBadRequest)
}

// 归属必须存在且已显式迁移，不能借旧宿主字段猜出新配置。
func (h *Handler) checkEditorCollection(meta StellarContentMetadata, profile, host string) error {
	result, err := validateEditorMetadata(meta, profile, false)
	if err != nil {
		return err
	}
	collection, _ := result.Value["collection"].(map[string]any)
	kind, id := "", ""
	if collection != nil {
		kind, _ = collection["profile"].(string)
		id, _ = collection["id"].(string)
	}
	if profile == "wiki" {
		if kind != "" && (kind != "wiki" || id != host) {
			return fmt.Errorf("collection_conflict: Wiki 文档必须属于 wiki/%s", host)
		}
		kind, id = "wiki", host
	}
	if kind == "" {
		return nil
	}
	if (profile == "post" && kind != "topic") || (profile == "page" && kind != "wiki" && kind != "notebook") {
		return fmt.Errorf("collection_not_found: %s 不能属于 %s/%s", profile, kind, id)
	}
	switch kind {
	case "wiki":
		item, e := h.store.GetWikiProject(id)
		if e != nil {
			return e
		}
		if item == nil {
			return fmt.Errorf("collection_not_found: wiki/%s", id)
		}
		_, err = item.ValidateStellar()
	case "topic":
		item, e := h.store.GetTopicBySlug(id)
		if e != nil {
			return e
		}
		if item == nil {
			return fmt.Errorf("collection_not_found: topic/%s", id)
		}
		_, err = item.ValidateStellar()
	case "notebook":
		item, e := h.store.GetNotebook(id)
		if e != nil {
			return e
		}
		if item == nil {
			return fmt.Errorf("collection_not_found: notebook/%s", id)
		}
		_, err = item.ValidateStellar()
	default:
		err = fmt.Errorf("collection_not_found: %s/%s", kind, id)
	}
	if err != nil {
		return fmt.Errorf("collection config invalid: %w", err)
	}
	return nil
}
