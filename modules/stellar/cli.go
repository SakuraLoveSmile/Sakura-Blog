package stellar

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"sync/atomic"

	"blog/internal/config"
	"blog/internal/post"
	"blog/internal/settings"
	"blog/internal/stellarcli"
	"blog/internal/stellarimages"
	"blog/internal/store"
	"blog/modules/stellar/routes"
	"blog/modules/stellar/web"
)

// RunToolCLI 是宿主命令入口，不装配 HTTP、后台任务、FTS 或启动迁移。
func RunToolCLI(ctx context.Context, args []string, root, configPath string, stdout, stderr io.Writer) int {
	env, err := ToolEnvironment()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return stellarcli.Run(ctx, args, stellarcli.Options{Root: root, ConfigPath: configPath, Environment: env, Images: PrepareToolImages}, stdout, stderr)
}

// PrepareToolImages 从真正的公开 SSR 快照发现图片，复用生产图片处理器。
func PrepareToolImages(ctx context.Context, app *config.App, st *store.Store, options stellarimages.PrepareOptions) (stellarimages.Report, error) {
	canonical, err := config.LoadStellarV2(st)
	if err != nil {
		return stellarimages.Report{}, err
	}
	site := new(atomic.Pointer[settings.Site])
	site.Store(&settings.Site{StellarV2: canonical})
	if err := config.RebuildSite(st, site); err != nil {
		return stellarimages.Report{}, err
	}
	icons, err := settings.LoadIconsEmbed()
	if err != nil {
		return stellarimages.Report{}, err
	}
	renderer := post.NewService(filepath.Join(app.Root, "content", "posts"))
	renderer.SetIconFunc(icons.Get)
	renderer.SetSettingsManager(settings.NewManagerFromPointer(site))
	assetVersioner, err := newIconAssetVersioner(false, app.Root, icons)
	if err != nil {
		return stellarimages.Report{}, err
	}
	web.SetAssetVersioner(assetVersioner)
	engine, err := web.NewEngineFromEmbed(icons)
	if err != nil {
		return stellarimages.Report{}, err
	}
	engine.SetStellarMarkdownRender(renderer.RenderStellarV2WithContext)
	pages, err := routes.RenderToolSnapshots(ctx, routes.Dependencies{App: app, Site: site, Store: st, Posts: renderer, Engine: engine, Icons: icons, Root: app.Root, UploadDir: app.UploadDir, NotesPath: filepath.Join(app.Root, "content", "notes.md")})
	if err != nil {
		return stellarimages.Report{}, err
	}
	local, closeAssets, err := ToolLocalAssets(app.UploadDir, icons)
	if err != nil {
		return stellarimages.Report{}, err
	}
	defer closeAssets()
	return (stellarimages.Processor{StorePath: filepath.Join(app.Root, "content", "_data", "caches", "images_metadata.json"), SiteURL: site.Load().URL, LocalAsset: local}).Prepare(ctx, pages, options)
}
