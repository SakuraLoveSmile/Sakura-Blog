package backup

import (
	"archive/zip"
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"

	"blog/internal/stellarv2"
	"blog/internal/store"
)

func TestStellarPlainMarkdownImportStrictAtomicAndDraftMirror(t *testing.T) {
	svc, st, _ := newStellarContentFixture(t)
	admin := &store.User{Username: "admin", Role: store.RoleAdmin, PasswordHash: "x", AuthSecret: "test"}
	if err := st.CreateUser(admin); err != nil {
		t.Fatal(err)
	}
	var bad bytes.Buffer
	writer := zip.NewWriter(&bad)
	for name, source := range map[string]string{"new.md": "---\ntitle: New\n---\n正文", "old.md": "---\ntopic: legacy\n---\n正文"} {
		entry, _ := writer.Create(name)
		_, _ = entry.Write([]byte(source))
	}
	_ = writer.Close()
	_, err := svc.Import("posts.zip", bad.Bytes())
	var schema *stellarv2.SchemaError
	if !errors.As(err, &schema) {
		t.Fatalf("legacy field was not strictly rejected: %v", err)
	}
	if got, _ := st.GetBySlug("new"); got != nil {
		t.Fatal("invalid Markdown batch partially wrote live data")
	}
	result, err := svc.Import("new.md", []byte(";;;\n{\"title\":\"New\",\"blog\":{\"status\":\"draft\"}}\n;;;\n正文\r\n"))
	if err != nil || !reflect.DeepEqual(result.Imported, []string{"new"}) {
		t.Fatalf("strict single Markdown import: %v", err)
	}
	p, _ := st.GetBySlug("new")
	if p == nil || p.Status != store.StatusDraft || p.StellarVersion != 2 || p.ContentMD != "正文\r\n" || !reflect.DeepEqual(p.AuthorIDs, []int64{admin.ID}) {
		t.Fatal("strict single import lost draft, source or explicit admin identity")
	}
	var exported bytes.Buffer
	if err := svc.WriteStellarContentZip(&exported); err != nil {
		t.Fatal(err)
	}
	_, sources, err := ReadStellarContentZip(exported.Bytes())
	if err != nil || !strings.Contains(string(sources["posts/new.md"]), `"status": "draft"`) {
		t.Fatal("standalone Markdown export lost draft mirror")
	}
}

func TestStellarContentRollbackAfterRebuildFailureKeepsAllMetadata(t *testing.T) {
	svc, st, archive := newStellarContentFixture(t)
	note, _ := st.GetNoteBySlug("book", "note")
	note.ContentMD = "当前正文"
	if err := st.UpdateNote(note); err != nil {
		t.Fatal(err)
	}
	before, _ := st.GetNoteByID(note.ID)
	svc.SetRestoreRebuild(func() error { return errors.New("forced rebuild failure") })
	if _, err := svc.Import("content.zip", archive); err == nil {
		t.Fatal("failed rebuild reported success")
	}
	after, _ := st.GetNoteByID(note.ID)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("rebuild rollback did not restore all note metadata/ownership/source")
	}
}
