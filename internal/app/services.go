// Package app owns shared service construction and process lifecycle.
package app

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"log"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"blog/internal/backup"
	"blog/internal/comments"
	"blog/internal/config"
	"blog/internal/content"
	"blog/internal/friends"
	ghpkg "blog/internal/github"
	"blog/internal/identity"
	"blog/internal/linkwatch"
	"blog/internal/maintenance"
	"blog/internal/module"
	_ "blog/internal/module/core"
	_ "blog/internal/module/moments"
	_ "blog/internal/module/perf"
	_ "blog/internal/module/seo"
	storagemodule "blog/internal/module/storage"
	"blog/internal/notify"
	"blog/internal/og"
	ogfonts "blog/internal/og/fonts"
	"blog/internal/plugin"
	"blog/internal/post"
	"blog/internal/rag"
	"blog/internal/search"
	seedpkg "blog/internal/seed"
	"blog/internal/settings"
	attachmentstorage "blog/internal/storage"
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
	// ResolvedApp使用已持有运行锁的路径快照，避免启动再次解析到另一个数据库。
	ResolvedApp *config.App
}

// Services is the process-wide owner of shared business services. HTTP module
// packages receive these instances but do not construct or close them.
type Services struct {
	Maintenance *maintenance.Coordinator
	devMode     bool
	App         *config.App
	Store       *store.Store
	Settings    *settings.Manager
	Site        *atomic.Pointer[settings.Site] // migration bridge; use Settings in new code
	Post        *post.Service
	Comment     *comments.Handler
	Identity    *identity.Service
	Search      *search.Index
	RAG         *rag.Service
	Backup      *backup.Service
	Notify      *notify.Service
	Friends     *friends.Aggregator
	LinkWatch   *linkwatch.Watcher
	GitHub      *ghpkg.Client
	OG          *og.Generator
	Modules     *module.Manager
	Storage     *attachmentstorage.Manager
	Plugins     *plugin.Runtime
	Events      content.Events
	Icons       *settings.IconRegistry
	SitePath    string
	closed      atomic.Bool
	background  sync.Once
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
		configPath = filepath.Join(root, "config", "app.yml")
	}
	var appConfig *config.App
	var err error
	if options.ResolvedApp != nil {
		snapshot := *options.ResolvedApp
		appConfig = &snapshot
	} else {
		appConfig, err = config.LoadApp(configPath)
		if err != nil {
			return nil, fmt.Errorf("load app config: %w", err)
		}
		if options.Root != "" {
			appConfig.Root = options.Root
		}
		appConfig = config.ResolveApp(appConfig)
	}

	sitePath := filepath.Join(appConfig.Root, "config", "site.yml")
	if err := recoverBeforeBootstrap(appConfig, sitePath); err != nil {
		log.Printf("startup recovery incomplete: %v", err)
		gate := maintenance.New()
		gate.Block()
		return &Services{App: appConfig, Maintenance: gate, SitePath: sitePath}, nil
	}
	site, err := config.LoadSite(sitePath)
	if err != nil {
		if !options.DevMode {
			return nil, fmt.Errorf("load site config: %w", err)
		}
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
	if options.Seed || appConfig.SeedOnStart {
		// 显式 seed 先检查门禁，不能通过后面的旧 Bootstrap 补配置来迁移。
		if _, err := config.LoadStellarV2(st); err != nil {
			return fail(fmt.Errorf("%w: seed requires reviewed Stellar v2 migration: %w", store.ErrStellarMigrationRequired, err))
		}
	}
	if err := st.EnsureTopicsTable(); err != nil {
		return fail(fmt.Errorf("ensure topics table: %w", err))
	}
	if err := st.EnsureNotebooksTable(); err != nil {
		return fail(fmt.Errorf("ensure notebooks table: %w", err))
	}
	if err := st.EnsureNotesTable(); err != nil {
		return fail(fmt.Errorf("ensure notes table: %w", err))
	}
	if legacy, err := st.DetectLegacyNotebookSchema(); err != nil {
		return fail(err)
	} else if legacy.Required() {
		// 保留 Console 迁移预览入口，旧集合访问由 store 返回明确门禁。
		log.Printf("notebook migration pending: %v", legacy.Error())
	}
	pluginRuntime := plugin.NewRuntime()
	if err := pluginRuntime.LoadDir(filepath.Join(appConfig.Root, "plugins")); err != nil {
		return fail(fmt.Errorf("load script plugins: %w", err))
	}
	if err := st.MigrateTopicBackfill(); err != nil {
		return fail(fmt.Errorf("migrate topic backfill: %w", err))
	}

	sitePtr := &atomic.Pointer[settings.Site]{}
	sitePtr.Store(site)
	settingsMgr := settings.NewManagerFromPointer(sitePtr)
	posts := post.NewService(filepath.Join(appConfig.Root, "content", "posts"))
	posts.SetPluginRuntime(pluginRuntime)
	posts.SetSettingsManager(settingsMgr)
	posts.SetIconFunc(icons.Get)
	posts.SetLinksFunc(func() ([]settings.LinkGroup, string) {
		live := sitePtr.Load()
		return live.Links, live.Default.Link
	})
	posts.SetAlbumsLinksFunc(func() ([]settings.LinkGroup, string) {
		live := sitePtr.Load()
		return live.Links, live.Default.Cover
	})
	posts.SetWikiTreeProvider(func(projectID string) (bool, []settings.WikiTreeNode) {
		exists := false
		if projects, err := st.ListWikiProjects(); err == nil {
			for _, p := range projects {
				if p.ID == projectID && p.Visible {
					exists = true
					break
				}
			}
		}
		if !exists {
			live := sitePtr.Load()
			for _, p := range live.Wiki {
				if p.ID == projectID {
					exists = true
					break
				}
			}
		}
		if !exists {
			return false, nil
		}
		docs, err := st.ListWikidocs(projectID)
		if err != nil {
			return true, nil
		}
		var published []*store.WikiDoc
		for _, d := range docs {
			if d.Status == store.StatusPublished {
				published = append(published, d)
			}
		}
		if len(published) == 0 {
			return true, nil
		}
		treeNodes := store.BuildDocTreeFromDocs(published)
		var toWikiTreeNodes func([]store.TreeNode) []settings.WikiTreeNode
		toWikiTreeNodes = func(nodes []store.TreeNode) []settings.WikiTreeNode {
			out := make([]settings.WikiTreeNode, 0, len(nodes))
			for _, n := range nodes {
				if n.Doc == nil || n.Doc.Status != store.StatusPublished {
					continue
				}
				out = append(out, settings.WikiTreeNode{
					Doc: &settings.WikiDocView{
						Slug:  n.Doc.Slug,
						Title: n.Doc.Title,
					},
					Children: toWikiTreeNodes(n.Children),
				})
			}
			return out
		}
		return true, toWikiTreeNodes(treeNodes)
	})
	posts.SetMomentsFunc(func(limit int) []*settings.MomentItem {
		if limit < 1 {
			limit = 5
		}
		moments, err := st.ListPublicMoments(1, limit)
		if err != nil {
			log.Printf("list moments for shortcode: %v", err)
			return nil
		}
		items := make([]*settings.MomentItem, 0, len(moments))
		for _, moment := range moments {
			items = append(items, &settings.MomentItem{
				ID:          moment.ID,
				ContentHTML: template.HTML(moment.ContentHTML),
				Images:      store.DecodeMomentImages(moment.Images),
				TimeAgo:     comments.TimeAgo(moment.CreatedAt),
				Date:        moment.CreatedAt.Format("2006-01-02 15:04"),
			})
		}
		return items
	})

	events := content.NewEvents()
	st.SetContentEvents(events)
	notifySvc := notify.New(st)
	notifySvc.SetPluginRuntime(pluginRuntime)
	commentHandler := comments.New(st, sitePtr)
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
	if !options.DevMode && !sitePtr.Load().Admin.AuthEnabled {
		return fail(fmt.Errorf("production requires an admin password; configure site.yml admin.password or migrate an admin user"))
	}

	attachmentStorage, err := storagemodule.NewManager(st, appConfig.UploadDir)
	if err != nil {
		return fail(fmt.Errorf("init attachment storage: %w", err))
	}

	seedOnStart := options.Seed || appConfig.SeedOnStart
	if seedOnStart {
		n, err := seedpkg.ImportStellar(st, posts, filepath.Join(appConfig.Root, "content"), false)
		if err != nil {
			return fail(fmt.Errorf("seed: %w", err))
		}
		log.Printf("seed: imported %d canonical content records", n)
	}

	// 增量自动发现：未做全量 seed 时，把 content/posts 里 DB 缺失的新文件补录进来
	// （只增不改，Console 编辑过的记录不被文件覆盖），新文章无需手动 -seed 即可上列表。
	discovered := 0
	if !seedOnStart {
		n, err := seedpkg.ImportStellar(st, posts, filepath.Join(appConfig.Root, "content"), true)
		if err != nil {
			if !errors.Is(err, store.ErrStellarMigrationRequired) {
				return fail(fmt.Errorf("discover new posts: %w", err))
			}
			// Console 的迁移预览仍可打开；普通启动不得自动迁移/降级 seed。
			log.Printf("seed: automatic discovery requires reviewed Stellar v2 migration")
		}
		discovered = n
		if n > 0 {
			log.Printf("seed: discovered %d new posts from content/posts", n)
		}
	}

	// Search.ItemsFromStore reads pages and moments. Ensure their schemas before
	// FTS backfill; Stellar route constructors run later in main.go.
	if err := st.EnsurePagesTable(); err != nil {
		return fail(fmt.Errorf("ensure pages table before search: %w", err))
	}
	if err := st.EnsureMomentsTable(); err != nil {
		return fail(fmt.Errorf("ensure moments table before search: %w", err))
	}
	searchIdx := search.New(st.DB())
	if err := searchIdx.Ensure(); err != nil {
		log.Printf("search: ensure fts failed (%v), server search disabled", err)
		searchIdx = nil
	} else {
		count, countErr := searchIdx.Count()
		if seedOnStart || discovered > 0 || (countErr == nil && count == 0) {
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
	moduleDeps := module.Deps{Store: st, Posts: posts, Storage: attachmentStorage, Site: sitePtr, Modules: moduleMgr}
	for _, mod := range module.All() {
		if err := mod.Migrate(moduleDeps); err != nil {
			return fail(fmt.Errorf("module %s migrate: %w", mod.Meta().ID, err))
		}
	}

	// RAG is an optional enhancement: schema/load failures are logged and do
	// not prevent the blog, Console, or public content routes from starting.
	var ragSvc *rag.Service
	if candidate, ragErr := rag.New(st, events); ragErr != nil {
		log.Printf("rag: disabled during bootstrap: %v", ragErr)
	} else {
		ragSvc = candidate
	}

	friendsAgg := friends.New(sitePtr)
	friendsAgg.SetMaintenance(st.Maintenance())
	linkWatch := linkwatch.New(st, sitePtr, appConfig.UploadDir)
	linkWatch.SetOnChanged(friendsAgg.Refresh)
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
		App: appConfig, Maintenance: st.Maintenance(), devMode: options.DevMode, Store: st, Settings: settingsMgr, Site: sitePtr, Post: posts,
		Comment: commentHandler, Identity: identity.NewService(st), Search: searchIdx, RAG: ragSvc,
		Backup: backupSvc, Notify: notifySvc, Friends: friendsAgg, LinkWatch: linkWatch,
		GitHub: ghClient, OG: ogGen, Modules: moduleMgr, Storage: attachmentStorage, Plugins: pluginRuntime, Events: events, Icons: icons, SitePath: sitePath,
	}
	backupSvc.SetOperationLock(func() (func(), error) { return maintenance.LockOperation(operationLockPath(appConfig)) })
	backupSvc.SetBackupLock(st.Maintenance().Acquire)
	backupSvc.SetRestoreRebuild(services.rebuildRestoredState)
	backupSvc.SetRestoreHook(func() {
		if ragSvc != nil {
			ragSvc.QueueRebuild()
		}
		friendsAgg.Refresh()
	})

	return services, nil
}

// StartBackground starts process loops after both HTTP modules have mounted.
// In particular, Stellar creates system pages during Mount; automatic backup
// must not race that initialization and capture a partial first snapshot.
func (s *Services) StartBackground() {
	if s == nil || s.closed.Load() || (s.Maintenance != nil && s.Maintenance.Active()) {
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
		if s.RAG != nil {
			s.RAG.Start()
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
	if s.RAG != nil {
		if err := s.RAG.Close(ctx); closeErr == nil {
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
