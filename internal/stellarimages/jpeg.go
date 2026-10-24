package stellarimages

import (
	"context"
	_ "embed"
	"encoding/binary"
	"errors"
	"image"
)

//go:embed native/jpeg.wasm
var jpegWASM []byte
var jpegRuntime wasmDecoder

func decodeJPEG(ctx context.Context, data []byte) (image.Image, *Dimensions, error) {
	b, e := jpegRuntime.run(ctx, jpegWASM, data, "JPEG decode")
	if e != nil {
		return nil, nil, e
	}
	if len(b) < 20 {
		return nil, nil, errors.New("invalid JPEG output")
	}
	ow, oh, w, h, n := int(binary.LittleEndian.Uint32(b)), int(binary.LittleEndian.Uint32(b[4:])), int(binary.LittleEndian.Uint32(b[8:])), int(binary.LittleEndian.Uint32(b[12:])), int(binary.LittleEndian.Uint32(b[16:]))
	if w <= 0 || h <= 0 || w > MaxPixels/h || n != 3 && n != 4 || len(b)-20 != w*h*n {
		return nil, nil, errors.New("invalid JPEG pixel buffer")
	}
	var img image.Image
	if n == 4 {
		img = &image.CMYK{Pix: b[20:], Stride: w * 4, Rect: image.Rect(0, 0, w, h)}
	} else {
		out := image.NewNRGBA(image.Rect(0, 0, w, h))
		for i := 0; i < w*h; i++ {
			copy(out.Pix[i*4:], b[20+i*3:20+i*3+3])
			out.Pix[i*4+3] = 255
		}
		img = out
	}
	return img, &Dimensions{Status: "ready", Width: ow, Height: oh}, nil
}
