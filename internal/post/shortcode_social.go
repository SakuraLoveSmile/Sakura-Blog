package post

import (
	tmpl "blog/internal/settings"
	"fmt"
	"html"
	"strconv"
	"strings"
)

// ---------- Spec 30: 标签插件四期（vote/friends/sites/radio + 行内标签） ----------

// renderVote 渲染 {% vote [id:x] [yes:icon] [no:icon] [title] %}，对接本地 /api/vote 数据服务。
// 输出结构对齐原版 vote.js：.tag-plugin.ds-vote > .header? + .body > .vote-up/.vote-down。
func renderVote(argsStr string, iconFunc func(string) string) string {
	named, positional := parseNamedPositionalArgs(argsStr, []string{"id", "yes", "no"}, []string{"title"})
	id := named["id"]
	if id == "" {
		id = "default"
	}
	title := named["title"]
	if title == "" && len(positional) > 0 {
		title = strings.Join(positional, " ")
	}
	iconHTML := func(key, def string) string {
		if key == "" {
			key = def
		}
		if iconFunc == nil {
			return ""
		}
		// 安全审计 2026-08-16：icon key 用户可控，格式白名单后再进入 iconFunc，非法降级为空
		key = safeIconKey(key)
		if key == "" {
			return ""
		}
		return iconFunc(key)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, `<div class="tag-plugin ds-vote" data-api="/api/vote" data-id="%s">`, html.EscapeString(id))
	if title != "" {
		fmt.Fprintf(&sb, `<div class="header"><span>%s</span></div>`, html.EscapeString(title))
	}
	sb.WriteString(`<div class="body">`)
	fmt.Fprintf(&sb, `<button class="vote-up">%s <span class="up">0</span></button>`, iconHTML(named["yes"], "vote:thumbsup"))
	fmt.Fprintf(&sb, `<button class="vote-down">%s <span class="down">0</span></button>`, iconHTML(named["no"], "vote:thumbsdown"))
	sb.WriteString(`</div></div>`)
	return sb.String()
}

// renderFriends 渲染 {% friends [api:url] %}，容器交给 services/friends.js 异步渲染。
// 结构对齐原版 friends.js：.tag-plugin.users-wrap > .data-service.ds-friends > .grid-box。
// 缺省 data-api 指向本地友链聚合端点 /api/friends（JSON 形状 {content:[{title,url,html_url,avatar_url,labels}]}
// 与 friends.js 期望字段一致，无需适配层）。
func renderFriends(argsStr string) string {
	named, _ := parseNamedPositionalArgs(argsStr, []string{"api", "posts"}, nil)
	api := named["api"]
	var sb strings.Builder
	// posts:true 对齐原版 {% friends posts:true %}：渲染带 RSS 文章列表的 ds-friends_and_posts 容器；
	// 缺省仍为 ds-friends 纯卡片流。
	if named["posts"] == "true" {
		if api == "" {
			api = "/api/friends-aggregated"
		}
		sb.WriteString(`<div class="tag-plugin users-posts-wrap">`)
		fmt.Fprintf(&sb, `<div class="data-service ds-friends_and_posts" data-api="%s"><div class="grid-box"></div></div>`, html.EscapeString(api))
	} else {
		if api == "" {
			api = "/api/friends"
		}
		sb.WriteString(`<div class="tag-plugin users-wrap">`)
		fmt.Fprintf(&sb, `<div class="data-service ds-friends" data-api="%s"><div class="grid-box"></div></div>`, html.EscapeString(api))
	}
	sb.WriteString(`</div>`)
	return sb.String()
}

// renderLinkGroupsHTML 渲染友链分组卡片流 HTML（{% links %} 服务端直出，
// 结构同旧 links.html，对齐原版 sites 标签插件 sites-wrap + site-card）。
// 供正常渲染（站点 links 配置注入）与组件库 Fixture 预览（确定性数据）共用。
func renderLinkGroupsHTML(groups []tmpl.LinkGroup, defaultLink string) string {
	if len(groups) == 0 {
		return "<p>暂无友链</p>"
	}
	var sb strings.Builder
	var unavailable []tmpl.LinkItem
	wroteGroup := false
	for _, g := range groups {
		available := make([]tmpl.LinkItem, 0, len(g.Items))
		for _, it := range g.Items {
			if it.Unavailable {
				unavailable = append(unavailable, it)
				continue
			}
			available = append(available, it)
		}
		if len(available) == 0 {
			continue
		}
		if g.Title != "" {
			fmt.Fprintf(&sb, "<h2>%s</h2>", html.EscapeString(g.Title))
		}
		renderSiteCards(&sb, available, defaultLink, false)
		wroteGroup = true
	}
	if len(unavailable) > 0 {
		if wroteGroup {
			sb.WriteString("\n")
		}
		fmt.Fprint(&sb, `<h2 class="unavailable-title">失联友链</h2>`)
		renderSiteCards(&sb, unavailable, defaultLink, true)
		wroteGroup = true
	}
	if !wroteGroup {
		return "<p>暂无友链</p>"
	}
	return sb.String()
}

func renderSiteCards(sb *strings.Builder, items []tmpl.LinkItem, defaultLink string, unavailable bool) {
	if unavailable {
		sb.WriteString(`<div class="tag-plugin sites-wrap unavailable-links"><div class="grid-box">`)
	} else {
		sb.WriteString(`<div class="tag-plugin sites-wrap"><div class="grid-box">`)
	}
	for _, it := range items {
		avatar := it.Avatar
		if avatar == "" {
			avatar = defaultLink
		}
		if avatar == "" {
			avatar = "https://gcore.jsdelivr.net/gh/cdn-x/placeholder@1.0.12/avatar/round/3442075.svg"
		}
		snapshot := it.Snapshot
		if snapshot == "" {
			snapshot = avatar
		}
		desc := it.Desc
		if desc == "" {
			desc = it.URL
		}
		if it.Unavailable {
			sb.WriteString(`<div class="grid-cell site-card"><div class="card-link unavailable-card" aria-disabled="true">`)
		} else {
			sb.WriteString(`<div class="grid-cell site-card"><a class="card-link" target="_blank" rel="noopener" href="` + html.EscapeString(it.URL) + `">`)
		}
		sb.WriteString(`<div class="lazy-box snapshot"><img src="` + html.EscapeString(snapshot) + `" alt="` + html.EscapeString(it.Name) + `"></div>`)
		sb.WriteString(`<div class="info"><div class="lazy-box icon"><img src="` + html.EscapeString(avatar) + `" alt=""></div>`)
		sb.WriteString(`<span class="title">` + html.EscapeString(it.Name) + `</span>`)
		sb.WriteString(`<span class="desc">` + html.EscapeString(desc) + `</span></div>`)
		if it.Unavailable {
			sb.WriteString(`</div></div>`)
		} else {
			sb.WriteString(`</a></div>`)
		}
	}
	sb.WriteString(`</div></div>`)
}

// renderLinks 渲染 {% links %}：分组友链卡片流服务端直出，数据经 SetLinksFunc 注入。
func (s *Service) renderLinks() string {
	var groups []tmpl.LinkGroup
	defaultLink := ""
	if s.linksFunc != nil {
		groups, defaultLink = s.linksFunc()
	}
	return renderLinkGroupsHTML(groups, defaultLink)
}

// momentsLimit 解析 {% moments [limit:n] %} 的数量参数。
// 限制最大数量，避免作者误填超大值时一次请求拉取过多公开瞬间。
func momentsLimit(argsStr string) int {
	named, _ := parseNamedPositionalArgs(argsStr, []string{"limit"}, nil)
	limit := 5
	if n, err := strconv.Atoi(named["limit"]); err == nil && n > 0 {
		limit = n
	}
	if limit > 50 {
		limit = 50
	}
	return limit
}

// renderMoments 渲染 {% moments [limit:n] %}：读取当前站点最新公开瞬间，
// 输出与 /moments 页面相同的 timeline/timenode 结构，便于插入任意文章或独立页面。
func (s *Service) renderMoments(argsStr string) string {
	var items []*tmpl.MomentItem
	if s.momentsFunc != nil {
		items = s.momentsFunc(momentsLimit(argsStr))
	}

	var site *tmpl.Site
	if s.settings != nil {
		site = s.settings.Current()
	}
	return renderMomentsHTML(items, site)
}

// renderMomentsHTML 渲染瞬间时间流的公共 HTML 结构。
// 生产数据由 store 转换为 MomentItem；组件库预览也复用此函数，保证两处结构一致。
func renderMomentsHTML(items []*tmpl.MomentItem, site *tmpl.Site) string {
	if len(items) == 0 {
		return `<p class="cap" style="text-align:center;padding:2rem 0;color:var(--text-p3);">暂无瞬间</p>`
	}

	avatar := ""
	author := ""
	if site != nil {
		avatar = safeURL(site.Avatar)
		author = site.LogoTitle
		if author == "" {
			author = site.Title
		}
	}

	var sb strings.Builder
	sb.WriteString(`<div class="tag-plugin timeline">`)
	for _, item := range items {
		if item == nil {
			continue
		}
		sb.WriteString(`<div class="timenode"><div class="header"><div class="user-info">`)
		fmt.Fprintf(&sb, `<img src="%s" alt=""><span>%s</span>`, html.EscapeString(avatar), html.EscapeString(author))
		fmt.Fprintf(&sb, `</div><span title="%s">%s</span></div><div class="body">`, html.EscapeString(item.Date), html.EscapeString(item.TimeAgo))
		if item.ContentHTML != "" {
			sb.WriteString(string(item.ContentHTML))
		}
		if len(item.Images) > 0 {
			fmt.Fprintf(&sb, `<div class="tag-plugin gallery grid-box" size="%s" ratio="%s">`, html.EscapeString(item.GridSize()), html.EscapeString(item.GridRatio()))
			for _, image := range item.Images {
				image = safeURL(image)
				if image == "" {
					continue
				}
				fmt.Fprintf(&sb, `<div class="grid-cell lazy-box"><img class="lazy" data-fancybox="moment-%d" data-src="%s" alt=""><div class="lazy-icon"></div><div class="image-meta"></div></div>`, item.ID, html.EscapeString(image))
			}
			sb.WriteString(`</div>`)
		}
		sb.WriteString(`</div></div>`)
	}
	sb.WriteString(`</div>`)
	return sb.String()
}

// renderSites 渲染 {% sites api:url %}，容器交给 services/sites.js 异步渲染。
// 结构对齐原版 sites.js：.tag-plugin.sites-wrap > .data-service.ds-sites > .grid-box。
// 无本地站点库，api 参数必填透传（原版即外部 API 驱动），缺失时原样输出。
func renderSites(argsStr string) (string, bool) {
	named, _ := parseNamedPositionalArgs(argsStr, []string{"api"}, nil)
	api := named["api"]
	if api == "" {
		return "", false
	}
	var sb strings.Builder
	sb.WriteString(`<div class="tag-plugin sites-wrap">`)
	fmt.Fprintf(&sb, `<div class="data-service ds-sites" data-api="%s"><div class="grid-box"></div></div>`, html.EscapeString(api))
	sb.WriteString(`</div>`)
	return sb.String(), true
}

// renderInlineLabels 渲染行内修饰标签（对齐原版 inline-labels.js）：
// {% u/emp/wavy/del/kbd/psw/blur text %} → <tag>text</tag>；
// {% sup/sub [color:x] text %} → <sup class="tag-plugin colorful sup" color="x">text</sup>。
func renderInlineLabels(md string) string {
	md = reInlineLabel.ReplaceAllStringFunc(md, func(match string) string {
		parts := reInlineLabel.FindStringSubmatch(match)
		if len(parts) < 3 || strings.TrimSpace(parts[2]) == "" {
			return match
		}
		tag := parts[1]
		// 安全审计 2026-08-16：text 用户可控，转义后再输出
		return fmt.Sprintf(`<%s>%s</%s>`, tag, html.EscapeString(parts[2]), tag)
	})
	md = reSupSub.ReplaceAllStringFunc(md, func(match string) string {
		parts := reSupSub.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		tag := parts[1]
		named, positional := parseNamedPositionalArgs(parts[2], []string{"color"}, nil)
		text := strings.Join(positional, " ")
		if text == "" {
			return match
		}
		var sb strings.Builder
		fmt.Fprintf(&sb, `<%s class="tag-plugin colorful %s"`, tag, tag)
		// 安全审计 2026-08-16：color 用户可控，白名单后再拼入；text 转义后再输出
		if c := safeColor(named["color"]); c != "" {
			fmt.Fprintf(&sb, ` color="%s"`, html.EscapeString(c))
		}
		fmt.Fprintf(&sb, `>%s</%s>`, html.EscapeString(text), tag)
		return sb.String()
	})
	return md
}

// replaceOutsideCode 对代码区段（围栏代码块/行内代码）之外的文本应用替换函数，代码原样保留。
func replaceOutsideCode(md string, fn func(string) string) string {
	locs := reCodeSegment.FindAllStringIndex(md, -1)
	if len(locs) == 0 {
		return fn(md)
	}
	var sb strings.Builder
	last := 0
	for _, loc := range locs {
		sb.WriteString(fn(md[last:loc[0]]))
		sb.WriteString(md[loc[0]:loc[1]])
		last = loc[1]
	}
	sb.WriteString(fn(md[last:]))
	return sb.String()
}
