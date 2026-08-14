package store

import (
	"path/filepath"
	"testing"
)

func TestEnsureLinksTableMigratesWatchColumns(t *testing.T) {
	st, err := New(filepath.Join(t.TempDir(), "links.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if _, err := st.DB().Exec(`CREATE TABLE links (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		group_title TEXT NOT NULL DEFAULT '', name TEXT NOT NULL DEFAULT '',
		url TEXT NOT NULL DEFAULT '', avatar TEXT NOT NULL DEFAULT '',
		description TEXT NOT NULL DEFAULT '', rss TEXT NOT NULL DEFAULT '',
		label TEXT NOT NULL DEFAULT '', color TEXT NOT NULL DEFAULT '', sort INTEGER NOT NULL DEFAULT 0
	)`); err != nil {
		t.Fatal(err)
	}
	if err := st.EnsureLinksTable(); err != nil {
		t.Fatal(err)
	}
	link := &Link{Name: "legacy", URL: "https://example.com"}
	if _, err := st.CreateLink(link); err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateLinkCheck(link.ID, 204, 123); err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateLinkSnapshot(link.ID, "/uploads/snapshots/1.jpg"); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetLink(link.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Snapshot != "/uploads/snapshots/1.jpg" || got.LastStatus != 204 || got.LastCheckedAt != 123 {
		t.Fatalf("migrated fields not persisted: %+v", got)
	}
	if err := st.EnsureLinksTable(); err != nil {
		t.Fatal(err)
	}
}
