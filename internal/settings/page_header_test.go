package settings

import "testing"

func TestPageBannerHelpers(t *testing.T) {
	p := &Page{Title: "页面", Banner: "https://example.test/o'reilly.jpg", BannerInfo: PageBannerInfo{Title: "横幅", Avatar: "/avatar.png", Icon: "/icon.png"}}
	if got := p.BannerTitle(); got != "横幅" {
		t.Fatalf("BannerTitle = %q, want 横幅", got)
	}
	if got := p.BannerAvatar(); got != "/avatar.png" {
		t.Fatalf("BannerAvatar should prefer avatar, got %q", got)
	}
	if got := p.BannerStyleURL(); got != "https://example.test/o%27reilly.jpg" {
		t.Fatalf("BannerStyleURL = %q, want apostrophe escaped", got)
	}
	p.BannerInfo = PageBannerInfo{Icon: "/icon.png"}
	if got := p.BannerAvatar(); got != "/icon.png" {
		t.Fatalf("BannerAvatar should fall back to icon, got %q", got)
	}
}
