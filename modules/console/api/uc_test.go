package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"blog/internal/identity"
	"blog/internal/notify"
	"blog/internal/store"
)

func doUCSessionReq(r *gin.Engine, method, path, body, adminCookie, publicCookie, token string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if adminCookie != "" {
		req.AddCookie(&http.Cookie{Name: authCookieName, Value: adminCookie})
	}
	if publicCookie != "" {
		req.AddCookie(&http.Cookie{Name: identity.UserCookieName, Value: publicCookie})
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	r.ServeHTTP(w, req)
	return w
}

func registerUCReader(t *testing.T, r *gin.Engine, admin *store.User) string {
	t.Helper()
	w := do2FA(r, http.MethodPost, registrationInvitePath, `{}`, authCookie(admin))
	if w.Code != http.StatusCreated {
		t.Fatalf("generate invite: status=%d", w.Code)
	}
	var invite RegistrationInviteResponse
	if err := json.Unmarshal(w.Body.Bytes(), &invite); err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf(`{"username":"uc-reader","password":"reader-fixture-password","email":"uc-reader@example.com","invitation_code":%q}`, invite.Code)
	w = doUCSessionReq(r, http.MethodPost, "/api/auth/register", body, authCookie(admin), "", "")
	if w.Code != http.StatusCreated {
		t.Fatalf("register reader: status=%d", w.Code)
	}
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == identity.UserCookieName && cookie.Value != "" {
			return cookie.Value
		}
	}
	t.Fatal("registration did not establish public session")
	return ""
}

func TestUserCenterDualCookiesSelectPublicReaderAndPreserveAdmin(t *testing.T) {
	r, st, admin, _ := newFullEnv(t)
	publicCookie := registerUCReader(t, r, admin)
	adminBefore, err := st.GetUserByID(admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	w := doUCSessionReq(r, http.MethodGet, "/api/v1/uc/me", "", authCookie(admin), publicCookie, "")
	var me UCMeResponse
	if err := json.Unmarshal(w.Body.Bytes(), &me); err != nil || w.Code != http.StatusOK || me.Username != "uc-reader" || me.Role != store.RoleReader {
		t.Fatalf("UC dual-cookie me selected wrong user: status=%d me=%+v err=%v", w.Code, me, err)
	}
	w = doUCSessionReq(r, http.MethodGet, "/api/v1/uc/posts", "", authCookie(admin), publicCookie, "")
	if w.Code != http.StatusForbidden {
		t.Fatalf("dual-cookie reader inherited administrator content permissions: status=%d", w.Code)
	}
	w = doUCSessionReq(r, http.MethodGet, "/api/v1/uc/profile", "", authCookie(admin), publicCookie, "")
	var profile profileResponse
	if err := json.Unmarshal(w.Body.Bytes(), &profile); err != nil || w.Code != http.StatusOK || profile.ID != me.ID {
		t.Fatalf("UC profile selected wrong user: status=%d profile=%+v err=%v", w.Code, profile, err)
	}
	w = doUCSessionReq(r, http.MethodPut, "/api/v1/uc/profile", `{"display_name":"读者昵称","email":"changed-reader@example.com","avatar":"/uploads/reader.png"}`, authCookie(admin), publicCookie, "")
	if w.Code != http.StatusOK {
		t.Fatalf("UC profile update: status=%d", w.Code)
	}
	reader, err := st.GetUserByID(me.ID)
	if err != nil || reader.DisplayName != "读者昵称" || reader.Email != "changed-reader@example.com" || reader.Avatar != "/uploads/reader.png" {
		t.Fatalf("reader profile was not saved: %+v err=%v", reader, err)
	}
	// Console 仍采用后台 cookie；UC 写入不能改动同一浏览器里的管理员资料。
	w = doUCSessionReq(r, http.MethodGet, "/api/v1/profile", "", authCookie(admin), publicCookie, "")
	if err := json.Unmarshal(w.Body.Bytes(), &profile); err != nil || w.Code != http.StatusOK || profile.ID != admin.ID {
		t.Fatalf("Console dual-cookie profile changed identity: status=%d profile=%+v err=%v", w.Code, profile, err)
	}
	adminAfter, err := st.GetUserByID(admin.ID)
	if err != nil || adminAfter.DisplayName != adminBefore.DisplayName || adminAfter.Email != adminBefore.Email || adminAfter.Avatar != adminBefore.Avatar || adminAfter.AuthSecret != adminBefore.AuthSecret || adminAfter.PasswordHash != adminBefore.PasswordHash {
		t.Fatal("UC reader operations changed administrator account")
	}
	w = doUCSessionReq(r, http.MethodPut, "/api/v1/uc/auth/password", `{"current":"reader-fixture-password","new":"reader-new-password"}`, authCookie(admin), publicCookie, "")
	if w.Code != http.StatusOK {
		t.Fatalf("UC password update: status=%d", w.Code)
	}
	w = doUCSessionReq(r, http.MethodGet, "/api/v1/uc/me", "", authCookie(admin), publicCookie, "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("invalidated public cookie must not fall back to admin: status=%d", w.Code)
	}
	logout := doUCSessionReq(r, http.MethodPost, "/api/auth/logout", "", authCookie(admin), publicCookie, "")
	for _, cookie := range logout.Result().Cookies() {
		if cookie.Name == authCookieName {
			t.Fatal("public logout touched administrator cookie")
		}
	}
	w = do2FA(r, http.MethodGet, "/api/v1/profile", "", authCookie(admin))
	if w.Code != http.StatusOK {
		t.Fatalf("UC password/logout invalidated administrator session: status=%d", w.Code)
	}
}

func TestUserCenterRejectsPromotedOrBannedPublicAccount(t *testing.T) {
	r, st, admin, _ := newFullEnv(t)
	reader := seedReaderUser(t, st, "uc-role-change")
	cookie := signReaderCookie(reader)
	if err := st.UpdateUserRole(reader.ID, store.RoleAdmin); err != nil {
		t.Fatal(err)
	}
	w := doUCSessionReq(r, http.MethodGet, "/api/v1/uc/me", "", authCookie(admin), cookie, "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("old public cookie acquired promoted administrator identity: status=%d", w.Code)
	}
	if err := st.UpdateUserRole(reader.ID, store.RoleReader); err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateUserStatus(reader.ID, store.UserStatusBanned); err != nil {
		t.Fatal(err)
	}
	w = doUCSessionReq(r, http.MethodGet, "/api/v1/uc/me", "", authCookie(admin), cookie, "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("banned public session fell back to administrator: status=%d", w.Code)
	}
}

func TestUserCenterAccountActionsRemainBoundToPublicOwner(t *testing.T) {
	r, st, admin, h := newFullEnv(t)
	reader := seedReaderUser(t, st, "uc-action-reader")
	publicCookie := signReaderCookie(reader)
	request := func(method, path, body string) *httptest.ResponseRecorder {
		return doUCSessionReq(r, method, "/api/v1/uc"+path, body, authCookie(admin), publicCookie, "")
	}
	w := request(http.MethodPost, "/profile/api-token", "")
	var tokenResponse struct {
		Token    string `json:"token"`
		Username string `json:"username"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &tokenResponse); err != nil || w.Code != http.StatusOK || tokenResponse.Username != reader.Username || tokenResponse.Token == "" {
		t.Fatal("UC token generation selected incorrect owner or failed")
	}
	updatedReader, _ := st.GetUserByID(reader.ID)
	updatedAdmin, _ := st.GetUserByID(admin.ID)
	if updatedReader.APITokenHash != store.HashAPIToken(tokenResponse.Token) || updatedAdmin.APITokenHash != "" {
		t.Fatal("UC token changed incorrect account")
	}
	if request(http.MethodGet, "/profile/api-token", "").Code != http.StatusOK || request(http.MethodDelete, "/profile/api-token", "").Code != http.StatusOK {
		t.Fatal("UC token status or revocation failed")
	}
	updatedReader, _ = st.GetUserByID(reader.ID)
	if updatedReader.APITokenHash != "" {
		t.Fatal("UC token revocation did not revoke public owner's token")
	}
	setup := request(http.MethodPost, "/profile/2fa/setup", "")
	var twoFA struct {
		Secret        string   `json:"secret"`
		AuthURL       string   `json:"auth_url"`
		RecoveryCodes []string `json:"recovery_codes"`
	}
	if err := json.Unmarshal(setup.Body.Bytes(), &twoFA); err != nil || setup.Code != http.StatusOK || !strings.Contains(twoFA.AuthURL, reader.Username) {
		t.Fatal("UC 2FA setup selected incorrect owner or failed")
	}
	confirmation, _ := json.Marshal(map[string]any{"secret": twoFA.Secret, "code": totpCodeFor(t, twoFA.Secret), "recovery_codes": twoFA.RecoveryCodes})
	if request(http.MethodPost, "/profile/2fa/confirm", string(confirmation)).Code != http.StatusOK {
		t.Fatal("UC 2FA confirmation failed")
	}
	updatedReader, _ = st.GetUserByID(reader.ID)
	updatedAdmin, _ = st.GetUserByID(admin.ID)
	if updatedReader.TOTPSecret != twoFA.Secret || updatedAdmin.TOTPSecret != "" {
		t.Fatal("UC 2FA confirmation changed incorrect account")
	}
	if request(http.MethodGet, "/profile/2fa/recovery-codes", "").Code != http.StatusOK {
		t.Fatal("UC 2FA recovery codes unavailable")
	}
	disable, _ := json.Marshal(map[string]string{"code": totpCodeFor(t, twoFA.Secret)})
	if request(http.MethodPost, "/profile/2fa/disable", string(disable)).Code != http.StatusOK {
		t.Fatal("UC 2FA disable failed")
	}
	h.notify = notify.New(st)
	adminNotification := &store.Notification{UserID: admin.ID, Title: "管理员通知", Type: "comment_new", CreatedAt: time.Now()}
	readerNotification := &store.Notification{UserID: reader.ID, Title: "读者通知", Type: "comment_reply", CreatedAt: time.Now()}
	for _, notification := range []*store.Notification{adminNotification, readerNotification} {
		if err := st.CreateNotification(notification); err != nil {
			t.Fatal(err)
		}
	}
	w = request(http.MethodGet, "/notifications", "")
	var notifications struct {
		Items []*store.Notification `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &notifications); err != nil || w.Code != http.StatusOK || len(notifications.Items) != 1 || notifications.Items[0].UserID != reader.ID {
		t.Fatal("UC notifications selected incorrect owner or failed")
	}
	if request(http.MethodGet, "/notifications/unread-count", "").Code != http.StatusOK {
		t.Fatal("UC unread count failed")
	}
	if request(http.MethodPut, fmt.Sprintf("/notifications/%d/read", adminNotification.ID), "").Code != http.StatusNotFound {
		t.Fatal("UC reader could modify administrator notification")
	}
	if request(http.MethodPut, fmt.Sprintf("/notifications/%d/read", readerNotification.ID), "").Code != http.StatusOK || request(http.MethodPut, "/notifications/read-all", "").Code != http.StatusOK {
		t.Fatal("UC reader could not mark own notifications read")
	}
	if count, err := st.CountUnreadNotifications(admin.ID); err != nil || count != 1 {
		t.Fatal("UC actions modified administrator unread notifications")
	}
}

func TestUserCenterRejectsNonPublicAuthenticationOnEveryAccountRoute(t *testing.T) {
	r, st, admin, h := newFullEnv(t)
	token := store.GenerateAPIToken()
	if err := st.SetAPITokenHash(admin.ID, store.HashAPIToken(token)); err != nil {
		t.Fatal(err)
	}
	for _, route := range []struct{ method, path string }{
		{http.MethodGet, "/me"}, {http.MethodGet, "/profile"}, {http.MethodPut, "/profile"},
		{http.MethodPut, "/auth/password"}, {http.MethodPost, "/profile/2fa/setup"},
		{http.MethodPost, "/profile/2fa/confirm"}, {http.MethodPost, "/profile/2fa/disable"},
		{http.MethodGet, "/profile/2fa/recovery-codes"}, {http.MethodGet, "/profile/api-token"},
		{http.MethodPost, "/profile/api-token"}, {http.MethodDelete, "/profile/api-token"},
		{http.MethodGet, "/notifications"}, {http.MethodGet, "/notifications/unread-count"},
		{http.MethodPut, "/notifications/1/read"}, {http.MethodPut, "/notifications/read-all"},
		{http.MethodGet, "/posts"}, {http.MethodGet, "/attachments"},
	} {
		for _, session := range []struct{ name, cookie, bearer string }{
			{"anonymous", "", ""}, {"admin-cookie", authCookie(admin), ""}, {"bearer", "", token}, {"both-admin", authCookie(admin), token},
		} {
			t.Run(session.name+route.method+route.path, func(t *testing.T) {
				w := doUCSessionReq(r, route.method, "/api/v1/uc"+route.path, `{}`, session.cookie, "", session.bearer)
				if w.Code != http.StatusUnauthorized {
					t.Fatalf("UC accepted non-public authentication: status=%d, want 401", w.Code)
				}
			})
		}
	}
	h.site.Load().Admin.AuthEnabled = false
	w := doUCSessionReq(r, http.MethodGet, "/api/v1/uc/me", "", "", "", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("UC accepted passwordless admin fallback: status=%d", w.Code)
	}
}

func TestUserCenterUsesPublicAuthorSessionAndOwnership(t *testing.T) {
	r, st, admin, _ := newFullStellarEnv(t)
	alice, err := st.GetUserByUsername("alice")
	if err != nil || alice == nil {
		t.Fatalf("get alice: %v", err)
	}
	bob, err := st.GetUserByUsername("bob")
	if err != nil || bob == nil {
		t.Fatalf("get bob: %v", err)
	}
	if _, err := st.Create(&store.Post{Slug: "uc-alice", Title: "Alice", ContentMD: "# Alice", Status: store.StatusPublished, AuthorIDs: []int64{alice.ID}}); err != nil {
		t.Fatalf("create alice post: %v", err)
	}
	if _, err := st.Create(&store.Post{Slug: "uc-bob", Title: "Bob", ContentMD: "# Bob", Status: store.StatusPublished, AuthorIDs: []int64{bob.ID}}); err != nil {
		t.Fatalf("create bob post: %v", err)
	}
	if _, err := st.CreateAttachment(&store.Attachment{Filename: "alice.png", Path: "alice.png", URL: "/uploads/alice.png", Mime: "image/png", OwnerID: alice.ID}); err != nil {
		t.Fatalf("create alice attachment: %v", err)
	}
	otherAttachment, err := st.CreateAttachment(&store.Attachment{Filename: "bob.png", Path: "bob.png", URL: "/uploads/bob.png", Mime: "image/png", OwnerID: bob.ID})
	if err != nil {
		t.Fatalf("create bob attachment: %v", err)
	}

	readerCookie := signReaderCookie(alice)
	w := doUCSessionReq(r, http.MethodGet, "/api/v1/uc/me", "", authCookie(admin), readerCookie, "")
	if w.Code != http.StatusOK || !json.Valid(w.Body.Bytes()) {
		t.Fatalf("UC me: status=%d body=%s", w.Code, w.Body.String())
	}
	w = doUserReq(r, http.MethodGet, "/api/v1/uc/profile", "", readerCookie)
	if w.Code != http.StatusOK {
		t.Fatalf("shared profile API: status=%d body=%s", w.Code, w.Body.String())
	}
	w = doUserReq(r, http.MethodGet, "/api/v1/uc/notifications", "", readerCookie)
	if w.Code != http.StatusOK {
		t.Fatalf("shared notifications API: status=%d body=%s", w.Code, w.Body.String())
	}
	w = doUserReq(r, http.MethodGet, "/api/v1/auth/check", "", readerCookie)
	var consoleCheck struct {
		Authenticated bool   `json:"authenticated"`
		Role          string `json:"role"`
		Disallowed    bool   `json:"console_access_disallowed"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &consoleCheck); err != nil || !consoleCheck.Authenticated || consoleCheck.Role != store.RoleAuthor || !consoleCheck.Disallowed {
		t.Fatalf("author Console check = %+v, err=%v body=%s", consoleCheck, err, w.Body.String())
	}
	w = doUCSessionReq(r, http.MethodGet, "/api/v1/uc/posts", "", authCookie(admin), readerCookie, "")
	if w.Code != http.StatusOK {
		t.Fatalf("UC posts: status=%d body=%s", w.Code, w.Body.String())
	}
	var posts UCPostListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &posts); err != nil || posts.Total != 1 || len(posts.Items) != 1 || posts.Items[0].Slug != "uc-alice" {
		t.Fatalf("UC posts = %+v, err=%v", posts, err)
	}

	w = doUCSessionReq(r, http.MethodPost, "/api/v1/uc/posts", `{"stellar_version":2,"stellar_config":"{}","blog_extensions":"{}","slug":"uc-new","title":"UC 新文","content_md":"正文","status":"published","authors":["bob"],"pinned":true,"category":"site-setting"}`, authCookie(admin), readerCookie, "")
	if w.Code != http.StatusCreated {
		t.Fatalf("UC create: status=%d body=%s", w.Code, w.Body.String())
	}
	created, err := st.GetBySlug("uc-new")
	if err != nil || created == nil || len(created.AuthorIDs) != 1 || created.AuthorIDs[0] != alice.ID || created.Pinned || created.Category != "" {
		t.Fatalf("UC created ownership = %+v, err=%v", created, err)
	}

	w = doUCSessionReq(r, http.MethodPut, "/api/v1/uc/posts/uc-bob", `{"stellar_version":2,"stellar_config":"{}","blog_extensions":"{}","title":"越权"}`, authCookie(admin), readerCookie, "")
	if w.Code != http.StatusForbidden {
		t.Fatalf("UC cross-owner update = %d body=%s, want 403", w.Code, w.Body.String())
	}
	w = doUCSessionReq(r, http.MethodGet, "/api/v1/uc/attachments", "", authCookie(admin), readerCookie, "")
	if w.Code != http.StatusOK {
		t.Fatalf("UC attachments: status=%d body=%s", w.Code, w.Body.String())
	}
	var attachments UCAttachmentListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &attachments); err != nil || attachments.Total != 1 || len(attachments.Items) != 1 || attachments.Items[0].Filename != "alice.png" {
		t.Fatalf("UC attachments = %+v, err=%v", attachments, err)
	}
	w = doUCSessionReq(r, http.MethodDelete, "/api/v1/uc/attachments/"+itoa(otherAttachment), "", authCookie(admin), readerCookie, "")
	if w.Code != http.StatusForbidden {
		t.Fatalf("UC cross-owner attachment delete = %d body=%s, want 403", w.Code, w.Body.String())
	}
	adminRequest := httptest.NewRequest(http.MethodDelete, "/api/v1/uc/attachments/"+itoa(otherAttachment), nil)
	adminRequest.AddCookie(&http.Cookie{Name: authCookieName, Value: authCookie(admin)})
	adminResponse := httptest.NewRecorder()
	r.ServeHTTP(adminResponse, adminRequest)
	w = adminResponse
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("UC admin-only attachment delete = %d body=%s, want 401", w.Code, w.Body.String())
	}
	if attachment, err := st.GetAttachment(otherAttachment); err != nil || attachment == nil {
		t.Fatalf("rejected UC admin-only request removed attachment: err=%v", err)
	}
}
