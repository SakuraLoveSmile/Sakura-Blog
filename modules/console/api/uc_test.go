package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"blog/internal/store"
)

func TestUserCenterUsesPublicAuthorSessionAndOwnership(t *testing.T) {
	r, st, _, _ := newFullEnv(t)
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
	w := doUserReq(r, http.MethodGet, "/api/v1/uc/me", "", readerCookie)
	if w.Code != http.StatusOK || !json.Valid(w.Body.Bytes()) {
		t.Fatalf("UC me: status=%d body=%s", w.Code, w.Body.String())
	}
	w = doUserReq(r, http.MethodGet, "/api/v1/profile", "", readerCookie)
	if w.Code != http.StatusOK {
		t.Fatalf("shared profile API: status=%d body=%s", w.Code, w.Body.String())
	}
	w = doUserReq(r, http.MethodGet, "/api/v1/notifications", "", readerCookie)
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
	w = doUserReq(r, http.MethodGet, "/api/v1/uc/posts", "", readerCookie)
	if w.Code != http.StatusOK {
		t.Fatalf("UC posts: status=%d body=%s", w.Code, w.Body.String())
	}
	var posts UCPostListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &posts); err != nil || posts.Total != 1 || len(posts.Items) != 1 || posts.Items[0].Slug != "uc-alice" {
		t.Fatalf("UC posts = %+v, err=%v", posts, err)
	}

	w = doUserReq(r, http.MethodPost, "/api/v1/uc/posts", `{"slug":"uc-new","title":"UC 新文","content_md":"正文","status":"published","authors":["bob"],"pinned":true,"category":"site-setting"}`, readerCookie)
	if w.Code != http.StatusCreated {
		t.Fatalf("UC create: status=%d body=%s", w.Code, w.Body.String())
	}
	created, err := st.GetBySlug("uc-new")
	if err != nil || created == nil || len(created.AuthorIDs) != 1 || created.AuthorIDs[0] != alice.ID || created.Pinned || created.Category != "" {
		t.Fatalf("UC created ownership = %+v, err=%v", created, err)
	}

	w = doUserReq(r, http.MethodPut, "/api/v1/uc/posts/uc-bob", `{"title":"越权"}`, readerCookie)
	if w.Code != http.StatusForbidden {
		t.Fatalf("UC cross-owner update = %d body=%s, want 403", w.Code, w.Body.String())
	}
	w = doUserReq(r, http.MethodGet, "/api/v1/uc/attachments", "", readerCookie)
	if w.Code != http.StatusOK {
		t.Fatalf("UC attachments: status=%d body=%s", w.Code, w.Body.String())
	}
	var attachments UCAttachmentListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &attachments); err != nil || attachments.Total != 1 || len(attachments.Items) != 1 || attachments.Items[0].Filename != "alice.png" {
		t.Fatalf("UC attachments = %+v, err=%v", attachments, err)
	}
	w = doUserReq(r, http.MethodDelete, "/api/v1/uc/attachments/"+itoa(otherAttachment), "", readerCookie)
	if w.Code != http.StatusForbidden {
		t.Fatalf("UC cross-owner attachment delete = %d body=%s, want 403", w.Code, w.Body.String())
	}
	admin, err := st.GetUserByUsername("admin")
	if err != nil || admin == nil {
		t.Fatalf("get admin: %v", err)
	}
	adminRequest := httptest.NewRequest(http.MethodDelete, "/api/v1/uc/attachments/"+itoa(otherAttachment), nil)
	adminRequest.AddCookie(&http.Cookie{Name: authCookieName, Value: authCookie(admin)})
	adminResponse := httptest.NewRecorder()
	r.ServeHTTP(adminResponse, adminRequest)
	w = adminResponse
	if w.Code != http.StatusOK {
		t.Fatalf("UC admin attachment delete = %d body=%s, want 200", w.Code, w.Body.String())
	}
}
