package github

import (
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

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
// /api/gh/repo：裁剪元数据端点（保留）；ghcard 短码由原生渲染器同步取数（GetRepo/GetUser）。
// /api/gh/widget/*：Spec 33 侧栏 widget 专用代理，输出 GitHub 形状键
// （services/ghinfo.js 按元素 id 回填、timeline.js 消费原始字段），前端不直连 api.github.com。
func (h *Handler) Register(r *gin.Engine) {
	r.GET("/api/gh/repo", h.repoInfo)
	r.GET("/api/gh/widget/repo", h.widgetRepo)
	r.GET("/api/gh/widget/user", h.widgetUser)
	r.GET("/api/gh/widget/tags", h.widgetTags)
	r.GET("/api/gh/widget/issues", h.widgetIssues)
}

// ghNameRe GitHub owner/repo/username 字符白名单（安全审计 2026-08-16）：
// 此前 ?repo=a/../../users/x 可构造 api.github.com 内路径穿越（host 固定无内网
// 风险，但可探测任意 API 端点形状并携带 PAT 请求）。GitHub 标识符字符集：
// 字母数字与 - . _。
var ghNameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)

// validGHName 校验 GitHub 标识符（owner/username/repo 名）。
func validGHName(s string) bool {
	return s != "" && len(s) <= 100 && ghNameRe.MatchString(s) && !strings.Contains(s, "..")
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
	if !validGHName(owner) || !validGHName(name) {
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

// repoParam 解析并校验 repo 查询参数，非法时写 400 并返回 ok=false。
// 安全审计 2026-08-16：owner/name 走 ghNameRe 白名单（防路径穿越注入）。
func repoParam(c *gin.Context) (owner, name string, ok bool) {
	repo := c.Query("repo")
	if repo == "" {
		c.String(http.StatusBadRequest, "missing repo param (user/name)")
		return "", "", false
	}
	owner, name = splitRepo(repo)
	if !validGHName(owner) || !validGHName(name) {
		c.String(http.StatusBadRequest, "invalid repo format, expected user/name")
		return "", "", false
	}
	return owner, name, true
}

// widgetRepo GitHub 形状仓库元数据（ghrepo widget，ds-ghinfo 按 id 回填 description/stargazers_count/forks_count）。
// GET /api/gh/widget/repo?repo=user/name
func (h *Handler) widgetRepo(c *gin.Context) {
	owner, name, ok := repoParam(c)
	if !ok {
		return
	}
	info, err := h.client.GetRepo(owner, name)
	if err != nil {
		c.String(http.StatusBadGateway, "github repo fetch failed: %v", err)
		return
	}
	c.Header("Cache-Control", "public, max-age=21600")
	c.JSON(http.StatusOK, gin.H{
		"description":      info.Description,
		"stargazers_count": info.Stars,
		"forks_count":      info.Forks,
		"html_url":         info.HTMLURL,
	})
}

// widgetUser GitHub 形状用户信息（ghuser widget，ds-ghinfo 按 id 回填 name/bio/avatar_url/followers/following/public_repos）。
// GET /api/gh/widget/user?username=xxx
func (h *Handler) widgetUser(c *gin.Context) {
	username := c.Query("username")
	if !validGHName(username) {
		c.String(http.StatusBadRequest, "missing or invalid username param")
		return
	}
	info, err := h.client.GetUser(username)
	if err != nil {
		c.String(http.StatusBadGateway, "github user fetch failed: %v", err)
		return
	}
	name := info.Name
	if name == "" {
		name = info.Login
	}
	c.Header("Cache-Control", "public, max-age=21600")
	c.JSON(http.StatusOK, gin.H{
		"name":         name,
		"bio":          info.Bio,
		"avatar_url":   info.AvatarURL,
		"followers":    info.Followers,
		"following":    info.Following,
		"public_repos": info.Repos,
		"html_url":     info.HTMLURL,
	})
}

// widgetTags 仓库 tags 原样透传（ghrepo widget latest-tag 子卡，ds-ghinfo index=0 取 name）。
// GET /api/gh/widget/tags?repo=user/name
func (h *Handler) widgetTags(c *gin.Context) {
	owner, name, ok := repoParam(c)
	if !ok {
		return
	}
	data, err := h.client.GetTagsRaw(owner, name)
	if err != nil {
		c.String(http.StatusBadGateway, "github tags fetch failed: %v", err)
		return
	}
	c.Header("Cache-Control", "public, max-age=21600")
	c.Data(http.StatusOK, "application/json; charset=utf-8", data)
}

// widgetIssues 仓库 issues 原样透传（ghissues widget，ds-timeline 渲染）。
// GET /api/gh/widget/issues?repo=user/name&per_page=3&labels=a,b
func (h *Handler) widgetIssues(c *gin.Context) {
	owner, name, ok := repoParam(c)
	if !ok {
		return
	}
	perPage, err := strconv.Atoi(c.DefaultQuery("per_page", "3"))
	if err != nil || perPage < 1 {
		perPage = 3
	} else if perPage > 30 {
		perPage = 30
	}
	q := url.Values{}
	q.Set("per_page", strconv.Itoa(perPage))
	if labels := c.Query("labels"); labels != "" {
		q.Set("labels", labels)
	}
	data, err := h.client.GetIssuesRaw(owner, name, q.Encode())
	if err != nil {
		c.String(http.StatusBadGateway, "github issues fetch failed: %v", err)
		return
	}
	c.Header("Cache-Control", "public, max-age=1800")
	c.Data(http.StatusOK, "application/json; charset=utf-8", data)
}
