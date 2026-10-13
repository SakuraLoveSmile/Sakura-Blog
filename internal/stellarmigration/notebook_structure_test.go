package stellarmigration

import (
	"blog/internal/store"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func legacyStructureFixture(t *testing.T) Snapshot {
	t.Helper()
	path := fixture(t, `CREATE TABLE users(id INTEGER PRIMARY KEY);INSERT INTO users VALUES(7);
 CREATE TABLE notebooks(id INTEGER PRIMARY KEY,name TEXT NOT NULL,description TEXT NOT NULL,cover TEXT NOT NULL,nb_order INTEGER NOT NULL,created_at DATETIME NOT NULL,extra TEXT);
 INSERT INTO notebooks VALUES(2,'原本','描述','/cover.png',7,'2020-01-02 03:04:05','{"large":9007199254740993,"empty":null}');
 CREATE TABLE notes(id INTEGER PRIMARY KEY,notebook_id INTEGER NOT NULL,title TEXT NOT NULL,content TEXT NOT NULL,content_html TEXT NOT NULL,tags TEXT NOT NULL,created_at DATETIME NOT NULL,updated_at DATETIME NOT NULL,unknown INTEGER);
 INSERT INTO notes VALUES(9007199254740993,2,'原笔记','正文
{% links %}
','<p>缓存原文</p>','Go/x,,重复,Go/x','2020-01-02 03:04:05','2021-02-03 04:05:06',9007199254740993);`)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := snapshot(t, path, nil)
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("readonly source changed")
	}
	return s
}
func structureApprovals(s Snapshot) NotebookStructureDecisions {
	owner := int64(7)
	slug := "技术/原笔记"
	status := "draft"
	updated := "2020-01-02 03:04:05"
	return NotebookStructureDecisions{SourceDigest: s.Digest, Records: map[string]NotebookRecordMetadata{
		"notebooks/2":            {Approved: true, OwnerID: &owner, UpdatedAt: &updated},
		"notes/9007199254740993": {Approved: true, OwnerID: &owner, Slug: &slug, Status: &status},
	}}
}
func TestNotebookStructureWithoutApprovalIsOnlyPreview(t *testing.T) {
	s := legacyStructureFixture(t)
	before := encode(s)
	p, err := PreviewNotebookStructure(s, NotebookStructureDecisions{})
	if err != nil {
		t.Fatal(err)
	}
	if !p.Blocking() {
		t.Fatal("missing lifecycle and ownership approved implicitly")
	}
	if _, exists := p.Rows["notes"][0]["slug"]; exists {
		t.Fatal("slug invented")
	}
	if _, exists := p.Rows["notes"][0]["status"]; exists {
		t.Fatal("status invented")
	}
	if _, exists := p.Rows["notes"][0]["author_id"]; exists {
		t.Fatal("owner inferred")
	}
	if p.Rows["notebooks"][0]["id"] != "2" || p.Rows["notes"][0]["notebook_id"] != "2" || p.Rows["notes"][0]["id"] != int64(9007199254740993) {
		t.Fatal("source identities lost")
	}
	if before != encode(s) {
		t.Fatal("preview mutated source")
	}
	if out, err := BuildNotebookStructureFixture(context.Background(), s, NotebookStructureDecisions{}); err == nil || out.Directory != "" {
		t.Fatal("unapproved fixture emitted")
	}
	c := candidate(t, s)
	if !hasCode(c.Diagnostics, "legacy_notebook_schema") || !hasCode(c.Records[1].Diagnostics, "note_missing_slug_status") {
		t.Fatal("primary candidate gate removed")
	}
}
func TestNotebookStructureFixtureUsesCurrentSchemaAndPreservesEverything(t *testing.T) {
	s := legacyStructureFixture(t)
	decisions := structureApprovals(s)
	p, err := PreviewNotebookStructure(s, decisions)
	if err != nil || p.Blocking() {
		t.Fatalf("approved structure rejected: %v %#v", err, p.Diagnostics)
	}
	f, err := BuildNotebookStructureFixture(context.Background(), s, decisions)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(f.Directory) })
	for path, want := range map[string]os.FileMode{f.Directory: 0700, f.DBPath: 0600, f.SourcePath: 0600} {
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != want {
			t.Fatalf("private artifact permission: %s %v", path, err)
		}
	}
	st, err := store.OpenExisting(f.DBPath, true)
	if err != nil {
		t.Fatal(err)
	}
	schema, err := st.DetectLegacyNotebookSchema()
	if err != nil || schema.Required() {
		t.Fatalf("fixture current schema mismatch: %v %#v", err, schema)
	}
	book, err := st.GetNotebook("2")
	if err != nil || book == nil {
		t.Fatalf("actual store cannot read notebook: %v", err)
	}
	note, err := st.GetNoteByID(9007199254740993)
	if err != nil || note == nil {
		t.Fatalf("actual store cannot read note: %v", err)
	}
	if book.ID != "2" || book.Sort != 7 || book.AuthorID != 7 || book.Name != "原本" || book.Description != "描述" || book.Cover != "/cover.png" {
		t.Fatal("notebook structural fields changed")
	}
	if note.NotebookID != "2" || note.Slug != "技术/原笔记" || note.Status != "draft" || note.AuthorID != 7 || note.ContentMD != "正文\n{% links %}\n" || note.ContentHTML != "<p>缓存原文</p>" || note.Title != "原笔记" {
		t.Fatal("note body, identity or approved lifecycle changed")
	}
	if !reflect.DeepEqual(note.Tags, []string{"Go/x", "", "重复", "Go/x"}) {
		t.Fatal("tag hierarchy, order, duplicates or empty segment lost")
	}
	if note.CreatedAt.Format("2006-01-02 15:04:05") != "2020-01-02 03:04:05" || note.UpdatedAt.Format("2006-01-02 15:04:05") != "2021-02-03 04:05:06" || !book.UpdatedAt.Equal(book.CreatedAt) {
		t.Fatal("dates invented or changed")
	}
	if book.StellarVersion != 0 || note.StellarVersion != 0 || note.StellarConfig != "" || note.BlogExtensions != "" {
		t.Fatal("structural conversion prematurely marked content migrated")
	}
	published, err := st.ListPublishedNotes("2")
	if err != nil || len(published) != 0 {
		t.Fatal("draft implicitly published")
	}
	if err = st.Close(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(f.SourcePath)
	if err != nil {
		t.Fatal(err)
	}
	archive, err := decode(string(b))
	if err != nil {
		t.Fatal(err)
	}
	tables := object(object(archive)["tables"])
	for _, table := range []string{"notebooks", "notes"} {
		original, err := decode(encode(s.Tables[table]))
		if err != nil || !reflect.DeepEqual(tables[table], original) {
			t.Fatal("full source schema, unknown columns or large values not preserved")
		}
	}
	fresh := snapshot(t, f.DBPath, nil)
	c := candidate(t, fresh)
	if hasCode(c.Diagnostics, "legacy_notebook_schema") {
		t.Fatal("current fixture still requires legacy schema")
	}
	for _, record := range c.Records {
		if hasCode(record.Diagnostics, "note_missing_slug_status") {
			t.Fatal("explicit lifecycle missing in fresh snapshot")
		}
	}
	if err := ValidateCandidate(c, fresh); err != nil {
		t.Fatalf("fresh isolated schema cannot produce a strict content candidate: %v", err)
	}
	// Even after proving a fixture, the original source and its candidate remain blocked.
	original := candidate(t, s)
	if !hasCode(original.Diagnostics, "legacy_notebook_schema") {
		t.Fatal("fixture proof silently resolved source schema choice")
	}
}
func TestNotebookStructureRejectsUnapprovedStaleAndUnsafeDecisions(t *testing.T) {
	s := legacyStructureFixture(t)
	cases := map[string]func(*NotebookStructureDecisions){
		"unapproved": func(d *NotebookStructureDecisions) {
			v := d.Records["notes/9007199254740993"]
			v.Approved = false
			d.Records["notes/9007199254740993"] = v
		},
		"unknown owner": func(d *NotebookStructureDecisions) {
			v := d.Records["notes/9007199254740993"]
			owner := int64(99)
			v.OwnerID = &owner
			d.Records["notes/9007199254740993"] = v
		},
		"negative owner": func(d *NotebookStructureDecisions) {
			v := d.Records["notes/9007199254740993"]
			owner := int64(-1)
			v.OwnerID = &owner
			d.Records["notes/9007199254740993"] = v
		},
		"unsafe slug": func(d *NotebookStructureDecisions) {
			v := d.Records["notes/9007199254740993"]
			slug := "../escape"
			v.Slug = &slug
			d.Records["notes/9007199254740993"] = v
		},
		"status": func(d *NotebookStructureDecisions) {
			v := d.Records["notes/9007199254740993"]
			status := "unknown"
			v.Status = &status
			d.Records["notes/9007199254740993"] = v
		},
		"date": func(d *NotebookStructureDecisions) {
			v := d.Records["notebooks/2"]
			date := "today"
			v.UpdatedAt = &date
			d.Records["notebooks/2"] = v
		},
		"unknown source": func(d *NotebookStructureDecisions) { d.Records["notes/99"] = NotebookRecordMetadata{Approved: true} },
		"collection slug": func(d *NotebookStructureDecisions) {
			v := d.Records["notebooks/2"]
			slug := "invented"
			v.Slug = &slug
			d.Records["notebooks/2"] = v
		},
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			d := structureApprovals(s)
			change(&d)
			p, err := PreviewNotebookStructure(s, d)
			if err != nil || !p.Blocking() {
				t.Fatalf("bad decision accepted: %v", err)
			}
		})
	}
	d := structureApprovals(s)
	d.SourceDigest = "outdated"
	if _, err := PreviewNotebookStructure(s, d); err == nil {
		t.Fatal("stale approvals accepted")
	}
}
func TestNotebookStructureSourceConflictsAndTransactionRollback(t *testing.T) {
	s := legacyStructureFixture(t)
	s.Tables["notes"].Rows[0]["content_md"] = "different"
	s.Digest = s.SourceDigest()
	p, err := PreviewNotebookStructure(s, structureApprovals(s))
	if err != nil || !p.Blocking() {
		t.Fatal("two body sources silently resolved")
	}
	s = legacyStructureFixture(t)
	p, err = PreviewNotebookStructure(s, structureApprovals(s))
	if err != nil || p.Blocking() {
		t.Fatal("valid fixture rejected")
	}
	st, err := store.New(filepath.Join(t.TempDir(), "transaction.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err = st.EnsureNotesTable(); err != nil {
		t.Fatal(err)
	}
	duplicate := clone(p.Rows["notes"][0])
	duplicate["id"] = int64(99)
	p.Rows["notes"] = append(p.Rows["notes"], duplicate)
	tx, err := st.DB().Begin()
	if err != nil {
		t.Fatal(err)
	}
	if err = insertNotebookStructure(context.Background(), tx, p); err == nil {
		t.Fatal("unique slug failure not exercised")
	}
	if err = tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"notebooks", "notes"} {
		var count int
		if err = st.DB().QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&count); err != nil || count != 0 {
			t.Fatal("partial conversion or version marker persisted")
		}
	}
}
func TestNotebookStructureCancelledFixtureIsNotEmitted(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	s := legacyStructureFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	f, err := BuildNotebookStructureFixture(ctx, s, structureApprovals(s))
	if err == nil || f.DBPath != "" {
		t.Fatal("cancelled fixture emitted")
	}
	files, err := os.ReadDir(os.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if strings.HasPrefix(file.Name(), "stellar-notebook-structure-") {
			t.Fatal("failed private fixture retained")
		}
	}
}
func TestNotebookStructureCurrentSchemaNeedsNoInventedMetadata(t *testing.T) {
	path := fixture(t, `CREATE TABLE notebooks(id TEXT PRIMARY KEY,name TEXT,description TEXT,icon TEXT,cover TEXT,sort INTEGER,author_id INTEGER,created_at DATETIME,updated_at DATETIME,stellar_version INTEGER,stellar_config TEXT,blog_extensions TEXT);
 INSERT INTO notebooks VALUES('book','Book','','','',0,0,'2020-01-01 00:00:00','2020-01-01 00:00:00',2,'{"name":"Book","route":{"path":"notebooks/book"}}','{}');
 CREATE TABLE notes(id INTEGER PRIMARY KEY,notebook_id TEXT,slug TEXT,title TEXT,content_md TEXT,content_html TEXT,status TEXT,tags_json TEXT,author_id INTEGER,created_at DATETIME,updated_at DATETIME,stellar_version INTEGER,stellar_config TEXT,blog_extensions TEXT);
 INSERT INTO notes VALUES(5,'book','keep','Keep','body','','published','["a","a"]',0,'2020-01-01 00:00:00','2020-01-01 00:00:00',2,'{"collection":{"profile":"notebook","id":"book"}}','{}');`)
	s := snapshot(t, path, nil)
	p, err := PreviewNotebookStructure(s, NotebookStructureDecisions{})
	if err != nil || p.Blocking() {
		t.Fatalf("modern preserved data rejected: %v %#v", err, p.Diagnostics)
	}
	if p.Rows["notes"][0]["slug"] != "keep" || p.Rows["notes"][0]["stellar_version"] != int64(2) {
		t.Fatal("modern metadata altered")
	}
	owner := int64(8)
	d := NotebookStructureDecisions{SourceDigest: s.Digest, Records: map[string]NotebookRecordMetadata{"notes/5": {Approved: true, OwnerID: &owner}}}
	p, err = PreviewNotebookStructure(s, d)
	if err != nil || !p.Blocking() {
		t.Fatal("existing ownership overwritten during structural conversion")
	}
	// Preserve JSON exactly, including whitespace, rather than reserializing author data.
	s.Tables["notes"].Rows[0]["tags_json"] = "[ \"a\",\"a\" ]"
	s.Digest = s.SourceDigest()
	p, err = PreviewNotebookStructure(s, NotebookStructureDecisions{})
	if err != nil || p.Rows["notes"][0]["tags_json"] != "[ \"a\",\"a\" ]" {
		t.Fatal("existing tags JSON rewritten")
	}
}

func TestNotebookStructureDoesNotHideConflictingOrUnrepresentableSource(t *testing.T) {
	cases := map[string]func(*Snapshot){
		"missing parent":  func(s *Snapshot) { s.Tables["notes"].Rows[0]["notebook_id"] = int64(999) },
		"two sorts":       func(s *Snapshot) { s.Tables["notebooks"].Rows[0]["sort"] = int64(99) },
		"two tag sources": func(s *Snapshot) { s.Tables["notes"].Rows[0]["tags_json"] = `["different"]` },
		"nonstring tag": func(s *Snapshot) {
			delete(s.Tables["notes"].Rows[0], "tags")
			s.Tables["notes"].Rows[0]["tags_json"] = `[4]`
		},
		"nullable old title":   func(s *Snapshot) { s.Tables["notes"].Rows[0]["title"] = nil },
		"unrepresentable blob": func(s *Snapshot) { s.Tables["notes"].Rows[0]["unknown"] = string([]byte{0xff, 0xfe}) },
		"duplicate slug": func(s *Snapshot) {
			extra := clone(s.Tables["notes"].Rows[0])
			extra["id"] = int64(99)
			table := s.Tables["notes"]
			table.Rows = append(table.Rows, extra)
			s.Tables["notes"] = table
		},
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			s := legacyStructureFixture(t)
			change(&s)
			s.Digest = s.SourceDigest()
			d := structureApprovals(s)
			if name == "duplicate slug" {
				d.Records["notes/99"] = d.Records["notes/9007199254740993"]
			}
			p, err := PreviewNotebookStructure(s, d)
			if err != nil || !p.Blocking() {
				t.Fatalf("source conflict lost: %v", err)
			}
			if f, err := BuildNotebookStructureFixture(context.Background(), s, d); err == nil || f.Directory != "" {
				t.Fatal("unrepresentable source emitted a new fixture")
			}
		})
	}
}
func TestNotebookStructureZeroOwnerIsAnExplicitChoice(t *testing.T) {
	s := legacyStructureFixture(t)
	d := structureApprovals(s)
	owner := int64(0)
	for key, metadata := range d.Records {
		metadata.OwnerID = &owner
		d.Records[key] = metadata
	}
	p, err := PreviewNotebookStructure(s, d)
	if err != nil || p.Blocking() {
		t.Fatalf("explicit unowned state rejected: %v", err)
	}
	for _, rows := range p.Rows {
		for _, row := range rows {
			if row["author_id"] != int64(0) {
				t.Fatal("explicit unowned state changed")
			}
		}
	}
	d.Records["notes/9007199254740993"] = NotebookRecordMetadata{Approved: true, Slug: d.Records["notes/9007199254740993"].Slug, Status: d.Records["notes/9007199254740993"].Status}
	p, err = PreviewNotebookStructure(s, d)
	if err != nil || !p.Blocking() {
		t.Fatal("missing owner mistaken for explicitly unowned")
	}
}
