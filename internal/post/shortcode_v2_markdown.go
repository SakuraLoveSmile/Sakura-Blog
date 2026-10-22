package post

import (
	"bufio"
	"bytes"
	"fmt"
	nethtml "golang.org/x/net/html"
	"html"
	"regexp"
	"sort"
	"strings"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
	"golang.org/x/net/html/atom"
)

// 官方 diagrams runtime 消费 .mermaid 原文；普通 fence 委托同一 Chroma 配置。
type stellarCodeBlocks struct{}

func (stellarCodeBlocks) Extend(markdown goldmark.Markdown) {
	base := highlighting.NewHTMLRenderer(highlighting.WithGuessLanguage(true), highlighting.WithFormatOptions(chromahtml.WithClasses(true), chromahtml.WithLineNumbers(true), chromahtml.LineNumbersInTable(true)))
	collector := codeBlockRendererCollector{}
	base.RegisterFuncs(&collector)
	markdown.Renderer().AddOptions(renderer.WithNodeRenderers(util.Prioritized(&stellarCodeBlockRenderer{fallback: collector.fenced}, 200)))
}

type codeBlockRendererCollector struct{ fenced renderer.NodeRendererFunc }

func (r *codeBlockRendererCollector) Register(kind ast.NodeKind, fn renderer.NodeRendererFunc) {
	if kind == ast.KindFencedCodeBlock {
		r.fenced = fn
	}
}

type stellarCodeBlockRenderer struct{ fallback renderer.NodeRendererFunc }

func (r *stellarCodeBlockRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindFencedCodeBlock, r.render)
}
func (r *stellarCodeBlockRenderer) render(writer util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	fenced := node.(*ast.FencedCodeBlock)
	if !strings.EqualFold(string(fenced.Language(source)), "mermaid") {
		if !entering {
			return ast.WalkContinue, nil
		}
		var rendered bytes.Buffer
		buffer := bufio.NewWriter(&rendered)
		status, err := r.fallback(buffer, source, node, entering)
		if err != nil {
			return status, err
		}
		if err = buffer.Flush(); err != nil {
			return status, err
		}
		markup, err := stellarCopyCodeMarkup(rendered.String(), string(fenced.Language(source)))
		if err != nil {
			return status, err
		}
		_, err = writer.WriteString(markup)
		return status, err
	}
	if !entering {
		return ast.WalkContinue, nil
	}
	_, _ = writer.WriteString(`<pre class="mermaid language-mermaid" data-language="mermaid">`)
	for i := 0; i < fenced.Lines().Len(); i++ {
		segment := fenced.Lines().At(i)
		_, _ = writer.WriteString(html.EscapeString(string(segment.Value(source))))
	}
	_, _ = writer.WriteString("</pre>\n")
	return ast.WalkSkipChildren, nil
}

// Chroma 保留服务端高亮；补齐官方容器和仅含正文的复制目标，行号不能进入剪贴板。
func stellarCopyCodeMarkup(markup, language string) (string, error) {
	nodes, err := nethtml.ParseFragment(strings.NewReader(markup), &nethtml.Node{Type: nethtml.ElementNode, Data: "div", DataAtom: atom.Div})
	if err != nil {
		return "", err
	}
	addClass := func(n *nethtml.Node, class string) {
		for i, a := range n.Attr {
			if a.Key == "class" {
				n.Attr[i].Val = a.Val + " " + class
				return
			}
		}
		n.Attr = append(n.Attr, nethtml.Attribute{Key: "class", Val: class})
	}
	var hasCode func(*nethtml.Node) bool
	hasCode = func(n *nethtml.Node) bool {
		if n.Type == nethtml.ElementNode && n.Data == "code" {
			return true
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if hasCode(c) {
				return true
			}
		}
		return false
	}
	var walk func(*nethtml.Node)
	walk = func(n *nethtml.Node) {
		if n.Type == nethtml.ElementNode && n.Data == "td" {
			if hasCode(n) {
				addClass(n, "code")
			} else {
				addClass(n, "gutter")
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	var out bytes.Buffer
	for _, n := range nodes {
		if n.Type == nethtml.ElementNode {
			container := n
			if n.Data == "pre" {
				container = &nethtml.Node{Type: nethtml.ElementNode, Data: "div", DataAtom: atom.Div}
				code := &nethtml.Node{Type: nethtml.ElementNode, Data: "div", DataAtom: atom.Div, Attr: []nethtml.Attribute{{Key: "class", Val: "code"}}}
				code.AppendChild(n)
				container.AppendChild(code)
			} else {
				walk(n)
			}
			addClass(container, strings.TrimSpace("highlight "+language))
			if err = nethtml.Render(&out, container); err != nil {
				return "", err
			}
		} else if err = nethtml.Render(&out, n); err != nil {
			return "", err
		}
	}
	return out.String(), nil
}

// 代码和已渲染的数学节点不能再次解释定界符（包括嵌套 shortcode 的输出）。
func stellarMarkdownProtectedRanges(source string) [][2]int {
	ranges := v2CodeRanges(source)
	masked := []byte(source)
	mask := func(locations [][2]int) {
		for _, loc := range locations {
			for i := loc[0]; i < loc[1]; i++ {
				if masked[i] != '\n' && masked[i] != '\r' {
					masked[i] = ' '
				}
			}
		}
	}
	mask(ranges)
	for _, spec := range []struct {
		re      *regexp.Regexp
		left    int
		display bool
	}{{reBlockDollar, 2, true}, {reBracketMath, 2, true}, {reParenMath, 2, false}, {reStellarInlineDollar, 1, false}} {
		mask(stellarMathLocations(spec.re, source, spec.left, spec.display))
	}
	tokenizer := nethtml.NewTokenizer(strings.NewReader(string(masked)))
	offset := 0
	activeTag := ""
	activeStart, depth := 0, 0
	for {
		kind := tokenizer.Next()
		raw := tokenizer.Raw()
		start := offset
		offset += len(raw)
		if kind == nethtml.ErrorToken {
			break
		}
		if kind != nethtml.StartTagToken && kind != nethtml.EndTagToken && kind != nethtml.SelfClosingTagToken {
			continue
		}
		token := tokenizer.Token()
		if activeTag != "" {
			if token.Data == activeTag {
				if kind == nethtml.StartTagToken {
					depth++
				} else {
					depth--
					if depth == 0 {
						ranges = append(ranges, [2]int{activeStart, offset})
						activeTag = ""
					}
				}
			}
			continue
		}
		ranges = append(ranges, [2]int{start, offset})
		if kind == nethtml.StartTagToken {
			protected := token.Data == "pre" || token.Data == "code" || token.Data == "script" || token.Data == "style" || token.Data == "textarea" || token.Data == "noscript"
			for _, a := range token.Attr {
				if a.Key == "class" {
					for _, cl := range strings.Fields(a.Val) {
						if cl == "katex" || cl == "katex-display" || cl == "has-jax" {
							protected = true
						}
					}
				}
			}
			if protected {
				activeTag = token.Data
				activeStart = start
				depth = 1
			}
		}
	}
	if activeTag != "" {
		ranges = append(ranges, [2]int{activeStart, len(source)})
	}
	sort.Slice(ranges, func(i, j int) bool { return ranges[i][0] < ranges[j][0] })
	var merged [][2]int
	for _, segment := range ranges {
		if len(merged) > 0 && segment[0] <= merged[len(merged)-1][1] {
			merged[len(merged)-1][1] = max(merged[len(merged)-1][1], segment[1])
		} else {
			merged = append(merged, segment)
		}
	}
	return merged
}
func (r *shortcodeV2Renderer) prepareMath(source string) (string, map[string]string) {
	if v2Object(r.page["render"])["math"] == false {
		return source, nil
	}
	provider := v2String(v2Object(v2Object(r.theme["features"])["math"])["provider"])
	if override, ok := v2Object(r.page["render"])["math"].(string); ok && override != "" {
		provider = override
	}
	replacements := map[string]string{}
	process := func(input string) string {
		var placeholders []string
		wrap := func(value, tex string, display bool) string {
			if r.err != nil {
				return value
			}
			tag := "span"
			if display {
				tag = "div"
			}
			markup := `<` + tag + ` class="has-jax">` + escapeStellarMath(value) + `</` + tag + `>`
			if provider == "katex" {
				rendered, e := r.katexFormula(tex, display)
				if e != nil {
					r.fail("math", 1, e.Error())
					return value
				}
				marker := `<` + tag + ` data-stellar-math-placeholder="` + r.next("math") + `"></` + tag + `>`
				replacements[marker] = rendered
				markup = marker
			}
			i := len(placeholders)
			placeholders = append(placeholders, markup)
			return fmt.Sprintf("\x00STELLARMATH%d\x00", i)
		}
		replace := func(source string, re *regexp.Regexp, left, right int, display bool) string {
			var out strings.Builder
			last := 0
			for _, loc := range stellarMathLocations(re, source, left, display) {
				value := source[loc[0]:loc[1]]
				inner := value[left : len(value)-right]
				out.WriteString(source[last:loc[0]])
				out.WriteString(wrap(value, inner, display))
				last = loc[1]
			}
			out.WriteString(source[last:])
			return out.String()
		}
		input = replace(input, reBlockDollar, 2, 2, true)
		input = replace(input, reBracketMath, 2, 2, true)
		input = replace(input, reParenMath, 2, 2, false)
		input = replace(input, reStellarInlineDollar, 1, 1, false)
		for i, p := range placeholders {
			input = strings.ReplaceAll(input, fmt.Sprintf("\x00STELLARMATH%d\x00", i), p)
		}
		return input
	}
	var out strings.Builder
	last := 0
	for _, segment := range stellarMarkdownProtectedRanges(source) {
		out.WriteString(process(source[last:segment[0]]))
		out.WriteString(source[segment[0]:segment[1]])
		last = segment[1]
	}
	out.WriteString(process(source[last:]))
	return out.String(), replacements
}
func escapeStellarMath(value string) string {
	text := html.EscapeString(value)
	for _, pair := range [][2]string{{`\`, `&#92;`}, {"*", "&#42;"}, {"_", "&#95;"}, {"`", "&#96;"}, {"~", "&#126;"}} {
		text = strings.ReplaceAll(text, pair[0], pair[1])
	}
	return text
}

var reStellarInlineDollar = regexp.MustCompile(`\$((?:\\.|[^$\\\n])+)\$`)

// 拒绝转义或空白边界时只越过开符，不能吞掉后面的真实公式开符。
func stellarMathLocations(re *regexp.Regexp, source string, left int, display bool) [][2]int {
	var locations [][2]int
	offset := 0
	for offset < len(source) {
		relative := re.FindStringIndex(source[offset:])
		if relative == nil {
			break
		}
		start, end := offset+relative[0], offset+relative[1]
		slashes := 0
		for i := start - 1; i >= 0 && source[i] == '\\'; i-- {
			slashes++
		}
		inner := source[start+left : end-left]
		if slashes%2 == 1 || (!display && strings.TrimSpace(inner) != inner) {
			offset = start + left
			continue
		}
		locations = append(locations, [2]int{start, end})
		offset = end
	}
	return locations
}
