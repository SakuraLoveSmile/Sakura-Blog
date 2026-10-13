package stellarimages

import (
	"bytes"
	"compress/zlib"
	_ "embed"
	"encoding/binary"
	"errors"
	"image"
	"io"
	"sort"
	"strings"
)

//go:embed native/cmyk.icm
var defaultCMYK []byte

func applyICC(img image.Image, profile []byte, highPrecision bool) (image.Image, error) {
	_, cmyk := img.(*image.CMYK)
	if len(profile) == 0 && !cmyk {
		return img, nil
	}
	if cmyk && len(profile) == 0 {
		profile = defaultCMYK
	}
	b := img.Bounds()
	count := b.Dx() * b.Dy()
	model := uint32(2)
	if highPrecision {
		model |= 256
	}
	if !highPrecision {
		model = 1
	}
	channels := 3
	if cmyk {
		model = 3
		channels = 4
	}
	gray := len(profile) > 20 && string(profile[16:20]) == "GRAY"
	if gray {
		if highPrecision {
			return img, nil
		}
		model = 7
		channels = 1
	}
	raw := make([]byte, count*channels*int(2-model%2))
	alphas := make([]uint16, count)
	offset := 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			i := (y-b.Min.Y)*b.Dx() + x - b.Min.X
			if cmyk {
				c := img.(*image.CMYK).CMYKAt(x, y)
				copy(raw[offset:], []byte{c.C, c.M, c.Y, c.K})
				offset += 4
				alphas[i] = 65535
			} else {
				c := straightRGBA(img.At(x, y))
				for _, v := range []uint16{c.R, c.G, c.B}[:channels] {
					if model%2 == 1 {
						raw[offset] = byte(v >> 8)
						offset++
					} else {
						binary.LittleEndian.PutUint16(raw[offset:], v)
						offset += 2
					}
				}
				alphas[i] = c.A
			}
		}
	}
	converted, err := transformPixels(raw, profile, model, b.Dx(), b.Dy(), alphas)
	if len(profile) > 0 && err != nil && (strings.Contains(err.Error(), "invalid color profile") || strings.Contains(err.Error(), "cannot create color transform")) {
		return img, nil
	}
	return converted, err
}
func transformPixels(raw, profile []byte, model uint32, w, h int, alphas []uint16) (image.Image, error) {
	var input bytes.Buffer
	for _, v := range []uint32{uint32(len(profile)), model, uint32(w * h)} {
		_ = binary.Write(&input, binary.LittleEndian, v)
	}
	input.Write(profile)
	input.Write(raw)
	rgb, err := colorRuntime.run(colorWASM, input.Bytes(), "ICC transform")
	if err != nil {
		return nil, err
	}
	if len(rgb) != w*h*6 {
		return nil, errors.New("invalid ICC transform output")
	}
	out := image.NewNRGBA64(image.Rect(0, 0, w, h))
	for i := 0; i < w*h; i++ {
		for c := 0; c < 3; c++ {
			value := binary.LittleEndian.Uint16(rgb[i*6+c*2:])
			binary.BigEndian.PutUint16(out.Pix[i*8+c*2:], value)
		}
		a := uint16(65535)
		if len(alphas) > i {
			a = alphas[i]
		}
		binary.BigEndian.PutUint16(out.Pix[i*8+6:], a)
	}
	return out, nil
}
func embeddedICC(data []byte) []byte {
	if bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")) {
		for p := 8; p+12 <= len(data); {
			n := int(binary.BigEndian.Uint32(data[p:]))
			if n > len(data)-p-12 {
				break
			}
			if string(data[p+4:p+8]) == "iCCP" {
				chunk := data[p+8 : p+8+n]
				i := bytes.IndexByte(chunk, 0)
				if i < 0 || i+2 > len(chunk) || chunk[i+1] != 0 {
					return nil
				}
				r, e := zlib.NewReader(bytes.NewReader(chunk[i+2:]))
				if e != nil {
					return nil
				}
				defer r.Close()
				b, e := io.ReadAll(io.LimitReader(r, MaxBytes+1))
				if e == nil && len(b) <= MaxBytes {
					return b
				}
				return nil
			}
			p += 12 + n
		}
	}
	if bytes.HasPrefix(data, []byte("RIFF")) {
		for p := 12; p+8 <= len(data); {
			n := int(binary.LittleEndian.Uint32(data[p+4:]))
			if n > len(data)-p-8 {
				break
			}
			if string(data[p:p+4]) == "ICCP" {
				return data[p+8 : p+8+n]
			}
			p += 8 + n + (n % 2)
		}
	}
	if len(data) > 2 && data[0] == 255 && data[1] == 216 {
		type part struct {
			index, total int
			data         []byte
		}
		parts := []part{}
		for p := 2; p+4 <= len(data); {
			if data[p] != 255 {
				break
			}
			marker := data[p+1]
			p += 2
			if marker == 0xda || marker == 0xd9 {
				break
			}
			if marker == 0xff || marker == 0 || marker >= 0xd0 && marker <= 0xd7 {
				continue
			}
			n := int(binary.BigEndian.Uint16(data[p:]))
			if n < 2 || p+n > len(data) {
				break
			}
			if marker == 0xe2 && n >= 16 && bytes.Equal(data[p+2:p+14], []byte("ICC_PROFILE\x00")) {
				parts = append(parts, part{int(data[p+14]), int(data[p+15]), data[p+16 : p+n]})
			}
			p += n
		}
		if len(parts) > 0 {
			sort.Slice(parts, func(i, j int) bool { return parts[i].index < parts[j].index })
			var out []byte
			for i, p := range parts {
				if p.index != i+1 || p.total != len(parts) {
					return nil
				}
				out = append(out, p.data...)
			}
			return out
		}
	}
	return nil
}
