package backup

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	"blog/internal/config"
	"blog/internal/post"
	"blog/internal/store"
)

func newStellarContentFixture(t *testing.T) (*Service, *store.Store, []byte) {
	t.Helper()
	svc, st, _ := newTestService(t)
	if err := st.EnsureSchema(); err != nil {
		t.Fatal(err)
	}
	if err := st.EnsurePostAuthorsTable(); err != nil {
		t.Fatal(err)
	}
	owner := &store.User{Username: "owner", Role: store.RoleAuthor, PasswordHash: "x", AuthSecret: "test"}
	if err := st.CreateUser(owner); err != nil {
		t.Fatal(err)
	}
	if _, err := config.InitializeStellarV2(st, map[string]any{}, nil); err != nil {
		t.Fatal(err)
	}
	collectionRaw := ` {"name":"原集合","leftbar":{"widgets":[{"custom":{"huge":900719925474099312345,"value":false,"none":null}}]}} `
	pageRaw := ` {"title":"原文","comments":{"enabled":false,"title":null}} `
	blogRaw := ` {"poster":{"headline":"海报"},"huge":900719925474099312345,"value":false,"none":null} `
	topic := &store.Topic{Slug: "series", Name: "Series", StellarVersion: 2, StellarConfig: collectionRaw, BlogExtensions: blogRaw}
	if err := st.CreateTopic(topic); err != nil {
		t.Fatal(err)
	}
	wiki := &store.WikiProject{ID: "wiki", Name: "Wiki", Visible: true, Tags: []string{"one", "two"}, StellarVersion: 2, StellarConfig: collectionRaw, BlogExtensions: blogRaw}
	if err := st.CreateWikiProject(wiki); err != nil {
		t.Fatal(err)
	}
	notebook := &store.Notebook{ID: "book", Name: "Book", AuthorID: owner.ID, StellarVersion: 2, StellarConfig: collectionRaw, BlogExtensions: blogRaw}
	if err := st.CreateNotebook(notebook); err != nil {
		t.Fatal(err)
	}
	p := &store.Post{Slug: "post", Title: "Post", ContentMD: "文章\r\n末尾  \n", Status: store.StatusPublished, TopicID: topic.ID, AuthorIDs: []int64{owner.ID}, Categories: []string{"Tech/Go"}, DraftJSON: ` {"title":"未发布的草稿","false":false} `, StellarVersion: 2, StellarConfig: pageRaw, BlogExtensions: blogRaw}
	if err := st.Upsert(p); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreatePage(&store.Page{Slug: "page", Title: "Page", ContentMD: "页面\n", Status: store.StatusDraft, StellarVersion: 2, StellarConfig: pageRaw, BlogExtensions: blogRaw}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateWikidoc(&store.WikiDoc{ProjectID: wiki.ID, Slug: "intro", Title: "Wiki doc", ContentMD: "文档\n", Status: store.StatusPublished, StellarVersion: 2, StellarConfig: pageRaw, BlogExtensions: blogRaw}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateNote(&store.Note{NotebookID: notebook.ID, Slug: "note", Title: "Note", ContentMD: "笔记\n", Status: store.StatusDraft, AuthorID: owner.ID, Tags: []string{"A/B", "A/B"}, StellarVersion: 2, StellarConfig: pageRaw, BlogExtensions: blogRaw}); err != nil {
		t.Fatal(err)
	}
	svc.posts = post.NewService(t.TempDir())
	var archive bytes.Buffer
	if err := svc.WritePostsZip(&archive); err != nil {
		t.Fatal(err)
	}
	return svc, st, archive.Bytes()
}

func TestStellarContentPortableRejectsLegacyNotebookBeforeExportOrApply(t *testing.T) {
	svc, st, archive := newStellarContentFixture(t)
	manifest, _, err := ReadStellarContentZip(archive)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := st.GetBySlug("post")
	if _, err := st.DB().Exec(`DROP TABLE notes; DROP TABLE notebooks;
	CREATE TABLE notebooks(id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL UNIQUE, description TEXT NOT NULL DEFAULT '', cover TEXT NOT NULL DEFAULT '', nb_order INTEGER NOT NULL DEFAULT 0, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP);
	CREATE TABLE notes(id INTEGER PRIMARY KEY AUTOINCREMENT, notebook_id INTEGER NOT NULL, title TEXT NOT NULL DEFAULT '', content TEXT NOT NULL DEFAULT '', content_html TEXT NOT NULL DEFAULT '', tags TEXT NOT NULL DEFAULT '', created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, FOREIGN KEY(notebook_id) REFERENCES notebooks(id));
	INSERT INTO notebooks(id,name) VALUES(2,'old');
	INSERT INTO notes(id,notebook_id,content) VALUES(2,2,'original body');`); err != nil {
		t.Fatal(err)
	}
	for name, operation := range map[string]func() error{
		"export snapshot":         func() error { return svc.WriteStellarContentZip(io.Discard) },
		"online import preflight": func() error { _, err := svc.Import("content.zip", archive); return err },
		"isolated candidate":      func() error { return ApplyStellarContentManifest(st, manifest, svc.posts) },
		"seed candidate":          func() error { return SeedStellarContentManifest(st, manifest, svc.posts, true) },
	} {
		if err := operation(); !errors.Is(err, store.ErrStellarMigrationRequired) || !strings.Contains(err.Error(), "notebooks schema:") || !strings.Contains(err.Error(), "notes schema:") {
			t.Errorf("%s failed to locate legacy schema: %v", name, err)
		}
	}
	after, _ := st.GetBySlug("post")
	var body string
	if !reflect.DeepEqual(before, after) || st.Maintenance().Active() {
		t.Fatal("failed preflight mutated unrelated content or entered maintenance")
	}
	if err := st.DB().QueryRow(`SELECT content FROM notes WHERE id=2 AND notebook_id=2`).Scan(&body); err != nil || body != "original body" {
		t.Fatal("failed preflight changed legacy source identity/body")
	}
}

func TestStellarContentPortableRerenderUsesPageAndHostLanguage(t *testing.T) {
	svc, st, archive := newStellarContentFixture(t)
	manifest, _, err := ReadStellarContentZip(archive)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := config.SaveStellarV2(st, map[string]any{}, map[string]any{"site": map[string]any{"language": []any{"", "en"}}}); err != nil {
		t.Fatal(err)
	}
	for index := range manifest.Records {
		record := &manifest.Records[index]
		if !stellarDocumentTable(record.Table) {
			continue
		}
		record.Values["content_md"], _ = json.Marshal("{% mbti INTJ-T 93-72-78-83-51 %}")
		if record.Table == "notes" {
			raw, err := stellarRecordObject(*record, "stellar_config")
			if err != nil {
				t.Fatal(err)
			}
			raw["lang"] = []any{"", "zh-TW"}
			encoded, _ := json.Marshal(raw)
			record.Values["stellar_config"], _ = json.Marshal(string(encoded))
		}
	}
	if err := ApplyStellarContentManifest(st, manifest, svc.posts); err != nil {
		t.Fatal(err)
	}
	note, _ := st.GetNoteBySlug("book", "note")
	p, _ := st.GetBySlug("post")
	if !strings.Contains(note.ContentHTML, "架構師") || !strings.Contains(p.ContentHTML, "Architect") {
		t.Fatal("portable rerender ignored page/host language")
	}
}

func TestStellarContentPortableRoundTripAllFamilies(t *testing.T) {
	svc, st, archive := newStellarContentFixture(t)
	manifest, sources, err := ReadStellarContentZip(archive)
	if err != nil || len(sources) != 7 || len(manifest.Authors) != 1 {
		t.Fatalf("complete manifest/source protocol: %d %v", len(sources), err)
	}
	before, _ := st.GetBySlug("post")
	p := *before
	p.Title, p.ContentMD = "改变", "改变"
	if err := st.Upsert(&p); err != nil {
		t.Fatal(err)
	}
	note, _ := st.GetNoteBySlug("book", "note")
	note.ContentMD, note.Tags = "改变", []string{"different"}
	if err := st.UpdateNote(note); err != nil {
		t.Fatal(err)
	}
	result, err := svc.Import("content.zip", archive)
	if err != nil || len(result.Imported) != 7 || len(result.Skipped) != 0 {
		t.Fatalf("atomic portable import: %v", err)
	}
	got, _ := st.GetBySlug("post")
	if got.ID != before.ID || got.ContentMD != before.ContentMD || got.StellarConfig != before.StellarConfig || got.BlogExtensions != before.BlogExtensions || got.DraftJSON != before.DraftJSON || !reflect.DeepEqual(got.AuthorIDs, before.AuthorIDs) || !reflect.DeepEqual(got.Categories, before.Categories) {
		t.Fatal("post source, draft, canonical bytes or relations changed")
	}
	note, _ = st.GetNoteBySlug("book", "note")
	if note.AuthorID != manifest.Authors[0].ID || note.ContentMD != "笔记\n" || !reflect.DeepEqual(note.Tags, []string{"A/B", "A/B"}) {
		t.Fatal("note identity/body/hierarchical tags changed")
	}
	for _, record := range manifest.Records {
		if record.Source == "" {
			continue
		}
		var raw, blog string
		var version int
		var id any
		if record.Table == "notebooks" || record.Table == "wiki_projects" {
			id = stellarRecordText(record, "id")
		} else {
			id, _ = stellarRecordInteger(record, "id")
		}
		if err := st.DB().QueryRow(`SELECT stellar_version,stellar_config,blog_extensions FROM `+stellarQuote(record.Table)+` WHERE id=?`, id).Scan(&version, &raw, &blog); err != nil || version != 2 || raw != stellarRecordText(record, "stellar_config") || blog != stellarRecordText(record, "blog_extensions") {
			t.Fatalf("%s raw metadata did not round-trip: %v", record.Table, err)
		}
	}
	// SQLite 原生备份也保留所有新表/字段，恢复无需重建 JSON。
	var full bytes.Buffer
	if err := svc.WriteZip(&full); err != nil {
		t.Fatal(err)
	}
	note.ContentMD = "快照后改变"
	if err := st.UpdateNote(note); err != nil {
		t.Fatal(err)
	}
	if err := svc.Restore(full.Bytes(), false); err != nil {
		t.Fatal(err)
	}
	note, _ = st.GetNoteBySlug("book", "note")
	if note.ContentMD != "笔记\n" {
		t.Fatal("SQLite full restore lost note")
	}
}

func rewriteStellarArchive(t *testing.T, archive []byte, edit func(map[string][]byte)) []byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	for _, entry := range zr.File {
		reader, _ := entry.Open()
		files[entry.Name], _ = io.ReadAll(reader)
		_ = reader.Close()
	}
	edit(files)
	var result bytes.Buffer
	zw := zip.NewWriter(&result)
	for name, content := range files {
		writer, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = writer.Write(content)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return result.Bytes()
}

func TestStellarContentInvalidArchivesNeverPartiallyWrite(t *testing.T) {
	svc, st, original := newStellarContentFixture(t)
	before, _ := st.GetBySlug("post")
	for _, edit := range []func(map[string][]byte){
		func(files map[string][]byte) { files["../escape.md"] = []byte("escape") },
		func(files map[string][]byte) { delete(files, "posts/post.md") },
		func(files map[string][]byte) { files["posts/post.md"] = []byte("configuration mismatch") },
		func(files map[string][]byte) {
			var manifest StellarContentManifest
			_ = json.Unmarshal(files[StellarContentManifestName], &manifest)
			manifest.Records = append(manifest.Records, manifest.Records[0])
			files[StellarContentManifestName], _ = json.Marshal(manifest)
		},
		func(files map[string][]byte) {
			var manifest StellarContentManifest
			_ = json.Unmarshal(files[StellarContentManifestName], &manifest)
			manifest.Authors[0].Username = "different-account"
			files[StellarContentManifestName], _ = json.Marshal(manifest)
		},
		func(files map[string][]byte) {
			var manifest StellarContentManifest
			_ = json.Unmarshal(files[StellarContentManifestName], &manifest)
			for index, record := range manifest.Records {
				if record.Table == "notes" {
					manifest.Records[index].Values["unknown_column"] = json.RawMessage(`"rejected"`)
				}
			}
			files[StellarContentManifestName], _ = json.Marshal(manifest)
		},
	} {
		invalid := rewriteStellarArchive(t, original, edit)
		if _, err := svc.Import("content.zip", invalid); err == nil {
			t.Fatal("invalid complete archive accepted")
		}
		after, _ := st.GetBySlug("post")
		if !reflect.DeepEqual(before, after) {
			t.Fatal("invalid archive partially modified live data")
		}
	}
}

func TestStellarContentRebuildUsesLatestDatabaseInsideMaintenance(t *testing.T) {
	svc, st, archive := newStellarContentFixture(t)
	release, err := st.Maintenance().Acquire()
	if err != nil {
		t.Fatal(err)
	}
	task, err := svc.StartRestore(context.Background(), RestoreInput{Data: archive, ContentImport: true})
	if err != nil {
		release()
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for !st.Maintenance().Active() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if !st.Maintenance().Active() {
		release()
		t.Fatal("restore did not enter its confirmed maintenance wait")
	}
	// 已进入请求在排空前的最后一次写入，必须出现在提交候选中。
	note, _ := st.GetNoteBySlug("book", "note")
	extra := *note
	extra.ID, extra.Slug, extra.ContentMD = 0, "late", "预检后的合法并发写入"
	if _, err := st.CreateNote(&extra); err != nil {
		release()
		t.Fatal(err)
	}
	release()
	if err := svc.WaitRestore(context.Background(), task.ID); err != nil {
		t.Fatal(err)
	}
	retained, _ := st.GetNoteBySlug("book", "late")
	if retained == nil || !strings.Contains(retained.ContentMD, "合法并发") {
		t.Fatal("preflight snapshot overwrote an in-flight write")
	}
}
