package routes

import (
	"blog/internal/settings"
	"crypto/subtle"
	"log"
	"net/http"

	"time"

	"github.com/gin-gonic/gin"

	"blog/internal/store"
	tmpl "blog/modules/stellar/viewmodel"
)

// registerPostRoutes 注册文章域路由：/post/:slug（以及兼容旧链接的 /posts/:slug）、密码解锁、分享链接预览。
func (s *webServer) registerPostRoutes(r *gin.Engine) {
	// 文章路由只展示已发布且通过严格配置校验的记录。
	r.GET("/post/:slug", s.handlePost)
	// 兼容复数文章路径，统一重定向到 canonical 单数路径，避免同一文章出现两套 URL。
	r.GET("/posts/:slug", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/post/"+c.Param("slug"))
	})
	// 文章密码解锁
	r.POST("/post/:slug/unlock", s.handlePostUnlock)
	// 文章分享链接预览：/preview/shared-posts/:token
	// 无需认证，通过唯一 token 访问文章内容（已发布版或草稿版），支持有效期。
	// 注册在 NoRoute 之前，避免被 404 拦截；前台无此链接，不会触发 PJAX。
	r.GET("/preview/shared-posts/:token", s.handleSharePreview)
}

// handlePost 文章详情页：先授权，再读取完整快照和严格页面模型。
func (s *webServer) handlePost(c *gin.Context) { s.serveStellarPost(c, c.Param("slug"), nil) }

// handlePostUnlock 文章密码解锁：口令比对成功签 HMAC cookie 后回跳文章页。
// 安全审计 2026-08-16：补 IP+slug 维度限流（5 次失败/5 分钟），防在线爆破。
func (s *webServer) handlePostUnlock(c *gin.Context) {
	slug := c.Param("slug")
	p, err := s.st.GetPublishedBySlug(slug)
	if err != nil || p == nil || !p.IsPasswordProtected() {
		c.Redirect(http.StatusFound, "/post/"+slug)
		return
	}
	rlKey := c.ClientIP() + "|" + slug
	if postPWBlocked(rlKey) {
		s.renderPostPWPage(c, p, "尝试次数过多，请稍后再试")
		return
	}
	input := c.PostForm("password")
	if subtle.ConstantTimeCompare([]byte(input), []byte(p.Password)) != 1 {
		postPWRecordFail(rlKey)
		s.renderPostPWPage(c, p, "密码错误，请重试")
		return
	}
	postPWClear(rlKey)
	expiry := time.Now().Add(postPWCookieTTL).Unix()
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(postPWCookieName(slug), signPostPWCookie(slug, p.Password, expiry), int(postPWCookieTTL.Seconds()), "/", "", s.cookieSecureFor(c), true)
	c.Redirect(http.StatusFound, "/post/"+slug)
}

// renderPostPWPage 渲染口令输入页（解锁失败/限流共用）。
func (s *webServer) renderPostPWPage(c *gin.Context, p *store.Post, message string) {
	data := &tmpl.ViewData{Site: s.site.Load(), Page: &tmpl.Page{Title: "受保护文章", Slug: p.Slug, Layout: "post-password", Lang: "zh-CN", Description: message}}
	if in, err := stellarSiteInput(data.Site); err == nil {
		data.Stellar = &settings.StellarRenderContext{Version: 2, Profile: "post", Input: in}
	}
	s.outputStellar(c, data, http.StatusOK)
}

// handleSharePreview 分享链接预览：token 换文章（published/draft），带过期检查与预览提示条。
func (s *webServer) handleSharePreview(c *gin.Context) {
	token := c.Param("token")
	sp, err := s.st.GetShareByToken(token)
	if err != nil {
		log.Printf("get share by token: %v", err)
		s.render404(c)
		return
	}
	if sp == nil {
		s.render404(c)
		return
	}
	// 过期检查：ExpiresAt 非空且已过期则 404
	if sp.ExpiresAt != "" {
		if expiresAt, perr := time.Parse(time.RFC3339, sp.ExpiresAt); perr != nil || !time.Now().Before(expiresAt) {
			s.render404(c)
			return
		}
	}
	// 根据 ShareType 加载文章：published 走已发布查询，draft 走任意状态查询（获取草稿内容）
	var p *store.Post
	if sp.ShareType == "draft" {
		p, err = s.st.GetBySlug(sp.PostSlug)
	} else {
		p, err = s.st.GetPublishedBySlug(sp.PostSlug)
	}
	if err != nil {
		log.Printf("load post for share: %v", err)
		s.render500(c)
		return
	}
	if p == nil || p.DeletedAt != "" {
		s.render404(c)
		return
	}
	s.serveStellarPost(c, p.Slug, p)
}
