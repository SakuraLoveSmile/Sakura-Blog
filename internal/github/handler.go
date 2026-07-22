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
// 仅保留直连 api.github.com 的仓库元数据端点；ghcard/ghrepo/ghuser 均由原生渲染器
// 同步取数（GetRepo/GetUser），不再提供依赖第三方 github-readme-stats 的 SVG 卡片端点。
func (h *Handler) Register(r *gin.Engine) {
	r.GET("/api/gh/repo", h.repoInfo)
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
