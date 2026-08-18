package web

import (
	"strings"
	"testing"
)

// renderContent 渲染 partial/content（Spec 71 T3 文章末尾标签行验证用）。
func renderContent(t *testing.T, site *Site, page *Page) string {
	t.Helper()
	reg, err := LoadIconsEmbed()
	if err != nil {
		t.Fatalf("LoadIconsEmbed: %v", err)
	}
	eng, err := NewEngineFromEmbed(reg)
	if err != nil {
		t.Fatalf("NewEngineFromEmbed: %v", err)
	}
	html, err := eng.Render("partial/content", &ViewData{Site: site, Page: page})
	if err != nil {
		t.Fatalf("Render partial/content: %v", err)
	}
	return html
}

// TestArticleEffectiveTags article.tags 开关三态：nil=未配置按 1.41 默认开启，显式 false 关闭。
func TestArticleEffectiveTags(t *testing.T) {
	cases := []struct {
		name string
		tags *bool
		want bool
	}{
		{"未配置默认开", nil, true},
		{"显式开", boolPtr(true), true},
		{"显式关", boolPtr(false), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := (ArticleConfig{Tags: tc.tags}).EffectiveTags(); got != tc.want {
				t.Errorf("EffectiveTags() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestAdaptiveTextAssetsFollowSetting(t *testing.T) {
	engine, err := NewEngineFromEmbed(&IconRegistry{})
	if err != nil {
		t.Fatalf("NewEngineFromEmbed: %v", err)
	}
	render := func(site *Site) string {
		t.Helper()
		html, err := engine.Render("layout/base", &ViewData{
			Site: site,
			Page: &Page{Title: "带封面文章", Layout: "post", Banner: "/cover.jpg"},
		})
		if err != nil {
			t.Fatalf("render layout/base: %v", err)
		}
		return html
	}
	if html := render(&Site{}); !strings.Contains(html, "/assets/js/color.js") || !strings.Contains(html, `data-text-adaptive="split"`) {
		t.Fatal("adaptive text assets should be enabled by default")
	}
	if html := render(&Site{Plugins: PluginsConfig{AdaptiveText: &AdaptiveTextConfig{Enable: false}}}); strings.Contains(html, "/assets/js/color.js") {
		t.Fatal("adaptive text assets should be omitted when explicitly disabled")
	}
}

// TestArticleTagsRowRender 文章末尾标签行渲染：默认开启渲染、显式关闭不渲染、无标签零输出。
func TestArticleTagsRowRender(t *testing.T) {
	post := &Page{Title: "测试", Layout: "post", Tags: []string{"Go", "Stellar"}}

	t.Run("默认开启渲染标签行", func(t *testing.T) {
		html := renderContent(t, &Site{}, post)
		if !strings.Contains(html, `class="article-tags"`) {
			t.Fatalf("未渲染 .article-tags 容器")
		}
		if !strings.Contains(html, `href="/tags/Go"`) || !strings.Contains(html, `href="/tags/Stellar"`) {
			t.Fatalf("标签链接缺失，got: %s", html)
		}
		if !strings.Contains(html, "default:hashtag") && !strings.Contains(html, "<svg") {
			t.Fatalf("标签图标未内联渲染")
		}
	})

	t.Run("显式关闭不渲染", func(t *testing.T) {
		html := renderContent(t, &Site{Article: ArticleConfig{Tags: boolPtr(false)}}, post)
		if strings.Contains(html, "article-tags") {
			t.Fatalf("关闭后不应渲染标签行")
		}
	})

	t.Run("无标签零输出", func(t *testing.T) {
		html := renderContent(t, &Site{}, &Page{Title: "无标签", Layout: "post"})
		if strings.Contains(html, "article-tags") {
			t.Fatalf("无标签文章不应渲染标签行")
		}
	})

	t.Run("非文章页不渲染", func(t *testing.T) {
		html := renderContent(t, &Site{}, &Page{Title: "关于", Layout: "page", Tags: []string{"Go"}})
		if strings.Contains(html, "article-tags") {
			t.Fatalf("非 Layout=post 页面不应渲染文章标签行（对齐上游 layout==post 条件）")
		}
	})
}

// aiLabelSite 构造带四档默认配置的 Site（对齐 DefaultSite 的 AILabel 段）。
func aiLabelSite() *Site {
	return &Site{Article: ArticleConfig{
		AILabel: AILabelConfig{
			Manual:    &AILabelDef{Color: "#03a9f4", Icon: "default:shield-user"},
			Reviewed:  &AILabelDef{Color: "4caf50"},
			Polished:  &AILabelDef{Color: "#4caf50"},
			Generated: &AILabelDef{Color: "#ff9800"},
		},
	}}
}

// TestAILabelResolve ai_label 视图解析：档位回退、default、零渲染与颜色补 #。
func TestAILabelResolve(t *testing.T) {
	site := aiLabelSite()

	t.Run("未写 frontmatter 且 default 空 → 零渲染", func(t *testing.T) {
		vd := &ViewData{Site: site, Page: &Page{Layout: "post"}}
		if v := vd.AILabel(); v != nil {
			t.Fatalf("历史文章应零渲染，got %+v", v)
		}
	})

	t.Run("四档解析", func(t *testing.T) {
		for _, key := range []string{"manual", "reviewed", "polished", "generated"} {
			vd := &ViewData{Site: site, Page: &Page{Layout: "post", AILabel: key}}
			v := vd.AILabel()
			if v == nil {
				t.Fatalf("档位 %s 应渲染", key)
			}
			if v.Text == "" {
				t.Fatalf("档位 %s 文案缺失", key)
			}
		}
	})

	t.Run("颜色缺 # 自动补全", func(t *testing.T) {
		vd := &ViewData{Site: site, Page: &Page{Layout: "post", AILabel: "reviewed"}}
		if v := vd.AILabel(); v.Color != "#4caf50" {
			t.Fatalf("颜色应补 # 前缀，got %q", v.Color)
		}
	})

	t.Run("未知档位零渲染", func(t *testing.T) {
		vd := &ViewData{Site: site, Page: &Page{Layout: "post", AILabel: "half-ai"}}
		if v := vd.AILabel(); v != nil {
			t.Fatalf("未知档位应零渲染（对齐上游未命中零输出），got %+v", v)
		}
	})

	t.Run("default 档位回退", func(t *testing.T) {
		s := aiLabelSite()
		s.Article.AILabel.Default = "generated"
		vd := &ViewData{Site: s, Page: &Page{Layout: "post"}}
		if v := vd.AILabel(); v == nil || v.Text != Lang("meta.ai_label.generated") {
			t.Fatalf("default=generated 时未写 frontmatter 应渲染 generated 档")
		}
	})

	t.Run("配置缺失零渲染", func(t *testing.T) {
		vd := &ViewData{Site: &Site{}, Page: &Page{Layout: "post", AILabel: "manual"}}
		if v := vd.AILabel(); v != nil {
			t.Fatalf("未配置 ai_label 段时应零渲染，got %+v", v)
		}
	})
}

// TestAILabelRender banner 面包屑行的 ai-label span 渲染与零渲染。
func TestAILabelRender(t *testing.T) {
	t.Run("渲染档位标签", func(t *testing.T) {
		html := renderContent(t, aiLabelSite(), &Page{Title: "AI 文", Layout: "post", AILabel: "generated"})
		if !strings.Contains(html, `class="cap right ai-label"`) {
			t.Fatalf("无阅读时长时应输出 cap right ai-label，got: %s", html[:400])
		}
		if !strings.Contains(html, `style="color:#ff9800"`) {
			t.Fatalf("配置色未输出")
		}
		if !strings.Contains(html, Lang("meta.ai_label.generated")) {
			t.Fatalf("文案未输出")
		}
	})
	t.Run("banner 含图时不着色", func(t *testing.T) {
		html := renderContent(t, aiLabelSite(), &Page{Title: "AI 文", Layout: "post", AILabel: "generated", Banner: "/x.jpg"})
		if strings.Contains(html, "style=\"color:#ff9800\"") {
			t.Fatalf("banner 含图时应继承默认文字色（noColor）")
		}
		if !strings.Contains(html, "ai-label") {
			t.Fatalf("标签本体仍应渲染")
		}
	})
	t.Run("未写 frontmatter 零渲染", func(t *testing.T) {
		html := renderContent(t, aiLabelSite(), &Page{Title: "人类文", Layout: "post"})
		if strings.Contains(html, "ai-label") {
			t.Fatalf("历史文章零变化：不应出现 ai-label")
		}
	})
}

// TestAILabelFillDefaults settings 表 article 段整段覆盖后 AILabelConfig 零值：
// FillDefaults 补齐主题内置四档（未选档零渲染语义只由 Default 控制）。
func TestAILabelFillDefaults(t *testing.T) {
	var cfg AILabelConfig // 模拟 settings 行无 ai_label 键 → 零值
	cfg.FillDefaults()
	for _, key := range []string{"manual", "reviewed", "polished", "generated"} {
		if cfg.Def(key) == nil {
			t.Errorf("FillDefaults 后档位 %s 仍为 nil", key)
		}
	}
	if cfg.Default != "" {
		t.Errorf("Default 不应被 FillDefaults 填充（保持零渲染语义），got %q", cfg.Default)
	}
	// 站点显式覆盖某档色，FillDefaults 不动它
	custom := AILabelConfig{Manual: &AILabelDef{Color: "#123456"}}
	custom.FillDefaults()
	if custom.Manual.Color != "#123456" {
		t.Fatalf("显式配置档被覆盖: %+v", custom.Manual)
	}
	// 零值配置 + 零值文章：渲染前补默认不影响零渲染（Default 空）
	vd := &ViewData{Site: &Site{Article: ArticleConfig{AILabel: cfg}}, Page: &Page{Layout: "post"}}
	if v := vd.AILabel(); v != nil {
		t.Fatalf("未选档（Default 空 + frontmatter 空）应零渲染")
	}
}
