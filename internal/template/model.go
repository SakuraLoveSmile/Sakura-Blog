package template

import "html/template"

// ViewData 贯穿所有模板的渲染上下文，对齐 Stellar layout.ejs 的 page/theme/config 三层。
type ViewData struct {
	Site    *Site
	Page    *Page
	Content template.HTML // 文章正文 HTML（goldmark 渲染后）

	// Admin 后台专用字段。前台模板不读这些字段，零值即可。
	Admin *AdminData
}

// AdminData 后台模板渲染数据。
type AdminData struct {
	Posts     []*AdminPostRow // 列表页
	EditPost  *AdminPostRow   // 编辑页（新建时为空对象）
	SaveError string          // 保存失败提示
	IsEdit    bool            // true=编辑现有，false=新建
}

// AdminPostRow 后台列表/编辑表单用的文章行数据。
type AdminPostRow struct {
	Slug      string
	Title     string
	Excerpt   string
	ContentMD string
	Status    string
	Date      string // 列表展示用（created_at 或文件 date）
}

// Site 对齐 Stellar theme + config 的站点级配置。
type Site struct {
	Title       string
	Subtitle    string
	Description string
	Avatar      string // logo 头像 URL
	LogoTitle   string
	LogoSub     string
	Menubar     Menubar
	Footer      Footer
	RecentPosts []RecentPost
	Tags        []string
}

// Menubar 对齐 _config.yml menubar.items。
type Menubar struct {
	Columns int
	Items   []MenuItem
}

// MenuItem 对齐 menubar 单项。Theme 是 inline color，控制 SVG 的 currentColor。
type MenuItem struct {
	ID    string
	Title string
	URL   string
	Icon  string // iconify key，如 solar:documents-bold-duotone
	Theme string // 高亮色，如 #1BCDFC
}

// Footer 对齐 theme.footer.social。
type Footer struct {
	Social []SocialItem
}

// SocialItem 侧边栏底部社交图标。
// IconKey 非空时走 icon helper（内联 SVG），否则用 Icon 原样输出（如 <img src>）。
type SocialItem struct {
	Title   string
	URL     string
	Icon    string // raw HTML，如 <img src="...">
	IconKey string // icons.yml 的 key，如 solar:xxx
}

// RecentPost 近期发布 widget 的一项。
type RecentPost struct {
	Title string
	URL   string
}

// Page 对齐 Hexo page 对象，文章页用。
type Page struct {
	Title   string
	Slug    string
	Layout  string // post / page / index
	Type    string // tech / story
	Lang    string
	MenuID  string // 当前高亮的菜单 id
	Author  string
	Date    string // 原始日期字符串
	Updated string
	Excerpt string
}
