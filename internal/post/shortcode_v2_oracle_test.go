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
	return append(cases, v2OfficialParameterCases()...)
}

// 参数矩阵覆盖省略值、显式分支和官方实际转义的位置；期望仍由固定提交的 JS 计算。
func v2OfficialParameterCases() []v2OracleCase {
	images := "![first](/one.png)\n![](/two.png)\nnot an image"
	tabs := "<!-- tab One -->\nfirst\n<!-- tab Two -->\nsecond"
	okr := "Objective\n<!-- okr kr1 percent:25 -->\nOne\n<!-- okr kr2 percent:100 -->\nTwo"
	cases := []v2OracleCase{
		{"ghcard", "alice/repo show_owner:true title_color:112233 text_color:445566 icon_color:778899 bg_color:0,ffffff,eeeeee cache_seconds:1800 hide_border:true locale:cn theme:transparent", ""},
		{"ghcard", "alice title_color:abcdef text_color:fedcba icon_color:123456 bg_color:ffffff cache_seconds:0 show_owner:false hide_border:false", ""},
		{"gallery", "layout:unknown size:xl aspect_ratio:landscape", images},
		{"swiper", "", "no images"},
		{"image", "/image.png", ""},
		{"image", "/image.png 'Alt <tag> & \"quoted\"' fancybox:true height:200px", ""},
		{"image", "/image.png width:200px ratio:1/1", ""},
		{"image", "/image.png Alt download:/download.png", ""},
		{"frame", "iphone11 img:/image.png 'Alt <tag> & \"quoted\"'", ""},
		{"frame", "macbook video:/video.mp4", ""},
		{"frame", "iphone11", ""},
		{"frame", "img:/image.png", ""},
		{"audio", "/audio.ogg", ""},
		{"audio", "netease:123 autoplay:false", ""},
		{"video", "bilibili:BV123", ""},
		{"video", "youtube:video-id autoplay:false ratio:4/3 width:640px", ""},
		{"video", "youtube:video-id autoplay:true", ""},
		{"video", "youtube:video-id", ""},
		{"link", "/post 'Title <tag> & \"quoted\"' desc:'Description <tag> & \"quoted\"'", ""},
		{"link", "/post desc:true", ""},
		{"link", "", ""},
		{"copy", "git:https /alice/blog prefix:HTTPS", ""},
		{"copy", "", ""},
		{"emoji", "unknown smile", ""},
		{"emoji", "url:https://example.test/emoji.png", ""},
		{"emoji", "", ""},
		{"icon", "copy:copy", ""},
		{"button", "Label", ""},
		{"button", "", ""},
		{"radio", "checked:false symbol:minus Choice", ""},
		{"checkbox", "Task", ""},
		{"radio", "Choice", ""},
		{"mark", "'Highlight' tip:'Explanation <tag> & \"quoted\"'", ""},
		{"tip", "'**Word**' pop:'Explanation <tag> & \"quoted\"'", ""},
		{"tip", "Word", ""},
		{"note", "Title content without explicit color", ""},
		{"quot", "Heading", ""},
		{"quot", "el:h3 prefix:copy:copy Heading", ""},
		{"quot", "suffix:copy:copy Quote", ""},
		{"hashtag", "Label", ""},
		{"folding", "Title", "body"},
		{"folding", "child:codeblock open:false **Title**", "body"},
		{"box", "Title", "body"},
		{"box", "", "body"},
		{"folders", "", "<!-- folder One -->\nbody"},
		{"tabs", "", tabs},
		{"tabs", "active:99 align:right", tabs},
		{"tabs", "active:-1 align:left", tabs},
		{"tabs", "active:not-a-number", tabs},
		{"tabs", "", ""},
		{"dropdown", "'Menu <tag> & \"quoted\"'", "- [Item <tag> & \"quoted\"](/target?x=1&y=2)"},
		{"dropdown", "direction:invalid align:invalid open:false Menu", "- [One](/one)"},
		{"dropdown", "direction:down align:left Menu", "- [One](/one)"},
		{"dropdown", "", "- [One](/one)"},
		{"dropdown", "Menu", "not a menu item"},
		{"grid", "w:320px c:3 bg:plain", "<!-- cell -->\nbody"},
		{"grid", "", ""},
		{"banner", `bg:"/it's.png"`, `<div class="tag-plugin navbar">Navigation</div>`},
		{"banner", "", ""},
		{"navbar", "[One](/one) [Two](https://example.test)", ""},
		{"navbar", "", ""},
		{"blockquote", "indent:true", "body"},
		{"blockquote", "", "body"},
		{"table", "style:wrap", "| A |\n| - |\n| 1 |"},
		{"table", "style:invalid", "| A |\n| - |\n| 1 |"},
		{"table", "", "| A |\n| - |\n| 1 |"},
		{"toc", "wiki:docs open:true display:desktop", ""},
		{"timeline", "api:https://example.test/api user:alice,bob limit:10 hide:user,title", ""},
		{"timeline", "type:memos", "<!-- node Today -->\nbody"},
		{"timeline", "", ""},
		{"md", "https://example.test/readme.md wrap:true", ""},
		{"rating", "", ""},
		{"vote", "", ""},
		{"poetry", "", "body"},
		{"poetry", "date:Today", "body"},
		{"reel", "", "body"},
		{"reel", "date:Today", "body"},
		{"paper", "", "<!-- section -->\nbody\n<!-- line -->\nsignature"},
		{"okr", "o1", okr},
		{"okr", "o1", "Objective\n<!-- okr kr1 percent:100 -->\nOne\n<!-- okr kr2 percent:100 -->\nTwo"},
		{"okr", "o1 percent:12.9 status:unknown", okr},
		{"mbti", "entp-a 0-100-50-25-75 url:https://example.test/person", ""},
		{"mbti", "INFJ 1-2-3-4-5", ""},
		{"mbti", "", ""},
	}
	for _, effect := range []string{"slide", "fade", "coverflow", "cards"} {
		for _, width := range []string{"min", "max"} {
			cases = append(cases, v2OracleCase{"swiper", "width:" + width + " effect:" + effect, images})
		}
	}
	for _, layout := range []string{"grid", "flow"} {
		for _, size := range []string{"mix", "s", "m", "l", "xl"} {
			for _, ratio := range []string{"original", "square", "portrait", "landscape"} {
				cases = append(cases, v2OracleCase{"gallery", "layout:" + layout + " size:" + size + " aspect_ratio:" + ratio, images})
			}
		}
	}
	for _, kind := range []string{"albums", "posters", "friends", "sites"} {
		for _, args := range []string{"", "missing", "api:https://example.test/api", "group repo:alice/blog api:https://example.test/override"} {
			cases = append(cases, v2OracleCase{kind, args, ""})
		}
	}
	for _, kind := range []string{"albums", "posters"} {
		for _, size := range []string{"xs", "s", "m", "l", "xl", "mix"} {
			cases = append(cases, v2OracleCase{kind, "group size:" + size, ""})
		}
	}
	for _, status := range []string{"in_track", "at_risk", "off_track", "finished", "unfinished"} {
		cases = append(cases, v2OracleCase{"okr", "o1 percent:100 status:" + status, "Objective\n<!-- okr kr1 percent:0 status:" + status + " -->\nOne\n<!-- okr kr2 percent:100 status:" + status + " -->\nTwo"})
	}
	for _, name := range []string{"u", "emp", "wavy", "del", "kbd", "psw", "blur", "sup", "sub"} {
		cases = append(cases, v2OracleCase{name, "'text & entity'", ""})
	}
	chatBody := "alice:\n  name: Alice\n  avatar: /alice.png\n  label:\n    text: Admin\n    textColor: white\n    bgColor: '#2196f3'\n<!-- chattip Today -->\n<!-- chatcell user:alice md:Hello tag:first -->\n<!-- chatcell user:alice md:Reply quote:first -->\n<!-- chatcell user:alice emoji:smile -->"
	for _, style := range []string{"qq", "wechat"} {
		cases = append(cases, v2OracleCase{"chat", "style:" + style + " scene:person labelColorStyle:dynamic", chatBody})
		cases = append(cases, v2OracleCase{"chat", "style:" + style, chatBody})
	}
	for _, ext := range []string{"docx", "pptx", "xlsx", "pdf", "md", "zip", "js", "mp4", "png", "mp3", "json", "sqlite", "obj", "sh", "lnk", "unknown", ""} {
		filename := "/file"
		if ext != "" {
			filename += "." + ext
		}
		cases = append(cases, v2OracleCase{"chat", "iphone style:qq title:Files", "alice:\n  name: Alice\n  avatar: /alice.png\n<!-- chatcell user:alice file:" + filename + " -->"})
	}
	return cases
}

func TestStellarV2OfficialRendererOracle(t *testing.T) {
	context := StellarRenderContext{Theme: map[string]any{"services": map[string]any{"siteInfo": map[string]any{"provider": "site_info_api", "site_info_api": map[string]any{"endpoint": "https://example.test/info?url={href}"}}}}, Data: map[string]any{"icons": map[string]any{}, "siteURL": "https://example.test/", "links": map[string]any{"group": []any{map[string]any{"url": "https://example.test/", "title": "Site", "description": "Desc"}}}, "wiki": map[string]any{"tree": map[string]any{"docs": map[string]any{"sections": []any{map[string]any{"title": "First", "pages": []any{map[string]any{"path": "/one", "title": "One"}}}, map[string]any{"title": "Second", "pages": []any{map[string]any{"path": "/two", "title": "Two"}}}}}, "single": map[string]any{"pages": []any{map[string]any{"path": "/one", "title": "One"}}}}}}}
	seen := map[string]bool{}
	var strictDOM, decodedQuery, emptyRules int
	for i, c := range v2OfficialCases() {
		t.Run(fmt.Sprintf("%03d_%s", i, c.name), func(t *testing.T) {
			oracle := newV2Oracle(t, context)
			entry, _ := shortcodeV2Entry(c.name)
			tokens, _ := v2Args(c.args)
			got := oracle.r.render(c.name, v2MapArgs(tokens, entry.Params), c.body, c.args)
			want := oracle.render(t, c.name, c.args, c.body)
			if a, b := v2OfficialComparable(t, c.name, got), v2OfficialComparable(t, c.name, want); a != b {
				t.Errorf("got %s\nwant %s", a, b)
			} else if v2Canonical(t, got) == v2Canonical(t, want) {
				strictDOM++
			} else if c.name == "ghcard" {
				decodedQuery++
			} else if c.name == "chat" {
				emptyRules++
			}
		})
		seen[c.name] = true
	}
	t.Logf("parameter oracle: total=%d strict DOM=%d decoded query equivalence=%d empty CSS equivalence=%d", len(v2OfficialCases()), strictDOM, decodedQuery, emptyRules)
	for _, e := range ShortcodeCatalogStellarV2() {
		if e.Name != "gist" && !seen[e.Name] {
			t.Errorf("official tag %s has no renderer oracle", e.Name)
		}
	}
	if len(ShortcodeCatalogStellarV2()) != 54 {
		t.Errorf("official catalog count=%d", len(ShortcodeCatalogStellarV2()))
	}
}

// 只归一有明确浏览器语义依据的差异，不放宽可见 DOM、参数或非空样式的比较。
func v2OfficialComparable(t *testing.T, tag, source string) string {
	t.Helper()
	if tag != "ghcard" && tag != "chat" {
		return v2Canonical(t, source)
	}
	doc, err := nethtml.Parse(strings.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	var visit func(*nethtml.Node)
	visit = func(n *nethtml.Node) {
		if tag == "ghcard" && n.Type == nethtml.ElementNode && n.Data == "img" {
			for i, attr := range n.Attr {
				if attr.Key != "src" {
					continue
				}
				u, err := url.Parse(attr.Val)
				if err != nil {
					t.Fatal(err)
				}
				query, err := url.ParseQuery(u.RawQuery)
				if err != nil {
					t.Fatal(err)
				}
				// 官方直接拼接 query；Go 编码逗号等字符，但服务收到的全部键/值必须相同。
				u.RawQuery = query.Encode()
				n.Attr[i].Val = u.String()
			}
		}
		for child := n.FirstChild; child != nil; {
			next := child.NextSibling
			if tag == "chat" && child.Type == nethtml.ElementNode && child.Data == "style" {
				var css strings.Builder
				for text := child.FirstChild; text != nil; text = text.NextSibling {
					css.WriteString(text.Data)
				}
				// 没有带颜色 label 时，上游仍输出三个空规则；Go 省略无声明的 style。
				if strings.Join(strings.Fields(css.String()), "") == ":root{}@media(prefers-color-scheme:dark){:root:not([color-scheme]){}}[color-scheme='dark']{}" {
					n.RemoveChild(child)
					child = next
					continue
				}
			}
			visit(child)
			child = next
		}
	}
	visit(doc)
	var output bytes.Buffer
	if err := nethtml.Render(&output, doc); err != nil {
		t.Fatal(err)
	}
	return v2Canonical(t, output.String())
}

func TestStellarV2OfficialDocumentedSafetyDifferences(t *testing.T) {
	t.Run("image_download_quoted_alt", func(t *testing.T) {
		// 官方 caption/alt 已转义，但 download 使用原始 alt，双引号破坏其 HTML 属性。
		// 保留原始差分输入，明确验证 Go 安全偏差，不能称作严格 DOM 相同。
		args := `/image.png 'Alt <tag> & "quoted"' fancybox:true height:200px download:/download.png`
		oracle := newV2Oracle(t, StellarRenderContext{})
		entry, _ := shortcodeV2Entry("image")
		tokens, err := v2Args(args)
		if err != nil {
			t.Fatal(err)
		}
		got := oracle.r.render("image", v2MapArgs(tokens, entry.Params), "", args)
		want := oracle.render(t, "image", args, "")
		if v2Canonical(t, got) == v2Canonical(t, want) {
			t.Fatal("documented upstream unescaped download difference disappeared; review the exception")
		}
		for _, key := range []string{"src", "data-src", "alt"} {
			if a, b := v2OracleAttr(t, got, "img", key), v2OracleAttr(t, want, "img", key); a != b {
				t.Errorf("image.%s got %q want %q", key, a, b)
			}
		}
		if v2OracleAttr(t, got, "a", "href") != v2OracleAttr(t, want, "a", "href") {
			t.Fatal("download URL differs")
		}
		if actual := v2OracleAttr(t, got, "a", "download"); actual != `Alt <tag> & "quoted"` {
			t.Fatalf("unsafe download alt: %q", actual)
		}
		if official := v2OracleAttr(t, want, "a", "download"); official != "Alt <tag> & " {
			t.Fatalf("upstream exception changed: %q", official)
		}
		t.Log("明确安全偏差：官方 download 双引号未转义，Go 保留完整安全属性；caption/alt 与下载 URL 等效")
	})
	for _, status := range []string{"", "in_track", "at_risk", "off_track", "finished", "unfinished"} {
		t.Run("okr_single_kr_"+status, func(t *testing.T) {
			// 上游先要求整体 >=3 段，shift Objective 后却再次要求 >=3，故合法单 KR 被丢弃。
			// 加 trailing newline 不改变 filter(trim.length>0)，不会修复上游该分支。
			args, body := "o1", "Objective\n<!-- okr kr1 percent:100 -->\nOne"
			if status != "" {
				args += " percent:100 status:" + status
				body = "Objective\n<!-- okr kr1 percent:0 status:" + status + " -->\nOne"
			}
			for _, suffix := range []string{"", "\n", "\n\n"} {
				oracle := newV2Oracle(t, StellarRenderContext{})
				entry, _ := shortcodeV2Entry("okr")
				tokens, err := v2Args(args)
				if err != nil {
					t.Fatal(err)
				}
				got := oracle.r.render("okr", v2MapArgs(tokens, entry.Params), body+suffix, args)
				want := oracle.render(t, "okr", args, body+suffix)
				if !strings.Contains(got, `id="okr-kr1"`) || strings.Contains(want, `id="okr-kr1"`) {
					t.Fatalf("documented single KR behavior changed\ngot %s\nwant %s", got, want)
				}
				if status == "" && (!strings.Contains(want, "NaN%") || strings.Contains(got, "NaN%")) {
					t.Fatal("documented upstream empty average changed")
				}
			}
			t.Log("明确上游缺陷偏差：Go 渲染单 KR；官方丢弃单 KR，自动平均值为 NaN。正常双 KR 所有状态仍严格差分")
		})
	}
}

func v2OfficialGistCases() []v2OracleCase {
	return []v2OracleCase{
		{"gist", "octocat/abc123", ""},
		{"gist", `octocat/abc123 file:"a b.go"`, ""},
		{"gist", `octocat/abc123 file:"目录/a&b.go"`, ""},
	}
}

func v2OracleAttr(t *testing.T, source, element, key string) string {
	t.Helper()
	doc, err := nethtml.Parse(strings.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	var result string
	var visit func(*nethtml.Node)
	visit = func(n *nethtml.Node) {
		if n.Type == nethtml.ElementNode && n.Data == element {
			for _, attr := range n.Attr {
				if attr.Key == key {
					result = attr.Val
					return
				}
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(doc)
	if result == "" {
		t.Fatalf("%s.%s absent in %s", element, key, source)
	}
	return result
}

func TestStellarV2OfficialGistRequestOracle(t *testing.T) {
	// 已确认的隔离 iframe 改变 DOM；owner/id 与 file 必须仍对应官方动态脚本请求。
	for i, c := range v2OfficialGistCases() {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			oracle := newV2Oracle(t, StellarRenderContext{})
			entry, _ := shortcodeV2Entry(c.name)
			tokens, err := v2Args(c.args)
			if err != nil {
				t.Fatal(err)
			}
			got := oracle.r.render(c.name, v2MapArgs(tokens, entry.Params), "", c.args)
			iframeURL, err := url.Parse(v2OracleAttr(t, got, "iframe", "src"))
			if err != nil {
				t.Fatal(err)
			}
			query := iframeURL.Query()
			base := strings.TrimRight(v2String(v2Object(oracle.r.services["github"])["gistUrl"]), "/")
			reconstructed := base + "/" + query.Get("gist") + ".js"
			if file := query.Get("file"); file != "" {
				reconstructed += "?" + url.Values{"file": {file}}.Encode()
			}
			want := v2OracleAttr(t, oracle.render(t, c.name, c.args, ""), "script", "src")
			actualURL, err := url.Parse(reconstructed)
			if err != nil {
				t.Fatal(err)
			}
			officialURL, err := url.Parse(want)
			if err != nil {
				t.Fatal(err)
			}
			if actualURL.Scheme != officialURL.Scheme || actualURL.Host != officialURL.Host || actualURL.Path != officialURL.Path || actualURL.Query().Encode() != officialURL.Query().Encode() {
				t.Errorf("isolated request %s differs from official %s", reconstructed, want)
			}
			if iframeURL.Path != "/stellar/embed/gist" || v2OracleAttr(t, got, "iframe", "sandbox") != "allow-scripts" {
				t.Fatalf("Gist isolation changed: %s", got)
			}
		})
	}
}

func TestStellarV2OfficialOracleParameterCoverage(t *testing.T) {
	// 这是测试矩阵的门禁，不是渲染断言；每个字段的结果仍由上述真实 JS oracle 对照。
	covered := map[string]map[string]bool{}
	for _, c := range append(v2OfficialCases(), v2OfficialGistCases()...) {
		entry, ok := shortcodeV2Entry(c.name)
		if !ok {
			t.Fatalf("uncatalogued oracle tag %s", c.name)
		}
		tokens, err := v2Args(c.args)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		values := v2MapArgs(tokens, entry.Params)
		if covered[c.name] == nil {
			covered[c.name] = map[string]bool{}
		}
		for _, param := range entry.Params {
			if param.Role == ShortcodeParamContent {
				covered[c.name][param.Key] = covered[c.name][param.Key] || c.body != ""
			} else if param.Role == ShortcodeParamNamed || param.Role == ShortcodeParamFlag {
				for _, token := range tokens {
					if strings.HasPrefix(token, param.Key+":") {
						covered[c.name][param.Key] = true
					}
				}
			} else {
				covered[c.name][param.Key] = covered[c.name][param.Key] || values[param.Key] != ""
			}
		}
	}
	for _, entry := range ShortcodeCatalogStellarV2() {
		for _, param := range entry.Params {
			if !covered[entry.Name][param.Key] {
				t.Errorf("official oracle has no explicit %s.%s (%s) parameter case", entry.Name, param.Key, param.Role)
			}
		}
	}
}

func TestStellarV2OfficialThemeDefaultAndLocaleOracle(t *testing.T) {
	cases := []v2OracleCase{
		{"note", "Title body", ""}, {"box", "Title", "body"},
		{"button", "Label /target", ""}, {"mark", "Text", ""},
		{"icon", "copy:copy", ""}, {"hashtag", "Label /tag", ""},
		{"gallery", "", "![Caption](/image.png)"}, {"quot", "Heading", ""},
		{"copy", "Text", ""}, {"rating", "Title", ""}, {"vote", "Title", ""},
		{"mbti", "INTJ-T 93-72-78-83-51", ""},
		{"okr", "o1", "Objective\n<!-- okr kr1 percent:25 -->\nOne\n<!-- okr kr2 percent:100 -->\nTwo"},
	}
	theme := map[string]any{"tags": map[string]any{
		"note": map[string]any{"defaultColor": "purple"}, "button": map[string]any{"defaultColor": "cyan"},
		"mark": map[string]any{"defaultColor": "orange"}, "icon": map[string]any{"defaultColor": "green"},
		"hashtag": map[string]any{"defaultColor": "blue"},
		"gallery": map[string]any{"size": "xl", "aspectRatio": "landscape"},
	}}
	for _, language := range []string{"zh-CN", "zh-TW", "en"} {
		for _, c := range cases {
			t.Run(language+"/"+c.name, func(t *testing.T) {
				oracle := newV2Oracle(t, StellarRenderContext{Language: language, Theme: theme})
				entry, _ := shortcodeV2Entry(c.name)
				tokens, err := v2Args(c.args)
				if err != nil {
					t.Fatal(err)
				}
				got := oracle.r.render(c.name, v2MapArgs(tokens, entry.Params), c.body, c.args)
				want := oracle.render(t, c.name, c.args, c.body)
				if a, b := v2Canonical(t, got), v2Canonical(t, want); a != b {
					t.Errorf("got %s\nwant %s", a, b)
				}
			})
		}
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
	if len(fingerprints) != 54 {
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
