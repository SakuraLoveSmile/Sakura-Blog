package stellarimages

import (
	"bytes"
	"context"
	"embed"
	"encoding/binary"
	"fmt"
	"image"
	"io/fs"
	"sync"
	"time"

	"blog/internal/stellarimages/wasmffi"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

//go:embed native/librsvg.wasm
var svgWASM []byte

//go:embed native/svg/NotoSans.ttf native/svg/NotoSansSC.ttf
var svgFonts embed.FS

//go:embed native/librsvg/fonts.conf
var svgFontConfig embed.FS

//go:embed native/color.wasm
var colorWASM []byte
var colorRuntime wasmDecoder

type wasmDecoder struct {
	once    sync.Once
	runtime wazero.Runtime
	module  wazero.CompiledModule
	err     error
}

func (d *wasmDecoder) run(wasm, data []byte, label string) ([]byte, error) {
	d.once.Do(func() {
		ctx := context.Background()
		d.runtime = wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfig().WithMemoryLimitPages(16384).WithCloseOnContextDone(true))
		_, d.err = wasi_snapshot_preview1.Instantiate(ctx, d.runtime)
		if d.err == nil {
			d.module, d.err = d.runtime.CompileModule(ctx, wasm)
		}
	})
	if d.err != nil {
		return nil, d.err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var stdout, stderr bytes.Buffer
	module, err := d.runtime.InstantiateModule(ctx, d.module, wazero.NewModuleConfig().WithName("").WithStdin(bytes.NewReader(data)).WithStdout(&stdout).WithStderr(&stderr))
	if module != nil {
		defer module.Close(ctx)
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %s (%w)", label, stderr.String(), err)
	}
	return stdout.Bytes(), nil
}

var svgRuntime struct {
	once    sync.Once
	runtime wazero.Runtime
	module  wazero.CompiledModule
	err     error
}

func decodeSVG(data []byte) (image.Image, *Dimensions, error) {
	svgRuntime.once.Do(func() {
		ctx := context.Background()
		svgRuntime.runtime = wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfig().WithMemoryLimitPages(8192).WithCloseOnContextDone(true))
		_, svgRuntime.err = wasi_snapshot_preview1.Instantiate(ctx, svgRuntime.runtime)
		if svgRuntime.err == nil {
			svgRuntime.err = wasmffi.Install(ctx, svgRuntime.runtime)
		}
		if svgRuntime.err == nil {
			var wasm []byte
			wasm, svgRuntime.err = wasmffi.ExportScalarFunctions(svgWASM)
			if svgRuntime.err == nil {
				svgRuntime.module, svgRuntime.err = svgRuntime.runtime.CompileModule(ctx, wasm)
			}
		}
	})
	if svgRuntime.err != nil {
		return nil, nil, svgRuntime.err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var stdout, stderr bytes.Buffer
	fonts, err := fs.Sub(svgFonts, "native/svg")
	if err != nil {
		return nil, nil, err
	}
	config, err := fs.Sub(svgFontConfig, "native/librsvg")
	if err != nil {
		return nil, nil, err
	}
	files := wazero.NewFSConfig().WithFSMount(fonts, "/fonts").WithFSMount(config, "/etc/fonts")
	module, err := svgRuntime.runtime.InstantiateModule(ctx, svgRuntime.module, wazero.NewModuleConfig().WithName("").WithFSConfig(files).WithEnv("FONTCONFIG_FILE", "/etc/fonts/fonts.conf").WithEnv("TZ", "UTC").WithEnv("LANG", "C.UTF-8").WithStdin(bytes.NewReader(data)).WithStdout(&stdout).WithStderr(&stderr))
	if module != nil {
		defer module.Close(ctx)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("SVG rendering: %s (%w)", stderr.String(), err)
	}
	b := stdout.Bytes()
	if len(b) < 16 {
		return nil, nil, fmt.Errorf("SVG renderer returned no dimensions")
	}
	ow, oh := int(binary.LittleEndian.Uint32(b)), int(binary.LittleEndian.Uint32(b[4:]))
	w, h := int(binary.LittleEndian.Uint32(b[8:])), int(binary.LittleEndian.Uint32(b[12:]))
	if ow <= 0 || oh <= 0 || ow > MaxPixels/oh || w <= 0 || h <= 0 || w > SampleSize || h > SampleSize || len(b)-16 != w*h*4 {
		return nil, nil, fmt.Errorf("invalid SVG renderer output")
	}
	return &image.RGBA{Pix: bytes.Clone(b[16:]), Stride: w * 4, Rect: image.Rect(0, 0, w, h)}, &Dimensions{Status: "ready", Width: ow, Height: oh}, nil
}
