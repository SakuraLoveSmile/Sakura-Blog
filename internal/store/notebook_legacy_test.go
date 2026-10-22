package store

import (
	"database/sql"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// 与真实 sqlite_master 声明一致，测试只能在 TempDir 的隔离库使用。
const legacyNotebooksDDL = `CREATE TABLE notebooks (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL UNIQUE,
	description TEXT NOT NULL DEFAULT '',
	cover TEXT NOT NULL DEFAULT '',
	nb_order INTEGER NOT NULL DEFAULT 0,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);`
const legacyNotesDDL = `CREATE TABLE notes (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	notebook_id INTEGER NOT NULL,
	title TEXT NOT NULL DEFAULT '',
	content TEXT NOT NULL DEFAULT '',
	content_html TEXT NOT NULL DEFAULT '',
	tags TEXT NOT NULL DEFAULT '',
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (notebook_id) REFERENCES notebooks(id)
);`

func legacyNotebookFixture(t *testing.T, ddls ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "legacy.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, ddl := range ddls {
		if _, err := db.Exec(ddl); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

func TestLegacyNotebookStartupPreservesExactSchemaAndBytes(t *testing.T) {
	path := legacyNotebookFixture(t, legacyNotebooksDDL, legacyNotesDDL,
		`INSERT INTO notebooks(id,name,description,cover,nb_order,created_at) VALUES(2,'原本','保留描述','原封面',7,'2020-01-02 03:04:05');
		INSERT INTO notes(id,notebook_id,title,content,content_html,tags,created_at,updated_at) VALUES(2,2,'原笔记','正文\n{% legacy %}','<p>派生</p>','技术/Go,重复','2020-01-02 03:04:05','2021-02-03 04:05:06');`)
	// 复现此前启动直接执行新 DDL 的失败，避免夹具误用新版表。
	probe, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := probe.Exec(notesDDL); err == nil || !strings.Contains(err.Error(), "status") {
		t.Fatalf("old schema no longer reproduces incompatible index: %v", err)
	}
	probe.Close()
	var original LegacyNotebookSchema
	for repeat := 0; repeat < 3; repeat++ {
		s, err := New(path)
		if err != nil {
			t.Fatalf("ordinary startup %d blocked: %v", repeat, err)
		}
		for _, ensure := range []func() error{s.EnsureSchema, s.EnsureNotebooksTable, s.EnsureNotesTable} {
			if err := ensure(); err != nil {
				t.Fatalf("safe initialization blocked: %v", err)
			}
		}
		report, err := s.DetectLegacyNotebookSchema()
		if err != nil || !report.Required() || !errors.Is(report.Error(), ErrStellarMigrationRequired) {
			t.Fatalf("missing legacy report: %#v %v", report, err)
		}
		if repeat == 0 {
			original = report
		} else if !reflect.DeepEqual(original, report) {
			t.Fatal("repeated startup changed legacy schema")
		}
		if report.Tables[0].Columns["id"] != "INTEGER" || report.Tables[1].Columns["notebook_id"] != "INTEGER" || report.Tables[0].Columns["author_id"] != "" || report.Tables[1].Columns["stellar_version"] != "" {
			t.Fatal("startup altered source identities, ownership or metadata")
		}
		var notebookID, noteID, parent, order int
		var name, body, html, tags, created, updated string
		if err := s.db.QueryRow(`SELECT id,name,nb_order FROM notebooks`).Scan(&notebookID, &name, &order); err != nil {
			t.Fatal(err)
		}
		if err := s.db.QueryRow(`SELECT id,notebook_id,content,content_html,tags,created_at,updated_at FROM notes`).Scan(&noteID, &parent, &body, &html, &tags, &created, &updated); err != nil {
			t.Fatal(err)
		}
		if notebookID != 2 || noteID != 2 || parent != 2 || order != 7 || name != "原本" || body != `正文\n{% legacy %}` || html != "<p>派生</p>" || tags != "技术/Go,重复" || !strings.HasPrefix(created, "2020-01-02") || !strings.HasPrefix(updated, "2021-02-03") {
			t.Fatal("ordinary startup changed legacy records")
		}
		assertLegacyNotebookGates(t, s)
		var count int
		if err := s.db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version=3`).Scan(&count); err != nil || count != 0 {
			t.Fatalf("unconverted notebook ownership migration falsely recorded: %d %v", count, err)
		}
		if err := s.EnsureSettingsTable(); err != nil {
			t.Fatal("legacy layout blocked Console settings")
		}
		if err := s.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func assertLegacyNotebookGates(t *testing.T, s *Store) {
	t.Helper()
	checks := map[string]func() error{
		"get notebook":    func() error { _, err := s.GetNotebook("2"); return err },
		"list notebooks":  func() error { _, err := s.ListNotebooks(); return err },
		"create notebook": func() error { return s.CreateNotebook(&Notebook{ID: "new", StellarVersion: 2, StellarConfig: `{}`}) },
		"update notebook": func() error { return s.UpdateNotebook("2", &Notebook{ID: "2", StellarVersion: 2, StellarConfig: `{}`}) },
		"delete notebook": func() error { return s.DeleteNotebook("2") },
		"get note":        func() error { _, err := s.GetNoteByID(2); return err },
		"get note slug":   func() error { _, err := s.GetNoteBySlug("2", "old"); return err },
		"public note":     func() error { _, err := s.GetPublishedNote("2", "old"); return err },
		"list notes":      func() error { _, err := s.ListNotes("2"); return err },
		"public notes":    func() error { _, err := s.ListPublishedNotes("2"); return err },
		"tag notes":       func() error { _, err := s.ListNotesByTag("2", "go", true); return err },
		"create note": func() error {
			_, err := s.CreateNote(&Note{NotebookID: "2", Slug: "new", StellarVersion: 2, StellarConfig: `{}`})
			return err
		},
		"update note": func() error {
			return s.UpdateNote(&Note{ID: 2, NotebookID: "2", Slug: "old", StellarVersion: 2, StellarConfig: `{}`})
		},
		"delete note": func() error { return s.DeleteNote(2) },
	}
	for name, check := range checks {
		if err := check(); !errors.Is(err, ErrStellarMigrationRequired) || !strings.Contains(err.Error(), "schema:") {
			t.Errorf("%s failed to locate legacy layout: %v", name, err)
		}
	}
}

func TestLegacyNotebookPartialLayoutsDoNotAutoCreateCompanion(t *testing.T) {
	for name, ddls := range map[string][]string{
		"only legacy notebook":  {legacyNotebooksDDL},
		"only legacy note":      {legacyNotesDDL},
		"new notebook old note": {notebooksDDL, legacyNotesDDL},
		"partial note upgrade":  {notebooksDDL, strings.ReplaceAll(strings.ReplaceAll(notesDDL, "\tslug TEXT NOT NULL,\n", ""), "\tUNIQUE(notebook_id,slug),\n", "")},
		"text note id":          {notebooksDDL, strings.ReplaceAll(notesDDL, "id INTEGER PRIMARY KEY AUTOINCREMENT", "id TEXT PRIMARY KEY")},
	} {
		t.Run(name, func(t *testing.T) {
			path := legacyNotebookFixture(t, ddls...)
			s, err := New(path)
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			before, err := s.DetectLegacyNotebookSchema()
			if err != nil || !before.Required() {
				t.Fatalf("partial layout undetected: %#v %v", before, err)
			}
			for repeat := 0; repeat < 2; repeat++ {
				if err := s.EnsureSchema(); err != nil {
					t.Fatal(err)
				}
			}
			after, _ := s.DetectLegacyNotebookSchema()
			if !reflect.DeepEqual(before, after) {
				t.Fatal("partial layout silently upgraded or companion table auto-created")
			}
			assertLegacyNotebookGates(t, s)
		})
	}
}

func TestNotebookNewLayoutAndDeferredLedgerAfterReviewedConversion(t *testing.T) {
	path := legacyNotebookFixture(t, legacyNotebooksDDL, legacyNotesDDL)
	s, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	// 仅隔离夹具模拟审核转换完成；检测器自己绝不执行 DROP/重建。
	if _, err := s.db.Exec(`DROP TABLE notes; DROP TABLE notebooks;`); err != nil {
		t.Fatal(err)
	}
	for repeat := 0; repeat < 2; repeat++ {
		if err := s.EnsureSchema(); err != nil {
			t.Fatal(err)
		}
	}
	report, err := s.DetectLegacyNotebookSchema()
	if err != nil || report.Required() {
		t.Fatalf("new layout rejected: %#v %v", report, err)
	}
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version=3`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("completed ownership schema not recorded once: %d %v", count, err)
	}
	if err := s.CreateNotebook(&Notebook{ID: "new", StellarVersion: 2, StellarConfig: `{}`}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateNote(&Note{NotebookID: "new", Slug: "note", ContentMD: "新正文", StellarVersion: 2, StellarConfig: `{}`}); err != nil {
		t.Fatal(err)
	}
}

func TestLegacyNotebookExistingLedgerDoesNotBypassDetection(t *testing.T) {
	path := legacyNotebookFixture(t, legacyNotebooksDDL, legacyNotesDDL, schemaMigrationsDDL,
		`INSERT INTO schema_migrations(version,name,applied_at) VALUES(3,'notebook-note-ownership','historical');`)
	s, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.EnsureSchema(); err != nil {
		t.Fatal(err)
	}
	assertLegacyNotebookGates(t, s)
	var applied string
	if err := s.db.QueryRow(`SELECT applied_at FROM schema_migrations WHERE version=3`).Scan(&applied); err != nil || applied != "historical" {
		t.Fatal("startup rewrote historical ledger to hide the schema discrepancy")
	}
}
