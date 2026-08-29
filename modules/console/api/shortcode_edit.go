package api

import (
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

	result, err := post.ParseShortcodeForEdit(req.Raw)
	if err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	c.JSON(http.StatusOK, result)
}
