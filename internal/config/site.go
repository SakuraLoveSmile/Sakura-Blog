package config

import (
	"fmt"
	"os"

	"github.com/goccy/go-yaml"

	tmpl "blog/internal/settings"
)

// LoadSite 从 yaml 文件加载站点配置（如 config/site.yml）。
// 失败时调用方应 fallback 到 DefaultSite()，保证 demo 不因配置问题起不来。
func LoadSite(path string) (*tmpl.Site, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read site config %s: %w", path, err)
	}
	s := &tmpl.Site{}
	if err := yaml.Unmarshal(b, s); err != nil {
		return nil, fmt.Errorf("parse site config %s: %w", path, err)
	}
	normalizeThemeDefaults(s)
	return s, nil
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
	if s.Notebook.AutoExcerpt == 0 {
		s.Notebook.AutoExcerpt = 128
	}
	if s.Notebook.OrderBy == "" {
		s.Notebook.OrderBy = "-updated"
	}
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
				{ID: "post", Title: "博客", URL: "/", Icon: "solar:documents-bold-duotone", Theme: "#1BCDFC"},
				{ID: "wiki", Title: "文库", URL: "/wiki", Icon: "solar:notebook-bookmark-bold-duotone", Theme: "#3DC550"},
				{ID: "notes", Title: "笔记", URL: "/notes", Icon: "default:edit", Theme: "#FA7D23"},
				{ID: "friends", Title: "友链", URL: "/friends", Icon: "solar:planet-bold-duotone", Theme: "#FF6B9D"},
			},
		},
		Rightbar: tmpl.RightbarConfig{
			// 欢迎语卡片链接列表（对齐原版 markdown widget 的 linklist）
			WelcomeLinklist: tmpl.LinkListConfig{
				Items: []tmpl.LinkListItem{
					{Title: "查看文库", URL: "/wiki", Icon: "solar:notebook-bookmark-bold-duotone"},
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
			PinStyle:      "carousel",
			Type:          "tech",                             // 文章列表卡片布局：tech（紧凑）/story（图文大间距），Spec 50
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
			Service:     "local_search",
			LocalSearch: tmpl.LocalSearchConfig{Field: "all", Path: "/search.json", Content: boolPtr(true)},
		},
		Notebook: tmpl.NotebookConfig{AutoExcerpt: 128, OrderBy: "-updated", Share: false},
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
