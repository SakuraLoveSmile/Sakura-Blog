package i18n

import (
	"sync"
	"testing"
)

func TestOfficialLocalesAndUnknownKeys(t *testing.T) {
	for _, c := range []struct{ locale, key, want string }{{"zh-CN", "btn.home", "主页"}, {"zh-TW", "btn.home", "首頁"}, {"en", "btn.home", "Home"}, {"en-US", "btn.home", "Home"}, {"zh_TW", "btn.home", "首頁"}, {"en", "Custom label", "Custom label"}} {
		if got := Text(c.locale, c.key); got != c.want {
			t.Fatalf("%s %s = %q want %q", c.locale, c.key, got, c.want)
		}
	}
	if Text("zh-CN", "meta.command", 3) != "命令 3" {
		t.Fatal("placeholder substitution")
	}
}
func TestRequestLocaleIsIndependent(t *testing.T) {
	var wg sync.WaitGroup
	for _, locale := range []string{"zh-CN", "zh-TW", "en"} {
		wg.Add(1)
		go func(language string) {
			defer wg.Done()
			expected := Text(language, "settings.title")
			for i := 0; i < 1000; i++ {
				if Text(language, "settings.title") != expected {
					t.Error("cross request language mutation")
				}
			}
		}(locale)
	}
	wg.Wait()
}
