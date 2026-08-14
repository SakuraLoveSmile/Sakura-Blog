package comment

import (
	"strings"
	"testing"
)

const testEmojiSrc = "https://cdn.jsdelivr.net/gh/cdn-x/emoticons@3.1.0/qq/aini.gif"

func TestExcerptHTMLTruncatesTextByRune(t *testing.T) {
	input := strings.Repeat("字", 41)
	want := strings.Repeat("字", 40) + "…"

	if got := string(ExcerptHTML(input, 40)); got != want {
		t.Fatalf("ExcerptHTML() = %q, want %q", got, want)
	}
}

func TestExcerptHTMLKeepsEmojiWithRebuiltAttributes(t *testing.T) {
	input := `Hello <img title="discard" class="user-class" src="` + testEmojiSrc + `" alt="aini" loading="eager"> world`
	want := `Hello <img class="cmt-emoji-img" src="` + testEmojiSrc + `" alt="aini" loading="lazy"> world`

	if got := string(ExcerptHTML(input, 40)); got != want {
		t.Fatalf("ExcerptHTML() = %q, want %q", got, want)
	}
}

func TestExcerptHTMLDropsNonEmojiImages(t *testing.T) {
	input := `before <img src="https://example.com/uploads/photo.png" alt="photo"> after`
	want := "before  after"

	if got := string(ExcerptHTML(input, 40)); got != want {
		t.Fatalf("ExcerptHTML() = %q, want %q", got, want)
	}
}

func TestExcerptHTMLEscapesDecodedTextOnce(t *testing.T) {
	input := `<p>&lt;hello&gt; &amp; &quot;world&quot;</p>`
	want := `&lt;hello&gt; &amp; &#34;world&#34;`

	if got := string(ExcerptHTML(input, 40)); got != want {
		t.Fatalf("ExcerptHTML() = %q, want %q", got, want)
	}
}

func TestExcerptHTMLStopsAfterTruncation(t *testing.T) {
	input := strings.Repeat("字", 41) + `<img src="` + testEmojiSrc + `" alt="after">after`
	want := strings.Repeat("字", 40) + "…"

	if got := string(ExcerptHTML(input, 40)); got != want {
		t.Fatalf("ExcerptHTML() = %q, want %q", got, want)
	}
}

func TestExcerptHTMLEmojiDoesNotConsumeLimit(t *testing.T) {
	input := strings.Repeat("字", 40) + `<img src="` + testEmojiSrc + `" alt="emoji">after`
	want := strings.Repeat("字", 40) + `<img class="cmt-emoji-img" src="` + testEmojiSrc + `" alt="emoji" loading="lazy">…`

	if got := string(ExcerptHTML(input, 40)); got != want {
		t.Fatalf("ExcerptHTML() = %q, want %q", got, want)
	}
}
