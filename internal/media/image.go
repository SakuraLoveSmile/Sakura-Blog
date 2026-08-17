// Package media contains shared media processing helpers.
package media

import (
	"bytes"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"strings"

	"golang.org/x/image/draw"
)

// 图像优化阈值
const (
	optimizeSizeThreshold  = 200 * 1024 // 200KB 以上才压缩
	optimizeWidthThreshold = 2000       // 宽度超过 2000px 才压缩
	thumbMaxWidth          = 400        // 缩略图最大宽度
	mainMaxWidth           = 2000       // 主图最大宽度
	jpegMainQuality        = 80
	jpegThumbQuality       = 70
	// 解码炸弹防护（安全审计 2026-08-16）：Decode 前用 DecodeConfig 读声明尺寸，
	// 超限直接拒绝——Go png/jpeg 解码器按声明尺寸一次性分配像素缓冲，
	// 几十 KB 的 PNG 声明 50000×50000 即分配 ~10GB 内存打挂进程。
	maxDecodeWidth  = 30000                  // 单边像素上限
	maxDecodePixels = 100_000_000            // 总像素上限（1 亿）
	maxDecodeSide   = int64(maxDecodeWidth)  // 便于比较
	maxDecodeArea   = int64(maxDecodePixels) // 便于比较
)

// ImageResult 图像优化结果。
type ImageResult struct {
	MainData  []byte // 优化后主图数据（可能等于原数据=透传）
	ThumbData []byte // 缩略图数据，nil=无需缩略图
	Width     int    // 最终主图宽度
	Height    int    // 最终主图高度
	OutMime   string // 输出 MIME（优化后可能变 image/jpeg）
	OutExt    string // 输出扩展名（如 .jpg）
	Optimized bool   // 是否实际执行了优化
}

// OptimizeImage 对上传图片进行优化处理。
// 规则：
//   - gif/svg/webp 直接透传（不重编码）
//   - jpeg/png 小于阈值且宽度不超限时透传
//   - 需要优化时：缩放至 mainMaxWidth 以内 + JPEG 质量 80 编码
//   - 缩略图：缩放至 thumbMaxWidth 宽 + JPEG 质量 70
func OptimizeImage(src []byte, mime string) (*ImageResult, error) {
	// gif / svg / webp 透传
	if mime == "image/gif" || mime == "image/svg+xml" || mime == "image/webp" {
		return &ImageResult{
			MainData:  src,
			ThumbData: nil,
			Width:     0,
			Height:    0,
			OutMime:   mime,
			OutExt:    mimeToExt(mime),
			Optimized: false,
		}, nil
	}

	// 解码炸弹预检（安全审计 2026-08-16）：先 DecodeConfig 读声明尺寸，超限拒绝。
	cfg, _, err := image.DecodeConfig(bytes.NewReader(src))
	if err != nil {
		// 连配置头都解不出来：按原语义透传（后续 Decode 也会失败）
		return &ImageResult{
			MainData:  src,
			ThumbData: nil,
			Width:     0,
			Height:    0,
			OutMime:   mime,
			OutExt:    mimeToExt(mime),
			Optimized: false,
		}, nil
	}
	if cfg.Width <= 0 || cfg.Height <= 0 ||
		int64(cfg.Width) > maxDecodeSide || int64(cfg.Height) > maxDecodeSide ||
		int64(cfg.Width)*int64(cfg.Height) > maxDecodeArea {
		return nil, fmt.Errorf("图片声明尺寸 %dx%d 超限（单边 ≤%d、总像素 ≤%d），疑似解码炸弹",
			cfg.Width, cfg.Height, maxDecodeWidth, maxDecodePixels)
	}

	// 解码图像获取尺寸
	img, format, err := image.Decode(bytes.NewReader(src))
	if err != nil {
		// 解码失败则透传
		return &ImageResult{
			MainData:  src,
			ThumbData: nil,
			Width:     0,
			Height:    0,
			OutMime:   mime,
			OutExt:    mimeToExt(mime),
			Optimized: false,
		}, nil
	}
	_ = format

	bounds := img.Bounds()
	origW := bounds.Dx()
	origH := bounds.Dy()

	// 判断是否需要优化
	needOptimize := len(src) > optimizeSizeThreshold || origW > optimizeWidthThreshold
	if !needOptimize {
		// 小图：仍生成缩略图（Console 列表用），主图透传
		thumb := generateThumb(img, origW, origH)
		return &ImageResult{
			MainData:  src,
			ThumbData: thumb,
			Width:     origW,
			Height:    origH,
			OutMime:   mime,
			OutExt:    mimeToExt(mime),
			Optimized: false,
		}, nil
	}

	// 需要优化：缩放主图
	mainImg := img
	mainW, mainH := origW, origH
	if origW > mainMaxWidth {
		mainH = origH * mainMaxWidth / origW
		mainW = mainMaxWidth
		mainImg = resizeImage(img, mainW, mainH)
	}

	// 编码主图为 JPEG
	var mainBuf bytes.Buffer
	if err := jpeg.Encode(&mainBuf, mainImg, &jpeg.Options{Quality: jpegMainQuality}); err != nil {
		// 编码失败则透传
		return &ImageResult{
			MainData:  src,
			ThumbData: nil,
			Width:     origW,
			Height:    origH,
			OutMime:   mime,
			OutExt:    mimeToExt(mime),
			Optimized: false,
		}, nil
	}

	// 生成缩略图
	thumb := generateThumb(mainImg, mainW, mainH)

	return &ImageResult{
		MainData:  mainBuf.Bytes(),
		ThumbData: thumb,
		Width:     mainW,
		Height:    mainH,
		OutMime:   "image/jpeg",
		OutExt:    ".jpg",
		Optimized: true,
	}, nil
}

// generateThumb 生成缩略图（JPEG 质量 70，最大宽度 400px）。
func generateThumb(img image.Image, w, h int) []byte {
	if w <= 0 || h <= 0 {
		return nil
	}
	tw, th := w, h
	if tw > thumbMaxWidth {
		th = h * thumbMaxWidth / w
		tw = thumbMaxWidth
	}
	thumbImg := resizeImage(img, tw, th)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, thumbImg, &jpeg.Options{Quality: jpegThumbQuality}); err != nil {
		return nil
	}
	return buf.Bytes()
}

// resizeImage 使用 CatmullRom 高质量缩放。
func resizeImage(img image.Image, w, h int) image.Image {
	if w <= 0 || h <= 0 {
		return img
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Over, nil)
	return dst
}

// imageDimensions 快速获取图片宽高（不完整解码）。
func imageDimensions(src []byte, mime string) (w, h int) {
	if mime == "image/svg+xml" {
		return 0, 0
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(src))
	if err != nil {
		return 0, 0
	}
	return cfg.Width, cfg.Height
}

func mimeToExt(mime string) string {
	switch strings.ToLower(mime) {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "image/svg+xml":
		return ".svg"
	default:
		return ".bin"
	}
}

// init 注册标准库解码器（png/gif 需要 import 触发 init）。
func init() {
	_ = png.Decode
	_ = gif.Decode
}
