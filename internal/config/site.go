package config

import (
	"fmt"
	"os"

	"github.com/goccy/go-yaml"

	tmpl "blog/internal/template"
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
	return s, nil
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
				{ID: "archive", Title: "归档", URL: "/archives/", Icon: "default:calendar", Theme: "#3DC550"},
				{ID: "links", Title: "友链", URL: "/links", Icon: "solar:chat-square-like-bold-duotone", Theme: "#FA7D23"},
			},
		},
		Footer: tmpl.Footer{
			Social: []tmpl.SocialItem{
				{Title: "GitHub", URL: "https://github.com/", Icon: `<img src="https://gcore.jsdelivr.net/gh/cdn-x/placeholder@1.0.12/social/08a41b181ce68.svg">`},
				{Title: "Music", URL: "#", Icon: `<img src="https://gcore.jsdelivr.net/gh/cdn-x/placeholder@1.0.12/social/3845874.svg">`},
				{Title: "Unsplash", URL: "#", Icon: `<img src="https://gcore.jsdelivr.net/gh/cdn-x/placeholder@1.0.12/social/3616429.svg">`},
				{Title: "Comments", URL: "#", Icon: `<img src="https://gcore.jsdelivr.net/gh/cdn-x/placeholder@1.0.12/social/942ebbf1a4b91.svg">`},
			},
		},
		Comments: tmpl.CommentsConfig{
			Enabled:        true,
			Title:          "评论",
			Moderation:     false, // 默认直接发布；true=先审后发
			AdminBadge:     "博主",
			GravatarMirror: "https://weavatar.com/avatar",
		},
		Article: tmpl.ArticleConfig{
			Copyright: tmpl.CopyrightConfig{
				Enabled:    true,
				License:    "署名-非商业性使用-相同方式共享 4.0 国际",
				LicenseURL: "https://creativecommons.org/licenses/by-nc-sa/4.0/",
			},
			Share: tmpl.ShareConfig{
				Enabled:   true,
				Platforms: []string{"weibo", "x", "qq", "link"},
			},
			Reward: tmpl.RewardConfig{
				Enabled: false,
				Text:    "打赏",
			},
		},
		Links: nil, // 友链由 site.yml links 段驱动；缺省为空，/links 显示「暂无友链」
	}
}
