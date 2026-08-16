// Package api owns the public HTTP endpoints used by Stellar's browser assets.
package api

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"blog/internal/friends"
	ghpkg "blog/internal/github"
	"blog/internal/store"
)

// Dependencies are the public services exposed to the Stellar frontend.
type Dependencies struct {
	Store   *store.Store
	Friends *friends.Aggregator
	GH      *ghpkg.Client
}

// Mount registers public feed aggregation, GitHub proxy, rating, and voting
// endpoints without depending on the Console module.
func Mount(r *gin.Engine, d Dependencies) error {
	if d.Store == nil {
		return fmt.Errorf("stellar api: store is required")
	}
	if d.Friends != nil {
		d.Friends.Register(r)
	}
	if d.GH != nil {
		ghpkg.NewHandler(d.GH).Register(r)
	}
	store.NewRatingHandler(d.Store).Register(r)
	store.NewVoteHandler(d.Store).Register(r)
	return nil
}
