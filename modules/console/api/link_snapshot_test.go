package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"blog/internal/linkwatch"
	"blog/internal/store"
)

func TestSnapshotLinkAPI(t *testing.T) {
	r, st, user, h := newFullEnv(t)
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte("fake-jpeg"))
	}))
	defer provider.Close()
	site := h.site.Load()
	site.Friends.SnapshotProvider = provider.URL + "?url={url}"
	// 安全审计 2026-08-16：生产 watcher 的截图客户端走 safefetch（拒绝环回），
	// 测试对 127.0.0.1 httptest 服务验证截图落盘逻辑，注入普通 client。
	h.linkWatch = linkwatch.NewWithClients(st, h.site, filepath.Join(t.TempDir(), "uploads"),
		&http.Client{Timeout: 3 * time.Second}, &http.Client{Timeout: 3 * time.Second})
	link := &store.Link{Name: "example", URL: "https://example.com"}
	if _, err := st.CreateLink(link); err != nil {
		t.Fatal(err)
	}

	w := do2FA(r, http.MethodPost, "/api/v1/links/"+strconv.FormatInt(link.ID, 10)+"/snapshot", "", authCookie(user))
	if w.Code != http.StatusOK {
		t.Fatalf("snapshot status=%d body=%s", w.Code, w.Body.String())
	}
	var got linkJSON
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Snapshot != "/uploads/snapshots/1.jpg" {
		t.Fatalf("snapshot URL=%q", got.Snapshot)
	}
}

func TestUpdateLinkPreservesCheckState(t *testing.T) {
	r, st, user, _ := newFullEnv(t)
	link := &store.Link{Name: "example", URL: "https://example.com", LastStatus: http.StatusOK, LastCheckedAt: 123}
	if _, err := st.CreateLink(link); err != nil {
		t.Fatal(err)
	}

	body := `{"name":"updated","url":"https://updated.example.com"}`
	w := do2FA(r, http.MethodPut, "/api/v1/links/"+strconv.FormatInt(link.ID, 10), body, authCookie(user))
	if w.Code != http.StatusOK {
		t.Fatalf("update status=%d body=%s", w.Code, w.Body.String())
	}
	got, err := st.GetLink(link.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "updated" || got.LastStatus != http.StatusOK || got.LastCheckedAt != 123 {
		t.Fatalf("update did not preserve check state: %+v", got)
	}
}
