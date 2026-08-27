package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestImportMarkdownPostReturnsMetadataAndStellarTags(t *testing.T) {
	r, st, user, _ := newFullEnv(t)
	content := "---\nslug: imported\ntitle: 导入标题\nexcerpt: 摘要\ntags: [Go, Stellar]\ncategory: 技术\nbanner: /banner.png\ncustom: value\n---\n\n{% tip text:提示 %}\n内容\n{% endtip %}\n{% table %}\n| A | B |\n{% endtable %}\n"
	w := do2FA(r, http.MethodPost, "/api/v1/editor/import/markdown", "{\"filename\":\"demo.md\",\"kind\":\"post\",\"content\":"+mustJSON(t, content)+"}", authCookie(user))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
	}
	var resp MarkdownImportResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.ContentMD != "{% tip text:提示 %}\n内容\n{% endtip %}\n{% table %}\n| A | B |\n{% endtable %}\n" {
		t.Fatalf("content changed: %q", resp.ContentMD)
	}
	if resp.Metadata.Title != "导入标题" || resp.Metadata.Excerpt != "摘要" || resp.Metadata.Category != "技术" {
		t.Fatalf("metadata = %+v", resp.Metadata)
	}
	if resp.SlugSuggestion != "imported" {
		t.Fatalf("slug suggestion = %q", resp.SlugSuggestion)
	}
	if len(resp.StellarTags) != 2 || resp.StellarTags[0] != "tip" || resp.StellarTags[1] != "table" {
		t.Fatalf("stellar tags = %#v", resp.StellarTags)
	}
	if len(resp.UnknownFields) != 1 || resp.UnknownFields[0] != "custom" {
		t.Fatalf("unknown fields = %#v", resp.UnknownFields)
	}
	if len(resp.UnmappedFields) != 0 {
		t.Fatalf("unmapped fields = %#v", resp.UnmappedFields)
	}
	if posts, err := st.List(); err != nil || len(posts) != 0 {
		t.Fatalf("import must not create posts: len=%d err=%v", len(posts), err)
	}
}

func TestImportMarkdownPageReportsPostOnlyFields(t *testing.T) {
	r, _, user, _ := newFullEnv(t)
	body := "---\ntitle: 页面\ncover: /cover.png\nstatus: published\n---\n正文"
	w := do2FA(r, http.MethodPost, "/api/v1/editor/import/markdown", "{\"filename\":\"page.md\",\"kind\":\"page\",\"content\":"+mustJSON(t, body)+"}", authCookie(user))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
	}
	var resp MarkdownImportResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.UnmappedFields) != 1 || resp.UnmappedFields[0] != "cover" {
		t.Fatalf("unmapped fields = %#v", resp.UnmappedFields)
	}
	if resp.Metadata.Status != "published" || resp.ContentMD != "正文" {
		t.Fatalf("response = %+v", resp)
	}
}

func TestImportMarkdownRejectsInvalidInput(t *testing.T) {
	r, _, user, _ := newFullEnv(t)
	cases := []string{
		`{"kind":"other","content":"# x"}`,
		`{"kind":"post","content":"---\ntitle: broken"}`,
	}
	for _, body := range cases {
		w := do2FA(r, http.MethodPost, "/api/v1/editor/import/markdown", body, authCookie(user))
		if w.Code != http.StatusBadRequest {
			t.Errorf("body %s: status = %d, want 400", body, w.Code)
		}
	}
}

func mustJSON(t *testing.T, value string) string {
	t.Helper()
	b, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}
