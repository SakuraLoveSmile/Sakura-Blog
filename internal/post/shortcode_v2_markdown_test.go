package post

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dop251/goja"
	nethtml "golang.org/x/net/html"
)

type offlineCountingGithub struct{ repos, users int }

func (c *offlineCountingGithub) GetRepo(owner, repo string) (*GHRepoInfo, error) {
	c.repos++
	return &GHRepoInfo{FullName: owner + "/" + repo, Description: "live callback", Stars: 42}, nil
}
func (c *offlineCountingGithub) GetUser(name string) (*GHUserInfo, error) {
	c.users++
	return &GHUserInfo{Login: name, Name: "live callback", HTMLURL: "https://github.com/" + name}, nil
}
func TestStellarV2OfflineGHUsesCurrentContextWithoutNetwork(t *testing.T) {
	s := NewService("")
	client := &offlineCountingGithub{}
	s.SetGHClient(client)
	body := "{% blog_ghrepo octocat/hello-world %}\n\n{% blog_ghuser octocat %}\n\n{% button Label /target %}"
	ctx := StellarRenderContext{Offline: true, BlogExtensions: true, Theme: map[string]any{"tags": map[string]any{"button": map[string]any{"defaultColor": "cyan"}}, "services": map[string]any{"github": map[string]any{"apiUrl": "https://github-api.test"}}}}
	out, e := s.RenderStellarV2WithContext(body, ctx)
	if e != nil {
		t.Fatal(e)
	}
	if client.repos != 0 || client.users != 0 {
		t.Fatal("offline renderer requested GitHub")
	}
	for _, want := range []string{`ds-ghinfo ds-ghrepo`, `ds-ghinfo ds-ghuser`, `data-api="https://github-api.test/repos/octocat/hello-world"`, `data-api="https://github-api.test/users/octocat"`, `id="html_url"`, `type="text" id="full_name"`, `color="cyan"`} {
		if !strings.Contains(out, want) {
			t.Fatal(out)
		}
	}
	ctx.Offline = false
	out, e = s.RenderStellarV2WithContext(body, ctx)
	if e != nil {
		t.Fatal(e)
	}
	if client.repos != 1 || client.users != 1 || !strings.Contains(out, "live callback") {
		t.Fatal("normal request lost its configured callback", out)
	}
}
func TestStellarV2AllMermaidFencesRetainSourceAndChromaCode(t *testing.T) {
	s := NewService("")
	for _, syntax := range []string{"flowchart LR\n  A[\"<tag>\"] --> B", "sequenceDiagram\n  Alice->>Bob: $$cost$$", "classDiagram\n  Animal <|-- Dog", "stateDiagram-v2\n  [*] --> Ready", "erDiagram\n  CAR ||--o{ PART : has", "gantt\n  title Schedule\n  section Work\n  Task :a1, 2026-01-01, 1d", "pie\n  \"One\" : 1", "mindmap\n  root((Root))", "gitGraph\n  commit", "timeline\n  2026 : Complete", "xychart-beta\n  x-axis [a,b]\n  bar [1,2]", "quadrantChart\n  A: [0.3,0.6]"} {
		for _, fence := range []string{"```", "~~~~"} {
			body := fence + "mermaid\n" + syntax + "\n" + fence + "\n\n```go\nfmt.Println(42)\n```"
			out, e := s.RenderStellarV2(body, nil)
			if e != nil {
				t.Fatal(e)
			}
			doc := parseStellarHTML(t, out)
			nodes := selectStellarClass(doc, "mermaid")
			if len(nodes) != 1 || stellarText(nodes[0]) != syntax+"\n" {
				t.Fatalf("fence content changed: %s", out)
			}
			if !strings.Contains(out, `data-language="mermaid"`) || !strings.Contains(out, `class="chroma"`) || !strings.Contains(out, `class="lntable"`) {
				t.Fatal("diagram or ordinary Chroma lost", out)
			}
		}
	}
}
func TestStellarV2MathJaxMarkupPreservesTeXAndCode(t *testing.T) {
	s := NewService("")
	body := `Inline $a_b*c$ and \(\alpha_{n}\), block:

$$\begin{cases}x<y\\x>y\end{cases}$$

\[\sum_{i=1}^n i\]

\$5 and $10.

` + "`$code$`\n\n```mermaid\nflowchart LR\n A[\"$$not-math$$\"]\n```"
	out, e := s.RenderStellarV2WithContext(body, StellarRenderContext{Theme: map[string]any{"features": map[string]any{"math": map[string]any{"provider": "mathjax"}}}})
	if e != nil {
		t.Fatal(e)
	}
	nodes := selectStellarClass(parseStellarHTML(t, out), "has-jax")
	if len(nodes) != 4 {
		t.Fatalf("expected 4 formula DOM nodes: %s", out)
	}
	for i, want := range []string{`$a_b*c$`, `\(\alpha_{n}\)`, `$$\begin{cases}x<y\\x>y\end{cases}$$`, `\[\sum_{i=1}^n i\]`} {
		if stellarText(nodes[i]) != want {
			t.Errorf("TeX changed %q => %q", want, stellarText(nodes[i]))
		}
	}
	if !strings.Contains(out, `<code>$code$</code>`) || !strings.Contains(stellarText(parseStellarHTML(t, out)), `$$not-math$$`) {
		t.Fatal("code/math boundary failed", out)
	}
}
func TestStellarV2KatexActualGoldenDOMOptionsAndErrors(t *testing.T) {
	cases := []struct {
		tex     string
		display bool
		options map[string]any
	}{
		{`E=mc^2`, false, nil}, {`\frac{1}{2}+\sqrt{x}`, true, nil}, {`\begin{cases}x & x>0\\-x & x\le0\end{cases}`, true, nil},
		{`\R^n`, false, map[string]any{"macros": map[string]any{`\R`: `\mathbb{R}`}, "fleqn": true, "leqno": true}},
		{`x+y`, false, map[string]any{"output": "html", "minRuleThickness": json.Number("0.05"), "maxSize": json.Number("12")}},
		{`\invalidcommand`, false, map[string]any{"throwOnError": false, "errorColor": "#ff0000"}},
	}
	s := NewService("")
	for _, c := range cases {
		ctx := StellarRenderContext{Offline: true, Theme: map[string]any{"features": map[string]any{"math": map[string]any{"provider": "katex", "katex": c.options}}}}
		body := "$" + c.tex + "$"
		if c.display {
			body = "$$" + c.tex + "$$"
		}
		got, e := s.RenderStellarV2WithContext(body, ctx)
		if e != nil {
			t.Fatal(e)
		}
		want := directStellarKatex(t, c.tex, c.display, c.options)
		className := "katex"
		expected := selectStellarClass(parseStellarHTML(t, want), className)
		if len(expected) == 0 {
			className = "katex-error"
			expected = selectStellarClass(parseStellarHTML(t, want), className)
		}
		actual := selectStellarClass(parseStellarHTML(t, got), className)
		if len(actual) != 1 || len(expected) != 1 || v2Canonical(t, stellarNodeHTML(actual[0])) != v2Canonical(t, stellarNodeHTML(expected[0])) {
			t.Fatalf("actual KaTeX DOM differs\n%s\n%s", got, want)
		}
		if c.options == nil && (!strings.Contains(got, "katex-mathml") || !strings.Contains(got, "katex-html")) {
			t.Fatal("not real HTML+MathML", got)
		}
	}
	if _, e := s.RenderStellarV2WithContext(`$\invalidcommand$`, StellarRenderContext{Page: map[string]any{"render": map[string]any{"math": "katex"}}}); e == nil || !strings.Contains(e.Error(), "Undefined control sequence") {
		t.Fatal("KaTeX error semantics swallowed", e)
	}
	nested := `{% box %}` + "\n" + `$x_y$` + "\n{% endbox %}"
	out, e := s.RenderStellarV2WithContext(nested, StellarRenderContext{Page: map[string]any{"render": map[string]any{"math": "katex"}}})
	if e != nil || len(selectStellarClass(parseStellarHTML(t, out), "katex")) != 1 {
		t.Fatalf("nested math double rendered %s %v", out, e)
	}
	code := "`$x$`\n\n```text\n$$\\invalidcommand$$\n```"
	out, e = s.RenderStellarV2WithContext(code, StellarRenderContext{Page: map[string]any{"render": map[string]any{"math": "katex"}}})
	if e != nil || strings.Contains(out, "katex") {
		t.Fatalf("code formula executed %s %v", out, e)
	}
}
func TestStellarV2KatexConcurrentMacroIsolation(t *testing.T) {
	s := NewService("")
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			symbol := fmt.Sprint(i + 1)
			macros := map[string]any{`\shared`: symbol}
			ctx := StellarRenderContext{Offline: true, Theme: map[string]any{"features": map[string]any{"math": map[string]any{"provider": "katex", "katex": map[string]any{"macros": macros, "globalGroup": true}}}}}
			out, e := s.RenderStellarV2WithContext(`$\gdef\local{z}\shared$`, ctx)
			if e != nil {
				t.Error(e)
				return
			}
			if !strings.Contains(out, `>`+symbol+`</mn>`) || len(macros) != 1 {
				t.Errorf("macro context leaked or mutated: %s %#v", out, macros)
			}
		}(i)
	}
	wg.Wait()
	if _, e := s.RenderStellarV2WithContext(`$\local$`, StellarRenderContext{Page: map[string]any{"render": map[string]any{"math": "katex"}}}); e == nil {
		t.Fatal("gdef leaked to different request")
	}
}
func TestStellarV2KatexSourcePinnedAndBounded(t *testing.T) {
	raw, e := stellarKatexFiles.ReadFile("math_katex_data/source.json")
	if e != nil {
		t.Fatal(e)
	}
	var meta struct {
		Version string            `json:"version"`
		Files   map[string]string `json:"files"`
	}
	if e = json.Unmarshal(raw, &meta); e != nil {
		t.Fatal(e)
	}
	if meta.Version != "0.16.23" {
		t.Fatal(meta.Version)
	}
	for name, want := range meta.Files {
		data, e := stellarKatexFiles.ReadFile("math_katex_data/" + name)
		if e != nil {
			t.Fatal(e)
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(data)); got != want {
			t.Fatalf("source drift %s", name)
		}
	}
	if _, e := NewService("").RenderStellarV2WithContext("$"+strings.Repeat("x", stellarKatexMaxFormulaBytes+1)+"$", StellarRenderContext{Page: map[string]any{"render": map[string]any{"math": "katex"}}}); e == nil {
		t.Fatal("unbounded formula accepted")
	}
}

func directStellarKatex(t *testing.T, tex string, display bool, options map[string]any) string {
	t.Helper()
	source, e := stellarKatexFiles.ReadFile("math_katex_data/katex.min.js")
	if e != nil {
		t.Fatal(e)
	}
	vm := goja.New()
	_ = vm.Set("module", map[string]any{"exports": map[string]any{}})
	_ = vm.Set("exports", map[string]any{})
	if _, e = vm.RunString(string(source)); e != nil {
		t.Fatal(e)
	}
	f, ok := goja.AssertFunction(vm.Get("module").ToObject(vm).Get("exports").ToObject(vm).Get("renderToString"))
	if !ok {
		t.Fatal("no renderToString")
	}
	opts := map[string]any{}
	for k, v := range options {
		opts[k] = v
	}
	opts["displayMode"] = display
	raw, e := json.Marshal(opts)
	if e != nil {
		t.Fatal(e)
	}
	_ = vm.Set("goldenOptionsJSON", string(raw))
	value, e := vm.RunString("JSON.parse(goldenOptionsJSON)")
	if e != nil {
		t.Fatal(e)
	}
	out, e := f(goja.Undefined(), vm.ToValue(tex), value)
	if e != nil {
		t.Fatal(e)
	}
	return out.String()
}
func parseStellarHTML(t *testing.T, source string) *nethtml.Node {
	t.Helper()
	doc, e := nethtml.Parse(strings.NewReader(source))
	if e != nil {
		t.Fatal(e)
	}
	return doc
}
func selectStellarClass(node *nethtml.Node, class string) []*nethtml.Node {
	var result []*nethtml.Node
	var visit func(*nethtml.Node)
	visit = func(n *nethtml.Node) {
		for _, a := range n.Attr {
			if a.Key == "class" {
				for _, v := range strings.Fields(a.Val) {
					if v == class {
						result = append(result, n)
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			visit(c)
		}
	}
	visit(node)
	return result
}
func stellarText(node *nethtml.Node) string {
	if node.Type == nethtml.TextNode {
		return node.Data
	}
	var b strings.Builder
	for c := node.FirstChild; c != nil; c = c.NextSibling {
		b.WriteString(stellarText(c))
	}
	return b.String()
}
func stellarNodeHTML(node *nethtml.Node) string {
	var b strings.Builder
	_ = nethtml.Render(&b, node)
	return b.String()
}

func TestStellarV2OfficialDiagramJSConsumesRenderedFenceDOM(t *testing.T) {
	body := "```mermaid\nflowchart LR\n A[\"<tag>\"] --> B\n```"
	out, e := NewService("").RenderStellarV2(body, nil)
	if e != nil {
		t.Fatal(e)
	}
	nodes := selectStellarClass(parseStellarHTML(t, out), "mermaid")
	if len(nodes) != 1 {
		t.Fatal(out)
	}
	source, e := readV2OracleFixture("testdata/stellar_v2_oracle/runtime/extensions/diagrams.js")
	if e != nil {
		t.Fatal(e)
	}
	vm := goja.New()
	_ = vm.Set("diagramText", stellarText(nodes[0]))
	if _, e = vm.RunString(`var seenNodes=[],seenAsset='',seenTheme='';var window={matchMedia:()=>({matches:true}),mermaid:{initialize:config=>{seenTheme=config.theme},run:async options=>{seenNodes=options.nodes.map(n=>n.textContent)}}};var root={matches:()=>false,querySelectorAll:selector=>selector==='.mermaid'?[{textContent:diagramText}]:[]};var context={extension:{config:{assets:{js:'/assets/vendor/mermaid/mermaid.min.js'},colorScheme:'auto',theme:'neutral'}},assets:{script:async asset=>{seenAsset=asset}},signal:{throwIfAborted:()=>{}}};` + strings.ReplaceAll(string(source), "export async function", "async function")); e != nil {
		t.Fatal(e)
	}
	value, e := vm.RunString("mount(root,context)")
	if e != nil {
		t.Fatal(e)
	}
	promise, ok := value.Export().(*goja.Promise)
	if !ok || promise.State() != goja.PromiseStateFulfilled {
		t.Fatalf("official extension mount failed %#v", value.Export())
	}
	if vm.Get("seenTheme").String() != "dark" || vm.Get("seenAsset").String() != "/assets/vendor/mermaid/mermaid.min.js" {
		t.Fatal("provider configuration not consumed")
	}
	if items := vm.Get("seenNodes").Export().([]any); len(items) != 1 || items[0] != stellarText(nodes[0]) {
		t.Fatal("actual renderer DOM was not forwarded to Mermaid", items)
	}
}
func TestStellarV2OfficialMathJaxJSConsumesFormulaDOM(t *testing.T) {
	out, e := NewService("").RenderStellarV2WithContext(`Inline $a_b*c$`, StellarRenderContext{Page: map[string]any{"render": map[string]any{"math": "mathjax"}}})
	if e != nil {
		t.Fatal(e)
	}
	nodes := selectStellarClass(parseStellarHTML(t, out), "has-jax")
	if len(nodes) != 1 {
		t.Fatal(out)
	}
	source, e := readV2OracleFixture("testdata/stellar_v2_oracle/runtime/extensions/mathjax.js")
	if e != nil {
		t.Fatal(e)
	}
	vm := goja.New()
	_ = vm.Set("formulaText", stellarText(nodes[0]))
	if _, e = vm.RunString(`var seenAsset='',seenRoot=null,cleanup=null;var window={};var root={textContent:formulaText,querySelectorAll:()=>[]};var context={extension:{config:{asset:'/mathjax.js'}},assets:{script:async asset=>{seenAsset=asset;window.MathJax.startup={promise:Promise.resolve()};window.MathJax.typesetPromise=async roots=>{seenRoot=roots[0]};window.MathJax.typesetClear=roots=>{};}},signal:{aborted:false,throwIfAborted:()=>{}},onCleanup:fn=>{cleanup=fn}};` + strings.ReplaceAll(string(source), "export async function", "async function")); e != nil {
		t.Fatal(e)
	}
	value, e := vm.RunString("mount(root,context)")
	if e != nil {
		t.Fatal(e)
	}
	promise, ok := value.Export().(*goja.Promise)
	if !ok || promise.State() != goja.PromiseStateFulfilled {
		t.Fatalf("official MathJax mount failed %#v", value.Export())
	}
	if vm.Get("seenAsset").String() != "/mathjax.js" || vm.Get("seenRoot").ToObject(vm).Get("textContent").String() != `$a_b*c$` {
		t.Fatal("actual formula DOM was not consumed")
	}
	if _, ok := goja.AssertFunction(vm.Get("cleanup")); !ok {
		t.Fatal("no official cleanup installed")
	}
}

func TestStellarV2MathDoesNotRewriteHTMLAttributesOrScriptCode(t *testing.T) {
	body := `{% copy "$x$" %}` + "\n\n" + `<a href="/query?q=$x$">link</a>` + "\n\n" + `<script>var example='$x$';</script>`
	out, e := NewService("").RenderStellarV2WithContext(body, StellarRenderContext{Page: map[string]any{"render": map[string]any{"math": "katex"}}})
	if e != nil {
		t.Fatal(e)
	}
	if strings.Contains(out, "katex") || !strings.Contains(out, `value="$x$"`) || !strings.Contains(out, `href="/query?q=$x$"`) || !strings.Contains(out, `var example='$x$'`) {
		t.Fatal(out)
	}
}

func TestStellarV2EscapedDollarDoesNotConsumeNextFormula(t *testing.T) {
	out, e := NewService("").RenderStellarV2WithContext(`Price \$5 and $x$`, StellarRenderContext{Page: map[string]any{"render": map[string]any{"math": "katex"}}})
	if e != nil {
		t.Fatal(e)
	}
	if len(selectStellarClass(parseStellarHTML(t, out), "katex")) != 1 || !strings.Contains(out, "Price $5 and") {
		t.Fatal(out)
	}
}

func TestStellarKatexExecutionBudgetActuallyInterruptsVM(t *testing.T) {
	vm := goja.New()
	if _, e := runStellarKatex(vm, 10*time.Millisecond, func() (goja.Value, error) { return vm.RunString(`while(true){}`) }); e == nil {
		t.Fatal("execution budget did not interrupt VM")
	}
	value, e := vm.RunString(`1+1`)
	if e != nil || value.ToInteger() != 2 {
		t.Fatal("old timeout polluted next VM operation", e)
	}
}
