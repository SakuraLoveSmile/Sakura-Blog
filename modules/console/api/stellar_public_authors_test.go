package api

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"

	"blog/internal/config"
	"blog/internal/store"
)

func TestStrictPublicAuthorsTitleOnlyPublishDoesNotRequireUsersOrChangeOwners(t *testing.T) {
	r, st, admin, _ := newFullStellarEnv(t)
	cookie := authCookie(admin)
	if err := st.PutSetting(config.SectionBlogExtensions, `{"data":{"authors":{"admin":{"name":"Admin"},"writer":{"name":"Writer"},"bob":{"name":"Bob"}}}}`); err != nil {
		t.Fatal(err)
	}
	raw := ` {"article":{"author":"admin"},"comments":{"options":{"large":9007199254740993,"flag":false,"nil":null}}} `
	blog := ` {"content":{"authors":["admin","writer"]},"opaque":{"large":9007199254740993,"flag":false,"nil":null}} `
	payload := func(title string, extensions string) string {
		body, _ := json.Marshal(map[string]any{"stellar_version": 2, "stellar_config": raw, "blog_extensions": extensions, "slug": "public-story", "title": title, "content_md": "正文", "status": "published", "authors": []string{"admin", "writer"}, "author": "admin"})
		return string(body)
	}
	w := do2FA(r, http.MethodPost, "/api/v1/posts", payload("原标题", blog), cookie)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	before, _ := st.GetBySlug("public-story")
	if !slices.Equal(before.AuthorIDs, []int64{admin.ID}) {
		t.Fatal("new post ownership did not bind creator")
	}
	for _, operation := range []struct{ method, path string }{{http.MethodPut, ""}, {http.MethodPut, "/draft"}, {http.MethodPost, "/publish"}} {
		w = do2FA(r, operation.method, "/api/v1/posts/public-story"+operation.path, payload("只改标题", blog), cookie)
		if w.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", operation.path, w.Code, w.Body.String())
		}
		got, _ := st.GetBySlug("public-story")
		if got.StellarConfig != raw || got.BlogExtensions != blog || !slices.Equal(got.AuthorIDs, before.AuthorIDs) || strings.Join(got.Authors, ",") != "admin,writer" {
			t.Fatal("save changed public raw or owner IDs")
		}
	}
	if writer, err := st.GetUserByUsername("writer"); err != nil || writer != nil {
		t.Fatal("saving public writer invented a user")
	}
	// 有同名用户的公开署名同样不能授予文章编辑权限。
	w = do2FA(r, http.MethodPost, "/api/v1/posts/public-story/publish", payload("改署名", `{"content":{"authors":["bob"]}}`), cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("public author change: %d %s", w.Code, w.Body.String())
	}
	bob, _ := st.GetUserByUsername("bob")
	w = do2FA(r, http.MethodGet, "/api/v1/posts/public-story", "", authorCookie(bob))
	if w.Code != http.StatusForbidden {
		t.Fatalf("public author gained permissions: %d", w.Code)
	}
	stable, _ := st.GetBySlug("public-story")
	for _, path := range []string{"", "/draft", "/publish"} {
		method := http.MethodPut
		if path == "/publish" {
			method = http.MethodPost
		}
		w = do2FA(r, method, "/api/v1/posts/public-story"+path, payload("不得保存", `{"content":{"authors":["unknown-private"]}}`), cookie)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("unknown author %s: %d %s", path, w.Code, w.Body.String())
		}
		if strings.Contains(w.Body.String(), "unknown-private") {
			t.Fatal("unknown private author reference echoed")
		}
		got, _ := st.GetBySlug("public-story")
		if got.Title != stable.Title || got.BlogExtensions != stable.BlogExtensions || got.DraftJSON != stable.DraftJSON || !slices.Equal(got.AuthorIDs, stable.AuthorIDs) {
			t.Fatal("rejected author modified record")
		}
	}
	w = do2FA(r, http.MethodGet, "/api/v1/posts/public-story", "", cookie)
	var result postJSON
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Authors) != 1 || result.Authors[0] != "bob" || result.Author != "admin" {
		t.Fatalf("reopen lost independent single/multi public authors: %+v", result.Authors)
	}
	// 管理员存在公开署名，但仅Alice拥有的旧权限ID在严格保存后仍保持。
	alice, _ := st.GetUserByUsername("alice")
	p := &store.Post{Slug: "owned-by-alice", Title: "旧标题", Status: store.StatusPublished, AuthorIDs: []int64{alice.ID}, StellarVersion: 2, StellarConfig: raw, BlogExtensions: blog}
	if _, err := st.Create(p); err != nil {
		t.Fatal(err)
	}
	body := strings.Replace(payload("管理员修改标题", blog), `"slug":"public-story"`, `"slug":"owned-by-alice"`, 1)
	w = do2FA(r, http.MethodPut, "/api/v1/posts/owned-by-alice", body, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("admin preserving existing owner: %d %s", w.Code, w.Body.String())
	}
	got, _ := st.GetBySlug("owned-by-alice")
	if !slices.Equal(got.AuthorIDs, []int64{alice.ID}) {
		t.Fatal("admin changed existing owner IDs")
	}
}
