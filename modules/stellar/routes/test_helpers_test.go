package routes

import (
	"encoding/json"
	"sync/atomic"
	"testing"

	"blog/internal/config"
	"blog/internal/store"
	tmpl "blog/modules/stellar/viewmodel"
)

// sitePtr 包装 *tmpl.Site 为原子指针，供路由测试构造 webServer。
func sitePtr(s *tmpl.Site) *atomic.Pointer[tmpl.Site] {
	p := &atomic.Pointer[tmpl.Site]{}
	p.Store(s)
	return p
}

// canonicalRouteSite 显式选择上游默认主题，绝不从旧字段推断迁移。
func canonicalRouteSite(t *testing.T, title string, theme, blog map[string]any) *tmpl.Site {
	t.Helper()
	if theme == nil {
		theme = map[string]any{}
	}
	v2, err := config.ValidateStellarV2(theme, blog)
	if err != nil {
		t.Fatal(err)
	}
	return &tmpl.Site{Title: title, URL: "http://example.com/", StellarV2: v2}
}

func ensureRouteSchema(t *testing.T, st *store.Store) {
	t.Helper()
	for _, ensure := range []func() error{st.EnsureSchema, st.EnsureWikiProjectsTable, st.EnsureTopicsTable, st.EnsureWikidocsTable, st.EnsureNotebooksTable, st.EnsureNotesTable, st.EnsureMomentsTable} {
		if err := ensure(); err != nil {
			t.Fatal(err)
		}
	}
}

func createRouteWikiProject(t *testing.T, st *store.Store, id string, raw map[string]any) {
	t.Helper()
	b, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateWikiProject(&store.WikiProject{ID: id, Visible: true, StellarVersion: 2, StellarConfig: string(b), BlogExtensions: "{}"}); err != nil {
		t.Fatal(err)
	}
}
