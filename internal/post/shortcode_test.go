package post

import (
	"strings"
	"testing"
)

// runShortcodes 便捷调用：identity renderMD + 简单 icon 桩，不依赖 goldmark/GitHub。
func runShortcodes(md string) string {
	iconFunc := func(key string) string { return `<svg data-icon="` + key + `"></svg>` }
	return processShortcodes(md, func(s string) string { return s }, iconFunc, nil)
}

func TestRadioAndCheckboxCoexist(t *testing.T) {
	out := runShortcodes("{% checkbox checked:true 选项A %}\n{% radio color:cyan 选项B %}")
	if !strings.Contains(out, `<input type="checkbox" checked="true"/><span>选项A</span>`) {
		t.Errorf("checkbox not rendered: %s", out)
	}
	if !strings.Contains(out, `<input type="radio"/><span>选项B</span>`) {
		t.Errorf("radio not rendered: %s", out)
	}
	if !strings.Contains(out, `color="cyan"`) {
		t.Errorf("radio color attr missing: %s", out)
	}
}

func TestRadioCheckedPositional(t *testing.T) {
	out := runShortcodes("{% radio checked 已选中 %}")
	if !strings.Contains(out, `<input type="radio" checked="true"/><span>已选中</span>`) {
		t.Errorf("radio checked not rendered: %s", out)
	}
}

func TestVoteRender(t *testing.T) {
	out := runShortcodes("{% vote id:post-1 这篇文章有用吗 %}")
	for _, want := range []string{
		`class="tag-plugin ds-vote"`,
		`data-api="/api/vote"`,
		`data-id="post-1"`,
		`<div class="header"><span>这篇文章有用吗</span></div>`,
		`<button class="vote-up"><svg data-icon="vote:thumbsup"></svg> <span class="up">0</span></button>`,
		`<button class="vote-down"><svg data-icon="vote:thumbsdown"></svg> <span class="down">0</span></button>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("vote output missing %q: %s", want, out)
		}
	}
}

func TestVoteDefaultID(t *testing.T) {
	out := runShortcodes("{% vote %}")
	if !strings.Contains(out, `data-id="default"`) {
		t.Errorf("vote default id missing: %s", out)
	}
}

func TestFriendsRender(t *testing.T) {
	// 缺省指向本地友链聚合端点
	out := runShortcodes("{% friends %}")
	if !strings.Contains(out, `<div class="tag-plugin users-wrap"><div class="data-service ds-friends" data-api="/api/friends"><div class="grid-box"></div></div></div>`) {
		t.Errorf("friends default output mismatch: %s", out)
	}
	// api 参数透传
	out = runShortcodes("{% friends api:https://example.com/data.json %}")
	if !strings.Contains(out, `data-api="https://example.com/data.json"`) {
		t.Errorf("friends api not passed through: %s", out)
	}
}

func TestSitesRender(t *testing.T) {
	out := runShortcodes("{% sites api:https://example.com/sites.json %}")
	if !strings.Contains(out, `<div class="tag-plugin sites-wrap"><div class="data-service ds-sites" data-api="https://example.com/sites.json"><div class="grid-box"></div></div></div>`) {
		t.Errorf("sites output mismatch: %s", out)
	}
	// api 缺失原样输出（语法容错）
	raw := "{% sites %}"
	if got := runShortcodes(raw); got != raw {
		t.Errorf("sites without api should stay raw, got: %s", got)
	}
}

func TestInlineLabels(t *testing.T) {
	out := runShortcodes("{% u 下划线 %} {% emp 着重 %} {% wavy 波浪 %} {% del 删除 %} {% kbd Ctrl %} {% psw 密码 %} {% blur 模糊 %}")
	for _, want := range []string{
		`<u>下划线</u>`, `<emp>着重</emp>`, `<wavy>波浪</wavy>`, `<del>删除</del>`,
		`<kbd>Ctrl</kbd>`, `<psw>密码</psw>`, `<blur>模糊</blur>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("inline label missing %q: %s", want, out)
		}
	}
}

func TestInlineSupSub(t *testing.T) {
	out := runShortcodes("{% sup color:red 上标 %} 与 {% sub 下标 %}")
	if !strings.Contains(out, `<sup class="tag-plugin colorful sup" color="red">上标</sup>`) {
		t.Errorf("sup output mismatch: %s", out)
	}
	if !strings.Contains(out, `<sub class="tag-plugin colorful sub">下标</sub>`) {
		t.Errorf("sub output mismatch: %s", out)
	}
}

func TestInlineLabelsSkipCode(t *testing.T) {
	md := "{% u 正文生效 %}\n\n```\n{% u 代码块不动 %}\n```\n\n行内 `{% kbd X %}` 也不动"
	out := runShortcodes(md)
	if !strings.Contains(out, `<u>正文生效</u>`) {
		t.Errorf("inline label in body not rendered: %s", out)
	}
	if !strings.Contains(out, "{% u 代码块不动 %}") {
		t.Errorf("fenced code should stay raw: %s", out)
	}
	if !strings.Contains(out, "`{% kbd X %}`") {
		t.Errorf("inline code should stay raw: %s", out)
	}
}
