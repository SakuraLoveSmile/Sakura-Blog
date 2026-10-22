package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"blog/internal/store"

	"github.com/gin-gonic/gin"
)

func TestLegacyEmptyTopicPreviewIsReadOnly(t *testing.T) {
	r, st, user, _ := newFullEnv(t)
	before, err := st.ListTopics()
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"post", "page"} {
		request := StellarMarkdownImportRequest{Legacy: true, Kind: kind, Filename: "TeamSpeak使用指南.md", Content: "---\ntitle: TeamSpeak使用指南\ntopic:\n---\n原始正文。\r\n"}
		body, _ := json.Marshal(request)
		w := do2FA(r, http.MethodPost, "/api/v1/editor/import/stellar", string(body), authCookie(user))
		if w.Code != http.StatusOK {
			t.Fatalf("empty topic preview: %d %s", w.Code, w.Body.String())
		}
		var result StellarMarkdownImportResponse
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if !result.LegacyConverted || result.ContentMD != "原始正文。\r\n" || strings.Contains(result.FrontMatter, "collection") || !strings.Contains(strings.Join(result.Warnings, "\n"), "topic 为空") {
			t.Fatalf("empty topic preview lost semantics: %+v", result)
		}
		request.Legacy = false
		body, _ = json.Marshal(request)
		if w := do2FA(r, http.MethodPost, "/api/v1/editor/import/stellar", string(body), authCookie(user)); w.Code != http.StatusBadRequest {
			t.Fatal("strict import bypassed explicit conversion")
		}
	}
	if posts, err := st.List(); err != nil || len(posts) != 0 {
		t.Fatal("preview wrote a post")
	}
	if pages, err := st.ListPages(); err != nil || len(pages) != 0 {
		t.Fatal("preview wrote a page")
	}
	if after, err := st.ListTopics(); err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("preview created or modified a topic")
	}
}

func TestExplicitLegacyStellarPreviewIsReadOnlyAndAuthorized(t *testing.T) {
	r, st, user, _ := newFullEnv(t)
	if err := st.CreateTopic(&store.Topic{Slug: "series", Name: "Series"}); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"post", "page"} {
		request := StellarMarkdownImportRequest{Legacy: true, Kind: kind, Filename: "old.md", Content: "---\nslug: original\nstatus: published\ncategory: Go\ntopic: Series\n---\n{% tip old %}\nbody\n{% endtip %}"}
		body, _ := json.Marshal(request)
		if w := do2FA(r, http.MethodPost, "/api/v1/editor/import/stellar", string(body), ""); w.Code != http.StatusUnauthorized {
			t.Fatalf("unauthenticated legacy import: %d", w.Code)
		}
		w := do2FA(r, http.MethodPost, "/api/v1/editor/import/stellar", string(body), authCookie(user))
		if w.Code != http.StatusOK {
			t.Fatalf("preview: %d %s", w.Code, w.Body.String())
		}
		var result StellarMarkdownImportResponse
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if !result.LegacyConverted || result.SlugSuggestion != "original" || len(result.Warnings) == 0 || !strings.Contains(result.BlogExtensions, `"status": "published"`) || !strings.Contains(result.ContentMD, "blog_tip") {
			t.Fatalf("missing converted preview: %+v", result)
		}
		request.Legacy = false
		body, _ = json.Marshal(request)
		if w := do2FA(r, http.MethodPost, "/api/v1/editor/import/stellar", string(body), authCookie(user)); w.Code != http.StatusBadRequest {
			t.Fatal("strict path fell back to conversion")
		}
	}
	if posts, err := st.List(); err != nil || len(posts) != 0 {
		t.Fatal("legacy preview persisted or published a post")
	}
	if pages, err := st.ListPages(); err != nil || len(pages) != 0 {
		t.Fatal("legacy preview persisted or published a page")
	}
}

func TestLegacyMarkdownPreviewAllowsAuthorButBatchRemainsAdminOnly(t *testing.T) {
	r, st, _, author, _ := newAuthorEnv(t)
	if err := st.EnsureSchema(); err != nil {
		t.Fatal(err)
	}
	body := `{"kind":"post","legacy":true,"content":"---\nstatus: draft\n---\nbody"}`
	if w := do2FA(r, http.MethodPost, "/api/v1/editor/import/stellar", body, authorCookie(author)); w.Code != http.StatusOK {
		t.Fatalf("author preview: %d %s", w.Code, w.Body.String())
	}
	if w := do2FA(r, http.MethodPost, "/api/v1/backup/import/markdown", "", authorCookie(author)); w.Code != http.StatusForbidden {
		t.Fatalf("author batch import: %d %s", w.Code, w.Body.String())
	}
}

func TestStrictStellarImportAndExportAreLosslessAndReadOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &Handler{}
	r := gin.New()
	r.POST("/import", h.importStellarMarkdown)
	r.POST("/export", h.exportStellarMarkdown)
	perform := func(path string, request any) *httptest.ResponseRecorder {
		raw, err := json.Marshal(request)
		if err != nil {
			t.Fatal(err)
		}
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(string(raw)))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		return w
	}
	source := ";;;\n{\"title\":\"New\",\"comments\":{\"enabled\":false,\"options\":{\"custom_limit\":9007199254740993}},\"blog\":{\"poster\":{\"headline\":\"Local\"}}}\n;;;\n{% box Title %}\nbody  \n{% endbox %}\n"
	response := perform("/import", StellarMarkdownImportRequest{Kind: "post", Filename: "new.md", Content: source})
	if response.Code != http.StatusOK {
		t.Fatalf("import %d %s", response.Code, response.Body.String())
	}
	var imported StellarMarkdownImportResponse
	if err := json.Unmarshal(response.Body.Bytes(), &imported); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(imported.FrontMatter, "9007199254740993") || strings.Contains(imported.FrontMatter, "poster") || !strings.Contains(imported.BlogExtensions, "poster") {
		t.Fatal("configuration lost or Blog boundary leaked")
	}
	exportedResponse := perform("/export", StellarMarkdownExportRequest{FrontMatter: imported.FrontMatter, BlogExtensions: imported.BlogExtensions, ContentMD: imported.ContentMD})
	if exportedResponse.Code != http.StatusOK {
		t.Fatalf("export %d %s", exportedResponse.Code, exportedResponse.Body.String())
	}
	var exported StellarMarkdownExportResponse
	if err := json.Unmarshal(exportedResponse.Body.Bytes(), &exported); err != nil {
		t.Fatal(err)
	}
	reopenedResponse := perform("/import", StellarMarkdownImportRequest{Kind: "post", Content: exported.Content})
	var reopened StellarMarkdownImportResponse
	if err := json.Unmarshal(reopenedResponse.Body.Bytes(), &reopened); err != nil {
		t.Fatal(err)
	}
	if reopened.FrontMatter != imported.FrontMatter || reopened.BlogExtensions != imported.BlogExtensions || reopened.ContentMD != imported.ContentMD {
		t.Fatal("reopening changed document")
	}
	if rejected := perform("/import", StellarMarkdownImportRequest{Kind: "post", Content: "---\ntopic: legacy\n---\nbody"}); rejected.Code != http.StatusBadRequest {
		t.Fatal("legacy front matter accepted")
	}
	if rejected := perform("/export", StellarMarkdownExportRequest{FrontMatter: `{"pinned":true}`, BlogExtensions: `{}`, ContentMD: "body"}); rejected.Code != http.StatusBadRequest {
		t.Fatal("legacy export silently migrated")
	}
	// 未装配 Store，成功导入/导出即证明这些处理器没有持久化副作用。
}

func TestStrictStellarImportRouteRequiresAuthorAndDoesNotWrite(t *testing.T) {
	r, st, user, _ := newFullEnv(t)
	body := `{"kind":"post","filename":"new.md","content":"---\ntitle: New\ncomments: {enabled: false}\n---\nbody"}`
	if w := do2FA(r, http.MethodPost, "/api/v1/editor/import/stellar", body, ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated strict import: %d %s", w.Code, w.Body.String())
	}
	if w := do2FA(r, http.MethodPost, "/api/v1/editor/import/stellar", body, authCookie(user)); w.Code != http.StatusOK {
		t.Fatalf("authorized strict import: %d %s", w.Code, w.Body.String())
	}
	if posts, err := st.List(); err != nil || len(posts) != 0 {
		t.Fatalf("preview wrote posts: %d %v", len(posts), err)
	}
}
