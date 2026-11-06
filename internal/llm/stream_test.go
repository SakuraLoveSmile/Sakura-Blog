package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// writeSSE 写出一行 SSE 数据并立即刷新，模拟上游逐块输出。
func writeSSE(w http.ResponseWriter, lines ...string) {
	for _, line := range lines {
		fmt.Fprint(w, line, "\n")
	}
	fmt.Fprint(w, "\n")
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

func deltaChunk(text string) string {
	payload, _ := json.Marshal(map[string]any{"choices": []map[string]any{{"delta": map[string]any{"content": text}}}})
	return "data: " + string(payload)
}

func collect(t *testing.T, client *Client, cfg Config, opts StreamOptions) ([]string, StreamResult, error) {
	t.Helper()
	var deltas []string
	result, err := client.ChatCompletionStream(context.Background(), cfg, []Message{{Role: "user", Content: "hi"}}, opts, func(text string) error {
		deltas = append(deltas, text)
		return nil
	})
	return deltas, result, err
}

func TestChatCompletionStreamDeliversDeltasAndSkipsNoise(t *testing.T) {
	var gotAuth, gotAccept string
	var payload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("path = %q", r.URL.Path)
		}
		gotAuth, gotAccept = r.Header.Get("Authorization"), r.Header.Get("Accept")
		_ = json.NewDecoder(r.Body).Decode(&payload)
		w.Header().Set("Content-Type", "text/event-stream")
		writeSSE(w, ": keep-alive")
		writeSSE(w, `data: {"choices":[{"delta":{"role":"assistant"}}]}`)
		writeSSE(w, `data: {"choices":[{"delta":{"reasoning_content":"先想一想"}}]}`)
		writeSSE(w, "event: message", deltaChunk("你好"))
		writeSSE(w, "data:"+strings.TrimPrefix(deltaChunk("，世界"), "data: ")) // 无空格的 data: 也要识别
		writeSSE(w, "data: ping")                                            // 非 JSON 心跳
		writeSSE(w, `data: {"choices":[{"delta":{"content":null},"finish_reason":"stop"}]}`)
		writeSSE(w, `data: {"choices":[],"usage":{"total_tokens":9}}`)
		writeSSE(w, "data: [DONE]")
	}))
	defer srv.Close()

	temperature := float32(0.7)
	deltas, result, err := collect(t, New(srv.Client()), Config{BaseURL: srv.URL + "/v1/", APIKey: "sk-test", Model: "m1"}, StreamOptions{Temperature: &temperature})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	if got := strings.Join(deltas, "|"); got != "你好|，世界" {
		t.Fatalf("deltas = %q", got)
	}
	if result.FinishReason != "stop" || result.Truncated || result.Runes != 5 {
		t.Fatalf("result = %+v", result)
	}
	if gotAuth != "Bearer sk-test" || gotAccept != "text/event-stream" {
		t.Fatalf("headers: auth=%q accept=%q", gotAuth, gotAccept)
	}
	if payload["stream"] != true || payload["model"] != "m1" {
		t.Fatalf("payload = %v", payload)
	}
	// float32(0.7) 序列化后不是精确的 0.7，落在附近即可。
	if got, _ := payload["temperature"].(float64); got < 0.69 || got > 0.71 {
		t.Fatalf("temperature = %v, want about 0.7", payload["temperature"])
	}
}

func TestChatCompletionStreamDefaultTemperature(t *testing.T) {
	var payload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&payload)
		w.Header().Set("Content-Type", "text/event-stream")
		writeSSE(w, deltaChunk("ok"), "")
	}))
	defer srv.Close()

	if _, _, err := collect(t, New(srv.Client()), Config{BaseURL: srv.URL, Model: "m"}, StreamOptions{}); err != nil {
		t.Fatalf("stream: %v", err)
	}
	// float32(0.2) 序列化后不是精确的 0.2，只要落在默认值附近即可。
	temperature, _ := payload["temperature"].(float64)
	if temperature < 0.19 || temperature > 0.21 {
		t.Fatalf("temperature = %v, want about 0.2", payload["temperature"])
	}
}

func TestChatCompletionStreamReportsUpstreamErrors(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{"json error object", http.StatusUnauthorized, `{"error":{"message":"invalid api key"}}`, "invalid api key"},
		{"json error string", http.StatusTooManyRequests, `{"error":"slow down"}`, "slow down"},
		{"top-level message", http.StatusBadRequest, `{"message":"bad model"}`, "bad model"},
		{"html body", http.StatusBadGateway, `<html>bad gateway</html>`, "HTTP 502"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()
			_, _, err := collect(t, New(srv.Client()), Config{BaseURL: srv.URL, Model: "m"}, StreamOptions{})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestChatCompletionStreamReportsInStreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		writeSSE(w, deltaChunk("半句"))
		writeSSE(w, `data: {"error":{"message":"rate limited"}}`)
	}))
	defer srv.Close()

	deltas, _, err := collect(t, New(srv.Client()), Config{BaseURL: srv.URL, Model: "m"}, StreamOptions{})
	if err == nil || !strings.Contains(err.Error(), "rate limited") {
		t.Fatalf("error = %v", err)
	}
	if len(deltas) != 1 || deltas[0] != "半句" {
		t.Fatalf("deltas before the error should still be delivered: %v", deltas)
	}
}

func TestChatCompletionStreamFallsBackToWholeJSONBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []map[string]any{{"message": map[string]string{"role": "assistant", "content": "整段结果"}}}})
	}))
	defer srv.Close()

	deltas, result, err := collect(t, New(srv.Client()), Config{BaseURL: srv.URL, Model: "m"}, StreamOptions{})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	if len(deltas) != 1 || deltas[0] != "整段结果" || result.Runes != 4 {
		t.Fatalf("deltas = %v result = %+v", deltas, result)
	}
}

func TestChatCompletionStreamRejectsMalformedChunk(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		writeSSE(w, "data: {not json}")
	}))
	defer srv.Close()

	_, _, err := collect(t, New(srv.Client()), Config{BaseURL: srv.URL, Model: "m"}, StreamOptions{})
	if err == nil || !strings.Contains(err.Error(), "malformed") {
		t.Fatalf("error = %v", err)
	}
}

func TestChatCompletionStreamTruncatesAtRuneLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		writeSSE(w, deltaChunk("一二三"))
		writeSSE(w, deltaChunk("四五六"))
		writeSSE(w, deltaChunk("七八九"))
	}))
	defer srv.Close()

	deltas, result, err := collect(t, New(srv.Client()), Config{BaseURL: srv.URL, Model: "m"}, StreamOptions{MaxOutputRunes: 4})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	if got := strings.Join(deltas, "|"); got != "一二三|四" {
		t.Fatalf("deltas = %q", got)
	}
	if !result.Truncated || result.Runes != 4 {
		t.Fatalf("result = %+v", result)
	}
}

func TestChatCompletionStreamExactLimitIsNotTruncated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		writeSSE(w, deltaChunk("一二三四"), "data: [DONE]")
	}))
	defer srv.Close()

	_, result, err := collect(t, New(srv.Client()), Config{BaseURL: srv.URL, Model: "m"}, StreamOptions{MaxOutputRunes: 4})
	if err != nil || result.Truncated || result.Runes != 4 {
		t.Fatalf("result = %+v err = %v", result, err)
	}
}

func TestChatCompletionStreamIdleTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		writeSSE(w, deltaChunk("开头"))
		<-r.Context().Done() // 之后一直静默，直到客户端断开
	}))
	defer srv.Close()

	start := time.Now()
	deltas, _, err := collect(t, New(srv.Client()), Config{BaseURL: srv.URL, Model: "m"}, StreamOptions{IdleTimeout: 120 * time.Millisecond})
	if !errors.Is(err, ErrStreamIdle) {
		t.Fatalf("error = %v, want ErrStreamIdle", err)
	}
	if len(deltas) != 1 || deltas[0] != "开头" {
		t.Fatalf("deltas = %v", deltas)
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("idle timeout took %s", elapsed)
	}
}

func TestChatCompletionStreamIdleTimeoutBeforeHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		// 服务端读完请求体后才会监测客户端断连；不先读完，r.Context() 永远不会取消。
		_, _ = io.Copy(io.Discard, r.Body)
		<-r.Context().Done() // 一直不返回响应头
	}))
	defer srv.Close()

	_, _, err := collect(t, New(srv.Client()), Config{BaseURL: srv.URL, Model: "m"}, StreamOptions{IdleTimeout: 120 * time.Millisecond})
	if !errors.Is(err, ErrStreamIdle) {
		t.Fatalf("error = %v, want ErrStreamIdle", err)
	}
}

func TestChatCompletionStreamTrafficResetsIdleTimer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		for i := 0; i < 5; i++ {
			writeSSE(w, ": keep-alive") // 仅注释行也算活动
			time.Sleep(60 * time.Millisecond)
		}
		writeSSE(w, deltaChunk("完成"), "data: [DONE]")
	}))
	defer srv.Close()

	deltas, _, err := collect(t, New(srv.Client()), Config{BaseURL: srv.URL, Model: "m"}, StreamOptions{IdleTimeout: 150 * time.Millisecond})
	if err != nil || len(deltas) != 1 {
		t.Fatalf("deltas = %v err = %v", deltas, err)
	}
}

func TestChatCompletionStreamStopsOnContextCancel(t *testing.T) {
	disconnected := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		writeSSE(w, deltaChunk("第一段"))
		<-r.Context().Done()
		close(disconnected)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, err := New(srv.Client()).ChatCompletionStream(ctx, Config{BaseURL: srv.URL, Model: "m"}, nil, StreamOptions{}, func(string) error {
		cancel() // 浏览器断开：收到第一段后取消
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	select {
	case <-disconnected:
	case <-time.After(3 * time.Second):
		t.Fatal("upstream connection was not closed after cancellation")
	}
}

func TestChatCompletionStreamPropagatesCallbackError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		writeSSE(w, deltaChunk("a"), "", deltaChunk("b"))
	}))
	defer srv.Close()

	sentinel := errors.New("client went away")
	calls := 0
	_, err := New(srv.Client()).ChatCompletionStream(context.Background(), Config{BaseURL: srv.URL, Model: "m"}, nil, StreamOptions{}, func(string) error {
		calls++
		return sentinel
	})
	if !errors.Is(err, sentinel) || calls != 1 {
		t.Fatalf("err = %v calls = %d", err, calls)
	}
}

func TestChatCompletionStreamHandlesLongLines(t *testing.T) {
	long := strings.Repeat("长", 120_000) // 约 360KB 的单行 data
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		writeSSE(w, deltaChunk(long), "data: [DONE]")
	}))
	defer srv.Close()

	_, result, err := collect(t, New(srv.Client()), Config{BaseURL: srv.URL, Model: "m"}, StreamOptions{})
	if err != nil || result.Runes != 120_000 {
		t.Fatalf("runes = %d err = %v", result.Runes, err)
	}
}

func TestChatCompletionStreamValidatesInput(t *testing.T) {
	client := New(nil)
	noop := func(string) error { return nil }
	if _, err := client.ChatCompletionStream(context.Background(), Config{Model: "m"}, nil, StreamOptions{}, noop); err == nil {
		t.Fatal("missing base url should fail")
	}
	if _, err := client.ChatCompletionStream(context.Background(), Config{BaseURL: "https://example.com"}, nil, StreamOptions{}, noop); err == nil {
		t.Fatal("missing model should fail")
	}
	if _, err := client.ChatCompletionStream(context.Background(), Config{BaseURL: "https://example.com", Model: "m"}, nil, StreamOptions{}, nil); err == nil {
		t.Fatal("missing callback should fail")
	}
}

func TestChatCompletionStreamRejectsLoopbackWithProductionClient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("loopback endpoint must not receive a request")
	}))
	defer srv.Close()

	_, _, err := collect(t, New(nil), Config{BaseURL: srv.URL, Model: "m"}, StreamOptions{})
	if err == nil || !strings.Contains(err.Error(), "forbidden address") {
		t.Fatalf("error = %v, want SSRF rejection", err)
	}
}

func TestFirstRunes(t *testing.T) {
	cases := []struct {
		in   string
		n    int
		want string
	}{
		{"一二三", 2, "一二"},
		{"一二三", 3, "一二三"},
		{"一二三", 9, "一二三"},
		{"一二三", 0, ""},
		{"一二三", -1, ""},
		{"", 3, ""},
		{"a😀b", 2, "a😀"},
	}
	for _, tc := range cases {
		if got := firstRunes(tc.in, tc.n); got != tc.want {
			t.Errorf("firstRunes(%q, %d) = %q, want %q", tc.in, tc.n, got, tc.want)
		}
	}
}
