package stellarimages

import (
	_ "embed"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"math"
)

//go:embed native/tiff.wasm
var tiffWASM []byte
var tiffRuntime wasmDecoder

func decodeTIFF(data []byte) (image.Image, error) {
	b, err := tiffRuntime.run(tiffWASM, data, "TIFF decode")
	if err != nil {
		return nil, err
	}
	if len(b) < 32 {
		return nil, errors.New("invalid TIFF output")
	}
	values := [8]uint32{}
	for i := range values {
		values[i] = binary.LittleEndian.Uint32(b[i*4:])
	}
	w, h, photo, bits, spp, format, orientation, plen := int(values[0]), int(values[1]), values[2], values[3], int(values[4]), values[5], int(values[6]), int(values[7])
	if w <= 0 || h <= 0 || w > MaxPixels/h || plen > MaxBytes || plen > len(b)-32 {
		return nil, errors.New("invalid TIFF dimensions")
	}
	profile := b[32 : 32+plen]
	pixels := b[32+plen:]
	sampleBytes := int(bits / 8)
	if sampleBytes <= 0 || len(pixels) != w*h*spp*sampleBytes {
		return nil, errors.New("invalid TIFF pixel buffer")
	}
	channels := 3
	model := uint32(2)
	switch photo {
	case 0, 1:
		channels = 1
		model = 8
	case 2:
	case 5:
		channels = 4
		model = 4
		if len(profile) == 0 {
			profile = defaultCMYK
		}
	case 8, 9:
		channels = 3
		model = 6
	default:
		return nil, fmt.Errorf("TIFF photometric %d requires conversion", photo)
	}
	if spp < channels {
		return nil, errors.New("invalid TIFF samples")
	}
	if bits == 32 && format&65535 == 3 {
		out := &floatImage{w: w, h: h, pixels: make([][4]float64, w*h), lab: photo == 8 || photo == 9, alpha: spp > channels}
		for i := range out.pixels {
			for c := 0; c < channels; c++ {
				out.pixels[i][c] = float64(math.Float32frombits(binary.LittleEndian.Uint32(pixels[(i*spp+c)*4:])))
			}
			if channels == 1 {
				out.pixels[i][1], out.pixels[i][2] = out.pixels[i][0], out.pixels[i][0]
			}
			out.pixels[i][3] = 1
			if out.lab {
				out.pixels[i][3] = 255
			}
			if out.alpha {
				out.pixels[i][3] = float64(math.Float32frombits(binary.LittleEndian.Uint32(pixels[(i*spp+channels)*4:])))
			}
		}
		return out, nil
	}
	raw := make([]byte, w*h*channels*2)
	alpha := make([]uint16, w*h)
	for i := 0; i < w*h; i++ {
		sample := func(c int) uint16 {
			off := (i*spp + c) * sampleBytes
			switch bits {
			case 8:
				return uint16(pixels[off]) * 257
			case 16:
				return binary.LittleEndian.Uint16(pixels[off:])
			case 32:
				v := float64(math.Float32frombits(binary.LittleEndian.Uint32(pixels[off:])))
				return uint16(math.Floor(max(0.0, min(65535.0, v*257)) + 0.5))
			}
			return 0
		}
		a := uint16(65535)
		if spp > channels {
			a = sample(channels)
		}
		alpha[i] = a
		for c := 0; c < channels; c++ {
			v := sample(c)
			if photo == 0 {
				v = 65535 - v
			}
			if photo == 8 && c > 0 {
				if bits == 8 {
					v = uint16(uint8(pixels[(i*spp+c)*sampleBytes]+128)) * 257
				} else {
					v ^= 32768
				}
			}
			if format&0x10000 != 0 && a > 0 {
				v = uint16(min(uint64(65535), uint64(v)*65535/uint64(a)))
			}
			binary.LittleEndian.PutUint16(raw[(i*channels+c)*2:], v)
		}
	}
	var img image.Image
	if len(profile) > 0 || photo == 5 || photo == 8 || photo == 9 {
		if bits == 8 {
			model--
			small := make([]byte, len(raw)/2)
			for i := range small {
				small[i] = byte(binary.LittleEndian.Uint16(raw[i*2:]) / 257)
			}
			raw = small
		}
		if bits == 16 && photo == 2 {
			model |= 256
		}
		if bits == 16 && photo == 5 {
			model |= 512
		}
		img, err = transformPixels(raw, profile, model, w, h, alpha)
	} else {
		out := image.NewNRGBA64(image.Rect(0, 0, w, h))
		for i := 0; i < w*h; i++ {
			v := [3]uint16{}
			if channels == 1 {
				gray := binary.LittleEndian.Uint16(raw[i*2:])
				v = [3]uint16{gray, gray, gray}
			} else {
				for c := range v {
					v[c] = binary.LittleEndian.Uint16(raw[(i*3+c)*2:])
				}
			}
			out.SetNRGBA64(i%w, i/w, color.NRGBA64{R: v[0], G: v[1], B: v[2], A: alpha[i]})
		}
		img = out
	}
	if err != nil {
		return nil, err
	}
	_ = orientation // rotation is applied after sampling by Analyze
	if bits == 16 && photo != 5 {
		img = &highDepth{img, spp > channels}
	}
	return img, nil
}
