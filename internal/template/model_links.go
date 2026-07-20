package template

// LinkGroup 友链分组，对应 site.yml links 段的一项。
type LinkGroup struct {
	Title string     `yaml:"title" json:"title"`
	Items []LinkItem `yaml:"items" json:"items"`
}

// FriendsConfig 友链动态聚合配置（site.yml friends 段）。
type FriendsConfig struct {
	RefreshInterval int `yaml:"refresh_interval" json:"refresh_interval"` // RSS 抓取刷新间隔（分钟），<=0 用默认 60
}

// LinkItem 友链条目（名称 / 跳转 / 头像 / 描述）。
// RSS/Label/Color 为友链动态聚合（/friends 页 ds-friends_and_posts）扩展字段：
//   - RSS：友链的 RSS/Atom 订阅地址，非空时后端 friends 聚合器抓取最新文章。
//   - Label/Color：友链分类标签与配色（16 进制不带 #，对齐 friends_and_posts.js label 渲染）。
type LinkItem struct {
	Name   string `yaml:"name" json:"name"`
	URL    string `yaml:"url" json:"url"`
	Avatar string `yaml:"avatar" json:"avatar"`
	Desc   string `yaml:"desc" json:"desc"`
	RSS    string `yaml:"rss" json:"rss"`
	Label  string `yaml:"label" json:"label"`
	Color  string `yaml:"color" json:"color"`
}
