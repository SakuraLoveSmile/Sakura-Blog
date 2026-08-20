package config

import (
	"encoding/json"
	"fmt"
	"log"

	tmpl "blog/internal/settings"
	"blog/internal/store"
)

// 站点配置分段名（settings 表的 section 主键）。
const (
	SectionSite      = "site"      // 基础信息（标题/副标题/头像/logo）
	SectionMenubar   = "menubar"   // 菜单栏
	SectionRightbar  = "rightbar"  // 右栏
	SectionFooter    = "footer"    // 页脚
	SectionComments  = "comments"  // 评论系统
	SectionArticle   = "article"   // 文章页脚
	SectionSearch    = "search"    // 本地搜索
	SectionSiteTree  = "site_tree" // 页面树（首页评论等）
	SectionWiki      = "wiki"      // 文库项目
	SectionFriends   = "friends"   // 友链动态聚合
	SectionSEO       = "seo"       // 站点级 SEO（Spec 42）
	SectionInject    = "inject"    // 代码注入（Spec 42，仅入库不入 site.yml）
	SectionDashboard = "dashboard" // 仪表盘组件自定义（仅 Console 消费）
	SectionStyle     = "style"     // 外观样式（Spec 34/47）
	SectionWidgets   = "widgets"   // 侧边栏组件编排（Spec 33/47）
	SectionAI        = "ai"        // AI 属性生成（OpenAI 兼容接口配置，仅 Console/后端代理消费）
	// 以下五段此前为 site.yml 纯文件配置，现接入 settings 热更新管线（入库后以 DB 为准）：
	SectionCover      = "cover"       // 首页 Hero cover 区
	SectionNavTabs    = "nav_tabs"    // 导航栏标签页
	SectionAuthors    = "authors"     // 多作者体系
	SectionPlugins    = "plugins"     // 前端体验增强插件开关
	SectionTagPlugins = "tag_plugins" // 标签插件默认参数（对齐原版 _config.yml tag_plugins）
	SectionDefault    = "default"     // 占位素材组（对齐原版 _config.yml default）
	SectionPages      = "pages"       // 页面级设置（friends 页等特色设置，按页面键管理）
	SectionNotebook   = "notebook"    // 笔记本默认行为
	SectionDataCache  = "data_cache"  // 动态数据客户端缓存（base.html 注入 ctx.data_cache 消费）

	// SectionAdmin 管理密码段（迁移用，Spec 61 阶段 4）：明文仅存于迁移窗口期——
	// Bootstrap 检测非空时迁移到 users 表（bcrypt）并清空，此后不再写入。
	SectionAdmin = "admin"
)

// siteBasic 对应 settings 表 "site" 段的基础信息。
type siteBasic struct {
	Title       string `json:"title"`
	Subtitle    string `json:"subtitle"`
	Description string `json:"description"`
	URL         string `json:"url"`
	Avatar      string `json:"avatar"`
	AvatarBG    string `json:"avatar_bg"`
	LogoTitle   string `json:"logo_title"`
	LogoSub     string `json:"logo_sub"`
	// favicon/apple_touch_icon：Spec Task 9 的 Console UI 依赖；sw_cache_version 供 /sw.js 注入 CACHE_VERSION。
	FaviconURL string `json:"favicon_url"`      // Favicon URL，留空回退 /favicon.ico
	AppleIcon  string `json:"apple_touch_icon"` // Apple Touch Icon URL，留空回退默认图
	SWCacheVer string `json:"sw_cache_version"` // Service Worker 缓存版本，留空用内置缺省
}

// dashboardWidget 对应 settings 表 "dashboard" 段的单个组件配置。
type dashboardWidget struct {
	ID      string `json:"id"`
	Enabled bool   `json:"enabled"`
}

// dashboardConfig 对应 settings 表 "dashboard" 段（仪表盘组件开关与排序）。
type dashboardConfig struct {
	Widgets []dashboardWidget `json:"widgets"`
}

// Bootstrap 首次启动导入：settings/links 表为空时，把 site（DefaultSite 合并 site.yml）写入 DB。
// 幂等——非空表跳过，二次启动不重复导入。
func Bootstrap(st *store.Store, site *tmpl.Site) error {
	if err := st.EnsureSettingsTable(); err != nil {
		return err
	}
	if err := st.EnsureLinksTable(); err != nil {
		return err
	}

	n, err := st.CountSettings()
	if err != nil {
		return err
	}
	if n == 0 {
		if err := writeSiteSections(st, site); err != nil {
			return err
		}
	}

	settings, err := st.AllSettings()
	if err != nil {
		return err
	}
	// Wiki 项目从 site.yml/settings.wiki 一次性迁移到独立表；之后运行时不再
	// 依赖 settings.wiki，项目管理由 Wiki Store/API 负责。
	legacyWiki := make([]*store.WikiProject, 0, len(site.Wiki))
	for i := range site.Wiki {
		legacyWiki = append(legacyWiki, storeWikiProject(&site.Wiki[i]))
	}
	if len(legacyWiki) == 0 && settings[SectionWiki] != "" {
		var oldWiki []tmpl.WikiProject
		if json.Unmarshal([]byte(settings[SectionWiki]), &oldWiki) == nil {
			for i := range oldWiki {
				legacyWiki = append(legacyWiki, storeWikiProject(&oldWiki[i]))
			}
		}
	}
	if err := st.MigrateWikiProjects(legacyWiki); err != nil {
		return fmt.Errorf("migrate wiki projects: %w", err)
	}
	// admin 段兼容补导（Spec 61 阶段 4 收紧）：仅当 users 表 admin 尚无密码哈希时，
	// 才把 site.yml 初始密码补入 admin 段供下方迁移消费；已迁移的库不再从 site.yml 回灌明文。
	if _, ok := settings[SectionAdmin]; !ok && site.Admin.Password != "" {
		if adminUser, _ := st.GetUserByUsername("admin"); adminUser == nil || adminUser.PasswordHash == "" {
			b, err := json.Marshal(map[string]string{"password": site.Admin.Password})
			if err != nil {
				return err
			}
			if err := st.PutSetting(SectionAdmin, string(b)); err != nil {
				return err
			}
		}
	}

	// seo/inject 段兼容导入（Spec 42）：旧库升级时 settings 非空但缺段，补导一次，
	// 避免 RebuildSite 用零值覆盖 site.yml 加载的 seo 配置。
	if _, ok := settings[SectionSEO]; !ok {
		b, err := json.Marshal(site.SEO)
		if err != nil {
			return err
		}
		if err := st.PutSetting(SectionSEO, string(b)); err != nil {
			return err
		}
	}
	if _, ok := settings[SectionSearch]; !ok {
		b, err := json.Marshal(site.Search)
		if err != nil {
			return err
		}
		if err := st.PutSetting(SectionSearch, string(b)); err != nil {
			return err
		}
	}
	if _, ok := settings[SectionSiteTree]; !ok {
		b, err := json.Marshal(site.SiteTree)
		if err != nil {
			return err
		}
		if err := st.PutSetting(SectionSiteTree, string(b)); err != nil {
			return err
		}
	}
	if _, ok := settings[SectionNotebook]; !ok {
		b, err := json.Marshal(site.Notebook)
		if err != nil {
			return err
		}
		if err := st.PutSetting(SectionNotebook, string(b)); err != nil {
			return err
		}
	}
	if _, ok := settings[SectionInject]; !ok {
		b, err := json.Marshal(site.Inject)
		if err != nil {
			return err
		}
		if err := st.PutSetting(SectionInject, string(b)); err != nil {
			return err
		}
	}
	if _, ok := settings[SectionStyle]; !ok {
		b, _ := json.Marshal(site.Style)
		st.PutSetting(SectionStyle, string(b))
	}
	if _, ok := settings[SectionWidgets]; !ok {
		b, _ := json.Marshal(site.Widgets)
		st.PutSetting(SectionWidgets, string(b))
	}
	// cover/nav_tabs/authors/plugins 段兼容导入：旧库升级时 settings 非空但缺段，
	// 从 site.yml 初值补导一次，之后以 DB 为准（与 seo/style/widgets 同语义）。
	if _, ok := settings[SectionCover]; !ok {
		b, _ := json.Marshal(site.Cover)
		st.PutSetting(SectionCover, string(b))
	}
	if _, ok := settings[SectionNavTabs]; !ok {
		b, _ := json.Marshal(site.NavTabs)
		st.PutSetting(SectionNavTabs, string(b))
	}
	if _, ok := settings[SectionAuthors]; !ok {
		b, _ := json.Marshal(site.Authors)
		st.PutSetting(SectionAuthors, string(b))
	}
	if _, ok := settings[SectionPlugins]; !ok {
		b, _ := json.Marshal(site.Plugins)
		st.PutSetting(SectionPlugins, string(b))
	}
	// data_cache 段兼容导入：旧库升级时 settings 非空但缺段，从 site.yml 初值补导一次
	//（与 seo/style/widgets 同语义），之后以 DB 为准。
	if _, ok := settings[SectionDataCache]; !ok {
		b, _ := json.Marshal(site.DataCache)
		st.PutSetting(SectionDataCache, string(b))
	}

	// site 段 favicon 兼容导入：旧库 site 段缺 favicon_url/apple_touch_icon 键时，
	// 从 site.yml 初值合并补写，避免 RebuildSite 把已加载值清零（site.yml 留空则不补）。
	if raw, ok := settings[SectionSite]; ok && raw != "" {
		var sec map[string]json.RawMessage
		if json.Unmarshal([]byte(raw), &sec) == nil {
			changed := false
			if _, has := sec["favicon_url"]; !has && site.FaviconURL != "" {
				b, _ := json.Marshal(site.FaviconURL)
				sec["favicon_url"] = b
				changed = true
			}
			if _, has := sec["apple_touch_icon"]; !has && site.AppleIcon != "" {
				b, _ := json.Marshal(site.AppleIcon)
				sec["apple_touch_icon"] = b
				changed = true
			}
			if changed {
				if b, err := json.Marshal(sec); err == nil {
					st.PutSetting(SectionSite, string(b))
				}
			}
		}
	}

	nl, err := st.CountLinks()
	if err != nil {
		return err
	}
	if nl == 0 && len(site.Links) > 0 {
		if err := writeSiteLinks(st, site.Links); err != nil {
			return err
		}
	}

	// Spec 61 阶段 4: admin 密码迁移到 users 表（幂等）。
	// settings admin 段非空（旧版本明文密码）时：确保 users 表 admin 用户存在且 bcrypt 匹配该密码
	// （缺失/不匹配则创建/更新），随后清空 admin 段；之后密码唯一真值在 users 表。
	if err := st.EnsureUsersTable(); err != nil {
		log.Printf("[warn] EnsureUsersTable: %v", err)
	}
	if settings, _ := st.AllSettings(); settings[SectionAdmin] != "" {
		var adminCfg struct {
			Password string `json:"password"`
		}
		if json.Unmarshal([]byte(settings[SectionAdmin]), &adminCfg) == nil && adminCfg.Password != "" {
			admin, err := st.GetUserByUsername("admin")
			if err != nil {
				return err
			}
			needCreate := admin == nil
			needUpdate := !needCreate && store.CheckPassword(admin.PasswordHash, adminCfg.Password) != nil
			if needCreate {
				hash, err := store.HashPassword(adminCfg.Password)
				if err != nil {
					return err
				}
				admin = &store.User{
					Username:     "admin",
					PasswordHash: hash,
					DisplayName:  "管理员",
					AuthSecret:   store.GenerateAuthSecret(),
					Role:         store.RoleAdmin,
				}
				if err := st.CreateUser(admin); err != nil {
					log.Printf("[warn] migrate admin user: %v", err)
				} else {
					log.Printf("[info] Spec 61: migrated admin password to users table (user: admin)")
				}
			} else if needUpdate {
				hash, err := store.HashPassword(adminCfg.Password)
				if err != nil {
					return err
				}
				if err := st.UpdatePassword(admin.ID, hash, admin.AuthSecret); err != nil {
					log.Printf("[warn] update admin password hash: %v", err)
				} else {
					log.Printf("[info] Spec 61: refreshed admin password hash from settings admin section")
				}
			}
		}
		// 无论是否迁移成功，清空明文 admin 段（此后密码只存在于 users 表）
		if err := st.PutSetting(SectionAdmin, ""); err != nil {
			log.Printf("[warn] clear admin section: %v", err)
		}
	}

	return nil
}

func storeWikiProject(project *tmpl.WikiProject) *store.WikiProject {
	if project == nil {
		return nil
	}
	p := &store.WikiProject{
		ID: project.ID, Name: project.Name, Title: project.Title, Headline: project.Headline,
		Subtitle: project.Subtitle, Description: project.Description, Icon: project.Icon,
		Cover: project.Cover, Available: project.Available, URL: project.URL, Repo: project.Repo,
		Branch: project.Branch, Sort: project.Sort, Pin: project.Pin, Visible: project.Visible == nil || *project.Visible,
		Tags: append([]string(nil), project.Tags...),
	}
	if project.Hero != nil {
		h := &store.WikiHero{
			Background: project.Hero.Background, Title: project.Hero.Title,
			Description: project.Hero.Description, Start: project.Hero.Start,
		}
		if project.Hero.Preview != nil {
			h.Preview = &store.WikiHeroPreview{Type: project.Hero.Preview.Type, Image: project.Hero.Preview.Image}
			for _, cmd := range project.Hero.Preview.Commands {
				h.Preview.Commands = append(h.Preview.Commands, store.WikiHeroCommand{Codes: append([]string(nil), cmd.Codes...)})
			}
		}
		for _, action := range project.Hero.Actions {
			h.Actions = append(h.Actions, store.WikiHeroAction{Title: action.Title, URL: action.URL, Icon: action.Icon})
		}
		p.Hero = h
	}
	return p
}

// writeSiteSections 把 site 各段序列化为 JSON 写入 settings 表。
func writeSiteSections(st *store.Store, site *tmpl.Site) error {
	put := func(section string, v any) error {
		b, err := json.Marshal(v)
		if err != nil {
			return err
		}
		return st.PutSetting(section, string(b))
	}
	if err := put(SectionSite, siteBasic{
		Title: site.Title, Subtitle: site.Subtitle, Description: site.Description,
		URL: site.URL, Avatar: site.Avatar, AvatarBG: site.AvatarBG,
		LogoTitle: site.LogoTitle, LogoSub: site.LogoSub,
	}); err != nil {
		return err
	}
	if err := put(SectionMenubar, site.Menubar); err != nil {
		return err
	}
	if err := put(SectionRightbar, site.Rightbar); err != nil {
		return err
	}
	if err := put(SectionFooter, site.Footer); err != nil {
		return err
	}
	if err := put(SectionComments, site.Comments); err != nil {
		return err
	}
	if err := put(SectionArticle, site.Article); err != nil {
		return err
	}
	if err := put(SectionSearch, site.Search); err != nil {
		return err
	}
	if err := put(SectionSiteTree, site.SiteTree); err != nil {
		return err
	}
	if err := put(SectionWiki, site.Wiki); err != nil {
		return err
	}
	if err := put(SectionFriends, site.Friends); err != nil {
		return err
	}
	if err := put(SectionSEO, site.SEO); err != nil {
		return err
	}
	if err := put(SectionInject, site.Inject); err != nil {
		return err
	}
	if err := put(SectionDashboard, defaultDashboardConfig()); err != nil {
		return err
	}
	if err := put(SectionStyle, site.Style); err != nil {
		return err
	}
	if err := put(SectionWidgets, site.Widgets); err != nil {
		return err
	}
	if err := put(SectionCover, site.Cover); err != nil {
		return err
	}
	if err := put(SectionNavTabs, site.NavTabs); err != nil {
		return err
	}
	if err := put(SectionAuthors, site.Authors); err != nil {
		return err
	}
	if err := put(SectionPlugins, site.Plugins); err != nil {
		return err
	}
	if err := put(SectionNotebook, site.Notebook); err != nil {
		return err
	}
	if err := put(SectionPages, site.Pages); err != nil {
		return err
	}
	if err := put(SectionDataCache, site.DataCache); err != nil {
		return err
	}
	// lang 段不写初始值：settings 无 lang 行 = 无文案覆盖（全用内置 zhCN）。
	return nil
}

// writeSiteLinks 把 site.Links（分组）扁平化写入 links 表。
func writeSiteLinks(st *store.Store, groups []tmpl.LinkGroup) error {
	for _, g := range groups {
		for _, it := range g.Items {
			if _, err := st.CreateLink(&store.Link{
				GroupTitle:  g.Title,
				Name:        it.Name,
				URL:         it.URL,
				Avatar:      it.Avatar,
				Description: it.Desc,
				RSS:         it.RSS,
				Label:       it.Label,
				Color:       it.Color,
			}); err != nil {
				return err
			}
		}
	}
	return nil
}
