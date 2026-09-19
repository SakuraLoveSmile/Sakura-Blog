package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	tmpl "blog/internal/settings"
	"blog/internal/store"
)

func newAuthorEnv(t *testing.T) (*gin.Engine, *store.Store, *store.User, *store.User, *store.User) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	st := newTestStore(t)
	if err := st.EnsureUsersTable(); err != nil {
		t.Fatalf("ensure users table: %v", err)
	}
	if err := st.EnsurePostAuthorsTable(); err != nil {
		t.Fatalf("ensure post authors table: %v", err)
	}
	if err := st.EnsureAttachmentsTable(); err != nil {
		t.Fatalf("ensure attachments table: %v", err)
	}
	if err := st.EnsureRevisionsTable(); err != nil {
		t.Fatalf("ensure revisions table: %v", err)
	}
	if err := st.EnsurePagesTable(); err != nil {
		t.Fatalf("ensure pages table: %v", err)
	}
	if err := st.EnsureMomentsTable(); err != nil {
		t.Fatalf("ensure moments table: %v", err)
	}

	site := &tmpl.Site{}
	enableAdmin(t, st, site, "adminpass")
	admin, err := st.GetUserByUsername("admin")
	if err != nil || admin == nil {
		t.Fatalf("get admin: %v", err)
	}

	createAuthor := func(username, pass string) *store.User {
		hash, err := store.HashPassword(pass)
		if err != nil {
			t.Fatalf("hash: %v", err)
		}
		u := &store.User{
			Username:     username,
			PasswordHash: hash,
			AuthSecret:   store.GenerateAuthSecret(),
			Role:         store.RoleAuthor,
			Permissions:  []string{store.PermissionContentWrite, store.PermissionConsoleAccess},
			Status:       store.UserStatusActive,
		}
		if err := st.CreateUser(u); err != nil {
			t.Fatalf("create author %s: %v", username, err)
		}
		return u
	}

	authorA := createAuthor("authorA", "passA")
	authorB := createAuthor("authorB", "passB")

	r := gin.New()
	mountPublicIdentityForTest(r, st, site)
	if _, err := MountConsole(r, MountDeps{Config: Config{Store: st, Site: sitePtr(site)}}); err != nil {
		t.Fatalf("mount console: %v", err)
	}

	return r, st, admin, authorA, authorB
}

func authorCookie(u *store.User) string {
	expiry := time.Now().Add(time.Hour).Unix()
	return signAuthCookie(cookieAudAdmin, expiry, u.ID, u.AuthSecret)
}

func TestAuthorPostOwnershipIsolation(t *testing.T) {
	r, st, admin, authorA, authorB := newAuthorEnv(t)

	// Create post for authorA
	postA := &store.Post{
		Slug:      "post-a",
		Title:     "Post A Title",
		ContentMD: "![a](/uploads/2026/09/a.png)",
		Status:    store.StatusPublished,
		Author:    authorA.Username,
		Authors:   []string{authorA.Username},
		AuthorIDs: []int64{authorA.ID},
		DraftJSON: `{"title":"Draft of A"}`,
	}
	if _, err := st.Create(postA); err != nil {
		t.Fatalf("create postA: %v", err)
	}

	// Create post for authorB
	postB := &store.Post{
		Slug:      "post-b",
		Title:     "Post B Title",
		ContentMD: "![a](/uploads/2026/09/a.png)",
		Status:    store.StatusPublished,
		Author:    authorB.Username,
		Authors:   []string{authorB.Username},
		AuthorIDs: []int64{authorB.ID},
		DraftJSON: `{"title":"Draft of B"}`,
	}
	if _, err := st.Create(postB); err != nil {
		t.Fatalf("create postB: %v", err)
	}

	// Create co-authored post (authorA and authorB)
	postAB := &store.Post{
		Slug:      "post-ab",
		Title:     "Co-authored Post",
		ContentMD: "Joint Content",
		Status:    store.StatusPublished,
		Author:    authorA.Username,
		Authors:   []string{authorA.Username, authorB.Username},
		AuthorIDs: []int64{authorA.ID, authorB.ID},
	}
	if _, err := st.Create(postAB); err != nil {
		t.Fatalf("create postAB: %v", err)
	}

	// 1. GET /api/v1/posts (List)
	// Author A should only see post-a and post-ab
	reqA := httptest.NewRequest(http.MethodGet, "/api/v1/posts", nil)
	reqA.AddCookie(&http.Cookie{Name: authCookieName, Value: authorCookie(authorA)})
	wA := httptest.NewRecorder()
	r.ServeHTTP(wA, reqA)
	if wA.Code != http.StatusOK {
		t.Fatalf("authorA list posts status = %d: %s", wA.Code, wA.Body.String())
	}
	var listResp struct {
		Items []struct {
			Slug string `json:"slug"`
		} `json:"items"`
		Total int `json:"total"`
	}
	if err := json.Unmarshal(wA.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("unmarshal list posts: %v", err)
	}
	if listResp.Total != 2 {
		t.Fatalf("authorA expected 2 posts, got %d", listResp.Total)
	}
	slugs := map[string]bool{}
	for _, it := range listResp.Items {
		slugs[it.Slug] = true
	}
	if !slugs["post-a"] || !slugs["post-ab"] || slugs["post-b"] {
		t.Fatalf("authorA got wrong posts: %+v", slugs)
	}

	// 2. GET /api/v1/posts/:slug (Detail)
	// Author A reading Author B's post must get 403 Forbidden
	reqDetail := httptest.NewRequest(http.MethodGet, "/api/v1/posts/post-b", nil)
	reqDetail.AddCookie(&http.Cookie{Name: authCookieName, Value: authorCookie(authorA)})
	wDetail := httptest.NewRecorder()
	r.ServeHTTP(wDetail, reqDetail)
	if wDetail.Code != http.StatusForbidden {
		t.Fatalf("authorA accessing post-b status = %d, want 403", wDetail.Code)
	}

	// Author A reading Co-authored post should get 200
	reqDetailAB := httptest.NewRequest(http.MethodGet, "/api/v1/posts/post-ab", nil)
	reqDetailAB.AddCookie(&http.Cookie{Name: authCookieName, Value: authorCookie(authorA)})
	wDetailAB := httptest.NewRecorder()
	r.ServeHTTP(wDetailAB, reqDetailAB)
	if wDetailAB.Code != http.StatusOK {
		t.Fatalf("authorA accessing post-ab status = %d, want 200", wDetailAB.Code)
	}
	// Omitting authors during a co-authored edit retains the existing ownership list.
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Set("userID", authorA.ID)
	ctx.Set("userPermissions", []string{store.PermissionContentWrite})
	request := &postRequest{}
	if err := (&Handler{store: st}).normalizePostAuthorsRequest(ctx, request, &store.Post{AuthorIDs: []int64{authorA.ID, authorB.ID}}); err != nil {
		t.Fatalf("normalize co-authored owners: %v", err)
	}
	if len(request.authorRefs) != 2 || request.authorRefs[0] != fmt.Sprint(authorA.ID) || request.authorRefs[1] != fmt.Sprint(authorB.ID) {
		t.Fatalf("co-authored owners changed after omitted-author edit: %v", request.authorRefs)
	}

	// Admin reading Author B's post should get 200
	reqDetailAdmin := httptest.NewRequest(http.MethodGet, "/api/v1/posts/post-b", nil)
	reqDetailAdmin.AddCookie(&http.Cookie{Name: authCookieName, Value: authorCookie(admin)})
	wDetailAdmin := httptest.NewRecorder()
	r.ServeHTTP(wDetailAdmin, reqDetailAdmin)
	if wDetailAdmin.Code != http.StatusOK {
		t.Fatalf("admin accessing post-b status = %d, want 200", wDetailAdmin.Code)
	}

	// 3. Revisions
	revID, err := st.CreateRevision(&store.Revision{
		PostSlug:  "post-b",
		Title:     "Old B Title",
		ContentMD: "Old B Markdown",
	})
	if err != nil {
		t.Fatalf("create revision: %v", err)
	}

	// Author A listing revisions for post-b gets 403
	reqRevList := httptest.NewRequest(http.MethodGet, "/api/v1/posts/post-b/revisions", nil)
	reqRevList.AddCookie(&http.Cookie{Name: authCookieName, Value: authorCookie(authorA)})
	wRevList := httptest.NewRecorder()
	r.ServeHTTP(wRevList, reqRevList)
	if wRevList.Code != http.StatusForbidden {
		t.Fatalf("authorA listing revisions for post-b got %d, want 403", wRevList.Code)
	}

	// Author A getting revision detail for post-b gets 403
	reqRevGet := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/revisions/%d", revID), nil)
	reqRevGet.AddCookie(&http.Cookie{Name: authCookieName, Value: authorCookie(authorA)})
	wRevGet := httptest.NewRecorder()
	r.ServeHTTP(wRevGet, reqRevGet)
	if wRevGet.Code != http.StatusForbidden {
		t.Fatalf("authorA getting revision for post-b got %d, want 403", wRevGet.Code)
	}

	// Author B getting revision detail gets 200
	reqRevGetB := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/revisions/%d", revID), nil)
	reqRevGetB.AddCookie(&http.Cookie{Name: authCookieName, Value: authorCookie(authorB)})
	wRevGetB := httptest.NewRecorder()
	r.ServeHTTP(wRevGetB, reqRevGetB)
	if wRevGetB.Code != http.StatusOK {
		t.Fatalf("authorB getting revision for post-b got %d, want 200", wRevGetB.Code)
	}

	// 4. Search
	reqSearch := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=Post", nil)
	reqSearch.AddCookie(&http.Cookie{Name: authCookieName, Value: authorCookie(authorA)})
	wSearch := httptest.NewRecorder()
	r.ServeHTTP(wSearch, reqSearch)
	if wSearch.Code != http.StatusOK {
		t.Fatalf("search status = %d", wSearch.Code)
	}
	var searchResp struct {
		Items []struct {
			Slug string `json:"slug"`
		} `json:"items"`
	}
	if err := json.Unmarshal(wSearch.Body.Bytes(), &searchResp); err != nil {
		t.Fatalf("unmarshal search: %v", err)
	}
	for _, it := range searchResp.Items {
		if it.Slug == "post-b" {
			t.Fatalf("authorA search returned post-b from authorB!")
		}
	}
}

func TestAuthorAttachmentOwnershipIsolation(t *testing.T) {
	r, st, admin, authorA, authorB := newAuthorEnv(t)

	// Create attachment for authorA
	attA := &store.Attachment{
		Filename:  "author_a.png",
		Path:      "2026/09/a.png",
		URL:       "/uploads/2026/09/a.png",
		Mime:      "image/png",
		Size:      1024,
		CreatedAt: time.Now(),
		OwnerID:   authorA.ID,
	}
	idA, err := st.CreateAttachment(attA)
	if err != nil {
		t.Fatalf("create attA: %v", err)
	}

	// Create attachment for authorB
	attB := &store.Attachment{
		Filename:  "author_b.png",
		Path:      "2026/09/b.png",
		URL:       "/uploads/2026/09/b.png",
		Mime:      "image/png",
		Size:      2048,
		CreatedAt: time.Now(),
		OwnerID:   authorB.ID,
	}
	idB, err := st.CreateAttachment(attB)
	if err != nil {
		t.Fatalf("create attB: %v", err)
	}

	// Create legacy/system attachment (owner_id = 0)
	att0 := &store.Attachment{
		Filename:  "legacy.png",
		Path:      "2026/09/legacy.png",
		URL:       "/uploads/2026/09/legacy.png",
		Mime:      "image/png",
		Size:      512,
		CreatedAt: time.Now(),
		OwnerID:   0,
	}
	id0, err := st.CreateAttachment(att0)
	if err != nil {
		t.Fatalf("create att0: %v", err)
	}
	for _, post := range []*store.Post{
		{Slug: "post-a", Title: "Post A Title", ContentMD: "placeholder", Status: store.StatusPublished, AuthorIDs: []int64{authorA.ID}},
		{Slug: "post-b", Title: "Post B Title", ContentMD: "placeholder", Status: store.StatusDraft, AuthorIDs: []int64{authorB.ID}},
	} {
		if _, err := st.Create(post); err != nil {
			t.Fatalf("create ref post: %v", err)
		}
	}
	if _, err := st.DB().Exec(`UPDATE posts SET content_md = ? WHERE slug IN (?, ?)`, "![a](/uploads/2026/09/a.png)", "post-a", "post-b"); err != nil {
		t.Fatalf("add attachment refs: %v", err)
	}

	// 1. List attachments
	reqListA := httptest.NewRequest(http.MethodGet, "/api/v1/attachments", nil)
	reqListA.AddCookie(&http.Cookie{Name: authCookieName, Value: authorCookie(authorA)})
	wListA := httptest.NewRecorder()
	r.ServeHTTP(wListA, reqListA)
	if wListA.Code != http.StatusOK {
		t.Fatalf("list attachments authorA got %d", wListA.Code)
	}
	var attListResp struct {
		Items []struct {
			ID int64 `json:"id"`
		} `json:"items"`
		Total int `json:"total"`
	}
	if err := json.Unmarshal(wListA.Body.Bytes(), &attListResp); err != nil {
		t.Fatalf("unmarshal att list: %v", err)
	}
	if attListResp.Total != 1 || attListResp.Items[0].ID != idA {
		t.Fatalf("authorA expected only idA (%d), got total=%d items=%+v", idA, attListResp.Total, attListResp.Items)
	}

	// Admin list attachments should see all 3
	reqListAdmin := httptest.NewRequest(http.MethodGet, "/api/v1/attachments", nil)
	reqListAdmin.AddCookie(&http.Cookie{Name: authCookieName, Value: authorCookie(admin)})
	wListAdmin := httptest.NewRecorder()
	r.ServeHTTP(wListAdmin, reqListAdmin)
	if wListAdmin.Code != http.StatusOK {
		t.Fatalf("list attachments admin got %d", wListAdmin.Code)
	}
	var adminListResp struct {
		Total int `json:"total"`
	}
	if err := json.Unmarshal(wListAdmin.Body.Bytes(), &adminListResp); err != nil {
		t.Fatalf("unmarshal admin att list: %v", err)
	}
	if adminListResp.Total != 3 {
		t.Fatalf("admin expected 3 attachments, got %d", adminListResp.Total)
	}

	// 2. Delete attachment
	// Author A deleting Author B's attachment -> 403
	reqDelB := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/attachments/%d", idB), nil)
	reqDelB.AddCookie(&http.Cookie{Name: authCookieName, Value: authorCookie(authorA)})
	wDelB := httptest.NewRecorder()
	r.ServeHTTP(wDelB, reqDelB)
	if wDelB.Code != http.StatusForbidden {
		t.Fatalf("authorA delete idB got %d, want 403", wDelB.Code)
	}

	// Author A deleting historical attachment (owner_id = 0) -> 403
	reqDel0 := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/attachments/%d", id0), nil)
	reqDel0.AddCookie(&http.Cookie{Name: authCookieName, Value: authorCookie(authorA)})
	wDel0 := httptest.NewRecorder()
	r.ServeHTTP(wDel0, reqDel0)
	if wDel0.Code != http.StatusForbidden {
		t.Fatalf("authorA delete id0 got %d, want 403", wDel0.Code)
	}

	// 3. Attachment refs
	// Author A querying refs for Author B's attachment -> 403
	reqRefsB := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/attachments/%d/refs", idB), nil)
	reqRefsB.AddCookie(&http.Cookie{Name: authCookieName, Value: authorCookie(authorA)})
	wRefsB := httptest.NewRecorder()
	r.ServeHTTP(wRefsB, reqRefsB)
	if wRefsB.Code != http.StatusForbidden {
		t.Fatalf("authorA refs for idB got %d, want 403", wRefsB.Code)
	}
	reqRefsA := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/attachments/%d/refs", idA), nil)
	reqRefsA.AddCookie(&http.Cookie{Name: authCookieName, Value: authorCookie(authorA)})
	wRefsA := httptest.NewRecorder()
	r.ServeHTTP(wRefsA, reqRefsA)
	if wRefsA.Code != http.StatusOK || strings.Contains(wRefsA.Body.String(), "Post B Title") || !strings.Contains(wRefsA.Body.String(), "Post A Title") {
		t.Fatalf("authorA refs leaked or missed own post: status=%d body=%s", wRefsA.Code, wRefsA.Body.String())
	}

	// 4. Batch move
	// Create attachment group
	group := &store.AttachmentGroup{Name: "New Group", Order: 1}
	groupID, err := st.CreateAttachmentGroup(group)
	if err != nil {
		t.Fatalf("create group: %v", err)
	}

	// Author A attempting batch move of [idA, idB] -> 403 (whole batch rejected)
	batchPayload, _ := json.Marshal(map[string]any{
		"ids":      []int64{idA, idB},
		"group_id": groupID,
	})
	reqBatch := httptest.NewRequest(http.MethodPost, "/api/v1/attachments/move", bytes.NewReader(batchPayload))
	reqBatch.Header.Set("Content-Type", "application/json")
	reqBatch.AddCookie(&http.Cookie{Name: authCookieName, Value: authorCookie(authorA)})
	wBatch := httptest.NewRecorder()
	r.ServeHTTP(wBatch, reqBatch)
	if wBatch.Code != http.StatusForbidden {
		t.Fatalf("authorA moving [idA, idB] status = %d, want 403", wBatch.Code)
	}

	// Verify idA was NOT moved (remains group_id = 0)
	checkA, _ := st.GetAttachment(idA)
	if checkA.GroupID != 0 {
		t.Fatalf("idA should not have been moved, got group_id = %d", checkA.GroupID)
	}

	// Author A moving only [idA] -> 200
	batchValid, _ := json.Marshal(map[string]any{
		"ids":      []int64{idA},
		"group_id": groupID,
	})
	reqBatchValid := httptest.NewRequest(http.MethodPost, "/api/v1/attachments/move", bytes.NewReader(batchValid))
	reqBatchValid.Header.Set("Content-Type", "application/json")
	reqBatchValid.AddCookie(&http.Cookie{Name: authCookieName, Value: authorCookie(authorA)})
	wBatchValid := httptest.NewRecorder()
	r.ServeHTTP(wBatchValid, reqBatchValid)
	if wBatchValid.Code != http.StatusOK {
		t.Fatalf("authorA moving [idA] status = %d, want 200: %s", wBatchValid.Code, wBatchValid.Body.String())
	}
	checkA2, _ := st.GetAttachment(idA)
	if checkA2.GroupID != groupID {
		t.Fatalf("idA should have been moved to %d, got %d", groupID, checkA2.GroupID)
	}
}
