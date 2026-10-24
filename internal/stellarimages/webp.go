package stellarimages

import (
	"context"
	_ "embed"
	"encoding/binary"
	"errors"
	"image"
)

//go:embed native/webp.wasm
var webpWASM []byte
var webpRuntime wasmDecoder

func decodeWebP(ctx context.Context, data []byte) (image.Image, *Dimensions, error) {
	b, e := webpRuntime.run(ctx, webpWASM, data, "WebP decode")
	if e != nil {
		return nil, nil, e
	}
	if len(b) < 16 {
		return nil, nil, errors.New("invalid WebP output")
	}
	ow, oh, w, h := int(binary.LittleEndian.Uint32(b)), int(binary.LittleEndian.Uint32(b[4:])), int(binary.LittleEndian.Uint32(b[8:])), int(binary.LittleEndian.Uint32(b[12:]))
	if w <= 0 || h <= 0 || w > MaxPixels/h || len(b)-16 != w*h*4 {
		return nil, nil, errors.New("invalid WebP pixel buffer")
	}
	return &image.NRGBA{Pix: b[16:], Stride: w * 4, Rect: image.Rect(0, 0, w, h)}, &Dimensions{Status: "ready", Width: ow, Height: oh}, nil
}
