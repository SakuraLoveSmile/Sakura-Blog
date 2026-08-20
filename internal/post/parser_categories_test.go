package post

import (
	"reflect"
	"testing"
)

func TestParseFrontmatterCategories(t *testing.T) {
	cases := []struct {
		name string
		src  string
		cat  string
		want []string
	}{
		{"legacy scalar", "---\ncategory: 技术\n---\n正文", "技术", []string{"技术"}},
		{"inline list", "---\ncategories: [产研/技术, 产品]\n---\n正文", "产研/技术", []string{"产研/技术", "产品"}},
		{"block list", "---\ncategories:\n  - 产研/技术\n  - 产品\n---\n正文", "产研/技术", []string{"产研/技术", "产品"}},
		{"categories wins", "---\ncategory: 旧\ncategories: [新/子]\n---\n正文", "新/子", []string{"新/子"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fm, body := newParser().Parse([]byte(tc.src))
			if fm.Category != tc.cat || !reflect.DeepEqual(fm.Categories, tc.want) || body != "正文" {
				t.Fatalf("category=%q categories=%#v body=%q", fm.Category, fm.Categories, body)
			}
		})
	}
}
