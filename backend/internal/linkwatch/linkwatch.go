// Package linkwatch periodically checks configured friend links and maintains
// their cached website screenshots.
package linkwatch

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"blog/internal/config"
	"blog/internal/store"
	tmpl "blog/internal/template"
)

const (
	DefaultCheckInterval    = 360 * time.Minute
	DefaultSnapshotInterval = 1440 * time.Minute
	maxSnapshotBytes        = 2 << 20
)

// Result is the persisted result of one link probe.
type Result struct {
	ID     int64 `json:"id"`
	OK     bool  `json:"ok"`
	Status int   `json:"status"`
}

// Watcher owns the periodic link check and snapshot work.
type Watcher struct {
	store     *store.Store
	site      *atomic.Pointer[tmpl.Site]
	uploadDir string
	probe     *http.Client
	snapshot  *http.Client
	sem       chan struct{}
}

// New creates a watcher. Configuration is read from site on every cycle.
func New(st *store.Store, site *atomic.Pointer[tmpl.Site], uploadDir string) *Watcher {
	return &Watcher{
		store:     st,
		site:      site,
		uploadDir: uploadDir,
		probe:     &http.Client{Timeout: 8 * time.Second},
		snapshot:  &http.Client{Timeout: 15 * time.Second},
		sem:       make(chan struct{}, 10),
	}
}

// Start launches the watcher loop. A cycle runs immediately, then sleeps for
// the currently configured interval before re-reading configuration.
func (w *Watcher) Start() {
	go func() {
		for {
			cfg := w.site.Load().Friends
			if cfg.CheckInterval > 0 {
				if err := w.RunOnce(); err != nil {
					log.Printf("linkwatch: check cycle failed: %v", err)
				}
				time.Sleep(time.Duration(cfg.CheckInterval) * time.Minute)
				continue
			}
			time.Sleep(time.Minute)
		}
	}()
}

// RunOnce performs one complete check/snapshot cycle. It is exported for
// deterministic tests and for callers that need an immediate refresh.
func (w *Watcher) RunOnce() error {
	if w.site.Load().Friends.CheckInterval <= 0 {
		return nil
	}
	links, err := w.store.ListLinks()
	if err != nil {
		return err
	}
	results := make([]Result, len(links))
	var wg sync.WaitGroup
	for i, link := range links {
		if link.URL == "" {
			results[i] = Result{ID: link.ID}
			continue
		}
		wg.Add(1)
		w.sem <- struct{}{}
		go func(index int, id int64, url string) {
			defer wg.Done()
			defer func() { <-w.sem }()
			ok, status := ProbeURLWithClient(w.probe, url)
			results[index] = Result{ID: id, OK: ok, Status: status}
		}(i, link.ID, link.URL)
	}
	wg.Wait()

	now := time.Now().Unix()
	for i, result := range results {
		links[i].LastStatus = result.Status
		links[i].LastCheckedAt = now
		if err := w.store.UpdateLinkCheck(result.ID, result.Status, now); err != nil {
			return err
		}
	}
	tagged, restored := ApplyExpiredLabels(w.store, links, results)
	siteChanged := tagged+restored > 0

	cfg := w.site.Load().Friends
	if cfg.SnapshotInterval > 0 && cfg.SnapshotProvider != "" {
		for i, result := range results {
			if !result.OK || !snapshotExpired(links[i], w.uploadDir, time.Duration(cfg.SnapshotInterval)*time.Minute) {
				continue
			}
			path, err := w.capture(links[i], cfg.SnapshotProvider)
			if err != nil {
				log.Printf("linkwatch: snapshot %d failed: %v", links[i].ID, err)
				continue
			}
			if err := w.store.UpdateLinkSnapshot(links[i].ID, path); err != nil {
				return err
			}
			siteChanged = true
		}
	}
	if siteChanged {
		return config.RebuildSite(w.store, w.site)
	}
	return nil
}

// Snapshot immediately captures one link, regardless of the periodic
// snapshot interval, and returns its refreshed DB record.
func (w *Watcher) Snapshot(id int64) (*store.Link, error) {
	link, err := w.store.GetLink(id)
	if err != nil {
		return nil, err
	}
	if link == nil {
		return nil, fmt.Errorf("link %d not found", id)
	}
	provider := w.site.Load().Friends.SnapshotProvider
	if provider == "" {
		provider = tmpl.DefaultSnapshotProvider
	}
	path, err := w.capture(link, provider)
	if err != nil {
		return nil, err
	}
	if err := w.store.UpdateLinkSnapshot(id, path); err != nil {
		return nil, err
	}
	if err := config.RebuildSite(w.store, w.site); err != nil {
		return nil, err
	}
	return w.store.GetLink(id)
}

func (w *Watcher) capture(link *store.Link, provider string) (string, error) {
	if !strings.Contains(provider, "{url}") {
		return "", fmt.Errorf("snapshot provider must contain {url}")
	}
	providerURL := strings.ReplaceAll(provider, "{url}", link.URL)
	resp, err := w.snapshot.Get(providerURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("provider returned HTTP %d", resp.StatusCode)
	}
	if contentType := strings.ToLower(resp.Header.Get("Content-Type")); !strings.HasPrefix(contentType, "image/") {
		return "", fmt.Errorf("provider returned non-image content type %q", contentType)
	}
	if resp.ContentLength > maxSnapshotBytes {
		return "", fmt.Errorf("snapshot exceeds %d bytes", maxSnapshotBytes)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxSnapshotBytes+1))
	if err != nil {
		return "", err
	}
	if len(body) > maxSnapshotBytes {
		return "", fmt.Errorf("snapshot exceeds %d bytes", maxSnapshotBytes)
	}

	dir := filepath.Join(w.uploadDir, "snapshots")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(dir, fmt.Sprintf(".%d-*.tmp", link.ID))
	if err != nil {
		return "", err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return "", err
	}
	if _, err := tmp.Write(body); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	finalName := filepath.Join(dir, fmt.Sprintf("%d.jpg", link.ID))
	if err := os.Rename(tmpName, finalName); err != nil {
		return "", err
	}
	return "/uploads/snapshots/" + fmt.Sprintf("%d.jpg", link.ID), nil
}

func snapshotExpired(link *store.Link, uploadDir string, interval time.Duration) bool {
	if link.Snapshot == "" {
		return true
	}
	if !strings.HasPrefix(link.Snapshot, "/uploads/") {
		return true
	}
	rel := strings.TrimPrefix(link.Snapshot, "/uploads/")
	root, err := filepath.Abs(uploadDir)
	if err != nil {
		return true
	}
	path, err := filepath.Abs(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return true
	}
	withinRoot, err := filepath.Rel(root, path)
	if err != nil || withinRoot == ".." || strings.HasPrefix(withinRoot, ".."+string(filepath.Separator)) {
		return true
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return true
	}
	return time.Since(info.ModTime()) >= interval
}

// ProbeURL checks a friend URL with HEAD and falls back to GET for servers
// that reject HEAD. 2xx and 3xx are considered reachable.
func ProbeURL(url string) (bool, int) {
	return ProbeURLWithClient(&http.Client{Timeout: 8 * time.Second}, url)
}

func ProbeURLWithClient(client *http.Client, url string) (bool, int) {
	req, err := http.NewRequest(http.MethodHead, url, nil)
	if err != nil {
		return false, 0
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; StellarBlog/1.0)")
	resp, err := client.Do(req)
	if err != nil {
		req, err = http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return false, 0
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; StellarBlog/1.0)")
		resp, err = client.Do(req)
		if err != nil {
			return false, 0
		}
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 400, resp.StatusCode
}

const (
	ExpiredLabel = "失效"
	ExpiredColor = "EF4444"
)

// ApplyExpiredLabels maintains the automatic "失效" label.
func ApplyExpiredLabels(st *store.Store, links []*store.Link, results []Result) (tagged, restored int) {
	for i, result := range results {
		if i >= len(links) {
			break
		}
		link := links[i]
		switch {
		case !result.OK && strings.TrimSpace(link.Label) == "":
			link.Label, link.Color = ExpiredLabel, ExpiredColor
			if err := st.UpdateLink(link.ID, link); err == nil {
				tagged++
			}
		case result.OK && link.Label == ExpiredLabel:
			link.Label, link.Color = "", ""
			if err := st.UpdateLink(link.ID, link); err == nil {
				restored++
			}
		}
	}
	return tagged, restored
}
