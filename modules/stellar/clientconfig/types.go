// Package clientconfig projects the official Stellar 2.0.0 runtime configuration.
// 输入仅为已解析的配置；资源映射与 Blog 扩展在这一浏览器边界分离。
package clientconfig

type Input struct {
	Root, PageURL, SiteURL, SiteTitle, Language, Profile, CollectionID string
	Theme, Page, Collection, Regions, Data                             map[string]any
	Blog                                                               BlogConfig
	Messages                                                           map[string]any
	Icons                                                              map[string]string
}
type BlogConfig struct {
	GoSearch, NativeComments, AISummary, AIAsk, Reading, Tagcloud, Checkbox, Content, Gist bool
	Services                                                                               map[string]ServiceBinding
	SearchPath                                                                             string
	SearchTTL                                                                              int
}
type ServiceBinding struct {
	Module   string         `json:"module"`
	Selector string         `json:"selector"`
	Marked   string         `json:"marked,omitempty"`
	Options  map[string]any `json:"options,omitempty"`
}
type Bundle struct {
	ClientData  map[string]any
	Manifest    Manifest
	Comments    CommentsModel
	Styles      []StyleResource
	CoreScripts []string
	Bootstrap   string
}
type Manifest struct {
	Version    int         `json:"version"`
	Root       string      `json:"root"`
	Policy     Policy      `json:"policy"`
	Extensions []Extension `json:"extensions"`
}
type Policy struct {
	Cache     CachePolicy       `json:"cache"`
	Request   RequestPolicy     `json:"request"`
	Features  map[string]int    `json:"features"`
	Providers map[string]string `json:"providers"`
}
type CachePolicy struct {
	Enabled    bool           `json:"enabled"`
	DefaultTTL int            `json:"defaultTtl"`
	TTL        map[string]int `json:"ttl"`
	MaxEntries int            `json:"maxEntries"`
}
type RequestPolicy struct {
	Retries            int `json:"retries"`
	TimeoutMS          int `json:"timeoutMs"`
	IdleTimeoutMS      int `json:"idleTimeoutMs"`
	MaxCacheEntryBytes int `json:"maxCacheEntryBytes"`
}
type Extension struct {
	ID     string         `json:"id"`
	Module string         `json:"module"`
	Scope  string         `json:"scope"`
	When   When           `json:"when"`
	Config map[string]any `json:"config"`
}
type When struct {
	Always   bool   `json:"always,omitempty"`
	Selector string `json:"selector,omitempty"`
}

// Placement="head"；所有 Styles 按序输出，最后一条为预设 appearance 尾层。
// PageStyle 标记允许局部导航替换的页面资源，Integrity 非空需 crossorigin=anonymous。
type StyleResource struct {
	Href      string `json:"href"`
	Integrity string `json:"integrity,omitempty"`
	PageStyle bool   `json:"pageStyle"`
	Placement string `json:"placement"`
}
type CommentsModel struct {
	Enabled   bool           `json:"enabled"`
	Title     string         `json:"title"`
	ID        string         `json:"id"`
	Service   string         `json:"service"`
	Options   map[string]any `json:"options"`
	Assets    map[string]any `json:"assets"`
	PageTitle string         `json:"pageTitle"`
}
