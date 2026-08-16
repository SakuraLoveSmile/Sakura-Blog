package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"blog/internal/backup"
	"blog/internal/comment"
	"blog/internal/config"
	"blog/internal/friends"
	ghpkg "blog/internal/github"
	"blog/internal/linkwatch"
	"blog/internal/module"
	_ "blog/internal/module/perf"
	_ "blog/internal/module/seo"
	"blog/internal/notify"
	"blog/internal/og"
	ogfonts "blog/internal/og/fonts"
	"blog/internal/post"
	seedpkg "blog/internal/seed"
	"blog/internal/store"
	tmpl "blog/internal/template"
	console "blog/modules/console"
	consoleapi "blog/modules/console/api"
	"blog/modules/stellar"
	stellarapi "blog/modules/stellar/api"
	"blog/modules/stellar/web/assets"
)

// main only loads configuration, initializes shared services, and mounts the two
// HTTP modules. Product routes live under modules/console and modules/stellar.
func main() {
	var (
		root       string
		seed       bool
		devMode    bool
		iconsPath  string
		configPath string
	)
	flag.StringVar(&root, "root", "", "工作根目录（定位 config/DB/data；空则用 cwd 或 app.yml）")
	flag.BoolVar(&seed, "seed", false, "启动时把 content/posts/*.md 导入 SQLite（按 slug UPSERT）")
	flag.BoolVar(&devMode, "dev", false, "开发模式：从磁盘读 icons/templates/assets（默认走 go:embed）")
	flag.StringVar(&iconsPath, "icons", "", "icons.yml 路径（仅 -dev 模式生效，默认 <root>/internal/template/data/icons.yml）")
	flag.StringVar(&configPath, "config", "", "app.yml 路径（默认 <root>/backend/config/app.yml）")
	flag.Parse()

	bootstrapRoot := root
	if bootstrapRoot == "" {
		bootstrapRoot = os.Getenv("BLOG_ROOT")
	}
	if bootstrapRoot == "" {
		bootstrapRoot, _ = os.Getwd()
	}
	bootstrapRoot, _ = filepath.Abs(bootstrapRoot)
	if configPath == "" {
		configPath = filepath.Join(bootstrapRoot, "backend", "config", "app.yml")
	}
	app, err := config.LoadApp(configPath)
	if err != nil {
		log.Fatalf("load app config: %v", err)
	}
	if root != "" {
		app.Root = root
	}
	app = config.ResolveApp(app)

	reg := loadIcons(app.Root, devMode, iconsPath)
	engine := loadTemplates(app.Root, devMode, reg)
	tmpl.SetAssetVersioner(newAssetVersioner(devMode, app.Root))
	posts := post.NewService(filepath.Join(app.Root, "content", "posts"))
	posts.SetIconFunc(reg.Get)
	tmpl.SetMarkdownRender(func(s string) template.HTML {
		h, renderErr := posts.Render(s)
		if renderErr != nil {
			log.Printf("render footer markdown: %v", renderErr)
			return ""
		}
		return template.HTML(h)
	})

	st, err := store.New(app.DBPath)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer st.Close()
	log.Printf("sqlite ready: %s", app.DBPath)
	if err := st.EnsureTopicsTable(); err != nil {
		log.Fatalf("ensure topics table: %v", err)
	}
	if err := st.MigrateTopicBackfill(); err != nil {
		log.Fatalf("migrate topic backfill: %v", err)
	}

	ogCacheDir := filepath.Join(filepath.Dir(app.DBPath), "ogcache")
	ogFontPath := ""
	if devMode {
		ogFontPath = filepath.Join(app.Root, "internal", "og", "fonts", "LXGWWenKai-Regular.ttf")
	} else if data, readErr := ogfonts.FS.ReadFile("LXGWWenKai-Regular.ttf"); readErr == nil {
		_ = os.MkdirAll(ogCacheDir, 0o755)
		ogFontPath = filepath.Join(ogCacheDir, ".font.ttf")
		_ = os.WriteFile(ogFontPath, data, 0o644)
	}
	ogGen := og.New(ogFontPath, ogCacheDir)

	siteConfigPath := filepath.Join(app.Root, "backend", "config", "site.yml")
	site, err := config.LoadSite(siteConfigPath)
	if err != nil {
		log.Printf("load site config failed (%v), fallback to DefaultSite", err)
		site = config.DefaultSite()
	} else {
		log.Printf("loaded site config: %s", siteConfigPath)
	}
	sitePtr := &atomic.Pointer[tmpl.Site]{}
	sitePtr.Store(site)
	posts.SetLinksFunc(func() ([]tmpl.LinkGroup, string) {
		live := sitePtr.Load()
		return live.Links, live.Default.Link
	})
	if site.Comments.GravatarMirror == "" {
		site.Comments.GravatarMirror = "https://weavatar.com/avatar"
	}
	if site.Comments.AdminBadge == "" {
		site.Comments.AdminBadge = "博主"
	}
	if site.Comments.Title == "" {
		site.Comments.Title = "评论"
	}
	if err := config.Bootstrap(st, site); err != nil {
		log.Fatalf("bootstrap settings: %v", err)
	}
	if err := config.RebuildSite(st, sitePtr); err != nil {
		log.Fatalf("rebuild site from db: %v", err)
	}

	commentHandler := comment.New(st, sitePtr)
	commentHandler.SetUploadDir(app.UploadDir)
	notifySvc := notify.New(st)
	commentHandler.SetNotifier(notifySvc)
	ghClient := ghpkg.New(ghpkg.Config{
		Token:    app.GitHubToken,
		CacheDir: filepath.Join(filepath.Dir(app.DBPath), "ghcache"),
	})
	posts.SetGHClient(stellar.GitHubAdapter(ghClient))

	// The search index and write hooks are created inside Stellar because they
	// back public SSR resources (/search.json and /sitemap.xml).
	r := gin.New()
	stellarServer, err := stellar.Mount(r, stellar.Dependencies{
		App: app, Site: sitePtr, Store: st, Posts: posts, Engine: engine, OGGen: ogGen,
		DevMode: devMode, Root: app.Root, UploadDir: app.UploadDir,
		NotesPath: filepath.Join(app.Root, "content", "notes.md"), Comments: commentHandler,
	})
	if err != nil {
		log.Fatalf("mount stellar: %v", err)
	}
	defer stellarServer.Close()

	if seed || app.SeedOnStart {
		n, _ := seedpkg.Count(posts)
		if err := seedpkg.Import(st, posts); err != nil {
			log.Fatalf("seed: %v", err)
		}
		if err := st.MigrateTopicBackfill(); err != nil {
			log.Fatalf("migrate topic backfill after seed: %v", err)
		}
		log.Printf("seed: imported %d posts from content/posts", n)
	}

	backupSvc := backup.New(backup.Config{
		Store: st, Posts: posts, UploadDir: app.UploadDir, SitePath: siteConfigPath,
		BackupDir: filepath.Join(filepath.Dir(app.DBPath), "backups"), Keep: app.BackupKeep, Auto: app.AutoBackup,
	})
	if app.AutoBackup {
		backupSvc.StartAuto(24 * time.Hour)
	}
	backupSvc.SetRestoreHook(stellarServer.RestoreDerivedState)

	if err := st.EnsureModuleStatesTable(); err != nil {
		log.Fatalf("ensure module_states table: %v", err)
	}
	moduleMgr, err := module.NewManager(st)
	if err != nil {
		log.Fatalf("init module manager: %v", err)
	}
	moduleDeps := module.Deps{Store: st, Posts: posts, Site: sitePtr, Modules: moduleMgr}
	for _, m := range module.All() {
		if err := m.Migrate(moduleDeps); err != nil {
			log.Fatalf("module %s migrate: %v", m.Meta().ID, err)
		}
		m.RegisterPublic(r, moduleDeps)
	}
	log.Printf("modules: %d registered", len(module.All()))

	friendsAgg := friends.New(sitePtr)
	if err := st.EnsureFeedArticlesTable(); err != nil {
		log.Fatalf("ensure feed_articles table: %v", err)
	}
	friendsAgg.SetOnArticles(func(friendTitle, friendURL string, articles []friends.Post) {
		for _, p := range articles {
			if p.Title == "error" || p.Title == "empty" || p.Link == "" {
				continue
			}
			if err := st.UpsertFeedArticle(&store.FeedArticle{
				FriendTitle: friendTitle, FriendURL: friendURL, Title: p.Title, Link: p.Link,
				Summary: p.Summary, Published: p.Published, GUID: p.Link,
			}); err != nil {
				log.Printf("upsert feed article %s from %s: %v", p.Link, friendTitle, err)
			}
		}
	})
	friendsAgg.Start()
	linkWatch := linkwatch.New(st, sitePtr, app.UploadDir)
	linkWatch.Start()

	consoleHandler, err := console.Mount(r, console.MountDeps{
		API: consoleapi.MountDeps{Config: consoleapi.Config{
			Store: st, Posts: posts, Site: sitePtr, Comments: commentHandler, Backups: backupSvc,
			Modules: moduleMgr, Notify: notifySvc, Icons: reg,
			RenderSitePreview: stellarServer.RenderSitePreview,
			OnContentChanged:  stellarServer.InvalidateContent,
			UploadDir:         app.UploadDir, MaxUploadMB: app.MaxUploadMB, LinkWatch: linkWatch,
		}, LinkWatch: linkWatch, GH: ghClient, SkipPublic: true},
		DevMode: devMode, Root: app.Root, Fallback: stellarServer.NoRoute(nil),
	})
	if err != nil {
		log.Fatalf("mount console: %v", err)
	}
	if err := stellarapi.Mount(r, stellarapi.Dependencies{Store: st, Friends: friendsAgg, GH: ghClient}); err != nil {
		log.Fatalf("mount stellar api: %v", err)
	}
	consoleHandler.SetOnLinksChanged(friendsAgg.Refresh)

	addr := app.ListenAddr
	srv := &http.Server{Addr: addr, Handler: r}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go func() {
		fmt.Printf("Stellar blog server listening on http://localhost%s\n", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()
	<-ctx.Done()
	log.Println("shutting down gracefully (timeout 30s)...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
	log.Println("server stopped")
}

func loadIcons(root string, devMode bool, iconsPath string) *tmpl.IconRegistry {
	var (
		reg *tmpl.IconRegistry
		err error
	)
	if devMode {
		if iconsPath == "" {
			iconsPath = filepath.Join(root, "internal", "template", "data", "icons.yml")
		}
		reg, err = tmpl.LoadIcons(iconsPath)
		if err != nil {
			log.Fatalf("load icons (dev): %v", err)
		}
		log.Printf("loaded %d icons from %s (dev)", reg.Len(), iconsPath)
		return reg
	}
	reg, err = tmpl.LoadIconsEmbed()
	if err != nil {
		log.Fatalf("load icons (embed): %v", err)
	}
	log.Printf("loaded %d icons from embed", reg.Len())
	return reg
}

func loadTemplates(root string, devMode bool, reg *tmpl.IconRegistry) *tmpl.Engine {
	var (
		engine *tmpl.Engine
		err    error
	)
	if devMode {
		engine, err = tmpl.NewEngine(filepath.Join(root, "internal", "template"), reg)
		log.Printf("templates loaded from disk (dev)")
	} else {
		engine, err = tmpl.NewEngineFromEmbed(reg)
		log.Printf("templates loaded from embed")
	}
	if err != nil {
		log.Fatalf("load templates: %v", err)
	}
	return engine
}

// newAssetVersioner hashes Stellar assets so disk-mode changes invalidate caches.
func newAssetVersioner(devMode bool, root string) func(string) string {
	rel := func(p string) string { return strings.TrimPrefix(strings.TrimPrefix(p, "/assets/"), "assets/") }
	var mu sync.Mutex
	cache := map[string]string{}
	return func(p string) string {
		if !devMode {
			mu.Lock()
			if v, ok := cache[p]; ok {
				mu.Unlock()
				return v
			}
			mu.Unlock()
		}
		var data []byte
		var err error
		if devMode {
			data, err = os.ReadFile(filepath.Join(root, "modules", "stellar", "web", "assets", rel(p)))
		} else {
			data, err = assets.FS.ReadFile(rel(p))
		}
		if err != nil {
			return "0"
		}
		sum := sha256.Sum256(data)
		version := hex.EncodeToString(sum[:])[:8]
		if !devMode {
			mu.Lock()
			cache[p] = version
			mu.Unlock()
		}
		return version
	}
}
