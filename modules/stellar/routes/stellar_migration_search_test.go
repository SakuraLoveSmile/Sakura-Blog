package routes

import (
	"context"
	"strings"
	"testing"
	"time"

	"blog/internal/search"
	"blog/internal/store"
)

func TestMigrationSearchUsesActualCanonicalMarkdownAndPublicSnapshot(t *testing.T) {
	ws, st, _, _ := stellarRouteFixture(t)
	public := createStellarPost(t, st, "current", "")
	if _, err := st.DB().Exec(`UPDATE posts SET content_md='CanonicalNeedle body',content_html='<p>ObsoleteCache</p>' WHERE id=?`, public.ID); err != nil {
		t.Fatal(err)
	}
	protected := createStellarPost(t, st, "protected", "")
	if _, err := st.DB().Exec(`UPDATE posts SET password='password',content_md='PRIVATE_PASSWORD_SENTINEL' WHERE id=?`, protected.ID); err != nil {
		t.Fatal(err)
	}
	draft := createStellarPost(t, st, "draft", "")
	if _, err := st.DB().Exec(`UPDATE posts SET status='draft',content_md='PRIVATE_DRAFT_SENTINEL' WHERE id=?`, draft.ID); err != nil {
		t.Fatal(err)
	}
	createStellarPost(t, st, "nonsearchable", `,"visibility":{"searchable":false}`)
	if err := st.EnsurePagesTable(); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreatePage(&store.Page{Slug: "current-page", Title: "Page", ContentMD: "PageCanonicalNeedle", ContentHTML: "<p>PageObsoleteCache</p>", Status: store.StatusPublished, StellarVersion: 2, StellarConfig: `{"title":"Page"}`, BlogExtensions: "{}"}); err != nil {
		t.Fatal(err)
	}
	if err := st.EnsureMomentsTable(); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateMoment(&store.Moment{ContentMD: "MomentCanonicalNeedle", ContentHTML: "<p>MomentObsoleteCache</p>", Visibility: store.MomentVisibilityPublic, CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateMoment(&store.Moment{ContentMD: "PRIVATE_MOMENT_SENTINEL", Visibility: store.MomentVisibilityPrivate, CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	items, err := RenderToolSearchItems(context.Background(), Dependencies{Site: ws.site, Store: st, Posts: ws.posts})
	if err != nil {
		t.Fatal(err)
	}
	joined := ""
	for _, item := range items {
		joined += item.Slug + " " + item.Content + "\n"
	}
	for _, want := range []string{"CanonicalNeedle", "PageCanonicalNeedle", "MomentCanonicalNeedle"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing current rendered body %s", want)
		}
	}
	for _, secret := range []string{"ObsoleteCache", "PRIVATE_", "nonsearchable", "protected", "draft"} {
		if strings.Contains(joined, secret) {
			t.Fatalf("stale/private indexed: %s", secret)
		}
	}
	if err = ws.searchIdx.Rebuild(items); err != nil {
		t.Fatal(err)
	}
	results, err := ws.searchIdx.Search("CanonicalNeedle", search.TypePost, 20)
	if err != nil || len(results) != 1 || results[0].Slug != "current" {
		t.Fatalf("canonical FTS lookup failed: %+v %v", results, err)
	}
	results, err = ws.searchIdx.Search("ObsoleteCache", "", 20)
	if err != nil || len(results) != 0 {
		t.Fatalf("old FTS cache survived: %+v %v", results, err)
	}
	var oldHTML string
	if err = st.DB().QueryRow(`SELECT content_html FROM posts WHERE id=?`, public.ID).Scan(&oldHTML); err != nil || oldHTML != "<p>ObsoleteCache</p>" {
		t.Fatal("search rendering mutated source HTML cache")
	}
}
