package post

import (
	"strings"
	"testing"
)

func TestWrapTables(t *testing.T) {
	t.Run("普通表格被包", func(t *testing.T) {
		in := "<p>前文</p>\n<table><thead><tr><th>A</th></tr></thead><tbody><tr><td>1</td></tr></tbody></table>\n<p>后文</p>"
		out := wrapTables(in)
		if !strings.Contains(out, `<div class="md-table-scroll"><table>`) {
			t.Fatalf("普通表格未包容器: %s", out)
		}
		if !strings.Contains(out, "</table></div>") {
			t.Fatalf("容器未闭合: %s", out)
		}
	})
	t.Run("highlight 内表格不包", func(t *testing.T) {
		in := `<div class="highlight"><table class="lntable"><tr><td>1</td></tr></table></div>`
		out := wrapTables(in)
		if strings.Contains(out, "md-table-scroll") {
			t.Fatalf("highlight 表格不应包容器: %s", out)
		}
	})
	t.Run("tag-plugin table 不包", func(t *testing.T) {
		in := `<div class="tag-plugin colorful table"><table><tr><td>a</td></tr></table></div>`
		out := wrapTables(in)
		if strings.Contains(out, "md-table-scroll") {
			t.Fatalf("tag-plugin.table 不应包容器: %s", out)
		}
	})
	t.Run("无表格原样", func(t *testing.T) {
		in := "<p>无表格</p>"
		if out := wrapTables(in); out != in {
			t.Fatalf("无表格应原样: %s", out)
		}
	})
	t.Run("多表格各自包装", func(t *testing.T) {
		in := "<table><tr><td>1</td></tr></table><table><tr><td>2</td></tr></table>"
		out := wrapTables(in)
		if c := strings.Count(out, `class="md-table-scroll"`); c != 2 {
			t.Fatalf("应包装 2 个表格，got %d: %s", c, out)
		}
	})
}
