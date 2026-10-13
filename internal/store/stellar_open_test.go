package store

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenExistingNeverCreatesOrMigrates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.db")
	for _, ro := range []bool{true, false} {
		if _, err := OpenExisting(path, ro); err == nil {
			t.Fatal("missing database accepted")
		}
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("missing database was created")
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE legacy(value TEXT); INSERT INTO legacy VALUES('preserved')`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	st, err := OpenExisting(path, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().Exec(`CREATE TABLE forbidden(value TEXT)`); err == nil {
		t.Fatal("read-only store permitted write")
	}
	var count int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("unexpected schema: %d %v", count, err)
	}
	st.Close()
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("read-only opening modified database")
	}
	st, err = OpenExisting(path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	var mode string
	if err := st.DB().QueryRow(`PRAGMA journal_mode`).Scan(&mode); err != nil || mode != "delete" {
		t.Fatalf("journal mode changed: %s %v", mode, err)
	}
}

func TestOpenExistingReadsLatestActiveWAL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wal.sqlite")
	writer, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	if _, err := writer.DB().Exec(`PRAGMA wal_autocheckpoint=0; INSERT INTO posts(slug,title) VALUES('wal','latest uncheckpointed content')`); err != nil {
		t.Fatal(err)
	}
	reader, err := OpenExisting(path, true)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	var title string
	if err := reader.DB().QueryRow(`SELECT title FROM posts WHERE slug='wal'`).Scan(&title); err != nil || title != "latest uncheckpointed content" {
		t.Fatalf("lost active WAL content: %q %v", title, err)
	}
	if _, err := reader.DB().Exec(`UPDATE posts SET title='forbidden'`); err == nil {
		t.Fatal("read-only WAL connection allowed DML")
	}
	if err := writer.DB().QueryRow(`SELECT title FROM posts WHERE slug='wal'`).Scan(&title); err != nil || title != "latest uncheckpointed content" {
		t.Fatalf("changed content: %q %v", title, err)
	}
}
