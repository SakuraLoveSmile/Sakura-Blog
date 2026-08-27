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

// liveMomentsShortcode matches the moments tag whose output depends on the
// current moments table. Its cached HTML must be refreshed when a moment is
// created, edited, made private, or deleted.
var liveMomentsShortcode = regexp.MustCompile(`(?m){%\s*moments\b[^%]*%}`)

// renderStoredContent returns cached HTML for ordinary content and re-renders
// Markdown when it contains a shortcode backed by live site data. Empty cached
// HTML keeps the existing Markdown fallback behavior for all content.
func renderStoredContent(renderer *post.Service, markdown, cachedHTML string) (string, error) {
	if renderer == nil {
		return cachedHTML, nil
	}
	if strings.TrimSpace(cachedHTML) != "" &&
		!liveLinksShortcode.MatchString(markdown) &&
		!liveMomentsShortcode.MatchString(markdown) {
		return cachedHTML, nil
	}
	return renderer.Render(markdown)
}
