package routes

import (
	"bytes"
	"errors"
	"log"
	"strings"
	"testing"

	"blog/internal/content"
	"blog/internal/search"
)

func TestStrictSearchStartupAndContentLifecycle(t *testing.T) {
	for _, useEvents := range []bool{false, true} {
		name := "store-hooks"
		if useEvents {
			name = "content-events"
		}
		t.Run(name, func(t *testing.T) {
			ws, st, _, _ := stellarRouteFixture(t)
			row := createStellarPost(t, st, "canonical-search", "")
			row.ContentMD = "CurrentCanonicalOnly"
			row.ContentHTML = "<p>ObsoleteCachedOnly</p>"
			if err := st.Update(row.Slug, row); err != nil {
				t.Fatal(err)
			}
			if err := ws.searchIdx.IndexDoc(search.TypePost, row.Slug, row.Title, "ObsoleteCachedOnly"); err != nil {
				t.Fatal(err)
			}
			var events content.Events
			if useEvents {
				events = content.NewEvents()
				st.SetContentEvents(events)
			}
			server, err := New(Dependencies{Store: st, Site: ws.site, Posts: ws.posts, Search: ws.searchIdx, Events: events})
			if err != nil {
				t.Fatal(err)
			}
			defer server.Close()
			check := func(word string, want int) {
				t.Helper()
				got, err := ws.searchIdx.Search(word, search.TypePost, 20)
				if err != nil || len(got) != want {
					t.Fatalf("%s: hits=%d want=%d err=%v", word, len(got), want, err)
				}
			}
			check("CurrentCanonicalOnly", 1)
			check("ObsoleteCachedOnly", 0)
			row.ContentMD = "UpdatedCanonicalOnly"
			if err := st.Update(row.Slug, row); err != nil {
				t.Fatal(err)
			}
			check("UpdatedCanonicalOnly", 1)
			check("CurrentCanonicalOnly", 0)
			check("ObsoleteCachedOnly", 0)
			if events != nil {
				if err := ws.searchIdx.IndexDoc(search.TypePost, row.Slug, row.Title, "RestoredStaleOnly"); err != nil {
					t.Fatal(err)
				}
				events.Publish(content.Event{Kind: content.Restored})
				check("UpdatedCanonicalOnly", 1)
				check("RestoredStaleOnly", 0)
			}
			persisted, err := st.GetBySlug(row.Slug)
			if err != nil || persisted.ContentHTML != row.ContentHTML {
				t.Fatal("search rebuild rewrote preserved legacy HTML")
			}
		})
	}
}

func TestStrictSearchStartupAllowsBodyRepairAndClearsStalePrivateResults(t *testing.T) {
	var diagnostic bytes.Buffer
	previousOutput := log.Writer()
	log.SetOutput(&diagnostic)
	t.Cleanup(func() { log.SetOutput(previousOutput) })
	ws, st, _, r := stellarRouteFixture(t)
	row := createStellarPost(t, st, "broken-search", "")
	row.ContentMD = "{% tabs %}"
	if err := st.Update(row.Slug, row); err != nil {
		t.Fatal(err)
	}
	if err := ws.searchIdx.IndexDoc(search.TypePost, "removed-private", "PRIVATE_STALE_RESULT", "PRIVATE_STALE_RESULT"); err != nil {
		t.Fatal(err)
	}
	server, err := New(Dependencies{Store: st, Site: ws.site, Posts: ws.posts, Search: ws.searchIdx})
	if err != nil {
		t.Fatalf("strict Console startup blocked by reparable body: %v", err)
	}
	defer server.Close()
	for _, message := range []string{"posts/", "missing endtabs", "stale FTS cleared", "Console repair remains available"} {
		if !strings.Contains(diagnostic.String(), message) {
			t.Fatalf("startup diagnostic missing %q: %s", message, diagnostic.String())
		}
	}
	if n, err := ws.searchIdx.Count(); err != nil || n != 0 {
		t.Fatalf("stale index retained: %d %v", n, err)
	}
	if got, err := ws.searchIdx.Search("PRIVATE_STALE_RESULT", "", 20); err != nil || len(got) != 0 {
		t.Fatalf("private stale result reused: %+v %v", got, err)
	}
	if response := stellarRequest(r, "/post/broken-search"); response.Code != 500 {
		t.Fatalf("broken public body was hidden: %d", response.Code)
	}
	row.ContentMD = "RestoredCurrentNeedle"
	if err := st.Update(row.Slug, row); err != nil {
		t.Fatal(err)
	}
	if got, err := ws.searchIdx.Search("RestoredCurrentNeedle", "", 20); err != nil || len(got) != 1 {
		t.Fatalf("repair did not rebuild current search: %+v %v", got, err)
	}
}

func TestStrictSearchKeepsConfigurationAndStructureStartupGates(t *testing.T) {
	for _, raw := range []string{`{"style":{}}`, `{"collection":{"profile":"topic","id":"missing"}}`, `{}`} {
		t.Run(raw, func(t *testing.T) {
			ws, st, _, _ := stellarRouteFixture(t)
			row := createStellarPost(t, st, "invalid-config", "")
			version := 2
			if raw == `{}` {
				version = 0
			}
			if _, err := st.DB().Exec(`UPDATE posts SET stellar_version=?,stellar_config=? WHERE id=?`, version, raw, row.ID); err != nil {
				t.Fatal(err)
			}
			if server, err := New(Dependencies{Store: st, Site: ws.site, Posts: ws.posts, Search: ws.searchIdx}); err == nil {
				server.Close()
				t.Fatal("strict configuration/structure gate was bypassed")
			}
		})
	}
}

func TestStrictSearchClearFailureDisablesSharedConsumersAndRecoversAfterRepair(t *testing.T) {
	ws, st, _, _ := stellarRouteFixture(t)
	row := createStellarPost(t, st, "current", "")
	events := content.NewEvents()
	st.SetContentEvents(events)
	server, err := New(Dependencies{Store: st, Site: ws.site, Posts: ws.posts, Search: ws.searchIdx, Events: events})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	if _, err := st.DB().Exec(`UPDATE posts SET content_md=? WHERE id=?`, "{% tabs %}", row.ID); err != nil {
		t.Fatal(err)
	}
	if err := ws.searchIdx.IndexDoc(search.TypePost, "now-private", "PrivateStaleNeedle", "PrivateStaleNeedle"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().Exec(`PRAGMA query_only=ON`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = st.DB().Exec(`PRAGMA query_only=OFF`) })
	var diagnostic bytes.Buffer
	previousOutput := log.Writer()
	log.SetOutput(&diagnostic)
	t.Cleanup(func() { log.SetOutput(previousOutput) })
	events.Publish(content.Event{Kind: content.ContentChanged})
	if _, err := st.DB().Exec(`PRAGMA query_only=OFF`); err != nil {
		t.Fatal(err)
	}
	for _, message := range []string{"clear failed strict search", "missing endtabs", "index unavailable"} {
		if !strings.Contains(diagnostic.String(), message) {
			t.Fatalf("failed-clear diagnostic missing %q: %s", message, diagnostic.String())
		}
	}
	if got, err := ws.searchIdx.Search("PrivateStaleNeedle", "", 20); !errors.Is(err, search.ErrUnavailable) || len(got) != 0 {
		t.Fatalf("failed clear exposed old results: %+v %v", got, err)
	}
	if err := ws.searchIdx.IndexDoc(search.TypePost, "incremental", "Incremental", "Incremental"); !errors.Is(err, search.ErrUnavailable) {
		t.Fatalf("incremental write bypassed full recovery: %v", err)
	}
	row.ContentMD = "RecoveredNeedle"
	if err := st.Update(row.Slug, row); err != nil {
		t.Fatal(err)
	}
	if got, err := ws.searchIdx.Search("RecoveredNeedle", "", 20); err != nil || len(got) != 1 {
		t.Fatalf("repair did not restore search: %+v %v", got, err)
	}
	if got, err := ws.searchIdx.Search("PrivateStaleNeedle", "", 20); err != nil || len(got) != 0 {
		t.Fatalf("recovery retained stale private data: %+v %v", got, err)
	}
}

func TestStrictSearchStoreHookClearsBodyFailureAndRebuildsAfterRepair(t *testing.T) {
	ws, st, _, _ := stellarRouteFixture(t)
	row := createStellarPost(t, st, "current", "")
	server, err := New(Dependencies{Store: st, Site: ws.site, Posts: ws.posts, Search: ws.searchIdx})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	if err := ws.searchIdx.IndexDoc(search.TypePost, "now-private", "PrivateStaleNeedle", "PrivateStaleNeedle"); err != nil {
		t.Fatal(err)
	}
	row.ContentMD = "{% tabs %}"
	if err := st.Update(row.Slug, row); err != nil {
		t.Fatal(err)
	}
	if n, err := ws.searchIdx.Count(); err != nil || n != 0 {
		t.Fatalf("body failure left stale search: %d %v", n, err)
	}
	row.ContentMD = "HookRecoveredNeedle"
	if err := st.Update(row.Slug, row); err != nil {
		t.Fatal(err)
	}
	if got, err := ws.searchIdx.Search("HookRecoveredNeedle", "", 20); err != nil || len(got) != 1 {
		t.Fatalf("store hook did not rebuild after repair: %+v %v", got, err)
	}
}

func TestStrictSearchStartupRejectsBodyFailureWhenStaleIndexCannotBeCleared(t *testing.T) {
	ws, st, _, _ := stellarRouteFixture(t)
	row := createStellarPost(t, st, "broken", "")
	row.ContentMD = "{% tabs %}"
	if err := st.Update(row.Slug, row); err != nil {
		t.Fatal(err)
	}
	// 故障夹具保留 Ensure 所需列，但拒绝 DELETE，使清空失败发生在正文错误之后。
	for _, statement := range []string{
		`DROP TABLE posts_fts`,
		`CREATE TABLE posts_fts(title TEXT, content TEXT, slug TEXT, title_raw TEXT, doc_type TEXT)`,
		`INSERT INTO posts_fts VALUES ('PrivateStaleNeedle', 'PrivateStaleNeedle', 'private', 'PrivateStaleNeedle', 'post')`,
		`CREATE TRIGGER reject_fts_clear BEFORE DELETE ON posts_fts BEGIN SELECT RAISE(ABORT, 'injected clear failure'); END`,
	} {
		if _, err := st.DB().Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	server, err := New(Dependencies{Store: st, Site: ws.site, Posts: ws.posts, Search: ws.searchIdx})
	if err == nil {
		server.Close()
		t.Fatal("startup accepted a failed stale-index clear")
	}
	if !strings.Contains(err.Error(), "clear failed strict search") || !strings.Contains(err.Error(), "missing endtabs") || !strings.Contains(err.Error(), "injected clear failure") {
		t.Fatalf("startup failure omitted the body/clear diagnostics: %v", err)
	}
	var body *content.StellarBodyRenderError
	if errors.As(err, &body) {
		t.Fatal("failed clear still classified as safely reparable startup")
	}
	if got, err := ws.searchIdx.Search("PrivateStaleNeedle", "", 20); !errors.Is(err, search.ErrUnavailable) || len(got) != 0 {
		t.Fatalf("rejected startup left shared stale search available: %+v %v", got, err)
	}
}
