package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	tmpl "blog/internal/settings"
	"blog/internal/store"
)

const validDashboardLayoutBody = `{
  "version": 10,
  "layouts": {
    "lg": [{"i":"quick-1","id":"quick-actions","x":0,"y":0,"w":4,"h":20,"config":{"label":"one"}}],
    "md": [],
    "sm": [],
    "xs": [],
    "xxs": []
  }
}`

func TestDashboardLayoutFirstReadAndUserIsolation(t *testing.T) {
	r, st, user, _ := authEnhanceEnv(t)
	second := &store.User{
		Username:     "author",
		PasswordHash: "x",
		AuthSecret:   store.GenerateAuthSecret(),
		Role:         store.RoleAuthor,
	}
	if err := st.CreateUser(second); err != nil {
		t.Fatalf("create second user: %v", err)
	}

	firstCookie := authCookie(user)
	secondCookie := authCookie(second)
	w := do2FA(r, http.MethodGet, "/api/v1/dashboard/layout", "", firstCookie)
	if w.Code != http.StatusOK {
		t.Fatalf("first read: status=%d body=%s", w.Code, w.Body.String())
	}
	var first dashboardLayoutResponse
	if err := json.Unmarshal(w.Body.Bytes(), &first); err != nil {
		t.Fatalf("decode first read: %v", err)
	}
	if first.Version != dashboardLayoutVersion || first.Layouts != nil || w.Body.String() == "" {
		t.Fatalf("first read = %+v body=%s, want layouts=null", first, w.Body.String())
	}
	if want := `"layouts":null`; !json.Valid(w.Body.Bytes()) || !strings.Contains(w.Body.String(), want) {
		t.Fatalf("first read body=%s, want %s", w.Body.String(), want)
	}

	w = do2FA(r, http.MethodPut, "/api/v1/dashboard/layout", validDashboardLayoutBody, firstCookie)
	if w.Code != http.StatusOK {
		t.Fatalf("save user 1: status=%d body=%s", w.Code, w.Body.String())
	}
	w = do2FA(r, http.MethodGet, "/api/v1/dashboard/layout", "", firstCookie)
	if w.Code != http.StatusOK {
		t.Fatalf("read user 1: status=%d body=%s", w.Code, w.Body.String())
	}
	var saved dashboardLayoutResponse
	if err := json.Unmarshal(w.Body.Bytes(), &saved); err != nil {
		t.Fatalf("decode user 1: %v", err)
	}
	if saved.Layouts == nil || len((*saved.Layouts)["lg"]) != 1 || (*saved.Layouts)["lg"][0].Config["label"] != "one" {
		t.Fatalf("saved user 1 layout = %+v", saved)
	}
	if saved.UpdatedAt == "" {
		t.Fatal("saved layout should include updated_at")
	}

	w = do2FA(r, http.MethodGet, "/api/v1/dashboard/layout", "", secondCookie)
	if w.Code != http.StatusOK {
		t.Fatalf("read user 2: status=%d body=%s", w.Code, w.Body.String())
	}
	var isolated dashboardLayoutResponse
	if err := json.Unmarshal(w.Body.Bytes(), &isolated); err != nil {
		t.Fatalf("decode user 2: %v", err)
	}
	if isolated.Layouts != nil {
		t.Fatalf("user 2 should not see user 1 layout: %+v", isolated)
	}
}

func TestDashboardLayoutUserZeroCompatibilityBucket(t *testing.T) {
	st := newTestStore(t)
	site := &tmpl.Site{}
	enableAdmin(t, st, site, "")
	r := gin.New()
	if _, err := MountConsole(r, MountDeps{Config: Config{Store: st, Site: sitePtr(site)}}); err != nil {
		t.Fatalf("mount: %v", err)
	}

	w := do2FA(r, http.MethodPut, "/api/v1/dashboard/layout", validDashboardLayoutBody, "")
	if w.Code != http.StatusOK {
		t.Fatalf("save user 0: status=%d body=%s", w.Code, w.Body.String())
	}
	w = do2FA(r, http.MethodGet, "/api/v1/dashboard/layout", "", "")
	if w.Code != http.StatusOK {
		t.Fatalf("read user 0: status=%d body=%s", w.Code, w.Body.String())
	}
	var got dashboardLayoutResponse
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode user 0: %v", err)
	}
	if got.Layouts == nil || len((*got.Layouts)["lg"]) != 1 {
		t.Fatalf("user 0 layout = %+v", got)
	}
}

func TestDashboardLayoutRejectsInvalidDocuments(t *testing.T) {
	r, _, user, _ := authEnhanceEnv(t)
	cookie := authCookie(user)
	cases := []struct {
		name string
		body string
	}{
		{name: "version", body: `{"version":9,"layouts":{}}`},
		{name: "unknown breakpoint", body: `{"version":10,"layouts":{"tv":[]}}`},
		{name: "duplicate instance", body: `{"version":10,"layouts":{"lg":[{"i":"same","id":"quick-actions","x":0,"y":0,"w":4,"h":20},{"i":"same","id":"quick-actions","x":4,"y":0,"w":4,"h":20}]}}`},
		{name: "out of bounds", body: `{"version":10,"layouts":{"lg":[{"i":"wide","id":"quick-actions","x":11,"y":0,"w":2,"h":20}]}}`},
		{name: "invalid constraint", body: `{"version":10,"layouts":{"lg":[{"i":"bad-min","id":"quick-actions","x":0,"y":0,"w":4,"h":20,"minW":5}]}}`},
		{name: "unknown field", body: `{"version":10,"layouts":{},"unexpected":true}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := do2FA(r, http.MethodPut, "/api/v1/dashboard/layout", tc.body, cookie)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("status=%d body=%s, want 400", w.Code, w.Body.String())
			}
		})
	}
}
