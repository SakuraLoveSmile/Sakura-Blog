package stellarimages

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	whatwg "github.com/nlnwa/whatwg-url/url"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

func imageURL(src, base string) string {
	if src == "" {
		return ""
	}
	u, err := whatwg.ParseRef(base, src)
	if err != nil || u == nil || (u.Scheme() != "http" && u.Scheme() != "https") {
		return ""
	}
	return u.Href(false)
}
func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}
func setAttr(n *html.Node, key, value string) {
	for i := range n.Attr {
		if n.Attr[i].Key == key {
			n.Attr[i].Val = value
			return
		}
	}
	n.Attr = append(n.Attr, html.Attribute{Key: key, Val: value})
}
func removeAttr(n *html.Node, key string) {
	for i := range n.Attr {
		if n.Attr[i].Key == key {
			n.Attr = append(n.Attr[:i], n.Attr[i+1:]...)
			return
		}
	}
}
func hasClass(n *html.Node, class string) bool {
	for _, s := range strings.Fields(attr(n, "class")) {
		if s == class {
			return true
		}
	}
	return false
}
func inScope(n *html.Node) bool {
	for p := n.Parent; p != nil; p = p.Parent {
		if p.Data == "article" && hasClass(p, "md-text") && hasClass(p, "content") {
			return true
		}
		for _, c := range []string{"banner", "cover", "wiki-card-cover", "pin-slide", "wiki-hero"} {
			if hasClass(p, c) {
				return true
			}
		}
	}
	return false
}
func imageNodes(root *html.Node) []*html.Node {
	var out []*html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "img" && inScope(n) {
			out = append(out, n)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return out
}
func parseHTML(raw string) (*html.Node, bool, error) {
	full := regexp.MustCompile(`(?i)<!doctype\s+html|<html[\s>]`).MatchString(raw)
	if full {
		n, e := html.Parse(strings.NewReader(raw))
		return n, true, e
	}
	n := &html.Node{Type: html.ElementNode, Data: "div", DataAtom: atom.Div}
	nodes, e := html.ParseFragment(strings.NewReader(raw), n)
	if e != nil {
		return nil, false, e
	}
	for _, c := range nodes {
		n.AppendChild(c)
	}
	return n, false, nil
}

// Collect 按渲染后的作用域扫描，保持第一次出现的 WHATWG URL 顺序。
func Collect(raw, baseURL string) ([]string, error) {
	if u, err := whatwg.Parse(baseURL); err != nil || u == nil || (u.Scheme() != "http" && u.Scheme() != "https") {
		return nil, fmt.Errorf("invalid page base URL")
	}
	root, _, err := parseHTML(raw)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	out := []string{}
	for _, n := range imageNodes(root) {
		src := attr(n, "data-src")
		if src == "" {
			src = attr(n, "src")
		}
		u := imageURL(src, baseURL)
		if u != "" && !seen[u] {
			seen[u] = true
			out = append(out, u)
		}
	}
	return out, nil
}

type EnrichOptions struct{ AutoAspectRatio *bool }

var ratioStyle = regexp.MustCompile(`(?i)(?:^|;)\s*aspect-ratio\s*:[^;]*`)
var sizeStyle = regexp.MustCompile(`(?i)(?:^|;)\s*(width|height)\s*:\s*([^;]+)`)

// Enrich 只回填派生 HTML，不写入作者 Markdown 或原始图片。
func Enrich(raw, baseURL string, store Store, opts EnrichOptions) (string, error) {
	if len(store.Images) == 0 || !strings.Contains(strings.ToLower(raw), "<img") {
		return raw, nil
	}
	root, full, err := parseHTML(raw)
	if err != nil {
		return "", err
	}
	changed := false
	auto := opts.AutoAspectRatio == nil || *opts.AutoAspectRatio
	for _, n := range imageNodes(root) {
		src := attr(n, "data-src")
		if src == "" {
			src = attr(n, "src")
		}
		e, ok := store.Images[imageURL(src, baseURL)]
		if !ok {
			continue
		}
		if auto && dimensionsReady(e.Dimensions) {
			w, h := e.Dimensions.Width, e.Dimensions.Height
			if attr(n, "data-stellar-image-dimensions") != "" || (attr(n, "width") == "" && attr(n, "height") == "") {
				setAttr(n, "width", strconv.Itoa(w))
				setAttr(n, "height", strconv.Itoa(h))
				setAttr(n, "data-stellar-image-dimensions", "true")
				changed = true
			}
			for p := n.Parent; p != nil; p = p.Parent {
				if !hasClass(p, "image-bg") {
					continue
				}
				tagged := false
				for q := p.Parent; q != nil; q = q.Parent {
					if hasClass(q, "tag-plugin") && hasClass(q, "image") {
						tagged = true
						break
					}
				}
				if !tagged {
					continue
				}
				if attr(p, "data-stellar-image-ratio") != "" || !ratioStyle.MatchString(attr(p, "style")) {
					wrapperStyle := ratioStyle.ReplaceAllString(attr(p, "style"), "")
					imageStyle := sizeStyle.ReplaceAllStringFunc(attr(n, "style"), func(match string) string {
						parts := sizeStyle.FindStringSubmatch(match)
						key := strings.ToLower(parts[1])
						r := regexp.MustCompile(`(?i)(?:^|;)\s*` + key + `\s*:[^;]*`)
						wrapperStyle = r.ReplaceAllString(wrapperStyle, "") + ";" + key + ":" + parts[2]
						return ""
					})
					if strings.TrimSpace(imageStyle) != "" {
						setAttr(n, "style", imageStyle)
					} else {
						removeAttr(n, "style")
					}
					setAttr(p, "style", fmt.Sprintf("%s;aspect-ratio:%d/%d;", wrapperStyle, w, h))
					setAttr(p, "data-stellar-image-ratio", "true")
					changed = true
				}
				break
			}
		}
		if colorReady(e.Color) {
			b, _ := json.Marshal(struct {
				Size int       `json:"size"`
				HSLA []float64 `json:"hsla"`
			}{e.Color.Size, e.Color.HSLA})
			setAttr(n, "data-stellar-image-color", string(b))
			setAttr(n, "data-stellar-image-source", src)
			changed = true
		}
	}
	if !changed {
		return raw, nil
	}
	var b bytes.Buffer
	if full {
		err = html.Render(&b, root)
	} else {
		for c := root.FirstChild; c != nil; c = c.NextSibling {
			if err = html.Render(&b, c); err != nil {
				break
			}
		}
	}
	return b.String(), err
}
