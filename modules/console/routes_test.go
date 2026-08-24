package console

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestConsoleStaticGzipNegotiation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	dist := filepath.Join(root, "modules", "console", "web", "dist")
	assets := filepath.Join(dist, "assets")
	if err := os.MkdirAll(assets, 0o755); err != nil {
		t.Fatalf("mkdir dist: %v", err)
	}
	plain := []byte("console asset")
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	if _, err := writer.Write(plain); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	for name, data := range map[string][]byte{
		"index.html":       []byte("<!doctype html>"),
		"assets/app.js":    plain,
		"assets/app.js.gz": compressed.Bytes(),
	} {
		if err := os.WriteFile(filepath.Join(dist, name), data, 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	r := gin.New()
	if _, err := setupConsoleRoutes(r, true, root); err != nil {
		t.Fatalf("setup routes: %v", err)
	}

	gzipRequest := httptest.NewRequest(http.MethodGet, "/console/assets/app.js", nil)
	gzipRequest.Header.Set("Accept-Encoding", "gzip, deflate")
	gzipResponse := httptest.NewRecorder()
	r.ServeHTTP(gzipResponse, gzipRequest)
	if gzipResponse.Code != http.StatusOK {
		t.Fatalf("gzip status = %d", gzipResponse.Code)
	}
	if got := gzipResponse.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", got)
	}
	if got := gzipResponse.Header().Get("Vary"); got != "Accept-Encoding" {
		t.Fatalf("Vary = %q, want Accept-Encoding", got)
	}
	if got := gzipResponse.Header().Get("Content-Type"); got != "application/javascript" {
		t.Fatalf("Content-Type = %q, want application/javascript", got)
	}
	reader, err := gzip.NewReader(bytes.NewReader(gzipResponse.Body.Bytes()))
	if err != nil {
		t.Fatalf("open gzip response: %v", err)
	}
	decoded, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read gzip response: %v", err)
	}
	if string(decoded) != string(plain) {
		t.Fatalf("decoded body = %q, want %q", decoded, plain)
	}

	plainRequest := httptest.NewRequest(http.MethodGet, "/console/assets/app.js", nil)
	plainRequest.Header.Set("Accept-Encoding", "identity")
	plainResponse := httptest.NewRecorder()
	r.ServeHTTP(plainResponse, plainRequest)
	if plainResponse.Header().Get("Content-Encoding") != "" {
		t.Fatalf("plain response unexpectedly encoded: %q", plainResponse.Header().Get("Content-Encoding"))
	}
	if plainResponse.Body.String() != string(plain) {
		t.Fatalf("plain body = %q, want %q", plainResponse.Body.String(), plain)
	}
}

func TestAcceptsGzip(t *testing.T) {
	for _, test := range []struct {
		header string
		want   bool
	}{
		{"gzip", true},
		{"br, gzip;q=0.8", true},
		{"gzip;q=0", false},
		{"*;q=0", false},
		{"identity", false},
	} {
		if got := acceptsGzip(test.header); got != test.want {
			t.Errorf("acceptsGzip(%q) = %v, want %v", test.header, got, test.want)
		}
	}
}

func TestUserCenterStaticAndFallbackData(t *testing.T) {
	root := t.TempDir()
	dist := filepath.Join(root, "modules", "console", "web", "dist-uc")
	if err := os.MkdirAll(filepath.Join(dist, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dist, "uc.html"), []byte("<uc-app>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dist, "assets", "app.js"), []byte("uc"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	index, err := setupUCRoutes(r, true, root)
	if err != nil || index == nil || string(index.plain) != "<uc-app>" {
		t.Fatalf("setup UC routes: index=%+v err=%v", index, err)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/uc/assets/app.js", nil))
	if w.Code != http.StatusOK || w.Body.String() != "uc" {
		t.Fatalf("UC asset = %d %q", w.Code, w.Body.String())
	}

	r.NoRoute(func(c *gin.Context) {
		if c.Request.URL.Path == "/uc/posts/hello" {
			writeConsoleData(c, "text/html; charset=utf-8", index.plain, index.gzip)
			return
		}
		c.Status(http.StatusNotFound)
	})
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/uc/posts/hello", nil))
	if w.Code != http.StatusOK || w.Body.String() != "<uc-app>" {
		t.Fatalf("UC fallback = %d %q", w.Code, w.Body.String())
	}
}
