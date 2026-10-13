package stellarimages

import (
	"image"
	"image/color"
	"math"
)

type floatImage struct {
	w, h   int
	pixels [][4]float64
	lab    bool
	alpha  bool
}

func (f *floatImage) Bounds() image.Rectangle { return image.Rect(0, 0, f.w, f.h) }
func (f *floatImage) ColorModel() color.Model { return color.NRGBA64Model }
func (f *floatImage) At(x, y int) color.Color {
	p := f.pixels[y*f.w+x]
	return color.NRGBA64{R: uint16(max(0.0, min(65535.0, p[0]*257))), G: uint16(max(0.0, min(65535.0, p[1]*257))), B: uint16(max(0.0, min(65535.0, p[2]*257))), A: uint16(max(0.0, min(65535.0, p[3]*257)))}
}
func floatWeights(src, dst int, scale float64) [][]tap {
	out := make([][]tap, dst)
	n := 2*int(math.RoundToEven(3*scale)) + 1
	pad := n / 2
	for i := range out {
		coordinate := (float64(i)+0.5)*scale - (float64(dst)*scale-float64(src))/2
		start := int(coordinate)
		phase := float64(((int(coordinate*128)&127)+1)>>1) / 64
		half := phase + float64(n)/2 - 1
		sum := 0.0
		values := make([]float64, n)
		for j := range values {
			values[j] = lanczos((float64(j) - half) / scale)
			sum += values[j]
		}
		for j, v := range values {
			out[i] = append(out[i], tap{min(src-1, max(0, start+j-pad)), v / sum})
		}
	}
	return out
}
func sampleFloat(f *floatImage) [4]float64 {
	scale := min(1.0, 64.0/float64(max(f.w, f.h)))
	dw, dh := max(1, int(math.Floor(float64(f.w)*scale+0.5))), max(1, int(math.Floor(float64(f.h)*scale+0.5)))
	pixels := append([][4]float64(nil), f.pixels...)
	maxAlpha := 1.0
	if f.lab {
		maxAlpha = 255
	}
	if f.alpha && scale < 1 {
		for i := range pixels {
			a := max(0.0, min(maxAlpha, pixels[i][3])) / maxAlpha
			for c := 0; c < 3; c++ {
				pixels[i][c] = float64(float32(pixels[i][c]) * float32(a))
			}
		}
	}
	if scale < 1 {
		xs, ys := floatWeights(f.w, dw, 1/scale), floatWeights(f.h, dh, 1/scale)
		vertical := make([][4]float64, f.w*dh)
		for y := 0; y < dh; y++ {
			for x := 0; x < f.w; x++ {
				for _, t := range ys[y] {
					p := pixels[t.index*f.w+x]
					for c := 0; c < 4; c++ {
						vertical[y*f.w+x][c] += p[c] * t.weight
					}
				}
				for c := 0; c < 4; c++ {
					vertical[y*f.w+x][c] = float64(float32(vertical[y*f.w+x][c]))
				}
			}
		}
		pixels = make([][4]float64, dw*dh)
		for y := 0; y < dh; y++ {
			for x := 0; x < dw; x++ {
				for _, t := range xs[x] {
					p := vertical[y*f.w+t.index]
					for c := 0; c < 4; c++ {
						pixels[y*dw+x][c] += p[c] * t.weight
					}
				}
				for c := 0; c < 4; c++ {
					pixels[y*dw+x][c] = float64(float32(pixels[y*dw+x][c]))
				}
			}
		}
	}
	var sums [4]float64
	for _, p := range pixels {
		a := maxAlpha
		if f.alpha {
			a = p[3]
			if math.IsNaN(a) {
				a = 0
			}
			if scale < 1 {
				factor := 0.0
				if math.Abs(a) >= 0.01 {
					factor = maxAlpha / a
				}
				for c := 0; c < 3; c++ {
					p[c] = float64(float32(factor) * float32(p[c]))
				}
			}
		}
		if math.IsNaN(p[0]) || math.IsNaN(p[1]) || math.IsNaN(p[2]) {
			p[0], p[1], p[2] = 0, 0, 0
		}
		if f.lab {
			p = labToSRGB(p)
		} else {
			for c := 0; c < 3; c++ {
				p[c] = gammaLookup(p[c])
			}
		}
		for c := 0; c < 3; c++ {
			sums[c] += math.Floor(max(0.0, min(255.0, p[c])) + 0.5)
		}
		sums[3] += math.Floor(max(0.0, min(255.0, a/maxAlpha*255)))
	}
	for c := range sums {
		sums[c] /= float64(dw * dh)
	}
	return sums
}
func labToSRGB(p [4]float64) [4]float64 {
	l, a, b := float64(float32(p[0])), float64(float32(p[1])), float64(float32(p[2]))
	cy := 0.0
	y := 0.0
	if l < 8 {
		y = float64(float32(l * 100 / 903.3))
		cy = 7.787*(y/100) + 16.0/116
	} else {
		cy = (l + 16) / 116
		y = float64(float32(100 * cy * cy * cy))
	}
	x := a/500 + cy
	z := cy - b/200
	convert := func(v, white float64) float64 {
		if v < 0.2069 {
			return float64(float32(white * (v - 0.13793) / 7.787))
		}
		return float64(float32(white * v * v * v))
	}
	x, z = convert(x, 95.047), convert(z, 108.8827)
	xf, yf, zf := float32(x)/100, float32(y)/100, float32(z)/100
	linear := [3]float32{3.240625*xf - 1.537208*yf - 0.498629*zf, -0.968931*xf + 1.875756*yf + 0.041518*zf, 0.055710*xf - 0.204021*yf + 1.056996*zf}
	for _, v := range linear {
		if math.IsNaN(float64(v)) {
			return [4]float64{0, 0, 0, p[3]}
		}
	}
	for c, v := range linear {
		p[c] = gammaLookup(float64(v))
	}
	return p
}

var gammaTable = func() [257]float32 {
	var t [257]float32
	for i := 0; i < 256; i++ {
		f := float32(i) / 255
		v := float32(0)
		if f <= 0.0031308 {
			v = 12.92 * f
		} else {
			v = float32(1.055)*float32(math.Pow(float64(f), float64(float32(1.0)/float32(2.4)))) - 0.055
		}
		t[i] = float32(math.RoundToEven(float64(255 * v)))
	}
	t[256] = t[255]
	return t
}()

func gammaLookup(v float64) float64 {
	if math.IsNaN(v) {
		return 0
	}
	position := max(float32(0), min(float32(255), float32(v)*255))
	i := int(position)
	result := gammaTable[i] + (gammaTable[i+1]-gammaTable[i])*(position-float32(i))
	return math.RoundToEven(float64(result))
}
