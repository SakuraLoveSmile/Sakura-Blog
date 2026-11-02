// Package ghcr 只读访问 GitHub Container Registry（GHCR）的镜像标签。
//
// Console「检查更新」用它替代 GitHub Release：发布只产出 Docker 镜像后，
// 最新版本由 ghcr.io 上的版本标签决定。公开镜像包可以匿名换取 pull token；
// 客户端带内存 + 磁盘缓存，`refresh` 时绕过缓存。
package ghcr

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"blog/internal/safefetch"
)

const (
	defaultRegistryBase = "https://ghcr.io"
	defaultTTL          = 6 * time.Hour
	maxPages            = 5
	maxBodyBytes        = 1 << 20
)

// imagePattern 允许 owner/name 形式的镜像名（GHCR 要求小写）。
var imagePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._/-]*$`)

// Config 构造 Client 所需配置。
type Config struct {
	RegistryBase string       // GHCR 基址，默认 https://ghcr.io（测试可注入）
	CacheDir     string       // 磁盘缓存目录，默认 data/ghcache
	HTTPClient   *http.Client // HTTP client（仅测试注入；生产留空以启用 safefetch）
	TTL          time.Duration
}

// Client 读取镜像标签的只读客户端。
type Client struct {
	registryBase string
	cacheDir     string
	http         *http.Client
	ttl          time.Duration

	mu  sync.RWMutex
	mem map[string]memEntry
}

type memEntry struct {
	tags      []string
	expiresAt time.Time
}

type diskCache struct {
	Image     string    `json:"image"`
	Tags      []string  `json:"tags"`
	FetchedAt time.Time `json:"fetched_at"`
}

// New 创建镜像标签客户端。
func New(cfg Config) *Client {
	base := strings.TrimRight(strings.TrimSpace(cfg.RegistryBase), "/")
	if base == "" {
		base = defaultRegistryBase
	}
	cacheDir := cfg.CacheDir
	if cacheDir == "" {
		cacheDir = "data/ghcache"
	}
	client := cfg.HTTPClient
	if client == nil {
		client = safefetch.New(safefetch.Options{Timeout: 15 * time.Second})
	}
	ttl := cfg.TTL
	if ttl <= 0 {
		ttl = defaultTTL
	}
	return &Client{
		registryBase: base,
		cacheDir:     cacheDir,
		http:         client,
		ttl:          ttl,
		mem:          make(map[string]memEntry),
	}
}

// ListTags 返回镜像的全部标签；refresh=true 时跳过缓存直接查询 GHCR。
// 返回的标签顺序与上游一致，版本筛选由调用方负责。
func (c *Client) ListTags(image string, refresh bool) ([]string, error) {
	image = strings.ToLower(strings.TrimSpace(image))
	if !imagePattern.MatchString(image) || strings.Contains(image, "..") {
		return nil, fmt.Errorf("无效的镜像名 %q", image)
	}
	if !refresh {
		if tags, ok := c.cached(image); ok {
			return tags, nil
		}
	}
	tags, err := c.fetch(image)
	if err != nil {
		return nil, err
	}
	c.store(image, tags)
	return tags, nil
}

// cached 依次检查内存与磁盘缓存，命中未过期条目时返回副本。
func (c *Client) cached(image string) ([]string, bool) {
	now := time.Now()
	c.mu.RLock()
	entry, ok := c.mem[image]
	c.mu.RUnlock()
	if ok && now.Before(entry.expiresAt) {
		return append([]string(nil), entry.tags...), true
	}
	data, err := os.ReadFile(c.cachePath(image))
	if err != nil {
		return nil, false
	}
	var cached diskCache
	if err := json.Unmarshal(data, &cached); err != nil || cached.Image != image {
		return nil, false
	}
	expiresAt := cached.FetchedAt.Add(c.ttl)
	if now.After(expiresAt) {
		return nil, false
	}
	tags := append([]string(nil), cached.Tags...)
	c.mu.Lock()
	c.mem[image] = memEntry{tags: tags, expiresAt: expiresAt}
	c.mu.Unlock()
	return append([]string(nil), tags...), true
}

// store 写入内存与磁盘缓存；缓存失败不影响本次结果。
func (c *Client) store(image string, tags []string) {
	expiresAt := time.Now().Add(c.ttl)
	c.mu.Lock()
	c.mem[image] = memEntry{tags: append([]string(nil), tags...), expiresAt: expiresAt}
	c.mu.Unlock()

	data, err := json.Marshal(diskCache{Image: image, Tags: tags, FetchedAt: time.Now().UTC()})
	if err != nil {
		return
	}
	if err := os.MkdirAll(c.cacheDir, 0o755); err != nil {
		return
	}
	path := c.cachePath(image)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, path)
}

func (c *Client) cachePath(image string) string {
	sum := sha256.Sum256([]byte(image))
	return filepath.Join(c.cacheDir, "ghcr-tags-"+hex.EncodeToString(sum[:8])+".json")
}

// fetch 依次获取匿名 pull token 与标签列表，最多跟随 5 页。
func (c *Client) fetch(image string) ([]string, error) {
	token, err := c.pullToken(image)
	if err != nil {
		return nil, err
	}
	var tags []string
	// image 已通过 imagePattern 校验（仅小写字母、数字、. _ - /），可直接拼进路径。
	next := fmt.Sprintf("%s/v2/%s/tags/list?n=100", c.registryBase, image)
	for page := 0; page < maxPages && next != ""; page++ {
		req, err := http.NewRequest(http.MethodGet, next, nil)
		if err != nil {
			return nil, fmt.Errorf("构造标签请求失败: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/json")
		resp, err := c.http.Do(req)
		if err != nil {
			return nil, fmt.Errorf("请求镜像标签失败: %w", err)
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("读取镜像标签失败: %w", readErr)
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("镜像标签接口返回 %d", resp.StatusCode)
		}
		var payload struct {
			Tags []string `json:"tags"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			return nil, fmt.Errorf("解析镜像标签失败: %w", err)
		}
		tags = append(tags, payload.Tags...)
		next = c.nextPage(resp.Header.Get("Link"))
	}
	return tags, nil
}

// pullToken 用匿名 scope 换取 pull token；私有包会返回 401/403。
func (c *Client) pullToken(image string) (string, error) {
	query := url.Values{}
	query.Set("service", "ghcr.io")
	query.Set("scope", "repository:"+image+":pull")
	endpoint := c.registryBase + "/token?" + query.Encode()
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("构造 token 请求失败: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("获取镜像 token 失败: %w", err)
	}
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	_ = resp.Body.Close()
	if readErr != nil {
		return "", fmt.Errorf("读取镜像 token 失败: %w", readErr)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("镜像 token 接口返回 %d", resp.StatusCode)
	}
	var payload struct {
		Token       string `json:"token"`
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", fmt.Errorf("解析镜像 token 失败: %w", err)
	}
	token := payload.Token
	if token == "" {
		token = payload.AccessToken
	}
	if token == "" {
		return "", fmt.Errorf("镜像 token 为空")
	}
	return token, nil
}

// nextPage 解析 Link 头的 rel="next"，只接受同一 registry 主机，避免被上游引导到任意地址。
func (c *Client) nextPage(link string) string {
	for _, part := range strings.Split(link, ",") {
		fields := strings.Split(part, ";")
		if len(fields) < 2 {
			continue
		}
		if !strings.Contains(fields[1], `rel="next"`) {
			continue
		}
		raw := strings.Trim(strings.TrimSpace(fields[0]), "<>")
		base, err := url.Parse(c.registryBase)
		if err != nil {
			return ""
		}
		target, err := url.Parse(raw)
		if err != nil || target.Host != base.Host || target.Scheme != base.Scheme {
			return ""
		}
		return target.String()
	}
	return ""
}
