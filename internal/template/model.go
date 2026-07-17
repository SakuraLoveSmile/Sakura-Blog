package template

import (
	"html/template"
	"time"
)

// ViewData 贯穿所有模板的渲染上下文，对齐 Stellar layout.ejs 的 page/theme/config 三层。
type ViewData struct {
	Site    *Site
	Page    *Page
	Content template.HTML // 文章正文 HTML（goldmark 渲染后）

	// 首页文章列表（Layout=index 时使用）。
	Posts     []*PostCard // 文章卡片列表
	Paginator *Paginator  // 分页器数据（nil 或 Total<=1 则不渲染）

	// 归档与分类/标签页（Layout=archive/categories/tags 时使用）。
	Archives []*ArchiveYear // 归档页年份分组
	Cats     []*CatItem     // 分类索引页
	TagItems []*TagItem     // 标签索引页

	// Admin 后台专用字段。前台模板不读这些字段，零值即可。
	Admin *AdminData
}

// ArchiveYear 归档页按年份分组的文章列表，对齐 Stellar archive.ejs。
type ArchiveYear struct {
	Year  string
	Posts []*ArchiveItem
}

// ArchiveItem 归档列表单项（time(MM-DD) + 标题 + 链接）。
type ArchiveItem struct {
	Slug  string
	Title string
	Date  time.Time
}

// CatItem 分类索引页单项（名称 + 文章数 badge）。
type CatItem struct {
	Name  string
	Count int
}

// TagItem 标签索引页单项。
type TagItem struct {
	Name  string
	Count int
}

// PostCard 首页文章卡片数据，对齐 Stellar _partial/main/post_list/post_card.ejs。
type PostCard struct {
	Slug    string
	Title   string
	Excerpt string
	Cover   string    // 封面图 URL，空则不渲染 .post-cover
	Date    time.Time // 用于 <time> 标签与格式化
}

// Paginator 分页器数据，对齐 Stellar _partial/main/post_list/paginator.ejs。
type Paginator struct {
	Current int
	Total   int   // 总页数
	Pages   []int // 页码序列 [1..Total]
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
	Cover     string // 封面图 URL（编辑表单回显用）
	Category  string // 分类（编辑表单回显用）
	Tags      string // 标签，逗号分隔（编辑表单回显用）
	ContentMD string
	Status    string
	Date      string // 列表展示用（created_at 或文件 date）
}

// Site 对齐 Stellar theme + config 的站点级配置。
// yaml tag 供 config/site.yml 加载（goccy/go-yaml），snake_case 精确映射。
type Site struct {
	Title       string       `yaml:"title"`
	Subtitle    string       `yaml:"subtitle"`
	Description string       `yaml:"description"`
	Avatar      string       `yaml:"avatar"` // logo 头像 URL
	LogoTitle   string       `yaml:"logo_title"`
	LogoSub     string       `yaml:"logo_sub"`
	Menubar     Menubar      `yaml:"menubar"`
	Footer      Footer       `yaml:"footer"`
	RecentPosts []RecentPost `yaml:"recent_posts"`
	Tags        []string     `yaml:"tags"`
}

// Menubar 对齐 _config.yml menubar.items。
type Menubar struct {
	Columns int        `yaml:"columns"`
	Items   []MenuItem `yaml:"items"`
}

// MenuItem 对齐 menubar 单项。Theme 是 inline color，控制 SVG 的 currentColor。
type MenuItem struct {
	ID    string `yaml:"id"`
	Title string `yaml:"title"`
	URL   string `yaml:"url"`
	Icon  string `yaml:"icon"`  // iconify key，如 solar:documents-bold-duotone
	Theme string `yaml:"theme"` // 高亮色，如 #1BCDFC
}

// Footer 对齐 theme.footer.social。
type Footer struct {
	Social []SocialItem `yaml:"social"`
}

// SocialItem 侧边栏底部社交图标。
// IconKey 非空时走 icon helper（内联 SVG），否则用 Icon 原样输出（如 <img src>）。
type SocialItem struct {
	Title   string `yaml:"title"`
	URL     string `yaml:"url"`
	Icon    string `yaml:"icon"`     // raw HTML，如 <img src="...">
	IconKey string `yaml:"icon_key"` // icons.yml 的 key，如 solar:xxx
}

// RecentPost 近期发布 widget 的一项。
type RecentPost struct {
	Title string `yaml:"title"`
	URL   string `yaml:"url"`
}

// Page 对齐 Hexo page 对象，文章页用。
type Page struct {
	Title    string
	Slug     string
	Layout   string // post / page / index / archive / categories / tags
	Type     string // tech / story
	Lang     string
	MenuID   string // 当前高亮的菜单 id
	Author   string
	Date     string // 原始日期字符串
	Updated  string
	Excerpt  string
	Category string // 分类文章列表页的当前分类名（navbar 显示"分类：xxx"）
	Tag      string // 标签文章列表页的当前标签名
}
