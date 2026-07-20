package template

import "html/template"

// WikiProject 文库项目卡片，对应 site.yml wiki 段的一项。
// 对齐 Stellar index_wiki 布局的 wiki_card：图标 + 标签 + 标题 + 描述，点击跳到 URL。
type WikiProject struct {
	ID          string   `yaml:"id"`
	Name        string   `yaml:"name"`
	Title       string   `yaml:"title"`
	Description string   `yaml:"description"`
	Icon        string   `yaml:"icon"` // 项目 Logo 图片 URL
	Tags        []string `yaml:"tags"`
	URL         string   `yaml:"url"`  // 首页/文档链接（站内文章、独立页面或外链）
	Sort        int      `yaml:"sort"` // 越大越靠前
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
