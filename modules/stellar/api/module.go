// Package api owns the public HTTP endpoints used by Stellar's browser assets.
package api

import (
	"fmt"
	"sync/atomic"

	"github.com/gin-gonic/gin"

	"blog/internal/content"
	"blog/internal/friends"
	ghpkg "blog/internal/github"
	"blog/internal/identity"
	tmpl "blog/internal/settings"
	"blog/internal/store"
)

// Dependencies are the public services exposed to the Stellar frontend.
type Dependencies struct {
	Store    *store.Store
	Site     *atomic.Pointer[tmpl.Site]
	Friends  *friends.Aggregator
	GH       *ghpkg.Client
	Identity *identity.PublicHandler
	Events   content.Events
}

// Mount registers public feed aggregation, GitHub proxy, rating, and voting
// endpoints without depending on the Console module.
func Mount(r *gin.Engine, d Dependencies) error {
	if d.Store == nil {
		return fmt.Errorf("stellar api: store is required")
	}
	publicIdentity := d.Identity
	if publicIdentity == nil {
		publicIdentity = identity.NewPublicHandler(d.Store, d.Site)
		r.Use(publicIdentity.Middleware())
	}
	publicIdentity.Register(r)
	if err := NewPublicContent(d.Store, d.Site).Register(r); err != nil {
		return err
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
