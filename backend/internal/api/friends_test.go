package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"blog/internal/store"
)

func TestProbeURLReachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	ok, status := probeURL(srv.URL)
	if !ok || status != 200 {
		t.Fatalf("expected reachable 200, got ok=%v status=%d", ok, status)
	}
}

func TestProbeURL404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	ok, status := probeURL(srv.URL)
	if ok || status != 404 {
		t.Fatalf("expected unreachable 404, got ok=%v status=%d", ok, status)
	}
}

func TestProbeURLNetworkError(t *testing.T) {
	// 未监听的地址 → 网络错误，状态码 0
	ok, status := probeURL("http://127.0.0.1:1")
	if ok || status != 0 {
		t.Fatalf("expected network error (ok=false status=0), got ok=%v status=%d", ok, status)
	}
}

// TestCheckLinksAutoExpiredLabels Spec 56：检测后自动打/摘「失效」标签三规则：
// 失败且无标签 → 打标；恢复且标签为「失效」 → 摘除；已有其他标签 → 不覆盖。
func TestCheckLinksAutoExpiredLabels(t *testing.T) {
	r, st, user, _ := newFullEnv(t)
	cookie := authCookie(user)

	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer up.Close()
	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer down.Close()

	mk := func(name, url, label string) *store.Link {
		t.Helper()
		l := &store.Link{Name: name, URL: url, Label: label}
		if _, err := st.CreateLink(l); err != nil {
			t.Fatalf("create link %s: %v", name, err)
		}
		return l
	}
	dead := mk("dead", down.URL, "")
	alive := mk("alive", up.URL, expiredLabel) // 曾被打标，现已恢复
	labeled := mk("labeled", down.URL, "大佬")  // 已有标签，不应覆盖

	var body struct {
		Items    []linkCheckResult `json:"items"`
		Tagged   int               `json:"tagged"`
		Restored int               `json:"restored"`
	}
	run := func() {
		t.Helper()
		w := do2FA(r, http.MethodPost, "/api/v1/links/check", "", cookie)
		if w.Code != http.StatusOK {
			t.Fatalf("check: status = %d, body = %s", w.Code, w.Body.String())
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode check: %v", err)
		}
	}

	// 首轮：dead 打标，alive 摘标，labeled 不动
	run()
	if body.Tagged != 1 || body.Restored != 1 {
		t.Fatalf("round1: tagged = %d, restored = %d, want 1/1", body.Tagged, body.Restored)
	}
	got, _ := st.GetLink(dead.ID)
	if got.Label != expiredLabel || got.Color != expiredColor {
		t.Errorf("dead link should be tagged: %+v", got)
	}
	got, _ = st.GetLink(alive.ID)
	if got.Label != "" || got.Color != "" {
		t.Errorf("alive link should have label removed: %+v", got)
	}
	got, _ = st.GetLink(labeled.ID)
	if got.Label != "大佬" {
		t.Errorf("existing label should not be overwritten: %+v", got)
	}

	// 次轮幂等：无新增打/摘
	run()
	if body.Tagged != 0 || body.Restored != 0 {
		t.Fatalf("round2 should be idempotent, got tagged = %d, restored = %d", body.Tagged, body.Restored)
	}

	// dead 站点恢复后（URL 改为可达，保留已打的标签）再检测 → 摘标
	if err := st.UpdateLink(dead.ID, &store.Link{Name: "dead", URL: up.URL, Label: expiredLabel, Color: expiredColor}); err != nil {
		t.Fatalf("update dead link: %v", err)
	}
	run()
	if body.Restored != 1 {
		t.Fatalf("round3: restored = %d, want 1", body.Restored)
	}
	got, _ = st.GetLink(dead.ID)
	if got.Label != "" {
		t.Errorf("recovered link should have label removed: %+v", got)
	}
}
