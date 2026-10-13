package web

import (
	"fmt"
	"net/url"

	"blog/internal/stellarimages"
	"blog/internal/stellarv2"
)

// ImagePageBaseURL 使用站点 origin 和实际页面路径；镜像请求的 Host 不作为缓存键。
func ImagePageBaseURL(siteURL, pageURL string) (string, error) {
	site, err := url.Parse(siteURL)
	if err != nil || site.Host == "" || (site.Scheme != "http" && site.Scheme != "https") {
		return "", fmt.Errorf("image metadata requires absolute HTTP(S) site URL")
	}
	page, err := url.Parse(pageURL)
	if err != nil {
		return "", fmt.Errorf("image page URL: %w", err)
	}
	if page.Path == "" {
		page = site
	}
	base := &url.URL{Scheme: site.Scheme, Host: site.Host, Path: page.Path, RawPath: page.RawPath}
	if base.Path == "" {
		base.Path = "/"
	}
	return base.String(), nil
}

// EnrichStellarImages 在 ProcessSite 后、导航签名前只消费已准备好的缓存。
func EnrichStellarImages(source string, data *StellarRenderData, cache stellarimages.Store) (string, error) {
	pageURL := data.Page.CanonicalURL
	if data.ViewData.Stellar != nil {
		if request := data.ViewData.Stellar.RequestURL; request != "" {
			pageURL = request
		}
	}
	base, err := ImagePageBaseURL(data.Site.URL, pageURL)
	if err != nil {
		return "", err
	}
	var auto *bool
	if value, ok := stellarv2.Get(data.Stellar.Theme, "features.lazyLoading.autoAspectRatio"); ok {
		if flag, ok := value.(bool); ok {
			auto = &flag
		}
	}
	return stellarimages.Enrich(source, base, cache, stellarimages.EnrichOptions{AutoAspectRatio: auto})
}
