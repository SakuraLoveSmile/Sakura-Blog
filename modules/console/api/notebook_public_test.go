package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// Console 列表需要的“在博客中查看”路径与笔记数量，必须和前台路由同源，并按当前用户的权限统计。
func TestNotebookAPIReportsPublicPathAndNoteCounts(t *testing.T) {
	r, _, _, admin, alice, _ := newNotebookAPI(t)
	w := do2FA(r, http.MethodPost, "/api/v1/notebooks", notebookPayload("work", `{"name":"工作"}`), authCookie(alice))
	var book NotebookResponse
	if w.Code != http.StatusCreated || json.Unmarshal(w.Body.Bytes(), &book) != nil || book.PublicPath != "/notebooks/work" || book.NoteCount != 0 || book.PublishedCount != 0 {
		t.Fatalf("new notebook should expose its default public path with zero notes: %d %s", w.Code, w.Body.String())
	}
	create := func(cookie, slug, config string) NotebookNoteResponse {
		t.Helper()
		request := NotebookNoteRequest{Slug: &slug, ContentMD: ptrNotebook("正文"), StellarVersion: ptrNotebook(2), StellarConfig: &config}
		w := do2FA(r, http.MethodPost, "/api/v1/notebooks/work/notes", marshalNotebookRequest(request), cookie)
		var note NotebookNoteResponse
		if w.Code != http.StatusCreated || json.Unmarshal(w.Body.Bytes(), &note) != nil {
			t.Fatalf("create %s: %d %s", slug, w.Code, w.Body.String())
		}
		return note
	}
	draft := create(authCookie(alice), "first", `{"title":"草稿"}`)
	nested := create(authCookie(alice), "nested/second", `{"title":"嵌套"}`)
	if draft.PublicPath != "" || nested.PublicPath != "" {
		t.Fatalf("drafts have no public page: %q %q", draft.PublicPath, nested.PublicPath)
	}
	w = do2FA(r, http.MethodPost, fmt.Sprintf("/api/v1/notebooks/work/notes/%d/publish", nested.ID), `{}`, authCookie(alice))
	var published NotebookNoteResponse
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &published) != nil || published.PublicPath != "/notebooks/work/nested%2Fsecond" {
		t.Fatalf("published note should carry the same escaped path the public router serves: %d %s", w.Code, w.Body.String())
	}
	create(authCookie(admin), "by-admin", `{"title":"管理员"}`)

	listFor := func(cookie string) NotebookResponse {
		t.Helper()
		w := do2FA(r, http.MethodGet, "/api/v1/notebooks", "", cookie)
		var list NotebookListResponse
		if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &list) != nil || len(list.Items) != 1 {
			t.Fatalf("list notebooks: %d %s", w.Code, w.Body.String())
		}
		return list.Items[0]
	}
	if got := listFor(authCookie(alice)); got.NoteCount != 2 || got.PublishedCount != 1 || got.PublicPath != "/notebooks/work" {
		t.Fatalf("author must only count notes they can manage: %+v", got)
	}
	if got := listFor(authCookie(admin)); got.NoteCount != 3 || got.PublishedCount != 1 {
		t.Fatalf("admin counts every note: %+v", got)
	}

	// 集合自己的 route.path 优先，笔记路径跟着变；笔记 permalink 又优先于集合路径。
	w = do2FA(r, http.MethodPut, "/api/v1/notebooks/work", notebookPayload("work", `{"name":"工作","route":{"path":"/my/work/"}}`), authCookie(alice))
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &book) != nil || book.PublicPath != "/my/work" {
		t.Fatalf("route.path must drive the public path: %d %s", w.Code, w.Body.String())
	}
	w = do2FA(r, http.MethodGet, fmt.Sprintf("/api/v1/notebooks/work/notes/%d", nested.ID), "", authCookie(alice))
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &published) != nil || published.PublicPath != "/my/work/nested%2Fsecond" {
		t.Fatalf("note path must follow the collection route: %d %s", w.Code, w.Body.String())
	}
	config := `{"title":"嵌套","permalink":"/special/page/"}`
	w = do2FA(r, http.MethodPut, fmt.Sprintf("/api/v1/notebooks/work/notes/%d", nested.ID), marshalNotebookRequest(NotebookNoteRequest{StellarConfig: &config}), authCookie(alice))
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &published) != nil || published.PublicPath != "/special/page" {
		t.Fatalf("permalink must win like the public router: %d %s", w.Code, w.Body.String())
	}

	// permalink 是作者填写的任意文本：Console 拿到的只能是站内路径，不能变成 javascript: 或外站链接。
	for _, permalink := range []string{"javascript:alert(1)", "//evil.example/x", "https://evil.example/y?z=1"} {
		config := fmt.Sprintf(`{"title":"嵌套","permalink":%q}`, permalink)
		w = do2FA(r, http.MethodPut, fmt.Sprintf("/api/v1/notebooks/work/notes/%d", nested.ID), marshalNotebookRequest(NotebookNoteRequest{StellarConfig: &config}), authCookie(alice))
		if w.Code != http.StatusOK {
			if w.Code < 400 || w.Code >= 500 {
				t.Fatalf("hostile permalink %q must be accepted safely or rejected by validation: %d %s", permalink, w.Code, w.Body.String())
			}
			continue
		}
		if json.Unmarshal(w.Body.Bytes(), &published) != nil {
			t.Fatalf("decode note for %q: %s", permalink, w.Body.String())
		}
		t.Logf("permalink %q -> public path %q", permalink, published.PublicPath)
		parsed, err := url.Parse(published.PublicPath)
		if err != nil || !strings.HasPrefix(published.PublicPath, "/") || strings.HasPrefix(published.PublicPath, "//") || parsed.Scheme != "" || parsed.Host != "" {
			t.Fatalf("permalink %q leaked into an unsafe public path %q", permalink, published.PublicPath)
		}
	}
}

// 迁移前站点没有 Stellar 2.0 主题快照，此时不给前台路径，避免 Console 展示打不开的链接。
func TestNotebookAPIOmitsPublicPathWithoutStellarTheme(t *testing.T) {
	r, _, h, _, alice, _ := newNotebookAPI(t)
	w := do2FA(r, http.MethodPost, "/api/v1/notebooks", notebookPayload("work", `{}`), authCookie(alice))
	if w.Code != http.StatusCreated {
		t.Fatal(w.Body.String())
	}
	next := *h.site.Load()
	next.StellarV2 = nil
	h.site.Store(&next)
	w = do2FA(r, http.MethodGet, "/api/v1/notebooks/work", "", authCookie(alice))
	var book NotebookResponse
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &book) != nil || book.PublicPath != "" {
		t.Fatalf("no theme snapshot => no public path: %d %s", w.Code, w.Body.String())
	}
}
