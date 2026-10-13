// Package stellarhtml 实现 Stellar 2.0 的正文和整页 HTML 后处理。
// 作者 HTML 净化属于调用者负责的前置边界。
package stellarhtml

import (
	"fmt"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// Context 接收归一化的 camelCase 主题；Icon 返回内联图标 HTML，
// IconData 返回尚未包裹的图片或 SVG 配置值。
type Context struct {
	Theme    map[string]any
	SiteURL  string
	Icon     func(key string) string
	IconData func(key string) string
	Language string
}

// ProcessContent 按 md_table、md_link 顺序执行，先于页面模型投影。
// 调用者必须先净化不可信的作者 HTML。
func ProcessContent(content string, ctx Context) (string, error) {
	if !strings.Contains(content, "<table") && !strings.Contains(content, "<a") {
		return content, nil
	}
	nodes, err := html.ParseFragment(strings.NewReader(content), &html.Node{Type: html.ElementNode, Data: "div", DataAtom: atom.Div})
	if err != nil {
		return "", fmt.Errorf("stellar HTML content: %w", err)
	}
	root := &html.Node{Type: html.ElementNode, Data: "div", DataAtom: atom.Div}
	for _, n := range nodes {
		root.AppendChild(n)
	}
	if strings.Contains(content, "<table") {
		walk(root, func(n *html.Node) {
			if n.Type != html.ElementNode || n.Data != "table" || hasClass(n.Parent, "md-table-scroll") || ancestor(n, func(p *html.Node) bool {
				return hasClass(p, "highlight") || (hasClass(p, "tag-plugin") && hasClass(p, "table"))
			}) {
				return
			}
			wrapper := &html.Node{Type: html.ElementNode, Data: "div", DataAtom: atom.Div, Attr: []html.Attribute{{Key: "class", Val: "md-table-scroll"}}}
			n.Parent.InsertBefore(wrapper, n)
			n.Parent.RemoveChild(n)
			wrapper.AppendChild(n)
		})
	}
	endpoint := siteInfoEndpoint(ctx.Theme)
	var filterErr error
	if strings.Contains(content, "<a") {
		walk(root, func(n *html.Node) {
			if filterErr != nil || n.Type != html.ElementNode || n.Data != "a" {
				return
			}
			href, ok := attr(n, "href")
			href = strings.TrimSpace(href)
			class, _ := attr(n, "class")
			role, _ := attr(n, "role")
			if !ok || href == "" || strings.HasPrefix(href, "#") || class != "" || role != "" || hasAttr(n, "cardlink") || hasAttr(n, "data-md-link") || hasAttr(n, "data-siteinfo-api") || strings.TrimSpace(nodeText(n)) == "" {
				return
			}
			if ancestor(n, func(p *html.Node) bool {
				return p.Data == "pre" || p.Data == "code" || hasClass(p, "highlight") || hasClass(p, "footnotes")
			}) || descendant(n, func(p *html.Node) bool { return p.Type == html.ElementNode && (p.Data == "img" || p.Data == "svg") }) {
				return
			}
			resolved, err := resolveHTTPURL(href, ctx.SiteURL)
			if err != nil {
				return
			}
			if ctx.Icon == nil {
				filterErr = fmt.Errorf("stellar HTML md_link: inline Icon callback required")
				return
			}
			icon := ctx.Icon("default:link")
			if icon == "" {
				filterErr = fmt.Errorf("stellar HTML md_link: missing default:link icon")
				return
			}
			span := &html.Node{Type: html.ElementNode, Data: "span", DataAtom: atom.Span, Attr: []html.Attribute{{Key: "class", Val: "md-link-icon ui-icon"}, {Key: "aria-hidden", Val: "true"}}}
			children, err := html.ParseFragment(strings.NewReader(icon), span)
			if err != nil {
				filterErr = fmt.Errorf("stellar HTML link icon: %w", err)
				return
			}
			for _, c := range children {
				span.AppendChild(c)
			}
			n.InsertBefore(span, n.FirstChild)
			setAttr(n, "data-md-link", "")
			if endpoint != "" {
				setAttr(n, "data-siteinfo-api", strings.Replace(endpoint, "{href}", encodeURIComponent(resolved), 1))
			}
		})
	}
	if filterErr != nil {
		return "", filterErr
	}
	var out strings.Builder
	for n := root.FirstChild; n != nil; n = n.NextSibling {
		if err := html.Render(&out, n); err != nil {
			return "", err
		}
	}
	return out.String(), nil
}

func siteInfoEndpoint(theme map[string]any) string {
	services, _ := theme["services"].(map[string]any)
	service, _ := services["siteInfo"].(map[string]any)
	provider, _ := service["provider"].(string)
	selected, _ := service[provider].(map[string]any)
	endpoint, _ := selected["endpoint"].(string)
	return endpoint
}
func attr(n *html.Node, key string) (string, bool) {
	if n == nil {
		return "", false
	}
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val, true
		}
	}
	return "", false
}
func hasAttr(n *html.Node, key string) bool { _, ok := attr(n, key); return ok }
func setAttr(n *html.Node, key, value string) {
	for i := range n.Attr {
		if n.Attr[i].Key == key {
			n.Attr[i].Val = value
			return
		}
	}
	n.Attr = append(n.Attr, html.Attribute{Key: key, Val: value})
}
func hasClass(n *html.Node, key string) bool {
	v, _ := attr(n, "class")
	for _, c := range strings.Fields(v) {
		if c == key {
			return true
		}
	}
	return false
}
func ancestor(n *html.Node, match func(*html.Node) bool) bool {
	for p := n; p != nil; p = p.Parent {
		if match(p) {
			return true
		}
	}
	return false
}
func descendant(n *html.Node, match func(*html.Node) bool) bool {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if match(c) || descendant(c, match) {
			return true
		}
	}
	return false
}
func nodeText(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	var b strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		b.WriteString(nodeText(c))
	}
	return b.String()
}
func walk(n *html.Node, fn func(*html.Node)) {
	fn(n)
	for c := n.FirstChild; c != nil; {
		next := c.NextSibling
		walk(c, fn)
		c = next
	}
}
