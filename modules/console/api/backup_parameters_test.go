package api

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestMarkdownImportUnsupportedParameterConsentMultipart(t *testing.T) {
	r, st, user, _ := newFullStellarEnv(t)
	source := []byte("---\nauthor: old-author\npassword: private-password-value\nmusic: {id: 321}\ncomments: true\ntopic: new-topic\nstatus: published\n---\n正文\n")
	upload := func(legacy string, ignored []string, topics string) *httptest.ResponseRecorder {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		_ = writer.WriteField("legacy", legacy)
		for _, value := range ignored {
			_ = writer.WriteField("ignored_parameters", value)
		}
		if topics != "" {
			_ = writer.WriteField("create_topics", topics)
		}
		entry, _ := writer.CreateFormFile("file", "Perfdog.md")
		_, _ = entry.Write(source)
		_ = writer.Close()
		return do2FARaw(r, http.MethodPost, "/api/v1/backup/import/markdown", &body, writer.FormDataContentType(), authCookie(user))
	}
	w := upload("true", nil, "")
	var problem MarkdownImportUnsupportedParametersProblem
	if err := json.Unmarshal(w.Body.Bytes(), &problem); err != nil || w.Code != http.StatusBadRequest || problem.Type != "legacy_markdown_unsupported_parameters" || len(problem.UnsupportedParameters) != 3 {
		t.Fatalf("structured problem: %d %s %v", w.Code, w.Body.String(), err)
	}
	if strings.Contains(w.Body.String(), "private-password-value") || strings.Contains(w.Body.String(), "321") || strings.Contains(w.Body.String(), "old-author") {
		t.Fatal("response leaked source values")
	}
	paths := []string{}
	draftCount := 0
	for _, parameter := range problem.UnsupportedParameters {
		paths = append(paths, parameter.Path)
		if parameter.RequiresDraft {
			draftCount++
		}
		if parameter.Source != "Perfdog.md" || parameter.Field == "" || parameter.Message == "" || parameter.Code == "" {
			t.Fatal("incomplete issue")
		}
	}
	if !reflect.DeepEqual(paths, []string{"/author", "/music", "/password"}) || draftCount != 2 {
		t.Fatalf("unexpected issues: %+v", problem.UnsupportedParameters)
	}
	if value, _ := st.GetBySlug("Perfdog"); value != nil {
		t.Fatal("unconfirmed request wrote data")
	}
	for _, test := range []struct {
		legacy string
		values []string
	}{
		{"false", []string{`[]`}}, {"false", []string{`null`}}, {"false", []string{``}},
		{"true", []string{`null`}}, {"true", []string{`{}`}}, {"true", []string{`[{"source":"Perfdog.md","path":"/password","all":true}]`}},
		{"true", []string{`[{"source":"Perfdog.md"}]`}}, {"true", []string{`[null]`}}, {"true", []string{`[] []`}},
		{"true", []string{`[]`, `[]`}}, {"true", []string{`[{"source":"Perfdog.md","path":"/title"}]`}},
		{"true", []string{`[{"source":"Perfdog.md","path":"/password"},{"source":"Perfdog.md","path":"/password"}]`}},
	} {
		w = upload(test.legacy, test.values, "")
		if w.Code != http.StatusBadRequest {
			t.Fatalf("invalid authorization accepted %+v: %d %s", test, w.Code, w.Body.String())
		}
		if value, _ := st.GetBySlug("Perfdog"); value != nil {
			t.Fatal("invalid request wrote data")
		}
	}
	w = upload("true", []string{`[{"source":"Perfdog.md","path":"/music"}]`}, "")
	if err := json.Unmarshal(w.Body.Bytes(), &problem); err != nil || len(problem.UnsupportedParameters) != 2 || problem.UnsupportedParameters[0].Path != "/author" || problem.UnsupportedParameters[1].Path != "/password" {
		t.Fatalf("partial consent response: %s", w.Body.String())
	}
	approved := `[{"source":"Perfdog.md","path":"/author"},{"source":"Perfdog.md","path":"/music"},{"source":"Perfdog.md","path":"/password"}]`
	w = upload("true", []string{approved}, "")
	var missing MarkdownImportMissingTopicsProblem
	if err := json.Unmarshal(w.Body.Bytes(), &missing); err != nil || missing.Type != "legacy_markdown_topics_required" || !reflect.DeepEqual(missing.MissingTopics, []string{"new-topic"}) {
		t.Fatalf("topic chain response: %s", w.Body.String())
	}
	if value, _ := st.GetBySlug("Perfdog"); value != nil {
		t.Fatal("topic prompt wrote article")
	}
	w = upload("true", []string{approved}, `["new-topic"]`)
	if w.Code != http.StatusOK {
		t.Fatalf("approved request failed: %d %s", w.Code, w.Body.String())
	}
	value, _ := st.GetBySlug("Perfdog")
	if value == nil || value.Status != "draft" || value.ContentMD != "正文\n" || value.Author != "" || len(value.AuthorIDs) == 0 || strings.Contains(value.StellarConfig+value.BlogExtensions, "private-password-value") || strings.Contains(value.StellarConfig+value.BlogExtensions, "old-author") {
		t.Fatalf("wrong persisted result: %+v", value)
	}
	var raw struct {
		Comments struct {
			Enabled bool `json:"enabled"`
		} `json:"comments"`
	}
	if err := json.Unmarshal([]byte(value.StellarConfig), &raw); err != nil || !raw.Comments.Enabled {
		t.Fatalf("legacy comment switch not persisted: %s %v", value.StellarConfig, err)
	}
}
