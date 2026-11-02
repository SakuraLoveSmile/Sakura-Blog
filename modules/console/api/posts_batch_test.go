package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"

	"blog/internal/content"
	tmpl "blog/internal/settings"
	"blog/internal/store"
)

// newBatchPostsEnv 装配批量状态测试用的最小 Console 环境：admin 密码认证，
// 可选注入共享内容事件总线（与生产装配一致：store 写路径和 Console 共用同一条总线）。
func newBatchPostsEnv(t *testing.T, events content.Events) (*gin.Engine, *store.Store, *store.User) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	st := newTestStore(t)
	site := &tmpl.Site{}
	enableAdmin(t, st, site, "secret")
	admin, err := st.GetUserByUsername("admin")
	if err != nil || admin == nil {
		t.Fatalf("get admin user: %v", err)
	}
	if events != nil {
		st.SetContentEvents(events)
	}
	r := gin.New()
	if _, err := MountConsole(r, MountDeps{Config: Config{Store: st, Site: sitePtr(site), Events: events}}); err != nil {
		t.Fatalf("mount console: %v", err)
	}
	return r, st, admin
}

// batchPostsResponseJSON 测试侧独立解码响应，避免直接复用实现类型而掩盖线上 JSON 形状。
type batchPostsResponseJSON struct {
	Succeeded int `json:"succeeded"`
	Skipped   int `json:"skipped"`
	Failed    int `json:"failed"`
}

// decodeBatchPostsResponse 解析计数响应体。
func decodeBatchPostsResponse(t *testing.T, w *httptest.ResponseRecorder) batchPostsResponseJSON {
	t.Helper()
	var resp batchPostsResponseJSON
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode batch response: %v", err)
	}
	return resp
}

// mustSeedPost 直接落库一篇文章（批量状态测试不需要经过编辑器渲染链路）。
func mustSeedPost(t *testing.T, st *store.Store, slug, status string) {
	t.Helper()
	if _, err := st.Create(&store.Post{Slug: slug, Title: slug, Status: status}); err != nil {
		t.Fatalf("seed post %s: %v", slug, err)
	}
}

// TestBatchPostsStatusAPI 覆盖路由挂载、publish↔draft 计数、真实落库、
// skipped 不刷新 updated_at，以及回收站/不存在计入 failed 且不被改动。
func TestBatchPostsStatusAPI(t *testing.T) {
	r, st, admin := newBatchPostsEnv(t, nil)
	cookie := authCookie(admin)
	if !routeSet(r)["POST /api/v1/posts/batch"] {
		t.Fatal("POST /api/v1/posts/batch not registered")
	}

	mustSeedPost(t, st, "batch-alpha", store.StatusPublished)
	mustSeedPost(t, st, "batch-beta", store.StatusPublished)
	mustSeedPost(t, st, "batch-gamma", store.StatusDraft)
	mustSeedPost(t, st, "batch-trashed", store.StatusDraft)
	if err := st.Delete("batch-trashed"); err != nil {
		t.Fatalf("trash post: %v", err)
	}
	alphaBefore, err := st.GetBySlug("batch-alpha")
	if err != nil || alphaBefore == nil {
		t.Fatalf("get alpha before: %v nil=%v", err, alphaBefore == nil)
	}

	// 去空白、去重后：gamma 真实发布，alpha/beta 已发布 skipped，
	// 回收站与不存在的 slug 计入 failed。
	w := do2FA(r, http.MethodPost, "/api/v1/posts/batch",
		`{"slugs":["batch-alpha","batch-beta","batch-gamma"," batch-trashed ","batch-missing","batch-gamma"],"action":"publish"}`, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("batch publish: status=%d body=%s", w.Code, w.Body.String())
	}
	if resp := decodeBatchPostsResponse(t, w); resp != (batchPostsResponseJSON{Succeeded: 1, Skipped: 2, Failed: 2}) {
		t.Fatalf("batch publish counts = %+v, want succeeded=1 skipped=2 failed=2", resp)
	}
	if got, _ := st.GetBySlug("batch-gamma"); got == nil || got.Status != store.StatusPublished {
		t.Fatalf("gamma status = %+v, want published", got)
	}
	alphaAfter, err := st.GetBySlug("batch-alpha")
	if err != nil || alphaAfter == nil {
		t.Fatalf("get alpha after: %v nil=%v", err, alphaAfter == nil)
	}
	if !alphaAfter.UpdatedAt.Equal(alphaBefore.UpdatedAt) {
		t.Fatalf("skipped post updated_at refreshed: before=%v after=%v", alphaBefore.UpdatedAt, alphaAfter.UpdatedAt)
	}
	trashed, err := st.GetBySlug("batch-trashed")
	if err != nil || trashed == nil {
		t.Fatalf("get trashed: %v nil=%v", err, trashed == nil)
	}
	if trashed.Status != store.StatusDraft || trashed.DeletedAt == "" {
		t.Fatalf("trashed post was modified: %+v", trashed)
	}

	// 反向：published → draft，三篇都真实变更。
	w = do2FA(r, http.MethodPost, "/api/v1/posts/batch",
		`{"slugs":["batch-alpha","batch-beta","batch-gamma"],"action":"draft"}`, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("batch draft: status=%d body=%s", w.Code, w.Body.String())
	}
	if resp := decodeBatchPostsResponse(t, w); resp != (batchPostsResponseJSON{Succeeded: 3, Skipped: 0, Failed: 0}) {
		t.Fatalf("batch draft counts = %+v, want succeeded=3 skipped=0 failed=0", resp)
	}
	for _, slug := range []string{"batch-alpha", "batch-beta", "batch-gamma"} {
		if got, _ := st.GetBySlug(slug); got == nil || got.Status != store.StatusDraft {
			t.Fatalf("%s status = %+v, want draft", slug, got)
		}
	}
}

// TestBatchPostsValidation 非法 action、空/空白 slugs、超 100 与非法 JSON 一律 400，
// 且被拒绝的请求不产生任何状态变更。
func TestBatchPostsValidation(t *testing.T) {
	r, st, admin := newBatchPostsEnv(t, nil)
	cookie := authCookie(admin)
	mustSeedPost(t, st, "batch-validate", store.StatusPublished)

	overSlugs := make([]string, 0, maxBatchPostSlugs+1)
	overSlugs = append(overSlugs, "batch-validate")
	for i := 0; i < maxBatchPostSlugs; i++ {
		overSlugs = append(overSlugs, "batch-extra-"+strconv.Itoa(i))
	}
	overBody, err := json.Marshal(map[string]any{"slugs": overSlugs, "action": "draft"})
	if err != nil {
		t.Fatalf("marshal over-limit body: %v", err)
	}

	cases := []struct {
		name string
		body string
	}{
		{"invalid action", `{"slugs":["batch-validate"],"action":"trash"}`},
		{"missing action", `{"slugs":["batch-validate"]}`},
		{"empty slugs", `{"slugs":[],"action":"publish"}`},
		{"nil slugs", `{"action":"publish"}`},
		{"whitespace slugs", `{"slugs":["  ",""],"action":"draft"}`},
		{"over limit", string(overBody)},
		{"invalid json", `{bad`},
	}
	for _, tc := range cases {
		w := do2FA(r, http.MethodPost, "/api/v1/posts/batch", tc.body, cookie)
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400 (body=%s)", tc.name, w.Code, w.Body.String())
		}
	}
	if got, _ := st.GetBySlug("batch-validate"); got == nil || got.Status != store.StatusPublished {
		t.Fatalf("rejected batches changed post status: %+v", got)
	}
}

// TestBatchPostsOwnershipIsolation 非管理员混入他人署名文章时整单 403，
// 且不产生部分成功；只操作自己署名的文章才成功。
func TestBatchPostsOwnershipIsolation(t *testing.T) {
	r, st, _, authorA, authorB := newAuthorEnv(t)
	for _, seed := range []struct {
		slug    string
		ownerID int64
	}{
		{"batch-author-a", authorA.ID},
		{"batch-author-b", authorB.ID},
	} {
		if _, err := st.Create(&store.Post{
			Slug: seed.slug, Title: seed.slug, Status: store.StatusDraft,
			AuthorIDs: []int64{seed.ownerID},
		}); err != nil {
			t.Fatalf("create %s: %v", seed.slug, err)
		}
	}
	cookie := authorCookie(authorA)

	w := do2FA(r, http.MethodPost, "/api/v1/posts/batch",
		`{"slugs":["batch-author-a","batch-author-b"],"action":"publish"}`, cookie)
	if w.Code != http.StatusForbidden {
		t.Fatalf("cross-author batch: status = %d, want 403 (body=%s)", w.Code, w.Body.String())
	}
	var problem Problem
	if err := json.Unmarshal(w.Body.Bytes(), &problem); err != nil {
		t.Fatalf("decode problem: %v", err)
	}
	if problem.Detail != "无权限：只能管理自己署名的文章" {
		t.Fatalf("403 detail = %q, want ownership message", problem.Detail)
	}
	// 部分成功语义被禁止：自己的那篇也不能先落库。
	for _, slug := range []string{"batch-author-a", "batch-author-b"} {
		if got, _ := st.GetBySlug(slug); got == nil || got.Status != store.StatusDraft {
			t.Fatalf("%s changed by rejected batch: %+v", slug, got)
		}
	}

	// 只含自己署名文章：成功并真实落库。
	w = do2FA(r, http.MethodPost, "/api/v1/posts/batch",
		`{"slugs":["batch-author-a"],"action":"publish"}`, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("own batch: status = %d, body = %s", w.Code, w.Body.String())
	}
	if resp := decodeBatchPostsResponse(t, w); resp.Succeeded != 1 {
		t.Fatalf("own batch counts = %+v, want succeeded=1", resp)
	}
	if got, _ := st.GetBySlug("batch-author-a"); got == nil || got.Status != store.StatusPublished {
		t.Fatalf("author-a post status = %+v, want published", got)
	}
	if got, _ := st.GetBySlug("batch-author-b"); got == nil || got.Status != store.StatusDraft {
		t.Fatalf("author-b post should stay draft, got %+v", got)
	}
}

// TestBatchPostsPublishesContentChangedOnce 成功变更后只发布一次 ContentChanged；
// store 仅对实际变更的 slug 触发写路径事件（skipped/failed 不发）。
func TestBatchPostsPublishesContentChangedOnce(t *testing.T) {
	events := content.NewEvents()
	r, st, admin := newBatchPostsEnv(t, events)
	cookie := authCookie(admin)
	mustSeedPost(t, st, "batch-event-changed", store.StatusDraft)
	mustSeedPost(t, st, "batch-event-skipped", store.StatusPublished)

	var got []content.Event
	sub := events.Subscribe(func(event content.Event) { got = append(got, event) })
	defer sub.Close()

	w := do2FA(r, http.MethodPost, "/api/v1/posts/batch",
		`{"slugs":["batch-event-changed","batch-event-skipped","batch-event-missing"],"action":"publish"}`, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("batch publish: status=%d body=%s", w.Code, w.Body.String())
	}
	if resp := decodeBatchPostsResponse(t, w); resp != (batchPostsResponseJSON{Succeeded: 1, Skipped: 1, Failed: 1}) {
		t.Fatalf("counts = %+v, want succeeded=1 skipped=1 failed=1", resp)
	}
	contentChanged, postChanged := 0, 0
	for _, event := range got {
		switch event.Kind {
		case content.ContentChanged:
			contentChanged++
		case content.PostChanged:
			postChanged++
			if event.Slug != "batch-event-changed" || event.Op != "update" {
				t.Fatalf("unexpected post event: %+v", event)
			}
		}
	}
	if contentChanged != 1 {
		t.Fatalf("ContentChanged events = %d, want 1", contentChanged)
	}
	if postChanged != 1 {
		t.Fatalf("PostChanged events = %d, want 1 (only changed slug)", postChanged)
	}
}
