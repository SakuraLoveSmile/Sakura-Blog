package stellarmodels

import (
	"blog/internal/config"
	"blog/internal/post"
	"blog/internal/store"
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestJSONNumberArithmeticAndTruth(t *testing.T) {
	for _, raw := range []string{"0", "-0", "0e2"} {
		if truth(json.Number(raw)) {
			t.Errorf("json.Number(%q) must be falsy", raw)
		}
	}
	for _, raw := range []string{"2", "2e1", "-1.5"} {
		if !truth(json.Number(raw)) {
			t.Errorf("json.Number(%q) must be truthy", raw)
		}
	}
	if text(json.Number("2e1")) != "20" || jsString(json.Number("-0")) != "0" || jsString(json.Number("1e21")) != "1e+21" {
		t.Fatal("numeric string coercion did not follow JS double semantics")
	}
	if got := number(json.Number("2e1")); got != 20 {
		t.Errorf("double arithmetic lost number: %v", got)
	}
	if got := number(json.Number("9007199254740993")); got != 9007199254740992 {
		t.Errorf("arithmetic does not follow JS double rounding: %v", got)
	}
	if got := normalizeDate(json.Number("1700000000000")); got != "2023-11-14T22:13:20.000Z" {
		t.Errorf("numeric timestamp lost: %v", got)
	}
	for _, raw := range []string{"1e400", "8640000000000001"} {
		if got := normalizeDate(json.Number(raw)); got != nil {
			t.Errorf("out-of-range JS date %s accepted: %v", raw, got)
		}
	}
	if got := normalizeDate(json.Number("8640000000000000")); got != "+275760-09-13T00:00:00.000Z" {
		t.Errorf("expanded ISO date changed: %v", got)
	}
	if got := normalizeDate(json.Number("-1.9")); got != "1969-12-31T23:59:59.999Z" {
		t.Errorf("timestamp does not truncate toward zero: %v", got)
	}
}

// 实际 Settings 保存/重开与 JSON Front Matter 导入链使用 UseNumber；
// 计算遵循 JS double，配置本身与开放参数袋的大整数保持原值。
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
	if _, err = config.InitializeStellarV2(st, Object{}, nil); err != nil {
		t.Fatal(err)
	}
	raw := Object{"article": Object{"related_posts_limit": json.Number("2"), "listing": Object{"excerpt_length": json.Number("12")}}, "comments": Object{"provider": "giscus", "giscus": Object{"custom_large": json.Number("9007199254740993")}}}
	if _, err = config.SaveStellarV2(st, raw, Object{"custom_large": json.Number("9007199254740993")}); err != nil {
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
	if got := get(saved.RawTheme, "article.listing.excerpt_length"); got != json.Number("12") {
		t.Fatalf("saved raw numeric representation changed: %#v", got)
	}
	if got := get(saved.Theme, "article.relatedPostsLimit"); got != json.Number("2") {
		t.Fatalf("strict runtime numeric representation changed: %#v", got)
	}
	if got := saved.Blog["custom_large"]; got != json.Number("9007199254740993") {
		t.Fatalf("Blog bag lost large integer: %#v", got)
	}
	doc, err := post.ParseStellarDocument([]byte(";;;\n{\"title\":\"Numeric\",\"date\":1700000000000,\"updated\":1700000001000,\"listing\":{\"priority\":3},\"comments\":{\"options\":{\"counter\":9007199254740993}}}\n;;;\nabcdefghijklmnopqrstuvwxyz"), post.StellarDocumentOptions{Source: "posts/numeric.md", Profile: "post"})
	if err != nil {
		t.Fatal(err)
	}
	page := PageModelInput(Object{"source": "posts/numeric.md", "path": "numeric/", "permalink": "https://site.example/numeric/", "content": doc.Body}, doc.Page)
	if page["date"] != json.Number("1700000000000") {
		t.Fatalf("snapshot changed raw numeric date: %#v", page["date"])
	}
	model, err := BuildPage(Input{Theme: saved.Theme, FrontMatter: doc.Page, Page: page})
	if err != nil {
		t.Fatal(err)
	}
	if get(model.Render, "listing.excerpt") != "abcdefghi..." {
		t.Fatalf("saved excerpt limit ignored: %v", get(model.Render, "listing.excerpt"))
	}
	if get(model.Render, "article.related.enabled") != true || get(model.Render, "article.related.maxCount") != json.Number("2") {
		t.Fatalf("saved related count ignored: %#v", get(model.Render, "article.related"))
	}
	if model.Item["date"] != "2023-11-14T22:13:20.000Z" || model.Item["updated"] != "2023-11-14T22:13:21.000Z" {
		t.Fatalf("imported numeric dates lost: %#v", model.Item)
	}
	if get(model.Item, "listing.priority") != json.Number("3") {
		t.Fatalf("priority numeric representation changed: %#v", get(model.Item, "listing.priority"))
	}
	for _, key := range []string{"custom_large", "counter"} {
		if get(model.Render, "article.comments.options."+key) != json.Number("9007199254740993") {
			t.Errorf("model open bag %s lost large integer: %#v", key, get(model.Render, "article.comments.options."+key))
		}
	}
	if get(doc.Raw, "comments.options.counter") != json.Number("9007199254740993") || get(saved.RawTheme, "comments.giscus.custom_large") != json.Number("9007199254740993") {
		t.Fatal("model arithmetic changed persisted inputs")
	}
}
