package post

import (
	"reflect"
	"testing"
)

func TestParseShortcodeForEditNote(t *testing.T) {
	got, err := ParseShortcodeForEdit(`{% note color:green 提示 这里是正文 %}`)
	if err != nil {
		t.Fatalf("ParseShortcodeForEdit: %v", err)
	}
	want := ShortcodeEditResult{
		Name:     "note",
		Editable: true,
		Values: map[string]any{
			"color":   "green",
			"title":   "提示",
			"content": "这里是正文",
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("result = %#v, want %#v", got, want)
	}
}

func TestParseShortcodeForEditSimpleTags(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want map[string]any
	}{
		{
			name: "checkbox flag",
			raw:  `{% checkbox checked 已完成的任务 color:green %}`,
			want: map[string]any{"text": "已完成的任务", "checked": true, "color": "green"},
		},
		{
			name: "folding block",
			raw:  "{% folding color:cyan open:true 点击查看详情 %}\n折叠正文\n{% endfolding %}",
			want: map[string]any{"color": "cyan", "open": true, "title": "点击查看详情", "content": "折叠正文"},
		},
		{
			name: "box block",
			raw:  "{% box color:green 卡片标题 %}\n卡片正文\n{% endbox %}",
			want: map[string]any{"color": "green", "title": "卡片标题", "content": "卡片正文"},
		},
		{
			name: "media",
			raw:  `{% image /assets/example.jpg 示例图 width:80% download:true %}`,
			want: map[string]any{"src": "/assets/example.jpg", "alt": "示例图", "width": "80%", "download": true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseShortcodeForEdit(tt.raw)
			if err != nil {
				t.Fatalf("ParseShortcodeForEdit: %v", err)
			}
			if !got.Editable {
				t.Fatal("result should be editable")
			}
			if !reflect.DeepEqual(got.Values, tt.want) {
				t.Fatalf("values = %#v, want %#v", got.Values, tt.want)
			}
		})
	}
}

func TestParseShortcodeForEditFallbacks(t *testing.T) {
	unsupported, err := ParseShortcodeForEdit("{% tabs %}\n<!-- tab 一 -->正文\n{% endtabs %}")
	if err != nil {
		t.Fatalf("unsupported shortcode should not fail: %v", err)
	}
	if unsupported.Name != "tabs" || unsupported.Editable {
		t.Fatalf("unsupported result = %#v", unsupported)
	}

	if _, err := ParseShortcodeForEdit("{% folding title %}正文"); err == nil {
		t.Fatal("malformed block should fail")
	}
	if _, err := ParseShortcodeForEdit(`{% image %}`); err == nil {
		t.Fatal("missing required image source should fail")
	}
	if _, err := ParseShortcodeForEdit(`{% copy git:user/repo prefix:git %}`); err == nil {
		t.Fatal("catalog-incomplete arguments should fail instead of being dropped")
	}
}
