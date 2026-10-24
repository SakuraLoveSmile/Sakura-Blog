package rag

import (
	"bytes"
	"path/filepath"
	"testing"

	"blog/internal/store"
)

func TestRestoreCopiesLogicalRAGIndexWithoutTouchingShadowTables(t *testing.T) {
	for _, test := range []struct {
		name         string
		sourceFilled bool
		liveFilled   bool
	}{
		{"full-to-full", true, true},
		{"full-to-empty", true, false},
		{"empty-to-full", false, true},
		{"empty-to-empty", false, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := newRestoreRAGStore(t, "sourcehit", test.sourceFilled)
			live := newRestoreRAGStore(t, "livehit", test.liveFilled)
			backup := filepath.Join(t.TempDir(), "backup.db")
			if err := source.BackupTo(backup); err != nil {
				t.Fatal(err)
			}

			if err := live.ValidateRestoreFrom(backup); err != nil {
				t.Fatalf("preflight: %v", err)
			}
			assertRestoredRAG(t, live, "livehit", test.liveFilled)
			if err := live.RestoreFrom(backup); err != nil {
				t.Fatalf("restore: %v", err)
			}
			// 原生恢复也用于启动前补偿，不能依赖应用层回调才恢复一致性。
			assertRestoredRAG(t, live, "sourcehit", test.sourceFilled)
			var oldMatches int
			if err := live.DB().QueryRow(`SELECT COUNT(*) FROM rag_chunks_fts WHERE rag_chunks_fts MATCH 'livehit'`).Scan(&oldMatches); err != nil || oldMatches != 0 {
				t.Fatalf("old index survived: count=%d err=%v", oldMatches, err)
			}
			if old, err := live.GetBySlug("livehit"); err != nil || old != nil {
				t.Fatalf("old content survived: post=%+v err=%v", old, err)
			}
			// 恢复后的虚表仍须能正常维护内部索引。
			if _, err := live.DB().Exec(`DELETE FROM rag_chunks_fts; INSERT INTO rag_chunks_fts(title, content, doc_type, slug, chunk_idx) VALUES ('freshhit', 'freshhit', 'post', 'freshhit', 0)`); err != nil {
				t.Fatalf("write restored FTS: %v", err)
			}
			assertRAGIntegrity(t, live)
		})
	}
}

func newRestoreRAGStore(t *testing.T, slug string, filled bool) *store.Store {
	t.Helper()
	st := newRAGStore(t)
	if err := EnsureTables(st.DB()); err != nil {
		t.Fatal(err)
	}
	if err := st.Upsert(&store.Post{Slug: slug, Title: slug, ContentMD: "body " + slug, Status: store.StatusDraft}); err != nil {
		t.Fatal(err)
	}
	if filled {
		if _, err := st.DB().Exec(`INSERT INTO rag_chunks(doc_type, slug, chunk_idx, title, content, embedding, content_hash) VALUES ('post', ?, 0, ?, ?, ?, ?)`, slug, slug, "body "+slug, EncodeVector([]float32{0.25, -0.5}), "hash-"+slug); err != nil {
			t.Fatal(err)
		}
		if _, err := st.DB().Exec(`INSERT INTO rag_chunks_fts(title, content, doc_type, slug, chunk_idx) VALUES (?, ?, 'post', ?, 0)`, slug, "body "+slug, slug); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := st.DB().Exec(`UPDATE rag_meta SET status='idle', embedding_model=?, embedding_dims=2, embedding_fingerprint=? WHERE id=1`, slug, "fingerprint-"+slug); err != nil {
		t.Fatal(err)
	}
	return st
}

func assertRestoredRAG(t *testing.T, st *store.Store, slug string, filled bool) {
	t.Helper()
	p, err := st.GetBySlug(slug)
	if err != nil || p == nil || p.ContentMD != "body "+slug || p.Status != store.StatusDraft {
		t.Fatalf("content mismatch: post=%+v err=%v", p, err)
	}
	want := 0
	if filled {
		want = 1
	}
	for _, query := range []string{
		`SELECT COUNT(*) FROM rag_chunks`,
		`SELECT COUNT(*) FROM rag_chunks_fts`,
		`SELECT COUNT(*) FROM rag_chunks_fts WHERE rag_chunks_fts MATCH '` + slug + `'`,
	} {
		var got int
		if err := st.DB().QueryRow(query).Scan(&got); err != nil || got != want {
			t.Fatalf("%s: count=%d want=%d err=%v", query, got, want, err)
		}
	}
	if filled {
		var embedding []byte
		var hash, content string
		if err := st.DB().QueryRow(`SELECT embedding, content_hash, content FROM rag_chunks WHERE slug=?`, slug).Scan(&embedding, &hash, &content); err != nil || !bytes.Equal(embedding, EncodeVector([]float32{0.25, -0.5})) || hash != "hash-"+slug || content != "body "+slug {
			t.Fatalf("RAG chunk mismatch: hash=%q content=%q embedding=%v err=%v", hash, content, embedding, err)
		}
	}
	var model, fingerprint string
	if err := st.DB().QueryRow(`SELECT embedding_model, embedding_fingerprint FROM rag_meta WHERE id=1`).Scan(&model, &fingerprint); err != nil || model != slug || fingerprint != "fingerprint-"+slug {
		t.Fatalf("RAG metadata mismatch: model=%q fingerprint=%q err=%v", model, fingerprint, err)
	}
	assertRAGIntegrity(t, st)
}

func assertRAGIntegrity(t *testing.T, st *store.Store) {
	t.Helper()
	var integrity string
	if err := st.DB().QueryRow(`PRAGMA integrity_check`).Scan(&integrity); err != nil || integrity != "ok" {
		t.Fatalf("database integrity=%q err=%v", integrity, err)
	}
	if _, err := st.DB().Exec(`INSERT INTO rag_chunks_fts(rag_chunks_fts) VALUES ('integrity-check')`); err != nil {
		t.Fatalf("FTS integrity: %v", err)
	}
}
