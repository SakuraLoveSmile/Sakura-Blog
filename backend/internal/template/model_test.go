package template

import (
	"html/template"
	"strings"
	"testing"
	"time"

	"blog/internal/store"
)

// TestNewPostCard 以 store.Post 为输入，断言文章卡片视图模型字段输出
// （首页/分类标签列表/相关文章共用的核心构建路径）。
func TestNewPostCard(t *testing.T) {
	date := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	colors := map[string]string{"技术": "1BCDFC"}
	icons := map[string]string{"技术": "solar:folder-bold"}

	tests := []struct {
		name       string
		post       *store.Post
		colors     map[string]string
		wantStyle  template.CSS
		wantIcon   string
		wantPoster *PosterCard
	}{
		{
			name: "常规字段直传+分类配色命中",
			post: &store.Post{
				Slug: "hello", Title: "你好", Excerpt: "摘要", Cover: "/img/c.png",
				CreatedAt: date, Pinned: true, Category: "技术",
			},
			colors:    colors,
			wantStyle: "--text-p2:#1BCDFC;--theme-block:#1BCDFC20",
			wantIcon:  "solar:folder-bold",
		},
		{
			name:      "分类未配置配色时 style 为空",
			post:      &store.Post{Slug: "a", Category: "随笔", CreatedAt: date},
			colors:    colors,
			wantStyle: "",
			wantIcon:  "default:category",
		},
		{
			name:     "合法海报 JSON 解码为 PosterCard",
			post:     &store.Post{Slug: "p", CreatedAt: date, Poster: `{"headline":"主标题","topic":"专栏","caption":"副题","color":"#fff"}`},
			colors:   nil,
			wantIcon: "default:category",
			wantPoster: &PosterCard{
				Headline: "主标题", Topic: "专栏", Caption: "副题", Color: "#fff",
			},
		},
		{
			name:     "海报缺 headline 时不渲染 photo 卡",
			post:     &store.Post{Slug: "p2", CreatedAt: date, Poster: `{"topic":"专栏"}`},
			colors:   nil,
			wantIcon: "default:category",
		},
		{
			name:     "海报 JSON 非法时静默降级",
			post:     &store.Post{Slug: "p3", CreatedAt: date, Poster: `{bad json`},
			colors:   nil,
			wantIcon: "default:category",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			card := NewPostCard(tt.post, tt.colors, icons)
			if card.Slug != tt.post.Slug || card.Title != tt.post.Title ||
				card.Excerpt != tt.post.Excerpt || card.Cover != tt.post.Cover ||
				card.Category != tt.post.Category || card.Pinned != tt.post.Pinned {
				t.Errorf("字段直传不一致: got %+v", card)
			}
			if !card.Date.Equal(tt.post.CreatedAt) {
				t.Errorf("Date = %v, want %v", card.Date, tt.post.CreatedAt)
			}
			if card.CategoryStyle != tt.wantStyle {
				t.Errorf("CategoryStyle = %q, want %q", card.CategoryStyle, tt.wantStyle)
			}
			if card.CategoryIcon != tt.wantIcon {
				t.Errorf("CategoryIcon = %q, want %q", card.CategoryIcon, tt.wantIcon)
			}
			if tt.wantPoster == nil {
				if card.Poster != nil {
					t.Errorf("Poster = %+v, want nil", card.Poster)
				}
			} else if card.Poster == nil || *card.Poster != *tt.wantPoster {
				t.Errorf("Poster = %+v, want %+v", card.Poster, tt.wantPoster)
			}
		})
	}
}

// TestCategoryColorStyle 对齐原版 category_color helper 的补 # 前缀与底色后缀规则。
func TestCategoryColorStyle(t *testing.T) {
	tests := []struct {
		name   string
		cat    string
		colors map[string]string
		want   template.CSS
	}{
		{"分类为空", "", map[string]string{"Go": "#fff"}, ""},
		{"配色表为空", "Go", nil, ""},
		{"分类未命中", "Go", map[string]string{"Rust": "#fff"}, ""},
		{"无前缀色值补 #，6 位底色后缀 20", "Go", map[string]string{"Go": "1BCDFC"}, "--text-p2:#1BCDFC;--theme-block:#1BCDFC20"},
		{"3 位色值底色后缀 2", "Go", map[string]string{"Go": "#fff"}, "--text-p2:#fff;--theme-block:#fff2"},
		{"6 位带前缀色值底色后缀 20", "Go", map[string]string{"Go": "#ffffff"}, "--text-p2:#ffffff;--theme-block:#ffffff20"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CategoryColorStyle(tt.cat, tt.colors); got != tt.want {
				t.Errorf("CategoryColorStyle(%q) = %q, want %q", tt.cat, got, tt.want)
			}
		})
	}
}

// TestCategoryIconKey 分类图标 key 解析：命中返回配置值，未命中/空分类回退 default:category。
func TestCategoryIconKey(t *testing.T) {
	tests := []struct {
		name  string
		cat   string
		icons map[string]string
		want  string
	}{
		{"分类为空回退默认", "", map[string]string{"Go": "solar:code-bold"}, "default:category"},
		{"图标表为空回退默认", "Go", nil, "default:category"},
		{"分类未命中回退默认", "Go", map[string]string{"Rust": "solar:code-bold"}, "default:category"},
		{"命中值为 icons.yml key", "Go", map[string]string{"Go": "solar:code-bold"}, "solar:code-bold"},
		{"命中值为图片 URL", "Go", map[string]string{"Go": "https://example.com/go.svg"}, "https://example.com/go.svg"},
		{"命中值为空串回退默认", "Go", map[string]string{"Go": ""}, "default:category"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CategoryIconKey(tt.cat, tt.icons); got != tt.want {
				t.Errorf("CategoryIconKey(%q) = %q, want %q", tt.cat, got, tt.want)
			}
		})
	}
}

// TestLinkListConfig 对齐原版 linklist.ejs 的列数/对齐/文字显隐三条派生规则。
func TestLinkListConfig(t *testing.T) {
	items := func(n int) []LinkListItem {
		s := make([]LinkListItem, n)
		return s
	}
	tests := []struct {
		name      string
		cfg       LinkListConfig
		wantCols  int
		wantAlign string
		wantText  bool
	}{
		{"未配置列数按链接数", LinkListConfig{Items: items(3)}, 3, "center", false},
		{"显式列数优先", LinkListConfig{Columns: 2, Items: items(5)}, 2, "center", true},
		{"单列多链接左对齐", LinkListConfig{Columns: 1, Items: items(3)}, 1, "left", true},
		{"单列单链接居中", LinkListConfig{Columns: 1, Items: items(1)}, 1, "center", true},
		{"空列表", LinkListConfig{}, 0, "center", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.EffectiveColumns(); got != tt.wantCols {
				t.Errorf("EffectiveColumns() = %d, want %d", got, tt.wantCols)
			}
			if got := tt.cfg.AlignClass(); got != tt.wantAlign {
				t.Errorf("AlignClass() = %q, want %q", got, tt.wantAlign)
			}
			if got := tt.cfg.ShowText(); got != tt.wantText {
				t.Errorf("ShowText() = %v, want %v", got, tt.wantText)
			}
		})
	}
}

// TestFooterSitemapColumns 对齐原版 footer.ejs：6 组→3 栏，7-8 组→4 栏，其余 min(len,5)。
func TestFooterSitemapColumns(t *testing.T) {
	tests := []struct {
		groups int
		want   int
	}{
		{0, 0}, {1, 1}, {5, 5}, {6, 3}, {7, 4}, {8, 4}, {9, 5}, {12, 5},
	}
	for _, tt := range tests {
		f := &Footer{Sitemap: make([]SitemapGroup, tt.groups)}
		if got := f.SitemapColumns(); got != tt.want {
			t.Errorf("SitemapColumns(%d 组) = %d, want %d", tt.groups, got, tt.want)
		}
	}
}

// TestTagCloudFontSize 标签云字号映射（12–22px）。
func TestTagCloudFontSize(t *testing.T) {
	tests := []struct {
		name            string
		count, min, max int
		want            int
	}{
		{"count 非正取下限", 0, 1, 10, 12},
		{"max<=min 取中间值", 5, 5, 5, 17},
		{"最小计数取下限", 1, 1, 11, 12},
		{"最大计数取上限", 11, 1, 11, 22},
		{"中间计数线性插值", 6, 1, 11, 17},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TagCloudFontSize(tt.count, tt.min, tt.max); got != tt.want {
				t.Errorf("TagCloudFontSize(%d,%d,%d) = %d, want %d", tt.count, tt.min, tt.max, got, tt.want)
			}
		})
	}
}

// TestWikiProjectDisplayTitle 标题回退链：Title → Name → ID。
func TestWikiProjectDisplayTitle(t *testing.T) {
	tests := []struct {
		name string
		w    WikiProject
		want string
	}{
		{"优先 Title", WikiProject{ID: "id", Name: "name", Title: "title"}, "title"},
		{"次选 Name", WikiProject{ID: "id", Name: "name"}, "name"},
		{"兜底 ID", WikiProject{ID: "id"}, "id"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.w.DisplayTitle(); got != tt.want {
				t.Errorf("DisplayTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestReadingStats(t *testing.T) {
	words, minutes := ReadingStats(`<p>你好世界</p><p>Go blog post-1</p>`)
	if words != 7 || minutes != 1 {
		t.Fatalf("ReadingStats mixed content = (%d, %d), want (7, 1)", words, minutes)
	}
	words, minutes = ReadingStats(strings.Repeat("中", 301))
	if words != 301 || minutes != 2 {
		t.Fatalf("ReadingStats long content = (%d, %d), want (301, 2)", words, minutes)
	}
	words, minutes = ReadingStats("")
	if words != 0 || minutes != 1 {
		t.Fatalf("ReadingStats empty content = (%d, %d), want (0, 1)", words, minutes)
	}
}

func TestParseTags(t *testing.T) {
	got := ParseTags("Go, Go，后端,, 前端")
	want := []string{"Go", "后端", "前端"}
	if len(got) != len(want) {
		t.Fatalf("ParseTags length = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("ParseTags[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestNewPostCardIncludesTags(t *testing.T) {
	card := NewPostCard(&store.Post{Tags: "Go, Go，后端,前端,设计,测试"}, nil, nil)
	if got := strings.Join(card.Tags, "|"); got != "Go|后端|前端|设计|测试" {
		t.Fatalf("PostCard.Tags = %q, want five ordered unique tags", got)
	}
}

// TestExtractTOC 从 goldmark 渲染 HTML 提取嵌套目录（子级 ol.toc-child 嵌在父级 li 内）。
func TestExtractTOC(t *testing.T) {
	tests := []struct {
		name string
		html string
		want string
	}{
		{"无标题返回空串", `<p>正文</p>`, ""},
		{
			"同级标题平铺",
			`<h2 id="a">甲</h2><p>x</p><h2 id="b">乙</h2>`,
			`<ol class="toc">` +
				`<li class="toc-item toc-level-2"><a class="toc-link" href="#a"><span class="toc-text">甲</span></a></li>` +
				`<li class="toc-item toc-level-2"><a class="toc-link" href="#b"><span class="toc-text">乙</span></a></li>` +
				`</ol>`,
		},
		{
			"子级嵌套后回到父级",
			`<h2 id="a">甲</h2><h3 id="b">乙</h3><h2 id="c">丙</h2>`,
			`<ol class="toc">` +
				`<li class="toc-item toc-level-2"><a class="toc-link" href="#a"><span class="toc-text">甲</span></a>` +
				`<ol class="toc-child">` +
				`<li class="toc-item toc-level-3"><a class="toc-link" href="#b"><span class="toc-text">乙</span></a></li>` +
				`</ol></li>` +
				`<li class="toc-item toc-level-2"><a class="toc-link" href="#c"><span class="toc-text">丙</span></a></li>` +
				`</ol>`,
		},
		{
			"剥离行内标签并还原实体",
			`<h2 id="x"><code>foo</code> &amp; bar</h2>`,
			`<ol class="toc">` +
				`<li class="toc-item toc-level-2"><a class="toc-link" href="#x"><span class="toc-text">foo &amp; bar</span></a></li>` +
				`</ol>`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := string(ExtractTOC(tt.html)); got != tt.want {
				t.Errorf("ExtractTOC() =\n%s\nwant\n%s", got, tt.want)
			}
		})
	}
}

// TestExpireAfterDays 过期提醒阈值：未配置（<=0）回退缺省 180 天（Spec 32）。
func TestExpireAfterDays(t *testing.T) {
	if got := (ArticleConfig{}).ExpireAfterDays(); got != 180 {
		t.Errorf("缺省阈值 = %d, want 180", got)
	}
	if got := (ArticleConfig{ExpireDays: 30}).ExpireAfterDays(); got != 30 {
		t.Errorf("配置阈值 = %d, want 30", got)
	}
}

// TestExpiredDays 过期天数计算：updated 优先、created 回退、未超阈值/解析失败返回 0。
func TestExpiredDays(t *testing.T) {
	now := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name             string
		created, updated string
		threshold, want  int
	}{
		{"超阈值用 updated", "2024-01-01", "2025-07-01", 180, 365},
		{"updated 空回退 created", "2025-07-01", "", 180, 365},
		{"未超阈值不提醒", "2024-01-01", "2026-06-01", 180, 0},
		{"带时分秒格式", "", "2025-07-01 12:00:00", 180, 365},
		{"日期为空返回 0", "", "", 180, 0},
		{"解析失败返回 0", "not-a-date", "", 180, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExpiredDays(tt.created, tt.updated, tt.threshold, now); got != tt.want {
				t.Errorf("ExpiredDays(%q, %q, %d) = %d, want %d", tt.created, tt.updated, tt.threshold, got, tt.want)
			}
		})
	}
}

// TestResolveEditPage 「编辑本文」链接推导：前缀替换 + commits API 规则（对齐原版 contributors.ejs）。
func TestResolveEditPage(t *testing.T) {
	mapping := map[string]string{
		"posts/": "https://github.com/alice/blog/blob/main/content/posts/",
	}
	got := ResolveEditPage("posts/hello.md", mapping)
	if got == nil {
		t.Fatal("命中前缀应返回非 nil")
	}
	if want := "https://github.com/alice/blog/blob/main/content/posts/hello.md"; got.EditURL != want {
		t.Errorf("EditURL = %s, want %s", got.EditURL, want)
	}
	if want := "https://api.github.com/repos/alice/blog/commits?path=content/posts/hello.md"; got.APIURL != want {
		t.Errorf("APIURL = %s, want %s", got.APIURL, want)
	}

	// 非 github/main 结构：保留编辑按钮但不推导 API
	got = ResolveEditPage("posts/a.md", map[string]string{"posts/": "https://gitee.com/alice/blog/blob/master/posts/"})
	if got == nil || got.APIURL != "" {
		t.Errorf("非 github/main 结构应仅保留编辑链接: %+v", got)
	}

	// 无匹配前缀 / 空输入
	if ResolveEditPage("pages/about.md", mapping) != nil {
		t.Error("未命中前缀应返回 nil")
	}
	if ResolveEditPage("", mapping) != nil || ResolveEditPage("posts/a.md", nil) != nil {
		t.Error("空输入应返回 nil")
	}

	// 多前缀按最长前缀匹配（结果确定性）
	multi := map[string]string{
		"posts/":      "https://github.com/alice/blog/blob/main/short/",
		"posts/deep/": "https://github.com/alice/blog/blob/main/deep/",
	}
	got = ResolveEditPage("posts/deep/x.md", multi)
	if got == nil || got.EditURL != "https://github.com/alice/blog/blob/main/deep/x.md" {
		t.Errorf("多前缀应取最长匹配: %+v", got)
	}
}

// TestMetaDescriptionPriority 断言 Spec 42 的 meta description 输出优先级：
// 文章级 Page.MetaDescription → 站点级 seo.default_description → 现状回退（Page.Excerpt → Site.Description）。
func TestMetaDescriptionPriority(t *testing.T) {
	tests := []struct {
		name string
		data *ViewData
		want string
	}{
		{
			name: "文章级最优先，覆盖站点级与摘要",
			data: &ViewData{
				Site: &Site{Description: "站点描述", SEO: SEOConfig{DefaultDescription: "站点默认desc"}},
				Page: &Page{MetaDescription: "文章级desc", Excerpt: "文章摘要"},
			},
			want: "文章级desc",
		},
		{
			name: "文章级未配置回退站点级默认",
			data: &ViewData{
				Site: &Site{Description: "站点描述", SEO: SEOConfig{DefaultDescription: "站点默认desc"}},
				Page: &Page{Excerpt: "文章摘要"},
			},
			want: "站点默认desc",
		},
		{
			name: "站点级未配置回退现状 Excerpt",
			data: &ViewData{
				Site: &Site{Description: "站点描述"},
				Page: &Page{Excerpt: "文章摘要"},
			},
			want: "文章摘要",
		},
		{
			name: "全部未配置回退 Site.Description",
			data: &ViewData{
				Site: &Site{Description: "站点描述"},
				Page: &Page{},
			},
			want: "站点描述",
		},
		{
			name: "Page 为 nil 不 panic",
			data: &ViewData{Site: &Site{Description: "站点描述"}},
			want: "站点描述",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.data.MetaDescription(); got != tt.want {
				t.Errorf("MetaDescription() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestMetaKeywordsPriority 断言 keywords 优先级：文章级 → 站点级默认；均空返回空串（模板不输出该 meta）。
func TestMetaKeywordsPriority(t *testing.T) {
	tests := []struct {
		name string
		data *ViewData
		want string
	}{
		{
			name: "文章级覆盖站点级",
			data: &ViewData{
				Site: &Site{SEO: SEOConfig{DefaultKeywords: "go,blog"}},
				Page: &Page{MetaKeywords: "gin,sqlite"},
			},
			want: "gin,sqlite",
		},
		{
			name: "文章级空回退站点级",
			data: &ViewData{
				Site: &Site{SEO: SEOConfig{DefaultKeywords: "go,blog"}},
				Page: &Page{},
			},
			want: "go,blog",
		},
		{
			name: "均空返回空串",
			data: &ViewData{Site: &Site{}, Page: &Page{}},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.data.MetaKeywords(); got != tt.want {
				t.Errorf("MetaKeywords() = %q, want %q", got, tt.want)
			}
		})
	}
}
