package github

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// newTestClient 构造指向 TLS 测试上游的 Client：apiBase 注入测试服务器，
// http.Client 信任测试证书，磁盘缓存落到临时目录。
func newTestClient(t *testing.T, upstream http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewTLSServer(upstream)
	t.Cleanup(srv.Close)
	c := New(Config{CacheDir: t.TempDir()})
	c.http = srv.Client()
	c.apiBase = srv.URL
	return c
}

// TestMemoryCache 同一实例二次请求命中内存缓存，上游仅被调用一次。
func TestMemoryCache(t *testing.T) {
	var hits int32
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Write([]byte("payload"))
	})
	url := c.apiBase + "/data"
	b1, err := c.get(url, time.Hour)
	if err != nil || string(b1) != "payload" {
		t.Fatalf("first call: %v %q", err, b1)
	}
	b2, err := c.get(url, time.Hour)
	if err != nil || string(b2) != "payload" {
		t.Fatalf("second call: %v %q", err, b2)
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Fatalf("expected 1 upstream hit, got %d", got)
	}
}

// TestDiskPersist 新实例内存为空，但磁盘缓存仍命中，避免二次回源。
func TestDiskPersist(t *testing.T) {
	var hits int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Write([]byte("disk"))
	}))
	defer srv.Close()
	dir := t.TempDir()
	url := srv.URL + "/data"

	c1 := New(Config{CacheDir: dir})
	c1.http = srv.Client()
	if _, err := c1.get(url, time.Hour); err != nil {
		t.Fatal(err)
	}

	c2 := New(Config{CacheDir: dir})
	c2.http = srv.Client()
	b, err := c2.get(url, time.Hour)
	if err != nil || string(b) != "disk" {
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
		w.Write([]byte("fresh"))
	}))
	defer srv.Close()
	dir := t.TempDir()
	url := srv.URL + "/data"

	c := New(Config{CacheDir: dir})
	c.http = srv.Client()
	if _, err := c.get(url, 24*time.Hour); err != nil {
		t.Fatal(err)
	}

	// 磁盘缓存 modtime 回拨 25h，超过 24h TTL 触发回源。
	diskPath := filepath.Join(dir, cacheKey(url)+".json")
	old := time.Now().Add(-25 * time.Hour)
	if err := os.Chtimes(diskPath, old, old); err != nil {
		t.Fatal(err)
	}

	atomic.StoreInt32(&fail, 1)
	c2 := New(Config{CacheDir: dir}) // 新实例避免命中内存
	c2.http = srv.Client()
	b, err := c2.get(url, 24*time.Hour)
	if err != nil {
		t.Fatalf("stale fallback should not error: %v", err)
	}
	if string(b) != "fresh" {
		t.Fatalf("expected stale data, got %q", b)
	}
}

// TestNoCacheUpstreamError 无缓存且上游 404 时返回错误。
func TestNoCacheUpstreamError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	if _, err := c.get(c.apiBase+"/missing", time.Hour); err == nil {
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

// TestGetIssues 裁剪解析：上游返回 GitHub 形状 issues 数组，验证字段裁剪与错误传播。
func TestGetIssues(t *testing.T) {
	payload, err := json.Marshal([]map[string]any{
		map[string]any{
			"number": 10, "title": "W1ndysの小屋", "state": "open",
			"body":   "### 友链信息\n```json\n{\"title\":\"x\",\"url\":\"https://x.top\"}\n```",
			"labels": []map[string]string{{"name": "失联"}},
		},
		map[string]any{"number": 3, "title": "no-block", "state": "open", "body": "plain body", "labels": []string{}},
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/sakurasep/Friends/issues" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("state"); got != "open" {
			t.Errorf("expected state=open, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(payload)
	}))
	defer srv.Close()

	c := New(Config{CacheDir: t.TempDir()})
	c.apiBase = srv.URL
	issues, err := c.GetIssues("sakurasep", "Friends", "state=open&per_page=100")
	if err != nil {
		t.Fatalf("GetIssues: %v", err)
	}
	if len(issues) != 2 {
		t.Fatalf("expected 2 issues, got %d", len(issues))
	}
	if issues[0].Number != 10 || issues[0].Title != "W1ndysの小屋" || issues[0].State != "open" {
		t.Errorf("issue[0] mismatch: %+v", issues[0])
	}
	if len(issues[0].Labels) != 1 || issues[0].Labels[0].Name != "失联" {
		t.Errorf("issue[0] labels mismatch: %+v", issues[0].Labels)
	}
	if !strings.Contains(issues[0].Body, "友链信息") {
		t.Errorf("issue[0] body missing")
	}

	// 上游 500 → 报错
	c2 := New(Config{CacheDir: t.TempDir()})
	errSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer errSrv.Close()
	c2.apiBase = errSrv.URL
	if _, err := c2.GetIssues("o", "r", ""); err == nil {
		t.Fatal("expected error on upstream 500")
	}
}
