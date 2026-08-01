package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"blog/internal/config"
)

func TestBuildAIPromptIncludesFieldsAndTaxonomy(t *testing.T) {
	system, user := buildAIPrompt(aiGenerateReq{
		Fields:     []string{"title", "category", "tags"},
		Title:      "旧标题",
		Content:    "正文内容",
		Categories: []string{"技术", "生活"},
		Tags:       []string{"Go", "Vue"},
	})
	if !strings.Contains(system, "JSON") {
		t.Fatal("system prompt should require JSON output")
	}
	for _, want := range []string{`"title"`, `"category"`, `"tags"`, "技术、生活", "Go、Vue", "旧标题", "正文内容"} {
		if !strings.Contains(user, want) {
			t.Fatalf("user prompt missing %q:\n%s", want, user)
		}
	}
	// 未请求的字段不应出现指令
	if strings.Contains(user, `"excerpt"`) {
		t.Fatal("user prompt should not include unrequested field excerpt")
	}
}

func TestBuildAIPromptTruncatesLongContent(t *testing.T) {
	long := strings.Repeat("字", aiContentLimit+100)
	_, user := buildAIPrompt(aiGenerateReq{Fields: []string{"excerpt"}, Content: long})
	if !strings.Contains(user, "正文已截断") {
		t.Fatal("long content should be truncated with marker")
	}
	if len([]rune(user)) > aiContentLimit+500 {
		t.Fatalf("user prompt too long: %d runes", len([]rune(user)))
	}
}

func TestParseAIContentPlainJSON(t *testing.T) {
	res, err := parseAIContent(`{"title":"新标题","tags":["a","b"]}`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if res.Title != "新标题" || len(res.Tags) != 2 {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestParseAIContentCodeFenceAndNoise(t *testing.T) {
	content := "好的，以下是结果：\n```json\n{\"slug\":\"hello-world\",\"excerpt\":\"摘要\"}\n```\n希望有帮助"
	res, err := parseAIContent(content)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if res.Slug != "hello-world" || res.Excerpt != "摘要" {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestParseAIContentInvalid(t *testing.T) {
	if _, err := parseAIContent("模型拒绝回答"); err == nil {
		t.Fatal("non-JSON content should error")
	}
}

func TestCallChatCompletionSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer sk-test" {
			t.Errorf("unexpected auth header: %q", got)
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["model"] != "test-model" {
			t.Errorf("unexpected model: %v", body["model"])
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": `{"title":"ok"}`}},
			},
		})
	}))
	defer srv.Close()

	content, err := callChatCompletion(config.AIConfig{
		BaseURL: srv.URL + "/v1/", // 末尾斜杠应被规范化
		APIKey:  "sk-test",
		Model:   "test-model",
	}, "sys", "user")
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if content != `{"title":"ok"}` {
		t.Fatalf("unexpected content: %q", content)
	}
}

func TestCallChatCompletionAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]string{"message": "invalid api key"},
		})
	}))
	defer srv.Close()

	_, err := callChatCompletion(config.AIConfig{BaseURL: srv.URL, Model: "m"}, "s", "u")
	if err == nil || !strings.Contains(err.Error(), "invalid api key") {
		t.Fatalf("expected api error message, got %v", err)
	}
}

func TestCallChatCompletionNonJSONResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("<html>bad gateway</html>"))
	}))
	defer srv.Close()

	_, err := callChatCompletion(config.AIConfig{BaseURL: srv.URL, Model: "m"}, "s", "u")
	if err == nil {
		t.Fatal("non-JSON response should error")
	}
}
