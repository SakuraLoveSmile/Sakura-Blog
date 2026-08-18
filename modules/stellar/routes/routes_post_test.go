package routes

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"

	"blog/internal/post"
	"blog/internal/store"
	tmpl "blog/modules/stellar/viewmodel"
	stellarweb "blog/modules/stellar/web"
)

func TestHandlePostFallbackHidesDraftFile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "draft.md"), []byte("---\ntitle: 草稿\nstatus: draft\n---\n\n仅供后台查看。\n"), 0o644); err != nil {
		t.Fatalf("write draft fixture: %v", err)
	}

	st, err := store.New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	eng, err := stellarweb.NewEngineFromEmbed(&tmpl.IconRegistry{})
	if err != nil {
		t.Fatalf("engine: %v", err)
	}
	ws := &webServer{
		site:      sitePtr(&tmpl.Site{Title: "测试站点", Widgets: tmpl.WidgetsConfig{}}),
		st:        st,
		posts:     post.NewService(root),
		engine:    eng,
		devMode:   true,
		uploadDir: t.TempDir(),
	}
	r := gin.New()
	r.GET("/post/:slug", ws.handlePost)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/post/draft", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("draft fallback status = %d, want 404", w.Code)
	}
}

func TestPluralPostPathRedirectsToCanonical(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	(&webServer{}).registerPostRoutes(r)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/posts/142-stellar-142", nil))
	if w.Code != http.StatusMovedPermanently {
		t.Fatalf("plural post status = %d, want %d", w.Code, http.StatusMovedPermanently)
	}
	if got := w.Header().Get("Location"); got != "/post/142-stellar-142" {
		t.Fatalf("plural post location = %q, want canonical path", got)
	}
}
