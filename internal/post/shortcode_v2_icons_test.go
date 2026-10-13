package post

import (
	"blog/internal/settings"
	"strings"
	"testing"
)

func TestStellarV2EmbeddedImageIcon(t *testing.T) {
	icons, err := settings.LoadIconsEmbed()
	if err != nil {
		t.Fatal(err)
	}
	s := NewService("")
	s.SetIconFunc(icons.Get)
	for _, key := range []string{"share:wechat", "share:weibo", "share:email", "share:link"} {
		t.Run(key, func(t *testing.T) {
			got, err := s.RenderStellarV2(`{% icon `+key+` style:"width:2em;background:url(javascript:bad)" %}`, nil)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(got, `<img style="width:2em" src="`+icons.Get(key)+`"`) || strings.Contains(got, "javascript:") {
				t.Fatalf("registered image icon was not rendered with safe attributes: %s", got)
			}
		})
	}
	// 上游 SVG 分支忽略图片专用属性，不能借图片修复改变此语义。
	got, err := s.RenderStellarV2(`{% icon default:copy style:width:2em %}`, nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, `style="width:2em"`) {
		t.Fatalf("SVG icon changed official image-only attribute semantics: %s", got)
	}
}
