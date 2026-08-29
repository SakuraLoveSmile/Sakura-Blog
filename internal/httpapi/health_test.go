package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

type healthPinger func(context.Context) error

func (p healthPinger) PingContext(ctx context.Context) error {
	return p(ctx)
}

func TestHealthRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterHealthRoutes(r, healthPinger(func(ctx context.Context) error {
		if _, ok := ctx.Deadline(); !ok {
			t.Error("readiness ping should receive a deadline")
		}
		return nil
	}))

	live := httptest.NewRecorder()
	r.ServeHTTP(live, httptest.NewRequest(http.MethodGet, "/livez", nil))
	if live.Code != http.StatusOK || live.Body.String() != `{"ok":true}` {
		t.Fatalf("livez = %d %q, want 200 {ok:true}", live.Code, live.Body.String())
	}

	ready := httptest.NewRecorder()
	r.ServeHTTP(ready, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if ready.Code != http.StatusOK || ready.Body.String() != `{"ok":true}` {
		t.Fatalf("readyz = %d %q, want 200 {ok:true}", ready.Code, ready.Body.String())
	}
}

func TestReadyzReturns503WhenStoreIsUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterHealthRoutes(r, healthPinger(func(context.Context) error {
		return errors.New("database unavailable")
	}))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if w.Code != http.StatusServiceUnavailable || w.Body.String() != `{"ok":false}` {
		t.Fatalf("readyz failure = %d %q, want 503 {ok:false}", w.Code, w.Body.String())
	}
}
