package settings

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strconv"
	"strings"
)

// ---------- Spec 33: widgets 配置化编排 ----------
//
// 对齐原版 Stellar 的 _data/widgets.yml（组件库）+ _config.yml site_tree（页面类型 → 左右栏序列）。
// 本项目配置单文件惯例：两段合并进 site.yml 的 widgets 段（library/pages）。
// 缺省值 = 改造前硬编码布局，零配置时渲染结果与现状一致（回归底线）。

// WidgetsConfig site.yml widgets 段（缺省为空 = 使用内置缺省布局）。
type WidgetsConfig struct {
	// Library 组件库：组件名 → {layout, ...参数}，对照原版 _data/widgets.yml。
	// 用户配置按组件名整键覆盖默认库同名组件的单个参数（浅合并）。
	Library map[string]WidgetDef `yaml:"library" json:"library"`
	// Pages 页面类型 → 左右栏组件名序列，对照原版 site_tree 的 leftbar/rightbar。
	// 页面类型枚举：index/post/page/archive/categories/tags/topic/wiki/friends/notes/moments，
	// 未配置的类型回退 index 布局（index 也未配置则回退内置缺省）。
	Pages map[string]PageBars `yaml:"pages" json:"pages"`
}

// PageBars 单个页面类型的左右栏组件名序列。
type PageBars struct {
	Leftbar  []string `yaml:"leftbar" json:"leftbar"`
	Rightbar []string `yaml:"rightbar" json:"rightbar"`
}

// WidgetDef 组件定义：layout + 任意参数（对齐原版 widgets.yml 单组件的自由键值形状）。
// yaml 解析为 map，模板经 WidgetInstance 的 Str/Int/Bool 访问器取参。
type WidgetDef map[string]any

// Layout 组件布局名（分派到 partial/widgets/<layout>.html），未定义返回空。
func (d WidgetDef) Layout() string {
	if d == nil {
		return ""
	}
	if s, ok := d["layout"].(string); ok {
		return s
	}
	return ""
}

// Str 字符串参数：键缺失或值为 null 时返回 fallback；显式配置空串返回空串（可用于关闭 rss 按钮等）。
func (d WidgetDef) Str(key, fallback string) string {
	if d == nil {
		return fallback
	}
	raw, ok := d[key]
	if !ok || raw == nil {
		return fallback
	}
	switch v := raw.(type) {
	case string:
		return v
	case bool:
		return strconv.FormatBool(v)
	case int, int64, uint64, float64:
		return fmt.Sprintf("%v", v)
	}
	return fallback
}

// Int 整数参数（兼容 yaml 解出的多种数值类型），缺失返回 fallback。
func (d WidgetDef) Int(key string, fallback int) int {
	if d == nil {
		return fallback
	}
	switch v := d[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case uint64:
		return int(v)
	case float64:
		return int(v)
	case string:
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

// Bool 布尔参数，缺失或类型不符返回 fallback。
func (d WidgetDef) Bool(key string, fallback bool) bool {
	if d == nil {
		return fallback
	}
	if v, ok := d[key].(bool); ok {
		return v
	}
	return fallback
}

// merge 浅合并：other 的键覆盖 d 的同名键，返回新 map（不改入参）。
func (d WidgetDef) merge(other WidgetDef) WidgetDef {
	if len(other) == 0 {
		return d
	}
	if len(d) == 0 {
		return other
	}
	out := make(WidgetDef, len(d)+len(other))
	for k, v := range d {
		out[k] = v
	}
	for k, v := range other {
		out[k] = v
	}
	return out
}

// WidgetInstance 单个 widget 的渲染视图模型（模板 partial/widgets/dispatch 的上下文）。
type WidgetInstance struct {
	Name   string    // library 中的组件名
	Layout string    // 布局名（分派模板）
	Params WidgetDef // 合并后的参数
	Data   any       // 按 layout 装配的数据（recent/popular/tagcloud/latest_comment 用）
	V      *ViewData // 根渲染上下文（Site/TOC/TopicSeries 等直读）
}

// AuthorWidgetData author widget 装配数据（Spec 38 多作者体系）。
// assembleWidgets 从 Page.Author 查找 Site.Authors 命中时设置，模板据此显示对应作者卡片。
type AuthorWidgetData struct {
	ID      string       // 作者 ID（用于 /author/{id}/ 链接）
	Author  AuthorDef    // 作者定义（name/avatar/description）
	Path    string       // 作者页路径（/author/{id}/）
	Authors []AuthorLink // 文章的完整作者列表；Author 保留第一位兼容字段
}

// Str/Int/Bool 参数访问便捷透传（模板里写 .Str "title" "默认"）。
func (w *WidgetInstance) Str(key, fallback string) string  { return w.Params.Str(key, fallback) }
func (w *WidgetInstance) Int(key string, fallback int) int { return w.Params.Int(key, fallback) }
func (w *WidgetInstance) Bool(key string, fallback bool) bool {
	return w.Params.Bool(key, fallback)
}

// AuthorData 返回 author widget 的装配数据（nil 表示走站长兜底卡片）。
func (w *WidgetInstance) AuthorData() *AuthorWidgetData {
	if d, ok := w.Data.(*AuthorWidgetData); ok {
		return d
	}
	return nil
}

// ---------- linklist（Spec 50 独立 widget）----------

// LinkListItems 从 widget 参数 items 解析链接列表（yaml 解出的 []any → []LinkListItem）。
// 键缺失/类型不符返回 nil（模板据此不渲染，零配置降级）。
func (w *WidgetInstance) LinkListItems() []LinkListItem {
	raw, ok := w.Params["items"]
	if !ok || raw == nil {
		return nil
	}
	list, ok := raw.([]any)
	if !ok {
		return nil
	}
	items := make([]LinkListItem, 0, len(list))
	for _, it := range list {
		m, ok := it.(map[string]any)
		if !ok {
			continue
		}
		items = append(items, LinkListItem{
			Title: widgetStr(m["title"]),
			URL:   widgetStr(m["url"]),
			Icon:  widgetStr(m["icon"]),
		})
	}
	return items
}

// LinkListColumns 返回 linklist widget 列数（columns 参数，缺省 1，<1 归一为 1）。
// 列数规则（对齐原版 linklist.ejs 的 columns 语义，区别于 welcome 实例的"按链接数"）：
// 1=竖排列表（图标+文字），2=两列网格（图标+文字），>=3=多列网格（仅图标）。
func (w *WidgetInstance) LinkListColumns() int {
	n := w.Int("columns", 1)
	if n < 1 {
		return 1
	}
	return n
}

// LinkListShowText columns<=2 时才显示文字（>2 纯图标，title 做 tooltip）。
func (w *WidgetInstance) LinkListShowText() bool {
	return w.LinkListColumns() <= 2
}

// LinkListAlignClass 对齐原版：columns==1 且链接数>1 时左对齐（left），否则居中（center）。
func (w *WidgetInstance) LinkListAlignClass() string {
	if w.LinkListColumns() == 1 && len(w.LinkListItems()) > 1 {
		return "left"
	}
	return "center"
}

// widgetStr map 值转字符串（yaml 解出的字段可能是 nil/string/数值）。
func widgetStr(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", x)
	}
}

// DefaultWidgetLibrary 内置组件库（缺省参数 = 改造前硬编码行为）。
// welcome/siteinfo/popular/latest_comment 为本项目自创组件（技术决策：保留注册进 library）；
// 其余名称与参数形状对照原版 _data/widgets.yml。
func DefaultWidgetLibrary() map[string]WidgetDef {
	return map[string]WidgetDef{
		"welcome":        {"layout": "markdown", "title": "欢迎，旅行者："},
		"author":         {"layout": "author"},
		"siteinfo":       {"layout": "siteinfo"},
		"latest_comment": {"layout": "latest_comment", "title": "最新评论", "limit": 5},
		"toc":            {"layout": "toc", "list_number": false, "min_depth": 1, "max_depth": 6, "collapse": "false"},
		"related":        {"layout": "related"},
		"recent":         {"layout": "recent", "limit": 5, "rss": "/rss.xml"},
		"tagcloud":       {"layout": "tagcloud", "title": "标签", "amount": 20, "min_font": 12, "max_font": 22, "orderby": "count", "order": -1, "show_count": false, "color": false},
		"popular":        {"layout": "popular", "limit": 5},
		// GitHub 系与通用 timeline：需用户在 library 配置 repo/username/api 等参数后才有输出
		"ghrepo":   {"layout": "ghrepo"},
		"ghuser":   {"layout": "ghuser", "avatar": true},
		"ghissues": {"layout": "ghissues", "limit": 3},
		"timeline": {"layout": "timeline", "title": "近期动态"},
		// Spec 35a：tree 为 wiki 文档页项目文档树（数据由 assembleWidgets 仅在本页注入，
		// 其余页面 Data 为空、模板不输出；缺省仅在 wiki-doc 页面类型左栏启用）
		"tree": {"layout": "tree"},
		// 同 tag 项目推荐（wiki 文档页左栏）：数据由 assembleWidgets 从 site.yml wiki 段
		// 内存过滤（与当前项目 tag 交集），title 缺省"更多项目"，有共同标签时装配期覆盖为"更多：{标签名}"
		"wiki_related": {"layout": "wiki_related", "title": "更多项目", "limit": 5},
		// Spec 35b：tagtree 为笔记本标签树（数据由 assembleWidgets 仅在本页注入，
		// 其余页面 Data 为空、模板不输出；缺省仅在 notebooks/notebook/note 页面类型左栏启用）
		"tagtree": {"layout": "tagtree", "expand_all": false, "expand_active": true, "show_tagcon": true},
	}
}

// DefaultPageBars 内置缺省左右栏序列（= 改造前硬编码顺序；搜索框固定在左栏 widgets 首位，
// 对齐原版 index_leftbar.ejs——search 不属于可配置序列）。
func DefaultPageBars() PageBars {
	return PageBars{
		Leftbar:  []string{"related", "recent", "tagcloud", "popular"},
		Rightbar: []string{"welcome", "author", "siteinfo", "latest_comment", "toc"},
	}
}

// Lookup 组件名 → 合并后的定义：默认库为基底，用户 library 同名键浅合并覆盖；
// 默认库不存在的名字直接用用户定义（自定义组件）。
func (w WidgetsConfig) Lookup(name string) WidgetDef {
	def := DefaultWidgetLibrary()[name]
	return def.merge(w.Library[name])
}

// BarsFor 页面类型 → 左右栏序列：pages[pageType] → pages[index] → wiki-doc 缺省 → 笔记本缺省 → 内置缺省。
// wiki-doc（Spec 35a）零配置时左栏首项挂 tree（项目文档树）；
// notebooks/notebook/note（Spec 35b）零配置时左栏首项挂 tagtree（笔记本标签树）；
// 其余沿用 index 缺省；用户显式配置 pages 时尊重配置（tree/tagtree 组件名可在任意页面类型的 leftbar 中启用）。
func (w WidgetsConfig) BarsFor(pageType string) PageBars {
	if b, ok := w.Pages[pageType]; ok {
		return b
	}
	if b, ok := w.Pages["index"]; ok {
		return b
	}
	def := DefaultPageBars()
	if pageType == "wiki-doc" {
		// 左栏：tree（项目文档树）+ wiki_related（同 tag 项目推荐）+ recent（当前项目最近更新），
		// 对齐原版不追加博客维度的 related/tagcloud/popular；
		// 右栏：对齐原版 site_tree.wiki.rightbar（ghrepo, toc）——ghrepo 仅项目配置 repo 时输出，否则右栏只剩 toc。
		return PageBars{
			Leftbar:  []string{"tree", "wiki_related", "recent"},
			Rightbar: []string{"ghrepo", "toc"},
		}
	}
	if pageType == "notebooks" || pageType == "notebook" || pageType == "note" {
		return PageBars{
			Leftbar:  append([]string{"tagtree"}, def.Leftbar...),
			Rightbar: def.Rightbar,
		}
	}
	return def
}

// BuildBar 组件名序列 → 实例列表；layout 为空的名字跳过（对齐原版 layoutWidgets 忽略无效项）。
// 如果 v.PageWidgetOverrides 非 nil 且含当前组件名，则页面级参数与全局 WidgetDef 浅合并（页面级覆盖全局）。
// Data 由调用方（web.go assembleWidgets）按 layout 装配。
func (w WidgetsConfig) BuildBar(names []string, v *ViewData) []*WidgetInstance {
	var out []*WidgetInstance
	for _, name := range names {
		def := w.Lookup(name)
		// 页面级参数覆盖（仅 Layout=page 自定义侧栏时设置 PageWidgetOverrides，其余页面为 nil）
		if v != nil && v.PageWidgetOverrides != nil {
			if over, ok := v.PageWidgetOverrides[name]; ok {
				def = def.merge(over)
			}
		}
		layout := def.Layout()
		if layout == "" {
			continue
		}
		out = append(out, &WidgetInstance{Name: name, Layout: layout, Params: def, V: v})
	}
	return out
}

// PageTypeOf Page.Layout → widgets 页面类型（未知布局回退 index，
// post-password/404/500 等异常页与列表页同用 index 布局）。
func PageTypeOf(layout string) string {
	switch layout {
	case "post", "page", "archive", "categories", "tags", "wiki", "friends", "notes", "moments", "index",
		"notebooks", "notebook", "note":
		// Spec 35b：笔记本三层页面为独立类型（BarsFor 零配置时左栏挂 tagtree）
		return layout
	case "topics":
		return "topic"
	case "author":
		return "author"
	case "wiki-doc":
		// Spec 35a：wiki 文档页独立类型（BarsFor 零配置时左栏挂 tree）
		return "wiki-doc"
	default:
		return "index"
	}
}

// ---------- tagcloud ----------

// TagCloudOptions 标签云构建参数（对照原版 widgets.yml tagcloud 段 / hexo tagcloud helper）。
type TagCloudOptions struct {
	Amount     int    // 最多显示条数（<=0 不限）
	MinFont    int    // 最小字号 px
	MaxFont    int    // 最大字号 px
	OrderBy    string // name / count / random
	Order      int    // 1 升序，-1 降序（random 时忽略）
	Color      bool   // 按计数插值着色
	StartColor string // 插值起点色（#rgb / #rrggbb）
	EndColor   string // 插值终点色
	ShowCount  bool   // 是否显示计数（模板消费 TagItem.Count）
}

// DefaultTagCloudOptions 改造前硬编码行为：按数量降序 top 20、字号 12–22 线性映射、无着色。
func DefaultTagCloudOptions() TagCloudOptions {
	return TagCloudOptions{Amount: 20, MinFont: 12, MaxFont: 22, OrderBy: "count", Order: -1}
}

// TagCloudOptionsFrom widget 参数 → 构建选项（缺省值与 DefaultTagCloudOptions 一致）。
func TagCloudOptionsFrom(d WidgetDef) TagCloudOptions {
	return TagCloudOptions{
		Amount:     d.Int("amount", 20),
		MinFont:    d.Int("min_font", 12),
		MaxFont:    d.Int("max_font", 22),
		OrderBy:    d.Str("orderby", "count"),
		Order:      d.Int("order", -1),
		Color:      d.Bool("color", false),
		StartColor: d.Str("start_color", ""),
		EndColor:   d.Str("end_color", ""),
		ShowCount:  d.Bool("show_count", false),
	}
}

// BuildTagCloud 标签计数 → 标签云条目：先按数量降序截取 Amount 个（保持「热门标签优先入选」
// 语义与原实现一致），再按 OrderBy/Order 重排展示顺序，最后映射字号与渐变色。
func BuildTagCloud(tagCounts map[string]int, opt TagCloudOptions) []*TagItem {
	items := make([]*TagItem, 0, len(tagCounts))
	for name, n := range tagCounts {
		items = append(items, &TagItem{Name: name, Count: n})
	}
	// 入选排序：数量降序（同量按名升序），与改造前一致
	sort.Slice(items, func(i, j int) bool {
		if items[i].Count != items[j].Count {
			return items[i].Count > items[j].Count
		}
		return items[i].Name < items[j].Name
	})
	if opt.Amount > 0 && len(items) > opt.Amount {
		items = items[:opt.Amount]
	}
	if len(items) == 0 {
		return items
	}
	// 热度层级（tagcloud.design 移植）：此刻 items 为 count DESC，按入选排名百分比分档，
	// 与后续展示重排无关（层级表达热度，不随 orderby 变化）。
	tagCloudLevels(items)
	minC, maxC := items[len(items)-1].Count, items[0].Count

	// 展示排序
	switch opt.OrderBy {
	case "name":
		sort.Slice(items, func(i, j int) bool {
			if opt.Order < 0 {
				return items[i].Name > items[j].Name
			}
			return items[i].Name < items[j].Name
		})
	case "random":
		rand.Shuffle(len(items), func(i, j int) { items[i], items[j] = items[j], items[i] })
	default: // count
		if opt.Order > 0 {
			sort.Slice(items, func(i, j int) bool {
				if items[i].Count != items[j].Count {
					return items[i].Count < items[j].Count
				}
				return items[i].Name < items[j].Name
			})
		}
	}

	for _, item := range items {
		item.Size = tagCloudScale(item.Count, minC, maxC, opt.MinFont, opt.MaxFont)
		if opt.Color {
			item.Color = interpolateColor(opt.StartColor, opt.EndColor, ratioOf(item.Count, minC, maxC))
		}
		// 散落装饰：垂直偏移按标签名哈希确定性生成（同名跨渲染稳定），
		// 与计数无关，纯视觉打散行列感；模板经 --tc-dy 消费。
		item.Dy = int(tagHash(item.Name)>>3%9) - 4
	}
	return items
}

// tagHash 标签名 → 稳定哈希（djb2），用于标签云色系号与散落偏移的确定性分配。
func tagHash(s string) uint32 {
	h := uint32(5381)
	for i := 0; i < len(s); i++ {
		h = h*33 + uint32(s[i])
	}
	return h
}

// tagCloudLevels 热度层级映射（对照 tagcloud.design 五档字号）：
// 条目须已按 count DESC 排序，按排名百分比分档 —— 前 8% xl / 25% lg / 55% md / 85% sm / 其余 xs，
// 少量标签时也保证至少一档 xl（热门标签突出，对齐设计稿视觉层级）。
func tagCloudLevels(items []*TagItem) {
	n := len(items)
	if n == 0 {
		return
	}
	bounds := []float64{0.08, 0.25, 0.55, 0.85}
	levels := []string{"xl", "lg", "md", "sm", "xs"}
	// 少量标签（<=5）时保证每个标签有独立层级，避免全部落到同一档
	if n <= 5 {
		for i := range items {
			items[i].Level = levels[i]
		}
		return
	}
	for i, it := range items {
		li := len(levels) - 1
		for b, bound := range bounds {
			if i < int(math.Ceil(float64(n)*bound)) {
				li = b
				break
			}
		}
		it.Level = levels[li]
	}
}

// ratioOf count 在 [min,max] 区间的归一化比例（max<=min 时取中值 0.5）。
func ratioOf(count, min, max int) float64 {
	if max <= min {
		return 0.5
	}
	return float64(count-min) / float64(max-min)
}

// tagCloudScale 按计数线性映射字号（参数化版 TagCloudFontSize，缺省 12–22 与改造前一致）。
func tagCloudScale(count, min, max, minPx, maxPx int) int {
	if count <= 0 {
		return minPx
	}
	if max <= min {
		return (minPx + maxPx) / 2
	}
	return minPx + int(ratioOf(count, min, max)*float64(maxPx-minPx))
}

// interpolateColor 在 start/end 两个十六进制色（#rgb / #rrggbb）间按 t∈[0,1] 线性插值，
// 任一颜色非法时返回空（模板不输出 color style，降级不破版）。
func interpolateColor(start, end string, t float64) string {
	r1, g1, b1, ok1 := parseHexColor(start)
	r2, g2, b2, ok2 := parseHexColor(end)
	if !ok1 || !ok2 {
		return ""
	}
	lerp := func(a, b int) int { return a + int(t*float64(b-a)) }
	return fmt.Sprintf("#%02x%02x%02x", lerp(r1, r2), lerp(g1, g2), lerp(b1, b2))
}

// parseHexColor 解析 #rgb / #rrggbb 为 RGB 分量。
func parseHexColor(s string) (r, g, b int, ok bool) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "#")
	switch len(s) {
	case 3:
		s = string([]byte{s[0], s[0], s[1], s[1], s[2], s[2]})
	case 6:
	default:
		return 0, 0, 0, false
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return 0, 0, 0, false
	}
	return int(v >> 16), int(v >> 8 & 0xff), int(v & 0xff), true
}

// ---------- 站点统计 ----------

// SiteStats 站点统计 widget 数据（文章/标签/评论/总阅读量）。
type SiteStats struct {
	Posts    int
	Tags     int
	Comments int
	Views    int64
}

// TagCloudFontSize 按标签文章数映射字号（12–22px），count 相同时取中间值。
func TagCloudFontSize(count, min, max int) int {
	return tagCloudScale(count, min, max, 12, 22)
}
