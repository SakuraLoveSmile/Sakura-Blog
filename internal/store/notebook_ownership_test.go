package store

import (
	"strings"
	"testing"
)

func TestNotebookOwnershipSchemaUpgradePreservesLegacyBytes(t *testing.T) {
	s := newMigrationStore(t)
	for _, ddl := range []string{notebooksDDL, notesDDL} {
		legacy := strings.ReplaceAll(ddl, "\tauthor_id INTEGER NOT NULL DEFAULT 0,\n", "")
		if _, err := s.db.Exec(legacy); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.db.Exec(`INSERT INTO notebooks(id,name,stellar_version,stellar_config,blog_extensions) VALUES('legacy','原本',0,'','')`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`INSERT INTO notes(notebook_id,slug,content_md,stellar_version,stellar_config,blog_extensions) VALUES('legacy','old','正文',0,'','')`); err != nil {
		t.Fatal(err)
	}
	for repeat := 0; repeat < 2; repeat++ {
		if err := s.EnsureSchema(); err != nil {
			t.Fatal(err)
		}
		if err := s.EnsureNotebooksTable(); err != nil {
			t.Fatal(err)
		}
		if err := s.EnsureNotesTable(); err != nil {
			t.Fatal(err)
		}
	}
	notebook, err := s.GetNotebook("legacy")
	if err != nil || notebook.AuthorID != 0 || notebook.StellarVersion != 0 || notebook.StellarConfig != "" || notebook.Name != "原本" {
		t.Fatalf("legacy notebook changed during schema upgrade: %v", err)
	}
	note, err := s.GetNoteBySlug("legacy", "old")
	if err != nil || note.AuthorID != 0 || note.StellarVersion != 0 || note.StellarConfig != "" || note.ContentMD != "正文" {
		t.Fatalf("legacy note changed during schema upgrade: %v", err)
	}
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version=3 AND name='notebook-note-ownership'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("ownership schema ledger not idempotent: %d %v", count, err)
	}
}

func TestNotebookOwnershipPersistsAcrossReopen(t *testing.T) {
	s := newStellarStore(t)
	notebook := &Notebook{ID: "owned", Name: "本", AuthorID: 42, StellarVersion: 2, StellarConfig: `{}`, BlogExtensions: `{}`}
	if err := s.CreateNotebook(notebook); err != nil {
		t.Fatal(err)
	}
	note := &Note{NotebookID: notebook.ID, Slug: "a", AuthorID: 43, StellarVersion: 2, StellarConfig: `{}`, BlogExtensions: `{}`}
	if _, err := s.CreateNote(note); err != nil {
		t.Fatal(err)
	}
	path := s.DBPath()
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	gotNotebook, err := reopened.GetNotebook("owned")
	if err != nil || gotNotebook.AuthorID != 42 {
		t.Fatal("notebook owner did not persist")
	}
	gotNote, err := reopened.GetNoteByID(note.ID)
	if err != nil || gotNote.AuthorID != 43 {
		t.Fatal("note owner did not persist")
	}
}
