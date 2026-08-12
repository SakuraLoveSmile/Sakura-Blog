package og

import (
	"bytes"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// testFontPath 返回测试用 TTF 路径（Spec 61 起字体归 OG 专用 embed，见同目录 fonts/）。
func testFontPath(t *testing.T) string {
	t.Helper()
	// 与 og/fonts embed 同源（backend/internal/og/fonts/）
	p := filepath.Join("fonts", "LXGWWenKai-Regular.ttf")
	if _, err := os.Stat(p); err != nil {
		t.Skipf("font not available: %v", err)
	}
	return p
}

func TestGenerateNonEmptyPNG(t *testing.T) {
	fontPath := testFontPath(t)
	g := New(fontPath, t.TempDir())

	data, err := g.Generate(PostInfo{
		Title:     "Hello 你好世界",
		Date:      time.Date(2026, 7, 21, 0, 0, 0, 0, time.Local),
		SiteTitle: "测试博客",
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("empty png")
	}
	// 验证是合法 PNG 且尺寸正确
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode png: %v", err)
	}
	b := img.Bounds()
	if b.Dx() != Width || b.Dy() != Height {
		t.Errorf("size = %dx%d, want %dx%d", b.Dx(), b.Dy(), Width, Height)
	}
}

func TestCacheAndInvalidate(t *testing.T) {
	fontPath := testFontPath(t)
	cacheDir := t.TempDir()
	g := New(fontPath, cacheDir)

	info := PostInfo{Title: "缓存测试", SiteTitle: "Blog"}

	// 首次生成
	data1, err := g.GetOrGenerate("test-slug", info)
	if err != nil {
		t.Fatal(err)
	}
	// 缓存文件应存在
	if _, err := os.Stat(g.CachePath("test-slug")); err != nil {
		t.Fatalf("cache file missing: %v", err)
	}
	// 再次读取应命中缓存（内容相同）
	data2, err := g.GetOrGenerate("test-slug", info)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data1, data2) {
		t.Error("cache hit returned different data")
	}

	// 失效后缓存文件应被删除
	g.Invalidate("test-slug")
	if _, err := os.Stat(g.CachePath("test-slug")); !os.IsNotExist(err) {
		t.Error("cache file still exists after invalidate")
	}

	// 失效后重新生成应成功
	data3, err := g.GetOrGenerate("test-slug", info)
	if err != nil {
		t.Fatal(err)
	}
	if len(data3) == 0 {
		t.Error("regenerated empty")
	}
}

func TestFontMissing(t *testing.T) {
	// 字体不存在时应降级返回占位图（不报错）
	g := New("/nonexistent/font.ttf", t.TempDir())
	data, err := g.Generate(PostInfo{Title: "无字体", SiteTitle: "Blog"})
	if err != nil {
		t.Fatalf("should not error on missing font: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("empty placeholder")
	}
	// 验证仍是合法 PNG
	if _, err := png.Decode(bytes.NewReader(data)); err != nil {
		t.Fatalf("placeholder not valid png: %v", err)
	}
}

func TestTruncateTitle(t *testing.T) {
	short := "短标题"
	if truncateTitle(short, 60) != short {
		t.Error("short title should not be truncated")
	}
	long := string(make([]rune, 100))
	result := truncateTitle(long, 60)
	if len([]rune(result)) != 60 {
		t.Errorf("truncated len = %d, want 60", len([]rune(result)))
	}
}
