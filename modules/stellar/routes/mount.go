package routes

import (
	"encoding/json"

	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/gin-gonic/gin"

	"blog/internal/comments"
	"blog/internal/config"
	"blog/internal/content"
	"blog/internal/friends"
	ghpkg "blog/internal/github"
	"blog/internal/module"
	"blog/internal/og"
	"blog/internal/post"
	"blog/internal/search"
	"blog/internal/store"
	tmpl "blog/modules/stellar/viewmodel"
	stellarweb "blog/modules/stellar/web"
)

// Dependencies contains the shared services required by the Stellar SSR module.
// It deliberately contains no Console implementation details.
type Dependencies struct {
	AppVersion string
	App        *config.App
	Site       *atomic.Pointer[tmpl.Site]
	Store      *store.Store
	Posts      *post.Service
	Engine     *stellarweb.Engine
	Icons      *tmpl.IconRegistry
	OGGen      *og.Generator
	Search     *search.Index
	DevMode    bool
	Root       string
	UploadDir  string
	NotesPath  string
	Comments   *comments.Handler
	Friends    *friends.Aggregator
	GH         *ghpkg.Client
	Modules    *module.Manager
	Events     content.Events
}

// Server owns the Stellar route state and its derived caches.
type Server struct {
	deps        Dependencies
	web         *webServer
	pageHandler *PageHandler
	pvBuffer    *pageViewBuffer
	searchIdx   *search.Index
	ogCacheDir  string
	eventSub    content.Subscription
}

// New creates the SSR server and installs content-index write hooks.
func New(d Dependencies) (*Server, error) {
	searchIdx := d.Search
	if searchIdx == nil {
		searchIdx = setupSearchIndex(d.Store)
	} else if err := searchIdx.Ensure(); err != nil {
		log.Printf("search: ensure shared fts failed (%v), server search disabled", err)
		searchIdx = nil
	} else if count, err := searchIdx.Count(); err == nil && count == 0 {
		items := search.ItemsFromStore(d.Store)
		if err := searchIdx.Rebuild(items); err != nil {
			log.Printf("search: rebuild shared fts failed (%v), server search disabled", err)
			searchIdx = nil
		}
	}
	ogCacheDir := ""
	if d.App != nil {
		ogCacheDir = filepath.Join(filepath.Dir(d.App.DBPath), "ogcache")
	}
	postHook := newPostWriteHook(d.Store, searchIdx, d.OGGen)
	pageHook := newPageWriteHook(d.Store, searchIdx)
	momentHook := newMomentWriteHook(d.Store, searchIdx)

	web := newWebServer(webDeps{
		Site:      d.Site,
		Icons:     d.Icons,
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
	// Content caches are initialized by newWebServer. With app.Services, one
	// published event is the sole invalidation path for derived data.
	var eventSub content.Subscription
	if d.Events != nil {
		eventSub = d.Events.Subscribe(func(event content.Event) {
			switch event.Kind {
			case content.PostChanged:
				if event.PreviousSlug != "" && event.PreviousSlug != event.Slug {
					postHook("delete", event.PreviousSlug)
				}
				postHook(event.Op, event.Slug)
			case content.PageChanged:
				pageHook(event.Op, event.Slug)
			case content.MomentChanged:
				momentHook(event.Op, event.Slug)
			case content.Restored:
				rebuildSearchIndex(d.Store, searchIdx)
				clearOGCache(ogCacheDir)
				web.invalidateSidebarCache()
			}
			web.contentCache.invalidate()
		})
	} else {
		// Package-level callers and focused tests may construct a Server without
		// app.Services; retain their old hook contract in that mode.
		d.Store.SetPostWriteHook(func(op, slug string) { postHook(op, slug); web.contentCache.invalidate() })
		d.Store.SetPageWriteHook(func(op, slug string) { pageHook(op, slug); web.contentCache.invalidate() })
		d.Store.SetMomentWriteHook(func(op, slug string) { momentHook(op, slug); web.contentCache.invalidate() })
	}

	return &Server{
		deps:       d,
		web:        web,
		searchIdx:  searchIdx,
		ogCacheDir: ogCacheDir,
		eventSub:   eventSub,
	}, nil
}

// Mount registers all Stellar SSR routes and static assets on the shared router.
func (s *Server) Mount(r *gin.Engine) error {
	if !s.deps.DevMode {
		gin.SetMode(gin.ReleaseMode)
	}
	s.pvBuffer = newPageViewBuffer(s.deps.Store)
	s.web.installMiddleware(r, s.deps.App, s.pvBuffer)
	r.Use(s.web.stellarDynamicRoutes)
	if err := s.web.registerStaticRoutes(r); err != nil {
		return err
	}
	s.web.registerEmbedRoutes(r)

	pageHandler, err := NewPageHandler(s.deps.Store, s.deps.Posts, s.deps.Engine, s.deps.Site)
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

	momentHandler, err := NewMomentHandler(s.deps.Store, s.deps.Posts, s.deps.Engine, s.deps.Site, s.web.fillSidebar, s.web.fillSEO)
	if err != nil {
		return err
	}
	momentHandler.Register(r)
	s.web.momentH = momentHandler

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
	requestURL := requestScheme(c) + "://" + c.Request.Host + c.Request.URL.EscapedPath()
	site, err := stellarPreviewSite(s.deps.Site.Load(), requestURL)
	if err != nil {
		stellarError(c, err)
		return
	}
	slug := strings.Trim(pv.Slug, "/")
	if slug == "" {
		slug = "__preview__"
	}
	row := &store.Post{Slug: slug, Title: pv.Title, ContentMD: pv.ContentMD, Excerpt: pv.Excerpt, CreatedAt: pv.CreatedAt, UpdatedAt: pv.CreatedAt, Status: store.StatusDraft, StellarVersion: pv.StellarVersion, StellarConfig: pv.StellarConfig, BlogExtensions: pv.BlogExtensions}
	snap, err := s.web.stellarSnapshotAt(c.Request.Context(), site, row, true)
	if err != nil {
		stellarError(c, err)
		return
	}
	for _, r := range snap.records {
		if r.post == row {
			data := s.web.stellarView(snap, r)
			data.Stellar.Input.RuntimeData = stellarCopyObject(data.Stellar.Input.RuntimeData)
			data.Stellar.Input.RuntimeData["preview"] = true
			data.Stellar.Input.RuntimeData["nativeComments"] = map[string]any{"enabled": false}
			data.Comments = nil
			data.CommentCount = 0
			stellarNotice(data, `<div class="blog-preview-notice">预览模式：这是未发布的草稿，仅供发布前查看</div>`)
			s.web.outputStellar(c, data, http.StatusOK)
			return
		}
	}
	s.web.render404(c)
}

// NoRoute returns the final fallback after Console has supplied its SPA index.
func (s *Server) NoRoute(consoleIndex []byte) gin.HandlerFunc {
	return func(c *gin.Context) {
		if consoleIndex != nil && strings.HasPrefix(c.Request.URL.Path, "/console") {
			c.Data(http.StatusOK, "text/html; charset=utf-8", consoleIndex)
			return
		}
		// Gin 的单参数路由不能匹配包含 / 的分类路径；把未命中的
		// /categories/a/b/ 和嵌套分类 RSS 统一交给分类路径解析器。
		if strings.HasPrefix(c.Request.URL.Path, "/categories/") {
			raw := strings.TrimPrefix(c.Request.URL.Path, "/categories/")
			decoded, err := url.PathUnescape(strings.TrimSuffix(raw, "/"))
			if err == nil && decoded != "" {
				if strings.HasSuffix(decoded, "/rss.xml") {
					name := strings.TrimSuffix(decoded, "/rss.xml")
					s.web.serveStellarFeed(c, "rss", "categories", name)
					return
				}
				s.web.handleCategoryPath(c, decoded)
				return
			}
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

// Close flushes the asynchronous page-view buffer before the store is closed.
func (s *Server) Close() {
	if s.eventSub != nil {
		s.eventSub.Close()
	}
	if s.pvBuffer != nil {
		s.pvBuffer.close()
	}
}
