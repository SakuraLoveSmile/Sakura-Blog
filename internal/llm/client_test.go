package llm

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestChatCompletionAndEmbeddingBatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("authorization = %q", got)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode request: %v", err)
		}
		switch r.URL.Path {
		case "/v1/chat/completions":
			if payload["model"] != "chat-model" {
				t.Errorf("chat model = %v", payload["model"])
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{
				map[string]any{"message": map[string]string{"content": "answer"}},
			}})
		case "/v1/embeddings":
			if payload["model"] != "embedding-model" {
				t.Errorf("embedding model = %v", payload["model"])
			}
			// Return out of order to verify that response indexes are honored.
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{
				map[string]any{"index": 1, "embedding": []float32{0, 1}},
				map[string]any{"index": 0, "embedding": []float32{1, 0}},
			}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := New(server.Client())
	answer, err := client.ChatCompletion(context.Background(), Config{BaseURL: server.URL + "/v1/", APIKey: "test-key", Model: "chat-model"}, []Message{{Role: "user", Content: "hello"}})
	if err != nil || answer != "answer" {
		t.Fatalf("chat answer=%q err=%v", answer, err)
	}
	vectors, err := client.Embed(context.Background(), Config{BaseURL: server.URL + "/v1", APIKey: "test-key", Model: "embedding-model"}, []string{"a", "b"})
	if err != nil || len(vectors) != 2 || vectors[0][0] != 1 || vectors[1][1] != 1 {
		t.Fatalf("embedding vectors=%v err=%v", vectors, err)
	}
}

func TestClientReportsHTTPAndNonJSONErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/error/") {
			w.WriteHeader(http.StatusBadGateway)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"message": "upstream failed"}})
			return
		}
		w.WriteHeader(http.StatusBadGateway)
		_, _ = io.WriteString(w, "not json")
	}))
	defer server.Close()
	client := New(server.Client())

	if _, err := client.ChatCompletion(context.Background(), Config{BaseURL: server.URL + "/error", Model: "m"}, nil); err == nil || !strings.Contains(err.Error(), "upstream failed") {
		t.Fatalf("JSON HTTP error = %v", err)
	}
	if _, err := client.ChatCompletion(context.Background(), Config{BaseURL: server.URL + "/other", Model: "m"}, nil); err == nil || !strings.Contains(err.Error(), "not JSON") {
		t.Fatalf("non-JSON HTTP error = %v", err)
	}
}

func TestClientRejectsSSRFWithProductionClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("SSRF endpoint received a request")
	}))
	defer server.Close()
	_, err := New(nil).ChatCompletion(context.Background(), Config{BaseURL: server.URL, Model: "m"}, nil)
	if err == nil || !strings.Contains(err.Error(), "forbidden address") {
		t.Fatalf("SSRF error = %v", err)
	}
}

func TestClientHonorsContextCancellation(t *testing.T) {
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		<-req.Context().Done()
		return nil, req.Context().Err()
	})
	client := New(&http.Client{Transport: transport})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := client.ChatCompletion(ctx, Config{BaseURL: "https://example.com", Model: "m"}, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation error = %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }
