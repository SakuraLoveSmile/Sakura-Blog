package linkwatch

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"blog/internal/config"
	"blog/internal/store"
	tmpl "blog/internal/template"
)

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

	watcher := New(st, sitePtr, filepath.Join(t.TempDir(), "uploads"))
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
