package github

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Client GitHub REST 代理客户端，内存 + 磁盘双层缓存，stale-while-error 降级。
// 仅直连 api.github.com（自托管渲染），不依赖任何第三方服务主机。
type Client struct {
	token   string // 可选 PAT，空则匿名（60 req/h）
	apiBase  string // GitHub REST API 基址，默认 https://api.github.com（测试可注入）
	cacheDir string // 磁盘缓存目录 data/ghcache/
	http     *http.Client

	mu    sync.RWMutex
	mem   map[string]*memEntry
}

type memEntry struct {
	data      []byte
	expiresAt time.Time
}

// Config 构造 Client 所需配置。
type Config struct {
	Token    string // GitHub PAT（可选）
	CacheDir string // 磁盘缓存目录（绝对路径）
}

// New 创建 GitHub 数据客户端。
func New(cfg Config) *Client {
	cacheDir := cfg.CacheDir
	if cacheDir == "" {
		cacheDir = "data/ghcache"
	}
	_ = os.MkdirAll(cacheDir, 0o755)
	return &Client{
		token:    cfg.Token,
		apiBase:  "https://api.github.com",
		cacheDir: cacheDir,
		http:     &http.Client{Timeout: 15 * time.Second},
		mem:      make(map[string]*memEntry),
	}
}

// GetRepo 获取仓库元数据（裁剪字段），缓存 TTL 6h。
func (c *Client) GetRepo(owner, repo string) (*RepoInfo, error) {
	url := fmt.Sprintf("%s/repos/%s/%s", c.apiBase, owner, repo)
	data, err := c.get(url, 6*time.Hour)
	if err != nil {
		return nil, err
	}
	var raw struct {
		Name        string `json:"name"`
		FullName    string `json:"full_name"`
		Description string `json:"description"`
		Stargazers  int    `json:"stargazers_count"`
		Forks       int    `json:"forks_count"`
		Language    string `json:"language"`
		License     *struct {
			Name string `json:"name"`
		} `json:"license"`
		Owner struct {
			Login     string `json:"login"`
			AvatarURL string `json:"avatar_url"`
		} `json:"owner"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse repo json: %w", err)
	}
	info := &RepoInfo{
		Name:        raw.Name,
		FullName:    raw.FullName,
		Description: raw.Description,
		Stars:       raw.Stargazers,
		Forks:       raw.Forks,
		Language:    raw.Language,
		OwnerAvatar: raw.Owner.AvatarURL,
		OwnerLogin:  raw.Owner.Login,
		HTMLURL:     raw.HTMLURL,
	}
	if raw.License != nil {
		info.License = raw.License.Name
	}
	return info, nil
}

// GetUser 获取用户信息，缓存 TTL 6h。
func (c *Client) GetUser(username string) (*UserInfo, error) {
	url := fmt.Sprintf("%s/users/%s", c.apiBase, username)
	data, err := c.get(url, 6*time.Hour)
	if err != nil {
		return nil, err
	}
	var raw struct {
		Login     string `json:"login"`
		Name      string `json:"name"`
		Bio       string `json:"bio"`
		AvatarURL string `json:"avatar_url"`
		Followers int    `json:"followers"`
		Following int    `json:"following"`
		Repos     int    `json:"public_repos"`
		HTMLURL   string `json:"html_url"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse user json: %w", err)
	}
	return &UserInfo{
		Login:     raw.Login,
		Name:      raw.Name,
		Bio:       raw.Bio,
		AvatarURL: raw.AvatarURL,
		Followers: raw.Followers,
		Following: raw.Following,
		Repos:     raw.Repos,
		HTMLURL:   raw.HTMLURL,
	}, nil
}

// GetTagsRaw 仓库 tags 列表原样 JSON（Spec 33 ghrepo widget 的 latest-tag 子卡，ds-ghinfo 按 GitHub 形状数组 index=0 回填），
// 缓存 TTL 6h，与仓库元数据同档。
func (c *Client) GetTagsRaw(owner, repo string) ([]byte, error) {
	return c.get(fmt.Sprintf("%s/repos/%s/%s/tags?per_page=5", c.apiBase, owner, repo), 6*time.Hour)
}

// GetIssuesRaw 仓库 issues 列表原样 JSON（Spec 33 ghissues widget，ds-timeline 消费 GitHub 形状字段）。
// query 为调用方用 url.Values 编码好的查询串（per_page/labels）；issues 时效性高于元数据，TTL 30min。
func (c *Client) GetIssuesRaw(owner, repo, query string) ([]byte, error) {
	return c.get(fmt.Sprintf("%s/repos/%s/%s/issues?%s", c.apiBase, owner, repo, query), 30*time.Minute)
}

// Issue 裁剪后的 issue 信息（友链导入等消费，对齐 GitHub REST 形状）。
type Issue struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	State  string `json:"state"`
	Body   string `json:"body"`
	Labels []struct {
		Name string `json:"name"`
	} `json:"labels"`
}

// GetIssues 仓库 issues 列表（裁剪字段）。query 为调用方用 url.Values 编码好的查询串；
// 复用 GetIssuesRaw 的缓存（TTL 30min），仅供已注入 apiBase 的测试或内部消费。
func (c *Client) GetIssues(owner, repo, query string) ([]Issue, error) {
	data, err := c.GetIssuesRaw(owner, repo, query)
	if err != nil {
		return nil, err
	}
	var issues []Issue
	if err := json.Unmarshal(data, &issues); err != nil {
		return nil, fmt.Errorf("parse issues json: %w", err)
	}
	return issues, nil
}

// get 通用缓存拉取：内存 → 磁盘 → 上游。上游失败时 stale-while-error。
func (c *Client) get(url string, ttl time.Duration) ([]byte, error) {
	key := cacheKey(url)

	// 1. 内存缓存
	c.mu.RLock()
	if e, ok := c.mem[key]; ok && time.Now().Before(e.expiresAt) {
		c.mu.RUnlock()
		return e.data, nil
	}
	c.mu.RUnlock()

	// 2. 磁盘缓存
	diskPath := filepath.Join(c.cacheDir, key+".json")
	var diskData []byte
	var diskTime time.Time
	if fi, err := os.Stat(diskPath); err == nil {
		diskTime = fi.ModTime()
		if b, err := os.ReadFile(diskPath); err == nil {
			diskData = b
			if time.Since(diskTime) < ttl {
				// 磁盘未过期 → 回填内存
				c.setMem(key, b, ttl)
				return b, nil
			}
		}
	}

	// 3. 上游拉取
	data, err := c.fetch(url)
	if err != nil {
		// stale-while-error：有磁盘旧缓存则返回
		if diskData != nil {
			c.setMem(key, diskData, ttl) // 续命避免频繁重试
			return diskData, nil
		}
		return nil, err
	}

	// 双写缓存
	c.setMem(key, data, ttl)
	_ = os.WriteFile(diskPath, data, 0o644)
	return data, nil
}

// fetch 实际 HTTP 请求。
func (c *Client) fetch(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "StellarBlog/1.0")
	req.Header.Set("Accept", "application/vnd.github+json, image/svg+xml, */*")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github fetch %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, resp.Body)
		return nil, fmt.Errorf("github api %s: status %d", url, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func (c *Client) setMem(key string, data []byte, ttl time.Duration) {
	c.mu.Lock()
	c.mem[key] = &memEntry{data: data, expiresAt: time.Now().Add(ttl)}
	c.mu.Unlock()
}

// cacheKey URL → 短哈希文件名。
func cacheKey(url string) string {
	h := sha256.Sum256([]byte(url))
	return hex.EncodeToString(h[:16])
}

// RepoInfo 裁剪后的仓库信息。
type RepoInfo struct {
	Name        string `json:"name"`
	FullName    string `json:"full_name"`
	Description string `json:"description"`
	Stars       int    `json:"stars"`
	Forks       int    `json:"forks"`
	Language    string `json:"language"`
	License     string `json:"license,omitempty"`
	OwnerAvatar string `json:"owner_avatar"`
	OwnerLogin  string `json:"owner_login"`
	HTMLURL     string `json:"html_url"`
}

// UserInfo 裁剪后的用户信息。
type UserInfo struct {
	Login     string `json:"login"`
	Name      string `json:"name"`
	Bio       string `json:"bio"`
	AvatarURL string `json:"avatar_url"`
	Followers int    `json:"followers"`
	Following int    `json:"following"`
	Repos     int    `json:"repos"`
	HTMLURL   string `json:"html_url"`
}
