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
