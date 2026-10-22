package api

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"blog/internal/config"
	"blog/internal/rag"
	tmpl "blog/internal/settings"
)

func TestRAGConsoleRoutesAreAdminOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st := newTestStore(t)
	if err := st.EnsureSettingsTable(); err != nil {
		t.Fatal(err)
	}
	site := &tmpl.Site{}
	enableAdmin(t, st, site, "secret")
	service, err := rag.New(st, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close(context.Background())
	r := gin.New()
	if _, err := MountConsole(r, MountDeps{Config: Config{Store: st, Site: sitePtr(site), RAG: service}}); err != nil {
		t.Fatal(err)
	}

	if response := do2FA(r, http.MethodGet, "/api/v1/rag/status", "", ""); response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status code = %d", response.Code)
	}
	admin, err := st.GetUserByUsername("admin")
	if err != nil || admin == nil {
		t.Fatalf("admin user: %v", err)
	}
	response := do2FA(r, http.MethodGet, "/api/v1/rag/status", "", authCookie(admin))
	if response.Code != http.StatusOK {
		t.Fatalf("authenticated status code = %d body=%s", response.Code, response.Body.String())
	}
	response = do2FA(r, http.MethodPost, "/api/v1/rag/rebuild", "{}", authCookie(admin))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("unconfigured rebuild code = %d body=%s", response.Code, response.Body.String())
	}
}

func TestRAGPublicGateQuestionLimitAndBYOKPolicy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st := newTestStore(t)
	if err := st.EnsureSettingsTable(); err != nil {
		t.Fatal(err)
	}
	if _, err := config.InitializeStellarV2(st, map[string]any{}, map[string]any{}); err != nil {
		t.Fatal(err)
	}
	service, err := rag.New(st, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close(context.Background())
	r := gin.New()
	if _, err := MountConsole(r, MountDeps{Config: Config{Store: st, Site: sitePtr(&tmpl.Site{}), RAG: service}}); err != nil {
		t.Fatal(err)
	}

	response := do2FA(r, http.MethodPost, "/api/v1/rag/ask/public", `{"question":"hello"}`, "")
	if response.Code != http.StatusForbidden {
		t.Fatalf("QA disabled code = %d body=%s", response.Code, response.Body.String())
	}
	if err := func() error {
		_, err := config.SaveStellarV2(st, map[string]any{}, map[string]any{"ai": map[string]any{"ask": map[string]any{"enabled": true, "allow_user_llm": false}}})
		return err
	}(); err != nil {
		t.Fatal(err)
	}
	config.InvalidateLoadAI()
	response = do2FA(r, http.MethodPost, "/api/v1/rag/ask/public", `{"question":"`+strings.Repeat("问", 1001)+`"}`, "")
	if response.Code != http.StatusBadRequest {
		t.Fatalf("question limit code = %d body=%s", response.Code, response.Body.String())
	}
	response = do2FA(r, http.MethodPost, "/api/v1/rag/ask/public", `{"question":"hello","endpoint":"https://api.example.com/v1","model":"m","api_key":"key"}`, "")
	if response.Code != http.StatusForbidden {
		t.Fatalf("BYOK disabled code = %d body=%s", response.Code, response.Body.String())
	}
}
