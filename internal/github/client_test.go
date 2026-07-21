package github

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// newTestClient 构造指向 TLS 测试上游的 Client：cardHost 与 apiBase 均注入测试服务器，
// http.Client 信任测试证书，磁盘缓存落到临时目录。
func newTestClient(t *testing.T, upstream http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewTLSServer(upstream)
	t.Cleanup(srv.Close)
	c := New(Config{CacheDir: t.TempDir()})
	c.http = srv.Client()
	c.cardHost = strings.TrimPrefix(srv.URL, "https://")
	c.apiBase = srv.URL
	return c
}

// TestCardSVGMemoryCache 同一实例二次请求命中内存缓存，上游仅被调用一次。
func TestCardSVGMemoryCache(t *testing.T) {
	var hits int32
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Write([]byte("<svg>ok</svg>"))
	})
	b1, err := c.GetCardSVG("username=x")
	if err != nil || string(b1) != "<svg>ok</svg>" {
		t.Fatalf("first call: %v %q", err, b1)
	}
	b2, err := c.GetCardSVG("username=x")
	if err != nil || string(b2) != "<svg>ok</svg>" {
		t.Fatalf("second call: %v %q", err, b2)
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Fatalf("expected 1 upstream hit, got %d", got)
	}
}

// TestCardSVGDiskPersist 新实例内存为空，但磁盘缓存仍命中，避免二次回源。
func TestCardSVGDiskPersist(t *testing.T) {
	var hits int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Write([]byte("<svg>disk</svg>"))
	}))
	defer srv.Close()
	dir := t.TempDir()
	host := strings.TrimPrefix(srv.URL, "https://")

	c1 := New(Config{CacheDir: dir})
	c1.http = srv.Client()
	c1.cardHost = host
	if _, err := c1.GetCardSVG("username=y"); err != nil {
		t.Fatal(err)
	}

	c2 := New(Config{CacheDir: dir})
	c2.http = srv.Client()
	c2.cardHost = host
	b, err := c2.GetCardSVG("username=y")
	if err != nil || string(b) != "<svg>disk</svg>" {
		t.Fatalf("disk hit failed: %v %q", err, b)
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Fatalf("expected disk cache to avoid 2nd upstream hit, got %d", got)
	}
}

// TestStaleWhileError 磁盘缓存过期且上游失败时，返回过期缓存而非报错。
func TestStaleWhileError(t *testing.T) {
	var fail int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.LoadInt32(&fail) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Write([]byte("<svg>fresh</svg>"))
	}))
	defer srv.Close()
	dir := t.TempDir()
	host := strings.TrimPrefix(srv.URL, "https://")

	c := New(Config{CacheDir: dir})
	c.http = srv.Client()
	c.cardHost = host
	if _, err := c.GetCardSVG("username=z"); err != nil {
		t.Fatal(err)
	}

	// 磁盘缓存 modtime 回拨 25h，超过 24h TTL 触发回源。
	url := "https://" + host + "/api?username=z"
	diskPath := filepath.Join(dir, cacheKey(url)+".json")
	old := time.Now().Add(-25 * time.Hour)
	if err := os.Chtimes(diskPath, old, old); err != nil {
		t.Fatal(err)
	}

	atomic.StoreInt32(&fail, 1)
	c2 := New(Config{CacheDir: dir}) // 新实例避免命中内存
	c2.http = srv.Client()
	c2.cardHost = host
	b, err := c2.GetCardSVG("username=z")
	if err != nil {
		t.Fatalf("stale fallback should not error: %v", err)
	}
	if string(b) != "<svg>fresh</svg>" {
		t.Fatalf("expected stale data, got %q", b)
	}
}

// TestNoCacheUpstreamError 无缓存且上游 404 时返回错误。
func TestNoCacheUpstreamError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	if _, err := c.GetCardSVG("username=nobody"); err == nil {
		t.Fatal("expected error when upstream 404 and no cache")
	}
}

// TestGetRepoParseAndCache 仓库元数据字段裁剪正确，且二次请求命中缓存。
func TestGetRepoParseAndCache(t *testing.T) {
	var hits int32
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"name":"stellar","full_name":"xaoxuu/stellar","description":"theme","stargazers_count":120,"forks_count":30,"language":"JavaScript","license":{"name":"MIT"},"owner":{"login":"xaoxuu","avatar_url":"http://a"},"html_url":"http://h"}`))
	})
	info, err := c.GetRepo("xaoxuu", "stellar")
	if err != nil {
		t.Fatal(err)
	}
	if info.Name != "stellar" || info.Stars != 120 || info.Forks != 30 ||
		info.Language != "JavaScript" || info.License != "MIT" || info.OwnerLogin != "xaoxuu" {
		t.Fatalf("parse mismatch: %+v", info)
	}
	if _, err := c.GetRepo("xaoxuu", "stellar"); err != nil {
		t.Fatal(err)
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Fatalf("expected 1 upstream hit, got %d", got)
	}
}

// TestGetUserParse 用户信息字段裁剪正确。
func TestGetUserParse(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"login":"xaoxuu","name":"X","bio":"dev","avatar_url":"http://a","followers":5,"following":2,"public_repos":10,"html_url":"http://h"}`))
	})
	u, err := c.GetUser("xaoxuu")
	if err != nil {
		t.Fatal(err)
	}
	if u.Login != "xaoxuu" || u.Followers != 5 || u.Repos != 10 || u.Bio != "dev" {
		t.Fatalf("parse mismatch: %+v", u)
	}
}
