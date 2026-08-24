// Package api owns the public HTTP endpoints used by Stellar's browser assets.
package api

import (
	"fmt"
	"sync/atomic"

	"github.com/gin-gonic/gin"

	"blog/internal/comments"
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
	Comments *comments.Handler
	Friends  *friends.Aggregator
	GH       *ghpkg.Client
	Identity *identity.PublicHandler
	Events   content.Events
}

// Mount registers all public HTTP APIs owned by Stellar without depending on
// the Console module. Shared packages provide callbacks and business rules;
// this package owns the public URL map.
func Mount(r *gin.Engine, d Dependencies) error {
	if d.Store == nil {
		return fmt.Errorf("stellar api: store is required")
	}
	publicIdentity := d.Identity
	if publicIdentity == nil {
		publicIdentity = identity.NewPublicHandler(d.Store, d.Site)
		r.Use(publicIdentity.Middleware())
	}
	identityRoutes := publicIdentity.PublicRoutes()
	auth := r.Group("/api/auth")
	auth.GET("/register/challenge", identityRoutes.RegisterChallenge)
	auth.POST("/register", identityRoutes.Register)
	auth.POST("/login", identityRoutes.Login)
	auth.GET("/me", identityRoutes.Me)
	auth.POST("/logout", identityRoutes.Logout)
	auth.POST("/upgrade", identityRoutes.Upgrade)

	if d.Comments != nil && d.Site != nil && d.Site.Load() != nil && d.Site.Load().Comments.Enabled {
		commentRoutes := d.Comments.PublicRoutes()
		comments := r.Group("/api/comments")
		comments.GET("", commentRoutes.List)
		comments.POST("", commentRoutes.Create)
		comments.POST("/:id/like", commentRoutes.Like)
		comments.POST("/:id/boost", commentRoutes.Boost)
		comments.DELETE("/:id/boost", commentRoutes.Unboost)
		comments.POST("/upload", commentRoutes.UploadImage)
	}

	if err := NewPublicContent(d.Store, d.Site).Register(r); err != nil {
		return err
	}
	if d.Friends != nil {
		friendRoutes := d.Friends.PublicRoutes()
		r.GET("/api/friends-aggregated", friendRoutes.Aggregated)
		r.GET("/api/friends", friendRoutes.Aggregated)
	}
	if d.GH != nil {
		ghRoutes := ghpkg.NewHandler(d.GH).PublicRoutes()
		r.GET("/api/gh/repo", ghRoutes.RepoInfo)
		r.GET("/api/gh/widget/repo", ghRoutes.WidgetRepo)
		r.GET("/api/gh/widget/release", ghRoutes.WidgetRelease)
		r.GET("/api/gh/widget/user", ghRoutes.WidgetUser)
		r.GET("/api/gh/widget/tags", ghRoutes.WidgetTags)
		r.GET("/api/gh/widget/issues", ghRoutes.WidgetIssues)
	}
	ratingRoutes := store.NewRatingHandler(d.Store).PublicRoutes()
	r.GET("/api/rating", ratingRoutes.Get)
	r.POST("/api/rating", ratingRoutes.Vote)
	voteRoutes := store.NewVoteHandler(d.Store).PublicRoutes()
	r.GET("/api/vote/info", voteRoutes.Info)
	r.POST("/api/vote/update", voteRoutes.Update)
	return nil
}
