package routes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"blog/internal/store"
)

// 旧数据尚未迁移时，浏览器看到可读说明页；脚本和 API 客户端仍得到原来的 JSON 错误。
func TestStellarMigrationPendingGivesBrowsersReadablePage(t *testing.T) {
	_, st, _, r := stellarRouteFixture(t)
	if _, err := st.Create(&store.Post{Slug: "legacy", Title: "LEGACY_TITLE", Status: store.StatusPublished, ContentMD: "LEGACY_BODY"}); err != nil {
		t.Fatal(err)
	}
	request := func(path, accept string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if accept != "" {
			req.Header.Set("Accept", accept)
		}
		r.ServeHTTP(w, req)
		return w
	}
	for _, path := range []string{"/", "/post/legacy"} {
		w := request(path, "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
		body := w.Body.String()
		if w.Code != http.StatusConflict || !strings.HasPrefix(w.Header().Get("Content-Type"), "text/html") {
			t.Fatalf("%s browser => %d %s", path, w.Code, w.Header().Get("Content-Type"))
		}
		if !strings.Contains(body, "站点内容正在升级") || !strings.Contains(body, "stellar_migration_required") || strings.HasPrefix(strings.TrimSpace(body), "{") {
			t.Fatalf("%s browser page is not the readable notice: %s", path, body)
		}
		if w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("X-Robots-Tag") != "noindex" {
			t.Fatalf("%s notice must not be cached or indexed: %v", path, w.Header())
		}
		if strings.Contains(body, "LEGACY_BODY") || strings.Contains(body, "LEGACY_TITLE") {
			t.Fatalf("%s notice leaked legacy content", path)
		}
	}
	for _, accept := range []string{"", "application/json", "*/*"} {
		w := request("/", accept)
		if w.Code != http.StatusConflict || !strings.HasPrefix(w.Header().Get("Content-Type"), "application/json") || !strings.Contains(w.Body.String(), `"code":"stellar_migration_required"`) {
			t.Fatalf("Accept %q must keep the JSON error: %d %s %s", accept, w.Code, w.Header().Get("Content-Type"), w.Body.String())
		}
	}
}
