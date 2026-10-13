package clientconfig

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/andybalholm/cascadia"
	"golang.org/x/net/html"
)

type NavigationOptions struct {
	CollectionProfile, CollectionID, PageURL, Root string
	// 键为公开URL路径，值为 JSON.stringify([profile,id]) 的等效JSON字符串。
	Members map[string]string
}
type navigationConfig struct {
	Collection string `json:"collection"`
	URL        string `json:"url"`
	Signature  string `json:"signature"`
}

var dataScriptType = regexp.MustCompile(`^(application/(?:ld\+)?json|math/tex(?:;.*)?)$`)

// PrepareNavigation 为同集合链接标记，并按真正保留的shell和扩展计算兼容签名。
// 非托管可执行脚本会禁用局部导航；非法manifest/selector等构造错误明确返回错误。
func PrepareNavigation(source string, options NavigationOptions) (string, error) {
	return prepareNavigation(source, options, false)
}

// RefreshNavigationSignature 回填同批 HTML 后更新签名，保留集合、URL 及已经授权的链接。
func RefreshNavigationSignature(source string) (string, error) {
	doc, err := html.Parse(strings.NewReader(source))
	if err != nil {
		return "", err
	}
	nodes := find(doc, "#stellar-navigation-config")
	if len(nodes) == 0 {
		return source, nil
	}
	if len(nodes) != 1 {
		return "", fmt.Errorf("duplicate navigation config")
	}
	var nav navigationConfig
	if err := json.Unmarshal([]byte(text(nodes[0])), &nav); err != nil {
		return "", err
	}
	var collection []string
	if err := json.Unmarshal([]byte(nav.Collection), &collection); err != nil || len(collection) != 2 {
		return "", fmt.Errorf("invalid navigation collection")
	}
	runtime := find(doc, "#stellar-runtime-config")
	if len(runtime) != 1 {
		return "", fmt.Errorf("missing navigation runtime")
	}
	var manifest Manifest
	if err := json.Unmarshal([]byte(text(runtime[0])), &manifest); err != nil {
		return "", err
	}
	return prepareNavigation(source, NavigationOptions{CollectionProfile: collection[0], CollectionID: collection[1], PageURL: nav.URL, Root: manifest.Root}, true)
}

func prepareNavigation(source string, options NavigationOptions, preserveAnchors bool) (string, error) {
	if options.CollectionProfile == "" || options.CollectionID == "" {
		return source, nil
	}
	doc, err := html.Parse(strings.NewReader(source))
	if err != nil {
		return "", fmt.Errorf("navigation HTML: %w", err)
	}
	for _, node := range find(doc, "#stellar-navigation-config") {
		remove(node)
	}
	if !preserveAnchors {
		for _, node := range find(doc, "a[data-stellar-navigation]") {
			delAttr(node, "data-stellar-navigation")
		}
	}
	main := find(doc, "#main")
	runtime := find(doc, "#stellar-runtime-config")
	if len(main) != 1 || len(runtime) != 1 {
		if strings.Contains(source, "stellar-navigation-config") || strings.Contains(source, "data-stellar-navigation") {
			return renderHTML(doc), nil
		}
		return source, nil
	}
	for _, node := range find(doc, "script") {
		if !dataScriptType.MatchString(strings.TrimSpace(strings.ToLower(attr(node, "type")))) && !hasAttr(node, "data-stellar-script") {
			// 不能让旧配置在禁用局部导航后继续生效。
			if strings.Contains(source, "stellar-navigation-config") || strings.Contains(source, "data-stellar-navigation") {
				return renderHTML(doc), nil
			}
			return source, nil
		}
	}
	base, err := url.Parse(options.PageURL)
	if err != nil || base.Scheme == "" || base.Host == "" || !(base.Scheme == "https" || base.Scheme == "http") {
		return "", fmt.Errorf("navigation PageURL must be an absolute HTTP(S) URL")
	}
	root := options.Root
	if root == "" {
		root = "/"
	}
	rootURL, err := url.Parse(root)
	if err != nil {
		return "", fmt.Errorf("navigation root: %w", err)
	}
	rootPath := base.ResolveReference(rootURL).Path
	rootPath = strings.TrimRight(rootPath, "/") + "/"
	collectionBytes, _ := json.Marshal([]string{options.CollectionProfile, options.CollectionID})
	collection := string(collectionBytes)
	for _, link := range find(doc, "a[href]") {
		if preserveAnchors {
			continue
		}
		href, err := url.Parse(attr(link, "href"))
		if err != nil {
			continue
		}
		target := base.ResolveReference(href)
		if !sameOrigin(target, base) || !strings.HasPrefix(target.Path, rootPath) {
			continue
		}
		path, err := decodeURIPath(target.EscapedPath())
		if err != nil {
			continue
		}
		member := options.Members[path]
		if member == "" {
			member = options.Members[target.EscapedPath()]
		}
		if member == "" && (strings.HasSuffix(path, "/") || path == rootPath) {
			member = options.Members[path+"index.html"]
		}
		if memberMatches(member, options.CollectionProfile, options.CollectionID) {
			setAttr(link, "data-stellar-navigation", "")
		}
	}
	var manifest Manifest
	decoder := json.NewDecoder(strings.NewReader(text(runtime[0])))
	decoder.UseNumber()
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return "", fmt.Errorf("navigation manifest: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return "", fmt.Errorf("navigation manifest contains trailing data")
	}
	if err := validateManifest(manifest); err != nil {
		return "", err
	}
	shell := cloneNode(doc)
	for _, node := range find(shell, "#main, #rightbar-region, [data-stellar-page-meta], [data-stellar-page-style], #stellar-runtime-config") {
		if node.Parent != nil {
			remove(node)
		}
	}
	for _, node := range find(shell, "[data-stellar-script='defines']") {
		for node.FirstChild != nil {
			node.RemoveChild(node.FirstChild)
		}
	}
	for _, node := range find(shell, "body") {
		for _, key := range []string{"data-page-type", "data-page-layout", "data-article-style", "data-text-indent"} {
			delAttr(node, key)
		}
	}
	for _, node := range find(shell, ".site-shell") {
		delAttr(node, "data-regions")
	}
	for _, node := range find(shell, "a[aria-current='page']") {
		delAttr(node, "aria-current")
	}
	for _, node := range find(shell, ".ui-collection__item.is-active") {
		classes := strings.Fields(attr(node, "class"))
		result := []string{}
		for _, class := range classes {
			if class != "is-active" {
				result = append(result, class)
			}
		}
		setAttr(node, "class", strings.Join(result, " "))
	}
	for _, node := range find(shell, ".ui-collection__indicator") {
		remove(node)
	}
	persistent := []Extension{}
	for _, extension := range manifest.Extensions {
		keep := extension.Scope == "document"
		if !keep && extension.When.Selector != "" {
			selector, err := cascadia.ParseGroup(extension.When.Selector)
			if err != nil {
				return "", fmt.Errorf("navigation selector for %s: %w", extension.ID, err)
			}
			keep = len(cascadia.QueryAll(shell, selector)) > 0
		}
		if keep {
			persistent = append(persistent, extension)
		}
	}
	compatibility := struct {
		Root       string      `json:"root"`
		Policy     Policy      `json:"policy"`
		Extensions []Extension `json:"extensions"`
	}{manifest.Root, manifest.Policy, persistent}
	data, err := json.Marshal(compatibility)
	if err != nil {
		return "", err
	}
	hash := sha256.New()
	_, _ = hash.Write([]byte(renderHTML(shell)))
	_, _ = hash.Write(data)
	nav := navigationConfig{Collection: collection, URL: base.String(), Signature: hex.EncodeToString(hash.Sum(nil))}
	data, err = json.Marshal(nav)
	if err != nil {
		return "", err
	}
	bodies := find(doc, "body")
	if len(bodies) != 1 {
		return "", fmt.Errorf("navigation HTML must have a body")
	}
	script := &html.Node{Type: html.ElementNode, Data: "script", Attr: []html.Attribute{{Key: "type", Val: "application/json"}, {Key: "id", Val: "stellar-navigation-config"}}}
	script.AppendChild(&html.Node{Type: html.TextNode, Data: string(data)})
	bodies[0].AppendChild(script)
	return renderHTML(doc), nil
}
func validateManifest(manifest Manifest) error {
	if manifest.Version != 1 || manifest.Root == "" {
		return fmt.Errorf("navigation manifest version/root invalid")
	}
	ids := map[string]bool{}
	for _, entry := range manifest.Extensions {
		if entry.ID == "" || entry.Module == "" || ids[entry.ID] {
			return fmt.Errorf("navigation extension ID/module invalid or duplicate")
		}
		ids[entry.ID] = true
		if entry.Scope != "document" && entry.Scope != "region" {
			return fmt.Errorf("navigation extension %s scope invalid", entry.ID)
		}
		if entry.When.Always == (entry.When.Selector != "") {
			return fmt.Errorf("navigation extension %s must declare exactly one activation", entry.ID)
		}
		if entry.Config == nil {
			return fmt.Errorf("navigation extension %s config required", entry.ID)
		}
	}
	return nil
}
func find(root *html.Node, selector string) []*html.Node {
	matcher, err := cascadia.ParseGroup(selector)
	if err != nil {
		panic(err)
	}
	return cascadia.QueryAll(root, matcher)
}
func attr(node *html.Node, key string) string {
	for _, a := range node.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}
func hasAttr(node *html.Node, key string) bool {
	for _, a := range node.Attr {
		if a.Key == key {
			return true
		}
	}
	return false
}
func delAttr(node *html.Node, key string) {
	r := node.Attr[:0]
	for _, a := range node.Attr {
		if a.Key != key {
			r = append(r, a)
		}
	}
	node.Attr = r
}
func setAttr(node *html.Node, key, value string) {
	for i, a := range node.Attr {
		if a.Key == key {
			node.Attr[i].Val = value
			return
		}
	}
	node.Attr = append(node.Attr, html.Attribute{Key: key, Val: value})
}
func remove(node *html.Node) {
	if node.Parent != nil {
		node.Parent.RemoveChild(node)
	}
}
func text(node *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(node)
	return b.String()
}
func cloneNode(node *html.Node) *html.Node {
	r := &html.Node{Type: node.Type, DataAtom: node.DataAtom, Data: node.Data, Namespace: node.Namespace, Attr: append([]html.Attribute(nil), node.Attr...)}
	for c := node.FirstChild; c != nil; c = c.NextSibling {
		r.AppendChild(cloneNode(c))
	}
	return r
}
func renderHTML(node *html.Node) string {
	var b bytes.Buffer
	_ = html.Render(&b, node)
	return b.String()
}

// decodeURI保留URL保留字符，避免把包含%2F的成员误标为另一条路由。
func decodeURIPath(path string) (string, error) {
	var b strings.Builder
	for i := 0; i < len(path); {
		if path[i] != '%' {
			b.WriteByte(path[i])
			i++
			continue
		}
		if i+2 >= len(path) {
			return "", fmt.Errorf("malformed URI escape")
		}
		decoded, err := url.PathUnescape(path[i : i+3])
		if err != nil {
			return "", err
		}
		if strings.ContainsAny(decoded, ";/?:@&=+$,#") {
			b.WriteString(path[i : i+3])
		} else {
			b.WriteString(decoded)
		}
		i += 3
	}
	if !utf8.ValidString(b.String()) {
		return "", fmt.Errorf("malformed URI UTF-8")
	}
	return b.String(), nil
}

func sameOrigin(a, b *url.URL) bool {
	normalized := func(u *url.URL) string {
		scheme := strings.ToLower(u.Scheme)
		port := u.Port()
		if (scheme == "https" && port == "443") || (scheme == "http" && port == "80") {
			port = ""
		}
		return scheme + "://" + strings.ToLower(u.Hostname()) + ":" + port
	}
	return normalized(a) == normalized(b)
}
func memberMatches(member, profile, id string) bool {
	var pair []string
	if err := json.Unmarshal([]byte(member), &pair); err != nil {
		return false
	}
	return len(pair) == 2 && pair[0] == profile && pair[1] == id
}
