package settings

import (
	"html/template"
	"net/url"
	"sort"
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
	// Root/Children are the 1.43.0 hierarchical representation. Tags remains
	// populated for API and old template compatibility.
	Root         *TagTreeNode   `json:"root,omitempty"`
	Children     []*TagTreeNode `json:"children,omitempty"`
	ExpandAll    bool           `json:"expand_all"`
	ExpandActive bool           `json:"expand_active"`
	ShowTagcon   bool           `json:"show_tagcon"`
}

// TagCount 标签聚合项（名称 + 笔记数）。
type TagCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// TagTreeNode is one path segment in a notebook tag tree.
type TagTreeNode struct {
	Root     bool           `json:"root,omitempty"`
	Children []*TagTreeNode `json:"children,omitempty"`
	ID       string         `json:"id"`   // full tag path, e.g. go/web
	Part     string         `json:"part"` // display segment, e.g. web
	Depth    int            `json:"depth"`
	Path     string         `json:"path"` // notebook URL with ?tag=...
	Active   bool           `json:"active"`
	Count    int            `json:"count"`
	Tagcon   string         `json:"tagcon,omitempty"`
	Expanded bool           `json:"expanded"`
}

func (n *TagTreeNode) IsLeaf() bool { return n == nil || len(n.Children) == 0 }

func (d *TagTreeWidgetData) HasTag(id string) bool {
	if d == nil || d.Root == nil {
		return false
	}
	id = strings.Trim(strings.TrimSpace(id), "/")
	if id == "" {
		return false
	}
	var walk func(*TagTreeNode) bool
	walk = func(node *TagTreeNode) bool {
		if node.ID == id {
			return true
		}
		for _, child := range node.Children {
			if walk(child) {
				return true
			}
		}
		return false
	}
	return walk(d.Root)
}

// BuildTagTree keeps flat tags while adding slash-delimited hierarchy. Counts
// on parents include the exact tag and all descendants, matching filtering
// semantics where selecting a parent includes every descendant note.
func BuildTagTree(tags []TagCount, notebookName, activeTag string, expandAll, expandActive, showTagcon bool) *TagTreeWidgetData {
	root := &TagTreeNode{Root: true, ID: "", Part: "", Depth: 0, Path: "/notebooks/" + notebookName}
	nodes := map[string]*TagTreeNode{"": root}
	for _, tag := range tags {
		id := strings.Trim(strings.TrimSpace(tag.Name), "/")
		if id == "" {
			continue
		}
		parts := strings.Split(id, "/")
		parentID := ""
		for depth, part := range parts {
			if part == "" {
				continue
			}
			childID := part
			if parentID != "" {
				childID = parentID + "/" + part
			}
			if _, ok := nodes[childID]; !ok {
				node := &TagTreeNode{ID: childID, Part: part, Depth: depth + 1}
				node.Path = "/notebooks/" + notebookName + "?tag=" + url.QueryEscape(childID)
				nodes[childID] = node
				parent := nodes[parentID]
				parent.Children = append(parent.Children, node)
			}
			parentID = childID
		}
		if node := nodes[id]; node != nil {
			node.Count += tag.Count
		}
	}
	// Propagate exact counts up the tree. Sorting makes output deterministic
	// across DB/map iteration order and preserves a stable sidebar.
	var visit func(*TagTreeNode)
	visit = func(node *TagTreeNode) {
		for _, child := range node.Children {
			visit(child)
			node.Count += child.Count
		}
		if node.ID != "" {
			node.Active = activeTag == node.ID
			node.Expanded = expandAll || (expandActive && (node.Active || strings.HasPrefix(activeTag, node.ID+"/")))
		}
	}
	visit(root)
	sortTagTree(root)
	return &TagTreeWidgetData{
		Tags: tags, ActiveTag: activeTag, NotebookName: notebookName,
		Root: root, Children: root.Children, ExpandAll: expandAll,
		ExpandActive: expandActive, ShowTagcon: showTagcon,
	}
}

// ApplyTagcons attaches the existing notebook.tagcons icon mapping to every
// matching path/segment. The empty key is the final fallback, as in upstream.
func (d *TagTreeWidgetData) ApplyTagcons(tagcons map[string]string) {
	if d == nil || d.Root == nil {
		return
	}
	var walk func(*TagTreeNode)
	walk = func(node *TagTreeNode) {
		if !node.Root {
			for _, key := range []string{node.ID, node.Part, strings.ToLower(node.Part), ""} {
				if icon := strings.TrimSpace(tagcons[key]); icon != "" {
					node.Tagcon = icon
					break
				}
			}
		}
		for _, child := range node.Children {
			walk(child)
		}
	}
	walk(d.Root)
}

func sortTagTree(node *TagTreeNode) {
	if node == nil {
		return
	}
	sort.SliceStable(node.Children, func(i, j int) bool { return node.Children[i].ID < node.Children[j].ID })
	for _, child := range node.Children {
		sortTagTree(child)
	}
}

// TagMatches reports whether an exact tag or any descendant belongs to a
// selected parent. It is shared by routes and tests.
func TagMatches(tag, selected string) bool {
	tag = strings.Trim(strings.TrimSpace(tag), "/")
	selected = strings.Trim(strings.TrimSpace(selected), "/")
	return selected != "" && (tag == selected || strings.HasPrefix(tag, selected+"/"))
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
