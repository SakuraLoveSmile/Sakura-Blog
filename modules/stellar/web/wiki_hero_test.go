package web

import (
	"encoding/json"
	"html"
	"regexp"
	"strings"
	"testing"

	"blog/internal/settings"
)

func TestWikiHeroAnimationMarkupAndEscaping(t *testing.T) {
	engine, err := NewEngineFromEmbed(&IconRegistry{})
	if err != nil {
		t.Fatalf("NewEngineFromEmbed: %v", err)
	}
	htmlText, err := engine.RenderValue("partial/wiki_doc", &ViewData{
		Site:            &Site{Title: "测试站点"},
		WikiProjectName: "Stellar",
		WikiProjectID:   "stellar",
		WikiHero: &settings.HeroConfig{
			Background: "https://cdn.example/cover.jpg",
			Animation: &settings.HeroAnimation{Type: "galaxy", Params: map[string]any{
				"density": 4,
				"label":   `</canvas><script>alert("xss")</script>`,
				"quote":   `a'b\"c&d`,
			}},
		},
		WikiDoc: &settings.WikiDocView{},
	})
	if err != nil {
		t.Fatalf("render wiki hero: %v", err)
	}
	for _, want := range []string{
		`class="wiki-cover-background galaxy image"`,
		`data-text-adaptive="split"`,
		`style="--cover-url:url('https://cdn.example/cover.jpg')"`,
	} {
		if !strings.Contains(htmlText, want) {
			t.Fatalf("wiki hero missing %q: %s", want, htmlText)
		}
	}
	if strings.Contains(htmlText, `</canvas><script>`) || strings.Contains(htmlText, `<script>alert`) {
		t.Fatalf("Galaxy params escaped content leaked into markup: %s", htmlText)
	}
	match := regexp.MustCompile(`data-galaxy-params="([^"]*)"`).FindStringSubmatch(htmlText)
	if len(match) != 2 {
		t.Fatalf("Galaxy params attribute missing: %s", htmlText)
	}
	var params map[string]any
	if err := json.Unmarshal([]byte(html.UnescapeString(match[1])), &params); err != nil {
		t.Fatalf("Galaxy params are not valid JSON after HTML decoding: %v (%s)", err, match[1])
	}
	if params["density"] != float64(4) || params["label"] == nil {
		t.Fatalf("Galaxy params changed during rendering: %#v", params)
	}
}
