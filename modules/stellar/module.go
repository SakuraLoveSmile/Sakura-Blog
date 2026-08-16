// Package stellar mounts the public Stellar server-rendered experience.
package stellar

import (
	"github.com/gin-gonic/gin"

	ghpkg "blog/internal/github"
	"blog/internal/post"
	"blog/modules/stellar/routes"
)

// Dependencies is the public module boundary for Stellar's shared services.
type Dependencies = routes.Dependencies

// Server exposes lifecycle and callback hooks needed by the other module mount.
type Server struct {
	*routes.Server
}

// Mount creates the Stellar SSR server and registers its routes on router.
func Mount(router *gin.Engine, deps Dependencies) (*Server, error) {
	server, err := routes.New(deps)
	if err != nil {
		return nil, err
	}
	if err := server.Mount(router); err != nil {
		return nil, err
	}
	return &Server{Server: server}, nil
}

// GitHubAdapter keeps the post renderer independent from the concrete proxy client.
func GitHubAdapter(client *ghpkg.Client) post.GHClient {
	return routes.NewGHAdapter(client)
}
