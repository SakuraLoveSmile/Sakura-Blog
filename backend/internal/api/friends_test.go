package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
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
