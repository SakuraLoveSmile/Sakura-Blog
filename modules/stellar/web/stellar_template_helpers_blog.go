package web

import (
	"fmt"
	"html/template"
	"net/url"
	"strings"
)

func stellarBlogDetail(d *StellarRenderData) bool {
	switch d.Stellar.Layout {
	case "post", "page", "wiki-doc", "note":
		return true
	}
	return d.Stellar.Model != nil
}
func (r stellarTemplateRenderer) blogShare(d *StellarRenderData) template.HTML {
	if !d.Stellar.BlogContent || d.Stellar.Model != nil || !stellarBlogDetail(d) {
		return ""
	}
	return r.share(d, map[string]any{})
}
func (r stellarTemplateRenderer) blogShareButtons(d *StellarRenderData, share map[string]any, official map[string]bool) (template.HTML, bool) {
	if !d.Stellar.BlogContent || stellarGet(d.Stellar.Data, "blog.content.share.enabled") != true {
		return "", false
	}
	var b strings.Builder
	qrcode := false
	for _, raw := range stellarList(stellarGet(d.Stellar.Data, "blog.content.share.platforms")) {
		platform := stellarText(raw)
		if official[platform] {
			continue
		}
		if platform != "wechat" && platform != "link" && platform != "weibo" && platform != "email" {
			continue
		}
		official[platform] = true
		if platform == "wechat" {
			qrcode = true
		}
		label := Lang("share." + platform)
		if platform == "link" {
			label = Lang("share.copy_link")
		}
		href := "#"
		attrs := ""
		pageURL := stellarText(stellarFirst(share["permalink"], d.Page.CanonicalURL))
		title := stellarText(stellarFirst(share["title"], d.Page.Title+" - "+d.Site.Title))
		switch platform {
		case "weibo":
			href = "https://service.weibo.com/share/share.php?url=" + url.QueryEscape(pageURL) + "&title=" + url.QueryEscape(title)
			attrs = ` target="_blank" rel="external nofollow noopener noreferrer"`
		case "email":
			href = "mailto:?subject=" + url.QueryEscape(title) + "&body=" + url.QueryEscape(pageURL)
		}
		fmt.Fprintf(&b, `<a class="social ui-interactive share-item %s" data-platform="%s" href="%s"%s title="%s" aria-label="%s"><span class="share-item__icon ui-icon">%s</span></a>`, platform, platform, stellarEscape(href), attrs, stellarEscape(label), stellarEscape(label), r.icon("share:"+platform))
	}
	return template.HTML(b.String()), qrcode
}
func (r stellarTemplateRenderer) blogSummary(d *StellarRenderData) template.HTML {
	if !stellarBlogDetail(d) || stellarGet(d.Stellar.Data, "blog.ai.summary.enabled") != true || d.Page.Title == "" {
		return ""
	}
	byok := stellarGet(d.Stellar.Data, "blog.ai.summary.allow_user_api") == true
	var b strings.Builder
	b.WriteString(`<div class="tag-plugin colorful note ai-summary" id="ai-summary" color="blue"`)
	if byok {
		b.WriteString(` data-user-api="1"`)
	}
	for _, key := range []string{"generating", "done", "required"} {
		fmt.Fprintf(&b, ` data-%s-msg="%s"`, key, stellarEscape(Lang("ai.summary."+key)))
	}
	fmt.Fprintf(&b, `><div class="title"><svg class="ai-summary-icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M12 2l1.8 6.2L20 10l-6.2 1.8L12 18l-1.8-6.2L4 10l6.2-1.8z"/></svg><span class="ai-summary-title-text">%s</span><span class="ai-summary-tag">%s</span></div><div class="body">`, stellarEscape(Lang("ai.summary.title")), stellarEscape(Lang("ai.summary.tag")))
	if d.Page.AISummary != "" {
		fmt.Fprintf(&b, `<p>%s</p>`, stellarEscape(d.Page.AISummary))
	}
	b.WriteString(`</div>`)
	if byok {
		fmt.Fprintf(&b, `<div class="ai-summary-panel"><button type="button" class="ai-summary-open">%s</button><div class="ai-summary-form" hidden>`, stellarEscape(Lang("ai.summary.open")))
		for _, field := range []struct{ name, kind string }{{"endpoint", "url"}, {"model", "text"}, {"key", "password"}} {
			fmt.Fprintf(&b, `<input type="%s" class="ai-summary-%s" placeholder="%s" autocomplete="off">`, field.kind, field.name, stellarEscape(Lang("ai.summary."+field.name)))
		}
		fmt.Fprintf(&b, `<label class="ai-summary-remember"><input type="checkbox" class="ai-summary-remember-check">%s</label><button type="button" class="ai-summary-generate">%s</button><p class="cap ai-summary-privacy">%s</p></div></div>`, stellarEscape(Lang("ai.summary.remember")), stellarEscape(Lang("ai.summary.generate")), stellarEscape(Lang("ai.summary.privacy")))
	}
	b.WriteString(`</div>`)
	return template.HTML(b.String())
}

func stellarBlogURL(d *StellarRenderData) string {
	return stellarPublicPageURL(d)
}
