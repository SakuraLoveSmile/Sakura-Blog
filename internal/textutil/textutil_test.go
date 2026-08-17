package textutil

import "testing"

// TestTruncate 按 rune 截断，多字节安全，超长加省略号。
func TestTruncate(t *testing.T) {
	cases := []struct {
		in   string
		n    int
		want string
	}{
		{"短文本", 40, "短文本"},
		{"一二三四五", 3, "一二三…"},
		{"", 5, ""},
	}
	for _, c := range cases {
		if got := Truncate(c.in, c.n); got != c.want {
			t.Errorf("Truncate(%q, %d) = %q, want %q", c.in, c.n, got, c.want)
		}
	}
}

// TestStripHTML 去标签 + 压缩空白。
func TestStripHTML(t *testing.T) {
	got := StripHTML("<p>hello   <b>world</b></p>\n\n<span>!</span>")
	if got != "hello world !" {
		t.Errorf("StripHTML = %q, want %q", got, "hello world !")
	}
}
