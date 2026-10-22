package stellarmigration

import (
	"database/sql"
	"testing"
	"time"
)

func TestNotebookDateAcceptsFormalDriverGoTimeString(t *testing.T) {
	path := fixture(t, `CREATE TABLE dates(value DATETIME);`)
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, expected := range []time.Time{
		time.Date(2026, 10, 13, 8, 9, 10, 123456000, time.UTC),
		time.Date(2026, 10, 13, 8, 9, 10, 0, time.FixedZone("CST", 8*3600)),
	} {
		value := expected.String()
		if !validNotebookDate(value) {
			t.Fatal("formal driver Go time.String rejected")
		}
		if _, err = db.Exec(`DELETE FROM dates;INSERT INTO dates(value) VALUES(?)`, value); err != nil {
			t.Fatal(err)
		}
		var actual time.Time
		if err = db.QueryRow(`SELECT value FROM dates`).Scan(&actual); err != nil || !actual.Equal(expected) {
			t.Fatal("claimed date format is not actually consumed by formal driver")
		}
	}
	// 在结构预览中保留原TEXT，既不转RFC3339也不生成新时间。
	s := legacyStructureFixture(t)
	value := time.Date(2026, 10, 13, 8, 9, 10, 123456000, time.UTC).String()
	s.Tables["notebooks"].Rows[0]["created_at"] = value
	s.Tables["notes"].Rows[0]["created_at"] = value
	s.Tables["notes"].Rows[0]["updated_at"] = value
	s.Digest = s.SourceDigest()
	p, err := PreviewNotebookStructure(s, structureApprovals(s))
	if err != nil || p.Blocking() || p.Rows["notebooks"][0]["created_at"] != value || p.Rows["notes"][0]["updated_at"] != value {
		t.Fatal("supported Go time.String did not survive structural preview exactly")
	}
}

func TestNotebookDateRejectsMalformedDatesAndNonText(t *testing.T) {
	for _, value := range []string{"", "2026-02-30 08:09:10 +0000 UTC", "2026-10-13 28:09:10 +0000 UTC", "2026-10-13 08:09:10 +9900 UTC", "2026-10-13 08:09:10 +0000 UTC trailing", "true"} {
		if validNotebookDate(value) {
			t.Fatal("malformed date accepted")
		}
	}
	for _, value := range []any{nil, true, 20261013, map[string]any{"date": "2026-10-13"}} {
		s := legacyStructureFixture(t)
		s.Tables["notebooks"].Rows[0]["created_at"] = value
		s.Digest = s.SourceDigest()
		p, err := PreviewNotebookStructure(s, structureApprovals(s))
		if err != nil || !p.Blocking() {
			t.Fatal("non-TEXT date silently coerced")
		}
	}
}
