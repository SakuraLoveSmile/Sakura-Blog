// Package stellar mounts the public Stellar server-rendered experience.
package stellar

import (
	"fmt"
	"html/template"
	"log"
	"path/filepath"

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

// Server exposes lifecycle and callback hooks needed by the other module mount.
type Server struct {
	*routes.Server
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
	web.SetAssetVersioner(NewAssetVersioner(deps.DevMode, deps.Root))
	if deps.Posts != nil {
		web.SetMarkdownRender(func(source string) template.HTML {
			html, renderErr := deps.Posts.Render(source)
			if renderErr != nil {
				log.Printf("render footer markdown: %v", renderErr)
				return ""
			}
			return template.HTML(html)
		})
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
	return &Server{Server: server}, nil
}

// GitHubAdapter keeps the post renderer independent from the concrete proxy client.
func GitHubAdapter(client *ghpkg.Client) post.GHClient {
	return routes.NewGHAdapter(client)
}
