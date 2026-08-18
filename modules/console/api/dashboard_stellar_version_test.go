package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	ghpkg "blog/internal/github"
	tmpl "blog/internal/settings"
)

func TestDashboardStellarVersionStates(t *testing.T) {
	tests := []struct {
		name         string
		tag          string
		status       int
		wantSync     bool
		wantError    bool
		wantUpstream string
	}{
		{name: "synced", tag: "v1.42.1", status: http.StatusOK, wantSync: true, wantUpstream: "1.42.1"},
		{name: "outdated", tag: "v1.42.2", status: http.StatusOK, wantUpstream: "1.42.2"},
		{name: "upstream failure", status: http.StatusInternalServerError, wantError: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/repos/xaoxuu/hexo-theme-stellar/releases/latest" {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
				if tc.status != http.StatusOK {
					w.WriteHeader(tc.status)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"tag_name":"` + tc.tag + `","html_url":"https://github.com/xaoxuu/hexo-theme-stellar/releases/latest"}`))
			}))
			defer upstream.Close()

			gh := ghpkg.New(ghpkg.Config{CacheDir: t.TempDir(), APIBase: upstream.URL})
			st := newTestStore(t)
			site := &tmpl.Site{}
			enableAdmin(t, st, site, "")
			r := gin.New()
			if _, err := MountConsole(r, MountDeps{Config: Config{
				Store: st, Site: sitePtr(site), StellarVersion: "1.42.1",
			}, GH: gh}); err != nil {
				t.Fatalf("mount: %v", err)
			}

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/stellar-version", nil)
			r.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
			}
			var got stellarVersionResponse
			if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if got.LocalVersion != "1.42.1" || got.InSync != tc.wantSync || got.UpstreamVersion != tc.wantUpstream {
				t.Fatalf("response = %+v", got)
			}
			if (got.Error != "") != tc.wantError {
				t.Fatalf("error = %q, want present=%v", got.Error, tc.wantError)
			}
			if got.CheckedAt == "" {
				t.Fatal("checked_at should be populated")
			}
		})
	}
}

func TestSemverEqual(t *testing.T) {
	for _, tc := range []struct {
		left, right string
		want        bool
	}{
		{"1.42.1", "v1.42.1", true},
		{"1.42.1", "1.42.2", false},
		{"1.42.1-rc.1", "1.42.1-rc.1", true},
		{"1.42.1-rc.1", "1.42.1", false},
		{"1.42", "1.42.0", false},
	} {
		if got := semverEqual(normalizeSemver(tc.left), normalizeSemver(tc.right)); got != tc.want {
			t.Errorf("semverEqual(%q, %q) = %v, want %v", tc.left, tc.right, got, tc.want)
		}
	}
}
