package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"

	tmpl "blog/internal/settings"
	"blog/internal/store"
)

// Wave 4 测试覆盖补强：文章 CRUD 全链路（列表/详情/创建/更新/草稿/发布/删除/恢复/彻底删除）。

// TestPostCRUDFlow 文章生命周期：创建（slug 自动生成）→ 列表筛选 → 详情 →
// 更新（生成 revision）→ 草稿头 → 发布 → 回收站 → 恢复 → 彻底删除。
func TestPostCRUDFlow(t *testing.T) {
	r, st, user, _ := newFullEnv(t)
	cookie := authCookie(user)

	// 创建：空 slug 从标题自动生成，中文标题保留
	w := do2FA(r, http.MethodPost, "/api/v1/posts",
		`{"title":"Hello World","content_md":"# 标题\n\n正文","status":"published"}`, cookie)
	if w.Code != http.StatusCreated {
		t.Fatalf("create post: status = %d, body = %s", w.Code, w.Body.String())
	}
	var created postJSON
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created post: %v", err)
	}
	if created.Slug != "hello-world" {
		t.Errorf("auto slug = %q, want hello-world", created.Slug)
	}
	if created.Status != "published" || created.Title != "Hello World" {
		t.Errorf("created fields mismatch: %+v", created)
	}
	if created.Cover == "" {
		t.Error("published post should get default cover")
	}

	// 创建：slug 冲突 → 409
	w = do2FA(r, http.MethodPost, "/api/v1/posts", `{"title":"x","slug":"hello-world"}`, cookie)
	if w.Code != http.StatusConflict {
		t.Errorf("duplicate slug: status = %d, want 409", w.Code)
	}

	// 创建：非法 JSON → 400
	w = do2FA(r, http.MethodPost, "/api/v1/posts", `{bad`, cookie)
	if w.Code != http.StatusBadRequest {
		t.Errorf("invalid json: status = %d, want 400", w.Code)
	}

	// 列表：status 筛选
	w = do2FA(r, http.MethodGet, "/api/v1/posts?status=published", "", cookie)
	var list struct {
		Items []postJSON `json:"items"`
		Total int        `json:"total"`
		Page  int        `json:"page"`
		Size  int        `json:"size"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if list.Total != 1 || len(list.Items) != 1 || list.Items[0].Slug != "hello-world" {
		t.Errorf("published list = %+v, want 1 item hello-world", list)
	}

	// 详情
	w = do2FA(r, http.MethodGet, "/api/v1/posts/hello-world", "", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("get post: status = %d", w.Code)
	}
	var detail postJSON
	if err := json.Unmarshal(w.Body.Bytes(), &detail); err != nil {
		t.Fatalf("decode detail: %v", err)
	}
	if detail.ContentMD != "# 标题\n\n正文" {
		t.Errorf("detail content_md mismatch: %q", detail.ContentMD)
	}

	// 详情：不存在 → 404
	w = do2FA(r, http.MethodGet, "/api/v1/posts/no-such", "", cookie)
	if w.Code != http.StatusNotFound {
		t.Errorf("missing post: status = %d, want 404", w.Code)
	}

	// 更新：改标题 → 200，revision 落库
	w = do2FA(r, http.MethodPut, "/api/v1/posts/hello-world",
		`{"title":"Hello Updated","content_md":"# 新版\n\n正文","status":"published"}`, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("update post: status = %d, body = %s", w.Code, w.Body.String())
	}
	var updated postJSON
	if err := json.Unmarshal(w.Body.Bytes(), &updated); err != nil || updated.Title != "Hello Updated" {
		t.Fatalf("update result mismatch: %+v (err %v)", updated, err)
	}
	revs, err := st.ListRevisions("hello-world")
	if err != nil || len(revs) < 1 {
		t.Errorf("update should create revision, revs = %d (err %v)", len(revs), err)
	}

	// 更新：slug 被占用 → 409（先造第二篇）
	do2FA(r, http.MethodPost, "/api/v1/posts", `{"title":"Second Post","slug":"second-post"}`, cookie)
	w = do2FA(r, http.MethodPut, "/api/v1/posts/hello-world",
		`{"title":"x","slug":"second-post","content_md":"c"}`, cookie)
	if w.Code != http.StatusConflict {
		t.Errorf("update to occupied slug: status = %d, want 409", w.Code)
	}

	// 更新：不存在 → 404
	w = do2FA(r, http.MethodPut, "/api/v1/posts/no-such", `{"title":"x","content_md":"c"}`, cookie)
	if w.Code != http.StatusNotFound {
		t.Errorf("update missing post: status = %d, want 404", w.Code)
	}

	// 草稿头：保存后详情 has_draft=true 且带 draft 字段
	w = do2FA(r, http.MethodPut, "/api/v1/posts/hello-world/draft",
		`{"title":"草稿中","content_md":"未保存内容"}`, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("save draft: status = %d, body = %s", w.Code, w.Body.String())
	}
	w = do2FA(r, http.MethodGet, "/api/v1/posts/hello-world", "", cookie)
	_ = json.Unmarshal(w.Body.Bytes(), &detail)
	if !detail.HasDraft || len(detail.Draft) == 0 {
		t.Errorf("post with draft should return has_draft=true + draft, got %+v", detail)
	}
	// 放弃草稿头
	w = do2FA(r, http.MethodDelete, "/api/v1/posts/hello-world/draft", "", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("discard draft: status = %d", w.Code)
	}

	// 发布（已发布状态再发布幂等）
	w = do2FA(r, http.MethodPost, "/api/v1/posts/hello-world/publish",
		`{"title":"Hello Updated","content_md":"# 新版\n\n正文","status":"published"}`, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("publish: status = %d, body = %s", w.Code, w.Body.String())
	}

	// 删除 → 回收站列表可见
	w = do2FA(r, http.MethodDelete, "/api/v1/posts/hello-world", "", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("delete post: status = %d", w.Code)
	}
	w = do2FA(r, http.MethodGet, "/api/v1/posts?status=trashed", "", cookie)
	// 回收站列表返回原始 store.Post（含 DeletedAt），不是 postJSON
	var trash struct {
		Items []store.Post `json:"items"`
		Total int          `json:"total"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &trash); err != nil || trash.Total != 1 {
		t.Fatalf("trashed list: %+v (err %v), want 1", trash, err)
	}
	// 普通列表不再包含
	w = do2FA(r, http.MethodGet, "/api/v1/posts", "", cookie)
	_ = json.Unmarshal(w.Body.Bytes(), &list)
	if list.Total != 1 {
		t.Errorf("normal list should exclude trashed, total = %d", list.Total)
	}

	// 恢复
	w = do2FA(r, http.MethodPost, "/api/v1/posts/hello-world/restore", "", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("restore: status = %d", w.Code)
	}
	w = do2FA(r, http.MethodGet, "/api/v1/posts/hello-world", "", cookie)
	if w.Code != http.StatusOK {
		t.Errorf("restored post should be fetchable, got %d", w.Code)
	}

	// 彻底删除：仅回收站内文章可 purge（恢复后需先再删除）
	w = do2FA(r, http.MethodDelete, "/api/v1/posts/hello-world", "", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("re-delete for purge: status = %d", w.Code)
	}
	w = do2FA(r, http.MethodDelete, "/api/v1/posts/hello-world/purge", "", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("purge: status = %d", w.Code)
	}
	w = do2FA(r, http.MethodGet, "/api/v1/posts/hello-world", "", cookie)
	if w.Code != http.StatusNotFound {
		t.Errorf("purged post should 404, got %d", w.Code)
	}
}

// TestPostCRUDAuth 文章接口未认证一律 401。
func TestPostCRUDAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st := newTestStore(t)
	site := &tmpl.Site{}
	enableAdmin(t, st, site, "secret")
	r := gin.New()
	if _, err := Mount(r, MountDeps{Config: Config{Store: st, Site: sitePtr(site)}}); err != nil {
		t.Fatalf("mount: %v", err)
	}
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/posts"},
		{http.MethodPost, "/api/v1/posts"},
		{http.MethodPut, "/api/v1/posts/x"},
		{http.MethodDelete, "/api/v1/posts/x"},
	} {
		w := do2FA(r, tc.method, tc.path, "{}", "")
		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s %s without auth: status = %d, want 401", tc.method, tc.path, w.Code)
		}
	}
}

// TestGenerateSlug slug 生成边界：空格/标点/折叠连字符/空标题兜底。
func TestGenerateSlug(t *testing.T) {
	cases := []struct{ title, want string }{
		{"Hello World", "hello-world"},
		{"  Trim  Me  ", "trim-me"},
		{"A---B", "a-b"},
		{"中文标题 测试", "中文标题-测试"},
	}
	for _, tc := range cases {
		if got := generateSlug(tc.title); got != tc.want {
			t.Errorf("generateSlug(%q) = %q, want %q", tc.title, got, tc.want)
		}
	}
	// 空标题兜底时间戳格式（generateSlug 内部直接兜底，不返回空串）
	if got := generateSlug("!!!"); len(got) < len("post-") || got[:5] != "post-" {
		t.Errorf("empty slug fallback = %q, want post-<ts>", got)
	}
}

// TestNormalizeTags 标签去重与空白清理。
func TestNormalizeTags(t *testing.T) {
	if got := normalizeTags(" go, go, rust ,,"); got != "go,rust" {
		t.Errorf("normalizeTags = %q, want go,rust", got)
	}
	if got := normalizeTags(""); got != "" {
		t.Errorf("normalizeTags empty = %q, want empty", got)
	}
}

// TestParsePublishAt 定时发布解析：多格式兼容，非法回退零值。
func TestParsePublishAt(t *testing.T) {
	if got := store.ParsePublishAtLocal("2026-08-01T10:30"); got.IsZero() {
		t.Error("T format should parse")
	}
	if got := store.ParsePublishAtLocal("2026-08-01 10:30"); got.IsZero() {
		t.Error("space format should parse")
	}
	if got := store.ParsePublishAtLocal("not-a-date"); !got.IsZero() {
		t.Errorf("invalid date should be zero, got %v", got)
	}
}
