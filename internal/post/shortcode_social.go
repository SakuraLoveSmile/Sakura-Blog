package post

import (
	tmpl "blog/internal/settings"
	"fmt"
	"html"
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
	for _, g := range groups {
		if g.Title != "" {
			fmt.Fprintf(&sb, "<h2>%s</h2>", html.EscapeString(g.Title))
		}
		sb.WriteString(`<div class="tag-plugin sites-wrap"><div class="grid-box">`)
		for _, it := range g.Items {
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
			sb.WriteString(`<div class="grid-cell site-card"><a class="card-link" target="_blank" rel="noopener" href="` + html.EscapeString(it.URL) + `">`)
			sb.WriteString(`<div class="lazy-box snapshot"><img src="` + html.EscapeString(snapshot) + `" alt="` + html.EscapeString(it.Name) + `"></div>`)
			sb.WriteString(`<div class="info"><div class="lazy-box icon"><img src="` + html.EscapeString(avatar) + `" alt=""></div>`)
			sb.WriteString(`<span class="title">` + html.EscapeString(it.Name) + `</span>`)
			sb.WriteString(`<span class="desc">` + html.EscapeString(desc) + `</span></div></a></div>`)
		}
		sb.WriteString(`</div></div>`)
	}
	return sb.String()
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
