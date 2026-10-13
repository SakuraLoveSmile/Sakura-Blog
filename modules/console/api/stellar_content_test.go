package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"blog/internal/content"
	"blog/internal/post"
	"blog/internal/store"
	"github.com/gin-gonic/gin"
)

const editorRawPage = ` {"title":"原值", "comments":{"enabled":false,"title":null},"leftbar":{"widgets":[{"custom":{"huge":900719925474099312345,"value":false,"none":null}}]}} `
const editorRawBlog = ` {"huge":900719925474099312345,"value":false,"none":null} `

func editorPayload(t *testing.T, fields map[string]any) string {
	t.Helper()
	fields["stellar_version"] = 2
	fields["stellar_config"] = editorRawPage
	fields["blog_extensions"] = editorRawBlog
	b, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestStellarEditorContentAPIReopensOriginalJSON(t *testing.T) {
	r, st, user, h := newFullStellarEnv(t)
	cookie := authCookie(user)
	for _, kind := range []string{"posts", "pages"} {
		body := editorPayload(t, map[string]any{"slug": "canonical-" + kind, "title": "宿主标题", "content_md": "{% frame iphone img:https://example.com/a.png %}", "status": "draft"})
		w := do2FA(r, http.MethodPost, "/api/v1/"+kind, body, cookie)
		if w.Code != http.StatusCreated {
			t.Fatalf("create %s %d %s", kind, w.Code, w.Body.String())
		}
		w = do2FA(r, http.MethodGet, "/api/v1/"+kind+"/canonical-"+kind, "", cookie)
		var got struct {
			StellarContentMetadata
			ContentMD string `json:"content_md"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil || got.StellarConfig != editorRawPage || got.BlogExtensions != editorRawBlog || got.StellarVersion != 2 {
			t.Fatalf("reopen %s lost raw metadata: %s %v", kind, w.Body.String(), err)
		}
		expected, err := h.renderEditorStellar(got.ContentMD, got.StellarContentMetadata, strings.TrimSuffix(kind, "s"))
		if err != nil {
			t.Fatal(err)
		}
		var html string
		if kind == "posts" {
			saved, _ := st.GetBySlug("canonical-" + kind)
			html = saved.ContentHTML
		} else {
			saved, _ := st.GetPageBySlug("canonical-" + kind)
			html = saved.ContentHTML
		}
		if html != expected || !strings.Contains(html, `class="frame-wrap"`) {
			t.Fatalf("%s did not use stock v2 renderer: %s", kind, html)
		}
		w = do2FA(r, http.MethodPut, "/api/v1/"+kind+"/canonical-"+kind, body, cookie)
		if w.Code != http.StatusOK {
			t.Fatalf("save %s %d %s", kind, w.Code, w.Body.String())
		}
	}
}

func TestStellarEditorRejectsLegacyAndInvalidWritesWithoutMutation(t *testing.T) {
	r, st, user, _ := newFullStellarEnv(t)
	cookie := authCookie(user)
	if _, err := st.Create(&store.Post{Slug: "legacy-editor", Title: "旧文", ContentMD: "原文", Status: store.StatusDraft}); err != nil {
		t.Fatal(err)
	}
	w := do2FA(r, http.MethodGet, "/api/v1/posts/legacy-editor", "", cookie)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"stellar_version":0`) {
		t.Fatalf("legacy preview read: %d %s", w.Code, w.Body.String())
	}
	for _, body := range []string{
		`{"title":"改写","content_md":"新文"}`,
		`{"stellar_version":2,"stellar_config":"{\"wiki\":\"old\"}","blog_extensions":"{}","title":"改写"}`,
		`{"stellar_version":2,"stellar_config":"{\"collection\":{\"profile\":\"topic\",\"id\":\"missing\"}}","blog_extensions":"{}","title":"改写"}`,
	} {
		w = do2FA(r, http.MethodPut, "/api/v1/posts/legacy-editor", body, cookie)
		if w.Code != http.StatusBadRequest && w.Code != http.StatusConflict {
			t.Fatalf("invalid save accepted: %d %s", w.Code, w.Body.String())
		}
		saved, _ := st.GetBySlug("legacy-editor")
		if saved.Title != "旧文" || saved.ContentMD != "原文" || saved.StellarVersion != 0 {
			t.Fatalf("invalid write mutated legacy: %+v", saved)
		}
	}
}

func TestStellarEditorShortcodeSerializerPreservesPresenceAndBlogNamespace(t *testing.T) {
	r := gin.New()
	h := &Handler{}
	r.POST("/serialize", h.shortcodeSerialize)
	r.POST("/parse", h.shortcodeEdit)
	request := func(path, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		return w
	}
	raw := `{% checkbox checked:false "" %}`
	parsed, err := post.ParseShortcodeForEditStellarV2(raw)
	if err != nil {
		t.Fatal(err)
	}
	parsed.Values["text"] = ""
	b, _ := json.Marshal(ShortcodeSerializeRequest{Name: parsed.Name, Values: parsed.Values, Present: &parsed.Present})
	w := request("/serialize", string(b))
	var response ShortcodeSerializeResponse
	_ = json.Unmarshal(w.Body.Bytes(), &response)
	again, err := post.ParseShortcodeForEditStellarV2(response.Raw)
	if w.Code != 200 || err != nil || again.Values["checked"] != "false" || len(again.Present) != len(parsed.Present) {
		t.Fatalf("presence lost: %d %s %+v %v", w.Code, w.Body.String(), again, err)
	}
	w = request("/parse", `{"raw":"{% blog_links %}"}`)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"editable":true`) {
		t.Fatalf("Blog namespace not editable: %s", w.Body.String())
	}
	w = request("/parse", `{"raw":"{% links %}"}`)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"editable":false`) {
		t.Fatalf("legacy tag silently admitted: %s", w.Body.String())
	}
	w = request("/serialize", `{"name":"copy","values":{"text":900719925474099312345}}`)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "900719925474099312345") {
		t.Fatalf("serializer lost number precision: %s", w.Body.String())
	}
}

func TestStellarEditorPreviewTokenPreservesCanonicalSnapshot(t *testing.T) {
	r, _, user, _ := newFullStellarEnv(t)
	w := do2FA(r, http.MethodPost, "/api/v1/preview/post", editorPayload(t, map[string]any{"title": "预览", "content_md": "原文"}), authCookie(user))
	var response struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &response)
	got, ok := GetPostPreview(response.Token)
	if w.Code != 200 || !ok || got.StellarVersion != 2 || got.StellarConfig != editorRawPage || got.BlogExtensions != editorRawBlog || got.ContentMD != "原文" {
		t.Fatalf("preview snapshot changed: %d %+v", w.Code, got)
	}
}

func TestStellarEditorValidationIsReadOnlyAndReportsUnknownFields(t *testing.T) {
	r, st, _, h := newFullStellarEnv(t)
	r.POST("/validation-fixture", h.validateStellarContent)
	before, _ := st.List()
	request := StellarContentValidationRequest{StellarContentMetadata: StellarContentMetadata{2, editorRawPage, editorRawBlog}, Profile: "post"}
	b, _ := json.Marshal(request)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/validation-fixture", strings.NewReader(string(b)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("valid candidate: %d %s", w.Code, w.Body.String())
	}
	request.StellarConfig = `{"style":{"darkmode":"dark"}}`
	b, _ = json.Marshal(request)
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/validation-fixture", strings.NewReader(string(b)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), `"path":"style"`) {
		t.Fatalf("missing located schema error: %d %s", w.Code, w.Body.String())
	}
	after, _ := st.List()
	if len(after) != len(before) {
		t.Fatal("validation wrote content")
	}
}

func TestStellarEditorPreviewCallbackReceivesCompleteCandidate(t *testing.T) {
	r, _, user, h := newFullStellarEnv(t)
	var captured content.StellarPreviewInput
	h.renderStellarContentPreview = func(input content.StellarPreviewInput) (string, error) {
		captured = input
		return "<!doctype html><html data-test=\"callback\"></html>", nil
	}
	body := editorPayload(t, map[string]any{"markdown": "{% frame iphone img:https://example.com/a.png %}", "preview_token": "current-preview-token", "title": "宿主标题", "layout": "post", "path": "/post/canonical"})
	w := do2FA(r, http.MethodPost, "/api/v1/preview/markdown", body, authCookie(user))
	var response StellarMarkdownPreviewResponse
	_ = json.Unmarshal(w.Body.Bytes(), &response)
	if w.Code != 200 || captured.StellarVersion != 2 || captured.StellarConfig != editorRawPage || captured.BlogExtensions != editorRawBlog || captured.Path != "/post/canonical" || captured.Layout != "post" || captured.PreviewToken != "current-preview-token" || !strings.Contains(captured.HTML, `class="frame-wrap"`) || !strings.HasSuffix(captured.RequestURL, "/api/v1/preview/markdown") || response.PreviewToken != captured.PreviewToken || !strings.Contains(response.Document, "data-test") {
		t.Fatalf("callback candidate mismatch: %d %+v %s", w.Code, captured, w.Body.String())
	}
	h.renderStellarContentPreview = nil
	w = do2FA(r, http.MethodPost, "/api/v1/preview/markdown", body, authCookie(user))
	if w.Code != http.StatusNotImplemented {
		t.Fatalf("missing document adapter must fail explicitly: %d", w.Code)
	}
}
