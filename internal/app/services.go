// Package app owns shared service construction and process lifecycle.
package app

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"blog/internal/backup"
	"blog/internal/comment"
	"blog/internal/config"
	"blog/internal/content"
	"blog/internal/friends"
	ghpkg "blog/internal/github"
	"blog/internal/identity"
	"blog/internal/linkwatch"
	"blog/internal/module"
	_ "blog/internal/module/perf"
	_ "blog/internal/module/seo"
	"blog/internal/notify"
	"blog/internal/og"
	ogfonts "blog/internal/og/fonts"
	"blog/internal/post"
	"blog/internal/search"
	seedpkg "blog/internal/seed"
	"blog/internal/settings"
	"blog/internal/store"
)

// Options controls shared service bootstrap. Root and ConfigPath preserve the
// existing -root/-config path resolution rules.
type Options struct {
	Root       string
	ConfigPath string
	IconsPath  string
	DevMode    bool
	Seed       bool
}

// Services is the process-wide owner of shared business services. HTTP module
// packages receive these instances but do not construct or close them.
type Services struct {
	App        *config.App
	Store      *store.Store
	Settings   *settings.Manager
	Site       *atomic.Pointer[settings.Site] // migration bridge; use Settings in new code
	Post       *post.Service
	Comment    *comment.Handler
	Identity   *identity.Service
	Search     *search.Index
	Backup     *backup.Service
	Notify     *notify.Service
	Friends    *friends.Aggregator
	LinkWatch  *linkwatch.Watcher
	GitHub     *ghpkg.Client
	OG         *og.Generator
	Modules    *module.Manager
	Events     content.Events
	Icons      *settings.IconRegistry
	SitePath   string
	closed     atomic.Bool
	background sync.Once
}

// Bootstrap loads configuration and constructs all shared services.
func Bootstrap(options Options) (*Services, error) {
	root := options.Root
	if root == "" {
		root = os.Getenv("BLOG_ROOT")
	}
	if root == "" {
		root, _ = os.Getwd()
	}
	root, _ = filepath.Abs(root)
	configPath := options.ConfigPath
	if configPath == "" {
		configPath = filepath.Join(root, "backend", "config", "app.yml")
	}
	appConfig, err := config.LoadApp(configPath)
	if err != nil {
		return nil, fmt.Errorf("load app config: %w", err)
	}
	if options.Root != "" {
		appConfig.Root = options.Root
	}
	appConfig = config.ResolveApp(appConfig)

	sitePath := filepath.Join(appConfig.Root, "backend", "config", "site.yml")
	site, err := config.LoadSite(sitePath)
	if err != nil {
		log.Printf("load site config failed (%v), fallback to DefaultSite", err)
		site = config.DefaultSite()
	} else {
		log.Printf("loaded site config: %s", sitePath)
	}
	if site.Comments.GravatarMirror == "" {
		site.Comments.GravatarMirror = "https://weavatar.com/avatar"
	}
	if site.Comments.AdminBadge == "" {
		site.Comments.AdminBadge = "博主"
	}
	if site.Comments.Title == "" {
		site.Comments.Title = "评论"
	}

	icons, err := loadIcons(appConfig.Root, options.DevMode, options.IconsPath)
	if err != nil {
		return nil, fmt.Errorf("load icons: %w", err)
	}
	st, err := store.New(appConfig.DBPath)
	if err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}
	fail := func(err error) (*Services, error) {
		st.Close()
		return nil, err
	}
	if err := st.EnsureTopicsTable(); err != nil {
		return fail(fmt.Errorf("ensure topics table: %w", err))
	}
	if err := st.MigrateTopicBackfill(); err != nil {
		return fail(fmt.Errorf("migrate topic backfill: %w", err))
	}

	sitePtr := &atomic.Pointer[settings.Site]{}
	sitePtr.Store(site)
	settingsMgr := settings.NewManagerFromPointer(sitePtr)
	posts := post.NewService(filepath.Join(appConfig.Root, "content", "posts"))
	posts.SetSettingsManager(settingsMgr)
	posts.SetIconFunc(icons.Get)
	posts.SetLinksFunc(func() ([]settings.LinkGroup, string) {
		live := sitePtr.Load()
		return live.Links, live.Default.Link
	})

	events := content.NewEvents()
	st.SetContentEvents(events)
	notifySvc := notify.New(st)
	commentHandler := comment.New(st, sitePtr)
	commentHandler.SetUploadDir(appConfig.UploadDir)
	commentHandler.SetNotifier(notifySvc)
	ghClient := ghpkg.New(ghpkg.Config{
		Token:    appConfig.GitHubToken,
		CacheDir: filepath.Join(filepath.Dir(appConfig.DBPath), "ghcache"),
	})
	posts.SetGHClient(githubPostAdapter{client: ghClient})

	ogCacheDir := filepath.Join(filepath.Dir(appConfig.DBPath), "ogcache")
	ogFontPath := ""
	if options.DevMode {
		ogFontPath = filepath.Join(appConfig.Root, "internal", "og", "fonts", "LXGWWenKai-Regular.ttf")
	} else if data, readErr := ogfonts.FS.ReadFile("LXGWWenKai-Regular.ttf"); readErr == nil {
		_ = os.MkdirAll(ogCacheDir, 0o755)
		ogFontPath = filepath.Join(ogCacheDir, ".font.ttf")
		_ = os.WriteFile(ogFontPath, data, 0o644)
	}
	ogGen := og.New(ogFontPath, ogCacheDir)

	if err := config.Bootstrap(st, site); err != nil {
		return fail(fmt.Errorf("bootstrap settings: %w", err))
	}
	if err := config.RebuildSite(st, sitePtr); err != nil {
		return fail(fmt.Errorf("rebuild site from db: %w", err))
	}

	seedOnStart := options.Seed || appConfig.SeedOnStart
	if seedOnStart {
		n, _ := seedpkg.Count(posts)
		if err := seedpkg.Import(st, posts); err != nil {
			return fail(fmt.Errorf("seed: %w", err))
		}
		if err := st.MigrateTopicBackfill(); err != nil {
			return fail(fmt.Errorf("migrate topic backfill after seed: %w", err))
		}
		log.Printf("seed: imported %d posts from content/posts", n)
	}

	searchIdx := search.New(st.DB())
	if err := searchIdx.Ensure(); err != nil {
		log.Printf("search: ensure fts failed (%v), server search disabled", err)
		searchIdx = nil
	} else {
		count, countErr := searchIdx.Count()
		if seedOnStart || (countErr == nil && count == 0) {
			if err := searchIdx.Rebuild(search.ItemsFromStore(st)); err != nil {
				log.Printf("search: rebuild failed (%v), server search disabled", err)
				searchIdx = nil
			}
		}
	}

	if err := st.EnsureModuleStatesTable(); err != nil {
		return fail(fmt.Errorf("ensure module_states table: %w", err))
	}
	moduleMgr, err := module.NewManager(st)
	if err != nil {
		return fail(fmt.Errorf("init module manager: %w", err))
	}
	moduleDeps := module.Deps{Store: st, Posts: posts, Site: sitePtr, Modules: moduleMgr}
	for _, mod := range module.All() {
		if err := mod.Migrate(moduleDeps); err != nil {
			return fail(fmt.Errorf("module %s migrate: %w", mod.Meta().ID, err))
		}
	}

	friendsAgg := friends.New(sitePtr)
	linkWatch := linkwatch.New(st, sitePtr, appConfig.UploadDir)
	backupSvc := backup.New(backup.Config{
		Store: st, Posts: posts, UploadDir: appConfig.UploadDir, SitePath: sitePath,
		BackupDir: filepath.Join(filepath.Dir(appConfig.DBPath), "backups"), Keep: appConfig.BackupKeep, Auto: appConfig.AutoBackup,
	})
	if err := st.EnsureFeedArticlesTable(); err != nil {
		return fail(fmt.Errorf("ensure feed_articles table: %w", err))
	}
	friendsAgg.SetOnArticles(func(friendTitle, friendURL string, articles []friends.Post) {
		for _, article := range articles {
			if article.Title == "error" || article.Title == "empty" || article.Link == "" {
				continue
			}
			if err := st.UpsertFeedArticle(&store.FeedArticle{
				FriendTitle: friendTitle, FriendURL: friendURL, Title: article.Title, Link: article.Link,
				Summary: article.Summary, Published: article.Published, GUID: article.Link,
			}); err != nil {
				log.Printf("upsert feed article %s from %s: %v", article.Link, friendTitle, err)
			}
		}
	})
	services := &Services{
		App: appConfig, Store: st, Settings: settingsMgr, Site: sitePtr, Post: posts,
		Comment: commentHandler, Identity: identity.NewService(st), Search: searchIdx,
		Backup: backupSvc, Notify: notifySvc, Friends: friendsAgg, LinkWatch: linkWatch,
		GitHub: ghClient, OG: ogGen, Modules: moduleMgr, Events: events, Icons: icons, SitePath: sitePath,
	}
	backupSvc.SetRestoreHook(func() {
		if err := config.RebuildSite(st, sitePtr); err != nil {
			log.Printf("restore: rebuild site config: %v", err)
		}
		events.Publish(content.Event{Kind: content.Restored})
	})

	return services, nil
}

// StartBackground starts process loops after both HTTP modules have mounted.
// In particular, Stellar creates system pages during Mount; automatic backup
// must not race that initialization and capture a partial first snapshot.
func (s *Services) StartBackground() {
	if s == nil || s.closed.Load() {
		return
	}
	s.background.Do(func() {
		if s.App != nil && s.App.AutoBackup && s.Backup != nil {
			s.Backup.StartAuto(24 * time.Hour)
		}
		if s.Friends != nil {
			s.Friends.Start()
		}
		if s.LinkWatch != nil {
			s.LinkWatch.Start()
		}
	})
}

// Close releases shared resources. Module-specific lifecycle objects are
// closed by their Mount return values before this method is called.
func (s *Services) Close(ctx context.Context) error {
	if s == nil || s.closed.Swap(true) {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	var closeErr error
	if s.Backup != nil {
		closeErr = s.Backup.Close(ctx)
	}
	if s.Friends != nil {
		if err := s.Friends.Close(ctx); closeErr == nil {
			closeErr = err
		}
	}
	if s.LinkWatch != nil {
		if err := s.LinkWatch.Close(ctx); closeErr == nil {
			closeErr = err
		}
	}
	if s.Store != nil {
		if err := s.Store.Close(); closeErr == nil {
			closeErr = err
		}
	}
	return closeErr
}
