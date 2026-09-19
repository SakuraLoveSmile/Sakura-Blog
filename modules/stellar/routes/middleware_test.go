package routes

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"blog/internal/config"
	"blog/internal/httpapi"
	tmpl "blog/modules/stellar/viewmodel"
)

func TestForwardedProtoRequiresTrustedProxy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ws := newWebServer(webDeps{Site: sitePtr(&tmpl.Site{})})
	for _, tc := range []struct {
		name, remote, want string
	}{
		{"untrusted", "10.0.0.2:1234", "http://proxy.example"},
		{"trusted", "10.0.0.1:1234", "https://proxy.example"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := gin.New()
			ws.installMiddleware(r, &config.App{TrustedProxy: "10.0.0.1/32"}, newPageViewBuffer(nil))
			r.GET("/", func(c *gin.Context) { c.String(http.StatusOK, ws.baseURL(c)) })
			req := httptest.NewRequest(http.MethodGet, "http://proxy.example/", nil)
			req.RemoteAddr = tc.remote
			req.Header.Set("X-Forwarded-Proto", "https")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if got := strings.TrimSpace(w.Body.String()); got != tc.want {
				t.Fatalf("base URL = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestHeadRoutesMatchGetBeforeGin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	for _, path := range []string{"/", "/rss.xml", "/atom.xml", "/sitemap.xml", "/robots.txt", "/console/"} {
		path := path
		r.GET(path, func(c *gin.Context) { c.Header("ETag", `"stable"`); c.String(http.StatusOK, path) })
	}
	r.NoRoute(func(c *gin.Context) { c.String(http.StatusNotFound, "not found") })
	server := httptest.NewServer(httpapi.HeadToGet(r))
	defer server.Close()
	client := server.Client()

	for _, path := range []string{"/", "/rss.xml", "/atom.xml", "/sitemap.xml", "/robots.txt", "/console/", "/unknown"} {
		get, err := client.Get(server.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		_, _ = io.Copy(io.Discard, get.Body)
		get.Body.Close()
		headReq, err := http.NewRequest(http.MethodHead, server.URL+path, nil)
		if err != nil {
			t.Fatalf("HEAD %s request: %v", path, err)
		}
		head, err := client.Do(headReq)
		if err != nil {
			t.Fatalf("HEAD %s: %v", path, err)
		}
		headBody, _ := io.ReadAll(head.Body)
		head.Body.Close()
		if head.StatusCode != get.StatusCode {
			t.Errorf("HEAD %s status = %d, GET = %d", path, head.StatusCode, get.StatusCode)
		}
		for _, key := range []string{"Content-Type", "Content-Length", "ETag"} {
			if got, want := head.Header.Get(key), get.Header.Get(key); got != want {
				t.Errorf("HEAD %s header %s = %q, GET = %q", path, key, got, want)
			}
		}
		if len(headBody) != 0 {
			t.Errorf("HEAD %s returned body %q", path, headBody)
		}
	}
}

func TestSecurityHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(securityHeaders())
	r.GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/console/x", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))
	if w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("nosniff header missing")
	}
	if w.Header().Get("X-Frame-Options") != "SAMEORIGIN" {
		t.Error("SAMEORIGIN header missing")
	}
	if w.Header().Get("Cache-Control") != "" {
		t.Error("non-console path should not get no-cache")
	}

	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, httptest.NewRequest(http.MethodGet, "/console/x", nil))
	if w2.Header().Get("Cache-Control") != "" {
		t.Errorf("security headers should not decide console cache policy, got %q", w2.Header().Get("Cache-Control"))
	}
}

func TestAssetsCacheControl(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(assetsCacheControl())
	r.GET("/assets/css/main.css", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/sw.js", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/assets/css/main.css", nil))
	if w.Header().Get("Cache-Control") != "public, max-age=31536000, immutable" {
		t.Errorf("assets should get immutable cache, got %q", w.Header().Get("Cache-Control"))
	}

	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, httptest.NewRequest(http.MethodGet, "/sw.js", nil))
	if w2.Header().Get("Cache-Control") != "" {
		t.Error("non-assets path should not get immutable cache")
	}
}

// TestRecoveryAPIPanicProblem API 路由 panic → 统一 JSON Problem（500），而非渲染 500 页。
func TestRecoveryAPIPanicProblem(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := newWebServer(webDeps{})
	r := gin.New()
	s.installMiddleware(r, &config.App{}, newPageViewBuffer(nil))
	r.GET("/api/v1/panic", func(c *gin.Context) {
		panic("boom")
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/panic", nil))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not JSON Problem: %v (body=%s)", err, w.Body.String())
	}
	if body["type"] != "internal_error" || body["error"] != "服务器内部错误，请稍后重试" || body["detail"] != "服务器内部错误，请稍后重试" {
		t.Fatalf("problem body wrong: %v", body)
	}
	if strings.Contains(w.Body.String(), "boom") {
		t.Fatalf("panic detail must not be exposed: %s", w.Body.String())
	}
	if body["instance"] != "/api/v1/panic" {
		t.Fatalf("instance should be request path, got %v", body["instance"])
	}
}
