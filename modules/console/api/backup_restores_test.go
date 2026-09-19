package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAsyncRestoreRequiresIdempotencyKey(t *testing.T) {
	r, _, user, _ := newFullEnv(t)
	cookie := authCookie(user)
	w := do2FA(r, http.MethodPost, "/api/v1/backup/restores", `{"snapshot":"manual.db"}`, cookie)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("missing idempotency key: status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestAsyncRestoreMissingTaskIsNotFound(t *testing.T) {
	r, _, user, _ := newFullEnv(t)
	cookie := authCookie(user)
	w := do2FA(r, http.MethodGet, "/api/v1/backup/restores/missing-task", "", cookie)
	if w.Code != http.StatusNotFound {
		t.Fatalf("missing restore task: status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestAsyncRestoreRejectsInvalidJSONBeforeTaskCreation(t *testing.T) {
	r, _, user, _ := newFullEnv(t)
	for _, test := range []struct {
		name, body string
		status     int
	}{
		{"oversized", `{"snapshot":"` + strings.Repeat("x", 4096) + `"}`, http.StatusRequestEntityTooLarge},
		{"multiple objects", `{"snapshot":"manual.db"}{}`, http.StatusBadRequest},
		{"ambiguous source", `{"snapshot":"manual.db","file":"other.zip"}`, http.StatusBadRequest},
		{"empty source", `{}`, http.StatusBadRequest},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/v1/backup/restores", strings.NewReader(test.body))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Idempotency-Key", "invalid-json-test")
			request.AddCookie(&http.Cookie{Name: authCookieName, Value: authCookie(user)})
			response := httptest.NewRecorder()
			r.ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
	response := do2FA(r, http.MethodGet, "/api/v1/backup/restores", "", authCookie(user))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"tasks":[]`) {
		t.Fatalf("invalid requests created tasks: %s", response.Body.String())
	}
}
