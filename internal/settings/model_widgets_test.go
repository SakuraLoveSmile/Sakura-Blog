package settings

import (
	"strings"
	"testing"
)

// TestWidgetDefAccessors WidgetDef 参数访问器：类型兼容与缺省回退（表驱动）。
func TestWidgetDefAccessors(t *testing.T) {
	def := WidgetDef{
		"layout": "recent",
		"title":  "最近",
		"empty":  "",
		"nilkey": nil,
		"n_int":  7,
		"n_i64":  int64(8),
		"n_u64":  uint64(9),
		"n_f64":  float64(10),
		"n_str":  "11",
		"b_true": true,
	}
	if got := def.Layout(); got != "recent" {
		t.Fatalf("Layout = %q, want recent", got)
	}
	strCases := []struct {
		key, fallback, want string
	}{
		{"title", "x", "最近"},
		{"missing", "fb", "fb"},  // 键缺失 → fallback
		{"nilkey", "fb", "fb"},   // 显式 null → fallback
		{"empty", "fb", ""},      // 显式空串 → 空串（可关闭 rss 按钮）
		{"n_int", "fb", "7"},     // 数值转字符串
		{"b_true", "fb", "true"}, // 布尔转字符串
	}
	for _, c := range strCases {
		if got := def.Str(c.key, c.fallback); got != c.want {
			t.Errorf("Str(%q) = %q, want %q", c.key, got, c.want)
		}
	}
	intCases := []struct {
		key      string
		fallback int
		want     int
	}{
		{"n_int", 0, 7},
		{"n_i64", 0, 8},
		{"n_u64", 0, 9},
		{"n_f64", 0, 10},
		{"n_str", 0, 11},
		{"missing", 42, 42},
		{"title", 42, 42}, // 非数值字符串 → fallback
	}
	for _, c := range intCases {
		if got := def.Int(c.key, c.fallback); got != c.want {
			t.Errorf("Int(%q) = %d, want %d", c.key, got, c.want)
		}
	}
	if got := def.Bool("b_true", false); got != true {
		t.Errorf("Bool(b_true) = false, want true")
	}
	if got := def.Bool("missing", true); got != true {
		t.Errorf("Bool(missing) fallback = false, want true")
	}
	var nilDef WidgetDef
	if got := nilDef.Str("k", "fb"); got != "fb" {
		t.Errorf("nil def Str = %q, want fb", got)
	}
}

// TestWidgetsConfigDefaults 零配置缺省注入：空 widgets 段 = 内置缺省布局（回归底线）。
func TestWidgetsConfigDefaults(t *testing.T) {
	var cfg WidgetsConfig // 零值 = site.yml 未写 widgets 段
	bars := cfg.BarsFor("post")
	wantLeft := []string{"related", "recent", "tagcloud", "popular"}
	wantRight := []string{"welcome", "author", "siteinfo", "latest_comment", "toc"}
	assertSeq(t, "缺省 leftbar", bars.Leftbar, wantLeft)
	assertSeq(t, "缺省 rightbar", bars.Rightbar, wantRight)

	// 任意页面类型都回退同一缺省
	for _, pt := range []string{"index", "archive", "wiki", "unknown-type"} {
		b := cfg.BarsFor(pt)
		assertSeq(t, pt+" leftbar", b.Leftbar, wantLeft)
	}

	// wiki 文档页（Spec 35a）：左栏挂 tree + wiki_related + recent（对齐原版不追加 related/tagcloud/popular），
	// 右栏对齐原版 site_tree.wiki.rightbar（ghrepo, toc）
	wikiDoc := cfg.BarsFor("wiki-doc")
	assertSeq(t, "wiki-doc leftbar", wikiDoc.Leftbar, []string{"tree", "wiki_related", "recent"})
	assertSeq(t, "wiki-doc rightbar", wikiDoc.Rightbar, []string{"ghrepo", "toc"})

	// 缺省库组件全部可解析出 layout
	for _, name := range append(append([]string{}, wantLeft...), wantRight...) {
		if cfg.Lookup(name).Layout() == "" {
			t.Errorf("缺省库组件 %q 无 layout", name)
		}
	}
}

// TestWidgetsConfigPagesFallback 页面类型回退链：pages[类型] → pages[index] → 内置缺省。
func TestWidgetsConfigPagesFallback(t *testing.T) {
	cfg := WidgetsConfig{
		Pages: map[string]PageBars{
			"post":  {Leftbar: []string{"recent"}, Rightbar: []string{"toc"}},
			"index": {Leftbar: []string{"welcome"}, Rightbar: []string{"tagcloud"}},
		},
	}
	assertSeq(t, "post leftbar", cfg.BarsFor("post").Leftbar, []string{"recent"})
	// 未配置的类型回退 index 布局（spec 技术决策）
	assertSeq(t, "wiki→index leftbar", cfg.BarsFor("wiki").Leftbar, []string{"welcome"})
	assertSeq(t, "archive→index rightbar", cfg.BarsFor("archive").Rightbar, []string{"tagcloud"})
}

// TestWidgetsConfigLookupMerge 用户 library 与缺省库浅合并：同名键覆盖、未覆盖键保留、自定义组件直通。
func TestWidgetsConfigLookupMerge(t *testing.T) {
	cfg := WidgetsConfig{
		Library: map[string]WidgetDef{
			"recent": {"limit": 10},                           // 覆盖缺省 limit=5，layout 继承缺省
			"mycard": {"layout": "markdown", "content": "hi"}, // 自定义组件
		},
	}
	recent := cfg.Lookup("recent")
	if got := recent.Layout(); got != "recent" {
		t.Errorf("merge 后 layout = %q, want recent（继承缺省）", got)
	}
	if got := recent.Int("limit", 0); got != 10 {
		t.Errorf("merge 后 limit = %d, want 10（用户覆盖）", got)
	}
	if got := recent.Str("rss", ""); got != "/rss.xml" {
		t.Errorf("merge 后 rss = %q, want /rss.xml（缺省保留）", got)
	}
	if got := cfg.Lookup("mycard").Str("content", ""); got != "hi" {
		t.Errorf("自定义组件 content = %q, want hi", got)
	}
}

// TestBuildBar 序列 → 实例：layout 缺失的名字跳过，V/Name/Params 正确绑定。
func TestBuildBar(t *testing.T) {
	cfg := WidgetsConfig{
		Library: map[string]WidgetDef{
			"broken": {"title": "无 layout"}, // 无 layout → 跳过
		},
	}
	v := &ViewData{}
	out := cfg.BuildBar([]string{"recent", "broken", "not-exist", "toc"}, v)
	if len(out) != 2 {
		t.Fatalf("BuildBar 实例数 = %d, want 2（broken/not-exist 跳过）", len(out))
	}
	if out[0].Name != "recent" || out[0].Layout != "recent" || out[0].V != v {
		t.Errorf("实例0 = %+v，Name/Layout/V 绑定错误", out[0])
	}
	if out[1].Layout != "toc" {
		t.Errorf("实例1 layout = %q, want toc", out[1].Layout)
	}
}

// TestPageTypeOf Page.Layout → 页面类型映射（未知回退 index）。
func TestPageTypeOf(t *testing.T) {
	cases := map[string]string{
		"post": "post", "index": "index", "wiki": "wiki", "moments": "moments",
		"topics": "topic", "404": "index", "500": "index", "post-password": "index", "": "index",
	}
	for layout, want := range cases {
		if got := PageTypeOf(layout); got != want {
			t.Errorf("PageTypeOf(%q) = %q, want %q", layout, got, want)
		}
	}
}

// TestBuildTagCloudDefault 缺省参数行为 = 改造前 buildTagCloud：数量降序 top N、字号 12–22 线性。
func TestBuildTagCloudDefault(t *testing.T) {
	counts := map[string]int{"a": 1, "b": 5, "c": 3, "d": 5}
	items := BuildTagCloud(counts, DefaultTagCloudOptions())
	if len(items) != 4 {
		t.Fatalf("条目数 = %d, want 4", len(items))
	}
	// 数量降序，同量按名升序：b(5) d(5) c(3) a(1)
	wantOrder := []string{"b", "d", "c", "a"}
	for i, w := range wantOrder {
		if items[i].Name != w {
			t.Fatalf("顺序[%d] = %s, want %s", i, items[i].Name, w)
		}
	}
	if items[0].Size != 22 || items[3].Size != 12 {
		t.Errorf("字号映射：max=%d(want 22) min=%d(want 12)", items[0].Size, items[3].Size)
	}
	if items[0].Color != "" {
		t.Errorf("缺省不着色，got %q", items[0].Color)
	}
	// amount 截断保留热门
	top2 := BuildTagCloud(counts, TagCloudOptions{Amount: 2, MinFont: 12, MaxFont: 22, OrderBy: "count", Order: -1})
	if len(top2) != 2 || top2[0].Name != "b" || top2[1].Name != "d" {
		t.Errorf("amount=2 应保留 b,d，got %v,%v", top2[0].Name, top2[1].Name)
	}
}

// TestTagCloudLevels 热度层级分档（tagcloud.design 移植）：少量标签保证至少一档 xl；
// 20 个标签按排名百分比 8/25/55/85 分档 xl/lg/md/sm/xs；层级不随 orderby 变化（表达热度）。
func TestTagCloudLevels(t *testing.T) {
	// 4 个标签：xl 界 ceil(4*0.08)=1 → 榜首 xl，其余落 md/sm
	small := BuildTagCloud(map[string]int{"a": 1, "b": 5, "c": 3, "d": 5}, DefaultTagCloudOptions())
	if small[0].Level != "xl" {
		t.Errorf("榜首层级 = %q, want xl", small[0].Level)
	}
	for _, it := range small {
		if it.Level == "" {
			t.Errorf("标签 %s 缺层级", it.Name)
		}
	}
	// 20 个标签：xl=ceil(1.6)=2 / lg 到 ceil(5)=5 / md 到 ceil(11)=11 / sm 到 ceil(17)=17 / 其余 xs
	counts := make(map[string]int, 20)
	for i := 0; i < 20; i++ {
		counts[string(rune('a'+i))] = i + 1
	}
	items := BuildTagCloud(counts, DefaultTagCloudOptions())
	wantLevels := map[string]int{"xl": 2, "lg": 3, "md": 6, "sm": 6, "xs": 3}
	got := map[string]int{}
	for _, it := range items {
		got[it.Level]++
	}
	for lv, n := range wantLevels {
		if got[lv] != n {
			t.Errorf("层级 %s 数量 = %d, want %d（全部：%v）", lv, got[lv], n, got)
		}
	}
	// orderby=name 重排后层级不变（按名查热度档）
	byName := BuildTagCloud(counts, TagCloudOptions{Amount: 20, MinFont: 12, MaxFont: 22, OrderBy: "name", Order: 1})
	levelOf := map[string]string{}
	for _, it := range items {
		levelOf[it.Name] = it.Level
	}
	for _, it := range byName {
		if it.Level != levelOf[it.Name] {
			t.Errorf("orderby=name 后 %s 层级 = %q, want %q", it.Name, it.Level, levelOf[it.Name])
		}
	}
}

// TestBuildTagCloudDecor 云朵散落装饰：垂直偏移按标签名哈希确定性生成，范围合法且具备多样性。
func TestBuildTagCloudDecor(t *testing.T) {
	counts := map[string]int{"a": 1, "b": 5, "c": 3, "d": 5, "e": 2, "f": 1, "g": 4, "h": 2}
	first := BuildTagCloud(counts, DefaultTagCloudOptions())
	second := BuildTagCloud(counts, DefaultTagCloudOptions())
	if len(first) != len(second) {
		t.Fatalf("两次构建条目数不一致：%d vs %d", len(first), len(second))
	}
	seen := map[int]bool{}
	for i, it := range first {
		if it.Dy < -4 || it.Dy > 4 {
			t.Errorf("%s Dy=%d 超出 -4–4", it.Name, it.Dy)
		}
		// 确定性：同名条目两次构建偏移一致
		if it.Name != second[i].Name || it.Dy != second[i].Dy {
			t.Errorf("%s 偏移不确定：%d vs %d", it.Name, it.Dy, second[i].Dy)
		}
		seen[it.Dy] = true
	}
	if len(seen) < 2 {
		t.Errorf("偏移缺乏多样性：%v", seen)
	}
}

// TestBuildTagCloudParams orderby/order/color 参数逐项生效。
func TestBuildTagCloudParams(t *testing.T) {
	counts := map[string]int{"a": 1, "b": 5, "c": 3}

	// orderby=name 升序
	byName := BuildTagCloud(counts, TagCloudOptions{Amount: 10, MinFont: 12, MaxFont: 22, OrderBy: "name", Order: 1})
	if byName[0].Name != "a" || byName[2].Name != "c" {
		t.Errorf("orderby=name asc 顺序错误: %s..%s", byName[0].Name, byName[2].Name)
	}
	// orderby=count 升序
	asc := BuildTagCloud(counts, TagCloudOptions{Amount: 10, MinFont: 12, MaxFont: 22, OrderBy: "count", Order: 1})
	if asc[0].Name != "a" || asc[2].Name != "b" {
		t.Errorf("orderby=count asc 顺序错误: %s..%s", asc[0].Name, asc[2].Name)
	}
	// 渐变着色：最小计数 = start_color，最大计数 = end_color
	colored := BuildTagCloud(counts, TagCloudOptions{
		Amount: 10, MinFont: 12, MaxFont: 22, OrderBy: "count", Order: -1,
		Color: true, StartColor: "#000000", EndColor: "#ffffff",
	})
	for _, it := range colored {
		switch it.Name {
		case "b":
			if it.Color != "#ffffff" {
				t.Errorf("max 计数色 = %q, want #ffffff", it.Color)
			}
		case "a":
			if it.Color != "#000000" {
				t.Errorf("min 计数色 = %q, want #000000", it.Color)
			}
		}
	}
	// 非法颜色 → 空（降级不破版）
	bad := BuildTagCloud(counts, TagCloudOptions{Amount: 10, MinFont: 12, MaxFont: 22, Color: true, StartColor: "red-ish", EndColor: "#fff"})
	if bad[0].Color != "" {
		t.Errorf("非法颜色应返回空，got %q", bad[0].Color)
	}
}

// TestTagCloudOptionsFrom widget 参数 → 构建选项（缺省与 DefaultTagCloudOptions 一致）。
func TestTagCloudOptionsFrom(t *testing.T) {
	if got := TagCloudOptionsFrom(WidgetDef{"layout": "tagcloud"}); got != DefaultTagCloudOptions() {
		t.Errorf("空参数选项 = %+v, want %+v", got, DefaultTagCloudOptions())
	}
	opt := TagCloudOptionsFrom(WidgetDef{
		"amount": 5, "min_font": 10, "max_font": 30, "orderby": "name", "order": 1,
		"color": true, "start_color": "#123", "end_color": "#456", "show_count": true,
	})
	want := TagCloudOptions{Amount: 5, MinFont: 10, MaxFont: 30, OrderBy: "name", Order: 1,
		Color: true, StartColor: "#123", EndColor: "#456", ShowCount: true}
	if opt != want {
		t.Errorf("参数解析 = %+v, want %+v", opt, want)
	}
}

// TestExtractTOCWithOptions toc 参数：min/max_depth 层级过滤 + list_number 层级编号（hexo 形状）。
func TestExtractTOCWithOptions(t *testing.T) {
	html := `<h2 id="a">A</h2><h3 id="b">B</h3><h3 id="c">C</h3><h2 id="d">D</h2><h4 id="e">E</h4>`

	// 深度过滤：只收 h2
	onlyH2 := string(ExtractTOCWith(html, TOCOptions{MinDepth: 2, MaxDepth: 2}))
	if strings.Contains(onlyH2, `#b`) || strings.Contains(onlyH2, `#e`) {
		t.Errorf("max_depth=2 不应含 h3/h4：%s", onlyH2)
	}
	if !strings.Contains(onlyH2, `#a`) || !strings.Contains(onlyH2, `#d`) {
		t.Errorf("max_depth=2 应含两个 h2：%s", onlyH2)
	}

	// 编号：h2=1. / 其下 h3=1.1. 1.2. / 第二个 h2=2.
	numbered := string(ExtractTOCWith(html, TOCOptions{ListNumber: true, MinDepth: 1, MaxDepth: 6}))
	for _, want := range []string{
		`<span class="toc-number">1.</span><span class="toc-text">A</span>`,
		`<span class="toc-number">1.1.</span><span class="toc-text">B</span>`,
		`<span class="toc-number">1.2.</span><span class="toc-text">C</span>`,
		`<span class="toc-number">2.</span><span class="toc-text">D</span>`,
		`<span class="toc-number">2.1.</span><span class="toc-text">E</span>`,
	} {
		if !strings.Contains(numbered, want) {
			t.Errorf("编号缺失 %s\n完整输出：%s", want, numbered)
		}
	}

	// 缺省参数 = 无编号（与 ExtractTOC 一致，回归底线）
	if got, legacy := string(ExtractTOCWith(html, DefaultTOCOptions())), string(ExtractTOC(html)); got != legacy {
		t.Errorf("缺省参数输出应与 ExtractTOC 一致")
	}
	if strings.Contains(string(ExtractTOC(html)), "toc-number") {
		t.Errorf("缺省不应输出 toc-number")
	}
}

// assertSeq 断言字符串序列相等。
func assertSeq(t *testing.T, label string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s 长度 = %d(%v), want %d(%v)", label, len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s[%d] = %q, want %q", label, i, got[i], want[i])
		}
	}
}
