package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// BLG-20261031-03：预览 iframe 是 sandbox="allow-scripts" 的 opaque origin，
// 读取公开 Fixture 端点属于跨源请求；这些端点必须返回 CORS 头，且预检可完成。
// 其余 /api/v1/preview/*（markdown 等需鉴权的渲染端点）不得被通配 CORS 覆盖。
func TestPreviewFixtureCORSHeaders(t *testing.T) {
	r, _, _, _ := newFullStellarEnv(t)

	cases := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/preview/rating"},
		{http.MethodPost, "/api/v1/preview/rating"},
		{http.MethodGet, "/api/v1/preview/rating/info"},
		{http.MethodPost, "/api/v1/preview/rating/update"},
		{http.MethodGet, "/api/v1/preview/vote/info"},
		{http.MethodPost, "/api/v1/preview/vote/update"},
		{http.MethodGet, "/api/v1/preview/friends"},
		{http.MethodGet, "/api/v1/preview/sites"},
		{http.MethodGet, "/api/v1/preview/md"},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(tc.method, tc.path, nil)
			r.ServeHTTP(w, req)
			if got := w.Header().Get("Access-Control-Allow-Origin"); got != "*" {
				t.Fatalf("Access-Control-Allow-Origin = %q, want *", got)
			}
			if got := w.Header().Get("Access-Control-Allow-Methods"); got == "" {
				t.Fatalf("Access-Control-Allow-Methods 为空")
			}
			if got := w.Header().Get("Access-Control-Allow-Credentials"); got != "" {
				t.Fatalf("公开 Fixture 不应允许凭据，got %q", got)
			}
		})
	}
}

func TestPreviewFixturePreflight(t *testing.T) {
	r, _, _, _ := newFullStellarEnv(t)

	for _, path := range []string{
		"/api/v1/preview/rating",
		"/api/v1/preview/rating/info",
		"/api/v1/preview/rating/update",
		"/api/v1/preview/vote/update",
		"/api/v1/preview/friends",
		"/api/v1/preview/sites",
		"/api/v1/preview/md",
	} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodOptions, path, nil)
		req.Header.Set("Origin", "null")
		req.Header.Set("Access-Control-Request-Method", "POST")
		req.Header.Set("Access-Control-Request-Headers", "Content-Type")
		r.ServeHTTP(w, req)
		if w.Code != http.StatusNoContent {
			t.Fatalf("%s preflight status = %d, want 204", path, w.Code)
		}
		if got := w.Header().Get("Access-Control-Allow-Origin"); got != "*" {
			t.Fatalf("%s preflight Access-Control-Allow-Origin = %q, want *", path, got)
		}
	}
}

func TestPreviewMarkdownIsNotWildcardCORS(t *testing.T) {
	r, _, _, _ := newFullStellarEnv(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/preview/markdown", nil)
	req.Header.Set("Origin", "https://evil.example")
	req.Header.Set("Access-Control-Request-Method", "POST")
	r.ServeHTTP(w, req)
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("需鉴权的预览渲染端点不应返回通配 CORS，got %q", got)
	}

	// 未鉴权 POST 必须 401，且同样不能获得通配 CORS。
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/preview/markdown", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://evil.example")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("未鉴权 preview/markdown status = %d, want 401", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("未鉴权 preview/markdown 不应返回 CORS，got %q", got)
	}
}

// rating.js / vote.js 的真实契约：/info 返回可计算的数据，/update 接受动作但不落库。
func TestPreviewRatingAndVoteFixtureShapes(t *testing.T) {
	r, _, _, _ := newFullStellarEnv(t)

	getJSON := func(path string) (int, map[string]any) {
		t.Helper()
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		var body map[string]any
		if w.Body.Len() > 0 {
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatalf("%s decode: %v", path, err)
			}
		}
		return w.Code, body
	}

	code, body := getJSON("/api/v1/preview/rating/info?id=preview-demo")
	if code != http.StatusOK {
		t.Fatalf("rating/info status = %d", code)
	}
	rating, ok := body["rating"].(map[string]any)
	if !ok || len(rating) == 0 {
		t.Fatalf("rating/info 形状应为 {rating:{score:count}}，got %v", body)
	}
	if rating["5"] != float64(86) {
		t.Fatalf("rating/info 5 星数量 = %v, want 86", rating["5"])
	}

	code, body = getJSON("/api/v1/preview/rating/info?state=empty")
	if code != http.StatusOK {
		t.Fatalf("rating/info empty status = %d", code)
	}
	if rating, ok := body["rating"].(map[string]any); !ok || len(rating) != 0 {
		t.Fatalf("rating/info empty 应为空对象，got %v", body)
	}

	code, _ = getJSON("/api/v1/preview/rating/info?state=error")
	if code != http.StatusInternalServerError {
		t.Fatalf("rating/info error status = %d, want 500", code)
	}

	code, body = getJSON("/api/v1/preview/vote/info?id=preview-demo")
	if code != http.StatusOK {
		t.Fatalf("vote/info status = %d", code)
	}
	votes, ok := body["votes"].(map[string]any)
	if !ok || votes["up"] == nil || votes["down"] == nil {
		t.Fatalf("vote/info 形状应为 {votes:{up,down}}，got %v", body)
	}

	for _, path := range []string{
		"/api/v1/preview/rating/update?id=preview-demo&value=5",
		"/api/v1/preview/vote/update?id=preview-demo&value=up",
	} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, path, nil))
		if w.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want 200", path, w.Code)
		}
		if got := w.Header().Get("Access-Control-Allow-Origin"); got != "*" {
			t.Fatalf("%s Access-Control-Allow-Origin = %q, want *", path, got)
		}
	}
}
