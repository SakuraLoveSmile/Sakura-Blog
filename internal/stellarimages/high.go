package stellarimages

import (
	"image"
	"math"
)

type highDepth struct {
	image.Image
	alpha bool
}

func sampleHigh(img image.Image, hasAlpha bool) [4]float64 {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	scale := min(1.0, 64.0/float64(max(w, h)))
	view := pixelView{w: w, h: h, at: func(x, y int) [4]float64 {
		c := straightRGBA(img.At(b.Min.X+x, b.Min.Y+y))
		p := [4]float64{float64(c.R), float64(c.G), float64(c.B), float64(c.A)}
		if hasAlpha && scale < 1 {
			a := float32(c.A) / 65535
			for k := 0; k < 3; k++ {
				p[k] = math.Floor(float64(float32(p[k]) * a))
			}
		}
		return p
	}}
	if scale < 1 {
		view = resizePixels(view, scale, true, 65535)
	}
	var sums [4]float64
	for y := 0; y < view.h; y++ {
		for x := 0; x < view.w; x++ {
			p := view.at(x, y)
			a := p[3]
			if !hasAlpha {
				p[3] = 65535
			}
			if hasAlpha && scale < 1 {
				factor := float32(0)
				if a > 0 {
					factor = float32(65535 / a)
				}
				for c := 0; c < 3; c++ {
					p[c] = math.Floor(max(0.0, min(65535.0, float64(float32(p[c])*factor))))
				}
			}
			for c := 0; c < 4; c++ {
				sums[c] += float64(uint16(p[c]) >> 8)
			}
		}
	}
	for c := range sums {
		sums[c] /= float64(view.w * view.h)
	}
	return sums
}
