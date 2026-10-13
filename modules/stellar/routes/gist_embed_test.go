package routes

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"blog/internal/config"
	tmpl "blog/modules/stellar/viewmodel"
	"github.com/gin-gonic/gin"
	"golang.org/x/net/html"
)

func TestGistEmbedIsolationAndEncoding(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/stellar/embed/gist", gistEmbedHandler(func() string { return "https://gist.github.com" }))
	file := `code </script><script>alert(1)</script> & 中文.go`
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/stellar/embed/gist?"+url.Values{"gist": {"octocat/abcd1234"}, "file": {file}}.Encode(), nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	csp := w.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "sandbox allow-scripts") || strings.Contains(csp, "allow-same-origin") || !strings.Contains(csp, "frame-ancestors 'self'") {
		t.Fatalf("unsafe isolation: %s", csp)
	}
	doc, err := html.Parse(strings.NewReader(w.Body.String()))
	if err != nil {
		t.Fatal(err)
	}
	var scripts []*html.Node
	var visit func(*html.Node)
	visit = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "script" {
			scripts = append(scripts, n)
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(doc)
	if len(scripts) != 2 {
		t.Fatalf("script injection: got %d scripts", len(scripts))
	}
	var source string
	for _, attr := range scripts[0].Attr {
		if attr.Key == "src" {
			source = attr.Val
		}
	}
	u, err := url.Parse(source)
	if err != nil || u.Host != "gist.github.com" || u.Path != "/octocat/abcd1234.js" || u.Query().Get("file") != file {
		t.Fatalf("wrong official script: %q (%v)", source, err)
	}
	if w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Fatal("missing document boundaries")
	}
}

func TestGistEmbedRejectsScriptLocationsAndAmbiguousQueries(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/stellar/embed/gist", gistEmbedHandler(func() string { return "https://gist.github.com" }))
	for _, query := range []string{"", "gist=abcd", "gist=owner%2F..%2Fabcd", "gist=owner%2Fabcd.js", "gist=https%3A%2F%2Fevil.test%2Fx", "gist=o%2Fa&gist=o%2Fb", "gist=o%2Fa&src=https%3A%2F%2Fevil.test", "gist=o%2Fa&file=%zz"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/stellar/embed/gist?"+query, nil))
		if w.Code != http.StatusBadRequest {
			t.Errorf("%q got %d", query, w.Code)
		}
	}
	for _, base := range []string{"javascript:alert(1)", "//gist.github.com", "https://u:p@gist.github.com", "https://gist.github.com?x=y", "https://gist.github.com/#x"} {
		r := gin.New()
		r.GET("/stellar/embed/gist", gistEmbedHandler(func() string { return base }))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/stellar/embed/gist?gist=o%2Fa", nil))
		if w.Code != http.StatusInternalServerError {
			t.Errorf("base %q got %d", base, w.Code)
		}
	}
}

func TestGistRouteUsesCurrentStrictTheme(t *testing.T) {
	gin.SetMode(gin.TestMode)
	site := &tmpl.Site{}
	s := &webServer{site: sitePtr(site)}
	r := gin.New()
	s.registerEmbedRoutes(r)
	request := func() *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/stellar/embed/gist?gist=octocat%2Fabcd", nil))
		return w
	}
	if request().Code != http.StatusConflict {
		t.Fatal("unmigrated theme accepted")
	}
	configuration, err := config.ValidateStellarV2(map[string]any{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	next := *site
	next.StellarV2 = configuration
	s.site.Store(&next)
	if w := request(); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "https://gist.github.com/octocat/abcd.js") {
		t.Fatalf("default gist resource: %d %s", w.Code, w.Body.String())
	}
	configuration, err = config.ValidateStellarV2(map[string]any{"services": map[string]any{"github": map[string]any{"gist_url": "https://gist.example.test/embeds"}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	next = *site
	next.StellarV2 = configuration
	s.site.Store(&next)
	if w := request(); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "https://gist.example.test/embeds/octocat/abcd.js") {
		t.Fatalf("updated gist resource: %d %s", w.Code, w.Body.String())
	}
}
