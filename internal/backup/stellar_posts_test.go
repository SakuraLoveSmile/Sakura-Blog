package backup

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"blog/internal/config"
	"blog/internal/stellarv2"
	"blog/internal/store"
)

func TestLegacyMarkdownMissingTopicsRequireApprovalAndPreserveRealHeader(t *testing.T) {
	svc, st, _ := newStellarContentFixture(t)
	if err := st.CreateUser(&store.User{Username: "admin", Role: store.RoleAdmin, PasswordHash: "x", AuthSecret: "test"}); err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile("../stellarmigration/testdata/teamspeak-legacy.md")
	if err != nil {
		t.Fatal(err)
	}
	before, _ := st.ListTopics()
	_, err = svc.ImportLegacyMarkdownContext(context.Background(), "TeamSpeak使用指南.md", source)
	var missing *MissingLegacyTopicsError
	if !errors.As(err, &missing) || !reflect.DeepEqual(missing.Topics, []string{"zero_tech"}) {
		t.Fatalf("missing concrete topic resolution: %v", err)
	}
	if after, _ := st.ListTopics(); !reflect.DeepEqual(before, after) {
		t.Fatal("missing topic preflight created a topic")
	}
	if value, _ := st.GetBySlug("TeamSpeak使用指南"); value != nil {
		t.Fatal("missing topic preflight wrote a post")
	}
	if _, err := svc.ImportLegacyMarkdownWithTopicsContext(context.Background(), "TeamSpeak使用指南.md", source, []string{"zero_tech"}); err != nil {
		t.Fatal(err)
	}
	topic, _ := st.GetTopicBySlug("zero_tech")
	value, _ := st.GetBySlug("TeamSpeak使用指南")
	if topic == nil || topic.Name != "zero_tech" || topic.StellarVersion != 2 || value == nil || value.TopicID != topic.ID || value.Status != store.StatusDraft {
		t.Fatalf("topic and draft were not imported together: topic=%+v post=%+v", topic, value)
	}
	var raw, blog map[string]any
	_ = json.Unmarshal([]byte(value.StellarConfig), &raw)
	_ = json.Unmarshal([]byte(value.BlogExtensions), &blog)
	for key, want := range map[string]any{
		"title": "TeamSpeak使用指南", "abbrlink": "TeamSpeak", "cover": "../../assets/cover/TeamSpeak使用指南.jpg",
		"tags": []any{"零基础", "游戏"}, "categories": []any{"技术分享"},
		"collection": map[string]any{"profile": "topic", "id": "zero_tech"},
		"render":     map[string]any{"math": false, "diagrams": false},
		"article":    map[string]any{"style": "tech"},
		"footer":     map[string]any{"references": []any{"[TeamSpeak官网](https://teamspeak.com/zh-CN/)", "[TeamSpeak相关文档](https://teamspeak.app/)"}},
	} {
		if !reflect.DeepEqual(raw[key], want) {
			t.Errorf("%s=%#v, want %#v", key, raw[key], want)
		}
	}
	if value.CreatedAt.Format("2006-01-02 15:04:05") != "2023-12-23 00:30:03" || !strings.Contains(value.ContentMD, "topic: 示例代码原样保留") || blog["content"].(map[string]any)["banner_image"] != "https://bu.dusays.com/2024/09/01/66d42682cd240.jpg" {
		t.Fatal("real header lost dates, Markdown or banner")
	}
}

func TestLegacyMarkdownApprovedTopicsAreAtomicAndDeduplicated(t *testing.T) {
	for _, invalid := range []bool{true, false} {
		t.Run(fmt.Sprint(invalid), func(t *testing.T) {
			svc, st, _ := newStellarContentFixture(t)
			if err := st.CreateUser(&store.User{Username: "admin", Role: store.RoleAdmin, PasswordHash: "x", AuthSecret: "test"}); err != nil {
				t.Fatal(err)
			}
			beforeTopics, _ := st.ListTopics()
			beforePosts, _ := st.List()
			var packed bytes.Buffer
			writer := zip.NewWriter(&packed)
			for _, filename := range []string{"one.md", "two.md"} {
				entry, _ := writer.Create(filename)
				header := "topic: zero_tech"
				if invalid && filename == "two.md" {
					header += "\nmystery: true"
				}
				_, _ = entry.Write([]byte("---\n" + header + "\n---\n原文"))
			}
			_ = writer.Close()
			_, err := svc.ImportLegacyMarkdownWithTopicsContext(context.Background(), "posts.zip", packed.Bytes(), []string{"zero_tech"})
			if invalid {
				if err == nil {
					t.Fatal("invalid batch succeeded")
				}
				if after, _ := st.ListTopics(); !reflect.DeepEqual(beforeTopics, after) {
					t.Fatal("failed batch left a new topic")
				}
				if after, _ := st.List(); !reflect.DeepEqual(beforePosts, after) {
					t.Fatal("failed batch changed posts")
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				after, _ := st.ListTopics()
				if len(after) != len(beforeTopics)+1 {
					t.Fatal("duplicate references created multiple topics")
				}
				topic, _ := st.GetTopicBySlug("zero_tech")
				for _, slug := range []string{"one", "two"} {
					value, _ := st.GetBySlug(slug)
					if value == nil || value.TopicID != topic.ID {
						t.Fatal("batch lost shared topic assignment")
					}
				}
			}
		})
	}
}

func TestLegacyMarkdownApprovedTopicRejectsConcurrentNameConflict(t *testing.T) {
	svc, st, _ := newStellarContentFixture(t)
	if err := st.CreateUser(&store.User{Username: "admin", Role: store.RoleAdmin, PasswordHash: "x", AuthSecret: "test"}); err != nil {
		t.Fatal(err)
	}
	original := &store.Topic{Name: "Old", Slug: "old", Description: "必须保留的专栏", StellarVersion: 2, StellarConfig: `{"name":"Old"}`, BlogExtensions: `{"custom":true}`}
	if err := st.CreateTopic(original); err != nil {
		t.Fatal(err)
	}
	beforePosts, _ := st.List()
	var concurrent *store.Topic
	// 在预检快照完成后、维护窗口重建候选前，模拟另一个已进入请求的最后写入。
	svc.SetOperationLock(func() (func(), error) {
		changed := *original
		changed.Name = "zero_tech"
		if err := st.UpdateTopic(&changed); err != nil {
			return nil, err
		}
		var err error
		concurrent, err = st.GetTopicByID(original.ID)
		return func() {}, err
	})
	_, err := svc.ImportLegacyMarkdownWithTopicsContext(context.Background(), "new.md", []byte("---\ntopic: zero_tech\n---\n待导入正文"), []string{"zero_tech"})
	if err == nil {
		t.Fatalf("concurrent topic name conflict was not rejected: %v", err)
	}
	if concurrent == nil {
		t.Fatal("concurrent write did not run after preflight")
	}
	if after, _ := st.GetTopicByID(original.ID); !reflect.DeepEqual(concurrent, after) {
		t.Fatalf("import overwrote the concurrent topic: before=%+v after=%+v", concurrent, after)
	}
	if topic, _ := st.GetTopicBySlug("zero_tech"); topic != nil {
		t.Fatal("failed import created a conflicting topic")
	}
	if after, _ := st.List(); !reflect.DeepEqual(beforePosts, after) {
		t.Fatal("failed import changed posts")
	}
}

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
		"filename.md": "---\nslug: historical\ntitle: History\nstatus: published\ndate: 2014-03-02 01:02:03\ntopic: Series\npinned: true\nmathjax: true\nmermaid: true\ntype: tech\nreferences: [https://example.com/ref]\nbanner: /old.jpg\n---\n{% tip old %}\nbody\n{% endtip %}",
		"draft.md":    "---\ntitle: New\ntopic: ''\n---\nbody\n{% image /old.jpg %}",
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
	var raw, blog map[string]any
	if err := json.Unmarshal([]byte(historical.StellarConfig), &raw); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(historical.BlogExtensions), &blog); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(raw["render"], map[string]any{"math": "mathjax", "diagrams": "mermaid"}) ||
		!reflect.DeepEqual(raw["article"], map[string]any{"style": "tech"}) ||
		!reflect.DeepEqual(raw["footer"], map[string]any{"references": []any{"https://example.com/ref"}}) ||
		!reflect.DeepEqual(blog["content"], map[string]any{"banner_image": "/old.jpg"}) {
		t.Fatalf("legacy fields did not survive persistence: raw=%#v blog=%#v", raw, blog)
	}
	draft, _ := st.GetBySlug("draft")
	if draft == nil || draft.Status != store.StatusDraft || draft.TopicID != 0 || strings.Contains(draft.StellarConfig, "collection") || !strings.Contains(draft.ContentMD, "fancybox:false") {
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
		"duplicate":       {"a.md": "---\nslug: shared\n---\na", "b.md": "---\nslug: shared\n---\nb"},
		"unknown":         {"safe.md": "---\npinned: true\n---\nsafe", "bad.md": "---\nmystery: true\n---\nbad"},
		"invalid-date":    {"safe.md": "body", "bad.md": "---\nstatus: published\ndate: never\n---\nbad"},
		"invalid-mathjax": {"post.md": "---\nmathjax: true\n---\nreplacement", "bad.md": "---\nmathjax: 'true'\n---\nbad"},
		"invalid-mermaid": {"post.md": "---\nmermaid: true\n---\nreplacement", "bad.md": "---\nmermaid: 1\n---\nbad"},
		"invalid-style":   {"post.md": "---\ntype: story\n---\nreplacement", "bad.md": "---\ntype: other\n---\nbad"},
		"render-conflict": {"post.md": "---\nmathjax: false\n---\nreplacement", "bad.md": "---\nmermaid: true\nrender: false\n---\nbad"},
		"style-conflict":  {"post.md": "---\ntype: tech\n---\nreplacement", "bad.md": "---\ntype: story\narticle: {style: tech}\n---\nbad"},
		"unknown-topic":   {"post.md": "---\ntopic:\n---\nreplacement", "bad.md": "---\ntopic: Missing\n---\nbad"},
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
	source := []byte("---\nslug: original\nstatus: draft\ndate: 2020-01-02\ntopic:\n---\nbody")
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
	if value == nil || value.Status != store.StatusDraft || value.TopicID != 0 || strings.Contains(value.StellarConfig, "collection") || !value.CreatedAt.Equal(time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC)) {
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
