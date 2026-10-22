package api

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"blog/internal/config"
	"blog/internal/stellarv2"
	"blog/internal/store"
)

func TestStellarSettingsLegacyReadValidateSaveGate(t *testing.T) {
	r, st, user, _ := newFullEnv(t)
	cookie := authCookie(user)
	if err := st.EnsureSettingsTable(); err != nil {
		t.Fatal(err)
	}
	if err := st.PutSetting("style", `{"prefers_theme":"dark"}`); err != nil {
		t.Fatal(err)
	}
	before, _ := st.ReadSettingsSnapshot()
	w := do2FA(r, http.MethodGet, "/api/v1/settings/stellar", "", cookie)
	var problem StellarSettingsProblem
	if err := json.Unmarshal(w.Body.Bytes(), &problem); err != nil || w.Code != http.StatusConflict || problem.Migration == nil || problem.Migration.Path != "stellar_v2" {
		t.Fatalf("legacy read diagnosis: %d %v", w.Code, err)
	}
	payload := `{"version":2,"theme":{},"blog_extensions":{}}`
	w = do2FA(r, http.MethodPost, "/api/v1/settings/stellar/validate", payload, cookie)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"valid":true`) {
		t.Fatalf("legacy candidate validation: %d %s", w.Code, w.Body.String())
	}
	w = do2FA(r, http.MethodPut, "/api/v1/settings/stellar", payload, cookie)
	if w.Code != http.StatusConflict {
		t.Fatalf("legacy save should require migration, got %d", w.Code)
	}
	after, _ := st.ReadSettingsSnapshot()
	if !reflect.DeepEqual(before, after) {
		t.Fatal("legacy read/validation/save gate wrote data")
	}
}

func TestStellarSettingsSaveReopenAndHotPublish(t *testing.T) {
	r, st, user, h := newFullEnv(t)
	cookie := authCookie(user)
	if err := st.EnsureSettingsTable(); err != nil {
		t.Fatal(err)
	}
	initial, err := config.InitializeStellarV2(st, map[string]any{}, map[string]any{
		"native_comments": map[string]any{"notify": map[string]any{"password": "smtp-private"}},
		"ai":              map[string]any{"api_key": "model-private"},
	})
	if err != nil {
		t.Fatal(err)
	}
	next := *h.site.Load()
	next.StellarV2 = initial
	h.site.Store(&next)
	w := do2FA(r, http.MethodGet, "/api/v1/settings/stellar", "", cookie)
	if w.Code != http.StatusOK || strings.Contains(w.Body.String(), "smtp-private") || strings.Contains(w.Body.String(), "model-private") {
		t.Fatal("GET failed or disclosed known secret")
	}
	payload := `{"version":2,"theme":{"appearance":{"color_scheme":"light"},"comments":{"giscus":{"large":9007199254740993,"false":false,"null":null,"zero":0,"array":[]}}},"blog_extensions":{"native_comments":{"notify":{"password":"****"}},"ai":{"api_key":"****"},"custom":{"password":"public-value","large":9007199254740993}}}`
	w = do2FA(r, http.MethodPut, "/api/v1/settings/stellar", payload, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("save: %d %s", w.Code, w.Body.String())
	}
	for _, marker := range []string{"9007199254740993", "public-value", `"****"`} {
		if !strings.Contains(w.Body.String(), marker) {
			t.Fatalf("save response lost raw value %s", marker)
		}
	}
	if strings.Contains(w.Body.String(), "smtp-private") || strings.Contains(w.Body.String(), "model-private") {
		t.Fatal("save response disclosed known secret")
	}
	if h.site.Load() == &next || h.site.Load().StellarV2 == initial || next.StellarV2 != initial {
		t.Fatal("snapshot was mutated rather than replaced")
	}
	if scheme, _ := stellarv2.Get(h.site.Load().StellarV2.Theme, "appearance.colorScheme"); scheme != "light" {
		t.Fatal("published configuration did not update")
	}
	w = do2FA(r, http.MethodGet, "/api/v1/settings/stellar", "", cookie)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "9007199254740993") {
		t.Fatal("reopened editor lost raw number")
	}
	dbPath := st.DBPath()
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := store.New(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = reopened.Close() }()
	loaded, err := config.LoadStellarV2(reopened)
	if err != nil || !reflect.DeepEqual(loaded, h.site.Load().StellarV2) {
		t.Fatalf("reopened database differs from published configuration: %v", err)
	}
	if loaded.Blog["ai"].(map[string]any)["api_key"] != "model-private" {
		t.Fatal("masked save discarded stored secret")
	}
}

func TestStellarSettingsValidationDiagnosticsAndBounds(t *testing.T) {
	r, st, user, _ := newFullEnv(t)
	cookie := authCookie(user)
	if err := st.EnsureSettingsTable(); err != nil {
		t.Fatal(err)
	}
	before, _ := st.ReadSettingsSnapshot()
	w := do2FA(r, http.MethodPost, "/api/v1/settings/stellar/validate", `{"version":2,"theme":{"style":{"prefers_theme":"dark"}},"blog_extensions":{}}`, cookie)
	var problem StellarSettingsProblem
	if err := json.Unmarshal(w.Body.Bytes(), &problem); err != nil || w.Code != http.StatusBadRequest || problem.Type != "stellar_schema_error" || len(problem.Issues) == 0 || problem.Issues[0].Source != "settings.stellar_v2.theme" {
		t.Fatalf("schema diagnosis: %d %v", w.Code, err)
	}
	for _, payload := range []string{`{`, `null`, `{"version":1,"theme":{},"blog_extensions":{}}`, `{"version":2,"theme":{},"blog_extensions":[]} `, `{"version":2,"theme":{},"blog_extensions":{},"extra":false}`, `{"version":2,"theme":{},"blog_extensions":{}} {}`} {
		w := do2FA(r, http.MethodPost, "/api/v1/settings/stellar/validate", payload, cookie)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("invalid document accepted: %d", w.Code)
		}
	}
	for _, payload := range []string{strings.Repeat("x", maxSettingsBodyBytes+1), `{"version":2,"theme":{},"blog_extensions":{}}` + strings.Repeat(" ", maxSettingsBodyBytes)} {
		w := do2FA(r, http.MethodPost, "/api/v1/settings/stellar/validate", payload, cookie)
		if w.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("request bound not enforced: %d", w.Code)
		}
	}
	after, _ := st.ReadSettingsSnapshot()
	if !reflect.DeepEqual(before, after) {
		t.Fatal("validation wrote settings")
	}
}

func TestStellarSettingsRoutesRequireAdmin(t *testing.T) {
	r, st, _, _ := newFullEnv(t)
	author, err := st.GetUserByUsername("alice")
	if err != nil {
		t.Fatal(err)
	}
	for _, route := range []struct{ method, path string }{{http.MethodGet, "/api/v1/settings/stellar"}, {http.MethodPut, "/api/v1/settings/stellar"}, {http.MethodPost, "/api/v1/settings/stellar/validate"}} {
		for _, cookie := range []string{"", authCookie(author)} {
			w := do2FA(r, route.method, route.path, `{"version":2,"theme":{},"blog_extensions":{}}`, cookie)
			if w.Code != http.StatusUnauthorized && w.Code != http.StatusForbidden {
				t.Fatalf("admin boundary missing: %s %d", route.path, w.Code)
			}
		}
	}
}
