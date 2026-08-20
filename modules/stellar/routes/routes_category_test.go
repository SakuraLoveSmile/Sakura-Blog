package routes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"blog/internal/store"
)

func TestNestedCategoryDetailAndRSSRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ws, st := newWikiVerifyServer(t)
	if _, err := st.Create(&store.Post{
		Slug:        "nested-category",
		Title:       "嵌套分类文章",
		Status:      store.StatusPublished,
		ContentHTML: "<p>nested</p>",
		Categories:  []string{"产研/技术/前端"},
		CreatedAt:   time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}

	server := &Server{deps: Dependencies{Store: st, Site: ws.site}, web: ws}
	r := gin.New()
	ws.registerTaxonomyRoutes(r)
	r.NoRoute(server.NoRoute(nil))

	request := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		return w
	}

	detail := request("/categories/产研/技术/")
	if detail.Code != http.StatusOK || !strings.Contains(detail.Body.String(), "嵌套分类文章") {
		t.Fatalf("nested category detail = %d, body missing article", detail.Code)
	}

	rss := request("/categories/产研/技术/rss.xml")
	if rss.Code != http.StatusOK || !strings.Contains(rss.Body.String(), "嵌套分类文章") {
		t.Fatalf("nested category rss = %d, body missing article", rss.Code)
	}

	missing := request("/categories/产研/不存在/")
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing nested category = %d, want 404", missing.Code)
	}
}
