package post

import (
	"blog/internal/stellarv2"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func TestStellarV2OracleComparatorVisitsDocument(t *testing.T) {
	a, b := v2Canonical(t, `<div><span data-value="0">alpha</span></div>`), v2Canonical(t, `<div><span data-value="1">beta</span></div>`)
	if a == "" || b == "" || a == b {
		t.Fatal("DOM comparator failed to observe actual text/attributes")
	}
}
func TestStellarV2OfficialDefaultArguments(t *testing.T) {
	seen := map[string]bool{}
	for _, c := range v2OfficialCases() {
		if seen[c.name] {
			continue
		}
		seen[c.name] = true
		t.Run(c.name, func(t *testing.T) {
			oracle := newV2Oracle(t, StellarRenderContext{Data: map[string]any{"icons": map[string]any{}}})
			entry, _ := shortcodeV2Entry(c.name)
			args, _ := v2Args(c.args)
			keys := map[string]bool{}
			for _, p := range entry.Params {
				if p.Role == ShortcodeParamNamed || p.NamedAlias {
					keys[p.Key] = true
				}
			}
			var minimal []string
			for _, a := range args {
				key, _, named := strings.Cut(a, ":")
				if named && keys[key] && !(c.name == "frame" && key == "img") {
					continue
				}
				minimal = append(minimal, a)
			}
			var quoted []string
			for _, value := range minimal {
				if strings.Contains(value, `"`) {
					quoted = append(quoted, "'"+value+"'")
				} else {
					quoted = append(quoted, `"`+value+`"`)
				}
			}
			raw := strings.Join(quoted, " ")
			got := oracle.r.render(c.name, v2MapArgs(minimal, entry.Params), c.body, raw)
			want := oracle.render(t, c.name, raw, c.body)
			if a, b := v2Canonical(t, got), v2Canonical(t, want); a != b {
				t.Errorf("got %s\nwant %s", a, b)
			}
		})
	}
}
func TestStellarV2EditorRoundTripOfficialParameters(t *testing.T) {
	s := NewService("")
	for i, c := range v2OfficialCases() {
		t.Run(fmt.Sprintf("%03d_%s", i, c.name), func(t *testing.T) {
			entry, _ := shortcodeV2Entry(c.name)
			raw := "{% " + c.name + " " + c.args + " %}"
			if entry.Type == ShortcodeBlock {
				raw += "\n" + c.body + "\n{% end" + c.name + " %}"
			}
			parsed, e := ParseShortcodeForEditStellarV2(raw)
			if e != nil {
				t.Fatal(e)
			}
			if c.name == "timeline" && strings.Contains(c.args, "custom:") {
				if parsed.Editable {
					t.Fatal("unmodelled parameter must remain raw")
				}
				return
			}
			if !parsed.Editable {
				t.Fatalf("expected editable %s", raw)
			}
			serialized, e := SerializeShortcodeEditStellarV2(c.name, parsed.Values, parsed.Present)
			if e != nil {
				t.Fatal(e)
			}
			reopened, e := ParseShortcodeForEditStellarV2(serialized)
			if e != nil {
				t.Fatal(e)
			}
			if !reflect.DeepEqual(parsed.Values, reopened.Values) {
				t.Fatalf("parse/save/reopen fields differ\n%#v\n%#v\n%s", parsed.Values, reopened.Values, serialized)
			}
			got, e := s.RenderStellarV2(serialized, nil)
			if e != nil {
				t.Fatal(e)
			}
			want, e := s.RenderStellarV2(raw, nil)
			if e != nil {
				t.Fatal(e)
			}
			if a, b := v2Canonical(t, got), v2Canonical(t, want); a != b {
				t.Errorf("save changes actual DOM\ngot %s\nwant %s", a, b)
			}
		})
	}
}
func TestStellarV2NestedCodeAndFailureBoundary(t *testing.T) {
	s := NewService("")
	source := "{% box Outer %}\n{% box Inner %}\n{% tip **term** pop:explanation %}\n{% endbox %}\n{% endbox %}"
	got, e := s.RenderStellarV2(source, nil)
	if e != nil {
		t.Fatal(e)
	}
	if strings.Count(got, `class="tag-plugin colorful note"`) != 2 || !strings.Contains(got, `<strong>term</strong>`) {
		t.Fatal(got)
	}
	for _, source := range []string{"`{% box missing %}`", "``{% box missing %}``", "~~~md\n{% box missing %}\n~~~", "````md\n{% box missing %}\n````", "    {% box missing %}"} {
		got, e := s.RenderStellarV2(source, nil)
		if e != nil || strings.Contains(got, `tag-plugin`) {
			t.Fatalf("code expanded: %s %v", got, e)
		}
	}
	for _, source := range []string{"title\n{% box Missing %}", "{% box A %}\n{% folding B %}\n{% endbox %}", "{% endbox %}", "{% box", "{% unknown param:x %}", "{% tip Old %}body{% endtip %}"} {
		if _, e := s.RenderStellarV2(source, nil); e == nil || !strings.Contains(e.Error(), "line") {
			t.Fatalf("malformed known directive must return source location: %s: %v", source, e)
		}
	}
	for _, source := range []string{"{% box A %}body", "{% unknown param:x %}body{% endunknown %}", "{% image /one unknown:x %}"} {
		result, _ := ParseShortcodeForEditStellarV2(source)
		if result.Editable {
			t.Errorf("cannot rebuild complex raw syntax: %s", source)
		}
	}
}
func TestStellarV2EditorPresenceAndBodyWhitespace(t *testing.T) {
	s := NewService("")
	raw := `{% checkbox checked:false color:"" Task %}`
	p, e := ParseShortcodeForEditStellarV2(raw)
	if e != nil {
		t.Fatal(e)
	}
	if p.Values["checked"] != "false" {
		t.Fatal(p.Values)
	}
	serialized, e := SerializeShortcodeEditStellarV2("checkbox", p.Values, p.Present)
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(serialized, "checked:false") {
		t.Fatal(serialized)
	}
	for _, name := range []string{"grid", "box", "folding", "table"} {
		raw := "{% " + name + " %}\n    code\n\n{% end" + name + " %}"
		p, e := ParseShortcodeForEditStellarV2(raw)
		if e != nil {
			t.Fatal(e)
		}
		if p.Values["content"] != "    code\n" {
			t.Fatal(p.Values)
		}
		serialized, e := SerializeShortcodeEditStellarV2(name, p.Values)
		if e != nil {
			t.Fatal(e)
		}
		a, e := s.RenderStellarV2(raw, nil)
		if e != nil {
			t.Fatal(e)
		}
		b, e := s.RenderStellarV2(serialized, nil)
		if e != nil {
			t.Fatal(e)
		}
		if v2Canonical(t, a) != v2Canonical(t, b) {
			t.Fatalf("body indentation changed: %s", serialized)
		}
	}
	// 空首位置字段不能使后续 note 正文变成标题。
	raw, e = SerializeShortcodeEditStellarV2("note", map[string]any{"title": "", "content": "正文"})
	if e != nil {
		t.Fatal(e)
	}
	p, e = ParseShortcodeForEditStellarV2(raw)
	if e != nil || p.Values["title"] != "" || p.Values["content"] != "正文" {
		t.Fatalf("%s %v %#v", raw, e, p)
	}
	for _, values := range []map[string]any{{"text": "a'b\"c"}, {"text": "a\nb"}, {"text": "%}"}, {"text": "fine", "unknown": "x"}} {
		if _, e := SerializeShortcodeEditStellarV2("copy", values); e == nil {
			t.Fatal("unsafe/unmodelled serialization succeeded")
		}
	}
}
func TestStellarV2ContextConcurrentIsolationAndJSONNumbers(t *testing.T) {
	s := NewService("")
	var wg sync.WaitGroup
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			color := fmt.Sprintf("color%d", i)
			raw := map[string]any{"tags": map[string]any{"button": map[string]any{"default_color": color}}, "services": map[string]any{"github_card": map[string]any{"github_readme_stats": map[string]any{"endpoint": "https://example.test/"}}}, "leftbar": map[string]any{"menu_columns": json.Number("3")}}
			parsed, e := stellarv2.ParseTheme(raw, stellarv2.Options{})
			if e != nil {
				t.Error(e)
				return
			}
			theme := parsed.Value
			theme["services"].(map[string]any)["rating"] = map[string]any{"provider": "star_vote", "star_vote": map[string]any{"endpoint": "https://example.test/" + json.Number(fmt.Sprint(i)).String()}}
			got, e := s.RenderStellarV2WithContext(`{% button Label /target %}`+"\n\n"+`{% copy word %}`+"\n\n"+`{% rating id:item %}`, StellarRenderContext{Theme: theme, Page: map[string]any{"count": json.Number("9007199254740993")}})
			if e != nil {
				t.Error(e)
				return
			}
			if !strings.Contains(got, `color="`+color+`"`) || !strings.Contains(got, `id="copy_1"`) || !strings.Contains(got, `data-api="https://example.test/`+fmt.Sprint(i)+`"`) {
				t.Errorf("request leaked context: %s", got)
			}
			if v2String(theme["tags"].(map[string]any)["button"].(map[string]any)["defaultColor"]) != color {
				t.Error("mutated input")
			}
		}(i)
	}
	wg.Wait()
}
func TestStellarV2GistAndGeneratedParameterSafety(t *testing.T) {
	s := NewService("")
	html, e := s.RenderStellarV2(`{% gist octocat/abc123 file:"a b.go" %}`, nil)
	if e != nil {
		t.Fatal(e)
	}
	for _, want := range []string{`data-stellar-gist`, `sandbox="allow-scripts"`, `gist=octocat%2Fabc123`, `file=a+b.go`} {
		if !strings.Contains(html, want) {
			t.Fatal(html)
		}
	}
	if strings.Contains(html, "allow-same-origin") || strings.Contains(html, "<script") {
		t.Fatal(html)
	}
	for _, source := range []string{`{% gist javascript:alert(1) %}`, `{% gist owner/../evil %}`} {
		html, e := s.RenderStellarV2(source, nil)
		if e != nil {
			t.Fatal(e)
		}
		if strings.Contains(html, "<iframe") {
			t.Fatal(html)
		}
	}
	source := `{% button text javascript:alert(1) %}` + "\n\n" + `{% image "java` + "\t" + `script:alert(1)" alt %}` + "\n\n" + `{% icon key style:"background:url(javascript:evil);width:2em" %}` + "\n\n" + `{% mark "<script>alert(1)</script>" tip:"<img onerror=x>" %}`
	got, e := s.RenderStellarV2(source, nil)
	if e != nil {
		t.Fatal(e)
	}
	if strings.Contains(got, `href="javascript:`) || strings.Contains(got, `data-src="java`+"\t") || strings.Contains(got, `<script>`) || strings.Contains(got, `url(javascript`) {
		t.Fatal(got)
	}
	// 原有作者HTML能力不在本修改中悄悄改变；此边界由主agent单独核验/决定。
	got, e = s.RenderStellarV2(`<div data-author="yes">author HTML</div>`, nil)
	if e != nil || !strings.Contains(got, `data-author="yes"`) {
		t.Fatalf("%s %v", got, e)
	}
}
func TestStellarV2ExplicitBlogExtensionNamespace(t *testing.T) {
	s := NewService("")
	source := `{% blog_about %}**local**{% endblog_about %}` + "\n\n" + `{% blog_tip text:Explain %}Term{% endblog_tip %}`
	if _, e := s.RenderStellarV2(source, nil); e == nil {
		t.Fatal("Blog extension implicitly enabled")
	}
	got, e := s.RenderStellarV2WithContext(source, StellarRenderContext{BlogExtensions: true})
	if e != nil || !strings.Contains(got, `data-blog-service="about"`) || !strings.Contains(got, `data-blog-service="tip"`) {
		t.Fatalf("%s %v", got, e)
	}
	for _, e := range ShortcodeCatalogStellarV2() {
		if strings.HasPrefix(e.Name, "blog_") {
			t.Fatal("extension polluted official catalog")
		}
	}
}

func TestStellarV2EditorExplicitEmptyOverridesDefault(t *testing.T) {
	s := NewService("")
	for _, raw := range []string{`{% copy "" %}`, `{% button color:"" Label /target %}`, `{% emoji qq "" %}`, ` {% emoji url:/a.png qq Alt %} `, `{% mark color:"" Text %}`} {
		p, e := ParseShortcodeForEditStellarV2(raw)
		if e != nil || !p.Editable {
			t.Fatalf("%s %v %#v", raw, e, p)
		}
		serialized, e := SerializeShortcodeEditStellarV2(p.Name, p.Values, p.Present)
		if e != nil {
			t.Fatal(e)
		}
		reopened, e := ParseShortcodeForEditStellarV2(serialized)
		if e != nil || !reflect.DeepEqual(p.Present, reopened.Present) {
			t.Fatalf("lost presence %s: %#v %#v", serialized, p, reopened)
		}
		a, e := s.RenderStellarV2(raw, nil)
		if e != nil {
			t.Fatal(e)
		}
		b, e := s.RenderStellarV2(serialized, nil)
		if e != nil {
			t.Fatal(e)
		}
		if v2Canonical(t, a) != v2Canonical(t, b) {
			t.Fatalf("empty override changes rendered behavior: %s", serialized)
		}
	}
}

func TestStellarV2DefaultsAndLocaleAreConsumed(t *testing.T) {
	s := NewService("")
	got, e := s.RenderStellarV2(`{% button Label /target %}`+"\n\n"+`{% emoji smile %}`+"\n\n"+`{% gallery %}`+"\n![a](/one.png)\n{% endgallery %}", nil)
	if e != nil {
		t.Fatal(e)
	}
	for _, want := range []string{`color="theme"`, `blobcat/smile.gif`, `aspect_ratio="square"`} {
		if !strings.Contains(got, want) {
			t.Fatal(got)
		}
	}
	for _, language := range []string{"zh-CN", "zh-TW", "en"} {
		oracle := newV2Oracle(t, StellarRenderContext{Language: language})
		entry, _ := shortcodeV2Entry("mbti")
		raw := "INTJ-T 93-72-78-83-51"
		args, _ := v2Args(raw)
		got := oracle.r.render("mbti", v2MapArgs(args, entry.Params), "", raw)
		want := oracle.render(t, "mbti", raw, "")
		if v2Canonical(t, got) != v2Canonical(t, want) {
			t.Fatalf("locale %s differs", language)
		}
	}
}

func TestStellarV2BodyDepthAndErrorBound(t *testing.T) {
	s := NewService("")
	source := strings.Repeat("{% box %}", 65) + "body" + strings.Repeat("{% endbox %}", 65)
	if _, e := s.RenderStellarV2(source, nil); e == nil || !strings.Contains(e.Error(), "64") {
		t.Fatal(e)
	}
	if _, e := s.RenderStellarV2(strings.Repeat("x", shortcodeV2MaxBody+1), nil); e == nil {
		t.Fatal("unbounded body accepted")
	}
}

func TestStellarV2ConfiguredServicesAndEncodedMarkdownSource(t *testing.T) {
	for _, services := range []map[string]any{
		{"rating": map[string]any{"provider": nil}, "vote": map[string]any{"provider": nil}},
		{"rating": map[string]any{"provider": "star_vote", "star_vote": map[string]any{"endpoint": "https://example.test/rating?a=1&b=2"}}, "vote": map[string]any{"provider": "star_vote", "star_vote": map[string]any{"endpoint": "https://example.test/vote"}}, "github": map[string]any{"rawUrl": "https://mirror.test/github/"}},
	} {
		ctx := StellarRenderContext{Theme: map[string]any{"services": services}}
		for _, c := range []v2OracleCase{{"rating", "id:record Rate", ""}, {"vote", "id:record Vote", ""}, {"md", "https://raw.githubusercontent.com/alice/blog/ref%2Ffeature/nested/my%20file.md wrap:false", ""}} {
			oracle := newV2Oracle(t, ctx)
			entry, _ := shortcodeV2Entry(c.name)
			tokens, _ := v2Args(c.args)
			got := oracle.r.render(c.name, v2MapArgs(tokens, entry.Params), c.body, c.args)
			want := oracle.render(t, c.name, c.args, c.body)
			if v2Canonical(t, got) != v2Canonical(t, want) {
				t.Errorf("service case %s differs\n%s\n%s", c.name, got, want)
			}
		}
	}
}
func TestStellarV2CatalogExamplesAndCopies(t *testing.T) {
	s := NewService("")
	first := ShortcodeCatalogStellarV2()
	for _, e := range first {
		if e.Signature == "" || e.Example == "" {
			t.Errorf("%s missing editor documentation", e.Name)
		}
		if _, err := s.RenderStellarV2(e.Example, nil); err != nil {
			t.Errorf("%s catalog example fails: %v", e.Name, err)
		}
	}
	first[0].Params[0].Key = "mutated"
	second := ShortcodeCatalogStellarV2()
	if second[0].Params[0].Key == "mutated" {
		t.Fatal("registry schema mutated by consumer")
	}
}

func TestStellarV2SerializedNumbersAndFalseAreNotOmitted(t *testing.T) {
	for _, value := range []any{json.Number("0"), json.Number("9007199254740993"), false} {
		raw, e := SerializeShortcodeEditStellarV2("copy", map[string]any{"text": value})
		if e != nil {
			t.Fatal(e)
		}
		p, e := ParseShortcodeForEditStellarV2(raw)
		if e != nil || p.Values["text"] != fmt.Sprint(value) {
			t.Fatalf("%s %#v %v", raw, p, e)
		}
	}
}
