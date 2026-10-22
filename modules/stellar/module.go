// Package stellar mounts the public Stellar server-rendered experience.
package stellar

import (
	"fmt"
	"path/filepath"
	"sync"

	"github.com/gin-gonic/gin"

	ghpkg "blog/internal/github"
	"blog/internal/identity"
	"blog/internal/module"
	"blog/internal/post"
	stellarapi "blog/modules/stellar/api"
	"blog/modules/stellar/routes"
	"blog/modules/stellar/web"
)

// Dependencies is the public module boundary for Stellar's shared services.
type Dependencies = routes.Dependencies

// RepositoryOwner and RepositoryName are the Stellar module's self-reported
// upstream identity. Console must consume these through its mount contract.
const (
	RepositoryOwner = "xaoxuu"
	RepositoryName  = "hexo-theme-stellar"
)

// Server exposes lifecycle and callback hooks needed by the other module mount.
type Server struct {
	*routes.Server
	images    *imageLifecycle
	closeOnce sync.Once
}

// Close 等待图片 worker 完全退出后刷新路由派生状态；宿主随后关闭 Store。
func (s *Server) Close() {
	s.closeOnce.Do(func() {
		if s.images != nil {
			s.images.close()
		}
		s.Server.Close()
	})
}

// Mount creates the Stellar SSR server and registers its routes on router.
func Mount(router *gin.Engine, deps Dependencies) (*Server, error) {
	var engine *web.Engine
	var err error
	if deps.DevMode {
		engine, err = web.NewEngine(filepath.Join(deps.Root, "modules", "stellar", "web", "templates"), deps.Icons)
	} else {
		engine, err = web.NewEngineFromEmbed(deps.Icons)
	}
	if err != nil {
		return nil, fmt.Errorf("load stellar templates: %w", err)
	}
	assetVersioner, err := newIconAssetVersioner(deps.DevMode, deps.Root, deps.Icons)
	if err != nil {
		return nil, err
	}
	web.SetAssetVersioner(assetVersioner)
	version := deps.AppVersion
	if version == "" {
		version = "dev"
	}
	engine.SetStellarHost(web.StellarHost{Name: "Blog", Version: version, Homepage: "https://github.com/SakuraLoveSmile/Sakura-Blog"})
	if deps.Posts != nil {
		engine.SetStellarMarkdownRender(deps.Posts.RenderStellarV2WithContext)
	}
	deps.Engine = engine
	publicIdentity := identity.NewPublicHandler(deps.Store, deps.Site)
	router.Use(publicIdentity.Middleware())
	server, err := routes.New(deps)
	if err != nil {
		return nil, err
	}
	if err := server.Mount(router); err != nil {
		return nil, err
	}
	if err := stellarapi.Mount(router, stellarapi.Dependencies{
		Store: deps.Store, Site: deps.Site, Comments: deps.Comments, Friends: deps.Friends, GH: deps.GH, Identity: publicIdentity, Events: deps.Events,
	}); err != nil {
		return nil, err
	}
	if deps.Modules != nil {
		moduleDeps := module.Deps{Store: deps.Store, Posts: deps.Posts, Site: deps.Site, Modules: deps.Modules}
		for _, mod := range module.All() {
			mod.RegisterPublic(router, moduleDeps)
		}
	}
	result := &Server{Server: server}
	result.images = newImageLifecycle(deps, server.RenderSnapshots)
	result.images.start()
	return result, nil
}

// GitHubAdapter keeps the post renderer independent from the concrete proxy client.
func GitHubAdapter(client *ghpkg.Client) post.GHClient {
	return routes.NewGHAdapter(client)
}
