package clientconfig

import (
	"blog/internal/stellarv2"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"
)

//go:embed resources.json
var resourceJSON []byte
var resources = func() object {
	r := object{}
	if err := json.Unmarshal(resourceJSON, &r); err != nil {
		panic(err)
	}
	return r
}()
var htmlComments = regexp.MustCompile(`<!--[\s\S]*?-->`)

const servicesSelector = "#artalk_container, #twikoo_container, #waline_container, .data-service, [class*='ds-'], a[cardlink], a[data-md-link][data-siteinfo-api], .site-card [data-siteinfo-api], .voice>audio, .video>video, .chat-file"
const lightboxSelector = "[data-fancybox]:not(.error), .with-fancybox, .ds-memos"

func Build(input Input) (Bundle, error) {
	if err := checkObjectFields(input.Theme, "theme"); err != nil {
		return Bundle{}, err
	}
	theme := merged(stellarv2.DefaultTheme(), input.Theme)
	language := input.Language
	if language == "" {
		language = "zh-CN"
	}
	if _, ok := obj(resources["languages"])[language]; !ok {
		language = "en"
	}
	input.Messages = merged(obj(at(resources, "languages."+language)), input.Messages)
	root := input.Root
	if root == "" {
		root = "/"
	}
	if !strings.HasSuffix(root, "/") {
		root += "/"
	}
	assets := obj(at(resources, "constants.assets"))
	defaults := obj(resources["defaults"])
	features := obj(theme["features"])
	for _, key := range []string{"copy", "colorScheme"} {
		if key == "colorScheme" && at(features, "colorSchemeSwitch.enabled") != true {
			continue
		}
		if value := input.Messages[key]; value != nil {
			if _, ok := value.(object); !ok {
				return Bundle{}, fmt.Errorf("messages.%s must be an object", key)
			}
		}
	}
	search := obj(theme["search"])
	appearance := obj(theme["appearance"])
	preset := str(appearance["preset"])
	if preset != "card" && preset != "glass" && preset != "minimal" && preset != "flat" {
		return Bundle{}, fmt.Errorf("appearance.preset %q is invalid", preset)
	}
	colorScheme := str(appearance["colorScheme"])
	if colorScheme != "auto" && colorScheme != "light" && colorScheme != "dark" {
		return Bundle{}, fmt.Errorf("appearance.colorScheme %q is invalid", colorScheme)
	}
	comments := resolveComments(theme, input)
	bundle := Bundle{Comments: comments, CoreScripts: []string{"/assets/js/utils.js", "/assets/js/main.js", "/assets/js/navigation-init.js", "/assets/js/profile.js"}, Bootstrap: "/assets/js/runtime/index.js"}
	bundle.ClientData = clientData(theme, assets, input, root)
	var policy Policy
	policySource := obj(clone(at(resources, "constants.runtime")))
	policySource["providers"] = clone(at(resources, "constants.providers"))
	b, _ := json.Marshal(policySource)
	if err := json.Unmarshal(b, &policy); err != nil {
		return Bundle{}, err
	}
	bundle.Manifest = Manifest{Version: 1, Root: root, Policy: policy, Extensions: []Extension{}}
	add := func(id, scope, selector string, config object) {
		when := When{Selector: selector}
		if selector == "" {
			when = When{Always: true}
		}
		bundle.Manifest.Extensions = append(bundle.Manifest.Extensions, Extension{ID: id, Module: "/assets/js/runtime/extensions/" + id + ".js", Scope: scope, When: when, Config: config})
	}
	add("pin-slider", "region", ".pin-slider", object{})
	add("page-controls", "region", "", object{"highlight": rewrite(at(assets, "runtime.searchHighlight")), "tagtree": rewrite(at(assets, "runtime.tagtree"))})
	bundle.Manifest.Extensions[len(bundle.Manifest.Extensions)-1].Module = "/assets/js/runtime/extensions/blog-page-controls.js"
	if at(features, "partialNavigation.enabled") == true {
		add("partial-navigation", "document", "", object{})
	}
	if at(features, "colorSchemeSwitch.enabled") == true {
		add("color-scheme-switch", "document", "", object{"messages": clone(obj(input.Messages["colorScheme"]))})
	}
	if !input.Blog.GoSearch {
		provider := str(search["provider"])
		if provider != "" {
			if provider != "local" && provider != "algolia" {
				return Bundle{}, fmt.Errorf("search.provider %q is invalid", provider)
			}
			options, res := split(obj(search[provider]), obj(at(defaults, "search."+provider)), "js")
			var client any
			if provider == "algolia" {
				client = rewrite(res["js"])
			}
			add("search", "document", ".search-input", object{"provider": provider, "options": options, "assets": object{"client": client, "provider": rewrite(at(assets, "search.providers."+provider)), "shortcut": rewrite(at(assets, "search.shortcut"))}})
			if provider == "local" && at(search, "local.includeContent") == false {
				entry := &bundle.Manifest.Extensions[len(bundle.Manifest.Extensions)-1]
				entry.Module = "/assets/js/runtime/extensions/blog-title-search.js"
				entry.Config["assets"].(object)["provider"] = "/assets/js/blog/search/title-search.js"
			}
		}
	}
	add("hero-parallax", "region", ".wiki-hero", object{})
	add("hero-effect", "region", "canvas[data-hero-effect]", object{})
	add("hero-video", "region", ".wiki-cover-background-video", object{})
	_, lazy := split(obj(features["lazyLoading"]), obj(at(defaults, "features.lazy_loading")), "js")
	add("lazy-loading", "document", "", object{"asset": rewrite(lazy["js"])})
	bundle.Manifest.Extensions[len(bundle.Manifest.Extensions)-1].Module = "/assets/js/runtime/extensions/blog-lazy-loading.js"
	add("deferred-icons", "region", "svg.icon[data-icon]", object{})
	add("dropdown", "region", "details.dropdown", object{})
	_, marked := split(obj(at(theme, "services.markdown")), obj(at(defaults, "services.markdown")), "js")
	add("services", "region", servicesSelector, object{"marked": rewrite(marked["js"])})
	// Wrapper 只保留本地服务DOM所有权；官方 services.js 本身保持完整原版。
	bundle.Manifest.Extensions[len(bundle.Manifest.Extensions)-1].Module = "/assets/js/runtime/extensions/blog-upstream-services.js"
	if comments.Enabled {
		add("comments", "region", "#comments", object{"provider": comments.Service, "options": clone(comments.Options), "pageTitle": comments.PageTitle, "assets": clone(comments.Assets)})
	}
	add("settings", "region", ".settings-page", object{})
	if at(features, "linkPrefetch.enabled") == true {
		_, res := split(obj(features["linkPrefetch"]), obj(at(defaults, "features.link_prefetch")), "js")
		add("link-prefetch", "document", "", object{"asset": rewrite(res["js"])})
	}
	if at(features, "lightbox.enabled") == true {
		options, res := split(obj(features["lightbox"]), obj(at(defaults, "features.lightbox")), "js", "css")
		selector := lightboxSelector
		if custom := str(at(features, "lightbox.selector")); custom != "" {
			selector += ", " + custom
		}
		options["assets"] = rewrite(merged(obj(at(assets, "features.lightbox")), res))
		add("lightbox", "region", selector, options)
	}
	if at(features, "reveal.enabled") == true {
		add("reveal", "region", ".slide-up", obj(clone(features["reveal"])))
	}
	render := obj(input.Page["render"])
	mathProvider := str(render["math"])
	if !truth(render["math"]) && render["math"] != false {
		mathProvider = str(at(features, "math.provider"))
	}
	if mathProvider == "mathjax" {
		options, res := split(obj(at(features, "math.mathjax")), obj(at(defaults, "features.math.mathjax")), "js")
		add("mathjax", "region", ".has-jax, script[type^='math/tex']", object{"options": options, "asset": rewrite(res["js"])})
	}
	diagramsProvider := str(at(features, "diagrams.provider"))
	diagramOptions := object{}
	if override, ok := render["diagrams"]; ok {
		switch x := override.(type) {
		case bool:
			if !x {
				diagramsProvider = ""
			}
		case string:
			diagramsProvider = x
		case object:
			diagramsProvider = "mermaid"
			diagramOptions = x
		}
	}
	if diagramsProvider == "mermaid" {
		options, res := split(obj(at(features, "diagrams.mermaid")), obj(at(defaults, "features.diagrams.mermaid")), "js")
		content, _ := split(diagramOptions, object{}, "js")
		options = merged(options, content)
		options["provider"] = "mermaid"
		options["assets"] = rewrite(res)
		options["colorScheme"] = colorScheme
		add("diagrams", "region", ".mermaid", options)
	}
	add("code-copy", "region", ".code", object{"assets": rewrite(at(assets, "features.codeCopy")), "messages": clone(obj(input.Messages["copy"]))})
	add("adaptive-text", "region", "[data-text-adaptive]", object{"assets": rewrite(at(assets, "features.adaptiveText"))})
	spotlight := at(features, "cardHover.spotlight") == true
	tilt := at(features, "cardHover.tilt") == true
	if spotlight || tilt {
		add("card-hover", "region", ".card-hover", object{"spotlight": spotlight, "tilt": tilt, "assets": rewrite(at(assets, "features.cardHover"))})
	}
	if at(features, "heti.enabled") == true {
		_, res := split(obj(features["heti"]), obj(at(defaults, "features.heti")), "js", "css")
		add("heti", "region", ".heti", object{"assets": rewrite(res)})
	}
	_, swiper := split(obj(features["swiper"]), obj(at(defaults, "features.swiper")), "js", "css")
	add("swiper", "region", "#swiper-api", object{"assets": rewrite(merged(obj(at(assets, "features.swiper")), swiper))})
	if err := addBlog(&bundle.Manifest, input.Blog, input.Messages); err != nil {
		return Bundle{}, err
	}
	bundle.Styles = buildStyles(theme, defaults, input.Blog, mathProvider, preset, spotlight, tilt)
	return bundle, nil
}
func clientData(theme, assets object, input Input, root string) object {
	icons := object{}
	for _, key := range []string{"default:link", "default:to-comment", "default:profile", "default:warning", "default:settings", "copy:copy", "weibo:repeat", "weibo:like"} {
		icons[key] = htmlComments.ReplaceAllString(input.Icons[key], "")
	}
	services := obj(rewrite(at(assets, "services")))
	services["gravatar"] = object{"baseUrl": at(theme, "services.gravatar.baseUrl")}
	siteinfo := obj(at(theme, "services.siteInfo"))
	endpoint := at(siteinfo, str(siteinfo["provider"])+".endpoint")
	services["siteinfo"].(object)["api"] = endpoint
	ctx := object{"root": root, "tag_plugins": object{"chat": object{"api": at(assets, "services.chat.endpoint")}}, "icons": icons, "ui": object{"classes": object{"interactive": "ui-interactive", "interactiveSpotlight": "ui-interactive card-hover card-hover--spotlight", "collectionItem": "ui-collection__item ui-interactive card-hover card-hover--spotlight", "spotlight": "card-hover card-hover--spotlight", "hoverCard": "card-hover card-hover--spotlight card-hover--tilt"}}, "services": services}
	provider := str(at(theme, "search.provider"))
	if input.Blog.GoSearch {
		provider = "local"
	}
	if provider != "" {
		search := object{"service": provider}
		if provider == "local" {
			path := "/search.json"
			ttl := at(theme, "search.local.cacheTtlSeconds")
			if input.Blog.GoSearch {
				if input.Blog.SearchPath != "" {
					path = input.Blog.SearchPath
				}
				ttl = input.Blog.SearchTTL
			}
			search["service"] = "local_search"
			search["path"] = path
			search["local_search"] = object{"field": at(theme, "search.local.scope"), "path": path, "content": at(theme, "search.local.includeContent"), "lazy_load": true, "cache_ttl": ttl}
		}
		ctx["search"] = search
	}
	// defines.ejs 的逐页面字段直接返回，模板无需重复猜测语言或日期参数。
	ctx["date_suffix"] = clone(obj(input.Messages["dateSuffix"]))
	if search, ok := ctx["search"].(object); ok {
		for _, key := range []string{"more", "no_more"} {
			if v, ok := obj(input.Messages["search"])[key]; ok {
				search[key] = clone(v)
			}
		}
	}
	canonical := obj(clone(theme["canonical"]))
	canonical["encoded"] = base64.StdEncoding.EncodeToString([]byte(str(canonical["host"])))
	canonical["param"] = object{"permalink": input.PageURL, "checklink": at(services, "video.js")}
	return object{"canonical": canonical, "ctx": ctx, "def": object{"avatar": at(theme, "fallbacks.avatar"), "cover": at(theme, "fallbacks.cover")}}
}
func resolveComments(theme object, input Input) CommentsModel {
	source := obj(theme["comments"])
	base := object{"enabled": source["enabled"] != false, "title": str(source["title"]), "id": str(source["id"]), "provider": nil, "options": object{}}
	if _, ok := source["provider"].(string); ok {
		base["provider"] = source["provider"]
	}
	profile := input.Profile
	profileKey := runtimeKey(profile)
	if profile == "404" {
		profileKey = "error"
	}
	if (profile == "home" && input.Data["isHomeFirstPage"] != false) || profileKey == "error" {
		if override := at(theme, "profiles."+profileKey+".comments"); truth(override) {
			if m, ok := override.(object); ok {
				base = mergeComments(base, m)
			} else {
				base["enabled"] = true
			}
		}
	}
	collection := obj(input.Collection["comments"])
	if v := at(input.Collection, "presentation.comments"); v != nil {
		collection = obj(v)
	}
	base = mergeComments(base, mergeConfig(collection, obj(input.Page["comments"])))
	service := str(base["provider"])
	options := mergeComments(obj(source[service]), obj(base["options"]))
	fields := []string{"js"}
	if service == "artalk" || service == "waline" {
		fields = append(fields, "css")
	}
	if service == "waline" {
		fields = append(fields, "meta_css")
	}
	for _, field := range fields {
		if v, ok := obj(base["options"])[field]; ok {
			options[field] = v
		}
	}
	resolved, assets := split(options, obj(at(resources, "defaults.comments."+service)), fields...)
	assets = merged(obj(at(resources, "constants.assets.comments."+service)), assets)
	if service == "artalk" {
		server := strings.TrimRight(strings.TrimSpace(str(options["server"])), "/")
		for _, field := range []string{"js", "css"} {
			if assets[field] == nil {
				if server != "" {
					assets[field] = server + "/dist/Artalk." + field
				}
			}
		}
	}
	if service == "giscus" && str(at(theme, "appearance.colorScheme")) != "auto" && options["data-theme"] == "preferred_color_scheme" {
		resolved["data-theme"] = at(theme, "appearance.colorScheme")
	}
	pageTitle := str(input.Data["title"])
	if pageTitle == "" {
		pageTitle = str(input.Page["title"])
	}
	return CommentsModel{Enabled: base["enabled"] != false && service != "", Title: str(base["title"]), ID: str(base["id"]), Service: service, Options: resolved, Assets: obj(rewrite(assets)), PageTitle: pageTitle}
}
func addBlog(manifest *Manifest, blog BlogConfig, messages object) error {
	add := func(id, module, scope, selector string, config object) {
		manifest.Extensions = append(manifest.Extensions, Extension{ID: id, Module: "/assets/js/runtime/extensions/" + module + ".js", Scope: scope, When: When{Selector: selector}, Config: config})
	}
	if blog.GoSearch {
		if blog.SearchTTL < 0 {
			return fmt.Errorf("blog search TTL cannot be negative")
		}
		add("blog-search", "blog-search", "document", ".search-input", object{})
	}
	natives := []struct {
		id, selector string
		enabled      bool
	}{{"blog-comments", "#cmt-body", blog.NativeComments}, {"blog-ai-summary", "#ai-summary", blog.AISummary}, {"blog-ai-ask", "#rag-ai-ask", blog.AIAsk}, {"blog-reading", "article.md-text.content", blog.Reading}, {"blog-tagcloud", ".widget-wrapper.tagcloud", blog.Tagcloud}, {"blog-checkbox", ".tag-plugin.checkbox input, .tag-plugin.radio input", blog.Checkbox}}
	for _, n := range natives {
		if n.enabled {
			add(n.id, "blog-native", "region", n.selector, object{})
		}
	}
	if blog.Content {
		add("blog-content", "blog-content", "region", "#share, #reward-toggle, [data-wiki-empty], .wiki-copy", object{"copied": obj(messages["copy"])["copied"], "promptCopy": obj(messages["share"])["promptCopy"]})
	}
	if blog.Gist {
		add("blog-gist", "blog-gist", "region", "iframe[data-stellar-gist]", object{})
	}
	if len(blog.Services) > 0 {
		bindings := object{}
		keys := make([]string, 0, len(blog.Services))
		for k := range blog.Services {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, key := range keys {
			binding := blog.Services[key]
			if binding.Module == "" || binding.Selector == "" {
				return fmt.Errorf("blog service %q requires module and selector", key)
			}
			if !strings.HasPrefix(binding.Module, "/assets/js/blog/services/") || !strings.HasSuffix(binding.Module, ".js") || path.Clean(binding.Module) != binding.Module {
				return fmt.Errorf("blog service %q module must be a local Blog service script", key)
			}
			bindings[key] = mapJSON(binding)
		}
		add("blog-services", "blog-services", "region", "[data-blog-service]", object{"services": bindings})
	}
	return nil
}
func buildStyles(theme, defaults object, blog BlogConfig, math, preset string, spotlight, tilt bool) []StyleResource {
	styles := []StyleResource{}
	add := func(path, integrity string, page bool) {
		if path != "" {
			styles = append(styles, StyleResource{Href: path, Integrity: integrity, PageStyle: page, Placement: "head"})
		}
	}
	add("/assets/css/main-"+preset+".css", "", false)
	if spotlight {
		add("/assets/css/main-card-hover-spotlight.css", "", false)
	}
	if tilt {
		add("/assets/css/main-card-hover-tilt.css", "", false)
	}
	if at(theme, "features.lazyLoading.transition") == "blur" {
		add("/assets/css/main-lazyload-blur.css", "", false)
	}
	if math == "katex" {
		add("/assets/css/main-katex.css", "", true)
		_, res := split(obj(at(theme, "features.math.katex")), obj(at(defaults, "features.math.katex")), "css")
		add(str(rewrite(res["css"])), str(at(theme, "features.math.katex.css_integrity")), true)
	}
	add(str(rewrite(normalizeResource(at(theme, "appearance.codeBlock.highlightStylesheet")))), "", false)
	// 服务端 Markdown 使用 Chroma token；官方 Highlight.js 样式不消费这些类名。
	add("/assets/css/highlight.css", "", false)
	add("/assets/css/pagination.css", "", false)
	if blog.GoSearch || blog.AIAsk {
		add("/assets/css/search.css", "", false)
	}
	if blog.Password {
		add("/assets/css/post-password.css", "", true)
	}
	if blog.NativeComments {
		add("/assets/css/comments.css", "", false)
	}
	if blog.Reading || blog.AISummary {
		add("/assets/css/reading.css", "", false)
	}
	if blog.Tagcloud {
		add("/assets/css/tagcloud.css", "", false)
	}
	if blog.Content {
		add("/assets/css/post-footer.css", "", false)
	}
	add("/assets/css/main-appearance-"+preset+".css", "", false)
	return styles
}
