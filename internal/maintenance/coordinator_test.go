package maintenance

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestMaintenanceDrainsAndRejectsNewWork(t *testing.T) {
	c := New()
	release, err := c.Acquire()
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan *Window, 1)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	go func() {
		window, err := c.Begin(ctx, "restore")
		if err != nil {
			started <- nil
			return
		}
		started <- window
	}()
	for !c.Active() {
		select {
		case <-ctx.Done():
			t.Fatal("maintenance never began")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	if _, err := c.Acquire(); !errors.Is(err, ErrBusy) {
		t.Fatal("new work entered maintenance")
	}
	if _, err := c.Begin(ctx, "update"); !errors.Is(err, ErrBusy) {
		t.Fatal("conflicting maintenance entered")
	}
	select {
	case <-started:
		t.Fatal("maintenance did not drain")
	default:
	}
	release()
	release()
	window := <-started
	if window == nil {
		t.Fatal("failed to drain")
	}
	window.Close()
	end, err := c.Acquire()
	if err != nil {
		t.Fatal(err)
	}
	end()
}

func TestFailedCompensationKeepsHTTPUnavailable(t *testing.T) {
	c := New()
	window, err := c.Begin(context.Background(), "restore")
	if err != nil {
		t.Fatal(err)
	}
	window.Hold()
	window.Close()
	var calls atomic.Int32
	handler := c.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.WriteHeader(http.StatusOK) }))
	for _, path := range []string{"/api/v1/auth/login", "/api/v1/backup/restores/task", "/readyz", "/uploads/file.png"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != 503 {
			t.Errorf("%s status=%d", path, response.Code)
		}
	}
	if calls.Load() != 0 {
		t.Fatal("business/auth handler called during recovery")
	}
	for _, path := range []string{"/livez", "/api/maintenance"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != 200 {
			t.Errorf("%s status=%d", path, response.Code)
		}
	}
}

func TestSynchronousRestoreReleasesItsOwnRequestLease(t *testing.T) {
	c := New()
	handler := c.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ReleaseRequest(r.Context())
		ctx, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		window, err := c.Begin(ctx, "restore")
		if err != nil {
			t.Error(err)
			return
		}
		window.Close()
	}))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/restore", nil))
	if c.Active() {
		t.Fatal("maintenance not released")
	}
}
