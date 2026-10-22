package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"blog/internal/post"
)

// ShortcodeEditRequest is the raw Stellar shortcode sent by the property editor.
type ShortcodeEditRequest struct {
	Raw string `json:"raw"`
}

// shortcodeEdit parses a raw shortcode without changing persisted content.
func (h *Handler) shortcodeEdit(c *gin.Context) {
	var req ShortcodeEditRequest
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Raw) == "" {
		Fail(c, http.StatusBadRequest, "shortcode 参数错误")
		return
	}

	result, err := post.ParseShortcodeForEditStellarV2(req.Raw)
	if strings.HasPrefix(strings.TrimSpace(result.Name), "blog_") {
		result, err = post.ParseShortcodeForEditBlogExtension(req.Raw)
	}
	if err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	c.JSON(http.StatusOK, result)
}

// Present 的 nil 与 [] 分别表示新插入和既有标签无参数。
type ShortcodeSerializeRequest struct {
	Name    string         `json:"name"`
	Values  map[string]any `json:"values"`
	Present *[]string      `json:"present,omitempty"`
}
type ShortcodeSerializeResponse struct {
	Raw string `json:"raw"`
}

func (h *Handler) shortcodeSerialize(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxSettingsBodyBytes)
	var req ShortcodeSerializeRequest
	decoder := json.NewDecoder(c.Request.Body)
	decoder.UseNumber()
	if err := decoder.Decode(&req); err != nil {
		Fail(c, http.StatusBadRequest, "shortcode 参数错误")
		return
	}
	var presence [][]string
	if req.Present != nil {
		presence = append(presence, *req.Present)
	}
	raw, err := post.SerializeShortcodeEditStellarV2(req.Name, req.Values, presence...)
	if strings.HasPrefix(req.Name, "blog_") {
		raw, err = post.SerializeShortcodeEditBlogExtension(req.Name, req.Values, presence...)
	}
	if err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	c.JSON(http.StatusOK, ShortcodeSerializeResponse{Raw: raw})
}
