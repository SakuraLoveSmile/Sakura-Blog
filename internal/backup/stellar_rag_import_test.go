package backup

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"blog/internal/rag"
	"blog/internal/store"
)

func TestMarkdownWithoutFrontMatterImportsWithPopulatedRAG(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		name := "strict"
		if legacy {
			name = "legacy-draft"
		}
		t.Run(name, func(t *testing.T) {
			svc, st, _ := newStellarContentFixture(t)
			if err := st.CreateUser(&store.User{Username: "admin", Role: store.RoleAdmin, PasswordHash: "x", AuthSecret: "test"}); err != nil {
				t.Fatal(err)
			}
			index, err := rag.New(st, nil)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = index.Close(context.Background()) })
			if _, err := st.DB().Exec(`INSERT INTO rag_chunks VALUES ('post', 'post', 0, 'oldhit', 'oldhit', ?, 'hash');
				INSERT INTO rag_chunks_fts(title, content, doc_type, slug, chunk_idx) VALUES ('oldhit', 'oldhit', 'post', 'post', 0)`, rag.EncodeVector([]float32{1})); err != nil {
				t.Fatal(err)
			}
			svc.SetRestoreRebuild(index.InvalidateForRestore)
			before, err := st.GetBySlug("post")
			if err != nil {
				t.Fatal(err)
			}
			source := []byte("## 作者\r\n\r\n> 作者：原作者\r\n\r\n![](https://example.com/redirectstorage/image.jpg)\r\n\r\n## 末尾\r\n完整正文。  \r\n")
			var result *ImportResult
			if legacy {
				result, err = svc.ImportLegacyMarkdownContext(context.Background(), "Redirectstorage.md", source)
			} else {
				result, err = svc.Import("Redirectstorage.md", source)
			}
			if err != nil || result == nil || !reflect.DeepEqual(result.Imported, []string{"Redirectstorage"}) {
				t.Fatalf("import result=%+v err=%v", result, err)
			}
			wantStatus := store.StatusPublished
			if legacy {
				wantStatus = store.StatusDraft
			}
			p, err := st.GetBySlug("Redirectstorage")
			if err != nil || p == nil || p.Title != "Redirectstorage" || p.ContentMD != string(source) || p.Status != wantStatus || !strings.Contains(p.ContentHTML, "完整正文") {
				t.Fatalf("imported Markdown changed: post=%+v err=%v", p, err)
			}
			after, err := st.GetBySlug("post")
			if err != nil || !reflect.DeepEqual(before, after) || st.Maintenance().Active() {
				t.Fatalf("import changed unrelated content or left maintenance active: %v", err)
			}
			for _, query := range []string{`SELECT COUNT(*) FROM rag_chunks`, `SELECT COUNT(*) FROM rag_chunks_fts`} {
				var count int
				if err := st.DB().QueryRow(query).Scan(&count); err != nil || count != 0 {
					t.Fatalf("restore callback did not invalidate RAG: count=%d err=%v", count, err)
				}
			}
			if _, err := st.DB().Exec(`INSERT INTO rag_chunks_fts(rag_chunks_fts) VALUES ('integrity-check')`); err != nil {
				t.Fatalf("FTS integrity after import: %v", err)
			}
		})
	}
}
