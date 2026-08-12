package template

import "html/template"

// WikiProject 文库项目卡片，对应 site.yml wiki 段的一项。
// 对齐 Stellar index_wiki 布局的 wiki_card：图标 + 标签 + 标题 + 描述，点击跳到 URL。
type WikiProject struct {
	ID          string   `yaml:"id" json:"id"`
	Name        string   `yaml:"name" json:"name"`
	Title       string   `yaml:"title" json:"title"`
	Description string   `yaml:"description" json:"description"`
	Icon        string   `yaml:"icon" json:"icon"` // 项目 Logo 图片 URL
	Tags        []string `yaml:"tags" json:"tags"`
	URL         string   `yaml:"url" json:"url"`   // 首页/文档链接（站内文章、独立页面或外链）
	Sort        int      `yaml:"sort" json:"sort"` // 越大越靠前
	Repo        string   `yaml:"repo" json:"repo"` // GitHub 仓库（owner/name 格式），非空时 wiki 文档页右栏渲染 ghrepo 卡片
	Hero        *HeroConfig `yaml:"hero,omitempty" json:"hero,omitempty"` // 项目级 hero banner（wiki 文档页页首渲染）
	// HasDocs 运行时标记：是否有已发布的本地文档（请求时由 handler 填充，不参与配置解析/JSON 输出）。
	HasDocs bool `yaml:"-" json:"-"`
}

// DisplayTitle 优先 Title，其次 Name，最后 ID。
func (w *WikiProject) DisplayTitle() string {
	if w.Title != "" {
		return w.Title
	}
	if w.Name != "" {
		return w.Name
	}
	return w.ID
}

// WikiDocView wiki 文档页当前文档视图（Layout=wiki-doc 时使用，Spec 35a）。
// ContentHTML 为已渲染正文，template.HTML 类型直接输出不转义（对齐 ViewData.Content）。
type WikiDocView struct {
	Slug        string
	Title       string
	ContentHTML template.HTML
	Updated     string // 更新日期（Spec 53 Part C：对齐原版 dateinfo.ejs wiki 分支，仅显示更新于）
}

// WikiDocNav wiki 文档页上下篇导航项（同 project 内按 doc_order 相邻的已发布文档）。
type WikiDocNav struct {
	Slug  string
	Title string
}

// WikiTreeNode wiki 文档树节点（leftbar tree widget 用，递归 ≤3 层，对齐 store.BuildDocTree 限制）。
type WikiTreeNode struct {
	Doc      *WikiDocView
	Children []WikiTreeNode
}

// WikiTreeWidgetData tree widget 装配数据（assembleWidgets 注入，仅 wiki 文档页；
// 其余页面 Data 留空，模板不输出，不进侧栏快照）。
type WikiTreeWidgetData struct {
	ProjectID   string
	ProjectName string
	ProjectIcon string // 项目 Logo 图片 URL（WikiProject.Icon），header 展示项目主题感
	CurrentSlug string // 当前文档 slug（树高亮）
	Tree        []WikiTreeNode
}

// Note 笔记(手记)条目，对应 site.yml notes 段的一项。
// 对齐 xaoxuu.com/notes/ 的手记风格：时间流内联展示短内容（非文章卡片跳转）。
type Note struct {
	Title   string   `yaml:"title"`   // 可选标题
	Date    string   `yaml:"date"`    // ISO 日期字符串，如 2024-07-20（字符串排序即时间序）
	Tags    []string `yaml:"tags"`
	Content string   `yaml:"content"` // Markdown 短内容，时间流内联渲染（手记正文）
	Pin     bool     `yaml:"pin"`     // 置顶

	// ContentHTML 为 Content 经 goldmark 渲染后的 HTML，请求时由 handler 填充（不从配置解析）。
	ContentHTML template.HTML `yaml:"-"`
}
