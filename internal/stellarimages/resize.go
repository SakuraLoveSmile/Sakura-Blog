package stellarimages

import "math"

type pixelView struct {
	w, h int
	at   func(int, int) [4]float64
}

func reduceAxis(view pixelView, target int, scale float64, vertical, integerPixels bool, maxValue float64) pixelView {
	size, cross := view.w, view.h
	if vertical {
		size, cross = view.h, view.w
	}
	if size == target && scale == 1 {
		return view
	}
	read := func(a, b int) [4]float64 {
		if vertical {
			return view.at(b, a)
		}
		return view.at(a, b)
	}
	extent := float64(size)
	factor := max(1, int(math.Floor(float64(size)/float64(target)/2)))
	round := func(v float64) float64 {
		if integerPixels {
			return math.Floor(max(0.0, min(maxValue, v+0.5)))
		}
		return float64(float32(v))
	}
	if factor > 1 {
		oldRead, oldSize := read, size
		read = func(a, b int) [4]float64 {
			var p [4]float64
			for j := 0; j < factor; j++ {
				q := oldRead(min(oldSize-1, a*factor+j), b)
				for c := range p {
					p[c] += q[c]
				}
			}
			for c := range p {
				p[c] = round(p[c] / float64(factor))
			}
			return p
		}
		size = (size + factor - 1) / factor
		extent /= float64(factor)
		scale /= float64(factor)
	}
	if scale == 1 && size == target {
		out := pixelView{w: target, h: cross}
		if vertical {
			out.w, out.h = cross, target
		}
		out.at = func(x, y int) [4]float64 {
			if vertical {
				return read(y, x)
			}
			return read(x, y)
		}
		return out
	}
	masks := axisWeights(size, target, scale, extent, integerPixels)
	pixels := make([][4]float64, target*cross)
	for b := 0; b < cross; b++ {
		for a := 0; a < target; a++ {
			var p [4]float64
			for _, tap := range masks[a] {
				q := read(tap.index, b)
				for c := range p {
					p[c] += q[c] * tap.weight
				}
			}
			for c := range p {
				p[c] = round(p[c])
			}
			index := b*target + a
			if vertical {
				index = a*cross + b
			}
			pixels[index] = p
		}
	}
	out := pixelView{w: target, h: cross}
	if vertical {
		out.w, out.h = cross, target
	}
	out.at = func(x, y int) [4]float64 { return pixels[y*out.w+x] }
	return out
}
func axisWeights(src, dst int, scale, extent float64, integer bool) [][]tap {
	out := make([][]tap, dst)
	n := 2*int(math.RoundToEven(3*scale)) + 1
	pad := n / 2
	for i := range out {
		coordinate := (float64(i)+0.5)*scale - (float64(dst)*scale-extent)/2
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
			weight := v / sum
			if integer {
				weight = math.Trunc(weight*4096) / 4096
			}
			out[i] = append(out[i], tap{min(src-1, max(0, start+j-pad)), weight})
		}
	}
	return out
}
func resizePixels(view pixelView, scale float64, integer bool, maxValue float64) pixelView {
	dw, dh := max(1, int(math.Floor(float64(view.w)*scale+0.5))), max(1, int(math.Floor(float64(view.h)*scale+0.5)))
	v := reduceAxis(view, dh, max(1.0, 1/scale), true, integer, maxValue)
	return reduceAxis(v, dw, max(1.0, 1/scale), false, integer, maxValue)
}
