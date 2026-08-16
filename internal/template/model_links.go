package template

// LinkGroup 友链分组，对应 site.yml links 段的一项。
type LinkGroup struct {
	Title string     `yaml:"title" json:"title"`
	Items []LinkItem `yaml:"items" json:"items"`
}

// FriendsConfig 友链动态聚合配置（site.yml friends 段）。
type FriendsConfig struct {
	RefreshInterval  int    `yaml:"refresh_interval" json:"refresh_interval"`   // RSS 抓取刷新间隔（分钟），<=0 用默认 60
	MaxPostsPerFeed  int    `yaml:"posts_per_link" json:"posts_per_link"`       // 每个友链展示的最新文章数，<=0 用默认 20
	CheckInterval    int    `yaml:"check_interval" json:"check_interval"`       // 连通性巡检间隔（分钟），<=0 关闭，缺省 360
	SnapshotInterval int    `yaml:"snapshot_interval" json:"snapshot_interval"` // 截图缓存间隔（分钟），<=0 关闭，缺省 1440
	SnapshotProvider string `yaml:"snapshot_provider" json:"snapshot_provider"` // 截图服务 URL 模板，使用 {url} 占位
}

const DefaultSnapshotProvider = "https://image.thum.io/get/width/1280/crop/720/{url}"

// LinkItem 友链条目（名称 / 跳转 / 头像 / 描述）。
// RSS/Label/Color 为友链动态聚合（/friends 页 ds-friends_and_posts）扩展字段：
//   - RSS：友链的 RSS/Atom 订阅地址，非空时后端 friends 聚合器抓取最新文章。
//   - Label/Color：友链分类标签与配色（16 进制不带 #，对齐 friends_and_posts.js label 渲染）。
type LinkItem struct {
	Name     string `yaml:"name" json:"name"`
	URL      string `yaml:"url" json:"url"`
	Avatar   string `yaml:"avatar" json:"avatar"`
	Snapshot string `yaml:"snapshot" json:"snapshot"`
	Desc     string `yaml:"desc" json:"desc"`
	RSS      string `yaml:"rss" json:"rss"`
	Label    string `yaml:"label" json:"label"`
	Color    string `yaml:"color" json:"color"`
}
