package template

// LinkGroup 友链分组，对应 site.yml links 段的一项。
type LinkGroup struct {
	Title string     `yaml:"title"`
	Items []LinkItem `yaml:"items"`
}

// LinkItem 友链条目（名称 / 跳转 / 头像 / 描述）。
type LinkItem struct {
	Name   string `yaml:"name"`
	URL    string `yaml:"url"`
	Avatar string `yaml:"avatar"`
	Desc   string `yaml:"desc"`
}
