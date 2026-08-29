package post

import (
	"strings"
	"testing"
)

// shortcodeEditFixtures 每个 catalog 标签一条最小合法 raw。
// TestShortcodeEditFixturesCoverCatalog 机械校验「catalog 全覆盖」：
// 以后新增第 59 个标签而忘记编辑器支持时，测试直接失败。
var shortcodeEditFixtures = map[string]string{
	"note":       `{% note color:green 提示 这是一条提示 %}`,
	"quot":       `{% quot el:h2 text:这是一个小标题 %}`,
	"mark":       `{% mark 重点内容 color:cyan %}`,
	"button":     `{% button 访问主页 https://example.com color:purple %}`,
	"checkbox":   `{% checkbox checked 已完成的任务 color:green %}`,
	"radio":      `{% radio checked 选项一 color:blue %}`,
	"hashtag":    `{% hashtag 日常 %}`,
	"emoji":      `{% emoji twemoji 1f44d %}`,
	"icon":       `{% icon example:planet color:blue %}`,
	"folding":    "{% folding color:cyan open:true 点击查看详情 %}\n折叠正文\n{% endfolding %}",
	"tabs":       "{% tabs active:1 %}\n<!-- tab 标签一 -->\n第一页内容。\n{% endtabs %}",
	"timeline":   "{% timeline %}\n<!-- node 2024-01 -->\n项目启动。\n{% endtimeline %}",
	"gallery":    "{% gallery %}\n![首页界面](/assets/img/preview/gallery-1.jpg)\n{% endgallery %}",
	"folders":    "{% folders color:blue %}\n<!-- folder 第一章 -->\n内容一。\n{% endfolders %}",
	"grid":       "{% grid bg:card c:2 gap:12px %}\n<!-- cell -->\n### 博客技术\n{% endgrid %}",
	"table":      "{% table style:scroll %}\n| a | b |\n| - | - |\n| 1 | 2 |\n{% endtable %}",
	"tip":        "{% tip text:提示文字 %}\n被提示的内容\n{% endtip %}",
	"blockquote": "{% blockquote indent:true %}\n引用正文\n{% endblockquote %}",
	"banner":     "{% banner title:我的博客 subtitle:记录技术成长 %}\n欢迎来到我的小站。\n{% endbanner %}",
	"navbar":     "{% navbar active:首页 %}\n[首页](/)\n[关于](/about)\n{% endnavbar %}",
	"frame":      "{% frame device:iphone width:320px %}\n![截图](/assets/img/preview/frame-shot.jpg)\n{% endframe %}",
	"box":        "{% box color:green 卡片标题 %}\n卡片正文\n{% endbox %}",
	"about":      "{% about avatar:/assets/img/preview/avatar-blog.svg back:true %}\n关于页正文\n{% endabout %}",
	"toc":        `{% toc 本文目录 %}`,
	"mbti":       `{% mbti INTJ-T 93-72-78-83-51 %}`,
	"poetry":     "{% poetry 静夜思 author:李白 %}\n床前明月光，\n{% endpoetry %}",
	"paper":      "{% paper title:一封家书 author:张三 %}\n<!-- paragraph -->\n见字如面。\n{% endpaper %}",
	"reel":       "{% reel 观影手记 author:阿甘 date:2026-07 %}\n第 1 帧：开场长镜头。\n{% endreel %}",
	"psw":        `{% psw 密码内容 %}`,
	"emp":        `{% emp 重点强调 %}`,
	"wavy":       `{% wavy 波浪线 %}`,
	"blur":       `{% blur 结局大反转 %}`,
	"sub":        `{% sub 2 %}`,
	"sup":        `{% sup color:blue 2 %}`,
	"u":          `{% u 下划线 %}`,
	"del":        `{% del 999 元 %}`,
	"kbd":        `{% kbd Ctrl %}`,
	"image":      `{% image /assets/img/preview/image-demo.jpg 示例图 width:80% download:true %}`,
	"link":       `{% link https://example.com 示例站点 desc:一个示例链接卡片 %}`,
	"video":      `{% video /assets/media/preview/sample-video.mp4 %}`,
	"audio":      `{% audio /assets/media/preview/sample-audio.mp3 %}`,
	"swiper":     "{% swiper %}\n![](/assets/img/preview/swiper-1.jpg)\n{% endswiper %}",
	"albums":     `{% albums api:https://example.com/data.json size:m %}`,
	"posters":    `{% posters api:https://example.com/posters.json size:s %}`,
	"copy":       `{% copy npm install stellar-theme %}`,
	"rating":     `{% rating demo-post 给这篇文章打分 %}`,
	"vote":       `{% vote 这篇文章有帮助吗 id:demo-post %}`,
	"friends":    `{% friends %}`,
	"links":      `{% links %}`,
	"moments":    `{% moments limit:5 %}`,
	"sites":      `{% sites api:https://example.com/sites.json %}`,
	"md":         `{% md /assets/md/preview/example.md %}`,
	"mermaid":    "{% mermaid %}\ngraph TB\n A[开始] --> B{判断}\n{% endmermaid %}",
	"okr":        "{% okr o1 percent:60 status:in_track %}\n提升博客阅读体验\n<!-- okr kr1 percent:80 -->\n完成标签插件三期\n{% endokr %}",
	"chat":       "{% chat iphone style:qq title:技术交流群 me:b %}\nalice:\n  name: 小明\n<!-- chattip 上午 10:00 -->\n<!-- chatcell user:alice md:大家好！ -->\n{% endchat %}",
	"ghcard":     `{% ghcard anuraghazra/github-readme-stats %}`,
	"ghrepo":     `{% ghrepo vuejs/vue %}`,
	"ghuser":     `{% ghuser octocat %}`,
}

func TestShortcodeEditFixturesCoverCatalog(t *testing.T) {
	catalog := ShortcodeCatalog()
	enabled := 0
	for _, item := range catalog {
		if item.Edit.Enabled {
			enabled++
		}
	}
	if enabled != len(shortcodeEditFixtures) {
		missing := make([]string, 0, 8)
		for _, item := range catalog {
			if !item.Edit.Enabled {
				continue
			}
			if _, ok := shortcodeEditFixtures[item.Name]; !ok {
				missing = append(missing, item.Name)
			}
		}
		extra := make([]string, 0, 8)
		for name := range shortcodeEditFixtures {
			found := false
			for _, item := range catalog {
				if !item.Edit.Enabled {
					continue
				}
				if item.Name == name {
					found = true
					break
				}
			}
			if !found {
				extra = append(extra, name)
			}
		}
		t.Fatalf("catalog %d 条 / fixture %d 条，缺失 %v，多余 %v", len(catalog), len(shortcodeEditFixtures), missing, extra)
	}

	for _, item := range catalog {
		if !item.Edit.Enabled {
			continue
		}
		raw, ok := shortcodeEditFixtures[item.Name]
		if !ok {
			t.Fatalf("catalog 标签 %s 缺少编辑 fixture", item.Name)
		}
		got, err := ParseShortcodeForEdit(raw)
		if err != nil {
			t.Errorf("%s 解析失败: %v", item.Name, err)
			continue
		}
		if got.Name != item.Name {
			t.Errorf("%s 解析出标签名 %s", item.Name, got.Name)
		}
		if !got.Editable {
			t.Errorf("%s 应可可视化编辑", item.Name)
		}
	}
}

func TestShortcodeEditSchemaOwnsRequiredValues(t *testing.T) {
	tests := []struct {
		name   string
		values map[string]any
		wantOK bool
	}{
		{name: "button missing url", values: map[string]any{"text": "按钮"}},
		{name: "video youtube alternative", values: map[string]any{"youtube": "abc123"}, wantOK: true},
		{name: "video missing media", values: map[string]any{}},
		{name: "audio netease alternative", values: map[string]any{"netease": "123"}, wantOK: true},
		{name: "mark missing text", values: map[string]any{}},
	}
	entries := map[string]ShortcodeCatalogEntry{}
	for _, item := range ShortcodeCatalog() {
		entries[item.Name] = item
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var entry ShortcodeCatalogEntry
			switch tt.name {
			case "button missing url":
				entry = entries["button"]
			case "video youtube alternative", "video missing media":
				entry = entries["video"]
			case "audio netease alternative":
				entry = entries["audio"]
			case "mark missing text":
				entry = entries["mark"]
			}
			err := validateShortcodeEditValues(entry, tt.values)
			if (err == nil) != tt.wantOK {
				t.Fatalf("validate %s = %v, wantOK=%v", entry.Name, err, tt.wantOK)
			}
		})
	}
}

// 编辑已有 shortcode 时，未被写出的参数必须是空值/ false，
// 绝不能回落到 catalog default，否则保存会静默写入作者没写过的参数。
func TestParseShortcodeForEditReturnsEverySchemaField(t *testing.T) {
	for _, item := range ShortcodeCatalog() {
		if !item.Edit.Enabled {
			continue
		}
		raw := shortcodeEditFixtures[item.Name]
		got, err := ParseShortcodeForEdit(raw)
		if err != nil {
			t.Fatalf("%s 解析失败: %v", item.Name, err)
		}
		if len(got.Values) != len(item.Params) && item.Name != "frame" {
			t.Errorf("%s values 数量 %d 与 schema %d 不一致", item.Name, len(got.Values), len(item.Params))
		}
		for _, param := range item.Params {
			value, ok := got.Values[param.Key]
			if !ok {
				t.Errorf("%s 缺少 schema 字段 %s", item.Name, param.Key)
				continue
			}
			if param.Role == ShortcodeParamFlag {
				if _, isBool := value.(bool); !isBool {
					t.Errorf("%s 字段 %s 应为布尔，实际 %#v", item.Name, param.Key, value)
				}
				continue
			}
			if _, isString := value.(string); !isString {
				t.Errorf("%s 字段 %s 应为字符串，实际 %#v", item.Name, param.Key, value)
			}
		}
	}
}

func TestParseShortcodeForEditNote(t *testing.T) {
	got, err := ParseShortcodeForEdit(`{% note color:green 提示 这里是正文 %}`)
	if err != nil {
		t.Fatalf("ParseShortcodeForEdit: %v", err)
	}
	want := ShortcodeEditResult{
		Name:     "note",
		Editable: true,
		Values: map[string]any{
			"color":   "green",
			"title":   "提示",
			"content": "这里是正文",
		},
	}
	if got.Name != want.Name || !got.Editable {
		t.Fatalf("result = %#v", got)
	}
	assertValues(t, got.Values, want.Values)
}

func TestParseShortcodeForEditSimpleTags(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want map[string]any
	}{
		{
			name: "checkbox flag",
			raw:  `{% checkbox checked 已完成的任务 color:green %}`,
			want: map[string]any{"text": "已完成的任务", "checked": true, "color": "green", "symbol": ""},
		},
		{
			name: "folding block",
			raw:  "{% folding color:cyan open:true 点击查看详情 %}\n折叠正文\n{% endfolding %}",
			want: map[string]any{"color": "cyan", "open": true, "title": "点击查看详情", "content": "折叠正文"},
		},
		{
			name: "box block",
			raw:  "{% box color:green 卡片标题 %}\n卡片正文\n{% endbox %}",
			want: map[string]any{"color": "green", "child": "", "title": "卡片标题", "content": "卡片正文"},
		},
		{
			name: "media",
			raw:  `{% image /assets/example.jpg 示例图 width:80% download:true %}`,
			want: map[string]any{
				"src": "/assets/example.jpg", "alt": "示例图", "width": "80%", "download": true,
				"height": "", "bg": "", "padding": "", "fancybox": "", "ratio": "",
			},
		},
		{
			name: "copy prefix",
			raw:  `{% copy npm install stellar-theme prefix:安装 %}`,
			want: map[string]any{"text": "npm install stellar-theme", "git": "", "prefix": "安装"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseShortcodeForEdit(tt.raw)
			if err != nil {
				t.Fatalf("ParseShortcodeForEdit: %v", err)
			}
			if !got.Editable {
				t.Fatal("result should be editable")
			}
			assertValues(t, got.Values, tt.want)
		})
	}
}

func TestParseShortcodeForEditSemantics(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want map[string]any
	}{
		{
			name: "emoji 普通模式",
			raw:  `{% emoji twemoji 1f44d height:20px %}`,
			want: map[string]any{"source": "twemoji", "name": "1f44d", "url": "", "height": "20px"},
		},
		{
			name: "emoji 省略 source",
			raw:  `{% emoji aini %}`,
			want: map[string]any{"source": "", "name": "aini", "url": "", "height": ""},
		},
		{
			name: "emoji URL 模式",
			raw:  `{% emoji url:https://example.com/a.png name:表情 height:20px %}`,
			want: map[string]any{"source": "", "name": "表情", "url": "https://example.com/a.png", "height": "20px"},
		},
		{
			name: "tabs 保留 marker 正文",
			raw:  "{% tabs active:2 align:center %}\n<!-- tab A -->\n内容 A\n<!-- tab B -->\n内容 B\n{% endtabs %}",
			want: map[string]any{"active": "2", "align": "center", "content": "<!-- tab A -->\n内容 A\n<!-- tab B -->\n内容 B"},
		},
		{
			name: "grid bg + content",
			raw:  "{% grid bg:card c:2 gap:12px %}\n<!-- cell -->\n内容一\n{% endgrid %}",
			// w 未写出，必须是空值而不是 schema default 240px
			want: map[string]any{"bg": "card", "w": "", "c": "2", "gap": "12px", "br": "", "content": "<!-- cell -->\n内容一"},
		},
		{
			name: "banner 旧版位置参数",
			raw:  "{% banner 我的博客 记录技术成长 %}\n正文\n{% endbanner %}",
			want: map[string]any{
				"title": "我的博客", "subtitle": "记录技术成长", "bg": "", "avatar": "", "link": "", "content": "正文",
			},
		},
		{
			name: "toc 全参数",
			raw:  `{% toc 本文目录 open:false display:mobile %}`,
			want: map[string]any{"title": "本文目录", "wiki": "", "open": "false", "display": "mobile"},
		},
		{
			name: "md wrap",
			raw:  `{% md /assets/example.md wrap:false %}`,
			want: map[string]any{"src": "/assets/example.md", "wrap": "false"},
		},
		{
			name: "ghcard 全参数",
			raw:  `{% ghcard vuejs/vue theme:dark show_owner:true title_color:fff text_color:eee icon_color:ddd bg_color:000 hide_border:true cache_seconds:3600 locale:zh %}`,
			want: map[string]any{
				"repo": "vuejs/vue", "theme": "dark", "show_owner": true, "title_color": "fff",
				"text_color": "eee", "icon_color": "ddd", "bg_color": "000", "hide_border": true,
				"cache_seconds": "3600", "locale": "zh",
			},
		},
		{
			name: "okr marker 正文",
			raw:  "{% okr o1 percent:60 status:in_track %}\n目标标题\n<!-- okr kr1 percent:80 -->\nKR1\n{% endokr %}",
			want: map[string]any{
				"title": "o1", "percent": "60", "status": "in_track", "color": "",
				"content": "目标标题\n<!-- okr kr1 percent:80 -->\nKR1",
			},
		},
		{
			name: "chat 正文保留 YAML 与 marker",
			raw:  "{% chat iphone style:wechat title:群聊 scene:single me:bob %}\nalice:\n  name: 小明\n<!-- chattip 上午 10:00 -->\n<!-- chatcell user:alice md:你好 -->\n{% endchat %}",
			want: map[string]any{
				"device": "iphone", "style": "wechat", "title": "群聊", "scene": "single", "me": "bob",
				"content": "alice:\n  name: 小明\n<!-- chattip 上午 10:00 -->\n<!-- chatcell user:alice md:你好 -->",
			},
		},
		{
			name: "links 零参数",
			raw:  `{% links %}`,
			want: map[string]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseShortcodeForEdit(tt.raw)
			if err != nil {
				t.Fatalf("ParseShortcodeForEdit: %v", err)
			}
			if !got.Editable {
				t.Fatal("result should be editable")
			}
			assertValues(t, got.Values, tt.want)
		})
	}
}

func TestParseShortcodeForEditHybridVariants(t *testing.T) {
	t.Run("navbar block", func(t *testing.T) {
		got, err := ParseShortcodeForEdit("{% navbar active:首页 %}\n[首页](/)\n[关于](/about)\n{% endnavbar %}")
		if err != nil {
			t.Fatalf("ParseShortcodeForEdit: %v", err)
		}
		if got.Variant != string(ShortcodeVariantBlock) {
			t.Fatalf("variant = %q", got.Variant)
		}
		assertValues(t, got.Values, map[string]any{"active": "首页", "content": "[首页](/)\n[关于](/about)"})
	})

	t.Run("navbar inline", func(t *testing.T) {
		got, err := ParseShortcodeForEdit(`{% navbar active:/ [首页](/) [关于](/about) %}`)
		if err != nil {
			t.Fatalf("ParseShortcodeForEdit: %v", err)
		}
		if got.Variant != string(ShortcodeVariantInline) {
			t.Fatalf("variant = %q", got.Variant)
		}
		assertValues(t, got.Values, map[string]any{"active": "/", "content": "[首页](/)\n[关于](/about)"})
	})

	t.Run("frame block", func(t *testing.T) {
		got, err := ParseShortcodeForEdit("{% frame device:ipad width:480px %}\n![截图](/a.jpg)\n{% endframe %}")
		if err != nil {
			t.Fatalf("ParseShortcodeForEdit: %v", err)
		}
		if got.Variant != string(ShortcodeVariantBlock) {
			t.Fatalf("variant = %q", got.Variant)
		}
		assertValues(t, got.Values, map[string]any{"device": "ipad", "width": "480px", "content": "![截图](/a.jpg)"})
	})

	t.Run("frame inline", func(t *testing.T) {
		got, err := ParseShortcodeForEdit(`{% frame iphone 手机截图 img:/a.jpg focus:top %}`)
		if err != nil {
			t.Fatalf("ParseShortcodeForEdit: %v", err)
		}
		if got.Variant != string(ShortcodeVariantInline) {
			t.Fatalf("variant = %q", got.Variant)
		}
		assertValues(t, got.Values, map[string]any{
			"device": "iphone", "img": "/a.jpg", "video": "", "focus": "top", "alt": "手机截图",
		})
	})

	t.Run("普通标签不输出 variant", func(t *testing.T) {
		got, err := ParseShortcodeForEdit(`{% note 内容 %}`)
		if err != nil {
			t.Fatalf("ParseShortcodeForEdit: %v", err)
		}
		if got.Variant != "" {
			t.Fatalf("普通标签不应输出 variant，实际 %q", got.Variant)
		}
	})
}

func TestParseShortcodeForEditFallbacks(t *testing.T) {
	t.Run("未登记标签", func(t *testing.T) {
		got, err := ParseShortcodeForEdit(`{% unknown 内容 %}`)
		if err != nil {
			t.Fatalf("未登记标签不应报错: %v", err)
		}
		if got.Name != "unknown" || got.Editable {
			t.Fatalf("unsupported result = %#v", got)
		}
	})

	t.Run("未知命名参数交回源码编辑", func(t *testing.T) {
		if _, err := ParseShortcodeForEdit(`{% image /a.jpg foo:bar %}`); err == nil {
			t.Fatal("未知命名参数应报错而不是被当成位置参数吞掉")
		}
	})

	t.Run("块级缺少结束标签", func(t *testing.T) {
		if _, err := ParseShortcodeForEdit("{% folding title %}正文"); err == nil {
			t.Fatal("malformed block should fail")
		}
	})

	t.Run("缺少必要参数", func(t *testing.T) {
		if _, err := ParseShortcodeForEdit(`{% image %}`); err == nil {
			t.Fatal("missing required image source should fail")
		}
	})

	t.Run("无法映射的多余位置参数", func(t *testing.T) {
		if _, err := ParseShortcodeForEdit("{% tabs active:1 多余 %}\n<!-- tab A -->内容\n{% endtabs %}"); err == nil {
			t.Fatal("无法映射的位置参数应报错")
		}
	})

	t.Run("无效格式", func(t *testing.T) {
		if _, err := ParseShortcodeForEdit("不是 shortcode"); err == nil {
			t.Fatal("非 shortcode 文本应报错")
		}
	})
}

// TestParseShortcodeForEditRoundTrip 用与前端 buildShortcode 一致的规则重建 raw，
// 再解析一次，确保「打开属性面板 → 不改任何值 → 应用」不会改变 shortcode 语义。
func TestParseShortcodeForEditRoundTrip(t *testing.T) {
	for _, item := range ShortcodeCatalog() {
		if !item.Edit.Enabled {
			continue
		}
		raw := shortcodeEditFixtures[item.Name]
		first, err := ParseShortcodeForEdit(raw)
		if err != nil {
			t.Fatalf("%s 解析失败: %v", item.Name, err)
		}
		rebuilt := rebuildShortcodeForTest(item.Name, item.Type, first.Variant, item.Params, first.Values)
		second, err := ParseShortcodeForEdit(rebuilt)
		if err != nil {
			t.Errorf("%s 重建后解析失败: %v\n重建结果: %s", item.Name, err, rebuilt)
			continue
		}
		if second.Variant != first.Variant {
			t.Errorf("%s variant 漂移: %q → %q", item.Name, first.Variant, second.Variant)
		}
		assertValues(t, second.Values, first.Values)
	}
}

// resetShortcodeCounters 复位全局自增序号（tab_N / copy_N / md_N / gallery-N）与
// hashtag 循环色板指针，让同一 raw 的两次渲染结果可比对。
func resetShortcodeCounters() {
	tabIndexCounter = 0
	galleryIndex = 0
	hashtagColorIdx = 0
	copyIndex = 0
	mdRenderIndex = 0
}

func renderForEditTest(s *Service, raw string) (string, error) {
	resetShortcodeCounters()
	return s.RenderComponentPreview(raw)
}

// TestShortcodeEditRoundTripKeepsRender 证明「打开属性面板 → 不改值 → 应用」不破坏渲染：
// 用重建后的 raw 渲染，输出必须与原始 raw 一致。
func TestShortcodeEditRoundTripKeepsRender(t *testing.T) {
	s := NewService(t.TempDir())
	for _, item := range ShortcodeCatalog() {
		if !item.Edit.Enabled {
			continue
		}
		raw := shortcodeEditFixtures[item.Name]
		first, err := renderForEditTest(s, raw)
		if err != nil {
			t.Errorf("%s 渲染失败: %v", item.Name, err)
			continue
		}
		if strings.TrimSpace(first) == "" {
			t.Errorf("%s 渲染结果为空", item.Name)
			continue
		}
		if strings.Contains(first, "{%") {
			t.Errorf("%s 未被渲染，输出仍含 shortcode: %s", item.Name, first)
			continue
		}
		parsed, err := ParseShortcodeForEdit(raw)
		if err != nil {
			t.Fatalf("%s 解析失败: %v", item.Name, err)
		}
		rebuilt := rebuildShortcodeForTest(item.Name, item.Type, parsed.Variant, item.Params, parsed.Values)
		after, err := renderForEditTest(s, rebuilt)
		if err != nil {
			t.Errorf("%s 重建后渲染失败: %v", item.Name, err)
			continue
		}
		if after != first {
			t.Errorf("%s 重建后渲染结果漂移\n原始: %s\n重建: %s\n渲染前: %s\n渲染后: %s", item.Name, raw, rebuilt, first, after)
		}
	}
}

// rebuildShortcodeForTest 复刻 modules/console/web/src/modules/plugins/shortcode.ts 的拼装规则。
func rebuildShortcodeForTest(name string, typ ShortcodeType, variant string, params []ShortcodeParamSchema, values map[string]any) string {
	var positional, named []string
	var body string
	for _, param := range params {
		value := strings.TrimSpace(shortcodeEditString(values, param.Key))
		switch param.Role {
		case ShortcodeParamContent:
			body = shortcodeEditString(values, param.Key)
		case ShortcodeParamFlag:
			if values[param.Key] == true {
				named = append(named, param.Key+":true")
			}
		case ShortcodeParamNamed:
			if value != "" {
				named = append(named, param.Key+":"+value)
			}
		default:
			if value != "" {
				positional = append(positional, value)
			}
		}
	}
	head := strings.Join(append([]string{name}, append(positional, named...)...), " ")
	isBlock := typ == ShortcodeBlock || (typ == ShortcodeHybrid && variant != string(ShortcodeVariantInline))
	if isBlock {
		content := strings.TrimSpace(body)
		if content == "" {
			return "{%" + " " + head + " %} {% end" + name + " %}"
		}
		return "{%" + " " + head + " %}\n" + content + "\n{% end" + name + " %}"
	}
	inline := strings.TrimSpace(body)
	if typ == ShortcodeHybrid {
		// 单行 hybrid 不能出现换行，否则 splitArgs 无法切分位置参数
		inline = strings.Join(strings.Fields(inline), " ")
	}
	if inline != "" {
		head = head + " " + inline
	}
	return "{%" + " " + head + " %}"
}

func assertValues(t *testing.T, got map[string]any, want map[string]any) {
	t.Helper()
	for key, expected := range want {
		actual, ok := got[key]
		if !ok {
			t.Errorf("缺少字段 %s（实际 %#v）", key, got)
			continue
		}
		if actual != expected {
			t.Errorf("字段 %s = %#v，期望 %#v", key, actual, expected)
		}
	}
	if len(got) != len(want) {
		t.Errorf("字段数量不一致：实际 %#v，期望 %#v", got, want)
	}
}
