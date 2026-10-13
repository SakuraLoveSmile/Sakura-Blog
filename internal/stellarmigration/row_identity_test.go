package stellarmigration

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"testing"
)

func TestSparseRowIDsAndExternalContentFTSRemainLinked(t *testing.T) {
	source := fixture(t, `CREATE TABLE external_docs(body);INSERT INTO external_docs(rowid,body) VALUES(7,'zebra searchable'),(90,'alpha searchable');
CREATE VIRTUAL TABLE external_fts USING fts5(body,content='external_docs');INSERT INTO external_fts(rowid,body) SELECT rowid,body FROM external_docs;
CREATE TABLE alias_docs(rowid TEXT,_rowid_ TEXT,body);INSERT INTO alias_docs(oid,rowid,_rowid_,body) VALUES(42,'named rowid','named underscore','aliases');
CREATE TABLE keyed_docs(id TEXT PRIMARY KEY,body) WITHOUT ROWID;INSERT INTO keyed_docs VALUES('key','without');`)
	s := snapshot(t, source, nil)
	if got := s.Tables["external_docs"].RowIDs; len(got) != 2 || got[0] != json.Number("7") || got[1] != json.Number("90") {
		t.Fatal("sparse rowids were not captured")
	}
	if !s.Tables["keyed_docs"].WithoutRowID || len(s.Tables["keyed_docs"].RowIDs) != 0 {
		t.Fatal("WITHOUT ROWID table was given synthetic identity")
	}
	copy := clone(s)
	if err := copy.Verify(); err != nil {
		t.Fatal(err)
	}
	db := restoreTestSnapshot(t, copy)
	var body string
	var id int64
	if err := db.QueryRow(`SELECT rowid,body FROM external_fts WHERE external_fts MATCH 'zebra'`).Scan(&id, &body); err != nil || id != 7 || body != "zebra searchable" {
		t.Fatalf("external FTS association changed: id=%d body=%q err=%v", id, body, err)
	}
	if err := db.QueryRow(`SELECT rowid,body FROM external_docs WHERE rowid=90`).Scan(&id, &body); err != nil || id != 90 || body != "alpha searchable" {
		t.Fatal("sparse ordinary rowid was reassigned")
	}
	if err := db.QueryRow(`SELECT oid FROM alias_docs WHERE rowid='named rowid' AND _rowid_='named underscore'`).Scan(&id); err != nil || id != 42 {
		t.Fatal("shadowed aliases changed ordinary row identity")
	}
	if err := db.QueryRow(`SELECT body FROM keyed_docs WHERE id='key'`).Scan(&body); err != nil || body != "without" {
		t.Fatal("WITHOUT ROWID restoration failed")
	}
	// 内容不变，仅改变隐藏rowid也必须使源摘要变化。
	writable, err := sql.Open("sqlite", source)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = writable.Exec(`UPDATE alias_docs SET oid=77 WHERE oid=42`); err != nil {
		t.Fatal(err)
	}
	writable.Close()
	next := snapshot(t, source, nil)
	if next.Digest == s.Digest {
		t.Fatal("rowid-only change was invisible to SourceDigest")
	}
}

func TestStorageClassesPreserveEqualIntegerAndRealValues(t *testing.T) {
	source := fixture(t, `CREATE TABLE numeric_values(value);INSERT INTO numeric_values(rowid,value) VALUES(7,1),(90,1.0);
CREATE TABLE declared_values(flag BOOLEAN,date TIMESTAMP,payload BLOB);INSERT INTO declared_values VALUES(1,'2026-10-13 09:00:00','legitimate TEXT in BLOB affinity');`)
	s := snapshot(t, source, nil)
	if s.Tables["numeric_values"].StorageClasses[0]["value"] != "integer" || s.Tables["numeric_values"].StorageClasses[1]["value"] != "real" {
		t.Fatal("equal INTEGER/REAL lost source storage classes")
	}
	db := restoreTestSnapshot(t, clone(s))
	var integerClass, realClass string
	if err := db.QueryRow(`SELECT typeof(value) FROM numeric_values WHERE rowid=7`).Scan(&integerClass); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT typeof(value) FROM numeric_values WHERE rowid=90`).Scan(&realClass); err != nil {
		t.Fatal(err)
	}
	if integerClass != "integer" || realClass != "real" {
		t.Fatalf("SQL binding changed classes: %s/%s", integerClass, realClass)
	}
	var flagClass, dateClass, date, payloadClass, payload string
	if err := db.QueryRow(`SELECT typeof(flag),typeof(date),+date,typeof(payload),+payload FROM declared_values`).Scan(&flagClass, &dateClass, &date, &payloadClass, &payload); err != nil {
		t.Fatal(err)
	}
	if flagClass != "integer" || dateClass != "text" || date != "2026-10-13 09:00:00" || payloadClass != "text" || payload != "legitimate TEXT in BLOB affinity" {
		t.Fatal("driver declaration coercion or affinity changed source storage values")
	}
	writable, err := sql.Open("sqlite", source)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = writable.Exec(`UPDATE numeric_values SET value=1.0 WHERE rowid=7`); err != nil {
		t.Fatal(err)
	}
	writable.Close()
	if snapshot(t, source, nil).Digest == s.Digest {
		t.Fatal("storage-class-only change was invisible to SourceDigest")
	}
}

func TestInaccessibleRowIDAndMissingIdentityMetadataAreRejected(t *testing.T) {
	source := fixture(t, `CREATE TABLE obscured(rowid,_rowid_,oid);INSERT INTO obscured VALUES('a','b','c');`)
	if _, err := ReadSnapshot(context.Background(), source, nil); err == nil {
		t.Fatal("inaccessible hidden rowid was silently discarded")
	}
	s := snapshot(t, fixture(t, `CREATE TABLE items(value);INSERT INTO items(rowid,value) VALUES(7,1.0);`), nil)
	for name, change := range map[string]func(*Snapshot){
		"format2":         func(s *Snapshot) { s.FormatVersion = 2 },
		"missing rowids":  func(s *Snapshot) { table := s.Tables["items"]; table.RowIDs = nil; s.Tables["items"] = table },
		"missing classes": func(s *Snapshot) { table := s.Tables["items"]; table.StorageClasses = nil; s.Tables["items"] = table },
		"invalid class":   func(s *Snapshot) { s.Tables["items"].StorageClasses[0]["value"] = "text" },
	} {
		t.Run(name, func(t *testing.T) {
			input := clone(s)
			change(&input)
			input.Digest = input.SourceDigest()
			c := candidate(t, input)
			f, err := BuildMigrationFixture(context.Background(), c, approveFixture(c), NotebookStructureDecisions{})
			if err == nil || f.Directory != "" {
				t.Fatal("incomplete identity/provenance accepted")
			}
		})
	}
}

func TestCurrentNotebookSparseIdentityIsPreserved(t *testing.T) {
	s := snapshot(t, fixture(t, `CREATE TABLE notebooks(id TEXT PRIMARY KEY,name TEXT,description TEXT,icon TEXT,cover TEXT,sort INTEGER,author_id INTEGER,created_at DATETIME,updated_at DATETIME,stellar_version INTEGER,stellar_config TEXT,blog_extensions TEXT);
INSERT INTO notebooks(rowid,id,name,description,icon,cover,sort,author_id,created_at,updated_at,stellar_version,stellar_config,blog_extensions) VALUES(42,'book','Book','','','',0,0,'2020-01-01 00:00:00','2020-01-01 00:00:00',2,'{"name":"Book","route":{"path":"notebooks/book"}}','{}');`), nil)
	db := restoreTestSnapshot(t, s)
	var rowID int64
	if err := db.QueryRow(`SELECT rowid FROM notebooks WHERE id='book'`).Scan(&rowID); err != nil || rowID != 42 {
		t.Fatal("Notebook schema handling reassigned sparse rowid")
	}
}

func restoreTestSnapshot(t *testing.T, s Snapshot) *sql.DB {
	t.Helper()
	c := candidate(t, s)
	f, err := BuildMigrationFixture(context.Background(), c, approveFixture(c), NotebookStructureDecisions{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(f.Directory) })
	db, err := sql.Open("sqlite", f.DBPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}
