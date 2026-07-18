// Package admin 管理后台 handler 集合，挂 /admin 路由组。
// 文章 CRUD + Markdown 编辑器，数据走 store.Store，Markdown 渲染复用 post.Service。
package admin

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"blog/internal/comment"
	"blog/internal/post"
	"blog/internal/store"
	tmpl "blog/internal/template"
)

// Handler 管理后台请求处理器，持有 store、post service、模板引擎、站点配置、评论 handler。
type Handler struct {
	store        *store.Store
	posts        *post.Service
	engine       *tmpl.Engine
	site         *tmpl.Site
	comments     *comment.Handler // 管理员回复时复用其通知管道（被回复者邮件通知）
	loginLimiter *ipLimiter       // 登录失败 IP 限流（防爆破）
}

// New 创建 admin handler。comments 可为 nil（评论功能关闭时管理员回复不触发通知）。
func New(s *store.Store, posts *post.Service, engine *tmpl.Engine, site *tmpl.Site, comments *comment.Handler) *Handler {
	return &Handler{store: s, posts: posts, engine: engine, site: site, comments: comments, loginLimiter: newIPLimiter()}
}

// Register 把 admin 路由挂到 r 上。登录/登出公开，其余走 authRequired 密码 gate。
func (h *Handler) Register(r *gin.Engine) {
	g := r.Group("/admin")
	g.GET("/login", h.loginForm) // 登录页
	g.POST("/login", h.login)    // 登录提交
	g.POST("/logout", h.logout)  // 退出登录

	authed := g.Group("", h.authRequired())
	authed.GET("", h.list) // 文章列表
	authed.GET("/", h.list)
	authed.GET("/new", h.editForm)         // 新建表单
	authed.GET("/edit/:slug", h.editForm)  // 编辑表单
	authed.POST("/save", h.save)           // 创建/更新
	authed.POST("/delete/:slug", h.delete) // 删除

	authed.GET("/comments", h.commentList)                 // 评论管理（筛选 + 分页）
	authed.POST("/comments/:id/approve", h.commentApprove) // 审核通过
	authed.POST("/comments/:id/delete", h.commentDelete)   // 删除（级联子评论）
	authed.POST("/comments/reply", h.commentReply)         // 管理员回复
}

// list 渲染文章列表页。
func (h *Handler) list(c *gin.Context) {
	posts, err := h.store.List()
	if err != nil {
		c.String(http.StatusInternalServerError, "list posts: %v", err)
		return
	}
	rows := make([]*tmpl.AdminPostRow, 0, len(posts))
	for _, p := range posts {
		rows = append(rows, &tmpl.AdminPostRow{
			Slug:     p.Slug,
			Title:    p.Title,
			Excerpt:  p.Excerpt,
			Category: p.Category,
			Status:   p.Status,
			Date:     p.CreatedAt.Format("2006-01-02 15:04"),
		})
	}
	data := &tmpl.ViewData{
		Site:  h.site,
		Page:  &tmpl.Page{Title: "文章管理", MenuID: "admin"},
		Admin: &tmpl.AdminData{Posts: rows},
	}
	h.render(c, "admin/list", data)
}

// editForm 渲染新建/编辑表单。:slug 存在为编辑，否则为新建。
func (h *Handler) editForm(c *gin.Context) {
	slug := c.Param("slug")
	row := &tmpl.AdminPostRow{Status: "draft"}
	isEdit := false
	if slug != "" {
		p, err := h.store.GetBySlug(slug)
		if err != nil {
			c.String(http.StatusInternalServerError, "load post: %v", err)
			return
		}
		if p == nil {
			c.String(http.StatusNotFound, "post not found: %s", slug)
			return
		}
		row.Slug = p.Slug
		row.Title = p.Title
		row.Excerpt = p.Excerpt
		row.Cover = p.Cover
		row.Category = p.Category
		row.Tags = p.Tags
		row.ContentMD = p.ContentMD
		row.Status = p.Status
		isEdit = true
	}
	data := &tmpl.ViewData{
		Site:  h.site,
		Page:  &tmpl.Page{Title: ternary(isEdit, "编辑文章", "新建文章"), MenuID: "admin"},
		Admin: &tmpl.AdminData{EditPost: row, IsEdit: isEdit},
	}
	h.render(c, "admin/edit", data)
}

// save 处理表单提交：slug 存在则更新，否则创建。服务端用 goldmark 渲染 content_html。
func (h *Handler) save(c *gin.Context) {
	slug := strings.TrimSpace(c.PostForm("slug"))
	title := strings.TrimSpace(c.PostForm("title"))
	excerpt := strings.TrimSpace(c.PostForm("excerpt"))
	cover := strings.TrimSpace(c.PostForm("cover"))
	category := strings.TrimSpace(c.PostForm("category"))
	tags := normalizeTags(c.PostForm("tags"))
	contentMD := c.PostForm("content")
	status := c.PostForm("status")
	origSlug := c.PostForm("orig_slug") // 编辑时的原 slug

	if slug == "" {
		h.renderSaveError(c, origSlug, slug, title, excerpt, cover, category, tags, contentMD, status, "slug 不能为空")
		return
	}
	if status != store.StatusDraft && status != store.StatusPublished {
		status = store.StatusDraft
	}

	// 服务端渲染 Markdown 为 HTML（复用 post.Service 的 goldmark 实例）
	contentHTML, err := h.posts.Render(contentMD)
	if err != nil {
		h.renderSaveError(c, origSlug, slug, title, excerpt, cover, category, tags, contentMD, status, "Markdown 渲染失败: "+err.Error())
		return
	}

	// 编辑模式：orig_slug 非空且 slug 变了，需先删旧再建新（slug 是唯一键）
	if origSlug != "" && origSlug != slug {
		if _, err := h.store.GetBySlug(slug); err == nil {
			// 新 slug 已被占用
			h.renderSaveError(c, origSlug, slug, title, excerpt, cover, category, tags, contentMD, status, "slug 已被其他文章占用: "+slug)
			return
		}
	}

	p := &store.Post{
		Slug:        slug,
		Title:       title,
		Excerpt:     excerpt,
		Cover:       cover,
		Category:    category,
		Tags:        tags,
		ContentMD:   contentMD,
		ContentHTML: contentHTML,
		Status:      status,
	}

	if origSlug != "" {
		// 更新（用 orig_slug 定位）
		p.Slug = slug
		if err := h.updateWithRename(origSlug, p); err != nil {
			h.renderSaveError(c, origSlug, slug, title, excerpt, cover, category, tags, contentMD, status, "保存失败: "+err.Error())
			return
		}
	} else {
		if _, err := h.store.Create(p); err != nil {
			h.renderSaveError(c, origSlug, slug, title, excerpt, cover, category, tags, contentMD, status, "创建失败: "+err.Error())
			return
		}
	}
	c.Redirect(http.StatusFound, "/admin")
}

// updateWithRename 处理 slug 变更：若 orig != new，先删旧记录再 Create 新记录（保留 created_at）。
func (h *Handler) updateWithRename(origSlug string, p *store.Post) error {
	if origSlug == p.Slug {
		return h.store.Update(origSlug, p)
	}
	existing, err := h.store.GetBySlug(origSlug)
	if err != nil {
		return err
	}
	if existing == nil {
		return h.store.Upsert(p)
	}
	// 保留原 created_at
	if err := h.store.Delete(origSlug); err != nil {
		return err
	}
	p.CreatedAt = existing.CreatedAt
	_, err = h.store.Create(p)
	return err
}

// delete 删除文章。
func (h *Handler) delete(c *gin.Context) {
	slug := c.Param("slug")
	if err := h.store.Delete(slug); err != nil {
		c.String(http.StatusInternalServerError, "delete: %v", err)
		return
	}
	c.Redirect(http.StatusFound, "/admin")
}

// commentList 评论管理页：状态筛选（all/pending/approved）+ 分页（20 条/页）。
func (h *Handler) commentList(c *gin.Context) {
	filter := c.DefaultQuery("status", "all")
	status := ""
	switch filter {
	case store.CommentPending, store.CommentApproved:
		status = filter
	default:
		filter = "all"
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	const size = 20
	total, err := h.store.CountComments(status)
	if err != nil {
		c.String(http.StatusInternalServerError, "count comments: %v", err)
		return
	}
	totalPages := (total + size - 1) / size
	if totalPages > 0 && page > totalPages {
		page = totalPages
	}
	comments, err := h.store.ListComments(status, page, size)
	if err != nil {
		c.String(http.StatusInternalServerError, "list comments: %v", err)
		return
	}
	rows := make([]*tmpl.AdminCommentRow, 0, len(comments))
	for _, cm := range comments {
		row := &tmpl.AdminCommentRow{
			ID:      cm.ID,
			PageKey: cm.PageKey,
			Nick:    cm.Nick,
			Email:   cm.Email,
			Excerpt: truncateText(stripHTML(cm.ContentHTML), 80),
			Status:  cm.Status,
			IsAdmin: cm.IsAdmin,
			Avatar:  comment.AvatarURL(h.site.Comments.GravatarMirror, cm.Email),
			Date:    cm.CreatedAt.Format("2006-01-02 15:04"),
		}
		// 被回复者昵称（子评论时）：逐条查询，20 条/页 SQLite 无压力
		if cm.ParentID != 0 {
			if parent, err := h.store.GetComment(cm.ParentID); err == nil && parent != nil {
				row.ReplyNick = parent.Nick
			}
		}
		rows = append(rows, row)
	}
	pages := make([]int, totalPages)
	for i := range pages {
		pages[i] = i + 1
	}
	data := &tmpl.ViewData{
		Site: h.site,
		Page: &tmpl.Page{Title: "评论管理", MenuID: "admin"},
		Admin: &tmpl.AdminData{
			Comments:      rows,
			CommentFilter: filter,
			CommentPage:   page,
			CommentPages:  pages,
		},
	}
	h.render(c, "admin/comments", data)
}

// commentApprove 审核通过（pending → approved）。
func (h *Handler) commentApprove(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, "invalid comment id")
		return
	}
	if err := h.store.UpdateCommentStatus(id, store.CommentApproved); err != nil {
		c.String(http.StatusInternalServerError, "approve: %v", err)
		return
	}
	c.Redirect(http.StatusFound, "/admin/comments")
}

// commentDelete 删除评论（级联删除其所有子评论）。
func (h *Handler) commentDelete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, "invalid comment id")
		return
	}
	if err := h.store.DeleteCommentCascade(id); err != nil {
		c.String(http.StatusInternalServerError, "delete: %v", err)
		return
	}
	c.Redirect(http.StatusFound, "/admin/comments")
}

// commentReply 管理员以博主身份回复：is_admin=1、直接 approved、触发被回复者邮件通知。
func (h *Handler) commentReply(c *gin.Context) {
	parentID, _ := strconv.ParseInt(c.PostForm("parent_id"), 10, 64)
	content := strings.TrimSpace(c.PostForm("content"))
	if parentID == 0 || content == "" {
		c.String(http.StatusBadRequest, "parent_id 与 content 必填")
		return
	}
	parent, err := h.store.GetComment(parentID)
	if err != nil || parent == nil {
		c.String(http.StatusBadRequest, "被回复的评论不存在")
		return
	}
	// 两层嵌套：回复子评论时挂到同一根下
	rootID := parent.ID
	if parent.RootID != 0 {
		rootID = parent.RootID
	}
	html, err := comment.Render(content)
	if err != nil {
		c.String(http.StatusInternalServerError, "render: %v", err)
		return
	}
	nick := h.site.Comments.AdminBadge
	if nick == "" {
		nick = "博主"
	}
	cm := &store.Comment{
		PageKey:     parent.PageKey,
		Nick:        nick,
		Email:       h.site.Comments.AdminEmail,
		ContentMD:   content,
		ContentHTML: html,
		ParentID:    parentID,
		RootID:      rootID,
		Status:      store.CommentApproved,
		IsAdmin:     true,
		IP:          c.ClientIP(),
		UA:          c.Request.UserAgent(),
	}
	if _, err := h.store.CreateComment(cm); err != nil {
		c.String(http.StatusInternalServerError, "reply: %v", err)
		return
	}
	// 触发被回复者通知（comment.Handler 内部规则：approved、有邮箱、非自回、非博主）
	if h.comments != nil {
		h.comments.NotifyReply(cm, parent)
	}
	c.Redirect(http.StatusFound, "/admin/comments")
}

// stripHTML / truncateText：评论摘要纯文本提取（与 main.go 搜索索引同款逻辑）。
var (
	htmlTagRe    = regexp.MustCompile(`<[^>]+>`)
	whitespaceRe = regexp.MustCompile(`\s+`)
)

func stripHTML(s string) string {
	s = htmlTagRe.ReplaceAllString(s, " ")
	return strings.TrimSpace(whitespaceRe.ReplaceAllString(s, " "))
}

func truncateText(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// normalizeTags 规范化标签输入：逗号拆分、逐项 trim、去空去重（保留首次出现顺序），
// 重新以 "," 连接。与 parser.parseList 语义一致，保证存储值规范，
// 使 ListByTag 的逗号包裹 LIKE 与 CountByTag 的拆分计数行为一致。
func normalizeTags(raw string) string {
	seen := map[string]bool{}
	var out []string
	for _, t := range strings.Split(raw, ",") {
		t = strings.TrimSpace(t)
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	return strings.Join(out, ",")
}

// renderSaveError 保存失败时回显表单数据并提示错误。
func (h *Handler) renderSaveError(c *gin.Context, origSlug, slug, title, excerpt, cover, category, tags, contentMD, status, errMsg string) {
	row := &tmpl.AdminPostRow{
		Slug:      slug,
		Title:     title,
		Excerpt:   excerpt,
		Cover:     cover,
		Category:  category,
		Tags:      tags,
		ContentMD: contentMD,
		Status:    status,
	}
	data := &tmpl.ViewData{
		Site:  h.site,
		Page:  &tmpl.Page{Title: "编辑文章", MenuID: "admin"},
		Admin: &tmpl.AdminData{EditPost: row, IsEdit: origSlug != "", SaveError: errMsg},
	}
	h.render(c, "admin/edit", data)
}

// render 渲染指定 admin 模板并写入响应。
func (h *Handler) render(c *gin.Context, name string, data *tmpl.ViewData) {
	html, err := h.engine.Render(name, data)
	if err != nil {
		c.String(http.StatusInternalServerError, "render %s: %v", name, err)
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}

// Seed 扫描 content/posts/*.md 导入 DB（按 slug UPSERT），供 main.go -seed 调用。
func (h *Handler) Seed() error {
	slugs, err := h.posts.ListSlugs()
	if err != nil {
		return err
	}
	for _, slug := range slugs {
		meta, body, err := h.posts.ParseFile(slug)
		if err != nil {
			return err
		}
		html, err := h.posts.Render(body)
		if err != nil {
			return err
		}
		title := meta.Title
		if title == "" {
			title = slug
		}
		p := &store.Post{
			Slug:        slug,
			Title:       title,
			Excerpt:     meta.Excerpt,
			Cover:       meta.Cover,
			Category:    meta.Category,
			Tags:        strings.Join(meta.Tags, ","),
			ContentMD:   body,
			ContentHTML: html,
			Status:      store.StatusPublished,
			CreatedAt:   parseSeedDate(meta.Date),
		}
		if err := h.store.Upsert(p); err != nil {
			return err
		}
	}
	return nil
}

// SeedCount 返回 seed 导入的文章数（仅日志用）。
func (h *Handler) SeedCount() (int, error) {
	slugs, err := h.posts.ListSlugs()
	if err != nil {
		return 0, err
	}
	return len(slugs), nil
}

// parseSeedDate 把 frontmatter 的 date 字段（如 2026-07-16）解析成 time.Time，失败用当前时间。
func parseSeedDate(s string) time.Time {
	for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02", time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Now()
}

func ternary(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}
