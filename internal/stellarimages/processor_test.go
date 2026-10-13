package stellarimages

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func fixture(t *testing.T) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 20, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 20; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: 18, G: 52, B: 86, A: 255})
		}
	}
	var b bytes.Buffer
	if e := png.Encode(&b, img); e != nil {
		t.Fatal(e)
	}
	return b.Bytes()
}
func TestAnalyzePNGAndSVG(t *testing.T) {
	for _, data := range [][]byte{fixture(t), []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="20" height="10"><path fill="#123456" d="M0 0h20v10H0z"/></svg>`)} {
		a, e := Analyze(data)
		if e != nil {
			t.Fatal(e)
		}
		if a.Dimensions.Width != 20 || a.Dimensions.Height != 10 || !reflect.DeepEqual(a.Color.HSLA, []float64{210, 65.3846, 20.3922, 1}) {
			t.Fatalf("%+v %+v", a.Dimensions, a.Color)
		}
	}
}
func TestDiscoveryWHATWGAndScope(t *testing.T) {
	raw := `<article class="md-text content"><img src="../a.png?a=1&amp;b=2"><img data-src="//cdn.test/x"><script>const x="<img src=bad>"</script><img src="data:image/png;base64,AA"><img src="https:\\CDN.TEST\x"></article><aside><img src="/sidebar"></aside><div class="wiki-hero"><img src="/hero"></div>`
	got, e := Collect(raw, "https://site.test/posts/one/")
	want := []string{"https://site.test/posts/a.png?a=1&b=2", "https://cdn.test/x", "https://site.test/hero"}
	if e != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("%v %v", got, e)
	}
}
func TestDryRunCacheRefreshAndEnrich(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "images_metadata.json")
	var calls atomic.Int32
	data := fixture(t)
	p := Processor{StorePath: path, SiteURL: "https://site.test/blog/", LocalAsset: func(_ context.Context, route string) (io.ReadCloser, error) {
		calls.Add(1)
		if route != "asset%20one.png" {
			t.Fatalf("escaped path changed: %q", route)
		}
		return io.NopCloser(bytes.NewReader(data)), nil
	}}
	pages := []PageSnapshot{{Path: "post/index.html", HTML: `<article class="md-text content"><img src="../asset%20one.png"><img src="../asset%20one.png"></article>`}}
	pages[0].HTML = strings.ReplaceAll(pages[0].HTML, "../asset", "/blog/asset")
	r, e := p.Prepare(context.Background(), pages, PrepareOptions{DryRun: true})
	if e != nil || len(r.Missing) != 1 || calls.Load() != 0 {
		t.Fatal(r, e)
	}
	if _, e = os.Stat(filepath.Dir(path)); !errors.Is(e, os.ErrNotExist) {
		t.Fatal("dry-run wrote directory")
	}
	r, e = p.Prepare(context.Background(), pages, PrepareOptions{})
	if e != nil || r.Processed != 1 || r.Failed != 0 || calls.Load() != 1 {
		t.Fatal(r, e)
	}
	r, e = p.Prepare(context.Background(), pages, PrepareOptions{})
	if e != nil || r.Skipped != 1 || calls.Load() != 1 {
		t.Fatal(r, e)
	}
	r, e = p.Prepare(context.Background(), pages, PrepareOptions{Refresh: "/blog/asset%20one.png"})
	if e != nil || r.Processed != 1 || calls.Load() != 2 {
		t.Fatal(r, e)
	}
	s, e := ReadStore(path)
	if e != nil {
		t.Fatal(e)
	}
	out, e := Enrich(pages[0].HTML, "https://site.test/blog/post/", s, EnrichOptions{})
	if e != nil || !strings.Contains(out, `width="20"`) || !strings.Contains(out, "data-stellar-image-color") {
		t.Fatal(out, e)
	}
}

func TestEmptyDryRunReportJSON(t *testing.T) {
	p := Processor{StorePath: filepath.Join(t.TempDir(), "nested", "images.json"), SiteURL: "https://site.test/"}
	for _, dryRun := range []bool{true, false} {
		r, err := p.Prepare(context.Background(), nil, PrepareOptions{DryRun: dryRun})
		if err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		var fields map[string]json.RawMessage
		if err = json.Unmarshal(data, &fields); err != nil {
			t.Fatal(err)
		}
		if missing, present := fields["missing"]; present != dryRun || dryRun && string(missing) != "[]" {
			t.Fatalf("dry-run=%v: %s", dryRun, data)
		}
	}
}

func TestSnapshotBaseUsesRenderedURL(t *testing.T) {
	p := Processor{StorePath: filepath.Join(t.TempDir(), "images.json"), SiteURL: "https://site.test/blog/"}
	for _, tc := range []struct{ base, image string }{
		{"", "https://site.test/blog/post/foo/image.png"},
		{"https://site.test/blog/post/foo", "https://site.test/blog/post/image.png"},
	} {
		pages := []PageSnapshot{{Path: "post/foo/index.html", BaseURL: tc.base, HTML: `<article class="md-text content"><img src="image.png"></article>`}}
		report, err := p.Prepare(context.Background(), pages, PrepareOptions{Page: "/post/foo/", DryRun: true})
		if err != nil || !reflect.DeepEqual(report.Missing, []string{tc.image}) {
			t.Fatalf("base=%q: %+v %v", tc.base, report, err)
		}
	}
}

func TestInvalidSnapshotBaseDoesNotWrite(t *testing.T) {
	for _, base := range []string{"/post/foo", "https://", "https:site.test/post/foo", "ftp://site.test/post/foo"} {
		t.Run(base, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "new")
			p := Processor{StorePath: filepath.Join(dir, "images.json"), SiteURL: "https://site.test/", LocalAsset: func(context.Context, string) (io.ReadCloser, error) {
				t.Fatal("invalid base fetched an image")
				return nil, nil
			}}
			_, err := p.Prepare(context.Background(), []PageSnapshot{{Path: "post/foo/index.html", BaseURL: base}}, PrepareOptions{})
			if err == nil {
				t.Fatal("invalid base accepted")
			}
			if _, err = os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("invalid base wrote directory: %v", err)
			}
		})
	}
}
func TestFailureRetainsOldResultsAndBackoff(t *testing.T) {
	path := filepath.Join(t.TempDir(), "images.json")
	now := time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)
	p := Processor{StorePath: path, SiteURL: "https://site.test/", Now: func() time.Time { return now }, LocalAsset: func(context.Context, string) (io.ReadCloser, error) { return nil, errors.New("offline") }}
	s := Store{Version: 1, Images: map[string]Entry{"https://site.test/a": {Dimensions: &Dimensions{Status: "ready", Width: 2, Height: 3}}}}
	if e := writeStore(path, s); e != nil {
		t.Fatal(e)
	}
	pages := []PageSnapshot{{Path: "index.html", HTML: `<article class="md-text content"><img src="/a"></article>`}}
	r, e := p.Prepare(context.Background(), pages, PrepareOptions{})
	if e != nil || r.Failed != 1 {
		t.Fatal(r, e)
	}
	s, e = ReadStore(path)
	if e != nil || s.Images["https://site.test/a"].Dimensions.Width != 2 || s.Images["https://site.test/a"].RetryAfter != now.Add(24*time.Hour).UnixMilli() {
		t.Fatal(s, e)
	}
	r, e = p.Prepare(context.Background(), pages, PrepareOptions{})
	if e != nil || r.Deferred != 1 {
		t.Fatal(r, e)
	}
	r, e = p.Prepare(context.Background(), nil, PrepareOptions{Refresh: "/a"})
	if e != nil || r.Failed != 1 {
		t.Fatal(r, e)
	}
}
func TestInvalidStoreAndForeignLockNeverOverwritten(t *testing.T) {
	path := filepath.Join(t.TempDir(), "images.json")
	p := Processor{StorePath: path, SiteURL: "https://site.test/"}
	if e := os.WriteFile(path, []byte("broken"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e := p.Prepare(context.Background(), nil, PrepareOptions{}); e == nil {
		t.Fatal("corrupt store accepted")
	}
	b, _ := os.ReadFile(path)
	if string(b) != "broken" {
		t.Fatal(string(b))
	}
	if e := os.WriteFile(path+".lock", []byte("other writer"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e := p.Prepare(context.Background(), nil, PrepareOptions{}); e == nil {
		t.Fatal("lock accepted")
	}
	b, _ = os.ReadFile(path + ".lock")
	if string(b) != "other writer" {
		t.Fatal("foreign lock removed")
	}
}
func TestEnrichPreservesAuthorDimensionsAndMovesTagStyles(t *testing.T) {
	s := Store{Version: 1, Images: map[string]Entry{"https://site.test/a": {Dimensions: &Dimensions{Status: "ready", Width: 20, Height: 10}, Color: &Color{Status: "ready", Algorithm: ColorAlgorithm, Size: 64, HSLA: []float64{210, 65, 20, 1}}}}}
	raw := `<article class="md-text content"><div class="tag-plugin image"><div class="image-bg"><img src="/a" style="width:100px;height:50px"></div></div><img src="/a" width="123"><aside><img src="/b"></aside></article>`
	out, e := Enrich(raw, "https://site.test/", s, EnrichOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(out, `width="123"`) || !strings.Contains(out, "aspect-ratio:20/10") || !strings.Contains(out, "width:100px") {
		t.Fatal(out)
	}
	root, _, _ := parseHTML(out)
	for _, n := range imageNodes(root) {
		if attr(n, "data-stellar-image-color") != "" {
			var c struct{ HSLA []float64 }
			if e = json.Unmarshal([]byte(attr(n, "data-stellar-image-color")), &c); e != nil || len(c.HSLA) != 4 {
				t.Fatal(e)
			}
		}
	}
}
