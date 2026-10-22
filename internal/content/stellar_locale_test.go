package content

import "testing"

func TestStellarLanguageUsesPageThenCanonicalHostAndSkipsEmptyArrayEntries(t *testing.T) {
	for _, test := range []struct {
		page, blog map[string]any
		want       string
	}{
		{map[string]any{"lang": []any{nil, "", "zh-TW"}, "language": "en"}, map[string]any{"site": map[string]any{"language": "en"}}, "zh-TW"},
		{map[string]any{"lang": false, "language": []string{"", "en"}}, nil, "en"},
		{nil, map[string]any{"site": map[string]any{"language": []any{0, "", "en"}}}, "en"},
		{nil, map[string]any{"site": map[string]any{"language": "zh-TW"}}, "zh-TW"},
		{map[string]any{"lang": []any{nil, ""}}, nil, "zh-CN"},
	} {
		if actual := ResolveStellarLanguage(test.page, test.blog); actual != test.want {
			t.Fatalf("language=%q want=%q", actual, test.want)
		}
	}
}
