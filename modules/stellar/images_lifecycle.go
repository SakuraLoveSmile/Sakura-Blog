package stellar

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"sync"

	"blog/internal/content"
	"blog/internal/stellarimages"
	"blog/internal/stellarv2"
	"blog/modules/stellar/clientconfig"
	"blog/modules/stellar/web"
)

// imageLifecycle 只在启动或内容事件后准备整批公开 HTML；GET 只读取内存缓存。
type imageLifecycle struct {
	deps         Dependencies
	cacheMu      sync.RWMutex
	cache        stellarimages.Store
	mu           sync.Mutex
	closed       bool
	activeCancel context.CancelFunc
	cancel       context.CancelFunc
	requests     chan struct{}
	done         chan struct{}
	sub          content.Subscription
	processor    stellarimages.Processor
	snapshots    func(context.Context) ([]stellarimages.PageSnapshot, error)
}

func newImageLifecycle(deps Dependencies, snapshots func(context.Context) ([]stellarimages.PageSnapshot, error)) *imageLifecycle {
	worker := &imageLifecycle{deps: deps, cache: stellarimages.Store{Version: 1, Images: map[string]stellarimages.Entry{}}, requests: make(chan struct{}, 1), done: make(chan struct{}), snapshots: snapshots}
	worker.processor.StorePath = filepath.Join(deps.Root, "content", "_data", "caches", "images_metadata.json")
	worker.reload()
	deps.Engine.SetStellarImageEnricher(func(source string, data *web.StellarRenderData) (string, error) {
		worker.cacheMu.RLock()
		defer worker.cacheMu.RUnlock()
		return web.EnrichStellarImages(source, data, worker.cache)
	})
	return worker
}

func (w *imageLifecycle) reload() bool {
	cache, err := stellarimages.ReadStore(w.processor.StorePath)
	if err != nil {
		log.Printf("stellar images: metadata unavailable; keeping previous cache: %v", err)
		return false
	}
	w.cacheMu.Lock()
	w.cache = cache
	w.cacheMu.Unlock()
	return true
}

func (w *imageLifecycle) start() {
	ctx, cancel := context.WithCancel(context.Background())
	w.cancel = cancel
	if w.deps.Events != nil {
		w.sub = w.deps.Events.Subscribe(func(content.Event) { w.trigger() })
	}
	go func() {
		defer close(w.done)
		for {
			select {
			case <-ctx.Done():
				return
			case <-w.requests:
				w.mu.Lock()
				batchCtx, cancel := context.WithCancel(ctx)
				w.activeCancel = cancel
				w.mu.Unlock()
				if err := w.prepare(batchCtx); err != nil && batchCtx.Err() == nil {
					log.Printf("stellar images: automatic preparation failed; keeping previous cache: %v", err)
				}
				cancel()
			}
		}
	}()
	w.trigger()
}

func (w *imageLifecycle) trigger() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return
	}
	if w.activeCancel != nil {
		w.activeCancel()
	}
	select {
	case w.requests <- struct{}{}:
	default:
	}
}

func (w *imageLifecycle) prepare(ctx context.Context) error {
	if !w.reload() {
		return fmt.Errorf("invalid persisted image metadata")
	}
	site := w.deps.Site.Load()
	if site == nil || site.StellarV2 == nil {
		return nil
	}
	if enabled, _ := stellarv2.Get(site.StellarV2.Theme, "features.imageOptimization.enabled"); enabled != true {
		return nil
	}
	processor := w.processor
	if processor.LocalAsset == nil {
		read, closeAssets, err := ToolLocalAssets(w.deps.UploadDir, w.deps.Icons)
		if err != nil {
			return err
		}
		defer closeAssets()
		processor.LocalAsset = read
	}
	pages, err := w.snapshots(ctx)
	if err != nil {
		return err
	}
	processor.SiteURL = site.URL
	report, err := processor.Prepare(ctx, pages, stellarimages.PrepareOptions{})
	if err != nil {
		return err
	}
	cache, err := stellarimages.ReadStore(processor.StorePath)
	if err != nil {
		return err
	}
	var auto *bool
	if value, _ := stellarv2.Get(site.StellarV2.Theme, "features.lazyLoading.autoAspectRatio"); value != nil {
		if flag, ok := value.(bool); ok {
			auto = &flag
		}
	}
	// 对捕获的同一批 HTML 回填并更新签名，确认派生结果可发布再替换请求缓存。
	if _, err := enrichImageBatch(pages, cache, stellarimages.EnrichOptions{AutoAspectRatio: auto}); err != nil {
		return err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	w.cacheMu.Lock()
	w.cache = cache
	w.cacheMu.Unlock()
	if report.Failed > 0 {
		log.Printf("stellar images: %d image preparations failed; prior metadata retained", report.Failed)
	}
	return nil
}

func enrichImageBatch(pages []stellarimages.PageSnapshot, cache stellarimages.Store, options stellarimages.EnrichOptions) ([]stellarimages.PageSnapshot, error) {
	result := append([]stellarimages.PageSnapshot(nil), pages...)
	for i, page := range pages {
		if page.BaseURL == "" {
			return nil, fmt.Errorf("SSR image snapshot is missing absolute BaseURL: %s", page.Path)
		}
		body, err := stellarimages.Enrich(page.HTML, page.BaseURL, cache, options)
		if err != nil {
			return nil, err
		}
		body, err = clientconfig.RefreshNavigationSignature(body)
		if err != nil {
			return nil, err
		}
		result[i].HTML = body
	}
	return result, nil
}

func (w *imageLifecycle) close() {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return
	}
	w.closed = true
	if w.activeCancel != nil {
		w.activeCancel()
	}
	w.cancel()
	w.mu.Unlock()
	if w.sub != nil {
		w.sub.Close()
	}
	<-w.done
}
