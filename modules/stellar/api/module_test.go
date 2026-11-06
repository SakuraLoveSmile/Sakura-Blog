package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"

	"blog/internal/comments"
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
	site := &atomic.Pointer[tmpl.Site]{}
	site.Store(&tmpl.Site{Comments: tmpl.CommentsConfig{Enabled: true, GravatarMirror: "https://avatar.test"}})
	r := gin.New()
	if err := Mount(r, Dependencies{Store: st, Site: site, Comments: comments.New(st, site)}); err != nil {
		t.Fatalf("mount: %v", err)
	}
	got := map[string]bool{}
	for _, route := range r.Routes() {
		got[route.Method+" "+route.Path] = true
	}
	want := []string{
		"GET /api/stellar/remote-md",
		"POST /api/auth/register",
		"POST /api/auth/login",
		"GET /api/auth/me",
		"POST /api/auth/logout",
		"POST /api/auth/upgrade",
		"GET /api/comments",
		"POST /api/comments",
		"POST /api/comments/:id/like",
		"POST /api/comments/:id/boost",
		"DELETE /api/comments/:id/boost",
		"POST /api/comments/upload",
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

func TestPublicFriendReadCORSStaysLimitedToAnonymousGET(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st, err := store.New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	site := &atomic.Pointer[tmpl.Site]{}
	site.Store(&tmpl.Site{})
	router := gin.New()
	if err := Mount(router, Dependencies{Store: st, Site: site, Friends: friends.New(site)}); err != nil {
		t.Fatal(err)
	}
	for _, origin := range []string{"null", "https://reader.example"} {
		for _, path := range []string{"/api/friends", "/api/friends-aggregated"} {
			request := httptest.NewRequest(http.MethodGet, path, nil)
			request.Header.Set("Origin", origin)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != http.StatusOK || !json.Valid(response.Body.Bytes()) {
				t.Fatalf("公开友链 %s => %d: %s", path, response.Code, response.Body.String())
			}
			if response.Header().Get("Access-Control-Allow-Origin") != "*" || response.Header().Get("Access-Control-Allow-Credentials") != "" {
				t.Errorf("公开友链须允许无凭据读取: %s: %v", path, response.Header())
			}
		}
	}
	for _, target := range []struct{ method, path string }{
		{http.MethodGet, "/api/auth/me"},
		{http.MethodPost, "/api/rating"},
		{http.MethodPost, "/api/friends"},
		{http.MethodOptions, "/api/friends-aggregated"},
	} {
		request := httptest.NewRequest(target.method, target.path, nil)
		request.Header.Set("Origin", "null")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Header().Get("Access-Control-Allow-Origin") != "" || response.Header().Get("Access-Control-Allow-Credentials") != "" {
			t.Errorf("其他接口不应获得友链 CORS: %s %s: %v", target.method, target.path, response.Header())
		}
	}
}
