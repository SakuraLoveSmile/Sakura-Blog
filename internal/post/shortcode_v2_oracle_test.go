package post

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"

	"blog/internal/stellarv2"
	"github.com/dop251/goja"
	"github.com/goccy/go-yaml"
	nethtml "golang.org/x/net/html"
)

type v2Oracle struct {
	vm   *goja.Runtime
	tags map[string]goja.Callable
	r    *shortcodeV2Renderer
}

func newV2Oracle(t *testing.T, ctx StellarRenderContext) *v2Oracle {
	t.Helper()
	s := NewService("")
	theme := v2Merge(stellarv2.DefaultTheme(), ctx.Theme)
	language := ctx.Language
	if language == "" {
		language = "zh-CN"
	}
	r := &shortcodeV2Renderer{s: s, tags: v2Object(theme["tags"]), services: v2Object(theme["services"]), fallbacks: v2Object(theme["fallbacks"]), theme: theme, data: ctx.Data, page: ctx.Page, index: map[string]int{}, language: language}
	vm := goja.New()
	oracle := &v2Oracle{vm: vm, tags: map[string]goja.Callable{}, r: r}
	_ = vm.Set("console", map[string]any{"warn": func(...any) {}, "error": func(...any) {}})
	_ = vm.Set("URL", func(call goja.ConstructorCall) *goja.Object {
		u, e := url.Parse(call.Argument(0).String())
		if e != nil || !u.IsAbs() {
			panic(vm.NewTypeError("invalid URL"))
		}
		o := call.This
		_ = o.Set("origin", u.Scheme+"://"+u.Host)
		_ = o.Set("hostname", u.Hostname())
		_ = o.Set("pathname", u.EscapedPath())
		return o
	})
	helpers := map[string]goja.Value{}
	ctxObject := vm.NewObject()
	_ = ctxObject.Set("stellar", map[string]any{"config": theme, "data": ctx.Data})
	_ = ctxObject.Set("config", map[string]any{"language": language, "url": "https://example.test/", "root": "/"})
	_ = ctxObject.Set("render", map[string]any{"renderSync": func(input map[string]any) any {
		if input["engine"] == "yaml" {
			m := map[string]any{}
			if e := yaml.Unmarshal([]byte(v2String(input["text"])), &m); e != nil {
				panic(vm.NewTypeError(e.Error()))
			}
			return m
		}
		var b strings.Builder
		if e := s.md.Convert([]byte(v2String(input["text"])), &b); e != nil {
			panic(e)
		}
		return b.String()
	}})
	_ = ctxObject.Set("theme", map[string]any{"i18n": map[string]any{"__": func(string) any {
		return func(call goja.FunctionCall) goja.Value {
			key := call.Argument(0).String()
			var args []any
			for _, v := range call.Arguments[1:] {
				args = append(args, v.Export())
			}
			return vm.ToValue(r.localized(key, args...))
		}
	}}})
	_ = ctxObject.Set("extend", map[string]any{"tag": map[string]any{"register": func(name string, value goja.Value, _ ...any) {
		f, ok := goja.AssertFunction(value)
		if !ok {
			t.Fatalf("tag %s not callable", name)
		}
		oracle.tags[name] = f
	}}, "helper": map[string]any{"register": func(name string, value goja.Value) { helpers[name] = value }, "get": func(name string) goja.Value { return helpers[name] }}})
	_ = vm.Set("hexo", ctxObject)
	cache := map[string]goja.Value{}
	var require func(string, string) goja.Value
	require = func(path, base string) goja.Value {
		if path == "hexo-util" {
			return vm.ToValue(map[string]any{"escapeHTML": func(v any) string { return html.EscapeString(v2String(v)) }, "url_for": func(v string) string { return v }, "full_url_for": func(v string) string { return r.fullURL(v) }})
		}
		var filename string
		if strings.HasPrefix(path, ".") {
			filename = filepath.Clean(filepath.Join(base, path))
		} else {
			filename = path
		}
		if filepath.Ext(filename) == "" {
			filename += ".js"
		}
		if v, ok := cache[filename]; ok {
			return v
		}
		if !strings.HasPrefix(filename, "testdata/stellar_v2_oracle/") {
			panic("oracle require outside fixed fixtures")
		}
		source, e := readV2OracleFixture(filename)
		if e != nil {
			t.Fatal(e)
		}
		wrapper, e := vm.RunString("(function(require,module,exports){" + string(source) + "\n})")
		if e != nil {
			t.Fatalf("%s: %v", filename, e)
		}
		f, _ := goja.AssertFunction(wrapper)
		module := vm.NewObject()
		_ = module.Set("exports", vm.NewObject())
		_, e = f(goja.Undefined(), vm.ToValue(func(path string) goja.Value { return require(path, filepath.Dir(filename)) }), module, module.Get("exports"))
		if e != nil {
			t.Fatalf("%s: %v", filename, e)
		}
		cache[filename] = module.Get("exports")
		return module.Get("exports")
	}
	utility := require("testdata/stellar_v2_oracle/lib/utils.js", "")
	uf, _ := goja.AssertFunction(utility)
	if _, e := uf(goja.Undefined(), ctxObject); e != nil {
		t.Fatal(e)
	}
	// 图标资源不是标签oracle范围；共同注入稳定占位，参数仍由各renderer决定。
	utils := ctxObject.Get("utils").ToObject(vm)
	_ = utils.Set("icon", func(key string, args ...any) string {
		more := ""
		if len(args) > 0 && args[0] != nil {
			more = " " + v2String(args[0])
		}
		return r.icon(key, more)
	})
	require("testdata/stellar_v2_oracle/lib/dynamic_color.js", "")
	require("testdata/stellar_v2_oracle/tags/index.js", "")
	require("testdata/stellar_v2_oracle/tags/inline-labels.js", "")
	return oracle
}
func (o *v2Oracle) render(t *testing.T, name, args, body string) string {
	t.Helper()
	tokens, e := v2Args(args)
	if e != nil {
		t.Fatal(e)
	}
	f, ok := o.tags[name]
	if !ok {
		t.Fatalf("official tag %s absent", name)
	}
	v, e := f(goja.Undefined(), o.vm.ToValue(tokens), o.vm.ToValue(body))
	if e != nil {
		t.Fatalf("%s oracle: %v", name, e)
	}
	if goja.IsUndefined(v) || goja.IsNull(v) {
		return ""
	}
	return v.String()
}

func v2Canonical(t *testing.T, source string) string {
	t.Helper()
	doc, e := nethtml.Parse(strings.NewReader(source))
	if e != nil {
		t.Fatal(e)
	}
	var b strings.Builder
	var visit func(*nethtml.Node)
	visit = func(n *nethtml.Node) {
		switch n.Type {
		case nethtml.ElementNode:
			if n.Data == "html" || n.Data == "head" || n.Data == "body" {
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					visit(c)
				}
				return
			}
			b.WriteString("<" + n.Data)
			var attrs []string
			for _, a := range n.Attr {
				if a.Key == "onerror" || a.Key == "data-fallback" {
					continue
				}
				value := html.EscapeString(strings.Join(strings.Fields(a.Val), " "))
				value = regexp.MustCompile(`chat-[0-9]+`).ReplaceAllString(value, "chat-N")
				attrs = append(attrs, a.Key+"="+value)
			}
			sort.Strings(attrs)
			for _, a := range attrs {
				b.WriteString(" " + a)
			}
			b.WriteString(">")
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				visit(c)
			}
			b.WriteString("</" + n.Data + ">")
		case nethtml.TextNode:
			text := strings.Join(strings.Fields(n.Data), " ")
			if n.Parent != nil && n.Parent.Data == "style" {
				text = strings.Join(strings.Fields(text), "")
				text = regexp.MustCompile(`chat-[0-9]+`).ReplaceAllString(text, "chat-N")
			}
			if text != "" {
				b.WriteString(html.EscapeString(text))
			}
		case nethtml.CommentNode:
		default:
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				visit(c)
			}
		}
	}
	visit(doc)
	return b.String()
}

type v2OracleCase struct{ name, args, body string }

func v2OfficialCases() []v2OracleCase {
	cases := []v2OracleCase{
		{"copy", `""`, ""},
		{"video", `/video.mp4 width:calc(100%-20px) autoplay:0`, ""},
		{"video", `bilibili:BV123 autoplay:1`, ""},
		{"image", `/image.png Alt width:calc(100%-20px) padding:"8px 16px" bg:linear-gradient(red,blue) fancybox:false`, ""},
		{"grid", `w:min(280px,100%) gap:"clamp(8px,2vw,16px)" br:var(--radius)`, "<!-- cell -->\nbody"},
		{"poetry", `author:'A "quote"' footer:'End "quote"' 'Title "quoted"'`, "**body**"},
		{"reel", `author:'A "quote"' 'Title "quoted"'`, "**body**"},
		{"paper", `title:'Title "quoted"' author:'A "quote"'`, "<!-- paragraph -->\n**body**"},
		{"tabs", "active:2 align:center", "<!-- tab First -->\n**one**\n<!-- tab Second -->\ntwo"}, {"tabs", "active:0", "intro\n<!-- tab First -->\n**one**"},
		{"box", "color:cyan child:tabs Title", "**body**"}, {"folding", "color:yellow child:iframe open:true Title", "**body**"}, {"folders", "color:red", "<!-- folder One -->\n**body**\n<!-- folder Two -->\ntwo"},
		{"grid", "bg:card c:2 gap:16px br:12px", "<!-- cell -->\n**left**\n<!-- cell -->\nright"}, {"grid", "", "<!-- cell -->\nleft"}, {"swiper", "width:max effect:cards", "![first](/one.png)\n![second](/two.png)"},
		{"gallery", "layout:flow size:l aspect_ratio:portrait", "![first](/one.png)\n![second](/two.png)"}, {"gallery", "", "![first](/one.png)"},
		{"banner", "Title Subtitle bg:/bg.png avatar:/avatar.png link:/target", ""}, {"banner", "Title Subtitle", ""},
		{"dropdown", "direction:up align:right open:true icon:star Menu", "- icon:star [One](/one)\n- [Two](https://example.test) icon:star"},
		{"friends", "repo:alice/blog posts:true", ""}, {"friends", "repo:alice/blog posts:false", ""}, {"albums", "repo:alice/blog size:l", ""}, {"posters", "api:https://example.test/api", ""}, {"sites", "repo:alice/blog", ""},
		{"ghcard", "alice/repo show_owner:false theme:dark hide_border:false locale:en", ""}, {"ghcard", "alice", ""},
		{"toc", "wiki:docs open:false display:mobile Guide", ""}, {"toc", "wiki:single Guide", ""}, {"toc", "wiki:missing Guide", ""}, {"toc", "", ""},
		{"timeline", "type:memos api:https://example.test/api user:alice limit:0 hide:footer custom:false", "<!-- node **Today** -->\n**body**"}, {"timeline", "", "<!-- node Today -->\none\n<!-- node Yesterday -->\ntwo"},
		{"md", "https://raw.githubusercontent.com/alice/blog/main/README.md wrap:false", ""}, {"md", "https://example.test/readme.md", ""},
		{"checkbox", "checked:false color:green symbol:plus Task text", ""}, {"checkbox", "checked:true Task", ""}, {"radio", "checked:true color:blue Choice", ""},
		{"copy", "git:ssh prefix:$ alice/blog", ""}, {"copy", "git:gh alice/blog", ""}, {"copy", "two words with spaces", ""},
		{"emoji", "qq smile height:2em", ""}, {"emoji", "smile", ""}, {"emoji", "url:https://example.test/emoji.png name:Alt height:32px", ""},
		{"icon", "star text color:cyan style:width:2em", ""}, {"frame", "iphone11 img:/image.png focus:top alt", ""}, {"frame", "ipad video:/video.mp4 img:/poster.png focus:bottom alt", ""},
		{"image", "/image.png Alt width:400px height:300px bg:#eee padding:16px fancybox:false ratio:16/9 download:true", ""}, {"image", "/image.png Alt fancybox:/large.png download:false", ""},
		{"link", "https://example.test title desc:false icon:/icon.png", ""}, {"link", "/post", ""}, {"button", "color:red icon:star size:xs Hello World /target", ""}, {"button", "Label /target", ""},
		{"mark", "color:yellow tip:Explanation Highlight", ""}, {"navbar", "active:/two [One](/one) [Two](/two) Anchor", ""}, {"note", "color:blue Title **content** and more", ""}, {"note", "singleword", ""},
		{"poetry", "Title author:Author date:Today footer:End", "**verse**"}, {"quot", "el:h2 icon:question color:cyan Heading", ""}, {"quot", "el:p prefix:star suffix:heart Quote words", ""},
		{"blockquote", "indent:false", "**quote**"}, {"hashtag", "Label /tag color:red", ""}, {"hashtag", "Label /tag", ""},
		{"mbti", "INTJ-T 93-72-78-83-51 url:person-id image:/banner.png", ""}, {"mbti", "unknown 0-150-12.4-xxx-51", ""},
		{"okr", "o1 percent:0 status:at_risk color:cyan", "Objective\nNote\n<!-- okr kr1 percent:30.8 status:finished -->\nKR one\n**note**\n<!-- okr kr2 percent:70 -->\nKR two\nNote"},
		{"audio", "/audio.mp3 type:audio/mp3", ""}, {"audio", "netease:123 type:2 autoplay:true", ""}, {"video", "/video.mp4 type:video/mp4 width:800px", ""}, {"video", "bilibili:BV123 ratio:16/9 autoplay:false", ""}, {"video", "youtube:video-id autoplay:1", ""},
		{"rating", "id:foo icon:star Rating title", ""}, {"vote", "id:foo yes:star no:heart Vote title", ""},
		{"table", "style:compact", "| A | B |\n| - | - |\n| 1 | 2 |"}, {"tip", "Word pop:Explanation", ""},
		{"reel", "Title author:Author date:Today footer:End", "**body**"}, {"paper", "style:underline title:Title author:Author date:Today footer:End", "<!-- section First -->\n**body**\n<!-- paragraph -->\ntext\n<!-- line right -->\nSigned"},
	}
	for _, name := range []string{"u", "emp", "wavy", "del", "kbd", "psw", "blur"} {
		cases = append(cases, v2OracleCase{name, "two words", ""})
	}
	for _, name := range []string{"sup", "sub"} {
		cases = append(cases, v2OracleCase{name, "color:blue text words", ""})
	}
	for _, style := range []string{"qq", "wechat"} {
		cases = append(cases, v2OracleCase{"chat", "iphone style:" + style + " title:Chat scene:group me:user2 labelColorStyle:hand", `user1:
  name: Alice
  avatar: /alice.png
  label:
    text: Admin
    textColor: white
    bgColor: '#2196f3'
user2:
  name: Bob
  avatar: /bob.png
<!-- chattip Today -->
<!-- chatcell user:user1 md:Hello **world** tag:one -->
<!-- chatcell user:user2 md:Reply quote:one -->
<!-- chatcell user:user1 image:/image.png tag:image -->
<!-- chatcell user:user2 md:Image reply quote:image -->
<!-- chatcell user:user2 emoji:smile source:qq -->
<!-- chatcell user:user1 voice:/voice.mp3 type:audio/mp3 -->
<!-- chatcell user:user2 video:/video.mp4 type:video/mp4 -->
<!-- chatcell user:user1 link:https://example.test from:QQ浏览器 -->
<!-- chatcell user:user2 file:/file.pdf -->`})
	}
	for _, kind := range []string{"friends", "albums", "posters", "sites"} {
		cases = append(cases, v2OracleCase{kind, "group", ""})
	}
	return cases
}
func TestStellarV2OfficialRendererOracle(t *testing.T) {
	context := StellarRenderContext{Theme: map[string]any{"services": map[string]any{"siteInfo": map[string]any{"provider": "site_info_api", "site_info_api": map[string]any{"endpoint": "https://example.test/info?url={href}"}}}}, Data: map[string]any{"icons": map[string]any{}, "siteURL": "https://example.test/", "links": map[string]any{"group": []any{map[string]any{"url": "https://example.test/", "title": "Site", "description": "Desc"}}}, "wiki": map[string]any{"tree": map[string]any{"docs": map[string]any{"sections": []any{map[string]any{"title": "First", "pages": []any{map[string]any{"path": "/one", "title": "One"}}}, map[string]any{"title": "Second", "pages": []any{map[string]any{"path": "/two", "title": "Two"}}}}}, "single": map[string]any{"pages": []any{map[string]any{"path": "/one", "title": "One"}}}}}}}
	seen := map[string]bool{}
	for i, c := range v2OfficialCases() {
		t.Run(fmt.Sprintf("%03d_%s", i, c.name), func(t *testing.T) {
			oracle := newV2Oracle(t, context)
			entry, _ := shortcodeV2Entry(c.name)
			tokens, _ := v2Args(c.args)
			got := oracle.r.render(c.name, v2MapArgs(tokens, entry.Params), c.body, c.args)
			want := oracle.render(t, c.name, c.args, c.body)
			if a, b := v2Canonical(t, got), v2Canonical(t, want); a != b {
				t.Errorf("got %s\nwant %s", a, b)
			}
		})
		seen[c.name] = true
	}
	for _, e := range ShortcodeCatalogStellarV2() {
		if e.Name != "gist" && !seen[e.Name] {
			t.Errorf("official tag %s has no renderer oracle", e.Name)
		}
	}
	if len(ShortcodeCatalogStellarV2()) != 54 {
		t.Errorf("official catalog count=%d", len(ShortcodeCatalogStellarV2()))
	}
}

func TestStellarV2OfficialFixtureFingerprint(t *testing.T) {
	// 测试输入身份单独验证；不把复制率当渲染行为的证据。
	data, e := os.ReadFile("testdata/stellar_v2_oracle/fingerprints.json")
	if e != nil {
		t.Fatal(e)
	}
	var fingerprints map[string]string
	if e = json.Unmarshal(data, &fingerprints); e != nil {
		t.Fatal(e)
	}
	for file, want := range fingerprints {
		data, e := readV2OracleFixture(filepath.Join("testdata/stellar_v2_oracle", file))
		if e != nil {
			t.Fatal(e)
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(data)); got != want {
			t.Errorf("fixture %s hash changed: %s", file, got)
		}
	}
	if len(fingerprints) != 52 {
		t.Fatalf("unexpected fixtures=%d", len(fingerprints))
	}
}

var v2OracleSourceOnce sync.Once
var v2OracleSources map[string][]byte
var v2OracleSourceError error

func readV2OracleFixture(filename string) ([]byte, error) {
	v2OracleSourceOnce.Do(func() {
		v2OracleSources = map[string][]byte{}
		compressed, e := os.ReadFile("testdata/stellar_v2_oracle/source.tar.gz")
		if e != nil {
			v2OracleSourceError = e
			return
		}
		reader, e := gzip.NewReader(bytes.NewReader(compressed))
		if e != nil {
			v2OracleSourceError = e
			return
		}
		defer reader.Close()
		archive := tar.NewReader(reader)
		for {
			header, e := archive.Next()
			if e == io.EOF {
				break
			}
			if e != nil {
				v2OracleSourceError = e
				return
			}
			data, e := io.ReadAll(archive)
			if e != nil {
				v2OracleSourceError = e
				return
			}
			v2OracleSources[filepath.Join("testdata/stellar_v2_oracle", header.Name)] = data
		}
	})
	if v2OracleSourceError != nil {
		return nil, v2OracleSourceError
	}
	if source, ok := v2OracleSources[filepath.Clean(filename)]; ok {
		return source, nil
	}
	return nil, fmt.Errorf("official fixture module missing: %s", filename)
}
