package seo

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"blog/internal/safefetch"
)

func callAnalyze(t *testing.T, m *Module, target string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/analyze", m.analyzeHandler)
	w := httptest.NewRecorder()
	payload, _ := json.Marshal(map[string]string{"url": target})
	req := httptest.NewRequest(http.MethodPost, "/analyze", strings.NewReader(string(payload)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func TestAnalyzeRejectsPrivateAndLoopbackTargets(t *testing.T) {
	for _, target := range []string{
		"http://127.0.0.1/",
		"http://10.0.0.1/",
		"http://192.168.1.1/",
		"http://172.16.0.1/",
		"http://169.254.169.254/latest/meta-data",
	} {
		w := callAnalyze(t, &Module{}, target)
		if w.Code != http.StatusOK {
			t.Errorf("%s status = %d, want 200 with analysis error", target, w.Code)
		}
		if strings.Contains(w.Body.String(), "safefetch") || strings.Contains(w.Body.String(), "169.254") {
			t.Errorf("%s leaked network details: %s", target, w.Body.String())
		}
		var body map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("%s response is not JSON: %v", target, err)
		}
		if body["error"] != "无法访问该页面" {
			t.Errorf("%s error = %v, want generic message", target, body["error"])
		}
	}
}

func TestAnalyzeRejectsUnsupportedScheme(t *testing.T) {
	w := callAnalyze(t, &Module{}, "file:///etc/passwd")
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "仅支持 http 或 https") {
		t.Fatalf("unsupported scheme response = %d %s", w.Code, w.Body.String())
	}
}

func TestAnalyzeAllowsValidatedPublicTarget(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, `<!doctype html><html lang="zh-CN"><head><title>一个合适的页面标题</title><meta name="description" content="desc"><link rel="canonical" href="https://example.test/"><meta property="og:title" content="title"><meta property="og:description" content="desc"><meta name="viewport" content="width=device-width"></head><body></body></html>`)
	}))
	defer server.Close()
	port := server.Listener.Addr().(*net.TCPAddr).Port
	client := localSafeClient(t, server, "public.test", "private.test")
	m := &Module{analyzeClient: client}
	w := callAnalyze(t, m, "http://public.test:"+itoa(port)+"/")
	if w.Code != http.StatusOK {
		t.Fatalf("public target status = %d, body = %s", w.Code, w.Body.String())
	}
	var result analyzeResult
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode analyze result: %v", err)
	}
	if result.URL == "" || result.Score == 0 {
		t.Fatalf("public target was not analyzed: %+v", result)
	}
}

func TestAnalyzeRejectsRedirectToPrivateTarget(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			_, port, _ := net.SplitHostPort(r.Host)
			http.Redirect(w, r, "http://private.test:"+port+"/secret", http.StatusFound)
			return
		}
		t.Error("private redirect target should never be requested")
	}))
	defer server.Close()
	port := server.Listener.Addr().(*net.TCPAddr).Port
	m := &Module{analyzeClient: localSafeClient(t, server, "public.test", "private.test")}
	w := callAnalyze(t, m, "http://public.test:"+itoa(port)+"/redirect")
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || w.Code != http.StatusOK || body["error"] != "无法访问该页面" {
		t.Fatalf("private redirect response = %d %s", w.Code, w.Body.String())
	}
}

func TestAnalyzeDoesNotExposeNetworkError(t *testing.T) {
	m := &Module{analyzeClient: failingAnalyzeClient{}}
	w := callAnalyze(t, m, "https://example.test/")
	if w.Code != http.StatusOK || strings.Contains(w.Body.String(), "127.0.0.1") || strings.Contains(w.Body.String(), "dial tcp") {
		t.Fatalf("network error leaked: %d %s", w.Code, w.Body.String())
	}
}

type failingAnalyzeClient struct{}

func (failingAnalyzeClient) Do(*http.Request) (*http.Response, error) {
	return nil, errors.New("dial tcp 127.0.0.1:8080: connection refused")
}

func localSafeClient(t *testing.T, server *httptest.Server, publicHost, privateHost string) *http.Client {
	t.Helper()
	return safefetch.New(safefetch.Options{
		Timeout:      3 * time.Second,
		MaxRedirects: 5,
		ResolveIP: func(_ context.Context, host string) ([]net.IP, error) {
			switch host {
			case publicHost:
				return []net.IP{net.ParseIP("1.1.1.1")}, nil
			case privateHost:
				return []net.IP{net.ParseIP("127.0.0.1")}, nil
			default:
				return nil, errors.New("unexpected test host: " + host)
			}
		},
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
		},
	})
}

func itoa(n int) string {
	return strconv.Itoa(n)
}
