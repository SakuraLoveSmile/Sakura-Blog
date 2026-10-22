package stellarmodels_test

import (
	"blog/internal/config"
	"blog/internal/post"
	models "blog/internal/stellarmodels"
	"blog/internal/stellarv2"
	"blog/internal/store"
	"encoding/json"
	"path/filepath"
	"testing"
)

func contractValue(m map[string]any, path string) any {
	value, _ := stellarv2.Get(m, path)
	return value
}

func TestSavedSettingsAndDocumentNumericModelContract(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "numeric-model.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	if err = st.EnsureSettingsTable(); err != nil {
		t.Fatal(err)
	}
	if _, err = config.InitializeStellarV2(st, models.Object{}, nil); err != nil {
		t.Fatal(err)
	}
	raw := models.Object{"article": models.Object{"related_posts_limit": json.Number("2"), "listing": models.Object{"excerpt_length": json.Number("12")}}, "comments": models.Object{"provider": "giscus", "giscus": models.Object{"custom_large": json.Number("9007199254740993")}}}
	if _, err = config.SaveStellarV2(st, raw, models.Object{"custom_large": json.Number("9007199254740993")}); err != nil {
		t.Fatal(err)
	}
	if err = st.Close(); err != nil {
		t.Fatal(err)
	}
	st, err = store.New(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := config.LoadStellarV2(st)
	if err != nil {
		t.Fatal(err)
	}
	if got := contractValue(saved.RawTheme, "article.listing.excerpt_length"); got != json.Number("12") {
		t.Fatalf("saved raw numeric representation changed: %#v", got)
	}
	if got := contractValue(saved.Theme, "article.relatedPostsLimit"); got != json.Number("2") {
		t.Fatalf("strict runtime numeric representation changed: %#v", got)
	}
	if got := saved.Blog["custom_large"]; got != json.Number("9007199254740993") {
		t.Fatalf("Blog bag lost large integer: %#v", got)
	}
	doc, err := post.ParseStellarDocument([]byte(";;;\n{\"title\":\"Numeric\",\"date\":1700000000000,\"updated\":1700000001000,\"listing\":{\"priority\":3},\"comments\":{\"options\":{\"counter\":9007199254740993}}}\n;;;\nabcdefghijklmnopqrstuvwxyz"), post.StellarDocumentOptions{Source: "posts/numeric.md", Profile: "post"})
	if err != nil {
		t.Fatal(err)
	}
	page := models.PageModelInput(models.Object{"source": "posts/numeric.md", "path": "numeric/", "permalink": "https://site.example/numeric/", "content": doc.Body}, doc.Page)
	if page["date"] != json.Number("1700000000000") {
		t.Fatalf("snapshot changed raw numeric date: %#v", page["date"])
	}
	model, err := models.BuildPage(models.Input{Theme: saved.Theme, FrontMatter: doc.Page, Page: page})
	if err != nil {
		t.Fatal(err)
	}
	if contractValue(model.Render, "listing.excerpt") != "abcdefghi..." {
		t.Fatalf("saved excerpt limit ignored: %v", contractValue(model.Render, "listing.excerpt"))
	}
	if contractValue(model.Render, "article.related.enabled") != true || contractValue(model.Render, "article.related.maxCount") != json.Number("2") {
		t.Fatalf("saved related count ignored: %#v", contractValue(model.Render, "article.related"))
	}
	if model.Item["date"] != "2023-11-14T22:13:20.000Z" || model.Item["updated"] != "2023-11-14T22:13:21.000Z" {
		t.Fatalf("imported numeric dates lost: %#v", model.Item)
	}
	if contractValue(model.Item, "listing.priority") != json.Number("3") {
		t.Fatalf("priority numeric representation changed: %#v", contractValue(model.Item, "listing.priority"))
	}
	for _, key := range []string{"custom_large", "counter"} {
		if contractValue(model.Render, "article.comments.options."+key) != json.Number("9007199254740993") {
			t.Errorf("model open bag %s lost large integer: %#v", key, contractValue(model.Render, "article.comments.options."+key))
		}
	}
	if contractValue(doc.Raw, "comments.options.counter") != json.Number("9007199254740993") || contractValue(saved.RawTheme, "comments.giscus.custom_large") != json.Number("9007199254740993") {
		t.Fatal("model arithmetic changed persisted inputs")
	}
}
