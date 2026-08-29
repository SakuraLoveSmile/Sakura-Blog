package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"blog/internal/post"
	tmpl "blog/internal/settings"
)

func TestShortcodeCatalogRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st := newTestStore(t)
	r := gin.New()
	if _, err := MountConsole(r, MountDeps{Config: Config{Store: st, Site: sitePtr(&tmpl.Site{})}}); err != nil {
		t.Fatalf("mount: %v", err)
	}

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/plugins/catalog", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("catalog status = %d, want 200: %s", w.Code, w.Body.String())
	}

	var response ShortcodeCatalogResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode catalog: %v", err)
	}
	if len(response.Items) < 50 {
		t.Fatalf("catalog item count = %d, want at least 50", len(response.Items))
	}
	for _, item := range response.Items {
		if item.Name == "note" {
			if item.Category != "提示标注" || item.Capability != "static" || len(item.Params) == 0 {
				t.Fatalf("note catalog entry incomplete: %+v", item)
			}
			return
		}
	}
	t.Fatal("catalog does not contain note")
}

func TestShortcodeEditRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st := newTestStore(t)
	r := gin.New()
	if _, err := MountConsole(r, MountDeps{Config: Config{Store: st, Site: sitePtr(&tmpl.Site{})}}); err != nil {
		t.Fatalf("mount: %v", err)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/plugins/shortcode/parse", strings.NewReader(`{"raw":"{% note color:green 提示 这里是正文 %}"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("shortcode edit status = %d, want 200: %s", w.Code, w.Body.String())
	}

	var response post.ShortcodeEditResult
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode shortcode edit: %v", err)
	}
	if response.Name != "note" || !response.Editable || response.Values["title"] != "提示" {
		t.Fatalf("shortcode edit response = %+v", response)
	}
}
