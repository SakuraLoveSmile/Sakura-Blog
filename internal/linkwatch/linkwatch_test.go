package linkwatch

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"blog/internal/config"
	tmpl "blog/internal/settings"
	"blog/internal/store"
)

func TestRunOnceRebuildsLinkHealthAndNotifies(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()

	st, err := store.New(filepath.Join(t.TempDir(), "blog.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.EnsureLinksTable(); err != nil {
		t.Fatal(err)
	}
	if err := st.EnsureSettingsTable(); err != nil {
		t.Fatal(err)
	}
	if err := st.EnsureUsersTable(); err != nil {
		t.Fatal(err)
	}
	site := config.DefaultSite()
	site.Friends.CheckInterval = 1
	site.Friends.SnapshotInterval = 0
	sitePtr := &atomic.Pointer[tmpl.Site]{}
	sitePtr.Store(site)
	link := &store.Link{Name: "custom label", URL: server.URL, Label: "博客"}
	if _, err := st.CreateLink(link); err != nil {
		t.Fatal(err)
	}

	watcher := NewWithClients(st, sitePtr, t.TempDir(), &http.Client{Timeout: 3 * time.Second}, &http.Client{Timeout: 3 * time.Second})
	var changed atomic.Int32
	watcher.SetOnChanged(func() { changed.Add(1) })
	if err := watcher.RunOnce(); err != nil {
		t.Fatal(err)
	}
	if changed.Load() != 1 {
		t.Fatalf("onChanged calls = %d, want 1", changed.Load())
	}
	live := sitePtr.Load()
	if len(live.Links) != 1 || !live.Links[0].Items[0].Unavailable {
		t.Fatalf("rebuilt link state = %+v", live.Links)
	}
}

func TestRunOncePersistsStatusAndSnapshot(t *testing.T) {
	friend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer friend.Close()
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("fake-png"))
	}))
	defer provider.Close()

	st, err := store.New(filepath.Join(t.TempDir(), "blog.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.EnsureLinksTable(); err != nil {
		t.Fatal(err)
	}
	if err := st.EnsureSettingsTable(); err != nil {
		t.Fatal(err)
	}
	if err := st.EnsureUsersTable(); err != nil {
		t.Fatal(err)
	}
	site := config.DefaultSite()
	site.Friends.CheckInterval = 1
	site.Friends.SnapshotInterval = 1
	site.Friends.SnapshotProvider = provider.URL + "/capture?url={url}"
	sitePtr := &atomic.Pointer[tmpl.Site]{}
	sitePtr.Store(site)
	link := &store.Link{Name: "friend", URL: friend.URL}
	if _, err := st.CreateLink(link); err != nil {
		t.Fatal(err)
	}

	// 安全审计 2026-08-16：生产 client 走 safefetch（拒绝环回），单测对
	// httptest 服务验证探测/截图逻辑，注入普通 client。
	watcher := NewWithClients(st, sitePtr, filepath.Join(t.TempDir(), "uploads"),
		&http.Client{Timeout: 3 * time.Second}, &http.Client{Timeout: 3 * time.Second})
	if err := watcher.RunOnce(); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetLink(link.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.LastStatus != http.StatusNoContent || got.LastCheckedAt == 0 {
		t.Fatalf("status was not persisted: %+v", got)
	}
	if got.Snapshot != "/uploads/snapshots/1.jpg" {
		t.Fatalf("snapshot URL = %q", got.Snapshot)
	}
	if _, err := os.Stat(filepath.Join(watcher.uploadDir, "snapshots", "1.jpg")); err != nil {
		t.Fatalf("snapshot file missing: %v", err)
	}
}

func TestBuildSnapshotProviderURL(t *testing.T) {
	target := "https://example.com/articles?page=2&from=home#top"

	pathProvider := "https://image.thum.io/get/width/1280/crop/720/{url}"
	pathURL, err := buildSnapshotProviderURL(pathProvider, target)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(pathURL, "/https://example.com/articles") {
		t.Fatalf("path provider lost target URL shape: %s", pathURL)
	}
	if strings.Contains(pathURL, "?page=2") || strings.Contains(pathURL, "#top") {
		t.Fatalf("path target rewrote provider URL: %s", pathURL)
	}
	if !strings.Contains(pathURL, "%3Fpage%3D2%26from%3Dhome%23top") {
		t.Fatalf("path target delimiters were not escaped: %s", pathURL)
	}

	queryProvider := "https://capture.example/render?url={url}"
	queryURL, err := buildSnapshotProviderURL(queryProvider, target)
	if err != nil {
		t.Fatal(err)
	}
	wantQueryURL := "https://capture.example/render?url=" + url.QueryEscape(target)
	if queryURL != wantQueryURL {
		t.Fatalf("query provider URL = %q, want %q", queryURL, wantQueryURL)
	}
}

func TestBuildSnapshotProviderURLRejectsUnsupportedPlaceholderLocation(t *testing.T) {
	if _, err := buildSnapshotProviderURL("https://capture.example/render#{url}", "https://example.com"); err == nil {
		t.Fatal("fragment placeholder should be rejected")
	}
}

func TestCaptureSupportsPathSnapshotProvider(t *testing.T) {
	const target = "https://example.com/articles?page=2&from=home#top"
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.RequestURI, "/https://example.com/articles%3Fpage%3D2%26from%3Dhome%23top") {
			t.Errorf("provider request URI = %q", r.RequestURI)
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("fake-png"))
	}))
	defer provider.Close()

	watcher := &Watcher{
		snapshot:  provider.Client(),
		uploadDir: t.TempDir(),
	}
	path, err := watcher.capture(&store.Link{ID: 7, URL: target}, provider.URL+"/capture/{url}")
	if err != nil {
		t.Fatal(err)
	}
	if path != "/uploads/snapshots/7.jpg" {
		t.Fatalf("snapshot path = %q", path)
	}
}

func TestCaptureRejectsNonImage(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("not an image"))
	}))
	defer provider.Close()
	st, err := store.New(filepath.Join(t.TempDir(), "blog.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.EnsureLinksTable(); err != nil {
		t.Fatal(err)
	}
	if err := st.EnsureSettingsTable(); err != nil {
		t.Fatal(err)
	}
	if err := st.EnsureUsersTable(); err != nil {
		t.Fatal(err)
	}
	site := config.DefaultSite()
	site.Friends.SnapshotProvider = provider.URL + "?url={url}"
	sitePtr := &atomic.Pointer[tmpl.Site]{}
	sitePtr.Store(site)
	link := &store.Link{Name: "friend", URL: "https://example.com"}
	if _, err := st.CreateLink(link); err != nil {
		t.Fatal(err)
	}
	watcher := New(st, sitePtr, t.TempDir())
	if _, err := watcher.Snapshot(link.ID); err == nil {
		t.Fatal("expected non-image provider response to fail")
	}
}

func TestSnapshotExpiredRejectsPathsOutsideUploadDir(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(filepath.Dir(root), "outside-snapshot.jpg")
	if err := os.WriteFile(outside, []byte("snapshot"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(outside) })

	link := &store.Link{Snapshot: "/uploads/../outside-snapshot.jpg"}
	if !snapshotExpired(link, root, time.Hour) {
		t.Fatal("snapshot outside upload directory must be treated as expired")
	}

	link.Snapshot = "/uploads/" + strings.ReplaceAll(filepath.Join("nested", "snapshot.jpg"), string(filepath.Separator), "/")
	if !snapshotExpired(link, root, time.Hour) {
		t.Fatal("missing snapshot must be treated as expired")
	}
}
