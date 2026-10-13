package api

import (
	"strings"
	"testing"

	"blog/internal/content"
	"blog/internal/post"
	"encoding/json"
	"net/http"
)

func TestStellarEditorWikiTOCUsesPublicContentContext(t *testing.T) {
	r, st, user, h := newFullStellarEnv(t)
	const markdown = "{% toc wiki:docs %}"
	var calls int
	h.renderStellarMarkdown = func(input content.StellarBodyInput) (string, error) {
		calls++
		if input.Markdown != markdown {
			t.Fatalf("changed Markdown: %q", input.Markdown)
		}
		return h.posts.RenderStellarV2WithContext(input.Markdown, post.StellarRenderContext{Page: input.Page, Data: map[string]any{"wiki": map[string]any{"tree": map[string]any{"docs": map[string]any{"pages": []any{map[string]any{"title": "Public introduction", "path": "docs/intro"}}}}}}})
	}
	cookie := authCookie(user)
	w := do2FA(r, http.MethodPost, "/api/v1/pages", editorPayload(t, map[string]any{"slug": "toc", "title": "TOC", "content_md": markdown, "status": "draft"}), cookie)
	if w.Code != http.StatusCreated {
		t.Fatal(w.Body.String())
	}
	saved, err := st.GetPageBySlug("toc")
	if err != nil || saved == nil || !strings.Contains(saved.ContentHTML, "Public introduction") {
		t.Fatalf("saved body missing context: %v %+v", err, saved)
	}
	w = do2FA(r, http.MethodPost, "/api/v1/preview/markdown", editorPayload(t, map[string]any{"markdown": markdown, "layout": "page", "preview_token": "preview_context_1234"}), cookie)
	var response StellarMarkdownPreviewResponse
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &response) != nil || response.HTML != saved.ContentHTML || !strings.Contains(response.Document, saved.ContentHTML) {
		t.Fatalf("preview body differs: %d %s", w.Code, w.Body.String())
	}
	if calls != 2 {
		t.Fatalf("renderer calls = %d", calls)
	}
}

func TestStellarEditorMissingBodyRendererRejectsWriteAndPreview(t *testing.T) {
	r, st, user, h := newFullStellarEnv(t)
	h.renderStellarMarkdown = nil
	cookie := authCookie(user)
	for _, endpoint := range []string{"/api/v1/pages", "/api/v1/preview/markdown"} {
		w := do2FA(r, http.MethodPost, endpoint, editorPayload(t, map[string]any{"slug": "no-renderer", "title": "Pending", "content_md": "Body", "markdown": "Body", "layout": "page", "preview_token": "preview_context_1234"}), cookie)
		if w.Code != http.StatusServiceUnavailable {
			t.Fatalf("%s: %d %s", endpoint, w.Code, w.Body.String())
		}
	}
	saved, err := st.GetPageBySlug("no-renderer")
	if err != nil || saved != nil {
		t.Fatalf("rejected write mutated store: %v %+v", err, saved)
	}
}

func TestNotebookNoteConsumesSharedBodyRenderer(t *testing.T) {
	r, st, h, _, alice, _ := newNotebookAPI(t)
	markdown := "{% toc wiki:docs %}"
	calls := 0
	h.renderStellarMarkdown = func(input content.StellarBodyInput) (string, error) {
		calls++
		if input.Markdown != markdown || input.Page["title"] != "My note" {
			t.Fatalf("detached note input: %+v", input)
		}
		return "<p>PUBLIC_WIKI_CONTEXT</p>", nil
	}
	cookie := authCookie(alice)
	w := do2FA(r, http.MethodPost, "/api/v1/notebooks", notebookPayload("book", `{}`), cookie)
	if w.Code != http.StatusCreated {
		t.Fatal(w.Body.String())
	}
	w = do2FA(r, http.MethodPost, "/api/v1/notebooks/book/notes", marshalNotebookRequest(NotebookNoteRequest{Slug: ptrNotebook("one"), ContentMD: &markdown, StellarVersion: ptrNotebook(2), StellarConfig: ptrNotebook(`{"title":"My note"}`)}), cookie)
	var response NotebookNoteResponse
	if w.Code != http.StatusCreated || json.Unmarshal(w.Body.Bytes(), &response) != nil {
		t.Fatalf("create note: %d %s", w.Code, w.Body.String())
	}
	saved, err := st.GetNoteByID(response.ID)
	if err != nil || saved == nil || saved.ContentHTML != "<p>PUBLIC_WIKI_CONTEXT</p>" || calls != 1 {
		t.Fatalf("note bypassed shared body renderer: %v %+v", err, saved)
	}
}

func TestStellarPreviewLanguageReachesSharedBodyRenderer(t *testing.T) {
	r, _, user, h := newFullStellarEnv(t)
	h.renderStellarMarkdown = func(input content.StellarBodyInput) (string, error) {
		if input.Language != "zh-TW" {
			t.Fatalf("preview language lost: %q", input.Language)
		}
		return h.posts.RenderStellarV2WithContext(input.Markdown, post.StellarRenderContext{Page: input.Page, Language: input.Language})
	}
	w := do2FA(r, http.MethodPost, "/api/v1/preview/markdown", editorPayload(t, map[string]any{"markdown": "{% mbti INTJ-T 93-72-78-83-51 %}", "layout": "page", "language": "zh-TW", "preview_token": "preview_context_1234"}), authCookie(user))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "架構師") {
		t.Fatalf("preview language: %d %s", w.Code, w.Body.String())
	}
}
