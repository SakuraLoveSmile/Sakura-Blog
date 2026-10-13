package stellarimages

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/experimental"
)

func TestCancelBatchDuringSVGDecodeRetainsCacheAndRetries(t *testing.T) {
	// 先完成共享编译；本用例只测实际解码期间取消。
	good := []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="20" height="10"><rect width="20" height="10" fill="red"/></svg>`)
	if _, err := Analyze(good); err != nil {
		t.Fatal(err)
	}
	slow := []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="200" height="100">` + strings.Repeat(`<text x="0" y="30" font-size="24">cancel during embedded Pango rendering</text>`, 30000) + `</svg>`)
	path := filepath.Join(t.TempDir(), "images.json")
	u := "https://site.test/a.svg"
	old := Store{Version: 1, Images: map[string]Entry{u: {Dimensions: &Dimensions{Status: "ready", Width: 2, Height: 3}, UpdatedAt: "old"}}}
	if err := writeStore(path, old); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	fetched := make(chan struct{})
	p := Processor{StorePath: path, SiteURL: "https://site.test/", LocalAsset: func(context.Context, string) (io.ReadCloser, error) {
		close(fetched)
		return io.NopCloser(bytes.NewReader(slow)), nil
	}}
	pages := []PageSnapshot{{Path: "index.html", HTML: `<article class="md-text content"><img src="/a.svg"></article>`}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := p.Prepare(ctx, pages, PrepareOptions{}); done <- err }()
	<-fetched
	select {
	case err := <-done:
		t.Fatalf("decode completed before cancellation: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	start := time.Now()
	cancel()
	// 即使回归也等待 worker 清理，以免泄漏真实 WASM 或临时锁。
	err = <-done
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("decode ignored batch cancellation for %s", elapsed)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("cancellation changed cache: %v", err)
	}
	if _, err := os.Stat(path + ".lock"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("cancellation leaked lock: %v", err)
	}
	p.LocalAsset = func(context.Context, string) (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(good)), nil }
	report, err := p.Prepare(context.Background(), pages, PrepareOptions{})
	if err != nil || report.Processed != 1 || report.Failed != 0 || report.Deferred != 0 {
		t.Fatalf("retry: %+v %v", report, err)
	}
}

func TestWASMExecutionCancellationAndDeadlineKeepRuntimeReusable(t *testing.T) {
	// _start 的真实 WASM 指令无限循环；listener 仅证明已进入执行，不替换解码器。
	wasm, err := hex.DecodeString("0061736d0100000001040160000003020100070a01065f737461727400000a0901070003400c000b0b")
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{}, 2)
	compileCtx := experimental.WithFunctionListenerFactory(context.Background(), experimental.FunctionListenerFactoryFunc(func(api.FunctionDefinition) experimental.FunctionListener {
		return experimental.FunctionListenerFunc(func(context.Context, api.Module, api.FunctionDefinition, []uint64, experimental.StackIterator) {
			started <- struct{}{}
		})
	}))
	d := &wasmDecoder{}
	d.once.Do(func() {
		d.ready = make(chan struct{})
		d.runtime = wazero.NewRuntimeWithConfig(context.Background(), wazero.NewRuntimeConfig().WithCloseOnContextDone(true))
		d.module, d.err = d.runtime.CompileModule(compileCtx, wasm)
		close(d.ready)
	})
	defer d.runtime.Close(context.Background())
	if d.err != nil {
		t.Fatal(d.err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := d.run(ctx, wasm, nil, "cancellation test"); done <- err }()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("WASM did not enter execution")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("WASM ignored cancellation")
	}
	// 同一编译缓存第二次确实再次进入 _start，而非 runtime 被首个取消关掉。
	ctx, stop := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer stop()
	_, err = d.run(ctx, wasm, nil, "deadline test")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
	select {
	case <-started:
	default:
		t.Fatal("canceled runtime was not reusable")
	}
}

type compilationWaitContext struct {
	context.Context
	waiting chan struct{}
	once    sync.Once
}

func (c *compilationWaitContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.waiting) })
	return c.Context.Done()
}

func TestCancelColdWASMCompilationWaitDoesNotPoisonCache(t *testing.T) {
	data, err := os.ReadFile("testdata/gradient.jpg")
	if err != nil {
		t.Fatal(err)
	}
	d := &wasmDecoder{}
	batchCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx := &compilationWaitContext{Context: batchCtx, waiting: make(chan struct{})}
	done := make(chan error, 1)
	go func() { _, err := d.run(ctx, jpegWASM, data, "cold JPEG"); done <- err }()
	select {
	case <-ctx.waiting:
	case <-time.After(2 * time.Second):
		t.Fatal("did not start compilation wait")
	}
	start := time.Now()
	cancel()
	err = <-done
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("cold compilation wait ignored cancellation")
	}
	// 编译继续完成并可复用；不把单个调用的取消存入 sync.Once 的永久错误。
	<-d.ready
	defer d.runtime.Close(context.Background())
	result, err := d.run(context.Background(), jpegWASM, data, "retry JPEG")
	if err != nil || len(result) < 20 {
		t.Fatalf("retry: %d bytes, %v", len(result), err)
	}
}
