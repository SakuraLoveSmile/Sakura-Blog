package backup

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"blog/internal/config"
	"blog/internal/stellarv2"
	"blog/internal/store"
)

func TestLegacyMarkdownBatchConvertsExplicitlyAndKeepsHistoryAndOwnership(t *testing.T) {
	svc, st, _ := newStellarContentFixture(t)
	admin := &store.User{Username: "admin", Role: store.RoleAdmin, PasswordHash: "x", AuthSecret: "test"}
	if err := st.CreateUser(admin); err != nil {
		t.Fatal(err)
	}
	previous, _ := st.GetBySlug("post")
	var packed bytes.Buffer
	writer := zip.NewWriter(&packed)
	for name, source := range map[string]string{
		"filename.md": "---\nslug: historical\ntitle: History\nstatus: published\ndate: 2014-03-02 01:02:03\ntopic: Series\npinned: true\n---\n{% tip old %}\nbody\n{% endtip %}",
		"draft.md":    "---\ntitle: New\n---\nbody\n{% image /old.jpg %}",
		"post.md":     ";;;\n{\"title\":\"V2\"}\n;;;\n{% image /same.jpg fancybox:true %}",
	} {
		entry, _ := writer.Create(name)
		_, _ = entry.Write([]byte(source))
	}
	_ = writer.Close()
	result, err := svc.ImportLegacyMarkdownContext(context.Background(), "posts.zip", packed.Bytes())
	if err != nil || len(result.Imported) != 3 {
		t.Fatalf("legacy batch: %v %+v", err, result)
	}
	historical, _ := st.GetBySlug("historical")
	if historical == nil || historical.Status != store.StatusPublished || historical.CreatedAt.Format("2006-01-02 15:04:05") != "2014-03-02 01:02:03" || historical.TopicID == 0 || !strings.Contains(historical.ContentMD, "blog_tip") || !reflect.DeepEqual(historical.AuthorIDs, []int64{admin.ID}) {
		t.Fatalf("lost history or semantics: %+v", historical)
	}
	draft, _ := st.GetBySlug("draft")
	if draft == nil || draft.Status != store.StatusDraft || !strings.Contains(draft.ContentMD, "fancybox:false") {
		t.Fatal("new legacy article lost draft status or old image default")
	}
	after, _ := st.GetBySlug("post")
	if after == nil || after.Status != previous.Status || !reflect.DeepEqual(after.AuthorIDs, previous.AuthorIDs) || after.ContentMD != "{% image /same.jpg fancybox:true %}" {
		t.Fatal("canonical overwrite lost ownership/status/body")
	}
	if value, _ := st.GetBySlug("filename"); value != nil {
		t.Fatal("ignored legacy front matter slug")
	}
}

func TestLegacyMarkdownBatchRejectsDuplicatesAndFailuresAtomically(t *testing.T) {
	for name, sources := range map[string]map[string]string{
		"duplicate":    {"a.md": "---\nslug: shared\n---\na", "b.md": "---\nslug: shared\n---\nb"},
		"unknown":      {"safe.md": "---\npinned: true\n---\nsafe", "bad.md": "---\nmystery: true\n---\nbad"},
		"invalid-date": {"safe.md": "body", "bad.md": "---\nstatus: published\ndate: never\n---\nbad"},
	} {
		t.Run(name, func(t *testing.T) {
			svc, st, _ := newStellarContentFixture(t)
			if err := st.CreateUser(&store.User{Username: "admin", Role: store.RoleAdmin, PasswordHash: "x", AuthSecret: "test"}); err != nil {
				t.Fatal(err)
			}
			before, _ := st.List()
			var packed bytes.Buffer
			writer := zip.NewWriter(&packed)
			for filename, source := range sources {
				entry, _ := writer.Create(filename)
				_, _ = entry.Write([]byte(source))
			}
			_ = writer.Close()
			if _, err := svc.ImportLegacyMarkdownContext(context.Background(), "posts.zip", packed.Bytes()); err == nil {
				t.Fatal("unsafe batch accepted")
			}
			after, _ := st.List()
			if !reflect.DeepEqual(before, after) {
				t.Fatal("failed batch changed live data")
			}
		})
	}
}

func TestLegacyMarkdownSingleNewStoreAndMigrationGate(t *testing.T) {
	svc, st, _ := newTestService(t)
	if err := st.EnsureSchema(); err != nil {
		t.Fatal(err)
	}
	if err := st.EnsurePostAuthorsTable(); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateUser(&store.User{Username: "admin", Role: store.RoleAdmin, PasswordHash: "x", AuthSecret: "test"}); err != nil {
		t.Fatal(err)
	}
	source := []byte("---\nslug: original\nstatus: draft\ndate: 2020-01-02\n---\nbody")
	if _, err := svc.ImportLegacyMarkdownContext(context.Background(), "file.md", source); !errors.Is(err, store.ErrStellarMigrationRequired) {
		t.Fatalf("bypassed old config gate: %v", err)
	}
	if _, err := config.InitializeStellarV2(st, map[string]any{}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ImportLegacyMarkdownContext(context.Background(), "file.md", source); err != nil {
		t.Fatal(err)
	}
	value, _ := st.GetBySlug("original")
	if value == nil || value.Status != store.StatusDraft || !value.CreatedAt.Equal(time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("new store import lost history: %+v", value)
	}
	if _, err := st.DB().Exec("UPDATE posts SET stellar_version=0 WHERE slug='original'"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ImportLegacyMarkdownContext(context.Background(), "file.md", source); !errors.Is(err, store.ErrStellarMigrationRequired) {
		t.Fatalf("overwrote legacy post: %v", err)
	}
}

func TestStellarMarkdownAndZipImportsUseCanonicalLanguage(t *testing.T) {
	svc, st, _ := newStellarContentFixture(t)
	if err := st.CreateUser(&store.User{Username: "admin", Role: store.RoleAdmin, PasswordHash: "x", AuthSecret: "test"}); err != nil {
		t.Fatal(err)
	}
	if _, err := config.SaveStellarV2(st, map[string]any{}, map[string]any{"site": map[string]any{"language": []any{"", "en"}}}); err != nil {
		t.Fatal(err)
	}
	body := "{% mbti INTJ-T 93-72-78-83-51 %}"
	hostSource := []byte("---\ntitle: Host\n---\n" + body)
	pageSource := []byte(";;;\n{\"title\":\"Page\",\"lang\":[\"\",\"zh-TW\"]}\n;;;\n" + body)
	for _, test := range []struct {
		slug, want string
		source     []byte
	}{
		{"builder-host", "Architect", hostSource},
		{"builder-page", "架構師", pageSource},
	} {
		p, err := BuildStellarPost(st, svc.posts, test.slug, test.source)
		if err != nil || !strings.Contains(p.ContentHTML, test.want) {
			t.Fatalf("builder %s language lost: %v", test.slug, err)
		}
	}
	if _, err := svc.Import("page.md", pageSource); err != nil {
		t.Fatal(err)
	}
	var packed bytes.Buffer
	zw := zip.NewWriter(&packed)
	member, err := zw.Create("host.md")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := member.Write(hostSource); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Import("posts.zip", packed.Bytes()); err != nil {
		t.Fatal(err)
	}
	for slug, want := range map[string]string{"page": "架構師", "host": "Architect"} {
		p, err := st.GetBySlug(slug)
		if err != nil || p == nil || !strings.Contains(p.ContentHTML, want) {
			t.Fatalf("import %s language lost: %v", slug, err)
		}
	}
}

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
