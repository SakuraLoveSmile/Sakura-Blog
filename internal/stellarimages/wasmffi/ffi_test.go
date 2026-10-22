package wasmffi

import (
	"bytes"
	"context"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"os"
	"strings"
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

func TestOfficialGLibPlatformAndGObject(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	r := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfig().WithCloseOnContextDone(true))
	defer r.Close(ctx)
	if _, err := wasi_snapshot_preview1.Instantiate(ctx, r); err != nil {
		t.Fatal(err)
	}
	if err := Install(ctx, r); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("testdata/glib.wasm")
	if err != nil {
		t.Fatal(err)
	}
	data, err = ExportScalarFunctions(data)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	module, err := r.InstantiateWithConfig(ctx, data, wazero.NewModuleConfig().WithStdout(&output).WithStderr(&output))
	if module != nil {
		defer module.Close(ctx)
	}
	if err != nil {
		t.Fatalf("%v\n%s", err, output.String())
	}
	if output.String() != "glib platform gobject ffi and gio streams ok\n" {
		t.Fatal(output.String())
	}
	invalid := module.ExportedFunction("test_invalid_type_init")
	if invalid == nil {
		t.Fatal("missing invalid callback fixture")
	}
	if _, err = invalid.Call(ctx); err == nil || !strings.Contains(err.Error(), "type initializer must return void") {
		t.Fatalf("invalid initializer accepted: %v", err)
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
