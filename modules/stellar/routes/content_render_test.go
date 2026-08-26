package routes

import (
	"path/filepath"
	"strings"
	"testing"

	"blog/internal/post"
	"blog/internal/settings"
)

func TestRenderStoredContentRefreshesLinksFromCurrentSite(t *testing.T) {
	renderer := post.NewService(filepath.Join(t.TempDir(), "posts"))
	currentName := "旧友链"
	renderer.SetLinksFunc(func() ([]settings.LinkGroup, string) {
		return []settings.LinkGroup{{Title: "朋友", Items: []settings.LinkItem{{Name: currentName, URL: "https://example.com"}}}}, ""
	})

	cached, err := renderer.Render("{% links %}")
	if err != nil {
		t.Fatalf("render initial links: %v", err)
	}
	if !strings.Contains(cached, currentName) {
		t.Fatalf("initial render missing %q: %s", currentName, cached)
	}

	currentName = "新友链"
	updated, err := renderStoredContent(renderer, "{% links %}", cached)
	if err != nil {
		t.Fatalf("render refreshed links: %v", err)
	}
	if !strings.Contains(updated, currentName) || strings.Contains(updated, "旧友链") {
		t.Fatalf("links should use current site data, got: %s", updated)
	}
}

func TestRenderStoredContentKeepsCachedHTMLForOrdinaryMarkdown(t *testing.T) {
	renderer := post.NewService(filepath.Join(t.TempDir(), "posts"))
	cached := `<p>已缓存的 HTML</p>`

	got, err := renderStoredContent(renderer, "普通 Markdown", cached)
	if err != nil {
		t.Fatalf("reuse cached HTML: %v", err)
	}
	if got != cached {
		t.Fatalf("ordinary content should reuse cache, got %q", got)
	}
}
