package seed

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"blog/internal/backup"
	"blog/internal/config"
	"blog/internal/post"
	"blog/internal/stellarv2"
	"blog/internal/store"
)

func newStellarSeedFixture(t *testing.T) (*store.Store, *post.Service, string) {
	t.Helper()
	root := t.TempDir()
	st, err := store.New(filepath.Join(root, "blog.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.EnsureSchema(); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateUser(&store.User{Username: "admin", Role: store.RoleAdmin, PasswordHash: "x", AuthSecret: "test"}); err != nil {
		t.Fatal(err)
	}
	if err := st.EnsurePostAuthorsTable(); err != nil {
		t.Fatal(err)
	}
	if _, err := config.InitializeStellarV2(st, map[string]any{}, nil); err != nil {
		t.Fatal(err)
	}
	contentRoot := filepath.Join(root, "content")
	if err := os.MkdirAll(filepath.Join(contentRoot, "posts"), 0700); err != nil {
		t.Fatal(err)
	}
	return st, post.NewService(filepath.Join(contentRoot, "posts")), contentRoot
}

func TestStellarSeedCurrentCanonicalSaveAndDiscover(t *testing.T) {
	st, renderer, root := newStellarSeedFixture(t)
	source := `;;;` + "\n" + `{"title":"原文","leftbar":{"widgets":[{"custom":{"large":900719925474099312345,"false":false,"null":null,"zero":0}}]},"blog":{"status":"draft","poster":{"headline":"海报"}}}` + "\n;;;\n正文\r\n末尾  \n"
	if err := os.WriteFile(filepath.Join(root, "posts", "alpha.md"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	n, err := ImportStellar(st, renderer, root, false)
	if err != nil || n != 1 {
		t.Fatalf("strict seed: %d %v", n, err)
	}
	first, _ := st.GetBySlug("alpha")
	if first.StellarVersion != 2 || first.Status != store.StatusDraft || first.ContentMD != "正文\r\n末尾  \n" || !strings.Contains(first.StellarConfig, "900719925474099312345") || !strings.Contains(first.BlogExtensions, "海报") {
		t.Fatal("canonical source/body/status/parameter bag was changed")
	}
	if n, err := ImportStellar(st, renderer, root, false); err != nil || n != 1 {
		t.Fatalf("explicit strict seed is not repeatable: %d %v", n, err)
	}
	got, _ := st.GetBySlug("alpha")
	if got.ID != first.ID {
		t.Fatal("explicit reseed changed identity")
	}
	got.Title, got.ContentMD = "Console 修改", "独立正文"
	if err := st.Upsert(got); err != nil {
		t.Fatal(err)
	}
	if n, err := ImportStellar(st, renderer, root, true); err != nil || n != 0 {
		t.Fatalf("automatic discovery reimported an existing source: %d %v", n, err)
	}
	preserved, _ := st.GetBySlug("alpha")
	if preserved.Title != "Console 修改" || preserved.ContentMD != "独立正文" {
		t.Fatal("discovery overwrote an existing record")
	}
	if err := os.WriteFile(filepath.Join(root, "posts", "beta.md"), []byte("---\ntitle: 新文章\n---\n新正文"), 0600); err != nil {
		t.Fatal(err)
	}
	if n, err := ImportStellar(st, renderer, root, true); err != nil || n != 1 {
		t.Fatalf("new canonical source not discovered: %d %v", n, err)
	}
}

func TestStellarSeedLegacyConfigurationOrRecordRejectsWithoutWrites(t *testing.T) {
	st, renderer, root := newStellarSeedFixture(t)
	if err := os.WriteFile(filepath.Join(root, "posts", "legacy.md"), []byte("---\ntitle: 原文\ntopic: 旧专栏\n---\n正文"), 0600); err != nil {
		t.Fatal(err)
	}
	before, _ := st.ReadSettingsSnapshot()
	_, err := ImportStellar(st, renderer, root, false)
	var schema *stellarv2.SchemaError
	if !errors.As(err, &schema) {
		t.Fatalf("legacy front matter was accepted: %v", err)
	}
	if value, _ := st.GetBySlug("legacy"); value != nil {
		t.Fatal("legacy source was automatically migrated")
	}
	if err := st.Upsert(&store.Post{Slug: "legacy", Title: "旧记录", ContentMD: "旧正文", Status: store.StatusDraft}); err != nil {
		t.Fatal(err)
	}
	if _, err := ImportStellar(st, renderer, root, false); !errors.Is(err, store.ErrStellarMigrationRequired) {
		t.Fatalf("v0 row did not require migration: %v", err)
	}
	legacy, _ := st.GetBySlug("legacy")
	if legacy.StellarVersion != 0 || legacy.ContentMD != "旧正文" {
		t.Fatal("v0 record changed")
	}
	after, _ := st.ReadSettingsSnapshot()
	if !reflect.DeepEqual(before, after) {
		t.Fatal("failed seed changed configuration")
	}
	if err := st.PutSetting(config.SectionStellarV2, `{"version":0,"theme":{}}`); err != nil {
		t.Fatal(err)
	}
	before, _ = st.ReadSettingsSnapshot()
	if _, err := ImportStellar(st, renderer, root, true); !errors.Is(err, store.ErrStellarMigrationRequired) {
		t.Fatal("unmigrated configuration did not gate discovery")
	}
	after, _ = st.ReadSettingsSnapshot()
	if !reflect.DeepEqual(before, after) {
		t.Fatal("migration gate wrote configuration")
	}
}

func TestStellarSeedInvalidBatchDoesNotPartiallyWrite(t *testing.T) {
	st, renderer, root := newStellarSeedFixture(t)
	for name, source := range map[string]string{"valid.md": "---\ntitle: Valid\n---\nBody", "invalid.md": "---\npinned: true\n---\nOld body"} {
		if err := os.WriteFile(filepath.Join(root, "posts", name), []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := ImportStellar(st, renderer, root, false); err == nil {
		t.Fatal("invalid batch succeeded")
	}
	if posts, _ := st.List(); len(posts) != 0 {
		t.Fatal("invalid batch partially wrote posts")
	}
}

func TestStellarSeedPortableDirectoryKeepsRawBytesAndMissingOnly(t *testing.T) {
	st, renderer, root := newStellarSeedFixture(t)
	owner, _ := st.GetUserByUsername("admin")
	notebook := &store.Notebook{ID: "book", Name: "原本", AuthorID: owner.ID, StellarVersion: 2, StellarConfig: ` {"name":"原本"} `, BlogExtensions: ` {"zero":0,"none":null} `}
	if err := st.CreateNotebook(notebook); err != nil {
		t.Fatal(err)
	}
	for _, slug := range []string{"first", "new"} {
		if _, err := st.CreateNote(&store.Note{NotebookID: "book", Slug: slug, AuthorID: owner.ID, ContentMD: "源正文", Tags: []string{"A/B", "A/B"}, StellarVersion: 2, StellarConfig: ` {"title":"笔记"} `, BlogExtensions: ` {"false":false} `}); err != nil {
			t.Fatal(err)
		}
	}
	service := backup.New(backup.Config{Store: st, BackupDir: filepath.Join(t.TempDir(), "backups")})
	var archive bytes.Buffer
	if err := service.WriteStellarContentZip(&archive); err != nil {
		t.Fatal(err)
	}
	reader, _ := zip.NewReader(bytes.NewReader(archive.Bytes()), int64(archive.Len()))
	for _, member := range reader.File {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, member.Name)), 0700); err != nil {
			t.Fatal(err)
		}
		data, _ := member.Open()
		content, _ := io.ReadAll(data)
		_ = data.Close()
		if err := os.WriteFile(filepath.Join(root, member.Name), content, 0600); err != nil {
			t.Fatal(err)
		}
	}
	first, _ := st.GetNoteBySlug("book", "first")
	first.ContentMD = "Console 编辑"
	if err := st.UpdateNote(first); err != nil {
		t.Fatal(err)
	}
	missing, _ := st.GetNoteBySlug("book", "new")
	if err := st.DeleteNote(missing.ID); err != nil {
		t.Fatal(err)
	}
	if n, err := ImportStellar(st, renderer, root, true); err != nil || n != 1 {
		t.Fatalf("portable missing-only seed: %d %v", n, err)
	}
	kept, _ := st.GetNoteBySlug("book", "first")
	created, _ := st.GetNoteBySlug("book", "new")
	if kept.ContentMD != "Console 编辑" || created.ID != missing.ID || created.StellarConfig != missing.StellarConfig || created.BlogExtensions != missing.BlogExtensions {
		t.Fatal("portable seed overwrote existing note or changed raw metadata/identity")
	}
	if n, err := ImportStellar(st, renderer, root, false); err != nil || n != 3 {
		t.Fatalf("explicit portable seed: %d %v", n, err)
	}
	kept, _ = st.GetNoteBySlug("book", "first")
	if kept.ContentMD != "源正文" {
		t.Fatal("explicit portable seed did not resync canonical source")
	}
}
