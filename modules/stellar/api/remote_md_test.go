package api

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"blog/internal/safefetch"
	"github.com/gin-gonic/gin"
)

type remoteMDTransport func(*http.Request) (*http.Response, error)

func (f remoteMDTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func mdClient(body string, status int) *http.Client {
	return &http.Client{Transport: remoteMDTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Request: r, Header: make(http.Header)}, nil
	})}
}
func mdRouter(client *http.Client, site string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	mountRemoteMarkdown(r, site, remoteMarkdownHandler(client))
	return r
}
func mdGet(r *gin.Engine, endpoint, source string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", endpoint+"?url="+url.QueryEscape(source), nil))
	return w
}

func TestRemoteMarkdownSanitizesScriptsAndKeepsGFM(t *testing.T) {
	body := "# Title\n\n## Section\n\n**bold** [relative](../doc.md) ![image](img.png)\n\n|A|B|\n|-|-|\n|x|y|\n\n<script>alert(1)</script><img src=\"x\" onerror=\"alert(2)\"><a href=\"javascript:alert(3)\">bad</a><iframe src=\"https://evil.test\"></iframe><svg onload=\"alert(4)\"></svg>"
	r := mdRouter(mdClient(body, 200), "https://site.test/sub")
	for _, endpoint := range []string{"/api/stellar/remote-md", "/sub/api/stellar/remote-md"} {
		w := mdGet(r, endpoint, "https://source.test/repo/README.md?raw=1")
		if w.Code != 200 {
			t.Fatalf("%s: %d %s", endpoint, w.Code, w.Body.String())
		}
		var got remoteMarkdownResponse
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		for _, bad := range []string{"<script", "onerror", "javascript:", "<iframe", "<svg", "onload"} {
			if strings.Contains(got.HTML, bad) {
				t.Fatalf("unsafe %s: %s", bad, got.HTML)
			}
		}
		for _, want := range []string{"<h2>Section</h2>", "<strong>bold</strong>", "<table>", "../doc.md", "img.png"} {
			if !strings.Contains(got.HTML, want) {
				t.Fatalf("missing %s: %s", want, got.HTML)
			}
		}
		if got.SourceURL != "https://source.test/repo/README.md?raw=1" {
			t.Fatal(got.SourceURL)
		}
	}
}
func TestRemoteMarkdownInvalidPrivateFailureAndLimit(t *testing.T) {
	r := mdRouter(mdClient("not fetched", 200), "")
	for _, raw := range []string{"/relative.md", "file:///tmp/a", "ftp://source.test/a", "http://127.0.0.1/a", "http://10.0.0.1/a", "http://[::1]/a", "https://user:pass@source.test/a"} {
		if w := mdGet(r, "/api/stellar/remote-md", raw); w.Code != 400 {
			t.Fatalf("%s: %d", raw, w.Code)
		}
	}
	if w := mdGet(mdRouter(mdClient("missing", 404), ""), "/api/stellar/remote-md", "https://source.test/a"); w.Code != 502 {
		t.Fatal(w.Code)
	}
	if w := mdGet(mdRouter(mdClient(strings.Repeat("a", remoteMarkdownLimit+1), 200), ""), "/api/stellar/remote-md", "https://source.test/a"); w.Code != 413 {
		t.Fatal(w.Code)
	}
}
func TestRemoteMarkdownRequestCancellation(t *testing.T) {
	entered, done := make(chan struct{}), make(chan struct{})
	client := &http.Client{Transport: remoteMDTransport(func(r *http.Request) (*http.Response, error) {
		close(entered)
		<-r.Context().Done()
		close(done)
		return nil, r.Context().Err()
	})}
	r := mdRouter(client, "")
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest("GET", "/api/stellar/remote-md?url=https%3A%2F%2Fsource.test%2Fa", nil).WithContext(ctx)
	finished := make(chan struct{})
	go func() { r.ServeHTTP(httptest.NewRecorder(), req); close(finished) }()
	<-entered
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("fetch was not cancelled")
	}
	<-finished
}
func TestRemoteMarkdownProductionFetchRejectsDNSAndRedirectPrivate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "http://127.0.0.1/private", http.StatusFound) }))
	defer server.Close()
	addr := strings.TrimPrefix(server.URL, "http://")
	client := safefetch.New(safefetch.Options{MaxRedirects: 5, ResolveIP: func(_ context.Context, host string) ([]net.IP, error) {
		if host == "private.test" {
			return []net.IP{net.ParseIP("10.0.0.1")}, nil
		}
		return []net.IP{net.ParseIP("93.184.216.34")}, nil
	}, DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, addr)
	}, RedirectValidator: func(u *url.URL) error { _, err := safefetch.ValidateURL(u.String()); return err }})
	r := mdRouter(client, "")
	for _, raw := range []string{"http://private.test/readme", "http://public.test/readme"} {
		if w := mdGet(r, "/api/stellar/remote-md", raw); w.Code != 502 {
			t.Fatalf("%s: %d", raw, w.Code)
		}
	}
}
