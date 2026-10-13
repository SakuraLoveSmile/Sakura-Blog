package stellarimages

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"strings"

	"github.com/gen2brain/gav1d/avif"
	_ "golang.org/x/image/tiff"
)

// Analyze 返回转正后的尺寸及上游 hsla-mean-v1 取样平均色。
func Analyze(data []byte) (analysis Analysis, failure error) {
	var probe *Dimensions
	if config, _, err := image.DecodeConfig(bytes.NewReader(data)); err == nil && config.Width > 0 && config.Height > 0 {
		w, h := config.Width, config.Height
		if exifOrientation(data) >= 5 {
			w, h = h, w
		}
		probe = &Dimensions{Status: "ready", Width: w, Height: h}
	}
	defer func() {
		if failure != nil && analysis.Dimensions == nil {
			analysis.Dimensions = probe
		}
	}()
	if len(data) > MaxBytes {
		return Analysis{}, fmt.Errorf("image exceeds %d bytes", MaxBytes)
	}
	var img image.Image
	var err error
	var profile []byte
	var originalDimensions *Dimensions
	highPrecision := len(data) > 25 && bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")) && data[24] == 16
	isTIFF := bytes.HasPrefix(data, []byte("II*\x00")) || bytes.HasPrefix(data, []byte("MM\x00*")) || bytes.HasPrefix(data, []byte("II+\x00")) || bytes.HasPrefix(data, []byte("MM\x00+"))
	trim := bytes.TrimSpace(data)
	isSVG := bytes.HasPrefix(trim, []byte("<svg")) || bytes.HasPrefix(trim, []byte("<?xml")) || bytes.Contains(trim[:min(len(trim), 1024)], []byte("<svg"))
	if isSVG {
		img, originalDimensions, err = decodeSVG(data)
	} else if isTIFF {
		img, err = decodeTIFF(data)
		_, highPrecision = img.(*highDepth)
	} else if bytes.HasPrefix(data, []byte{255, 216}) {
		img, originalDimensions, err = decodeJPEG(data)
		profile = embeddedICC(data)
	} else if bytes.HasPrefix(data, []byte("RIFF")) && len(data) >= 12 && string(data[8:12]) == "WEBP" {
		img, originalDimensions, err = decodeWebP(data)
		profile = embeddedICC(data)
	} else {
		config, format, e := image.DecodeConfig(bytes.NewReader(data))
		if e != nil {
			if errors.Is(e, image.ErrFormat) {
				return Analysis{Dimensions: &Dimensions{Status: "unsupported"}, Color: &Color{Status: "unsupported", Algorithm: ColorAlgorithm}}, nil
			}
			return Analysis{}, e
		}
		if config.Width <= 0 || config.Height <= 0 || config.Width > MaxPixels/config.Height {
			return Analysis{}, fmt.Errorf("image exceeds %d pixel limit", MaxPixels)
		}
		switch format {
		case "avif":
			var ci avif.ColorInfo
			img, ci, err = avif.DecodeColor(bytes.NewReader(data), avif.Options{AutoRotate: true, FrameSizeLimit: MaxPixels})
			profile = ci.ICCP
		default:
			img, _, err = image.Decode(bytes.NewReader(data))
		}
		if err == nil && format == "gif" && (img.Bounds() != image.Rect(0, 0, config.Width, config.Height)) {
			canvas := image.NewNRGBA(image.Rect(0, 0, config.Width, config.Height))
			draw.Draw(canvas, img.Bounds(), img, img.Bounds().Min, draw.Src)
			img = canvas
		}
		if len(profile) == 0 {
			profile = embeddedICC(data)
		}
	}
	if err != nil {
		return Analysis{}, err
	}
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w <= 0 || h <= 0 || w > MaxPixels/h {
		return Analysis{}, errors.New("decoded image exceeds pixel limit")
	}
	if !isSVG && !isTIFF {
		// 上游先缩放再旋转；提前旋转会交换两次固定点过滤的顺序，改变平均色。
	}
	if !isTIFF {
		img, err = applyICC(img, profile, highPrecision)
		if err != nil {
			return Analysis{Dimensions: &Dimensions{Status: "ready", Width: w, Height: h}}, err
		}
	}
	var sums [4]float64
	if highPrecision {
		hasAlpha := true
		if h, ok := img.(*highDepth); ok {
			hasAlpha = h.alpha
		}
		sums = sampleHigh(img, hasAlpha)
	} else {
		sums = sampleMean(img)
	}
	if originalDimensions != nil {
		w, h = originalDimensions.Width, originalDimensions.Height
	}
	if exifOrientation(data) >= 5 {
		w, h = h, w
	}
	return Analysis{Dimensions: &Dimensions{Status: "ready", Width: w, Height: h}, Color: &Color{Status: "ready", Algorithm: ColorAlgorithm, Size: SampleSize, HSLA: toHSLA(sums)}}, nil
}

func toHSLA(v [4]float64) []float64 {
	r, g, b := v[0]/255, v[1]/255, v[2]/255
	hi, lo := max(r, g, b), min(r, g, b)
	h, s, l := 0.0, 0.0, (hi+lo)/2
	if hi != lo {
		d := hi - lo
		if l > 0.5 {
			s = d / (2 - hi - lo)
		} else {
			s = d / (hi + lo)
		}
		switch hi {
		case r:
			h = (g - b) / d
			if g < b {
				h += 6
			}
		case g:
			h = (b-r)/d + 2
		default:
			h = (r-g)/d + 4
		}
		h /= 6
	}
	out := []float64{h * 360, s * 100, l * 100, v[3] / 255}
	for i := range out {
		out[i] = math.Floor(out[i]*10000+0.5) / 10000
	}
	return out
}
func sinc(x float64) float64 {
	if x == 0 {
		return 1
	}
	x *= math.Pi
	return math.Sin(x) / x
}
func lanczos(x float64) float64 {
	x = math.Abs(x)
	if x >= 3 {
		return 0
	}
	return sinc(x) * sinc(x/3)
}

type tap struct {
	index  int
	weight float64
}

func sampleMean(img image.Image) [4]float64 {
	if f, ok := img.(*floatImage); ok {
		return sampleFloat(f)
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	scale := min(1.0, float64(SampleSize)/float64(max(w, h)))
	dw, dh := max(1, int(math.Floor(float64(w)*scale+0.5))), max(1, int(math.Floor(float64(h)*scale+0.5)))
	if scale == 1 {
		var sums [4]float64
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				c := straightRGBA(img.At(b.Min.X+x, b.Min.Y+y))
				for k, v := range []uint16{c.R, c.G, c.B, c.A} {
					sums[k] += float64(v / 257)
				}
			}
		}
		for k := range sums {
			sums[k] /= float64(w * h)
		}
		return sums
	}
	view := pixelView{w: w, h: h, at: func(x, y int) [4]float64 {
		c := straightRGBA(img.At(b.Min.X+x, b.Min.Y+y))
		a := float32(c.A) / 65535
		return [4]float64{math.Floor(float64(float32(float64(c.R)/257) * a)), math.Floor(float64(float32(float64(c.G)/257) * a)), math.Floor(float64(float32(float64(c.B)/257) * a)), float64(c.A / 257)}
	}}
	view = resizePixels(view, scale, true, 255)
	var sums [4]float64
	for y := 0; y < dh; y++ {
		for x := 0; x < dw; x++ {
			v := view.at(x, y)
			a := v[3]
			for k := 0; k < 3; k++ {
				if a > 0 {
					v[k] = float64(float32(v[k]) * float32(255/a))
				} else {
					v[k] = 0
				}
				sums[k] += math.Floor(max(0.0, min(255.0, v[k])))
			}
			sums[3] += a
		}
	}
	for k := range sums {
		sums[k] /= float64(dw * dh)
	}
	return sums
}

func exifOrientation(data []byte) int {
	// AVIF 的 irot/imir 变换已由容器解码器处理。
	if len(data) > 12 && strings.Contains(string(data[4:12]), "ftyp") {
		return 1
	}
	return orientationFromContainers(data)
}

func straightRGBA(c color.Color) color.NRGBA64 {
	switch v := c.(type) {
	case color.NRGBA:
		return color.NRGBA64{R: uint16(v.R) * 257, G: uint16(v.G) * 257, B: uint16(v.B) * 257, A: uint16(v.A) * 257}
	case color.NRGBA64:
		return v
	default:
		return color.NRGBA64Model.Convert(c).(color.NRGBA64)
	}
}
