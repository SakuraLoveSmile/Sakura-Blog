// Package og 生成文章社交分享卡片图（Spec 27）。
// 1200×630 PNG，纯色渐变背景 + 标题自动换行 + 站名 + 日期。
// 字体由调用方注入 TTF 数据（LXGW WenKai），磁盘缓存 + 写时失效。
package og

import (
	"bytes"
	"fmt"
	"image/color"
	"image/png"
	"os"
	"sync"
	"path/filepath"
	"time"
	"unicode/utf8"

	"github.com/fogleman/gg"
)

const (
	// Width/Height OG 标准尺寸。
	Width  = 1200
	Height = 630
)

// PostInfo 生成 OG 图所需的文章元数据。
type PostInfo struct {
	Title     string
	Date      time.Time
	SiteTitle string
}

// ogCall 进行中的生成调用（singleflight：并发同 slug 共享一次生成，Spec 61 阶段 5）。
type ogCall struct {
	done chan struct{}
	data []byte
	err  error
}

// Generator OG 图生成器，持有已解析字体。
type Generator struct {
	fontPath  string // TTF 字体路径（运行时加载）
	cacheDir  string // 磁盘缓存目录
	fontOK    bool

	inFlightMu sync.Mutex
	inFlight   map[string]*ogCall // 进行中生成去重
}

// New 创建生成器。fontPath 为 TTF 文件路径；cacheDir 为缓存目录（自动创建）。
// 字体加载失败不 panic，Generate 时降级返回占位图。
func New(fontPath, cacheDir string) *Generator {
	_ = os.MkdirAll(cacheDir, 0o755)
	g := &Generator{fontPath: fontPath, cacheDir: cacheDir, inFlight: map[string]*ogCall{}}
	// 预检字体可读
	if _, err := os.ReadFile(fontPath); err == nil {
		g.fontOK = true
	}
	return g
}

// NewFromBytes 用内存字体数据创建生成器（embed 场景）。
// 将字体写入 cacheDir/.font.ttf 临时文件供 gg 加载。
func NewFromBytes(fontData []byte, cacheDir string) *Generator {
	_ = os.MkdirAll(cacheDir, 0o755)
	fontPath := filepath.Join(cacheDir, ".font.ttf")
	if err := os.WriteFile(fontPath, fontData, 0o644); err != nil {
		return &Generator{cacheDir: cacheDir, fontOK: false, inFlight: map[string]*ogCall{}}
	}
	return &Generator{fontPath: fontPath, cacheDir: cacheDir, fontOK: true, inFlight: map[string]*ogCall{}}
}

// CachePath 返回指定 slug 的缓存文件路径。
func (g *Generator) CachePath(slug string) string {
	// 安全审计 2026-08-16：Base 防御——slug 来自 URL 参数/DB，入库侧已加字符白名单，
	// 此处兜底保证任何含路径成分的 slug 都被折叠进缓存目录内。
	return filepath.Join(g.cacheDir, filepath.Base(slug)+".png")
}

// Invalidate 删除指定 slug 的缓存文件（文章更新/删除时调用）。
func (g *Generator) Invalidate(slug string) {
	_ = os.Remove(g.CachePath(slug))
}

// GetOrGenerate 优先读缓存；缓存不存在则生成并写入。返回 PNG 字节。
// Spec 61 阶段 5：并发同 slug 去重（singleflight 简化版）——首个请求生成，其余等待共享结果。
func (g *Generator) GetOrGenerate(slug string, info PostInfo) ([]byte, error) {
	path := g.CachePath(slug)
	if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
		return data, nil
	}
	g.inFlightMu.Lock()
	if call, ok := g.inFlight[slug]; ok {
		g.inFlightMu.Unlock()
		<-call.done
		return call.data, call.err
	}
	call := &ogCall{done: make(chan struct{})}
	g.inFlight[slug] = call
	g.inFlightMu.Unlock()

	data, err := g.Generate(info)
	if err == nil {
		_ = os.WriteFile(path, data, 0o644)
	}
	call.data, call.err = data, err
	close(call.done)
	g.inFlightMu.Lock()
	delete(g.inFlight, slug)
	g.inFlightMu.Unlock()
	return data, err
}

// Generate 绘制 OG 图并编码为 PNG 字节。
func (g *Generator) Generate(info PostInfo) ([]byte, error) {
	dc := gg.NewContext(Width, Height)

	// 渐变背景（深蓝 → 靛紫）
	drawGradient(dc)

	if !g.fontOK {
		// 字体不可用：返回纯色占位图（不 500）
		return encodePNG(dc)
	}

	// 加载字体
	titleSize := 52.0
	metaSize := 28.0
	if err := dc.LoadFontFace(g.fontPath, titleSize); err != nil {
		return nil, fmt.Errorf("load font: %w", err)
	}

	// 标题（白色，自动换行，垂直居中偏上）
	marginX := 100.0
	maxWidth := float64(Width) - marginX*2
	title := truncateTitle(info.Title, 60)
	dc.SetColor(color.White)
	dc.DrawStringWrapped(title, marginX, 220, 0, 0, maxWidth, 1.4, gg.AlignLeft)

	// 计算标题实际行数以定位 meta
	lines := wrapText(dc, title, maxWidth)
	metaY := 220 + float64(len(lines))*titleSize*1.4 + 60

	// 站名 + 日期（浅灰）
	if err := dc.LoadFontFace(g.fontPath, metaSize); err != nil {
		return nil, fmt.Errorf("load meta font: %w", err)
	}
	dc.SetColor(color.RGBA{200, 200, 220, 255})
	meta := info.SiteTitle
	if !info.Date.IsZero() {
		meta += "  ·  " + info.Date.Format("2006-01-02")
	}
	dc.DrawString(meta, marginX, metaY)

	return encodePNG(dc)
}

// drawGradient 绘制对角线渐变背景。
func drawGradient(dc *gg.Context) {
	c1 := color.RGBA{30, 41, 82, 255}  // 深蓝
	c2 := color.RGBA{76, 29, 149, 255} // 靛紫
	grad := gg.NewLinearGradient(0, 0, Width, Height)
	grad.AddColorStop(0, c1)
	grad.AddColorStop(1, c2)
	dc.SetFillStyle(grad)
	dc.DrawRectangle(0, 0, Width, Height)
	dc.Fill()
}

// encodePNG 编码为 PNG 字节。
func encodePNG(dc *gg.Context) ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, dc.Image()); err != nil {
		return nil, fmt.Errorf("encode png: %w", err)
	}
	return buf.Bytes(), nil
}

// truncateTitle 截断过长标题（rune 粒度）。
func truncateTitle(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	return string(runes[:max-1]) + "…"
}

// wrapText 计算文本在指定宽度内的行数（用于定位 meta）。
func wrapText(dc *gg.Context, text string, maxWidth float64) []string {
	var lines []string
	current := ""
	for _, r := range text {
		test := current + string(r)
		w, _ := dc.MeasureString(test)
		if w > maxWidth && current != "" {
			lines = append(lines, current)
			current = string(r)
		} else {
			current = test
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	if len(lines) == 0 {
		lines = []string{""}
	}
	return lines
}
