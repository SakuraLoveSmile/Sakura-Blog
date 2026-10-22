package web

import (
	"errors"
	"html/template"
	"reflect"
	"strings"
	"testing"

	"blog/internal/post"
)

func TestStellarMarkdownUsesRequestSnapshotAndStrictRenderer(t *testing.T) {
	e, err := NewEngineFromEmbed(nil)
	if err != nil {
		t.Fatal(err)
	}
	d := stellarTemplateFixture(t)
	d.Stellar.Language = "en"
	d.Stellar.BlogContent = true
	d.Stellar.Data["offline"] = true
	d.Stellar.Data["page"] = map[string]any{"render": map[string]any{"math": "katex"}, "layout": "page"}
	d.Stellar.Data["fixture"] = map[string]any{"title": "request data"}
	d.Stellar.Theme["footer"] = map[string]any{"content": "**Footer** $a^2$"}
	seen := []string{}
	svc := post.NewService("")
	e.SetStellarMarkdownRender(func(body string, ctx post.StellarRenderContext) (string, error) {
		if !reflect.DeepEqual(ctx.Theme, d.Stellar.Theme) || !reflect.DeepEqual(ctx.Data, d.Stellar.Data) || !reflect.DeepEqual(ctx.Page, d.Stellar.Data["page"]) || ctx.Language != "en" || !ctx.BlogExtensions || !ctx.Offline {
			t.Fatal("Markdown snapshot detached from request")
		}
		seen = append(seen, body)
		return svc.RenderStellarV2WithContext(body, ctx)
	})
	old := markdownRender
	defer func() { markdownRender = old }()
	markdownRender = func(string) template.HTML { t.Fatal("strict template used legacy global callback"); return "" }
	out, err := e.RenderValue("layout/stellar", d)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "<strong>Footer</strong>") || !strings.Contains(out, `class="katex"`) {
		t.Fatal("strict footer did not render Markdown/math")
	}
	r := stellarTemplateRenderer{}
	widget := string(r.widget(d, "leftbar", "markdown", map[string]any{"content": "**Widget**"}, nil))
	if !strings.Contains(widget, "<strong>Widget</strong>") || len(seen) != 2 {
		t.Fatal("widget ignored request renderer")
	}
}

func TestStellarMarkdownErrorPropagatesWithoutFallback(t *testing.T) {
	e, err := NewEngineFromEmbed(nil)
	if err != nil {
		t.Fatal(err)
	}
	d := stellarTemplateFixture(t)
	d.Stellar.Theme["footer"] = map[string]any{"content": "**Footer**"}
	sentinel := errors.New("strict renderer failure")
	e.SetStellarMarkdownRender(func(string, post.StellarRenderContext) (string, error) { return "", sentinel })
	if out, err := e.RenderValue("layout/stellar", d); out != "" || !errors.Is(err, sentinel) {
		t.Fatalf("partial/fallback output %q error %v", out, err)
	}
	e.SetStellarMarkdownRender(post.NewService("").RenderStellarV2WithContext)
	if out, err := e.RenderValue("layout/stellar", d); err != nil || !strings.Contains(out, "<strong>Footer</strong>") {
		t.Fatalf("request error leaked into next render: %v", err)
	}
	e.SetStellarMarkdownRender(nil)
	if _, err := e.RenderValue("layout/stellar", d); err == nil || !strings.Contains(err.Error(), "renderer required") {
		t.Fatal("missing strict renderer fell back")
	}
}
