package ghcr

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// newRegistry 启动一个模拟 GHCR 的测试服务：/token 与 /v2/<image>/tags/list。
func newRegistry(t *testing.T, tagsHandler http.HandlerFunc) (*httptest.Server, *int32) {
	t.Helper()
	var tokenHits int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/token":
			atomic.AddInt32(&tokenHits, 1)
			if r.URL.Query().Get("scope") == "" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"token":"test-token"}`)
		case r.URL.Path == "/v2/owner/app/tags/list":
			if r.Header.Get("Authorization") != "Bearer test-token" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			tagsHandler(w, r)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	return server, &tokenHits
}

func TestListTags(t *testing.T) {
	server, _ := newRegistry(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"name":"owner/app","tags":["2.0.4","2.0","latest","2.0.3"]}`)
	})
	client := New(Config{RegistryBase: server.URL, CacheDir: t.TempDir(), HTTPClient: server.Client()})

	tags, err := client.ListTags("owner/app", false)
	if err != nil {
		t.Fatalf("ListTags: %v", err)
	}
	want := []string{"2.0.4", "2.0", "latest", "2.0.3"}
	if len(tags) != len(want) {
		t.Fatalf("tags = %v, want %v", tags, want)
	}
	for i := range want {
		if tags[i] != want[i] {
			t.Fatalf("tags[%d] = %q, want %q", i, tags[i], want[i])
		}
	}
}

func TestListTagsCacheAndRefresh(t *testing.T) {
	var hits int32
	server, tokenHits := newRegistry(t, func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"tags":["2.0.3"]}`)
	})
	cacheDir := t.TempDir()
	client := New(Config{RegistryBase: server.URL, CacheDir: cacheDir, HTTPClient: server.Client()})

	if _, err := client.ListTags("owner/app", false); err != nil {
		t.Fatalf("first ListTags: %v", err)
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Fatalf("first call should hit tags endpoint once, got %d", got)
	}
	// 第二次应命中缓存，不再回源。
	if _, err := client.ListTags("owner/app", false); err != nil {
		t.Fatalf("cached ListTags: %v", err)
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Fatalf("cached call should not hit upstream, got %d", got)
	}
	// refresh 绕过缓存。
	if _, err := client.ListTags("owner/app", true); err != nil {
		t.Fatalf("refresh ListTags: %v", err)
	}
	if got := atomic.LoadInt32(&hits); got != 2 {
		t.Fatalf("refresh should hit upstream again, got %d", got)
	}
	if got := atomic.LoadInt32(tokenHits); got != 2 {
		t.Fatalf("token endpoint hits = %d, want 2", got)
	}

	// 新客户端共享同一磁盘缓存，仍不回源。
	another := New(Config{RegistryBase: server.URL, CacheDir: cacheDir, HTTPClient: server.Client()})
	if _, err := another.ListTags("owner/app", false); err != nil {
		t.Fatalf("disk-cached ListTags: %v", err)
	}
	if got := atomic.LoadInt32(&hits); got != 2 {
		t.Fatalf("disk cache should avoid upstream, got %d", got)
	}
}

func TestListTagsPagination(t *testing.T) {
	var page int32
	server, _ := newRegistry(t, func(w http.ResponseWriter, r *http.Request) {
		next := atomic.AddInt32(&page, 1)
		w.Header().Set("Content-Type", "application/json")
		if next == 1 {
			w.Header().Set("Link", fmt.Sprintf(`<%s/v2/owner/app/tags/list?n=100&last=2.0.2>; rel="next"`, "http://"+r.Host))
			fmt.Fprint(w, `{"tags":["2.0.4","2.0.2"]}`)
			return
		}
		fmt.Fprint(w, `{"tags":["2.0.1"]}`)
	})
	client := New(Config{RegistryBase: server.URL, CacheDir: t.TempDir(), HTTPClient: server.Client()})
	tags, err := client.ListTags("owner/app", false)
	if err != nil {
		t.Fatalf("ListTags: %v", err)
	}
	if len(tags) != 3 || tags[2] != "2.0.1" {
		t.Fatalf("paged tags = %v, want 3 tags ending with 2.0.1", tags)
	}
}

func TestListTagsRejectsForeignPagination(t *testing.T) {
	server, _ := newRegistry(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Link", `<http://evil.example/v2/owner/app/tags/list>; rel="next"`)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"tags":["2.0.4"]}`)
	})
	client := New(Config{RegistryBase: server.URL, CacheDir: t.TempDir(), HTTPClient: server.Client()})
	tags, err := client.ListTags("owner/app", false)
	if err != nil {
		t.Fatalf("ListTags: %v", err)
	}
	if len(tags) != 1 || tags[0] != "2.0.4" {
		t.Fatalf("foreign pagination must be ignored, got %v", tags)
	}
}

func TestListTagsErrors(t *testing.T) {
	t.Run("token 401", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		t.Cleanup(server.Close)
		client := New(Config{RegistryBase: server.URL, CacheDir: t.TempDir(), HTTPClient: server.Client()})
		if _, err := client.ListTags("owner/app", false); err == nil {
			t.Fatal("expected error for unauthorized token endpoint")
		}
	})
	t.Run("tags 500", func(t *testing.T) {
		server, _ := newRegistry(t, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})
		client := New(Config{RegistryBase: server.URL, CacheDir: t.TempDir(), HTTPClient: server.Client()})
		if _, err := client.ListTags("owner/app", false); err == nil {
			t.Fatal("expected error for failing tags endpoint")
		}
	})
	t.Run("invalid image", func(t *testing.T) {
		client := New(Config{CacheDir: t.TempDir()})
		if _, err := client.ListTags("Owner/App;rm", false); err == nil {
			t.Fatal("expected error for invalid image name")
		}
	})
}

func TestListTagsExpiredCacheRefetches(t *testing.T) {
	var hits int32
	server, _ := newRegistry(t, func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"tags":["2.0.4"]}`)
	})
	client := New(Config{RegistryBase: server.URL, CacheDir: t.TempDir(), HTTPClient: server.Client(), TTL: time.Millisecond})
	if _, err := client.ListTags("owner/app", false); err != nil {
		t.Fatalf("first ListTags: %v", err)
	}
	time.Sleep(5 * time.Millisecond)
	if _, err := client.ListTags("owner/app", false); err != nil {
		t.Fatalf("second ListTags: %v", err)
	}
	if got := atomic.LoadInt32(&hits); got != 2 {
		t.Fatalf("expired cache should refetch, got %d hits", got)
	}
}
