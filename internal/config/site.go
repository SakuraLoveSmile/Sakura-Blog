package config

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/goccy/go-yaml"

	tmpl "blog/internal/settings"
)

// LoadSite 从 yaml 文件加载站点配置（如 config/site.yml）。
// 是否回退到 DefaultSite() 由调用方根据运行模式决定；生产环境必须让错误向上传播。
func LoadSite(path string) (*tmpl.Site, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read site config %s: %w", path, err)
	}
	s := &tmpl.Site{}
	if err := yaml.Unmarshal(b, s); err != nil {
		return nil, fmt.Errorf("parse site config %s: %w", path, err)
	}
	if err := normalizeYAMLRuntimeSections(b, s); err != nil {
		return nil, fmt.Errorf("normalize runtime settings %s: %w", path, err)
	}
	normalizeThemeDefaults(s)
	return s, nil
}

// normalizeYAMLRuntimeSections applies the same presence-aware default merge
// used for settings rows. YAML decoding into ordinary Go strings loses whether
// a nested value was omitted or explicitly set to "", so normalize the raw
// runtime sections before the final in-memory defaults are published.
func normalizeYAMLRuntimeSections(data []byte, site *tmpl.Site) error {
	var generic map[string]any
	if err := yaml.Unmarshal(data, &generic); err != nil || generic == nil {
		return err
	}
	encoded, err := json.Marshal(generic)
	if err != nil {
		return err
	}
	var sections map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &sections); err != nil {
		return err
	}

	for _, section := range []string{SectionSearch, SectionDataCache, SectionTagPlugins, SectionServices} {
		raw, ok := sections[section]
		if !ok || string(raw) == "null" {
			continue
		}
		normalized, err := mergeRuntimeDefaults(raw, raw, section)
		if err != nil {
			return err
		}
		switch section {
		case SectionSearch:
			if err := json.Unmarshal(normalized, &site.Search); err != nil {
				return err
			}
		case SectionDataCache:
			if err := json.Unmarshal(normalized, &site.DataCache); err != nil {
				return err
			}
		case SectionTagPlugins:
			if err := json.Unmarshal(normalized, &site.TagPlugins); err != nil {
				return err
			}
		case SectionServices:
			if err := json.Unmarshal(normalized, &site.Services); err != nil {
				return err
			}
		}
	}
	return nil
}

// normalizeThemeDefaults keeps site.yml and legacy settings compatible with the
// 1.40 defaults when newly added keys are absent.
func normalizeThemeDefaults(s *tmpl.Site) {
	if s.Style.CornerShape == "" {
		s.Style.CornerShape = "superellipse(1.25)"
	}
	if s.Style.Leftbar.UIStyle == "" {
		s.Style.Leftbar.UIStyle = "card"
	}
	if s.Style.Gradient.Avatar == "" {
		s.Style.Gradient.Avatar = "conic-gradient(from 0deg, #04f3ff, #08ffc6, #ddf730, #ffbd19, #ff1fe0, #c418ff, #3b5bff, #04f3ff)"
	}
	if s.Article.PinStyle == "" {
		s.Article.PinStyle = "carousel"
	}
	// 1.41 ai_label：site.yml 未配 ai_label 段时补主题内置四档（DefaultSite 已显式含）
	s.Article.AILabel.FillDefaults()
	// Stellar 运行时配置默认值：settings 与 site.yml 共用同一份补齐逻辑。
	s.Search.FillDefaults()
	s.DataCache.FillDefaults()
	s.TagPlugins.FillDefaults()
}

// DefaultSite 构造 Demo 2 用的站点配置。
// 菜单配色和图标对齐 preview.html 已验证的值（青/绿/橙/红 + solar duotone）。
// 图标 key 对应 resource/hexo-theme-stellar/_data/icons.yml。
func DefaultSite() *tmpl.Site {
	return &tmpl.Site{
		Title:       "Stellar Demo",
		Subtitle:    "Go 服务端渲染",
		Description: "用 Go (Gin) + html/template + goldmark 重建 Stellar 风格博客，验证服务端渲染链路。",
		Avatar:      "https://gcore.jsdelivr.net/gh/cdn-x/placeholder@1.0.12/avatar/round/3442075.svg",
		AvatarBG:    "https://gcore.jsdelivr.net/gh/cdn-x/placeholder@1.0.12/avatar/round/rainbow64@3x.webp",
		LogoTitle:   "Stellar Demo",
		LogoSub:     "Go SSR 验证|Stellar 主题",
		Menubar: tmpl.Menubar{
			Columns: 2,
			Items: []tmpl.MenuItem{
				{ID: "post", Title: "博客", URL: "/", Icon: "default:documents", Theme: "#1BCDFC"},
				{ID: "wiki", Title: "文库", URL: "/wiki", Icon: "example:notebook", Theme: "#3DC550"},
				{ID: "notes", Title: "笔记", URL: "/notes", Icon: "default:edit", Theme: "#FA7D23"},
				{ID: "friends", Title: "友链", URL: "/friends", Icon: "example:planet", Theme: "#FF6B9D"},
			},
		},
		Rightbar: tmpl.RightbarConfig{
			// 欢迎语卡片链接列表（对齐原版 markdown widget 的 linklist）
			WelcomeLinklist: tmpl.LinkListConfig{
				Items: []tmpl.LinkListItem{
					{Title: "查看文库", URL: "/wiki", Icon: "example:notebook"},
					{Title: "传送到笔记", URL: "/notes", Icon: "default:edit"},
				},
			},
		},
		Footer: tmpl.Footer{
			Social: []tmpl.SocialItem{
				{Title: "GitHub", URL: "https://github.com/", Icon: `<img src="https://gcore.jsdelivr.net/gh/cdn-x/placeholder@1.0.12/social/08a41b181ce68.svg">`},
				{Title: "Music", URL: "#", Icon: `<img src="https://gcore.jsdelivr.net/gh/cdn-x/placeholder@1.0.12/social/3845874.svg">`},
				{Title: "Unsplash", URL: "#", Icon: `<img src="https://gcore.jsdelivr.net/gh/cdn-x/placeholder@1.0.12/social/3616429.svg">`},
				{Title: "Comments", URL: "#", Icon: `<img src="https://gcore.jsdelivr.net/gh/cdn-x/placeholder@1.0.12/social/942ebbf1a4b91.svg">`},
			},
			// 页脚站点地图（对齐原版 theme.footer.sitemap 多列）
			Sitemap: []tmpl.SitemapGroup{
				{Title: "博客", Items: []tmpl.SitemapItem{
					{Title: "近期发布", URL: "/"},
					{Title: "分类", URL: "/categories/"},
					{Title: "标签", URL: "/tags/"},
					{Title: "归档", URL: "/archives/"},
				}},
				{Title: "页面", Items: []tmpl.SitemapItem{
					{Title: "文库", URL: "/wiki"},
					{Title: "笔记", URL: "/notes"},
					{Title: "友链", URL: "/friends"},
					{Title: "瞬间", URL: "/moments"},
				}},
				{Title: "订阅", Items: []tmpl.SitemapItem{
					{Title: "RSS", URL: "/rss.xml"},
					{Title: "Atom", URL: "/atom.xml"},
				}},
			},
		},
		// NavTabs 缺省零值：空 items + enable_custom=false（导航栏保持固定 tab，Spec 50）
		Comments: tmpl.CommentsConfig{
			Enabled:        true,
			Title:          "评论",
			Moderation:     false, // 默认直接发布；true=先审后发
			AdminBadge:     "博主",
			GravatarMirror: "https://weavatar.com/avatar",
		},
		Article: tmpl.ArticleConfig{
			PinStyle: "carousel",
			Type:     "tech", // 文章列表卡片布局：tech（紧凑）/story（图文大间距），Spec 50
			// 末尾标签行（1.41）：显式默认开启，与上游 article.tags 缺省一致
			Tags: boolPtr(true),
			// AI 成分标签四档（1.41）：对齐上游 _config.yml 默认色与 shield 图标；
			// default 留空=未写 frontmatter 的历史文章零渲染
			AILabel: tmpl.AILabelConfig{
				Manual:    &tmpl.AILabelDef{Color: "#03a9f4", Icon: "default:shield-user"},
				Reviewed:  &tmpl.AILabelDef{Color: "#4caf50", Icon: "default:shield-check"},
				Polished:  &tmpl.AILabelDef{Color: "#4caf50", Icon: "default:shield-up"},
				Generated: &tmpl.AILabelDef{Color: "#ff9800", Icon: "default:shield-warning"},
			},
			CategoryColor: map[string]string{"技术": "#1BCDFC"}, // 对齐原版 theme.article.category_color
			Copyright: tmpl.CopyrightConfig{
				Enabled:    true,
				License:    "署名-非商业性使用-相同方式共享 4.0 国际",
				LicenseURL: "https://creativecommons.org/licenses/by-nc-sa/4.0/",
			},
			Share: tmpl.ShareConfig{
				Enabled:   true,
				Platforms: []string{"wechat", "weibo", "email", "link"},
			},
			Reward: tmpl.RewardConfig{
				Enabled: false,
				Text:    "打赏",
			},
			// Spec 32 文章页首尾增强缺省值（与 site.example.yml 同步）
			ReadNext:     true,
			RelatedPosts: 5,
			ExpireDays:   180,
			BannerRatio:  2.5,
			EditThisPage: nil, // 缺省不配映射；配置后文章页出现贡献者/编辑本文段
		},
		Search: tmpl.SearchConfig{
			Service: "local_search",
			LocalSearch: tmpl.LocalSearchConfig{
				Field: "all", Path: "/search.json", Content: boolPtr(true),
				// 1.41：懒加载默认开、缓存默认 1 天（Effective* 方法同值兜底，此处显式化）
				LazyLoad: boolPtr(true),
				CacheTTL: intPtr(86400),
			},
		},
		Style: tmpl.StyleConfig{
			CornerShape: "superellipse(1.25)",
			Leftbar:     tmpl.StyleLeftbar{UIStyle: "card"},
			Gradient:    tmpl.StyleGradient{Avatar: "conic-gradient(from 0deg, #04f3ff, #08ffc6, #ddf730, #ffbd19, #ff1fe0, #c418ff, #3b5bff, #04f3ff)"},
		},
		Links: nil, // 友链由 site.yml links 段驱动；缺省为空，/links 显示「暂无友链」
		Friends: tmpl.FriendsConfig{
			RefreshInterval:  60,
			MaxPostsPerFeed:  20,
			CheckInterval:    360,
			SnapshotInterval: 1440,
			SnapshotProvider: tmpl.DefaultSnapshotProvider,
		},
	}
}

func boolPtr(v bool) *bool { return &v }

func intPtr(v int) *int { return &v }
