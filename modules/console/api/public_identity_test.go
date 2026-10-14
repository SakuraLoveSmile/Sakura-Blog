package api

import (
	"time"

	"github.com/gin-gonic/gin"

	"blog/internal/identity"
	tmpl "blog/internal/settings"
	"blog/internal/store"
)

const testReaderCookieTTL = 30 * 24 * time.Hour

func mountPublicIdentityForTest(r *gin.Engine, st *store.Store, site *tmpl.Site) {
	reader := identity.NewPublicHandler(st, sitePtr(site))
	r.Use(reader.Middleware())
	routes := reader.PublicRoutes()
	auth := r.Group("/api/auth")
	auth.POST("/register", routes.Register)
	auth.POST("/login", routes.Login)
	auth.GET("/me", routes.Me)
	auth.POST("/logout", routes.Logout)
	auth.POST("/upgrade", routes.Upgrade)
}

func signReaderCookie(u *store.User) string {
	return identity.SignCookie(identity.ReaderAudience, time.Now().Add(testReaderCookieTTL).Unix(), u.ID, u.AuthSecret)
}
