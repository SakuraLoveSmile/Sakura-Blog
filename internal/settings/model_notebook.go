package settings

import (
	"html/template"
	"strings"

	"blog/internal/store"
)

// 笔记本体系视图模型（Spec 35b）：前台三层页面 /notebooks、/notebooks/:name、/note/:id。
// 数据来自 store.Notebook / store.Note，页面与 leftbar tagtree widget 共用。

// NotebookView 笔记本列表页卡片（Layout=notebooks 时使用）：嵌入 store.Notebook，附笔记数量。
type NotebookView struct {
	store.Notebook
	NoteCount int `json:"note_count"`
}

// NoteView 笔记详情页视图（Layout=note 时使用）：嵌入 store.Note。
// ContentHTML 为已渲染正文（优先 note.ContentHTML，为空时由 handler 渲染 Content）；
// NotebookName/NotebookTitle 为所属笔记本上下文（面包屑）；PrevNote/NextNote 为同笔记本上下篇。
type NoteView struct {
	store.Note
	ContentHTML   template.HTML `json:"content_html"`
	NotebookName  string        `json:"notebook_name"`
	NotebookTitle string        `json:"notebook_title"`
	PrevNote      *NoteNav      `json:"prev_note,omitempty"`
	NextNote      *NoteNav      `json:"next_note,omitempty"`
}

// NoteNav 笔记上下篇导航项（同笔记本内按 updated_at DESC 相邻的笔记）。
type NoteNav struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
}

// TagTreeWidgetData tagtree widget 装配数据（Spec 35b）。
// assembleWidgets 仅在本页注入（其余页面 Data 留空，模板不输出，不进侧栏快照）；
// 每个标签链接 /notebooks/{name}?tag={tag}，ActiveTag 为当前过滤标签（高亮）。
type TagTreeWidgetData struct {
	Tags         []TagCount `json:"tags"`
	ActiveTag    string     `json:"active_tag"`
	NotebookName string     `json:"notebook_name"`
}

// TagCount 标签聚合项（名称 + 笔记数）。
type TagCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// NoteTags 拆分笔记标签 CSV（逗号分隔，去空白与空段）。
func NoteTags(s string) []string {
	var out []string
	for _, t := range strings.Split(s, ",") {
		if t = strings.TrimSpace(t); t != "" {
			out = append(out, t)
		}
	}
	return out
}
