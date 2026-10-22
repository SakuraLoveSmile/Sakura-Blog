package stellar

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"blog/internal/content"
	"blog/internal/settings"
	"blog/internal/stellarimages"
	"blog/internal/stellarmodels"
	"blog/internal/stellarv2"
	"blog/modules/stellar/web"
)

func imageWorkerFixture(t *testing.T, snapshots func(context.Context) ([]stellarimages.PageSnapshot, error)) *imageLifecycle {
	t.Helper()
	root := t.TempDir()
	uploads := filepath.Join(root, "uploads")
	if err := os.Mkdir(uploads, 0700); err != nil {
		t.Fatal(err)
	}
	icons, err := settings.LoadIconsEmbed()
	if err != nil {
		t.Fatal(err)
	}
	engine, err := web.NewEngineFromEmbed(icons)
	if err != nil {
		t.Fatal(err)
	}
	site := &atomic.Pointer[settings.Site]{}
	site.Store(&settings.Site{URL: "https://fixture.test/sub", StellarV2: &settings.StellarV2Config{Theme: stellarv2.DefaultTheme()}})
	return newImageLifecycle(Dependencies{Root: root, UploadDir: uploads, Icons: icons, Engine: engine, Site: site, Events: content.NewEvents()}, snapshots)
}

func imageFixturePNG(t *testing.T, path string) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 7, 5))
	for y := 0; y < 5; y++ {
		for x := 0; x < 7; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: 255, A: 255})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func writeImageCache(t *testing.T, path string, cache stellarimages.Store) {
	t.Helper()
	b, err := json.Marshal(cache)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestImageLifecyclePreparesLocalAssetAndReusesCacheWhenDisabled(t *testing.T) {
	pages := []stellarimages.PageSnapshot{{Path: "post/item/index.html", BaseURL: "https://fixture.test/sub/post/item", HTML: `<article class="md-text content"><img src="../uploads/a%20b.png"></article>`}}
	calls := 0
	w := imageWorkerFixture(t, func(context.Context) ([]stellarimages.PageSnapshot, error) { calls++; return pages, nil })
	path := filepath.Join(w.deps.UploadDir, "a b.png")
	original := imageFixturePNG(t, path)
	if err := w.prepare(context.Background()); err != nil {
		t.Fatal(err)
	}
	cache, err := stellarimages.ReadStore(w.processor.StorePath)
	if err != nil {
		t.Fatal(err)
	}
	entry := cache.Images["https://fixture.test/sub/uploads/a%20b.png"]
	if entry.Dimensions == nil || entry.Dimensions.Width != 7 || entry.Dimensions.Height != 5 || entry.Color == nil || entry.Color.Status != "ready" {
		t.Fatalf("local image not analyzed: %+v", cache)
	}
	result, err := enrichImageBatch(pages, cache, stellarimages.EnrichOptions{})
	if err != nil || !strings.Contains(result[0].HTML, `width="7"`) || !strings.Contains(result[0].HTML, "data-stellar-image-color") {
		t.Fatalf("same batch not enriched: %v %+v", err, result)
	}
	got, _ := os.ReadFile(path)
	if !bytes.Equal(got, original) {
		t.Fatal("original image modified")
	}
	site := *w.deps.Site.Load()
	v2 := *site.StellarV2
	v2.Theme = stellarv2.DefaultTheme()
	v2.Theme["features"].(map[string]any)["imageOptimization"].(map[string]any)["enabled"] = false
	site.StellarV2 = &v2
	w.deps.Site.Store(&site)
	if err := w.prepare(context.Background()); err != nil || calls != 1 {
		t.Fatalf("disabled automatic scan ran: %v calls=%d", err, calls)
	}
	d := &settings.ViewData{Site: &site, Page: &settings.Page{Layout: "page", Title: "Cached", CanonicalURL: "https://fixture.test/sub/post/item"}, Content: `<img src="../uploads/a%20b.png">`, Stellar: &settings.StellarRenderContext{Version: 2, Profile: "page", Input: stellarmodels.Input{Page: map[string]any{"layout": "page", "permalink": "https://fixture.test/sub/post/item", "content": `<img src="../uploads/a%20b.png">`}, FrontMatter: map[string]any{}}}}
	output, err := w.deps.Engine.Render("layout/base", d)
	if err != nil || !strings.Contains(output, `width="7"`) {
		t.Fatalf("disabled cache not consumed: %v", err)
	}
}

func TestImageLifecycleKeepsOldCacheOnCorruptionAndLockFailure(t *testing.T) {
	pages := []stellarimages.PageSnapshot{{Path: "index.html", BaseURL: "https://fixture.test/sub/", HTML: `<article class="md-text content"><img src="uploads/a.png"></article>`}}
	w := imageWorkerFixture(t, func(context.Context) ([]stellarimages.PageSnapshot, error) { return pages, nil })
	cache := stellarimages.Store{Version: 1, Images: map[string]stellarimages.Entry{"old": {Dimensions: &stellarimages.Dimensions{Status: "ready", Width: 7, Height: 5}}}}
	writeImageCache(t, w.processor.StorePath, cache)
	if !w.reload() {
		t.Fatal("valid cache failed")
	}
	bad := []byte("{broken")
	if err := os.WriteFile(w.processor.StorePath, bad, 0600); err != nil {
		t.Fatal(err)
	}
	if err := w.prepare(context.Background()); err == nil {
		t.Fatal("corrupt cache overwritten")
	}
	got, _ := os.ReadFile(w.processor.StorePath)
	if !bytes.Equal(got, bad) || w.cache.Images["old"].Dimensions.Width != 7 {
		t.Fatal("prior cache or corrupt store lost")
	}
	writeImageCache(t, w.processor.StorePath, cache)
	if err := os.WriteFile(w.processor.StorePath+".lock", []byte("other owner"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := w.prepare(context.Background()); err == nil {
		t.Fatal("foreign lock ignored")
	}
	got, _ = os.ReadFile(w.processor.StorePath + ".lock")
	if string(got) != "other owner" || w.cache.Images["old"].Dimensions.Width != 7 {
		t.Fatal("foreign lock/cache altered")
	}
}

func TestImageLifecycleEventCancelsBatchAndCloseWaits(t *testing.T) {
	pages := []stellarimages.PageSnapshot{{Path: "index.html", BaseURL: "https://fixture.test/sub/", HTML: `<article class="md-text content"><img src="uploads/a.png"></article>`}}
	var batches atomic.Int32
	w := imageWorkerFixture(t, func(context.Context) ([]stellarimages.PageSnapshot, error) { batches.Add(1); return pages, nil })
	started := make(chan struct{}, 3)
	cancelled := make(chan struct{}, 3)
	w.processor.LocalAsset = func(ctx context.Context, _ string) (io.ReadCloser, error) {
		started <- struct{}{}
		<-ctx.Done()
		cancelled <- struct{}{}
		return nil, ctx.Err()
	}
	w.start()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("startup batch not started")
	}
	w.deps.Events.Publish(content.Event{Kind: content.ContentChanged})
	select {
	case <-cancelled:
	case <-time.After(5 * time.Second):
		t.Fatal("event did not cancel old batch")
	}
	// 新批次必须捕获更新后的完整公开快照，不复用旧批次。
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(5 * time.Millisecond)
	defer tick.Stop()
	for batches.Load() < 2 {
		select {
		case <-tick.C:
		case <-deadline.C:
			t.Fatal("content event did not capture new batch")
		}
	}
	w.close()
	select {
	case <-w.done:
	default:
		t.Fatal("close returned while worker alive")
	}
	w.close()
	before := batches.Load()
	w.deps.Events.Publish(content.Event{Kind: content.Restored})
	if batches.Load() != before {
		t.Fatal("closed worker retained event subscription")
	}
}

func TestImageLifecycleRestoreReopensReplacedUploadsDirectory(t *testing.T) {
	var filename atomic.Value
	filename.Store("before.png")
	w := imageWorkerFixture(t, func(context.Context) ([]stellarimages.PageSnapshot, error) {
		return []stellarimages.PageSnapshot{{Path: "index.html", BaseURL: "https://fixture.test/sub/", HTML: `<article class="md-text content"><img src="uploads/` + filename.Load().(string) + `"></article>`}}, nil
	})
	imageFixturePNG(t, filepath.Join(w.deps.UploadDir, "before.png"))
	w.start()
	defer w.close()
	waitReady := func(name string) {
		t.Helper()
		tick := time.NewTicker(5 * time.Millisecond)
		defer tick.Stop()
		deadline := time.NewTimer(5 * time.Second)
		defer deadline.Stop()
		for {
			w.cacheMu.RLock()
			e := w.cache.Images["https://fixture.test/sub/uploads/"+name]
			ready := e.Dimensions != nil && e.Dimensions.Status == "ready"
			w.cacheMu.RUnlock()
			if ready {
				return
			}
			select {
			case <-tick.C:
			case <-deadline.C:
				t.Fatalf("restored image %s not ready", name)
			}
		}
	}
	waitReady("before.png")
	if err := os.Rename(w.deps.UploadDir, w.deps.UploadDir+"-old"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(w.deps.UploadDir, 0700); err != nil {
		t.Fatal(err)
	}
	imageFixturePNG(t, filepath.Join(w.deps.UploadDir, "after.png"))
	filename.Store("after.png")
	w.deps.Events.Publish(content.Event{Kind: content.Restored})
	waitReady("after.png")
}
