package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"blog/internal/app"
	"blog/internal/httpapi"
	console "blog/modules/console"
	"blog/modules/stellar"
)

// main is the composition root: parse process flags, bootstrap shared
// services, mount the two top-level HTTP modules, and own server shutdown.
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
	flag.StringVar(&iconsPath, "icons", "", "icons.yml 路径（仅 -dev 模式生效，默认 <root>/internal/settings/data/icons.yml）")
	flag.StringVar(&configPath, "config", "", "app.yml 路径（默认 <root>/config/app.yml）")
	flag.Parse()
	if flag.NArg() > 0 {
		if flag.Arg(0) != "stellar" {
			fmt.Fprintln(os.Stderr, "unknown command:", flag.Arg(0))
			os.Exit(2)
		}
		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		code := stellar.RunToolCLI(ctx, flag.Args()[1:], root, configPath, os.Stdout, os.Stderr)
		stop()
		os.Exit(code)
	}

	services, err := app.Bootstrap(app.Options{
		Root: root, ConfigPath: configPath, IconsPath: iconsPath, DevMode: devMode, Seed: seed,
	})
	if err != nil {
		log.Fatalf("bootstrap app: %v", err)
	}

	router := gin.New()
	httpapi.RegisterHealthRoutes(router, services.Store)
	closeStellar := func() {}
	if !services.Maintenance.Active() {
		stellarServer, err := stellar.Mount(router, stellar.Dependencies{
			App: services.App, Site: services.Site, Store: services.Store, Posts: services.Post,
			OGGen: services.OG, Search: services.Search, DevMode: devMode, Root: services.App.Root,
			UploadDir: services.App.UploadDir, NotesPath: filepath.Join(services.App.Root, "content", "notes.md"),
			Comments: services.Comment, Friends: services.Friends, GH: services.GitHub, Modules: services.Modules, Icons: services.Icons, Events: services.Events,
		})
		if err != nil {
			_ = services.Close(context.Background())
			log.Fatalf("mount stellar: %v", err)
		}

		consoleHandler, err := console.Mount(router, console.MountDeps{
			API: console.APIConfig{
				Store: services.Store, Posts: services.Post, Site: services.Site, Comments: services.Comment,
				Backups: services.Backup, Modules: services.Modules, Notify: services.Notify, Icons: services.Icons,
				StellarVersion: stellar.StellarVersion,
				StellarOwner:   stellar.RepositoryOwner, StellarRepo: stellar.RepositoryName,
				RenderSitePreview:           stellarServer.RenderSitePreview,
				RenderStellarContentPreview: stellarServer.RenderContentPreview,
				StellarWidgetCatalog:        stellarServer.StellarWidgetCatalog,
				Events:                      services.Events,
				UploadDir:                   services.App.UploadDir, MaxUploadMB: services.App.MaxUploadMB, LinkWatch: services.LinkWatch,
				StorageManager: services.Storage,
				Plugins:        services.Plugins,
				RAG:            services.RAG,
				Root:           services.App.Root,
				AppVersion:     app.Version,
				UpdateRepo:     services.App.UpdateRepo,
			},
			GH: services.GitHub, DevMode: devMode, Root: services.App.Root, Fallback: stellarServer.NoRoute(nil),
		})
		if err != nil {
			stellarServer.Close()
			_ = services.Close(context.Background())
			log.Fatalf("mount console: %v", err)
		}
		consoleHandler.SetOnLinksChanged(services.Friends.Refresh)
		services.StartBackground()
		closeStellar = stellarServer.Close

	}

	server := &http.Server{
		Addr:              services.App.ListenAddr,
		Handler:           services.Maintenance.Wrap(httpapi.HeadToGet(router)),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       90 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go func() {
		fmt.Printf("Stellar blog server listening on http://localhost%s\n", services.App.ListenAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()
	<-ctx.Done()
	log.Println("shutting down gracefully (timeout 30s)...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
	closeStellar()
	if err := services.Close(shutdownCtx); err != nil {
		log.Printf("close app services failed: %v", err)
	}
	log.Println("server stopped")
}
