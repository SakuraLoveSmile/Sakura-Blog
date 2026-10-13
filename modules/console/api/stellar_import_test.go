package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

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
