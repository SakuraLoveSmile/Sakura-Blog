package app

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"blog/internal/store"
)

func TestBootstrapLegacyNotebookKeepsConsolePreviewAvailable(t *testing.T) {
	root, configPath := stellarAppSeedRoot(t)
	if err := os.MkdirAll(filepath.Join(root, "data"), 0700); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", filepath.Join(root, "data", "blog.db"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE notebooks(id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL UNIQUE, description TEXT NOT NULL DEFAULT '', cover TEXT NOT NULL DEFAULT '', nb_order INTEGER NOT NULL DEFAULT 0, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP);
	CREATE TABLE notes(id INTEGER PRIMARY KEY AUTOINCREMENT, notebook_id INTEGER NOT NULL, title TEXT NOT NULL DEFAULT '', content TEXT NOT NULL DEFAULT '', content_html TEXT NOT NULL DEFAULT '', tags TEXT NOT NULL DEFAULT '', created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, FOREIGN KEY(notebook_id) REFERENCES notebooks(id));
	INSERT INTO notebooks(id,name) VALUES(2,'old');
	INSERT INTO notes(id,notebook_id,content) VALUES(2,2,'original body');`)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	for repeat := 0; repeat < 2; repeat++ {
		services, err := Bootstrap(Options{Root: root, ConfigPath: configPath})
		if err != nil {
			t.Fatalf("ordinary startup blocked by notebook layout: %v", err)
		}
		if services.Identity == nil || services.Settings == nil {
			t.Fatal("legacy layout disabled Console shared services")
		}
		if _, err := services.Store.ReadSettingsSnapshot(); err != nil {
			t.Fatal("Console migration settings snapshot unavailable")
		}
		if _, err := services.Store.ListNotebooks(); !errors.Is(err, store.ErrStellarMigrationRequired) {
			t.Fatalf("legacy CRUD escaped migration gate: %v", err)
		}
		var body string
		if err := services.Store.DB().QueryRow(`SELECT content FROM notes WHERE id=2 AND notebook_id=2`).Scan(&body); err != nil || body != "original body" {
			t.Fatal("ordinary startup changed legacy identity/body")
		}
		if err := services.Close(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
}
