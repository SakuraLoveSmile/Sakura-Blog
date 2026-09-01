package post

import (
	"strings"
	"testing"

	"blog/internal/settings"
)

func runShortcodesWithTagPlugins(md string, tagPlugins settings.TagPluginsConfig) string {
	iconFunc := func(key string) string { return `<svg data-icon="` + key + `"></svg>` }
	return processShortcodes(md, func(s string) string { return s }, func(s string) string { return s }, iconFunc, nil, nil, nil, tagPlugins)
}

func TestQuotPresetAndExplicitIconPriority(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{name: "default preset", input: "{% quot text:标题 %}", want: []string{"data-icon=\"quot:quote-left\"", "data-icon=\"quot:quote-right\""}},
		{name: "hashtag preset", input: "{% quot icon:hashtag text:标题 %}", want: []string{"data-icon=\"quot:hashtag\""}},
		{name: "question preset", input: "{% quot icon:question text:标题 %}", want: []string{"data-icon=\"quot:question\""}},
		{name: "explicit prefix and suffix override preset", input: "{% quot icon:hashtag prefix:custom:left suffix:custom:right text:标题 %}", want: []string{"data-icon=\"custom:left\"", "data-icon=\"custom:right\""}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out := runShortcodes(tc.input)
			for _, want := range tc.want {
				if !strings.Contains(out, want) {
					t.Fatalf("output missing %q: %s", want, out)
				}
			}
		})
	}
}

func TestOKRUsesConfiguredAndCustomStatuses(t *testing.T) {
	config := settings.TagPluginsConfig{}
	config.FillDefaults()
	config.OKR.Status["custom"] = settings.TagPluginOKRStatus{Color: "pink", Label: "自定义"}
	out := runShortcodesWithTagPlugins("{% okr o1 status:custom %}目标{% endokr %}", config)
	if !strings.Contains(out, `color="pink"`) || !strings.Contains(out, `>自定义</span>`) {
		t.Fatalf("custom OKR status not rendered: %s", out)
	}
	// Explicit status wins over completion-based fallback.
	out = runShortcodesWithTagPlugins("{% okr o1 percent:100 status:at_risk %}目标{% endokr %}", config)
	if !strings.Contains(out, `color="yellow"`) || !strings.Contains(out, `>风险</span>`) {
		t.Fatalf("explicit OKR status did not win: %s", out)
	}
}

func TestEmojiQQSourceCanBeConfiguredIndependently(t *testing.T) {
	config := settings.TagPluginsConfig{}
	config.FillDefaults()
	config.Emoji["qq"] = "https://emoji.example/{name}.webp"
	out := runShortcodesWithTagPlugins("{% emoji qq smile %}", config)
	if !strings.Contains(out, `https://emoji.example/smile.webp`) {
		t.Fatalf("configured qq source not rendered: %s", out)
	}
}
