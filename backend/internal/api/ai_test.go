package api

import (
	"crypto/tls"
	"encoding/json"
	"net"
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
	}, "sys", "user", nil)
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

	_, err := callChatCompletion(config.AIConfig{BaseURL: srv.URL, Model: "m"}, "s", "u", nil)
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

	_, err := callChatCompletion(config.AIConfig{BaseURL: srv.URL, Model: "m"}, "s", "u", nil)
	if err == nil {
		t.Fatal("non-JSON response should error")
	}
}

// --- Spec 53 读者自助生成：SSRF 校验 + 限流 ---

func TestValidateGuestAIEndpoint(t *testing.T) {
	cases := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"http scheme rejected", "http://api.example.com/v1", true},
		{"loopback rejected", "https://127.0.0.1/v1", true},
		{"private 10.x rejected", "https://10.1.2.3/v1", true},
		{"private 192.168 rejected", "https://192.168.1.10/v1", true},
		{"link-local rejected", "https://169.254.1.1/v1", true},
		{"empty host rejected", "https:///v1", true},
		{"bad url rejected", "://bad", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ips, err := validateGuestAIEndpoint(c.url)
			if c.wantErr && err == nil {
				t.Fatalf("%s: expected error, got nil", c.url)
			}
			if !c.wantErr && err != nil {
				t.Fatalf("%s: unexpected error: %v", c.url, err)
			}
			if !c.wantErr && len(ips) == 0 {
				t.Fatalf("%s: expected resolved ips", c.url)
			}
		})
	}
}

func TestAIGuestRateAllow(t *testing.T) {
	// 包级限流 map：用测试专属 IP 避免与其他用例互相影响
	ip := "203.0.113.99"
	for i := 0; i < 10; i++ {
		if !aiGuestRateAllow(ip) {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}
	if aiGuestRateAllow(ip) {
		t.Fatal("11th request within window should be rejected")
	}
}

// TestGuestAIClientPinsIPAndRejectsRedirect（Spec 61 阶段 3）：加固客户端
// 1) DialContext 固定拨号校验 IP（封堵 TOCTOU 重绑定）；2) CheckRedirect 拒绝 302 跳转。
func TestGuestAIClientPinsIPAndRejectsRedirect(t *testing.T) {
	ok := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "https://127.0.0.1:1/evil", http.StatusFound)
			return
		}
		w.Write([]byte(`{"ok":true}`))
	}))
	defer ok.Close()

	host, _, _ := net.SplitHostPort(strings.TrimPrefix(ok.URL, "https://"))
	ips, _ := net.LookupIP(host)
	client := newGuestAIHTTPClient(host, ips)
	// 测试自签证书：仅放行证书校验（被测对象是固定拨号与重定向拒绝）
	client.Transport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	// 正常请求经固定拨号可达（非重定向路径不受影响）
	resp, err := client.Get(ok.URL)
	if err != nil {
		t.Fatalf("pinned dial should succeed: %v", err)
	}
	resp.Body.Close()

	// 302 重定向被 CheckRedirect 拒绝
	_, err = client.Get(ok.URL + "/redirect")
	if err == nil || !strings.Contains(err.Error(), "不允许重定向") {
		t.Fatalf("expected redirect rejection, got: %v", err)
	}
}
