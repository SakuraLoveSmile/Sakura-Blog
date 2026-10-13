package api

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"

	"blog/internal/safefetch"
)

const remoteMarkdownLimit = 2 << 20

type remoteMarkdownResponse struct {
	HTML      string `json:"html"`
	SourceURL string `json:"source_url"`
}
type remoteMarkdownError struct {
	Error string `json:"error"`
}

// MountRemoteMarkdown 为远程 Markdown 提供有界获取与净化；浏览器不接触作者脚本。
func MountRemoteMarkdown(r *gin.Engine, siteURL string) {
	handler := remoteMarkdownHandler(safefetch.New(safefetch.Options{
		Timeout: 15 * time.Second, MaxRedirects: 5,
		RedirectValidator: func(u *url.URL) error { _, err := safefetch.ValidateURL(u.String()); return err },
	}))
	mountRemoteMarkdown(r, siteURL, handler)
}

func mountRemoteMarkdown(r *gin.Engine, siteURL string, handler gin.HandlerFunc) {
	r.GET("/api/stellar/remote-md", handler)
	// SSR 的子目录 root 与客户端一致；保留根 API，不改变其他公开 API 的路由。
	if u, err := url.Parse(siteURL); err == nil && u.IsAbs() {
		root := strings.TrimRight(path.Clean(u.Path), "/")
		if strings.HasPrefix(root, "/") && root != "" {
			r.GET(root+"/api/stellar/remote-md", handler)
		}
	}
}

// 注入客户端仅供隔离测试。生产入口始终使用 safefetch 的逐跳 DNS/拨号保护。
func remoteMarkdownHandler(client *http.Client) gin.HandlerFunc {
	md := goldmark.New(goldmark.WithExtensions(extension.GFM), goldmark.WithRendererOptions(html.WithUnsafe()))
	policy := bluemonday.UGCPolicy()
	policy.AllowAttrs("class").OnElements("code", "pre", "table", "th", "td", "div", "span")
	return func(c *gin.Context) {
		raw := c.Query("url")
		u, err := safefetch.ValidateURL(raw)
		if err != nil || len(raw) > 4096 {
			c.JSON(http.StatusBadRequest, remoteMarkdownError{Error: "invalid remote Markdown URL"})
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			c.JSON(http.StatusBadRequest, remoteMarkdownError{Error: "invalid remote Markdown URL"})
			return
		}
		req.Header.Set("Accept", "text/markdown, text/plain;q=0.9, */*;q=0.1")
		resp, err := client.Do(req)
		if err != nil {
			if c.Request.Context().Err() != nil {
				return
			}
			c.JSON(http.StatusBadGateway, remoteMarkdownError{Error: "remote Markdown fetch failed"})
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			c.JSON(http.StatusBadGateway, remoteMarkdownError{Error: "remote Markdown source returned an error"})
			return
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, remoteMarkdownLimit+1))
		if err != nil {
			c.JSON(http.StatusBadGateway, remoteMarkdownError{Error: "remote Markdown read failed"})
			return
		}
		if len(body) > remoteMarkdownLimit {
			c.JSON(http.StatusRequestEntityTooLarge, remoteMarkdownError{Error: "remote Markdown exceeds 2 MiB"})
			return
		}
		var rendered strings.Builder
		if err := md.Convert(body, &rendered); err != nil {
			c.JSON(http.StatusUnprocessableEntity, remoteMarkdownError{Error: "remote Markdown render failed"})
			return
		}
		if ctx.Err() != nil {
			return
		}
		c.Header("Cache-Control", "public, max-age=300")
		c.JSON(http.StatusOK, remoteMarkdownResponse{HTML: policy.Sanitize(rendered.String()), SourceURL: resp.Request.URL.String()})
	}
}
