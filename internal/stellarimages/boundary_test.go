package stellarimages

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestDefaultFetchBlocksPrivateTargets(t *testing.T) {
	p := Processor{StorePath: filepath.Join(t.TempDir(), "images.json"), SiteURL: "https://site.test/"}
	r, e := p.Prepare(context.Background(), []PageSnapshot{{Path: "index.html", HTML: `<article class="md-text content"><img src="http://127.0.0.1/private"></article>`}}, PrepareOptions{})
	if e != nil || r.Failed != 1 {
		t.Fatal(r, e)
	}
	s, e := ReadStore(p.StorePath)
	if e != nil || s.Images["http://127.0.0.1/private"].Error == "" {
		t.Fatal(s, e)
	}
}
func TestThreeWorkersAndExclusiveBatchLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "images.json")
	data := fixture(t)
	started := make(chan struct{}, 8)
	release := make(chan struct{})
	var active, maxActive atomic.Int32
	p := Processor{StorePath: path, SiteURL: "https://site.test/", LocalAsset: func(context.Context, string) (io.ReadCloser, error) {
		n := active.Add(1)
		for {
			m := maxActive.Load()
			if n <= m || maxActive.CompareAndSwap(m, n) {
				break
			}
		}
		started <- struct{}{}
		<-release
		active.Add(-1)
		return io.NopCloser(bytes.NewReader(data)), nil
	}}
	done := make(chan error, 1)
	pages := []PageSnapshot{{Path: "index.html", HTML: `<article class="md-text content"><img src="/1"><img src="/2"><img src="/3"><img src="/4"></article>`}}
	go func() { _, e := p.Prepare(context.Background(), pages, PrepareOptions{}); done <- e }()
	for i := 0; i < 3; i++ {
		select {
		case <-started:
		case <-time.After(2 * time.Second):
			close(release)
			t.Fatal("workers did not start")
		}
	}
	if _, e := p.Prepare(context.Background(), pages, PrepareOptions{}); e == nil {
		close(release)
		t.Fatal("second processor acquired batch lock")
	}
	if _, e := os.Stat(path + ".lock"); e != nil {
		close(release)
		t.Fatal("foreign call removed lock", e)
	}
	close(release)
	if e := <-done; e != nil {
		t.Fatal(e)
	}
	if maxActive.Load() != 3 {
		t.Fatal("concurrency", maxActive.Load())
	}
	s, e := ReadStore(path)
	if e != nil || len(s.Images) != 4 {
		t.Fatal(s, e)
	}
	if _, e = os.Stat(path + ".lock"); !errors.Is(e, os.ErrNotExist) {
		t.Fatal("owned lock remains", e)
	}
}
func TestCompleteEntryWinsOverBackoff(t *testing.T) {
	path := filepath.Join(t.TempDir(), "images.json")
	s := Store{Version: 1, Images: map[string]Entry{"https://site.test/a": {Dimensions: &Dimensions{Status: "ready", Width: 2, Height: 3}, Color: &Color{Status: "unsupported", Algorithm: ColorAlgorithm}, Error: "stale", RetryAfter: time.Now().Add(time.Hour).UnixMilli()}}}
	if e := writeStore(path, s); e != nil {
		t.Fatal(e)
	}
	p := Processor{StorePath: path, SiteURL: "https://site.test/"}
	r, e := p.Prepare(context.Background(), []PageSnapshot{{Path: "index.html", HTML: `<article class="md-text content"><img src="/a"></article>`}}, PrepareOptions{})
	if e != nil || r.Skipped != 1 || r.Deferred != 0 || r.Processed != 0 {
		t.Fatal(r, e)
	}
}
func TestSVGTextUsesEmbeddedLatinAndChineseFonts(t *testing.T) {
	blank, e := Analyze([]byte(`<svg xmlns="http://www.w3.org/2000/svg" width="128" height="64"><rect width="128" height="64" fill="white"/></svg>`))
	if e != nil {
		t.Fatal(e)
	}
	text, e := Analyze([]byte(`<svg xmlns="http://www.w3.org/2000/svg" width="128" height="64"><rect width="128" height="64" fill="white"/><text x="0" y="35" font-family="Noto Sans SC" font-size="24" fill="black">Stellar 你好</text></svg>`))
	if e != nil {
		t.Fatal(e)
	}
	if text.Color.HSLA[2] >= blank.Color.HSLA[2]-2 {
		t.Fatalf("text was not rendered: %v versus %v", text.Color.HSLA, blank.Color.HSLA)
	}
}
