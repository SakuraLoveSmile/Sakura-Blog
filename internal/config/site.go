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
		LogoTitle:   "Stellar Demo",
		LogoSub:     "Go SSR 验证",
		Menubar: tmpl.Menubar{
			Columns: 4,
			Items: []tmpl.MenuItem{
				{ID: "post", Title: "博客", URL: "/", Icon: "solar:documents-bold-duotone", Theme: "#1BCDFC"},
				{ID: "archive", Title: "归档", URL: "/archives/", Icon: "default:calendar", Theme: "#3DC550"},
				{ID: "explore", Title: "探索", URL: "/explore/", Icon: "solar:planet-bold-duotone", Theme: "#FA6400"},
				{ID: "social", Title: "社交", URL: "/friends/", Icon: "solar:chat-square-like-bold-duotone", Theme: "#F44336"},
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
		RecentPosts: []tmpl.RecentPost{
			{Title: "Demo 2：Go 服务端渲染验证", URL: "/post/hello"},
			{Title: "Demo 1：Stellar CSS 编译验证", URL: "#"},
			{Title: "用 shim 方案脱离 Hexo 编译 Stylus", URL: "#"},
			{Title: "深色模式与菜单图标机制", URL: "#"},
			{Title: "Artalk 评论集成方案", URL: "#"},
		},
		Tags: []string{"CSS", "Stellar", "Go", "Gin", "Hexo", "SQLite"},
	}
}
