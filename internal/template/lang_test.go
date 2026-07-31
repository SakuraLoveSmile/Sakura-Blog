package template

import "testing"

// TestLangAllKeysHaveValues 确保语言表中每个 key 都映射到非空字符串。
func TestLangAllKeysHaveValues(t *testing.T) {
	for k, v := range zhCN {
		if v == "" {
			t.Errorf("zhCN[%q] 为空字符串", k)
		}
	}
}

// TestLangBasic 基础查询：命中返回对应文案。
func TestLangBasic(t *testing.T) {
	got := Lang("btn.recent_publish")
	want := "近期发布"
	if got != want {
		t.Errorf("Lang(btn.recent_publish) = %q, want %q", got, want)
	}
}

// TestLangWithArgs 带参数替换（fmt.Sprintf 式）。
func TestLangWithArgs(t *testing.T) {
	got := Lang("comment.count", 5)
	want := "5 条评论"
	if got != want {
		t.Errorf("Lang(comment.count, 5) = %q, want %q", got, want)
	}
}

// TestLangMiss 未命中返回 key 本身（便于排查遗漏）。
func TestLangMiss(t *testing.T) {
	got := Lang("nonexistent.key")
	if got != "nonexistent.key" {
		t.Errorf("Lang(nonexistent.key) = %q, want key itself", got)
	}
}

// TestLangNavPrefix 导航前缀带 %s 占位符。
func TestLangNavPrefix(t *testing.T) {
	got := Lang("nav.category", "技术")
	want := "分类：技术"
	if got != want {
		t.Errorf("Lang(nav.category, 技术) = %q, want %q", got, want)
	}
}

// TestLangDateSuffix 时间后缀组完整性。
func TestLangDateSuffix(t *testing.T) {
	cases := map[string]string{
		"date_suffix.just":  "刚刚",
		"date_suffix.min":   "分钟前",
		"date_suffix.hour":  "小时前",
		"date_suffix.day":   "天前",
		"date_suffix.month": "个月前",
	}
	for k, want := range cases {
		if got := Lang(k); got != want {
			t.Errorf("Lang(%q) = %q, want %q", k, got, want)
		}
	}
}

// TestLangSymbolGroup symbol 组完整性。
func TestLangSymbolGroup(t *testing.T) {
	if got := Lang("symbol.comma"); got != "，" {
		t.Errorf("symbol.comma = %q", got)
	}
	if got := Lang("symbol.colon"); got != "：" {
		t.Errorf("symbol.colon = %q", got)
	}
}
