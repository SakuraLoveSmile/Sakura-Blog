package rag

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"blog/internal/config"
	"blog/internal/content"
	"blog/internal/llm"
	"blog/internal/store"
)

func TestChunkTextAndVectorEncoding(t *testing.T) {
	if got := ChunkText(" "); got != nil {
		t.Fatalf("empty chunks = %#v", got)
	}
	for _, text := range []string{"中文正文：SQLite 性能", "English paragraph about retrieval"} {
		chunks := ChunkText(text)
		if len(chunks) != 1 || chunks[0] != text {
			t.Fatalf("chunks for %q = %#v", text, chunks)
		}
	}
	long := strings.Repeat("长", 1100)
	chunks := ChunkText(long)
	if len(chunks) < 3 || len([]rune(chunks[0])) != chunkTarget {
		t.Fatalf("long chunks = %d, first rune count=%d", len(chunks), len([]rune(chunks[0])))
	}
	if got := []rune(chunks[0])[chunkTarget-chunkOverlap:]; string(got) != string([]rune(chunks[1])[:chunkOverlap]) {
		t.Fatal("long paragraph chunks do not overlap by the configured rune count")
	}
	vector := []float32{1.5, -2, 0}
	decoded, err := DecodeVector(EncodeVector(vector))
	if err != nil || len(decoded) != len(vector) || decoded[1] != vector[1] {
		t.Fatalf("vector round trip = %v err=%v", decoded, err)
	}
	if _, err := DecodeVector([]byte{1}); err == nil {
		t.Fatal("odd-sized vector BLOB should fail")
	}
}

func TestRRFAndCitations(t *testing.T) {
	if got := buildMatch("SQLite 性能"); got != `"SQLite" "性 能"` {
		t.Fatalf("FTS match = %q", got)
	}
	chunks := []Chunk{
		{DocType: TypePost, Slug: "post", Title: "文章", Content: "正文"},
		{DocType: TypePage, Slug: "about", Title: "关于", Content: "页面"},
	}
	citations := citationsFromAnswer("参考 [S2]，也参考 [S1]，忽略 [S9]", chunks)
	if len(citations) != 2 || citations[0].URL != "/about" || citations[1].URL != "/post/post" {
		t.Fatalf("citations = %#v", citations)
	}
	if cosine([]float32{1, 0}, []float32{1, 0}) < 0.99 || cosine([]float32{1, 0}, []float32{0, 1}) != 0 {
		t.Fatal("cosine ranking is incorrect")
	}
}

func TestRebuildIndexesOnlyPublicContentAndKeepsOldIndexOnFailure(t *testing.T) {
	st := newRAGStore(t)
	st.SetContentEvents(content.NewEvents())
	putAIConfig(t, st, `{"base_url":"https://rag.example/v1","model":"chat","embedding_model":"embed"}`)
	mustCreatePost(t, st, &store.Post{Slug: "public", Title: "公开文章", Status: store.StatusPublished, ContentHTML: "<p>公开 SQLite 内容</p>"})
	mustCreatePost(t, st, &store.Post{Slug: "draft", Title: "草稿", Status: store.StatusDraft, ContentHTML: "<p>不可见</p>"})
	mustCreatePost(t, st, &store.Post{Slug: "locked", Title: "受保护", Status: store.StatusPublished, Password: "secret", ContentHTML: "<p>不可见</p>"})
	if _, err := st.CreatePage(&store.Page{Slug: "about", Title: "关于", Status: store.StatusPublished, ContentHTML: "<p>关于本站</p>"}); err != nil {
		t.Fatalf("create page: %v", err)
	}

	transport := &ragTransport{}
	service, err := NewWithClient(st, nil, llm.New(&http.Client{Transport: transport}))
	if err != nil {
		t.Fatalf("new RAG: %v", err)
	}
	if err := service.RebuildNow(context.Background()); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	status, err := service.Status()
	if err != nil || status.DocumentCount != 2 || status.ChunkCount != 2 || status.Status != StatusIdle {
		t.Fatalf("status = %+v err=%v", status, err)
	}
	var before int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM rag_chunks`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	transport.failEmbeddings.Store(true)
	if err := service.RebuildNow(context.Background()); err == nil {
		t.Fatal("failed embedding rebuild should return an error")
	}
	var after int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM rag_chunks`).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Fatalf("failed rebuild replaced old index: before=%d after=%d", before, after)
	}
	status, _ = service.Status()
	if status.Status != StatusError || status.LastError == "" {
		t.Fatalf("failed rebuild status = %+v", status)
	}
	if err := service.Close(context.Background()); err != nil {
		t.Fatalf("close without context: %v", err)
	}
}

func TestPublicToDraftRemovesIncrementalIndex(t *testing.T) {
	st := newRAGStore(t)
	events := content.NewEvents()
	st.SetContentEvents(events)
	putAIConfig(t, st, `{"base_url":"https://rag.example/v1","model":"chat","embedding_model":"embed"}`)
	mustCreatePost(t, st, &store.Post{Slug: "change", Title: "变更", Status: store.StatusPublished, ContentHTML: "<p>公开内容</p>"})
	service, err := NewWithClient(st, events, llm.New(&http.Client{Transport: &ragTransport{}}))
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RebuildNow(context.Background()); err != nil {
		t.Fatal(err)
	}
	service.Start()
	if err := st.Update("change", &store.Post{Slug: "change", Title: "变更", Status: store.StatusDraft, ContentHTML: "<p>草稿</p>"}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		var count int
		_ = st.DB().QueryRow(`SELECT COUNT(*) FROM rag_chunks WHERE slug = 'change'`).Scan(&count)
		if count == 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	var count int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM rag_chunks WHERE slug = 'change'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("draft remains indexed: count=%d err=%v", count, err)
	}
	_ = service.Close(context.Background())
}

func TestRenameRemovesPreviousSlugFromIndex(t *testing.T) {
	st := newRAGStore(t)
	if err := st.EnsureRevisionsTable(); err != nil {
		t.Fatal(err)
	}
	putAIConfig(t, st, `{"base_url":"https://rag.example/v1","model":"chat","embedding_model":"embed"}`)
	mustCreatePost(t, st, &store.Post{Slug: "old-slug", Title: "改名文章", Status: store.StatusPublished, ContentHTML: "<p>正文</p>"})
	events := content.NewEvents()
	service, err := NewWithClient(st, events, llm.New(&http.Client{Transport: &ragTransport{}}))
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close(context.Background())
	if err := service.RebuildNow(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateSlug("old-slug", "new-slug", &store.Post{Slug: "new-slug", Title: "改名文章", Status: store.StatusPublished, ContentHTML: "<p>正文</p>"}); err != nil {
		t.Fatal(err)
	}
	service.Start()
	service.onEvent(content.Event{Kind: content.PostChanged, Slug: "new-slug", PreviousSlug: "old-slug"})
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		var oldCount, newCount int
		_ = st.DB().QueryRow(`SELECT COUNT(*) FROM rag_chunks WHERE slug = 'old-slug'`).Scan(&oldCount)
		_ = st.DB().QueryRow(`SELECT COUNT(*) FROM rag_chunks WHERE slug = 'new-slug'`).Scan(&newCount)
		if oldCount == 0 && newCount > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("rename index state did not converge")
}

func TestInvalidateForRestorePreventsOldIndexLeak(t *testing.T) {
	st := newRAGStore(t)
	putAIConfig(t, st, `{"base_url":"https://rag.example/v1","model":"chat","embedding_model":"embed"}`)
	mustCreatePost(t, st, &store.Post{Slug: "restore", Title: "旧公开内容", Status: store.StatusPublished, ContentHTML: "<p>旧正文</p>"})
	transport := &ragTransport{}
	service, err := NewWithClient(st, nil, llm.New(&http.Client{Transport: transport}))
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close(context.Background())
	if err := service.RebuildNow(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := st.Update("restore", &store.Post{Slug: "restore", Title: "恢复内容", Status: store.StatusPublished, ContentHTML: "<p>恢复后正文</p>"}); err != nil {
		t.Fatal(err)
	}
	if err := service.InvalidateForRestore(); err != nil {
		t.Fatalf("invalidate: %v", err)
	}
	transport.failEmbeddings.Store(true)
	if err := service.RebuildNow(context.Background()); err == nil {
		t.Fatal("failed restore rebuild should not succeed")
	}
	if chunks, err := service.retrieve("旧正文", []float32{1}); err != nil || len(chunks) != 0 {
		t.Fatalf("invalidated index returned old content: chunks=%d err=%v", len(chunks), err)
	}
	if err := st.Update("restore", &store.Post{Slug: "restore", Title: "私有内容", Status: store.StatusDraft, ContentHTML: "<p>新私有正文</p>"}); err != nil {
		t.Fatal(err)
	}
	transport.failEmbeddings.Store(false)
	if err := service.RebuildNow(context.Background()); err != nil {
		t.Fatalf("valid rebuild: %v", err)
	}
	if chunks, err := service.retrieve("旧正文", []float32{1}); err != nil || len(chunks) != 0 {
		t.Fatalf("private restored content became indexed: chunks=%d err=%v", len(chunks), err)
	}
}

func newRAGStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.New(filepath.Join(t.TempDir(), "rag.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := st.EnsureSettingsTable(); err != nil {
		st.Close()
		t.Fatal(err)
	}
	if err := st.EnsurePagesTable(); err != nil {
		st.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func putAIConfig(t *testing.T, st *store.Store, raw string) {
	t.Helper()
	var ai map[string]any
	if err := json.Unmarshal([]byte(raw), &ai); err != nil {
		t.Fatal(err)
	}
	if _, err := config.InitializeStellarV2(st, map[string]any{}, map[string]any{"ai": ai}); err != nil {
		t.Fatal(err)
	}
	config.InvalidateLoadAI()
	t.Cleanup(config.InvalidateLoadAI)
}

func mustCreatePost(t *testing.T, st *store.Store, post *store.Post) {
	t.Helper()
	if _, err := st.Create(post); err != nil {
		t.Fatal(err)
	}
}

type ragTransport struct {
	failEmbeddings atomic.Bool
}

func (t *ragTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Path == "/v1/embeddings" {
		if t.failEmbeddings.Load() {
			return response(req, http.StatusBadGateway, `{"error":{"message":"embedding down"}}`), nil
		}
		var payload struct {
			Input []string `json:"input"`
		}
		body, _ := io.ReadAll(req.Body)
		_ = json.Unmarshal(body, &payload)
		data := make([]map[string]any, len(payload.Input))
		for i := range payload.Input {
			data[i] = map[string]any{"index": i, "embedding": []float32{1, 0}}
		}
		return response(req, http.StatusOK, marshalJSON(map[string]any{"data": data})), nil
	}
	return response(req, http.StatusOK, `{"choices":[{"message":{"content":"回答 [S1]"}}]}`), nil
}

func response(req *http.Request, status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Status: fmt.Sprintf("%d", status), Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: req}
}

func marshalJSON(value any) string {
	data, _ := json.Marshal(value)
	return string(data)
}
