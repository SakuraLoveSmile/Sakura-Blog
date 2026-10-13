package wasmffi

import (
	"bytes"
	"context"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"os"
	"testing"
	"time"
)

func TestOfficialLibFFIScalarRoundTrip(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	r := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfig().WithCloseOnContextDone(true))
	defer r.Close(ctx)
	if _, e := wasi_snapshot_preview1.Instantiate(ctx, r); e != nil {
		t.Fatal(e)
	}
	if e := Install(ctx, r); e != nil {
		t.Fatal(e)
	}
	data, e := os.ReadFile("testdata/scalar.wasm")
	if e != nil {
		t.Fatal(e)
	}
	data, e = ExportScalarFunctions(data)
	if e != nil {
		t.Fatal(e)
	}
	var stdout bytes.Buffer
	module, e := r.InstantiateWithConfig(ctx, data, wazero.NewModuleConfig().WithStdout(&stdout))
	if module != nil {
		defer module.Close(ctx)
	}
	if e != nil {
		t.Fatal(e)
	}
	if stdout.String() != "ffi scalar round-trip ok\n" {
		t.Fatal(stdout.String())
	}
}

func TestExporterPreservesEveryNonExportSection(t *testing.T) {
	original, e := os.ReadFile("testdata/scalar.wasm")
	if e != nil {
		t.Fatal(e)
	}
	patched, e := ExportScalarFunctions(original)
	if e != nil {
		t.Fatal(e)
	}
	withoutExports := func(wasm []byte) []byte {
		r := reader{b: wasm, p: 8}
		out := append([]byte(nil), wasm[:8]...)
		for r.p < len(r.b) {
			start := r.p
			id := r.b[r.p]
			r.p++
			n, e := r.u32()
			if e != nil {
				t.Fatal(e)
			}
			if e = r.skip(n); e != nil {
				t.Fatal(e)
			}
			if id != 7 {
				out = append(out, r.b[start:r.p]...)
			}
		}
		return out
	}
	if !bytes.Equal(withoutExports(original), withoutExports(patched)) {
		t.Fatal("function instructions, signatures, or data were changed")
	}
}
