package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	tmpl "blog/internal/settings"
)

const registrationInvitePath = "/api/v1/users/invitations"

// 与生产一致，仅通过 Store.New 和 MountConsole 启动，不提前调用 EnsureSchema。
func TestRegistrationInvitesFreshStartupCreatesAndRegisters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st := newTestStore(t)
	site := &tmpl.Site{}
	enableAdmin(t, st, site, "admin-fixture-password")
	admin, err := st.GetUserByUsername("admin")
	if err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	mountPublicIdentityForTest(r, st, site)
	if _, err := MountConsole(r, MountDeps{Config: Config{Store: st, Site: sitePtr(site)}}); err != nil {
		t.Fatalf("mount console: %v", err)
	}
	w := do2FA(r, http.MethodPost, registrationInvitePath, `{}`, authCookie(admin))
	if w.Code != http.StatusCreated {
		t.Fatalf("fresh startup create status = %d, want 201; body = %s", w.Code, w.Body.String())
	}
	var invite RegistrationInviteResponse
	if err := json.Unmarshal(w.Body.Bytes(), &invite); err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf(`{"username":"startup-reader","password":"fixture-reader-password","email":"startup@example.com","invitation_code":%q}`, invite.Code)
	registered := doUserReq(r, http.MethodPost, "/api/auth/register", body, "")
	if registered.Code != http.StatusCreated {
		t.Fatalf("fresh startup register status = %d, want 201", registered.Code)
	}
}

func TestRegistrationInvitesAdminOnly(t *testing.T) {
	r, st, admin, _ := newFullEnv(t)
	reader := seedReaderUser(t, st, "invite-reader")
	author, err := st.GetUserByUsername("alice")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		cookie string
		status int
	}{
		{"anonymous", "", http.StatusUnauthorized},
		{"author", authCookie(author), http.StatusForbidden},
		{"reader", authCookie(reader), http.StatusForbidden},
		{"admin", authCookie(admin), http.StatusCreated},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := do2FA(r, http.MethodPost, registrationInvitePath, `{}`, tc.cookie)
			if w.Code != tc.status {
				t.Fatalf("status = %d, want %d; body = %s", w.Code, tc.status, w.Body.String())
			}
			if tc.status != http.StatusCreated && strings.Contains(w.Body.String(), `"code"`) {
				t.Fatal("rejected request exposed an invitation code")
			}
		})
	}
	// 公开读者会话也不能通过 Console 的公共身份回退获得系统权限。
	w := doUserReq(r, http.MethodPost, registrationInvitePath, `{}`, signReaderCookie(reader))
	if w.Code != http.StatusForbidden {
		t.Fatalf("reader public cookie status = %d, want 403", w.Code)
	}
	var count int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM registration_invites`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("only administrator should create an invitation; persisted count = %d", count)
	}
}

func TestRegistrationInvitesExpirationAndResponse(t *testing.T) {
	r, st, admin, _ := newFullEnv(t)
	cookie := authCookie(admin)
	codes := make(map[string]bool)
	for _, tc := range []struct {
		name string
		body string
		days int
	}{
		{"omitted", `{}`, 7},
		{"zero", `{"expires_in_days":0}`, 7},
		{"one", `{"expires_in_days":1}`, 1},
		{"seven", `{"expires_in_days":7}`, 7},
		{"thirty", `{"expires_in_days":30}`, 30},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := time.Now()
			w := do2FA(r, http.MethodPost, registrationInvitePath, tc.body, cookie)
			after := time.Now()
			if w.Code != http.StatusCreated {
				t.Fatalf("status = %d, want 201; body = %s", w.Code, w.Body.String())
			}
			if w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("invitation response must not be cached")
			}
			var response RegistrationInviteResponse
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if len(response.Code) < 20 || codes[response.Code] {
				t.Fatal("invitation code is too short or reused")
			}
			codes[response.Code] = true
			expires, err := time.Parse(time.RFC3339, response.ExpiresAt)
			if err != nil {
				t.Fatalf("expiration must be RFC3339: %v", err)
			}
			duration := time.Duration(tc.days) * 24 * time.Hour
			if expires.Before(before.Add(duration).Truncate(time.Second)) || expires.After(after.Add(duration)) {
				t.Fatalf("expiration = %s, want %d days from creation", response.ExpiresAt, tc.days)
			}
			// 用户列表是后续读取入口，不得附带任何已生成的邀请码。
			list := do2FA(r, http.MethodGet, "/api/v1/users", "", cookie)
			if list.Code != http.StatusOK || strings.Contains(list.Body.String(), response.Code) {
				t.Fatal("user listing failed or exposed invitation code")
			}
		})
	}
	rows, err := st.DB().Query(`SELECT code_hash, created_by FROM registration_invites`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var persistedHash string
		var issuer int64
		if err := rows.Scan(&persistedHash, &issuer); err != nil {
			t.Fatal(err)
		}
		if persistedHash == "" || codes[persistedHash] || issuer != admin.ID {
			t.Fatal("invitation must persist its digest and current administrator ID")
		}
		count++
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if count != len(codes) {
		t.Fatalf("persisted invitation count = %d, want %d", count, len(codes))
	}
}

func TestRegistrationInvitesRejectInvalidExpiration(t *testing.T) {
	r, st, admin, _ := newFullEnv(t)
	cookie := authCookie(admin)
	for _, body := range []string{
		`{"expires_in_days":-1}`, `{"expires_in_days":2}`, `{"expires_in_days":8}`, `{"expires_in_days":31}`,
		`{"expires_in_days":2147483647}`, `{"expires_in_days":"7"}`, `{"expires_in_days":1.5}`, `{`, "",
	} {
		w := do2FA(r, http.MethodPost, registrationInvitePath, body, cookie)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("body %q: status = %d, want 400", body, w.Code)
		}
		if w.Header().Get("Cache-Control") != "no-store" || strings.Contains(w.Body.String(), `"code"`) {
			t.Fatal("rejected creation must not expose or cache a code")
		}
	}
	var count int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM registration_invites`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("invalid requests persisted %d invitations", count)
	}
}

func TestRegistrationInvitesStorageFailureDoesNotLeakDetails(t *testing.T) {
	r, st, admin, _ := newFullEnv(t)
	if _, err := st.DB().Exec(`DROP TABLE registration_invites`); err != nil {
		t.Fatal(err)
	}
	w := do2FA(r, http.MethodPost, registrationInvitePath, `{}`, authCookie(admin))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}
	var problem Problem
	if err := json.Unmarshal(w.Body.Bytes(), &problem); err != nil {
		t.Fatal(err)
	}
	if problem.Detail != "创建邀请码失败" || problem.Error != problem.Detail {
		t.Fatalf("unexpected public error: %s", w.Body.String())
	}
	if w.Header().Get("Cache-Control") != "no-store" || strings.Contains(w.Body.String(), "registration_invites") || strings.Contains(w.Body.String(), `"code"`) {
		t.Fatal("storage failure exposed details or a code")
	}
	// 失败不会在 handler 内隐式创建或修复数据库结构。
	var name string
	err := st.DB().QueryRow(`SELECT name FROM sqlite_master WHERE name = 'registration_invites'`).Scan(&name)
	if err == nil {
		t.Fatalf("handler recreated invitation table %q", name)
	}
}
