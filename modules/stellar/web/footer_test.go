package web

import (
	"strings"
	"testing"
)

// renderFooter 渲染 partial/footer 模板（Spec 55 Part D 备案号验证用）。
func renderFooter(t *testing.T, footer Footer) string {
	t.Helper()
	reg, err := LoadIconsEmbed()
	if err != nil {
		t.Fatalf("LoadIconsEmbed: %v", err)
	}
	eng, err := NewEngineFromEmbed(reg)
	if err != nil {
		t.Fatalf("NewEngineFromEmbed: %v", err)
	}
	html, err := eng.Render("partial/footer", &ViewData{
		Site: &Site{Title: "测试站点", Footer: footer},
	})
	if err != nil {
		t.Fatalf("Render partial/footer: %v", err)
	}
	return html
}

// TestFooterBeianEmpty beian 为空时页脚与现状一致：不出现备案链接。
func TestFooterBeianEmpty(t *testing.T) {
	html := renderFooter(t, Footer{})
	if !strings.Contains(html, "&copy; 测试站点") {
		t.Fatalf("署名行缺失，got: %s", html)
	}
	if strings.Contains(html, "beian.miit.gov.cn") {
		t.Fatalf("beian 为空时不应渲染备案链接，got: %s", html)
	}
}

// TestFooterBeianRendered beian 非空时署名行下方渲染备案号并链接工信部备案系统。
func TestFooterBeianRendered(t *testing.T) {
	html := renderFooter(t, Footer{Beian: "某ICP备00000000号"})
	if !strings.Contains(html, `<a href="https://beian.miit.gov.cn/" target="_blank" rel="noopener">某ICP备00000000号</a>`) {
		t.Fatalf("备案链接未按预期渲染，got: %s", html)
	}
}
