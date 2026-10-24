package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"blog/internal/content"
	"blog/internal/post"
	"blog/internal/stellarv2"
	"blog/internal/store"
)

const editorSnakePage = ` {"title":"Raw schema","article":{"ai_label":"manual","paragraph_indent":"always"},"footer":{"show_tags":true},"inject":{"head_begin":"<meta name=\"raw-head\">","head_end":"<!-- raw-head-end -->","body_begin":"<!-- raw-body-begin -->","body_end":"<!-- raw-body-end -->"}} `

// 与正式 Stellar body adapter 使用同一个严格边界；不能让测试回调吞掉 normalized 配置。
func strictEditorBody(h *Handler) func(content.StellarBodyInput) (string, error) {
	return func(input content.StellarBodyInput) (string, error) {
		parsed, err := stellarv2.ParsePage(input.Page, stellarv2.Options{Source: "editor.stellar_config"})
		if err != nil {
			return "", err
		}
		return h.posts.RenderStellarV2WithContext(input.Markdown, post.StellarRenderContext{Page: parsed.Value, Language: input.Language, BlogExtensions: true})
	}
}

func TestStellarEditorRawSchemaSaveReopenAndPreview(t *testing.T) {
	r, st, h, admin, _, _ := newNotebookAPI(t)
	h.renderStellarMarkdown = strictEditorBody(h)
	h.renderStellarContentPreview = func(input content.StellarPreviewInput) (content.StellarPreviewOutput, error) {
		page, err := decodeStellarJSONObject(input.StellarConfig)
		if err != nil {
			return content.StellarPreviewOutput{}, err
		}
		body, err := h.renderStellarBody(content.StellarBodyInput{Markdown: *input.Markdown, Page: page, Language: input.Language})
		return content.StellarPreviewOutput{HTML: body, Document: "<html><body>" + body + "</body></html>"}, err
	}
	if err := st.CreateWikiProject(&store.WikiProject{ID: "raw", Visible: true, StellarVersion: 2, StellarConfig: `{}`, BlogExtensions: `{}`}); err != nil {
		t.Fatal(err)
	}
	cookie := authCookie(admin)
	if w := do2FA(r, http.MethodPost, "/api/v1/notebooks", notebookPayload("raw", `{}`), cookie); w.Code != http.StatusCreated {
		t.Fatal(w.Body.String())
	}
	for _, test := range []struct{ profile, path string }{
		{"post", "/api/v1/posts"}, {"page", "/api/v1/pages"}, {"wiki", "/api/v1/wiki/raw/docs"}, {"note", "/api/v1/notebooks/raw/notes"},
	} {
		t.Run(test.profile, func(t *testing.T) {
			payload := map[string]any{"slug": "raw-schema", "title": "Raw schema", "content_md": "Original body", "status": "draft", "stellar_version": 2, "stellar_config": editorSnakePage, "blog_extensions": `{}`}
			if test.profile == "note" {
				delete(payload, "title")
			}
			encoded, _ := json.Marshal(payload)
			w := do2FA(r, http.MethodPost, test.path, string(encoded), cookie)
			if w.Code != http.StatusCreated {
				t.Fatalf("valid raw create: %d %s", w.Code, w.Body.String())
			}
			path := test.path + "/raw-schema"
			if test.profile == "note" {
				var note NotebookNoteResponse
				if err := json.Unmarshal(w.Body.Bytes(), &note); err != nil {
					t.Fatal(err)
				}
				path = fmt.Sprintf("%s/%d", test.path, note.ID)
			}
			payload["content_md"] = "Updated body"
			encoded, _ = json.Marshal(payload)
			if w = do2FA(r, http.MethodPut, path, string(encoded), cookie); w.Code != http.StatusOK {
				t.Fatalf("valid raw update: %d %s", w.Code, w.Body.String())
			}
			w = do2FA(r, http.MethodGet, path, "", cookie)
			var saved struct {
				StellarContentMetadata
				ContentMD string `json:"content_md"`
			}
			if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &saved) != nil || saved.StellarConfig != editorSnakePage || saved.ContentMD != "Updated body" {
				t.Fatalf("raw reopen: %d %s", w.Code, w.Body.String())
			}
			preview := map[string]any{"markdown": "Updated body", "layout": test.profile, "stellar_version": 2, "stellar_config": editorSnakePage, "blog_extensions": `{}`, "preview_token": "raw_schema_preview_1234"}
			encoded, _ = json.Marshal(preview)
			if w = do2FA(r, http.MethodPost, "/api/v1/preview/markdown", string(encoded), cookie); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "Updated body") {
				t.Fatalf("raw preview: %d %s", w.Code, w.Body.String())
			}
			payload["stellar_config"] = `{"article":{"aiLabel":"manual","paragraphIndent":"always"},"footer":{"showTags":true},"inject":{"headBegin":"bad"}}`
			payload["content_md"] = "Forbidden body"
			encoded, _ = json.Marshal(payload)
			if w = do2FA(r, http.MethodPut, path, string(encoded), cookie); w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "unknown_field") {
				t.Fatalf("camelCase update accepted: %d %s", w.Code, w.Body.String())
			}
			preview["stellar_config"] = payload["stellar_config"]
			encoded, _ = json.Marshal(preview)
			if w = do2FA(r, http.MethodPost, "/api/v1/preview/markdown", string(encoded), cookie); w.Code != http.StatusBadRequest {
				t.Fatalf("camelCase preview accepted: %d %s", w.Code, w.Body.String())
			}
			w = do2FA(r, http.MethodGet, path, "", cookie)
			if strings.Contains(w.Body.String(), "Forbidden body") || !strings.Contains(w.Body.String(), "Updated body") {
				t.Fatal("invalid write mutated saved body")
			}
		})
	}
}
