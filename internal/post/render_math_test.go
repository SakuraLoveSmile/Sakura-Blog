package post

import (
	"strings"
	"testing"
)

func TestRenderMathIntegration(t *testing.T) {
	s := NewService("../../content/posts")
	_, body, err := s.ParseFile("katex-test")
	if err != nil {
		t.Skip("katex-test.md not available:", err)
	}
	html, err := s.Render(body)
	if err != nil {
		t.Fatal(err)
	}
	for _, sub := range []string{"$E=mc^2$", `\(\mathrm{e}^{i\pi}`, `sum_{i=1}^{n}`, "$5 和 $10"} {
		if !strings.Contains(html, sub) {
			t.Fatalf("rendered HTML missing %q\n%s", sub, html)
		}
	}
}
