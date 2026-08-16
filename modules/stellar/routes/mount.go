package routes

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/gin-gonic/gin"

	"blog/internal/comment"
	"blog/internal/config"
	"blog/internal/content"
	"blog/internal/moment"
	"blog/internal/og"
	"blog/internal/page"
	"blog/internal/post"
	"blog/internal/search"
	"blog/internal/store"
	tmpl "blog/internal/template"
)

// Dependencies contains the shared services required by the Stellar SSR module.
// It deliberately contains no Console implementation details.
type Dependencies struct {
	App       *config.App
	Site      *atomic.Pointer[tmpl.Site]
	Store     *store.Store
	Posts     *post.Service
	Engine    *tmpl.Engine
	OGGen     *og.Generator
	DevMode   bool
	Root      string
	UploadDir string
	NotesPath string
	Comments  *comment.Handler
}

// Server owns the Stellar route state and its derived caches.
type Server struct {
	deps        Dependencies
	web         *webServer
	pageHandler *page.Handler
	pvBuffer    *pageViewBuffer
	searchIdx   *search.Index
	ogCacheDir  string
}

// New creates the SSR server and installs content-index write hooks.
func New(d Dependencies) (*Server, error) {
	searchIdx := setupSearchIndex(d.Store)
	ogCacheDir := ""
	if d.App != nil {
		ogCacheDir = filepath.Join(filepath.Dir(d.App.DBPath), "ogcache")
	}
	postHook := newPostWriteHook(d.Store, searchIdx, d.OGGen)
	pageHook := newPageWriteHook(d.Store, searchIdx)
	momentHook := newMomentWriteHook(d.Store, searchIdx)
	d.Store.SetPostWriteHook(postHook)
	d.Store.SetPageWriteHook(pageHook)
	d.Store.SetMomentWriteHook(momentHook)

	web := newWebServer(webDeps{
		Site:      d.Site,
		Store:     d.Store,
		Posts:     d.Posts,
		Engine:    d.Engine,
		SearchIdx: searchIdx,
		OGGen:     d.OGGen,
		DevMode:   d.DevMode,
		Root:      d.Root,
		UploadDir: d.UploadDir,
		NotesPath: d.NotesPath,
	})
	// Content caches are initialized by newWebServer, so writes can invalidate them
	// without exposing the web implementation to the command package.
	d.Store.SetPostWriteHook(func(op, slug string) { postHook(op, slug); web.contentCache.invalidate() })
	d.Store.SetPageWriteHook(func(op, slug string) { pageHook(op, slug); web.contentCache.invalidate() })
	d.Store.SetMomentWriteHook(func(op, slug string) { momentHook(op, slug); web.contentCache.invalidate() })

	return &Server{
		deps:       d,
		web:        web,
		searchIdx:  searchIdx,
		ogCacheDir: ogCacheDir,
	}, nil
}

// Mount registers all Stellar SSR routes and static assets on the shared router.
func (s *Server) Mount(r *gin.Engine) error {
	if !s.deps.DevMode {
		gin.SetMode(gin.ReleaseMode)
	}
	s.pvBuffer = newPageViewBuffer(s.deps.Store)
	s.web.installMiddleware(r, s.deps.App, s.pvBuffer)
	if err := s.web.registerStaticRoutes(r); err != nil {
		return err
	}

	pageHandler, err := page.New(s.deps.Store, s.deps.Posts, s.deps.Engine, s.deps.Site)
	if err != nil {
		return err
	}
	s.pageHandler = pageHandler
	pageHandler.SetRender404(s.web.render404)
	pageHandler.SetFillExtra(func(data *tmpl.ViewData, c *gin.Context) {
		s.web.fillSidebar(data)
		s.web.fillSEO(data, c)
	})
	pageHandler.Register(r)
	if err := pageHandler.EnsureSystemFriendsPage(); err != nil {
		log.Printf("[warn] seed friends page: %v", err)
	}
	if err := pageHandler.EnsureSystemNotesPage(s.web.notesMDPath); err != nil {
		log.Printf("[warn] seed notes page: %v", err)
	}
	if err := pageHandler.EnsureSystemLinksPage(); err != nil {
		log.Printf("[warn] seed links page: %v", err)
	}

	momentHandler, err := moment.New(s.deps.Store, s.deps.Posts, s.deps.Engine, s.deps.Site, s.web.fillSidebar, s.web.fillSEO)
	if err != nil {
		return err
	}
	momentHandler.Register(r)
	s.web.momentH = momentHandler

	if s.deps.Comments != nil && s.deps.Site.Load().Comments.Enabled {
		s.deps.Comments.Register(r)
	}
	s.web.registerPostRoutes(r)
	s.web.registerHomeRoutes(r)
	s.web.registerTaxonomyRoutes(r)
	s.web.registerSearchRoutes(r)
	s.web.registerFeedRoutes(r)

	// Draft previews are shared content snapshots, so this route stays in Stellar
	// while the Console API only creates the snapshot.
	r.GET("/preview/draft/:token", s.renderDraftPreview)
	return nil
}

func (s *Server) renderDraftPreview(c *gin.Context) {
	pv, ok := content.GetPostPreview(c.Param("token"))
	if !ok {
		s.web.render404(c)
		return
	}
	contentHTML, err := s.deps.Posts.Render(pv.ContentMD)
	if err != nil {
		log.Printf("render draft preview: %v", err)
		s.web.render500(c)
		return
	}
	title := strings.TrimSpace(pv.Title)
	if title == "" {
		title = "预览"
	}
	notice := template.HTML(`<div style="background:var(--card);border:1px solid var(--theme);color:var(--text-p1);padding:12px 16px;border-radius:8px;margin:0 0 20px;text-align:center;font-size:14px;">预览模式：这是未发布的草稿，仅供发布前查看</div>`)
	data := &tmpl.ViewData{
		Site: s.deps.Site.Load(),
		Page: &tmpl.Page{
			Title: title, Slug: pv.Slug, Layout: "post", Type: "tech", Lang: "zh-CN",
			MenuID: "post", Date: pv.CreatedAt.Format("2006-01-02"), Excerpt: pv.Excerpt, Topic: pv.Topic,
		},
		Content: notice + template.HTML(contentHTML),
	}
	data.TOC = tmpl.ExtractTOC(contentHTML)
	s.web.fillSidebar(data)
	s.web.fillSEO(data, c)
	html, err := s.deps.Engine.Render("layout/base", data)
	if err != nil {
		log.Printf("render draft preview: %v", err)
		s.web.render500(c)
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}

// NoRoute returns the final fallback after Console has supplied its SPA index.
func (s *Server) NoRoute(consoleIndex []byte) gin.HandlerFunc {
	return func(c *gin.Context) {
		if consoleIndex != nil && strings.HasPrefix(c.Request.URL.Path, "/console") {
			c.Data(http.StatusOK, "text/html; charset=utf-8", consoleIndex)
			return
		}
		if s.pageHandler != nil && s.pageHandler.ShowByPath(c) {
			return
		}
		s.web.render404(c)
	}
}

// RenderSitePreview is passed to Console settings API without exposing route state.
func (s *Server) RenderSitePreview(sections map[string]json.RawMessage, pageName string) (string, error) {
	return s.web.renderSitePreview(sections, pageName)
}

// InvalidateContent invalidates search and sitemap response caches.
func (s *Server) InvalidateContent() { s.web.contentCache.invalidate() }

// RestoreDerivedState refreshes in-memory state after a backup restore.
func (s *Server) RestoreDerivedState() {
	if err := config.RebuildSite(s.deps.Store, s.deps.Site); err != nil {
		log.Printf("restore: rebuild site config: %v", err)
	}
	rebuildSearchIndex(s.deps.Store, s.searchIdx)
	s.web.invalidateSidebarCache()
	s.web.contentCache.invalidate()
	clearOGCache(s.ogCacheDir)
}

// Close flushes the asynchronous page-view buffer before the store is closed.
func (s *Server) Close() {
	if s.pvBuffer != nil {
		s.pvBuffer.close()
	}
}
