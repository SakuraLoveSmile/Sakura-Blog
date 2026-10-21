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

	"blog/internal/stellarmigration"
	"blog/internal/store"
)

func TestLegacyMarkdownParameterBatchPreflightAndAtomicRetry(t *testing.T) {
	svc, st, _ := newStellarContentFixture(t)
	if err := st.CreateUser(&store.User{Username: "admin", Role: store.RoleAdmin, PasswordHash: "x", AuthSecret: "test"}); err != nil {
		t.Fatal(err)
	}
	beforePosts, _ := st.List()
	beforeTopics, _ := st.ListTopics()
	var packed bytes.Buffer
	writer := zip.NewWriter(&packed)
	for _, item := range [][2]string{{"z.MD", "password: secret\nmusic: {id: 123}\nstatus: published"}, {"a.md", "hide: true\ntopic: new-topic"}} {
		entry, _ := writer.Create(item[0])
		_, _ = entry.Write([]byte("---\n" + item[1] + "\n---\n正文\n```\npassword: keep\n```\n"))
	}
	_ = writer.Close()
	_, err := svc.ImportLegacyMarkdownContext(context.Background(), "posts.zip", packed.Bytes())
	var unsupported *stellarmigration.UnsupportedMarkdownParametersError
	if !errors.As(err, &unsupported) || len(unsupported.Parameters) != 3 {
		t.Fatalf("missing whole-batch issues: %+v %v", unsupported, err)
	}
	if unsupported.Parameters[0].Source != "a.md" || unsupported.Parameters[1].Source != "z.MD" {
		t.Fatalf("source order/names not preserved: %+v", unsupported.Parameters)
	}
	selected := []IgnoredMarkdownParameter{}
	for _, parameter := range unsupported.Parameters {
		selected = append(selected, IgnoredMarkdownParameter{Source: parameter.Source, Path: parameter.Path})
	}
	assertUntouched := func() {
		t.Helper()
		afterPosts, _ := st.List()
		afterTopics, _ := st.ListTopics()
		if !reflect.DeepEqual(beforePosts, afterPosts) || !reflect.DeepEqual(beforeTopics, afterTopics) {
			t.Fatal("preflight wrote real data")
		}
	}
	assertUntouched()
	_, err = svc.ImportLegacyMarkdownWithOptionsContext(context.Background(), "posts.zip", packed.Bytes(), nil, selected[:1])
	if !errors.As(err, &unsupported) || len(unsupported.Parameters) != 2 {
		t.Fatalf("partial authorization skipped unresolved file: %v", err)
	}
	assertUntouched()
	_, err = svc.ImportLegacyMarkdownWithOptionsContext(context.Background(), "posts.zip", packed.Bytes(), nil, selected)
	var missing *MissingLegacyTopicsError
	if !errors.As(err, &missing) || !reflect.DeepEqual(missing.Topics, []string{"new-topic"}) {
		t.Fatalf("missing-topic chain lost: %v", err)
	}
	assertUntouched()
	for _, invalid := range [][]IgnoredMarkdownParameter{
		append(append([]IgnoredMarkdownParameter{}, selected...), selected[0]),
		{{Source: "a.md", Path: "/topic"}}, {{Source: "z.md", Path: "/password"}}, {{Source: "missing.md", Path: "/password"}}, {{Source: "z.MD", Path: "password"}},
	} {
		if _, err := svc.ImportLegacyMarkdownWithOptionsContext(context.Background(), "posts.zip", packed.Bytes(), []string{"new-topic"}, invalid); err == nil {
			t.Fatalf("invalid consent accepted: %+v", invalid)
		}
		assertUntouched()
	}
	result, err := svc.ImportLegacyMarkdownWithOptionsContext(context.Background(), "posts.zip", packed.Bytes(), []string{"new-topic"}, selected)
	if err != nil || len(result.Imported) != 2 {
		t.Fatalf("approved import failed: %+v %v", result, err)
	}
	for _, slug := range []string{"a", "z"} {
		value, _ := st.GetBySlug(slug)
		if value == nil || value.Status != store.StatusDraft || value.ContentMD != "正文\n```\npassword: keep\n```\n" {
			t.Fatalf("incorrect imported post: %+v", value)
		}
		if strings.Contains(value.StellarConfig+value.BlogExtensions, "secret") {
			t.Fatal("ignored password archived")
		}
	}
}

func TestLegacyIgnoredLifecyclePreservesExistingProtectionOwnershipAndDrafts(t *testing.T) {
	svc, st, _ := newStellarContentFixture(t)
	existing, _ := st.GetBySlug("post")
	existing.Password = "existing-host-secret"
	existing.PublishAt = time.Now().Add(time.Hour).UTC().Truncate(time.Second)
	if err := st.Upsert(existing); err != nil {
		t.Fatal(err)
	}
	originalOwners := append([]int64{}, existing.AuthorIDs...)
	source := []byte("---\npassword: source-secret\nstatus: published\ntitle: Replacement\n---\nnew-body")
	_, err := svc.ImportLegacyMarkdownWithOptionsContext(context.Background(), "post.md", source, nil, []IgnoredMarkdownParameter{{Source: "post.md", Path: "/password"}})
	if err != nil {
		t.Fatal(err)
	}
	actual, _ := st.GetBySlug("post")
	if actual.Status != store.StatusDraft || actual.Password != existing.Password || !actual.PublishAt.Equal(existing.PublishAt) || !reflect.DeepEqual(actual.AuthorIDs, originalOwners) || actual.DraftJSON != existing.DraftJSON {
		t.Fatalf("host lifecycle/owners damaged: before=%+v after=%+v", existing, actual)
	}
	if strings.Contains(actual.StellarConfig+actual.BlogExtensions, "source-secret") {
		t.Fatal("source secret persisted")
	}
}

func TestLegacyParameterConsentCannotEraseValidationFailures(t *testing.T) {
	svc, st, _ := newStellarContentFixture(t)
	before, _ := st.GetBySlug("post")
	source := []byte("---\npassword: secret\nrender: {math: 7}\n---\nbody")
	if _, err := svc.ImportLegacyMarkdownWithOptionsContext(context.Background(), "post.md", source, nil, []IgnoredMarkdownParameter{{Source: "post.md", Path: "/password"}}); err == nil {
		t.Fatal("known invalid value accepted")
	}
	after, _ := st.GetBySlug("post")
	if !reflect.DeepEqual(before, after) {
		t.Fatal("failed conversion overwrote existing data")
	}
}

func TestLegacyMarkdownAliasParameterConsentPersistsOnlySelectedRemoval(t *testing.T) {
	svc, st, _ := newStellarContentFixture(t)
	before, _ := st.GetBySlug("post")
	source := []byte("---\ntitle: Alias import\nreferences: [{extra/key: private-value}, '[Known](/known)']\nposter: {headline: Known, extra~key: true}\nlicense: false\nshare: [email]\n---\n正文\n")
	_, err := svc.ImportLegacyMarkdownContext(context.Background(), "post.md", source)
	var unsupported *stellarmigration.UnsupportedMarkdownParametersError
	if !errors.As(err, &unsupported) || len(unsupported.Parameters) != 2 {
		t.Fatalf("alias preflight did not prompt: %+v %v", unsupported, err)
	}
	selected := []IgnoredMarkdownParameter{}
	for _, parameter := range unsupported.Parameters {
		selected = append(selected, IgnoredMarkdownParameter{Source: parameter.Source, Path: parameter.Path})
	}
	_, err = svc.ImportLegacyMarkdownWithOptionsContext(context.Background(), "post.md", source, nil, selected[:1])
	if !errors.As(err, &unsupported) || len(unsupported.Parameters) != 1 {
		t.Fatalf("partial alias consent accepted: %v", err)
	}
	after, _ := st.GetBySlug("post")
	if !reflect.DeepEqual(before, after) {
		t.Fatal("alias preflight wrote existing record")
	}
	if _, err := svc.ImportLegacyMarkdownWithOptionsContext(context.Background(), "post.md", source, nil, selected); err != nil {
		t.Fatal(err)
	}
	after, _ = st.GetBySlug("post")
	if after.ContentMD != "正文\n" || !strings.Contains(after.StellarConfig, `"[Known](/known)"`) || !strings.Contains(after.BlogExtensions, `"headline":"Known"`) || strings.Contains(after.StellarConfig+after.BlogExtensions, "private-value") {
		t.Fatalf("alias import changed retained content: %+v", after)
	}
}
