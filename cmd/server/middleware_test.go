package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHeadToGet(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// 只验证中间件契约：HEAD 被改写为 GET（net/http 会自动抑制 HEAD 响应体）。
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodHead, "/ping", nil)
	headToGet()(c)
	if c.Request.Method != http.MethodGet {
		t.Errorf("HEAD should be rewritten to GET, got %s", c.Request.Method)
	}
	// GET 不受影响
	c2, _ := gin.CreateTestContext(httptest.NewRecorder())
	c2.Request = httptest.NewRequest(http.MethodPost, "/ping", nil)
	headToGet()(c2)
	if c2.Request.Method != http.MethodPost {
		t.Errorf("non-HEAD method should be untouched, got %s", c2.Request.Method)
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
	if w2.Header().Get("Cache-Control") != "no-cache, no-store, must-revalidate" {
		t.Errorf("console path should get no-cache, got %q", w2.Header().Get("Cache-Control"))
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
