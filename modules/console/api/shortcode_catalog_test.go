package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

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
