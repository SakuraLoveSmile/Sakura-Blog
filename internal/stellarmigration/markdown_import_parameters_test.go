package stellarmigration

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"blog/internal/store"
)

func TestMarkdownUnsupportedParametersRequireExactConsentAndPreserveBody(t *testing.T) {
	source := []byte("---\ntitle: Keep\nmusic: {server: netease, id: 123}\nhide: true\nmermaid: false\n---\n正文\n```yaml\npassword: untouched\n```\n")
	_, err := ConvertLegacyMarkdown(source, "old.md", "post", nil)
	var problem *UnsupportedMarkdownParametersError
	if !errors.As(err, &problem) || len(problem.Parameters) != 2 {
		t.Fatalf("missing structured issues: %#v %v", problem, err)
	}
	_, err = ConvertLegacyMarkdownWithIgnoredParameters(source, "old.md", "post", nil, []string{"/music"})
	if !errors.As(err, &problem) || len(problem.Parameters) != 1 || problem.Parameters[0].Path != "/hide" {
		t.Fatalf("partial choice lost remaining issue: %+v %v", problem, err)
	}
	result, err := ConvertLegacyMarkdownWithIgnoredParameters(source, "old.md", "post", nil, []string{"/music", "/hide"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Document.Body != "正文\n```yaml\npassword: untouched\n```\n" || result.Document.Raw["title"] != "Keep" {
		t.Fatal("body/title changed")
	}
	if _, exists := result.Document.Raw["music"]; exists {
		t.Fatal("ignored field returned")
	}
	render, _ := result.Document.Raw["render"].(map[string]any)
	if render["diagrams"] != false {
		t.Fatal("existing supported mapping lost")
	}
	for _, paths := range [][]string{{"/title"}, {"/missing"}, {"/music", "/music"}, {"music"}, {"/music/~2"}} {
		if _, err := ConvertLegacyMarkdownWithIgnoredParameters(source, "old.md", "post", nil, paths); err == nil {
			t.Fatalf("accepted invalid authorization %v", paths)
		}
	}
}

func TestMarkdownIgnoredLifecycleAlwaysDraftAndNeverArchivesValues(t *testing.T) {
	for _, field := range []string{"password", "publish_at", "authors", "draft_json"} {
		for _, host := range []bool{false, true} {
			for _, value := range []string{"null", `"private-secret-value"`} {
				t.Run(field+value+map[bool]string{true: "host", false: "direct"}[host], func(t *testing.T) {
					raw := map[string]any{"title": "Keep", "blog": map[string]any{"status": "published", "custom": map[string]any{"keep": true}}}
					var decoded any
					_ = json.Unmarshal([]byte(value), &decoded)
					path := "/" + field
					if host {
						raw["blog"].(map[string]any)["host"] = map[string]any{field: decoded, "custom": "retain"}
						path = "/blog/host/" + field
					} else {
						raw[field] = decoded
					}
					source := []byte(";;;\n" + encode(raw) + "\n;;;\n正文")
					parameters, err := InspectLegacyMarkdownParameters(source, "lifecycle.md")
					if err != nil || len(parameters) != 1 || !parameters[0].RequiresDraft {
						t.Fatalf("missing lifecycle issue: %+v %v", parameters, err)
					}
					diagnostic, _ := json.Marshal(parameters)
					if strings.Contains(string(diagnostic), "private-secret-value") {
						t.Fatal("parameter response leaked value")
					}
					result, err := ConvertLegacyMarkdownWithIgnoredParameters(source, "lifecycle.md", "post", nil, []string{path})
					if err != nil {
						t.Fatal(err)
					}
					if result.Document.Blog["status"] != store.StatusDraft || result.Document.Body != "正文" {
						t.Fatal("lifecycle not forced draft")
					}
					serialized := encode(result.Document.Raw) + encode(result.Document.Blog) + strings.Join(result.Warnings, " ")
					if strings.Contains(serialized, "private-secret-value") {
						t.Fatal("removed lifecycle value was archived")
					}
					if host && result.Document.Blog["host"].(map[string]any)["custom"] != "retain" {
						t.Fatal("open host extension lost")
					}
				})
			}
		}
	}
	for _, status := range []string{"status: published", "blog: {status: published}"} {
		result, err := ConvertLegacyMarkdownWithIgnoredParameters([]byte("---\npassword: secret\n"+status+"\n---\nbody"), "old.md", "post", nil, []string{"/password"})
		if err != nil || result.Document.Blog["status"] != store.StatusDraft {
			t.Fatalf("published source not forced draft: %+v %v", result, err)
		}
	}
}

func TestMarkdownUnsupportedPointersProveExactObjectField(t *testing.T) {
	source := []byte(`;;;
{"title":"Keep","article.author":"unknown-root","article":{"author":"real-author","weird./~":{"secret":true}},"x/y~z":{"child":1},"blog":{"custom.key":{"untouched":true}}}
;;;
body`)
	parameters, err := InspectLegacyMarkdownParameters(source, "old.MD")
	if err != nil {
		t.Fatal(err)
	}
	paths := []string{}
	for _, parameter := range parameters {
		paths = append(paths, parameter.Path)
	}
	want := []string{"/article.author", "/article/weird.~1~0", "/x~1y~0z"}
	if !reflect.DeepEqual(paths, want) {
		t.Fatalf("unsafe/incomplete paths: %#v, want %#v", paths, want)
	}
	result, err := ConvertLegacyMarkdownWithIgnoredParameters(source, "old.MD", "post", nil, paths)
	if err != nil {
		t.Fatal(err)
	}
	if result.Document.Raw["article"].(map[string]any)["author"] != "real-author" || result.Document.Blog["custom.key"] == nil {
		t.Fatal("dotted field removal damaged supported/open field")
	}
}

func TestMarkdownIgnoreCannotRepairSupportedTypeConflictOrShortcode(t *testing.T) {
	for _, header := range []string{"footer: {references: [7]}", "render: {math: 7}", "type: bad", "status: private", "pin: 1\npinned: 2", "blog: {status: invalid}"} {
		source := []byte("---\npassword: secret\n" + header + "\n---\nbody")
		if _, err := ConvertLegacyMarkdownWithIgnoredParameters(source, "old.md", "post", nil, []string{"/password"}); err == nil {
			t.Fatalf("ignored invalid supported value %s", header)
		}
	}
	source := []byte("---\nhide: true\n---\n{% unsupported thing %}")
	if _, err := ConvertLegacyMarkdownWithIgnoredParameters(source, "old.md", "post", nil, []string{"/hide"}); err == nil {
		t.Fatal("shortcode validation bypassed")
	}
	source = []byte("---\npassword: secret\ntopic: new-topic\n---\nbody")
	_, err := ConvertLegacyMarkdownWithIgnoredParameters(source, "old.md", "post", nil, []string{"/password"})
	var missing *MissingLegacyTopicError
	if !errors.As(err, &missing) {
		t.Fatalf("missing topic chain lost: %v", err)
	}
	result, err := ConvertLegacyMarkdownWithIgnoredParameters(source, "old.md", "post", []*store.Topic{{Name: "new-topic", Slug: "new-topic"}}, []string{"/password"})
	if err != nil || result.Document.Blog["status"] != store.StatusDraft {
		t.Fatalf("topic retry lost ignore selection: %+v %v", result, err)
	}
}

func TestMarkdownUnsupportedCollectionNeverOffersListItems(t *testing.T) {
	source := []byte("---\nfooter: {references: [{extra/key: {keep: true}}]}\n---\nbody")
	parameters, err := InspectLegacyMarkdownParameters(source, "old.md")
	if err != nil || len(parameters) != 1 || parameters[0].Path != "/footer/references/0/extra~1key" {
		t.Fatalf("unknown field in array object: %+v %v", parameters, err)
	}
	result, err := ConvertLegacyMarkdownWithIgnoredParameters(source, "old.md", "post", nil, []string{parameters[0].Path})
	if err != nil || result.Document.Body != "body" {
		t.Fatalf("array object field consent failed: %+v %v", result, err)
	}
	invalid := []byte("---\nfooter: {references: [7]}\n---\nbody")
	parameters, err = InspectLegacyMarkdownParameters(invalid, "old.md")
	if err != nil || len(parameters) != 0 {
		t.Fatalf("invalid array item offered for removal: %+v %v", parameters, err)
	}
	if _, err := ConvertLegacyMarkdownWithIgnoredParameters(invalid, "old.md", "post", nil, []string{"/footer/references/0"}); err == nil {
		t.Fatal("array item authorization accepted")
	}
}

func TestMarkdownUnsupportedLegacyAliasFieldsMapBackToOriginalPointers(t *testing.T) {
	source := []byte("---\nreferences:\n  - extra/key: {private: keep-out-of-diagnostics}\n    other~field: true\n  - '[Known reference](/known)'\nlicense: false\nshare: [qrcode, email]\n---\n正文\n")
	parameters, err := InspectLegacyMarkdownParameters(source, "aliases.md")
	if err != nil || len(parameters) != 2 {
		t.Fatalf("missing alias diagnostics: %+v %v", parameters, err)
	}
	paths := []string{parameters[0].Path, parameters[1].Path}
	if !reflect.DeepEqual(paths, []string{"/references/0/extra~1key", "/references/0/other~0field"}) || parameters[0].Field != "references[0].extra/key" || parameters[0].Code != "unknown_field" {
		t.Fatalf("alias source paths lost: %+v", parameters)
	}
	_, err = ConvertLegacyMarkdownWithIgnoredParameters(source, "aliases.md", "post", nil, paths[:1])
	var remaining *UnsupportedMarkdownParametersError
	if !errors.As(err, &remaining) || len(remaining.Parameters) != 1 || remaining.Parameters[0].Path != paths[1] {
		t.Fatalf("partial alias consent lost: %+v %v", remaining, err)
	}
	result, err := ConvertLegacyMarkdownWithIgnoredParameters(source, "aliases.md", "post", nil, paths)
	if err != nil {
		t.Fatal(err)
	}
	footer := result.Document.Raw["footer"].(map[string]any)
	if result.Document.Body != "正文\n" || footer["license"] != false || !reflect.DeepEqual(footer["share"], []any{"qrcode", "email"}) || !reflect.DeepEqual(footer["references"], []any{map[string]any{}, "[Known reference](/known)"}) {
		t.Fatalf("valid alias content changed: %+v", result.Document)
	}
	diagnostic, _ := json.Marshal(parameters)
	if strings.Contains(string(diagnostic), "keep-out-of-diagnostics") {
		t.Fatal("alias diagnostic leaked value")
	}
	for _, path := range []string{"/references/1", "/references/0", "/footer/references/0/extra~1key", "/license", "/share"} {
		if _, err := ConvertLegacyMarkdownWithIgnoredParameters(source, "aliases.md", "post", nil, []string{path}); err == nil {
			t.Fatalf("invalid alias authorization accepted: %s", path)
		}
	}
}

func TestMarkdownLegacyAliasTypesConflictsAndPosterFieldBoundaries(t *testing.T) {
	for _, header := range []string{
		"references: wrong", "references: [7]", "license: {unknown: true}", "share: {unknown: true}", "share: [email, 7]",
		"references: [{extra: true}]\nfooter: {references: [known]}",
		"poster: {extra: true}\nposter_headline: conflicting", "poster: {extra: true}\nblog: {content: {poster: {headline: Known}}}",
	} {
		source := []byte("---\n" + header + "\n---\nbody")
		parameters, err := InspectLegacyMarkdownParameters(source, "bad.md")
		if err != nil || len(parameters) != 0 {
			t.Fatalf("type/conflict offered as unsupported: %s %+v %v", header, parameters, err)
		}
		if _, err := ConvertLegacyMarkdown(source, "bad.md", "post", nil); err == nil {
			t.Fatalf("invalid alias accepted: %s", header)
		}
	}
	source := []byte("---\nposter: {headline: Known, extra/key: {unknown: true}}\n---\nbody")
	parameters, err := InspectLegacyMarkdownParameters(source, "poster.md")
	if err != nil || len(parameters) != 1 || parameters[0].Path != "/poster/extra~1key" || parameters[0].Field != "poster.extra/key" {
		t.Fatalf("poster unknown field missing: %+v %v", parameters, err)
	}
	result, err := ConvertLegacyMarkdownWithIgnoredParameters(source, "poster.md", "post", nil, []string{parameters[0].Path})
	if err != nil {
		t.Fatal(err)
	}
	poster, _ := get(result.Document.Blog, "content.poster")
	if !reflect.DeepEqual(poster, map[string]any{"headline": "Known"}) || result.Document.Body != "body" {
		t.Fatalf("known poster fields lost: %+v", result.Document)
	}
	invalid := []byte("---\nposter: {headline: 7, extra: true}\n---\nbody")
	if _, err := ConvertLegacyMarkdownWithIgnoredParameters(invalid, "poster.md", "post", nil, []string{"/poster/extra"}); err == nil {
		t.Fatal("poster supported field type error hidden")
	}
}

func TestMarkdownUnsupportedFooterActionsKeepRealDiscriminatorContext(t *testing.T) {
	for _, test := range []struct{ action, path string }{
		{`{type: link, title: Keep, icon: 'ri:home-line', url: /, extra: true}`, "/leftbar/footer/actions/0/extra"},
		{`{type: button, title: Keep, icon: 'ri:home-line', onclick: 'handler()', extra: true}`, "/leftbar/footer/actions/0/extra"},
		{`{type: spacer, extra: true}`, "/leftbar/footer/actions/0/extra"},
		{`{type: dropdown, title: Keep, items: [{type: link, title: Child, url: /, extra: true}]}`, "/leftbar/footer/actions/0/items/0/extra"},
	} {
		t.Run(test.path+test.action, func(t *testing.T) {
			source := []byte("---\nleftbar: {footer: {actions: [" + test.action + "]}}\n---\nbody")
			parameters, err := InspectLegacyMarkdownParameters(source, "actions.md")
			if err != nil || len(parameters) != 1 || parameters[0].Path != test.path {
				t.Fatalf("action discriminator was lost: %+v %v", parameters, err)
			}
			result, err := ConvertLegacyMarkdownWithIgnoredParameters(source, "actions.md", "post", nil, []string{test.path})
			if err != nil || result.Document.Body != "body" {
				t.Fatalf("action precise ignore failed: %+v %v", result, err)
			}
		})
	}
}

func TestMarkdownActionDiscriminatorProofCannotAuthorizeSupportedDottedCollision(t *testing.T) {
	raw := map[string]any{
		"leftbar.footer.actions[0].url": "unknown dotted root",
		"leftbar":                       map[string]any{"footer": map[string]any{"actions": []any{map[string]any{"type": "link", "title": "Keep", "icon": "ri:home-line", "url": "/", "extra": true}}}},
	}
	source := []byte(";;;\n" + encode(raw) + "\n;;;\nbody")
	parameters, err := InspectLegacyMarkdownParameters(source, "actions.md")
	if err != nil {
		t.Fatal(err)
	}
	paths := []string{}
	for _, parameter := range parameters {
		paths = append(paths, parameter.Path)
	}
	if !reflect.DeepEqual(paths, []string{"/leftbar.footer.actions[0].url", "/leftbar/footer/actions/0/extra"}) {
		t.Fatalf("dotted collision confused action field identity: %+v", parameters)
	}
	for _, path := range []string{"/leftbar/footer/actions/0/type", "/leftbar/footer/actions/0/url", "/leftbar/footer/actions/0"} {
		if _, err := ConvertLegacyMarkdownWithIgnoredParameters(source, "actions.md", "post", nil, []string{path}); err == nil {
			t.Fatalf("supported/action item field authorized: %s", path)
		}
	}
	result, err := ConvertLegacyMarkdownWithIgnoredParameters(source, "actions.md", "post", nil, paths)
	if err != nil {
		t.Fatal(err)
	}
	action, exists := markdownParameterAt(result.Document.Raw, []string{"leftbar", "footer", "actions", "0"})
	if !exists || !reflect.DeepEqual(action, map[string]any{"type": "link", "title": "Keep", "icon": "ri:home-line", "url": "/"}) {
		t.Fatalf("supported action fields damaged: %#v", action)
	}
	for _, action := range []string{
		`{type: invalid, title: Keep, icon: 'ri:home-line', url: /, extra: true}`,
		`{title: Keep, icon: 'ri:home-line', url: /, extra: true}`,
		`{type: dropdown, title: Keep, items: [{type: spacer, extra: true}]}`,
	} {
		invalid := []byte("---\nleftbar: {footer: {actions: [" + action + "]}}\n---\nbody")
		parameters, err := InspectLegacyMarkdownParameters(invalid, "actions.md")
		if err != nil || len(parameters) != 0 {
			t.Fatalf("invalid discriminator offered for removal: %+v %v", parameters, err)
		}
		if _, err := ConvertLegacyMarkdown(invalid, "actions.md", "post", nil); err == nil {
			t.Fatal("invalid action discriminator accepted")
		}
	}
}
