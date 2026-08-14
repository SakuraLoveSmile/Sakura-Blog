package main

import (
	"crypto/subtle"
	"html/template"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"

	"blog/internal/config"
	"blog/internal/store"
	tmpl "blog/internal/template"
	"blog/internal/textutil"
)

// registerPostRoutes 注册文章域路由：/post/:slug、密码解锁、分享链接预览。
func (s *webServer) registerPostRoutes(r *gin.Engine) {
	// 文章路由 /post/:slug：DB 优先（published），未命中回退文件渲染（向后兼容 Demo 2）
	r.GET("/post/:slug", s.handlePost)
	// 文章密码解锁
	r.POST("/post/:slug/unlock", s.handlePostUnlock)
	// 文章分享链接预览：/preview/shared-posts/:token
	// 无需认证，通过唯一 token 访问文章内容（已发布版或草稿版），支持有效期。
	// 注册在 NoRoute 之前，避免被 404 拦截；前台无此链接，不会触发 PJAX。
	r.GET("/preview/shared-posts/:token", s.handleSharePreview)
}

// handlePost 文章详情页：DB 优先，密码门 → 阅读量 → 视图模型 → 渲染；DB 未命中回退文件渲染。
func (s *webServer) handlePost(c *gin.Context) {
	slug := c.Param("slug")

	// 优先查 DB
	if p, err := s.st.GetPublishedBySlug(slug); err == nil && p != nil {
		// 密码门：未解锁则渲染口令表单（不增加阅读量、不泄漏正文）
		if p.IsPasswordProtected() {
			cookie, _ := c.Cookie(postPWCookieName(slug))
			if !validPostPWCookie(cookie, slug, p.Password) {
				data := &tmpl.ViewData{
					Site: s.site.Load(),
					Page: &tmpl.Page{
						Title:  p.Title,
						Slug:   p.Slug,
						Layout: "post-password",
						Lang:   "zh-CN",
						MenuID: "post",
					},
				}
				s.fillSidebar(data)
				s.fillSEO(data, c)
				html, err := s.engine.Render("layout/base", data)
				if err != nil {
					log.Printf("render: %v", err)
					s.render500(c)
					return
				}
				c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
				return
			}
		}
		// 阅读量自增：每次访问 /post/:slug 计数 +1（Spec 61 阶段 5：单语句 RETURNING，一次往返）。
		// 失败仅记日志，不中断渲染。
		views, err := s.st.IncrViewsAndGet(slug)
		if err != nil {
			log.Printf("incr views for %s: %v", slug, err)
		}
		data := &tmpl.ViewData{
			Site: s.site.Load(),
			Page: &tmpl.Page{
				Title:      p.Title,
				Slug:       p.Slug,
				Layout:     "post",
				Type:       "tech",
				Lang:       "zh-CN",
				MenuID:     "post",
				Author:     p.Author,
				Date:       p.CreatedAt.Format("2006-01-02"),
				Updated:    p.UpdatedAt.Format("2006-01-02"),
				Excerpt:    p.Excerpt,
				Views:      views,
				Topic:      p.Topic,
				References: store.DecodeReferences(p.References),
				Banner:     p.Banner,
				// 文章级 SEO（Spec 42）：非空时优先输出 meta（优先级见 ViewData.MetaDescription）
				MetaDescription: p.MetaDescription,
				MetaKeywords:    p.MetaKeywords,
			},
			Content: template.HTML(p.ContentHTML),
		}
		data.WordCount, data.ReadingMinutes = tmpl.ReadingStats(p.ContentHTML)
		// AI 文章摘要（Spec 53）：预生成摘要 + settings ai 段双开关（未配置默认开）
		if aiCfg, err := config.LoadAI(s.st); err == nil && aiCfg.SummaryOn() {
			data.Page.AISummary = p.AISummary
			data.Page.AISummaryOn = true
			data.Page.AIUserAPIOn = aiCfg.UserAPIOn()
		}
		// 多作者命中时装配展示名（Spec 53 Part A：banner 作者链接）
		if a, ok := s.site.Load().Authors[p.Author]; ok {
			data.Page.AuthorName = a.Name
		}
		data.TOC = tmpl.ExtractTOCWith(p.ContentHTML, s.tocOptions())
		// 专栏（Spec 52）：已禁用（用户反馈功能多余）。左栏系列 widget 不再装配，保留查询代码注释后续恢复：
		// if p.Topic != "" {
		//     if series, err := s.st.ListByTopic(p.Topic); err == nil && len(series) > 0 {
		//         ts := &tmpl.TopicSeries{Name: p.Topic}
		//         for _, sp := range series {
		//             ts.Posts = append(ts.Posts, &tmpl.TopicSeriesPost{Slug: sp.Slug, Title: sp.Title, Active: sp.Slug == slug})
		//         }
		//         data.TopicSeries = ts
		//     }
		// }
		// read_next 邻篇导航（site.yml article.read_next 可关）：同分类优先，无同分类邻篇回退全局时间序；
		// 对齐原版 read_next.ejs：prev=较新文章，next=较早文章；排除加密/未发布文章。
		if s.site.Load().Article.ReadNext {
			if newer, err := s.st.GetAdjacentPost(slug, p.Category, true); err == nil && newer != nil {
				data.PrevPost = &tmpl.AdjacentPost{Slug: newer.Slug, Title: newer.Title}
			}
			if older, err := s.st.GetAdjacentPost(slug, p.Category, false); err == nil && older != nil {
				data.NextPost = &tmpl.AdjacentPost{Slug: older.Slug, Title: older.Title}
			}
		}
		// 相关文章：共同标签数 desc + 时间 desc，上限 site.yml article.related_posts（0=关）
		if n := s.site.Load().Article.RelatedPosts; n > 0 {
			if related, err := s.st.ListRelatedPosts(slug, p.Tags, n); err == nil && len(related) > 0 {
				data.RelatedPosts = s.toCards(related)
			}
		}
		// 贡献者/编辑本文：source 固定为 posts/{slug}.md，按 edit_this_page 映射生成链接
		data.EditPage = tmpl.ResolveEditPage("posts/"+slug+".md", s.site.Load().Article.EditThisPage)
		// 过期提醒：距最后更新超阈值（缺省 180 天）时文首渲染提醒块
		data.ExpiredDays = tmpl.ExpiredDays(data.Page.Date, data.Page.Updated, s.site.Load().Article.ExpireAfterDays(), time.Now())
		// 文章描述：优先 Excerpt，否则截断正文前 150 字符（去 HTML 标签）
		desc := p.Excerpt
		if desc == "" {
			d := textutil.StripHTML(p.ContentHTML)
			if len(d) > 150 {
				d = d[:150]
			}
			desc = d
		}
		data.Page.Description = desc
		s.fillSidebar(data)
		s.fillComments(data, "/post/"+slug)
		s.fillSEO(data, c)
		html, err := s.engine.Render("layout/base", data)
		if err != nil {
			log.Printf("render: %v", err)
			s.render500(c)
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
		return
	}

	// 回退：文件渲染（Demo 2 链路）
	p, err := s.posts.Load(slug)
	if err != nil {
		if os.IsNotExist(err) {
			s.render404(c)
			return
		}
		log.Printf("load post: %v", err)
		s.render500(c)
		return
	}

	data := &tmpl.ViewData{
		Site: s.site.Load(),
		Page: &tmpl.Page{
			Title:      p.Title,
			Slug:       p.Slug,
			Layout:     "post",
			Type:       "tech",
			Lang:       "zh-CN",
			MenuID:     "post",
			Date:       p.Date,
			Excerpt:    p.Excerpt,
			References: p.References,
		},
		Content: template.HTML(p.HTML),
	}
	data.WordCount, data.ReadingMinutes = tmpl.ReadingStats(p.HTML)

	data.TOC = tmpl.ExtractTOCWith(p.HTML, s.tocOptions())
	desc := p.Excerpt
	if desc == "" {
		d := textutil.StripHTML(p.HTML)
		if len(d) > 150 {
			d = d[:150]
		}
		desc = d
	}
	data.Page.Description = desc
	s.fillSidebar(data)
	s.fillComments(data, "/post/"+slug)
	s.fillSEO(data, c)
	html, err := s.engine.Render("layout/base", data)
	if err != nil {
		log.Printf("render: %v", err)
		s.render500(c)
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}

// handlePostUnlock 文章密码解锁：口令比对成功签 HMAC cookie 后回跳文章页。
func (s *webServer) handlePostUnlock(c *gin.Context) {
	slug := c.Param("slug")
	p, err := s.st.GetPublishedBySlug(slug)
	if err != nil || p == nil || !p.IsPasswordProtected() {
		c.Redirect(http.StatusFound, "/post/"+slug)
		return
	}
	input := c.PostForm("password")
	if subtle.ConstantTimeCompare([]byte(input), []byte(p.Password)) != 1 {
		data := &tmpl.ViewData{
			Site: s.site.Load(),
			Page: &tmpl.Page{
				Title:       p.Title,
				Slug:        p.Slug,
				Layout:      "post-password",
				Lang:        "zh-CN",
				MenuID:      "post",
				Description: "密码错误，请重试",
			},
		}
		s.fillSidebar(data)
		s.fillSEO(data, c)
		html, err := s.engine.Render("layout/base", data)
		if err != nil {
			log.Printf("render: %v", err)
			s.render500(c)
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
		return
	}
	expiry := time.Now().Add(postPWCookieTTL).Unix()
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(postPWCookieName(slug), signPostPWCookie(slug, p.Password, expiry), int(postPWCookieTTL.Seconds()), "/", "", s.cookieSecureFor(c), true)
	c.Redirect(http.StatusFound, "/post/"+slug)
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
		if expiresAt, perr := time.Parse(time.RFC3339, sp.ExpiresAt); perr == nil && time.Now().After(expiresAt) {
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
	if p == nil {
		s.render404(c)
		return
	}
	// 预览提示条（内联样式，不依赖外部 CSS）
	notice := template.HTML(`<div style="background:var(--card);border:1px solid var(--theme);color:var(--text-p1);padding:12px 16px;border-radius:8px;margin:0 0 20px;text-align:center;font-size:14px;">这是一篇通过分享链接预览的文章</div>`)
	data := &tmpl.ViewData{
		Site: s.site.Load(),
		Page: &tmpl.Page{
			Title:   p.Title,
			Slug:    p.Slug,
			Layout:  "post",
			Type:    "tech",
			Lang:    "zh-CN",
			MenuID:  "post",
			Date:    p.CreatedAt.Format("2006-01-02"),
			Excerpt: p.Excerpt,
			Topic:   p.Topic,
		},
		Content: notice + template.HTML(p.ContentHTML),
	}
	data.TOC = tmpl.ExtractTOCWith(p.ContentHTML, s.tocOptions())
	s.fillSidebar(data)
	s.fillSEO(data, c)
	html, err := s.engine.Render("layout/base", data)
	if err != nil {
		log.Printf("render share preview: %v", err)
		s.render500(c)
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}
