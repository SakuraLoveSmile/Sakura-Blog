package api

import (
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"

	"blog/internal/friends"
	tmpl "blog/internal/settings"
	"blog/internal/store"
)

func TestStellarPublicRouteOwnershipSnapshot(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st, err := store.New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()
	r := gin.New()
	if err := Mount(r, Dependencies{Store: st}); err != nil {
		t.Fatalf("mount: %v", err)
	}
	got := map[string]bool{}
	for _, route := range r.Routes() {
		got[route.Method+" "+route.Path] = true
	}
	want := []string{
		"POST /api/auth/register",
		"POST /api/auth/login",
		"GET /api/auth/me",
		"POST /api/auth/logout",
		"GET /api/public/v1/posts",
		"GET /api/public/v1/posts/:slug",
		"GET /api/public/v1/pages",
		"GET /api/public/v1/categories",
		"GET /api/public/v1/tags",
		"GET /api/public/v1/comments",
		"GET /api/rating",
		"POST /api/rating",
		"GET /api/vote/info",
		"POST /api/vote/update",
	}
	if len(got) != len(want) {
		t.Fatalf("Stellar public route count = %d, want %d: %+v", len(got), len(want), got)
	}
	for _, route := range want {
		if !got[route] {
			t.Errorf("Stellar route missing from ownership snapshot: %s", route)
		}
	}
}

func TestStellarRegistersOptionalFriendRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st, err := store.New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()
	site := &atomic.Pointer[tmpl.Site]{}
	site.Store(&tmpl.Site{})
	r := gin.New()
	if err := Mount(r, Dependencies{Store: st, Site: site, Friends: friends.New(site)}); err != nil {
		t.Fatalf("mount: %v", err)
	}
	got := map[string]bool{}
	for _, route := range r.Routes() {
		got[route.Method+" "+route.Path] = true
	}
	for _, route := range []string{"GET /api/friends-aggregated", "GET /api/friends", "GET /api/rating", "POST /api/rating", "GET /api/vote/info", "POST /api/vote/update"} {
		if !got[route] {
			t.Errorf("Stellar optional route missing: %s", route)
		}
	}
}
