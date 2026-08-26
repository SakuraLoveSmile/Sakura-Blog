package routes

import (
	"regexp"
	"strings"

	"blog/internal/post"
)

// liveLinksShortcode matches the links tag whose output depends on the current
// site configuration. Stored ContentHTML is otherwise safe to reuse, but it
// becomes stale as soon as a friend link is added, edited, or removed.
var liveLinksShortcode = regexp.MustCompile(`(?m){%\s*links\s*%}`)

// renderStoredContent returns cached HTML for ordinary content and re-renders
// Markdown when it contains the configuration-backed links shortcode. Empty
// cached HTML keeps the existing Markdown fallback behavior for all content.
func renderStoredContent(renderer *post.Service, markdown, cachedHTML string) (string, error) {
	if renderer == nil {
		return cachedHTML, nil
	}
	if strings.TrimSpace(cachedHTML) != "" && !liveLinksShortcode.MatchString(markdown) {
		return cachedHTML, nil
	}
	return renderer.Render(markdown)
}
