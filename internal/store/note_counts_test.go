package store

import (
	"reflect"
	"testing"
)

func TestCountNotesGroupsByNotebookAuthorAndStatus(t *testing.T) {
	s := newStellarStore(t)
	for _, id := range []string{"work", "empty"} {
		if err := s.CreateNotebook(&Notebook{ID: id, AuthorID: 1, StellarVersion: 2, StellarConfig: `{"name":"x"}`, BlogExtensions: `{}`}); err != nil {
			t.Fatal(err)
		}
	}
	add := func(slug string, author int64, status string) {
		t.Helper()
		note := &Note{NotebookID: "work", Slug: slug, AuthorID: author, Status: status, StellarVersion: 2, StellarConfig: `{"title":"` + slug + `"}`, BlogExtensions: `{}`}
		if _, err := s.CreateNote(note); err != nil {
			t.Fatal(err)
		}
	}
	add("a", 1, StatusPublished)
	add("b", 1, StatusDraft)
	add("c", 1, StatusPublished)
	add("d", 2, StatusDraft)
	got, err := s.CountNotes()
	if err != nil {
		t.Fatal(err)
	}
	want := []NoteCountRow{
		{NotebookID: "work", AuthorID: 1, Status: StatusDraft, Count: 1},
		{NotebookID: "work", AuthorID: 1, Status: StatusPublished, Count: 2},
		{NotebookID: "work", AuthorID: 2, Status: StatusDraft, Count: 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CountNotes = %#v, want %#v (empty notebooks must not appear)", got, want)
	}
}
