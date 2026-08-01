package config

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"blog/internal/store"
	tmpl "blog/internal/template"
)

// 站点配置分段名（settings 表的 section 主键）。
const (
	SectionSite      = "site"      // 基础信息（标题/副标题/头像/logo）
	SectionMenubar   = "menubar"   // 菜单栏
	SectionRightbar  = "rightbar"  // 右栏
	SectionFooter    = "footer"    // 页脚
	SectionComments  = "comments"  // 评论系统
	SectionArticle   = "article"   // 文章页脚
	SectionWiki      = "wiki"      // 文库项目
	SectionFriends   = "friends"   // 友链动态聚合
	SectionSEO       = "seo"       // 站点级 SEO（Spec 42）
	SectionInject    = "inject"    // 代码注入（Spec 42，仅入库不入 site.yml）
	SectionDashboard = "dashboard" // 仪表盘组件自定义（仅 Console 消费）
	SectionStyle     = "style"     // 外观样式（Spec 34/47）
	SectionWidgets   = "widgets"   // 侧边栏组件编排（Spec 33/47）
	SectionAI        = "ai"        // AI 属性生成（OpenAI 兼容接口配置，仅 Console/后端代理消费）

	// SectionAdmin 管理密码段。故意不入 KnownSections：不经 GET /settings 暴露，
	// 仅由 SaveAdminPassword 专用接口读写。
	SectionAdmin = "admin"
)

// siteMu 保护 RebuildSite 对共享 *tmpl.Site 的就地写入（写者之间互斥）。
// 注意：前台渲染的读者未持锁读取 site 字段，理论上与 RebuildSite 写存在数据竞争；
// 但设置修改是低频管理操作，此权衡为本项目（个人博客、低并发）可接受，符合计划的风险说明。
var siteMu sync.Mutex

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
}

// adminSection 对应 settings 表 "admin" 段（管理密码，明文存储与 site.yml 现状同安全等级）。
type adminSection struct {
	Password string `json:"password"`
}

// AIConfig 对应 settings 表 "ai" 段：OpenAI 兼容接口配置，供 AI 属性生成代理使用。
type AIConfig struct {
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key"`
	Model   string `json:"model"`
}

// LoadAI 从 settings 读取 AI 配置（未配置时返回零值）。
func LoadAI(st *store.Store) (AIConfig, error) {
	var cfg AIConfig
	v, err := st.GetSetting(SectionAI)
	if err != nil || v == "" {
		return cfg, err
	}
	if err := json.Unmarshal([]byte(v), &cfg); err != nil {
		return AIConfig{}, err
	}
	return cfg, nil
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

	// admin 段单独兼容导入：旧库升级时 settings 非空但无 admin 段，site.yml 有密码则补导一次。
	settings, err := st.AllSettings()
	if err != nil {
		return err
	}
	if _, ok := settings[SectionAdmin]; !ok && site.Admin.Password != "" {
		b, err := json.Marshal(adminSection{Password: site.Admin.Password})
		if err != nil {
			return err
		}
		if err := st.PutSetting(SectionAdmin, string(b)); err != nil {
			return err
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

	nl, err := st.CountLinks()
	if err != nil {
		return err
	}
	if nl == 0 && len(site.Links) > 0 {
		if err := writeSiteLinks(st, site.Links); err != nil {
			return err
		}
	}

	// Spec 45: 迁移 admin 密码到 users 表
	if err := st.EnsureUsersTable(); err != nil {
		log.Printf("[warn] EnsureUsersTable: %v", err)
	}
	users, _ := st.ListUsers()
	if len(users) == 0 {
		settings, _ := st.AllSettings()
		if adminJSON, ok := settings[SectionAdmin]; ok && adminJSON != "" {
			var adminCfg struct {
				Password string `json:"password"`
			}
			if json.Unmarshal([]byte(adminJSON), &adminCfg) == nil && adminCfg.Password != "" {
				hash, err := store.HashPassword(adminCfg.Password)
				if err == nil {
					admin := &store.User{
						Username:     "admin",
						PasswordHash: hash,
						DisplayName:  "管理员",
						AuthSecret:   store.GenerateAuthSecret(),
					}
					if err := st.CreateUser(admin); err != nil {
						log.Printf("[warn] migrate admin user: %v", err)
					} else {
						log.Printf("[info] Spec 45: migrated admin password to users table (user: admin)")
					}
				}
			}
		}
	}

	return nil
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

// RebuildSite 从 DB 重新加载 settings/links，就地更新共享 *site 的字段（写锁保护）。
// admin 段非空时覆盖 site.Admin.Password（site.yml 仅作首次导入的初始值）。设置/友链保存后调用以实现热更新。
func RebuildSite(st *store.Store, site *tmpl.Site) error {
	settings, err := st.AllSettings()
	if err != nil {
		return err
	}
	links, err := st.ListLinks()
	if err != nil {
		return err
	}

	// 先在锁外解析成局部值，缩小写锁临界区
	var basic siteBasic
	var menubar tmpl.Menubar
	var rightbar tmpl.RightbarConfig
	var footer tmpl.Footer
	var comments tmpl.CommentsConfig
	var article tmpl.ArticleConfig
	var wiki []tmpl.WikiProject
	var friends tmpl.FriendsConfig
	var seo tmpl.SEOConfig
	var inject tmpl.InjectConfig
	var style tmpl.StyleConfig
	var widgets tmpl.WidgetsConfig
	var admin adminSection
	adminSet := false

	unmarshal := func(section string, dst any) {
		if v, ok := settings[section]; ok && v != "" {
			_ = json.Unmarshal([]byte(v), dst)
		}
	}
	unmarshal(SectionSite, &basic)
	unmarshal(SectionMenubar, &menubar)
	unmarshal(SectionRightbar, &rightbar)
	unmarshal(SectionFooter, &footer)
	unmarshal(SectionComments, &comments)
	unmarshal(SectionArticle, &article)
	unmarshal(SectionWiki, &wiki)
	unmarshal(SectionFriends, &friends)
	unmarshal(SectionSEO, &seo)
	unmarshal(SectionInject, &inject)
	unmarshal(SectionStyle, &style)
	unmarshal(SectionWidgets, &widgets)
	if v, ok := settings[SectionAdmin]; ok && v != "" {
		if json.Unmarshal([]byte(v), &admin) == nil {
			adminSet = true
		}
	}

	groups := LinksToGroups(links)

	siteMu.Lock()
	defer siteMu.Unlock()
	site.Title = basic.Title
	site.Subtitle = basic.Subtitle
	site.Description = basic.Description
	site.URL = basic.URL
	site.Avatar = basic.Avatar
	site.AvatarBG = basic.AvatarBG
	site.LogoTitle = basic.LogoTitle
	site.LogoSub = basic.LogoSub
	site.Menubar = menubar
	site.Rightbar = rightbar
	site.Footer = footer
	site.Comments = comments
	site.Article = article
	site.Wiki = wiki
	site.Friends = friends
	site.SEO = seo
	site.Inject = inject
	site.Style = style
	site.Widgets = widgets
	site.Links = groups
	if adminSet {
		site.Admin.Password = admin.Password
	}
	return nil
}

// LinksToGroups 把扁平的 store.Link 列表按 GroupTitle 分组为 tmpl.LinkGroup，保留首次出现的组顺序。
func LinksToGroups(links []*store.Link) []tmpl.LinkGroup {
	idx := map[string]int{}
	var groups []tmpl.LinkGroup
	for _, l := range links {
		i, ok := idx[l.GroupTitle]
		if !ok {
			i = len(groups)
			idx[l.GroupTitle] = i
			groups = append(groups, tmpl.LinkGroup{Title: l.GroupTitle})
		}
		groups[i].Items = append(groups[i].Items, tmpl.LinkItem{
			Name:   l.Name,
			URL:    l.URL,
			Avatar: l.Avatar,
			Desc:   l.Description,
			RSS:    l.RSS,
			Label:  l.Label,
			Color:  l.Color,
		})
	}
	return groups
}

// KnownSections 返回可编辑的分段名（顺序固定，供 GET 返回与校验）。
func KnownSections() []string {
	return []string{
		SectionSite, SectionMenubar, SectionRightbar, SectionFooter,
		SectionComments, SectionArticle, SectionWiki, SectionFriends,
		SectionSEO, SectionInject, SectionDashboard,
		SectionStyle, SectionWidgets, SectionAI,
	}
}

// LoadSectionsJSON 读取全部可编辑段的原始 JSON（缺失段用 null 兑底），供 GET /settings。
func LoadSectionsJSON(st *store.Store) (map[string]json.RawMessage, error) {
	settings, err := st.AllSettings()
	if err != nil {
		return nil, err
	}
	out := map[string]json.RawMessage{}
	for _, sec := range KnownSections() {
		if v, ok := settings[sec]; ok && v != "" {
			out[sec] = json.RawMessage(v)
		} else {
			out[sec] = json.RawMessage("null")
		}
	}
	return out, nil
}

// SaveSection 校验并保存某段配置（用对应类型解析拒绝结构错误，再规范化回存），随后 RebuildSite 热更新。
func SaveSection(st *store.Store, site *tmpl.Site, section string, raw []byte) error {
	var dst any
	switch section {
	case SectionSite:
		dst = &siteBasic{}
	case SectionMenubar:
		dst = &tmpl.Menubar{}
	case SectionRightbar:
		dst = &tmpl.RightbarConfig{}
	case SectionFooter:
		dst = &tmpl.Footer{}
	case SectionComments:
		dst = &tmpl.CommentsConfig{}
	case SectionArticle:
		dst = &tmpl.ArticleConfig{}
	case SectionWiki:
		dst = &[]tmpl.WikiProject{}
	case SectionFriends:
		dst = &tmpl.FriendsConfig{}
	case SectionSEO:
		dst = &tmpl.SEOConfig{}
	case SectionInject:
		dst = &tmpl.InjectConfig{}
	case SectionDashboard:
		dst = &dashboardConfig{}
	case SectionStyle:
		dst = &tmpl.StyleConfig{}
	case SectionWidgets:
		dst = &tmpl.WidgetsConfig{}
	case SectionAI:
		dst = &AIConfig{}
	default:
		return fmt.Errorf("unknown section: %s", section)
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return fmt.Errorf("invalid %s payload: %w", section, err)
	}
	if cc, ok := dst.(*tmpl.CommentsConfig); ok {
		applyCommentDefaults(cc) // 评论段补运行时默认（镜像/徽章/标题不为空）
	}
	// menubar 服务端兖底校验：items 内 title/url 不得为空
	if mb, ok := dst.(*tmpl.Menubar); ok {
		for i, item := range mb.Items {
			if item.Title == "" || item.URL == "" {
				return fmt.Errorf("menubar item[%d]: title 和 url 不得为空", i)
			}
		}
	}
	norm, err := json.Marshal(dst)
	if err != nil {
		return err
	}
	if err := st.PutSetting(section, string(norm)); err != nil {
		return err
	}
	return RebuildSite(st, site)
}

// applyCommentDefaults 评论配置的运行时默认值（与 main.go 启动兑底一致）。
func applyCommentDefaults(cc *tmpl.CommentsConfig) {
	if cc.GravatarMirror == "" {
		cc.GravatarMirror = "https://weavatar.com/avatar"
	}
	if cc.AdminBadge == "" {
		cc.AdminBadge = "博主"
	}
	if cc.Title == "" {
		cc.Title = "评论"
	}
}

// SaveAdminPassword 保存管理密码到 settings 表 admin 段并热更新。
// 空密码合法（=关闭登录 gate，与 site.yml 留空语义一致）。
// 找回手段：sqlite3 data/blog.db "DELETE FROM settings WHERE section='admin'" 后重启，回落 site.yml。
func SaveAdminPassword(st *store.Store, site *tmpl.Site, password string) error {
	b, err := json.Marshal(adminSection{Password: password})
	if err != nil {
		return err
	}
	if err := st.PutSetting(SectionAdmin, string(b)); err != nil {
		return err
	}
	return RebuildSite(st, site)
}

// defaultDashboardConfig 返回仪表盘默认配置（全部部件启用、默认顺序）。
// ID 与 Console 部件注册表（console/src/modules/dashboard/widgets/registry.ts）一致。
func defaultDashboardConfig() dashboardConfig {
	return dashboardConfig{
		Widgets: []dashboardWidget{
			{ID: "stat-posts", Enabled: true},
			{ID: "stat-comments", Enabled: true},
			{ID: "stat-views", Enabled: true},
			{ID: "stat-attachments", Enabled: true},
			{ID: "quick-actions", Enabled: true},
			{ID: "recent-comments", Enabled: true},
			{ID: "trend-chart", Enabled: true},
			{ID: "hourly-chart", Enabled: true},
		},
	}
}
