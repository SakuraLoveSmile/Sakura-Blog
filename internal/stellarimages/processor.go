package stellarimages

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"blog/internal/safefetch"
	whatwg "github.com/nlnwa/whatwg-url/url"
)

type PageSnapshot struct{ Path, HTML, BaseURL string }
type PrepareOptions struct {
	Page, Refresh string
	DryRun        bool
}
type Report struct {
	Found     int      `json:"found"`
	Skipped   int      `json:"skipped"`
	Deferred  int      `json:"deferred"`
	Processed int      `json:"processed"`
	Failed    int      `json:"failed"`
	File      string   `json:"file"`
	Missing   []string `json:"missing,omitempty"`
}

func (r Report) MarshalJSON() ([]byte, error) {
	type fields Report
	var missing *[]string
	if r.Missing != nil {
		missing = &r.Missing
	}
	return json.Marshal(struct {
		fields
		Missing *[]string `json:"missing,omitempty"`
	}{fields: fields(r), Missing: missing})
}

type Processor struct {
	StorePath, SiteURL string
	// LocalAsset 接收相对 SiteURL 路径的 escaped route。
	// 消费端只严格解码一次，并仅解析明确发布的资产。
	LocalAsset func(context.Context, string) (io.ReadCloser, error)
	// Client 是显式隔离测试接口；生产保持 nil，使用 safefetch。
	Client *http.Client
	Now    func() time.Time
}

func (p Processor) now() time.Time {
	if p.Now != nil {
		return p.Now()
	}
	return time.Now()
}
func (p Processor) base(route string) (string, error) {
	base := strings.TrimRight(p.SiteURL, "/") + "/"
	u, err := whatwg.ParseRef(base, route)
	if err != nil || u == nil || (u.Scheme() != "http" && u.Scheme() != "https") {
		return "", errors.New("invalid site URL")
	}
	return u.Href(false), nil
}

func (p Processor) snapshotBase(page PageSnapshot) (string, error) {
	if page.BaseURL == "" {
		return p.base(page.Path)
	}
	parsed, err := url.Parse(page.BaseURL)
	if err != nil || !parsed.IsAbs() || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", errors.New("page base URL must be absolute HTTP(S)")
	}
	u, err := whatwg.Parse(page.BaseURL)
	if err != nil || u == nil {
		return "", errors.New("invalid page base URL")
	}
	return u.Href(false), nil
}
func readBytes(r io.Reader) ([]byte, error) {
	b, e := io.ReadAll(io.LimitReader(r, MaxBytes+1))
	if e != nil {
		return nil, e
	}
	if len(b) > MaxBytes {
		return nil, fmt.Errorf("image exceeds %d bytes", MaxBytes)
	}
	return b, nil
}
func (p Processor) fetch(ctx context.Context, raw string) ([]byte, error) {
	target, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	siteBase, err := p.base("")
	if err != nil {
		return nil, err
	}
	site, _ := url.Parse(siteBase)
	if target.Scheme == site.Scheme && target.Host == site.Host && strings.HasPrefix(target.EscapedPath(), site.EscapedPath()) {
		if p.LocalAsset == nil {
			return nil, errors.New("local asset resolver is not configured")
		}
		route := strings.TrimPrefix(target.EscapedPath(), site.EscapedPath())
		r, e := p.LocalAsset(ctx, route)
		if e != nil {
			return nil, e
		}
		if r == nil {
			return nil, errors.New("local asset resolver returned no stream")
		}
		defer r.Close()
		return readBytes(r)
	}
	if _, err = safefetch.ValidateURL(raw); err != nil {
		return nil, err
	}
	client := p.Client
	if client == nil {
		client = safefetch.New(safefetch.Options{Timeout: 15 * time.Second, MaxRedirects: 10, RedirectValidator: func(u *url.URL) error { _, e := safefetch.ValidateURL(u.String()); return e }})
		defer client.CloseIdleConnections()
	}
	requestCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, raw, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	if resp.ContentLength > MaxBytes {
		return nil, errors.New("image exceeds download limit")
	}
	return readBytes(resp.Body)
}

// Prepare 在整个批次持有独占锁，每图结束后原子保存 v1 缓存。
// DryRun 不创建目录、锁，也不发起请求。
func (p Processor) Prepare(ctx context.Context, pages []PageSnapshot, opts PrepareOptions) (Report, error) {
	report := Report{File: p.StorePath}
	if err := ctx.Err(); err != nil {
		return report, err
	}
	if p.StorePath == "" {
		return report, errors.New("image metadata path is required")
	}
	requested := strings.TrimPrefix(opts.Page, "/")
	if strings.HasSuffix(requested, "/") {
		requested += "index.html"
	}
	pageBases := make([]string, len(pages))
	if opts.Refresh == "" {
		for i, page := range pages {
			if opts.Page != "" && page.Path != requested {
				continue
			}
			var err error
			pageBases[i], err = p.snapshotBase(page)
			if err != nil {
				return report, err
			}
		}
	}
	if !opts.DryRun {
		if err := os.MkdirAll(filepath.Dir(p.StorePath), 0755); err != nil {
			return report, err
		}
		lock, err := os.OpenFile(p.StorePath+".lock", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return report, fmt.Errorf("image metadata is locked: %w", err)
		}
		if err = lock.Close(); err != nil {
			os.Remove(p.StorePath + ".lock")
			return report, err
		}
		defer os.Remove(p.StorePath + ".lock")
	}
	store, err := ReadStore(p.StorePath)
	if err != nil {
		return report, err
	}
	base, err := p.base("")
	if err != nil {
		return report, err
	}
	urls := []string{}
	seen := map[string]bool{}
	if opts.Refresh != "" {
		u := imageURL(opts.Refresh, base)
		if u == "" {
			return report, errors.New("refresh requires an image URL")
		}
		urls = append(urls, u)
	} else {
		matched := opts.Page == ""
		for i, page := range pages {
			if opts.Page != "" && page.Path != requested {
				continue
			}
			matched = true
			images, e := Collect(page.HTML, pageBases[i])
			if e != nil {
				return report, e
			}
			for _, u := range images {
				if !seen[u] {
					seen[u] = true
					urls = append(urls, u)
				}
			}
		}
		if !matched {
			return report, fmt.Errorf("page route not found: %s", requested)
		}
	}
	report.Found = len(urls)
	queue := []string{}
	for _, u := range urls {
		e := store.Images[u]
		if opts.Refresh == "" && complete(e) {
			report.Skipped++
			continue
		}
		if opts.Refresh == "" && e.RetryAfter > p.now().UnixMilli() {
			report.Deferred++
			continue
		}
		queue = append(queue, u)
	}
	if opts.DryRun {
		report.Missing = queue
		return report, nil
	}
	var mu sync.Mutex
	cursor := 0
	var persistErr error
	var cancelErr error
	var wg sync.WaitGroup
	for i := 0; i < min(3, len(queue)); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				mu.Lock()
				if cursor >= len(queue) || persistErr != nil || cancelErr != nil || ctx.Err() != nil {
					mu.Unlock()
					return
				}
				u := queue[cursor]
				cursor++
				mu.Unlock()
				data, fetchErr := p.fetch(ctx, u)
				var analysis Analysis
				if fetchErr == nil && ctx.Err() == nil {
					analysis, fetchErr = AnalyzeContext(ctx, data)
				}
				mu.Lock()
				// 批次被新内容替换/关闭时不污染缓存，也不产生 24h 图片失败延迟。
				if ctx.Err() != nil || errors.Is(fetchErr, context.Canceled) {
					cancelErr = ctx.Err()
					if cancelErr == nil {
						cancelErr = fetchErr
					}
					mu.Unlock()
					return
				}
				entry := store.Images[u]
				entry.UpdatedAt = p.now().UTC().Format("2006-01-02T15:04:05.000Z")
				if analysis.Dimensions != nil {
					entry.Dimensions = analysis.Dimensions
				}
				if analysis.Color != nil {
					entry.Color = analysis.Color
				}
				entry.Error = ""
				entry.RetryAfter = 0
				if fetchErr != nil {
					entry.Error = fetchErr.Error()
					entry.RetryAfter = p.now().Add(24 * time.Hour).UnixMilli()
					report.Failed++
				}
				store.Images[u] = entry
				report.Processed++
				if e := writeStore(p.StorePath, store); e != nil && persistErr == nil {
					persistErr = e
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if persistErr != nil {
		return report, persistErr
	}
	if ctx.Err() != nil {
		return report, ctx.Err()
	}
	if cancelErr != nil {
		return report, cancelErr
	}
	return report, nil
}
