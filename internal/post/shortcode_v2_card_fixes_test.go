package post

import (
	"strings"
	"testing"
)

// 本轮卡片渲染修复（BLG-20261109-02）的行为回归：
//  1. link 的 desc:true/false 按官方文档当描述区开关，不把字面量渲染成描述；
//  2. banner/quot/tip/frame 兼容旧版命名参数写法（title:/subtitle:/text:/device:）。

func TestStellarV2LinkDescBooleanSwitch(t *testing.T) {
	s := NewService("")
	render := func(body string) string {
		out, err := s.RenderStellarV2(body, nil)
		if err != nil {
			t.Fatalf("render %q: %v", body, err)
		}
		return string(out)
	}

	rich := render("{% link https://example.com/page 示例站点 desc:true %}")
	if !strings.Contains(rich, "link-card rich") {
		t.Fatalf("desc:true should use rich layout: %s", rich)
	}
	if strings.Contains(rich, ">true<") {
		t.Fatalf("desc:true leaked literal text: %s", rich)
	}
	if strings.Contains(rich, `class="cap desc footnote"`) {
		t.Fatalf("desc:true without siteinfo should omit empty desc line: %s", rich)
	}

	plain := render("{% link https://example.com/page 示例站点 desc:false %}")
	if !strings.Contains(plain, "link-card plain") {
		t.Fatalf("desc:false should use plain layout: %s", plain)
	}
	if strings.Contains(plain, ">false<") {
		t.Fatalf("desc:false leaked literal text: %s", plain)
	}

	withText := render("{% link https://example.com/page 示例站点 desc:一段描述 %}")
	if !strings.Contains(withText, "link-card rich") || !strings.Contains(withText, "一段描述") {
		t.Fatalf("desc text should stay rich with description: %s", withText)
	}
}

func TestStellarV2LinkDescBooleanSwitchLegacyRenderer(t *testing.T) {
	s := NewService("")
	out, err := s.Render("{% link https://example.com/page 示例站点 desc:true %}")
	if err != nil {
		t.Fatalf("legacy render: %v", err)
	}
	if !strings.Contains(out, "link-card rich") {
		t.Fatalf("legacy desc:true should use rich layout: %s", out)
	}
	if strings.Contains(out, ">true<") || strings.Contains(out, `class="cap desc footnote"`) {
		t.Fatalf("legacy desc:true should not render literal description: %s", out)
	}
	out, err = s.Render("{% link https://example.com/page 示例站点 desc:false %}")
	if err != nil {
		t.Fatalf("legacy render: %v", err)
	}
	if !strings.Contains(out, "link-card plain") || strings.Contains(out, ">false<") {
		t.Fatalf("legacy desc:false should use plain layout: %s", out)
	}
}

func TestStellarV2LegacyNamedParamCompatibility(t *testing.T) {
	s := NewService("")
	render := func(body string) string {
		out, err := s.RenderStellarV2(body, nil)
		if err != nil {
			t.Fatalf("render %q: %v", body, err)
		}
		return string(out)
	}
	for _, tc := range []struct{ name, legacy, official string }{
		{
			"banner",
			"{% banner title:我的博客 subtitle:记录技术成长 bg:/banner.jpg %}正文{% endbanner %}",
			"{% banner 我的博客 记录技术成长 bg:/banner.jpg %}正文{% endbanner %}",
		},
		{"quot", "{% quot text:你好世界 %}", "{% quot 你好世界 %}"},
		{"tip", "{% tip text:这是说明 %}", "{% tip 这是说明 %}"},
		{"frame", "{% frame device:iphone img:/frame.jpg %}", "{% frame iphone img:/frame.jpg %}"},
	} {
		legacy := render(tc.legacy)
		official := render(tc.official)
		if legacy != official {
			t.Errorf("%s legacy named params should render like official positional form\nlegacy:   %s\nofficial: %s", tc.name, legacy, official)
		}
		if strings.Contains(legacy, "title:") || strings.Contains(legacy, "subtitle:") ||
			strings.Contains(legacy, "text:") || strings.Contains(legacy, "device:") {
			t.Errorf("%s leaked legacy parameter names: %s", tc.name, legacy)
		}
	}
}

func TestStellarV2LegacyNamedParamsEditable(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		want      map[string]string
	}{
		{
			"banner",
			"{% banner title:我的博客 subtitle:记录技术成长 bg:/banner.jpg %}正文{% endbanner %}",
			map[string]string{"title": "我的博客", "subtitle": "记录技术成长", "bg": "/banner.jpg"},
		},
		{"quot", "{% quot text:你好世界 %}", map[string]string{"text": "你好世界"}},
		{"tip", "{% tip text:这是说明 %}", map[string]string{"text": "这是说明"}},
		{"frame", "{% frame device:iphone img:/frame.jpg %}", map[string]string{"device": "iphone", "img": "/frame.jpg"}},
	} {
		result, err := ParseShortcodeForEditStellarV2(tc.raw)
		if err != nil {
			t.Fatalf("%s parse: %v", tc.name, err)
		}
		if !result.Editable {
			t.Fatalf("%s legacy named form should stay editable: %+v", tc.name, result)
		}
		for key, want := range tc.want {
			if got := result.Values[key]; got != want {
				t.Errorf("%s.%s = %v, want %q", tc.name, key, got, want)
			}
		}
	}
}

func TestV2MapArgsLegacyNamedAliasKeepsOfficialPositions(t *testing.T) {
	entry, ok := shortcodeV2Entry("banner")
	if !ok {
		t.Fatal("banner entry missing")
	}
	named := v2MapArgs([]string{"title:我的博客", "subtitle:记录技术成长", "bg:/banner.jpg"}, entry.Params, v2LegacyNamedParams("banner")...)
	if named["title"] != "我的博客" || named["subtitle"] != "记录技术成长" || named["bg"] != "/banner.jpg" {
		t.Fatalf("legacy named args mismatched: %+v", named)
	}
	// 官方位置语法不受别名影响。
	positional := v2MapArgs([]string{"我的博客", "记录技术成长", "bg:/banner.jpg"}, entry.Params, v2LegacyNamedParams("banner")...)
	if positional["title"] != "我的博客" || positional["subtitle"] != "记录技术成长" {
		t.Fatalf("positional args mismatched: %+v", positional)
	}
	// 不带兼容键时，未知命名 token 仍按官方规则落入位置文本。
	plain := v2MapArgs([]string{"title:我的博客"}, entry.Params)
	if plain["title"] != "title:我的博客" {
		t.Fatalf("official parsing should keep unknown named token as positional text: %+v", plain)
	}
}
