package post

import (
	"regexp"
	"strings"
	"testing"

	"blog/internal/textutil"
)

func TestStellarPoetryPreservesLineBreaks(t *testing.T) {
	s := NewService("")
	body := "普通第一行\n普通第二行\n\n{% poetry 早发白帝城 author:李白 %}\n朝辞白帝彩云间，\n千里江陵一日还。\n两岸猿声啼不住，\n轻舟已过万重山。\n{% endpoetry %}"
	got, err := s.RenderStellarV2(body, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := "<p>朝辞白帝彩云间，<br>千里江陵一日还。<br>两岸猿声啼不住，<br>轻舟已过万重山。</p>"
	if !strings.Contains(got, want) || strings.Count(got, "<br>") != 3 {
		t.Fatalf("poetry must preserve four lines: %s", got)
	}
	if !strings.Contains(got, "<p>普通第一行\n普通第二行</p>") {
		t.Fatalf("ordinary Markdown soft breaks changed: %s", got)
	}
}

func TestStellarPoetryLineBreaksRespectMarkdown(t *testing.T) {
	s := NewService("")
	body := "{% poetry %}\n**第一行**\n第二行  \n第三行\n\n新段落 `code`\n\n```text\none\ntwo\n```\n{% endpoetry %}"
	got, err := s.RenderStellarV2(body, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "<p><strong>第一行</strong><br>第二行<br>第三行</p><p>新段落 <code>code</code></p>") {
		t.Fatalf("poetry lost Markdown paragraphs/emphasis or duplicated a hard break: %s", got)
	}
	if strings.Count(got, "<br>") != 2 || !strings.Contains(got, `class="chroma highlight text"`) || !strings.Contains(got, "one") || !strings.Contains(got, "two") {
		t.Fatalf("line breaks must not be inserted into the code block: %s", got)
	}
}

func TestStellarHeadingLinksPreserveAnchorsAndSearchText(t *testing.T) {
	s := NewService("")
	body := "## 一、为什么我们不再使用KOOK\n\nText.\n\n### Link & \"quoted\" [label](/docs)\n\n## 一、为什么我们不再使用KOOK"
	legacy, err := s.Render(body)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.RenderStellarV2(body, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<h2 id="kook"><a href="#kook" class="headerlink" title="一、为什么我们不再使用KOOK"></a>`,
		`<h2 id="kook-1"><a href="#kook-1" class="headerlink"`,
		`title="Link &amp; &#34;quoted&#34; label"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing heading anchor %q in %s", want, got)
		}
	}
	if strings.Count(got, `class="headerlink"`) != 3 || !strings.Contains(got, `<a href="/docs">label</a>`) {
		t.Fatalf("heading links replaced existing inline links: %s", got)
	}
	beforeText, beforeAnchors := textutil.StripHTMLWithAnchors(legacy)
	afterText, afterAnchors := textutil.StripHTMLWithAnchors(got)
	if beforeText != afterText || len(beforeAnchors) != len(afterAnchors) {
		t.Fatalf("search text/anchors changed: before=%q %#v after=%q %#v", beforeText, beforeAnchors, afterText, afterAnchors)
	}
	for i := range beforeAnchors {
		if beforeAnchors[i] != afterAnchors[i] {
			t.Fatalf("search anchor changed: before=%#v after=%#v", beforeAnchors[i], afterAnchors[i])
		}
	}
}

func TestStellarHeadingLinksInsideShortcodes(t *testing.T) {
	s := NewService("")
	got, err := s.RenderStellarV2("{% folding title %}\n## Nested\n{% endfolding %}\n\n```html\n<h2 id=\"example\">Example</h2>\n```", nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(got, `class="headerlink"`) != 1 || !regexp.MustCompile(`<h2 id="nested"><a [^>]*href="#nested"`).MatchString(got) {
		t.Fatalf("nested heading must receive exactly one anchor; code examples must not: %s", got)
	}
}
