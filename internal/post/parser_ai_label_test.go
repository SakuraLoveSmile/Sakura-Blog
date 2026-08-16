package post

import "testing"

// TestParseFrontmatterAILabel frontmatter 的 ai_label 字段解析（1.41）。
func TestParseFrontmatterAILabel(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{"manual 档", "---\ntitle: t\nai_label: manual\n---\n正文", "manual"},
		{"generated 档带引号", "---\nai_label: \"generated\"\n---\n", "generated"},
		{"未写为空", "---\ntitle: t\n---\n正文", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fm, body := newParser().Parse([]byte(tc.src))
			if fm.AILabel != tc.want {
				t.Fatalf("AILabel = %q, want %q", fm.AILabel, tc.want)
			}
			if tc.name == "manual 档" && body != "正文" {
				t.Fatalf("body = %q", body)
			}
		})
	}
}
