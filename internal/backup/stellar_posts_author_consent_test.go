package backup

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"blog/internal/config"
	"blog/internal/stellarmigration"
	"blog/internal/store"
)

func TestLegacyUnresolvedAuthorConsentImportsUnmodifiedSource(t *testing.T) {
	svc, st, _ := newStellarContentFixture(t)
	admin := &store.User{Username: "admin", Role: store.RoleAdmin, PasswordHash: "x", AuthSecret: "test"}
	if err := st.CreateUser(admin); err != nil {
		t.Fatal(err)
	}
	if _, err := config.SaveStellarV2(st, map[string]any{}, map[string]any{"data": map[string]any{"authors": map[string]any{"sakura": map[string]any{"name": "Sakura"}}}}); err != nil {
		t.Fatal(err)
	}
	beforePosts, _ := st.List()
	beforeSettings, _ := st.ReadSettingsSnapshot()
	body := "正文\r\n```yaml\nauthor: 保留正文\n```\n末尾  \n"
	source := []byte("---\ntitle: Perfdog\nauthor: 上杉九月\nabstract:\nbanner_img: /banner.png\nhide: false\nindex_img: /cover.png\nmath: false\npassword:\nsticky: 101\nwrong_pass_message: 密码错误\ncomments: true\nstatus: published\n---\n" + body)
	_, err := svc.ImportLegacyMarkdownContext(context.Background(), "Perfdog.md", source)
	var unsupported *stellarmigration.UnsupportedMarkdownParametersError
	if !errors.As(err, &unsupported) {
		t.Fatalf("unresolved author must join the parameter prompt: %v", err)
	}
	paths, selected, other := []string{}, []IgnoredMarkdownParameter{}, []IgnoredMarkdownParameter{}
	for _, parameter := range unsupported.Parameters {
		paths = append(paths, parameter.Path)
		approval := IgnoredMarkdownParameter{Source: parameter.Source, Path: parameter.Path}
		selected = append(selected, approval)
		if parameter.Path == "/author" {
			if parameter.Code != "unresolved_public_author" || !parameter.RequiresDraft || !strings.Contains(parameter.Message, "默认署名") {
				t.Fatalf("author consequence missing: %+v", parameter)
			}
		} else {
			other = append(other, approval)
		}
	}
	if !reflect.DeepEqual(paths, []string{"/abstract", "/author", "/banner_img", "/hide", "/index_img", "/math", "/password", "/sticky", "/wrong_pass_message"}) {
		t.Fatalf("incomplete original parameter prompt: %v", paths)
	}
	_, err = svc.ImportLegacyMarkdownWithOptionsContext(context.Background(), "Perfdog.md", source, nil, other)
	if !errors.As(err, &unsupported) || len(unsupported.Parameters) != 1 || unsupported.Parameters[0].Path != "/author" {
		t.Fatalf("unconfirmed author did not remain in prompt: %v", err)
	}
	afterPosts, _ := st.List()
	if !reflect.DeepEqual(beforePosts, afterPosts) {
		t.Fatal("preflight wrote articles")
	}
	if _, err := svc.ImportLegacyMarkdownWithOptionsContext(context.Background(), "Perfdog.md", source, nil, selected); err != nil {
		t.Fatal(err)
	}
	actual, _ := st.GetBySlug("Perfdog")
	if actual == nil || actual.ContentMD != body || actual.Status != store.StatusDraft || actual.Author != "" || !reflect.DeepEqual(actual.AuthorIDs, []int64{admin.ID}) || !strings.Contains(actual.StellarConfig, `"comments":{"enabled":true}`) || strings.Contains(actual.StellarConfig+actual.BlogExtensions, "上杉九月") {
		t.Fatalf("wrong compatible import: %+v", actual)
	}
	afterSettings, _ := st.ReadSettingsSnapshot()
	if !reflect.DeepEqual(beforeSettings, afterSettings) {
		t.Fatal("compatible import changed author catalog/settings")
	}
	for _, before := range beforePosts {
		after, _ := st.GetBySlug(before.Slug)
		if !reflect.DeepEqual(before, after) {
			t.Fatalf("compatible import changed unrelated article: %s", before.Slug)
		}
	}
}

func TestLegacyAuthorConsentPreservesOwnersAndRejectsStaleApproval(t *testing.T) {
	svc, st, _ := newStellarContentFixture(t)
	before, _ := st.GetBySlug("post")
	source := []byte("---\ntitle: Imported\nauthor: old-author\nstatus: published\n---\nbody")
	approved := []IgnoredMarkdownParameter{{Source: "post.md", Path: "/author"}}
	if _, err := svc.ImportLegacyMarkdownWithOptionsContext(context.Background(), "post.md", source, nil, approved); err != nil {
		t.Fatal(err)
	}
	after, _ := st.GetBySlug("post")
	if after.Status != store.StatusDraft || after.Author != "" || !reflect.DeepEqual(after.AuthorIDs, before.AuthorIDs) || after.DraftJSON != before.DraftJSON {
		t.Fatal("ignored public author changed editing ownership/draft")
	}
	if _, err := config.SaveStellarV2(st, map[string]any{}, map[string]any{"data": map[string]any{"authors": map[string]any{"old-author": map[string]any{"name": "Valid now"}}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ImportLegacyMarkdownWithOptionsContext(context.Background(), "post.md", source, nil, approved); err == nil || !strings.Contains(err.Error(), "无效") {
		t.Fatalf("stale approval erased a now-compatible author: %v", err)
	}
	if _, err := svc.ImportLegacyMarkdownContext(context.Background(), "post.md", source); err != nil {
		t.Fatal(err)
	}
	valid, _ := st.GetBySlug("post")
	if valid.Author != "old-author" || !reflect.DeepEqual(valid.AuthorIDs, before.AuthorIDs) {
		t.Fatalf("valid public author/owners not preserved: %+v", valid)
	}
}
