package api

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"blog/internal/config"
	"blog/internal/safefetch"
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
	}, "sys", "user", srv.Client())
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

	_, err := callChatCompletion(config.AIConfig{BaseURL: srv.URL, Model: "m"}, "s", "u", srv.Client())
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

	_, err := callChatCompletion(config.AIConfig{BaseURL: srv.URL, Model: "m"}, "s", "u", srv.Client())
	if err == nil {
		t.Fatal("non-JSON response should error")
	}
}

func TestCallChatCompletionRejectsLoopbackByDefault(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("loopback AI endpoint must not receive a request")
	}))
	defer srv.Close()

	_, err := callChatCompletion(config.AIConfig{BaseURL: srv.URL, Model: "m"}, "s", "u", nil)
	if err == nil || !strings.Contains(err.Error(), "forbidden address") {
		t.Fatalf("error = %v, want loopback rejection", err)
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

// TestSafeFetchClientRejectsRedirect（Spec 61 阶段 3）：统一客户端
// 1) 拨号前校验解析所得 IP；2) 不跟随 302，避免跳转到内网。
func TestSafeFetchClientRejectsRedirect(t *testing.T) {
	ok := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "https://private.test/evil", http.StatusFound)
			return
		}
		w.Write([]byte(`{"ok":true}`))
	}))
	defer ok.Close()

	_, port, _ := net.SplitHostPort(ok.Listener.Addr().String())
	client := safefetch.New(safefetch.Options{
		Timeout:      3 * time.Second,
		MaxRedirects: 0,
		ResolveIP: func(context.Context, string) ([]net.IP, error) {
			return []net.IP{net.ParseIP("1.1.1.1")}, nil
		},
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, ok.Listener.Addr().String())
		},
	})
	// 测试自签证书：仅放行证书校验（被测对象是固定拨号与重定向拒绝）
	client.Transport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	baseURL := "https://public.test:" + port

	// 正常请求经 SafeFetch 校验后可达。
	resp, err := client.Get(baseURL)
	if err != nil {
		t.Fatalf("safe dial should succeed: %v", err)
	}
	resp.Body.Close()

	// 302 保留给调用方，不会跟随到 private.test。
	resp, err = client.Get(baseURL + "/redirect")
	if err != nil {
		t.Fatalf("redirect response should be returned: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("redirect status = %d, want %d", resp.StatusCode, http.StatusFound)
	}
}
