package comments

import (
	stdhtml "html"
	"html/template"
	"strings"

	"golang.org/x/net/html"
)

// ExcerptHTML 从已消毒的评论 HTML 构造侧边栏摘要，保留白名单内的表情小图。
func ExcerptHTML(contentHTML string, limit int) template.HTML {
	nodes, err := html.ParseFragment(strings.NewReader(contentHTML), nil)
	if err != nil {
		return ""
	}

	b := excerptBuilder{remaining: limit}
	for _, node := range nodes {
		b.visit(node)
		if b.stopped {
			break
		}
	}
	return template.HTML(b.output.String())
}

type excerptBuilder struct {
	output    strings.Builder
	remaining int
	stopped   bool
}

func (b *excerptBuilder) visit(node *html.Node) {
	if b.stopped {
		return
	}
	if node.Type == html.TextNode {
		b.appendText(node.Data)
		return
	}
	if node.Type == html.ElementNode && node.Data == "img" {
		b.appendEmoji(node)
		return
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		b.visit(child)
		if b.stopped {
			return
		}
	}
}

func (b *excerptBuilder) appendText(text string) {
	if text == "" || b.stopped {
		return
	}
	runes := []rune(text)
	if len(runes) <= b.remaining {
		b.output.WriteString(stdhtml.EscapeString(text))
		b.remaining -= len(runes)
		return
	}

	if b.remaining > 0 {
		b.output.WriteString(stdhtml.EscapeString(string(runes[:b.remaining])))
	}
	b.output.WriteString("…")
	b.stopped = true
}

func (b *excerptBuilder) appendEmoji(node *html.Node) {
	src, alt := "", ""
	for _, attr := range node.Attr {
		switch attr.Key {
		case "src":
			src = attr.Val
		case "alt":
			alt = attr.Val
		}
	}
	if !isEmojiSrc(src) {
		return
	}
	b.output.WriteString(`<img class="cmt-emoji-img" src="`)
	b.output.WriteString(stdhtml.EscapeString(src))
	b.output.WriteString(`" alt="`)
	b.output.WriteString(stdhtml.EscapeString(alt))
	b.output.WriteString(`" loading="lazy">`)
}
