package settings

import (
	"html/template"
	"sort"
	"strconv"
	"strings"
)

// Collection 是 Stellar 1.43.0 的公共列表/网格组件模型。
// 所有页面组件先归一化为这一模型，再由 partial/components/collection.html
// 统一输出，避免每个 widget 自己复制 active、density 和 icon 语义。
type Collection struct {
	Layout    string
	Variant   string
	Density   string
	Columns   int
	ClassName string
	Items     []CollectionItem
}

// CollectionItem 是 Collection 的单项。文本和 URL 保持普通 string，交给
// html/template 在最终上下文中转义；只有 Before/After/RawIcon 是代码生成的
// 受信任片段，使用 template.HTML 原样输出。
type CollectionItem struct {
	Href          string
	Title         string
	Prefix        string
	Description   string
	Meta          string
	ClassName     string
	Theme         string
	Icon          string
	RawIcon       template.HTML
	Active        bool
	Indicator     bool
	IndicatorSet  bool
	ShowTitle     bool
	ShowTitleSet  bool
	LegacyLink    bool
	Depth         int
	GradientID    string
	GradientRef   string
	GradientAngle string
	Before        template.HTML
	After         template.HTML
}

// NewCollectionItem supplies the upstream defaults while keeping callers terse.
func NewCollectionItem(href, title string) CollectionItem {
	return CollectionItem{
		Href:      href,
		Title:     title,
		Indicator: true,
	}
}

// ShouldShowTitle follows collection-item.ejs: title is visible unless explicitly
// disabled. ShowTitleSet is separate from ShowTitle so the Go zero value keeps
// the upstream default while linklist can request icon-only mode.
func (i CollectionItem) ShouldShowTitle() bool {
	return !i.ShowTitleSet || i.ShowTitle
}

// ShouldShowIndicator follows the upstream "indicator !== false" rule.
func (i CollectionItem) ShouldShowIndicator() bool {
	if i.IndicatorSet {
		return i.Indicator
	}
	return true
}

// Style returns the small, generated inline style used by collection items.
// Values are constrained to the fields assembled by the server. Returning the
// trusted CSS type is required because html/template otherwise emits
// ZgotmplZ in a style attribute context.
func (i CollectionItem) Style() template.CSS {
	var b strings.Builder
	if theme := safeCSSValue(i.Theme); theme != "" {
		b.WriteString("--item-theme:")
		b.WriteString(theme)
		b.WriteByte(';')
	}
	if i.Depth > 0 {
		b.WriteString("--ui-item-depth:")
		b.WriteString(strconv.Itoa(i.Depth))
		b.WriteByte(';')
	}
	if id := safeGradientID(i.GradientID); id != "" {
		b.WriteString("--item-grad:url(#")
		b.WriteString(id)
		b.WriteString(");")
	} else if id := safeGradientID(i.GradientRef); id != "" {
		b.WriteString("--item-grad:url(#")
		b.WriteString(id)
		b.WriteString(");")
	}
	return template.CSS(b.String())
}

// safeCSSValue permits the small color/token grammar used by Stellar themes
// while rejecting declaration delimiters and markup. Collection themes may
// come from site configuration, so they must not become arbitrary CSS.
func safeCSSValue(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || strings.ContainsAny(s, ";{}<>\"'\\/*") {
		return ""
	}
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return ""
		}
	}
	return s
}

func (i CollectionItem) SafeClassName() string { return sanitizeClassList(i.ClassName) }

// ClassAttribute keeps the pre-1.43 wiki tree classes as a compatibility alias
// while retaining the public ui-collection selectors used by 1.43.0.
func (i CollectionItem) ClassAttribute() string {
	base := "ui-collection__item"
	if i.LegacyLink {
		legacy := "link"
		if i.Active {
			legacy += " active"
		}
		base = legacy + " " + base
	}
	if className := i.SafeClassName(); className != "" {
		base += " " + className
	}
	if i.Active {
		base += " is-active"
	}
	return base
}

func (i CollectionItem) GradientIDValue() string {
	return safeGradientID(i.GradientID)
}

// Normalized returns the same defaults as collection.ejs.
func (c Collection) Normalized() Collection {
	if c.Layout != "grid" {
		c.Layout = "list"
	}
	switch c.Variant {
	case "nav", "summary", "icon":
	default:
		c.Variant = "nav"
	}
	if c.Density != "compact" {
		c.Density = "auto"
	}
	if c.Columns < 1 {
		c.Columns = 1
	}
	c.ClassName = sanitizeClassList(c.ClassName)
	return c
}

// WidgetFrame is the public wrapper model shared by widget templates.
type WidgetFrame struct {
	ClassName string
	Title     string
	Action    template.HTML
	Body      template.HTML
	Footer    template.HTML
	Attrs     map[string]string
	Reveal    bool
	RevealSet bool
}

func (f WidgetFrame) SafeClassName() string { return sanitizeClassList(f.ClassName) }

type WidgetAttribute struct {
	Name  string
	Value string
}

// Attributes returns deterministic, allow-listed HTML attributes for the
// public widget-frame partial. Names are controlled by the model; values remain
// ordinary strings and are escaped by html/template.
func (f WidgetFrame) Attributes() []WidgetAttribute {
	if len(f.Attrs) == 0 {
		return nil
	}
	allowed := map[string]bool{"id": true, "collapse": true, "data-notebook": true, "data-wiki": true}
	keys := make([]string, 0, len(f.Attrs))
	for key := range f.Attrs {
		if allowed[key] {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	out := make([]WidgetAttribute, 0, len(keys))
	for _, key := range keys {
		out = append(out, WidgetAttribute{Name: key, Value: f.Attrs[key]})
	}
	return out
}

func (f WidgetFrame) ShouldReveal() bool {
	return !f.RevealSet || f.Reveal
}

// MenuCollection converts the fixed menu and generated page menu to the common
// collection model. It deliberately ignores malformed entries like upstream.
func (v *ViewData) MenuCollection() Collection {
	c := Collection{Layout: "list", Variant: "nav", Items: make([]CollectionItem, 0)}
	if v == nil || v.Site == nil {
		return c.Normalized()
	}
	appendMenu := func(items []MenuItem) {
		for _, item := range items {
			if strings.TrimSpace(item.ID) == "" || item.URL == "" || item.Title == "" {
				continue
			}
			entry := NewCollectionItem(item.URL, item.Title)
			entry.Icon = item.Icon
			entry.Theme = item.Theme
			entry.GradientID = "menu-grad-" + item.ID
			entry.GradientAngle = v.Site.Style.Gradient.Angle
			entry.Active = item.ID == v.Page.MenuID
			entry.ClassName = "is-themed"
			if item.Theme == "" {
				entry.ClassName = ""
			}
			c.Items = append(c.Items, entry)
		}
	}
	appendMenu(v.Site.Menubar.Items)
	appendMenu(v.MenuPages)
	return c.Normalized()
}

// Collection converts the data carried by common widgets to a public collection.
// Widgets with a tree/TOC-specific layout keep their specialized adapter.
func (w *WidgetInstance) Collection() Collection {
	c := Collection{Layout: "list", Variant: "nav", Density: "compact"}
	if w == nil {
		return c.Normalized()
	}
	switch w.Layout {
	case "recent":
		for _, p := range recentPosts(w.Data) {
			entry := NewCollectionItem(p.URL, p.Title)
			c.Items = append(c.Items, entry)
		}
	case "related":
		if w.V != nil && w.V.TopicSeries != nil {
			for _, p := range w.V.TopicSeries.Posts {
				if p == nil {
					continue
				}
				entry := NewCollectionItem("/post/"+p.Slug, p.Title)
				entry.Active = p.Active
				entry.IndicatorSet = true
				entry.Indicator = false
				if p.Active {
					entry.Icon = "default:bookmark.active"
				}
				c.Items = append(c.Items, entry)
			}
		}
	case "linklist":
		c.Layout = "grid"
		c.Variant = "icon"
		c.Density = "auto"
		c.Columns = w.LinkListColumns()
		for _, p := range w.LinkListItems() {
			entry := NewCollectionItem(p.URL, p.Title)
			entry.Icon = p.Icon
			entry.ShowTitle = w.LinkListShowText()
			entry.ShowTitleSet = true
			c.Items = append(c.Items, entry)
		}
	}
	return c.Normalized()
}

// Collection flattens the wiki document tree while preserving depth and active
// state. The surrounding widget still supplies the project header and the
// doc-tree-widget surface class.
func (d *WikiTreeWidgetData) Collection() Collection {
	c := Collection{Layout: "list", Variant: "nav", Density: "compact"}
	if d == nil {
		return c.Normalized()
	}
	var walk func([]WikiTreeNode, int)
	walk = func(nodes []WikiTreeNode, depth int) {
		for _, node := range nodes {
			if node.Doc == nil || node.Doc.Title == "" {
				continue
			}
			item := NewCollectionItem("/wiki/"+d.ProjectID+"/"+node.Doc.Slug, node.Doc.Title)
			item.Active = node.Doc.Slug == d.CurrentSlug
			item.GradientRef = "leftbar-active-icon-grad"
			item.Depth = depth
			item.IndicatorSet = true
			item.Indicator = false
			item.LegacyLink = true
			if item.Active {
				item.After = template.HTML(`<svg class="active-icon" viewBox="0 0 24 24" width="16" height="16" aria-hidden="true"><path fill="currentColor" d="M5 2h14a1 1 0 011 1v19.143a.5.5 0 01-.766.424L12 18.03l-7.234 4.536A.5.5 0 014 22.143V3a1 1 0 011-1z"/></svg>`)
			}
			c.Items = append(c.Items, item)
			walk(node.Children, depth+1)
		}
	}
	walk(d.Tree, 0)
	return c.Normalized()
}

func recentPosts(data any) []RecentPost {
	switch items := data.(type) {
	case []*RecentPost:
		out := make([]RecentPost, 0, len(items))
		for _, item := range items {
			if item != nil {
				out = append(out, *item)
			}
		}
		return out
	case []RecentPost:
		return items
	default:
		return nil
	}
}

func safeGradientID(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func sanitizeClassList(s string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(s) {
		if r == ' ' || r == '-' || r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}
