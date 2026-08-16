// 内容派生产物内存缓存（Spec 61 阶段 5）：/search.json 与 /sitemap.xml 每次请求全量重查
// 改为「写路径置脏 + 生成期缓存」——重建结果 atomic.Pointer 发布，并发共享一次重建。
// 置脏来源：post/page/moment 写 hook、wikidoc/notebook/note 保存、设置保存（api.OnContentChanged）。
package routes

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"log"
	"strconv"
	"sync"
	"sync/atomic"

	"blog/internal/store"
	tmpl "blog/internal/template"
	"blog/internal/textutil"
)

// contentCache 前台内容派生缓存（search.json / sitemap.xml）。
type contentCache struct {
	st      *store.Store
	site    *atomic.Pointer[tmpl.Site] // 站点配置（wiki 项目列表随 RebuildSite 热更）
	search  searchJSONCache
	sitemap sitemapCache
}

func newContentCache(st *store.Store, site *atomic.Pointer[tmpl.Site]) *contentCache {
	return &contentCache{st: st, site: site}
}

// invalidate 全部置脏（写路径统一调用）。
func (c *contentCache) invalidate() {
	c.search.invalidate()
	c.sitemap.invalidate()
}

// --- /search.json ---

type searchJSONCache struct {
	mu    sync.Mutex
	dirty atomic.Bool // Spec 62：invalidate 与 get 并发时无锁读写 bool 是 data race，改 atomic
	data  atomic.Pointer[[]byte]
}

func (c *searchJSONCache) invalidate() { c.dirty.Store(true) }

// get 返回索引 JSON；置脏或首次时重建（互斥保证并发共享一次重建）。
func (c *contentCache) searchJSON() ([]byte, error) {
	cache := &c.search
	if !cache.dirty.Load() {
		if d := cache.data.Load(); d != nil {
			return *d, nil
		}
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if !cache.dirty.Load() {
		if d := cache.data.Load(); d != nil {
			return *d, nil
		}
	}
	items, err := c.buildSearchItems()
	if err != nil {
		return nil, err
	}
	b, err := json.Marshal(items)
	if err != nil {
		return nil, fmt.Errorf("marshal search index: %w", err)
	}
	cache.data.Store(&b)
	cache.dirty.Store(false)
	return b, nil
}

// buildSearchItems 全量重建搜索索引（与旧 GET /search.json 同逻辑）：
// 已发布文章 + wiki 已发布文档（Path=/wiki/<projectID>/<slug>）。
func (c *contentCache) buildSearchItems() ([]searchItem, error) {
	posts, err := c.st.ListPublished()
	if err != nil {
		return nil, fmt.Errorf("list posts: %w", err)
	}
	searchCfg := c.site.Load().Search
	items := searchItemsFromPostsWithConfig(posts, searchCfg)
	for i := range c.site.Load().Wiki {
		projectID := c.site.Load().Wiki[i].ID
		docs, err := c.st.ListWikidocs(projectID)
		if err != nil {
			log.Printf("list wikidocs %s for search index: %v", projectID, err)
			continue // 查询失败跳过该项目，不影响其余索引
		}
		for _, d := range docs {
			if d.Status != store.StatusPublished {
				continue
			}
			path := "/wiki/" + projectID + "/" + d.Slug
			if searchCfg.LocalSearch.Field == "post" || !searchPathAllowed(path, searchCfg) {
				continue
			}
			content := textutil.Truncate(textutil.StripHTML(d.ContentHTML), 2000)
			if searchCfg.LocalSearch.Content != nil && !*searchCfg.LocalSearch.Content {
				content = ""
			}
			items = append(items, searchItem{
				Title:   d.Title,
				Content: content,
				Path:    path,
			})
		}
	}
	return items, nil
}

// --- /sitemap.xml ---

type sitemapCache struct {
	mu    sync.Mutex
	dirty atomic.Bool       // Spec 62：同 searchJSONCache，invalidate 无锁写改 atomic
	urls  map[string][]byte // baseURL → xml（site.url 未配置时按请求 host 变化，按 key 缓存）
}

func (c *sitemapCache) invalidate() { c.dirty.Store(true) }

// get 返回指定 baseURL 的 sitemap XML；置脏或首次时重建。
func (c *contentCache) sitemapXML(baseURL string) ([]byte, error) {
	cache := &c.sitemap
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if !cache.dirty.Load() {
		if d, ok := cache.urls[baseURL]; ok {
			return d, nil
		}
	}
	output, err := c.buildSitemap(baseURL)
	if err != nil {
		return nil, err
	}
	if cache.urls == nil {
		cache.urls = map[string][]byte{}
	}
	cache.urls[baseURL] = output
	cache.dirty.Store(false)
	return output, nil
}

// buildSitemap 全量重建 sitemap（与旧 GET /sitemap.xml 同逻辑）：
// 首页 + 已发布文章 + wiki 已发布文档 + 笔记本/笔记 + 静态索引页。
func (c *contentCache) buildSitemap(baseURL string) ([]byte, error) {
	type SitemapURL struct {
		Loc        string `xml:"loc"`
		LastMod    string `xml:"lastmod,omitempty"`
		ChangeFreq string `xml:"changefreq,omitempty"`
		Priority   string `xml:"priority,omitempty"`
	}
	type Sitemap struct {
		XMLName xml.Name     `xml:"urlset"`
		XMLNS   string       `xml:"xmlns,attr"`
		URLs    []SitemapURL `xml:"url"`
	}

	sm := Sitemap{XMLNS: "http://www.sitemaps.org/schemas/sitemap/0.9"}
	sm.URLs = append(sm.URLs, SitemapURL{Loc: baseURL + "/", Priority: "1.0"})

	posts, err := c.st.ListPublished()
	if err == nil {
		for _, p := range posts {
			sm.URLs = append(sm.URLs, SitemapURL{
				Loc:     baseURL + "/post/" + p.Slug,
				LastMod: p.UpdatedAt.Format("2006-01-02"),
			})
		}
	}

	for _, project := range c.site.Load().Wiki {
		docs, err := c.st.ListWikidocs(project.ID)
		if err != nil {
			log.Printf("sitemap: list wikidocs %s: %v", project.ID, err)
			continue
		}
		for _, doc := range docs {
			if doc.Status != store.StatusPublished {
				continue
			}
			sm.URLs = append(sm.URLs, SitemapURL{
				Loc:        baseURL + "/wiki/" + project.ID + "/" + doc.Slug,
				LastMod:    doc.UpdatedAt.Format("2006-01-02"),
				ChangeFreq: "weekly",
				Priority:   "0.6",
			})
		}
	}

	notebooks, err := c.st.ListNotebooks()
	if err == nil {
		for _, nb := range notebooks {
			sm.URLs = append(sm.URLs, SitemapURL{
				Loc:        baseURL + "/notebooks/" + nb.Name,
				ChangeFreq: "weekly",
				Priority:   "0.6",
			})
			notes, _, err := c.st.ListNotesByNotebook(nb.ID, -1, 0)
			if err != nil {
				log.Printf("sitemap: list notes of notebook %d: %v", nb.ID, err)
				continue
			}
			for _, n := range notes {
				sm.URLs = append(sm.URLs, SitemapURL{
					Loc:        baseURL + "/note/" + strconv.FormatInt(n.ID, 10),
					LastMod:    n.UpdatedAt.Format("2006-01-02"),
					ChangeFreq: "weekly",
					Priority:   "0.6",
				})
			}
		}
	}

	for _, p := range []string{"/archives", "/categories", "/tags", "/links", "/moments"} {
		sm.URLs = append(sm.URLs, SitemapURL{Loc: baseURL + p})
	}

	output, err := xml.MarshalIndent(sm, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal sitemap: %w", err)
	}
	return append([]byte(xml.Header), output...), nil
}
