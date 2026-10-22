package store

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestNotebookNoteCRUDAndTagIsolation(t *testing.T) {
	s := newStellarStore(t)
	notebook := &Notebook{ID: "工作", Name: "工作", StellarVersion: 2, StellarConfig: stellarCollectionRaw, BlogExtensions: stellarExtensionsRaw}
	if err := s.CreateNotebook(notebook); err != nil {
		t.Fatal(err)
	}
	other := *notebook
	other.ID = "other"
	if err := s.CreateNotebook(&other); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateNotebook(notebook); !errors.Is(err, ErrNotebookConflict) {
		t.Fatalf("duplicate notebook: %v", err)
	}
	got, err := s.GetNotebook(notebook.ID)
	if err != nil {
		t.Fatal(err)
	}
	assertStellarRaw(t, got.StellarVersion, got.StellarConfig, got.BlogExtensions, stellarCollectionRaw)
	created := got.CreatedAt
	got.Description = "更新"
	if err := s.UpdateNotebook(got.ID, got); err != nil {
		t.Fatal(err)
	}
	got, _ = s.GetNotebook(got.ID)
	if !created.Equal(got.CreatedAt) || got.Description != "更新" {
		t.Fatal("notebook timestamp or update mismatch")
	}
	collections, err := s.ListNotebooks()
	if err != nil || len(collections) != 2 {
		t.Fatalf("notebooks=%#v err=%v", collections, err)
	}
	tags := []string{"技术/Go", "技术/Go", "Work/AI", "WORK/ai", "/Work//AI/", "100%/under_score"}
	note := &Note{NotebookID: notebook.ID, Slug: "nested/entry", Title: "笔记", ContentMD: "正文", ContentHTML: "<p>正文</p>", Tags: tags, Status: StatusPublished, CreatedAt: time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC), StellarVersion: 2, StellarConfig: stellarPageRaw, BlogExtensions: stellarExtensionsRaw}
	id, err := s.CreateNote(note)
	if err != nil {
		t.Fatal(err)
	}
	noteGot, err := s.GetNoteBySlug(note.NotebookID, note.Slug)
	if err != nil {
		t.Fatal(err)
	}
	assertStellarRaw(t, noteGot.StellarVersion, noteGot.StellarConfig, noteGot.BlogExtensions, stellarPageRaw)
	if !reflect.DeepEqual(noteGot.Tags, tags) || noteGot.ContentMD != "正文" {
		t.Fatalf("note raw mismatch: %#v", noteGot)
	}
	duplicate := *note
	duplicate.ID = 0
	if _, err := s.CreateNote(&duplicate); !errors.Is(err, ErrNoteConflict) {
		t.Fatalf("duplicate note: %v", err)
	}
	duplicate.NotebookID = other.ID
	if _, err := s.CreateNote(&duplicate); err != nil {
		t.Fatalf("different notebook slug: %v", err)
	}
	draft := *note
	draft.ID = 0
	draft.Slug = "draft"
	draft.Status = StatusDraft
	if _, err := s.CreateNote(&draft); err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{"技术", "技术/go", "work", "work/ai", "100%", "100%/under_score"} {
		matches, err := s.ListNotesByTag(notebook.ID, query, true)
		if err != nil || len(matches) != 1 || matches[0].ID != id {
			t.Fatalf("tag %q matches=%#v err=%v", query, matches, err)
		}
	}
	matches, err := s.ListNotesByTag(notebook.ID, "work/ai", false)
	if err != nil || len(matches) != 2 {
		t.Fatalf("draft tag query=%#v %v", matches, err)
	}
	for _, query := range []string{"wor", "100", "under_score"} {
		matches, err := s.ListNotesByTag(notebook.ID, query, false)
		if err != nil || len(matches) != 0 {
			t.Fatalf("unrelated tag %q matches=%#v err=%v", query, matches, err)
		}
	}
	if published, _ := s.GetPublishedNote(notebook.ID, draft.Slug); published != nil {
		t.Fatal("draft publicly visible")
	}
	if err := s.DeleteNotebook(notebook.ID); !errors.Is(err, ErrNotebookInUse) {
		t.Fatalf("delete used notebook: %v", err)
	}
	noteGot.Slug = "renamed"
	noteGot.ContentMD = "新正文"
	noteGot.Tags = []string{"A/B", "A/B"}
	noteGot.CreatedAt = time.Time{}
	if err := s.UpdateNote(noteGot); err != nil {
		t.Fatal(err)
	}
	updated, _ := s.GetNoteByID(id)
	if !updated.CreatedAt.Equal(note.CreatedAt) || updated.ContentMD != "新正文" || updated.Slug != "renamed" {
		t.Fatalf("note update: %#v", updated)
	}
	if old, _ := s.GetNoteBySlug(note.NotebookID, note.Slug); old != nil {
		t.Fatal("old slug still exists")
	}
	updated.ContentMD = "不能写"
	updated.StellarConfig = `{"notebook":"legacy"}`
	if err := s.UpdateNote(updated); err == nil {
		t.Fatal("invalid note config accepted")
	}
	unchanged, _ := s.GetNoteByID(id)
	if unchanged.ContentMD != "新正文" || unchanged.StellarConfig != stellarPageRaw {
		t.Fatal("invalid config changed note")
	}
	unchanged.NotebookID = other.ID
	if err := s.UpdateNote(unchanged); !errors.Is(err, ErrNoteInvalid) {
		t.Fatalf("implicit notebook move: %v", err)
	}
	if err := s.DeleteNote(id); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteNote(draft.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteNotebook(notebook.ID); err != nil {
		t.Fatal(err)
	}
	if notes, err := s.ListNotes(other.ID); err != nil || len(notes) != 1 {
		t.Fatal("deletion crossed notebook boundary")
	}
}

func TestNotebookNoteValidationBeforeWrite(t *testing.T) {
	s := newStellarStore(t)
	if err := s.CreateNotebook(&Notebook{ID: "new"}); !errors.Is(err, ErrStellarMigrationRequired) {
		t.Fatal(err)
	}
	nb := &Notebook{ID: "new", StellarVersion: 2, StellarConfig: `{}`}
	if err := s.CreateNotebook(nb); err != nil {
		t.Fatal(err)
	}
	for _, note := range []*Note{
		{NotebookID: "new", Slug: "a", StellarVersion: 2, StellarConfig: `null`},
		{NotebookID: "new", Slug: "a", StellarVersion: 2, StellarConfig: `{"collection":{"profile":"notebook","id":"other"}}`},
		{NotebookID: "new", Slug: "a", StellarVersion: 2, StellarConfig: `{"collection":{"profile":"wiki","id":"new"}}`},
		{NotebookID: "new", Slug: "../a", StellarVersion: 2, StellarConfig: `{}`},
		{NotebookID: "new", Slug: "a", Status: "unknown", StellarVersion: 2, StellarConfig: `{}`},
		{NotebookID: "missing", Slug: "a", StellarVersion: 2, StellarConfig: `{}`},
	} {
		if _, err := s.CreateNote(note); err == nil {
			t.Fatalf("invalid note created: %#v", note)
		}
	}
	notes, err := s.ListNotes("new")
	if err != nil || len(notes) != 0 {
		t.Fatalf("failed writes left notes: %#v %v", notes, err)
	}
	if _, err := s.db.Exec(`UPDATE notebooks SET stellar_version=0,stellar_config='' WHERE id='new'`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateNote(&Note{NotebookID: "new", Slug: "under-legacy", StellarVersion: 2, StellarConfig: `{}`}); !errors.Is(err, ErrStellarMigrationRequired) {
		t.Fatalf("note under unmigrated notebook: %v", err)
	}
}
