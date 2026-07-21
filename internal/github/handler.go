package github

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler GitHub 代理 HTTP handler（公开路由，无需鉴权）。
type Handler struct {
	client *Client
}

// NewHandler 构造 Handler。
func NewHandler(c *Client) *Handler {
	return &Handler{client: c}
}

// Register 注册公开路由到 gin engine。
func (h *Handler) Register(r *gin.Engine) {
	r.GET("/api/gh/card", h.cardSVG)
	r.GET("/api/gh/repo", h.repoInfo)
}

// cardSVG 代理 github-readme-stats SVG 卡片。
// GET /api/gh/card?username=x&repo=y&theme=z...
// 透传全部 query 参数到上游，返回 image/svg+xml。
func (h *Handler) cardSVG(c *gin.Context) {
	params := c.Request.URL.RawQuery
	if params == "" {
		c.String(http.StatusBadRequest, "missing query params (username required)")
		return
	}
	svg, err := h.client.GetCardSVG(params)
	if err != nil {
		c.String(http.StatusBadGateway, "github card fetch failed: %v", err)
		return
	}
	c.Header("Cache-Control", "public, max-age=86400")
	c.Data(http.StatusOK, "image/svg+xml; charset=utf-8", svg)
}

// repoInfo 返回仓库裁剪元数据 JSON。
// GET /api/gh/repo?repo=user/name
func (h *Handler) repoInfo(c *gin.Context) {
	repo := c.Query("repo")
	if repo == "" {
		c.String(http.StatusBadRequest, "missing repo param (user/name)")
		return
	}
	owner, name := splitRepo(repo)
	if owner == "" || name == "" {
		c.String(http.StatusBadRequest, "invalid repo format, expected user/name")
		return
	}
	info, err := h.client.GetRepo(owner, name)
	if err != nil {
		c.String(http.StatusBadGateway, "github repo fetch failed: %v", err)
		return
	}
	c.Header("Cache-Control", "public, max-age=21600")
	c.JSON(http.StatusOK, info)
}

// splitRepo "user/name" → (user, name)。
func splitRepo(s string) (string, string) {
	for i := 0; i < len(s); i++ {
		if s[i] == '/' {
			return s[:i], s[i+1:]
		}
	}
	return "", ""
}
