package post

import (
	"fmt"
	"html"
	"strings"
)

// ---------- Spec 21: GitHub widgets（ghrepo/ghuser 卡片） ----------

// languageColors 常见编程语言色点颜色，对齐 GitHub linguist。
var languageColors = map[string]string{
	"Go": "#00ADD8", "Python": "#3572A5", "JavaScript": "#F1E05A", "TypeScript": "#3178C6",
	"Java": "#B07219", "C": "#555555", "C++": "#F34B7D", "C#": "#178600",
	"Rust": "#DEA584", "Ruby": "#701516", "PHP": "#4F5D95", "Swift": "#F05138",
	"Kotlin": "#A97BFF", "Dart": "#00B4AB", "Shell": "#89E051", "HTML": "#E34C26",
	"CSS": "#563D7C", "Vue": "#41B883", "Lua": "#000080", "Zig": "#EC915C",
}

// GitHub 统计行图标：Octicon 16px 内联 SVG（MIT，@primer/octicons）。
// 原先用 ⭐/🍴/📄 emoji，在 0.75rem 的统计行里 emoji 字形被压扁，🍴 只剩两根细线，
// 看起来像没显示全；矢量图标与字号解耦，明暗配色沿用文字颜色。
const (
	ghIconStar = `<svg class="repo-stat-icon" viewBox="0 0 16 16" width="16" height="16" fill="currentColor" aria-hidden="true" focusable="false"><path d="M8 .25a.75.75 0 0 1 .673.418l1.882 3.815 4.21.612a.75.75 0 0 1 .416 1.279l-3.046 2.97.719 4.192a.751.751 0 0 1-1.088.791L8 12.347l-3.766 1.98a.75.75 0 0 1-1.088-.79l.72-4.194L.818 6.374a.75.75 0 0 1 .416-1.28l4.21-.611L7.327.668A.75.75 0 0 1 8 .25Z"/></svg>`
	ghIconFork = `<svg class="repo-stat-icon" viewBox="0 0 16 16" width="16" height="16" fill="currentColor" aria-hidden="true" focusable="false"><path d="M5 5.372v.878c0 .414.336.75.75.75h4.5a.75.75 0 0 0 .75-.75v-.878a2.25 2.25 0 1 1 1.5 0v.878a2.25 2.25 0 0 1-2.25 2.25h-1.5v2.128a2.251 2.251 0 1 1-1.5 0V8.5h-1.5A2.25 2.25 0 0 1 3.5 6.25v-.878a2.25 2.25 0 1 1 1.5 0ZM5 3.25a.75.75 0 1 0-1.5 0 .75.75 0 0 0 1.5 0Zm6.75.75a.75.75 0 1 0 0-1.5.75.75 0 0 0 0 1.5Zm-3 8.75a.75.75 0 1 0-1.5 0 .75.75 0 0 0 1.5 0Z"/></svg>`
	ghIconLaw  = `<svg class="repo-stat-icon" viewBox="0 0 16 16" width="16" height="16" fill="currentColor" aria-hidden="true" focusable="false"><path d="M8.75.75V2h.985c.304 0 .603.08.867.231l1.29.736c.038.022.08.033.124.033h2.234a.75.75 0 0 1 0 1.5h-.427l2.111 4.692a.75.75 0 0 1-.154.838l-.53-.53.529.531-.001.002-.002.002-.006.006-.006.005-.01.01-.045.04c-.21.176-.441.327-.686.45C14.556 10.78 13.88 11 13 11a4.498 4.498 0 0 1-2.023-.454 3.544 3.544 0 0 1-.686-.45l-.045-.04-.016-.015-.006-.006-.004-.004v-.001a.75.75 0 0 1-.154-.838L12.178 4.5h-.162c-.305 0-.604-.079-.868-.231l-1.29-.736a.245.245 0 0 0-.124-.033H8.75V13h2.5a.75.75 0 0 1 0 1.5h-6.5a.75.75 0 0 1 0-1.5h2.5V3.5h-.984a.245.245 0 0 0-.124.033l-1.289.737c-.265.15-.564.23-.869.23h-.162l2.112 4.692a.75.75 0 0 1-.154.838l-.53-.53.529.531-.001.002-.002.002-.006.006-.016.015-.045.04c-.21.176-.441.327-.686.45C4.556 10.78 3.88 11 3 11a4.498 4.498 0 0 1-2.023-.454 3.544 3.544 0 0 1-.686-.45l-.045-.04-.016-.015-.006-.006-.004-.004v-.001a.75.75 0 0 1-.154-.838L2.178 4.5H1.75a.75.75 0 0 1 0-1.5h2.234a.249.249 0 0 0 .125-.033l1.288-.737c.265-.15.564-.23.869-.23h.984V.75a.75.75 0 0 1 1.5 0Zm2.945 8.477c.285.135.718.273 1.305.273s1.02-.138 1.305-.273L13 6.327Zm-10 0c.285.135.718.273 1.305.273s1.02-.138 1.305-.273L3 6.327Z"/></svg>`
)

// renderGHRepo 渲染 {% ghrepo user/repo %} 为静态仓库卡片。API 失败时降级为纯链接。
func renderGHRepo(argsStr string, ghClient GHClient) string {
	repoPath := strings.TrimSpace(argsStr)
	if repoPath == "" {
		return ""
	}
	// 支持 "user/repo" 或完整 URL
	repoPath = strings.TrimPrefix(repoPath, "https://github.com/")
	parts := strings.SplitN(repoPath, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return fmt.Sprintf(`<a href="https://github.com/%s">%s</a>`, html.EscapeString(repoPath), html.EscapeString(repoPath))
	}
	owner, repo := parts[0], parts[1]
	fullURL := "https://github.com/" + owner + "/" + repo

	if ghClient == nil {
		return ghRepoFallback(fullURL, owner+"/"+repo)
	}
	info, err := ghClient.GetRepo(owner, repo)
	if err != nil {
		return ghRepoFallback(fullURL, owner+"/"+repo)
	}

	langDot := ""
	if info.Language != "" {
		color := languageColors[info.Language]
		if color == "" {
			color = "#8b8b8b"
		}
		langDot = fmt.Sprintf(`<span class="lang-dot" style="background:%s"></span><span>%s</span>`, color, html.EscapeString(info.Language))
	}

	var sb strings.Builder
	sb.WriteString(`<div class="tag-plugin ghcard"><a class="ghcard" href="` + html.EscapeString(fullURL) + `" target="_blank" rel="external nofollow noopener noreferrer">`)
	sb.WriteString(`<div class="ghrepo-card">`)
	// 头部：头像 + 全名
	sb.WriteString(`<div class="repo-header">`)
	if info.OwnerAvatar != "" {
		sb.WriteString(`<img class="repo-avatar" src="` + html.EscapeString(info.OwnerAvatar) + `&s=40" alt=""/>`)
	}
	sb.WriteString(`<span class="repo-name">` + html.EscapeString(info.FullName) + `</span>`)
	sb.WriteString(`</div>`)
	// 描述
	if info.Description != "" {
		sb.WriteString(`<div class="repo-desc">` + html.EscapeString(info.Description) + `</div>`)
	}
	// 统计
	sb.WriteString(`<div class="repo-stats">`)
	sb.WriteString(fmt.Sprintf(`<span class="repo-stat" aria-label="%d stars">%s%d</span>`, info.Stars, ghIconStar, info.Stars))
	sb.WriteString(fmt.Sprintf(`<span class="repo-stat" aria-label="%d forks">%s%d</span>`, info.Forks, ghIconFork, info.Forks))
	if langDot != "" {
		sb.WriteString(`<span class="repo-lang">` + langDot + `</span>`)
	}
	if info.License != "" {
		sb.WriteString(`<span class="repo-stat">` + ghIconLaw + html.EscapeString(info.License) + `</span>`)
	}
	sb.WriteString(`</div>`)
	sb.WriteString(`</div></a></div>`)
	return sb.String()
}

// ghRepoFallback API 失败时的纯链接降级。
func ghRepoFallback(url, name string) string {
	return fmt.Sprintf(`<div class="tag-plugin ghcard"><a class="ghcard" href="%s" target="_blank" rel="external nofollow noopener noreferrer"><div class="ghrepo-card"><div class="repo-header"><span class="repo-name">%s</span></div></div></a></div>`,
		html.EscapeString(url), html.EscapeString(name))
}

// renderGHUser 渲染 {% ghuser username %} 为静态用户卡片。API 失败时降级为纯链接。
func renderGHUser(argsStr string, ghClient GHClient) string {
	username := strings.TrimSpace(argsStr)
	if username == "" {
		return ""
	}
	username = strings.TrimPrefix(username, "https://github.com/")
	username = strings.TrimPrefix(username, "@")
	fullURL := "https://github.com/" + username

	if ghClient == nil {
		return ghUserFallback(fullURL, username)
	}
	info, err := ghClient.GetUser(username)
	if err != nil {
		return ghUserFallback(fullURL, username)
	}

	displayName := info.Name
	if displayName == "" {
		displayName = info.Login
	}

	var sb strings.Builder
	sb.WriteString(`<div class="tag-plugin ghcard"><a class="ghcard" href="` + html.EscapeString(fullURL) + `" target="_blank" rel="external nofollow noopener noreferrer">`)
	sb.WriteString(`<div class="ghuser-card">`)
	// 头像 + 名称
	sb.WriteString(`<div class="user-header">`)
	if info.AvatarURL != "" {
		sb.WriteString(`<img class="user-avatar" src="` + html.EscapeString(info.AvatarURL) + `&s=64" alt=""/>`)
	}
	sb.WriteString(`<div class="user-info">`)
	sb.WriteString(`<span class="user-name">` + html.EscapeString(displayName) + `</span>`)
	sb.WriteString(`<span class="user-login">@` + html.EscapeString(info.Login) + `</span>`)
	sb.WriteString(`</div></div>`)
	// Bio
	if info.Bio != "" {
		sb.WriteString(`<div class="user-bio">` + html.EscapeString(info.Bio) + `</div>`)
	}
	// 统计
	sb.WriteString(`<div class="user-stats">`)
	sb.WriteString(fmt.Sprintf(`<span>%d followers</span>`, info.Followers))
	sb.WriteString(fmt.Sprintf(`<span>%d repos</span>`, info.Repos))
	sb.WriteString(`</div>`)
	sb.WriteString(`</div></a></div>`)
	return sb.String()
}

// ghUserFallback API 失败时的纯链接降级。
func ghUserFallback(url, username string) string {
	return fmt.Sprintf(`<div class="tag-plugin ghcard"><a class="ghcard" href="%s" target="_blank" rel="external nofollow noopener noreferrer"><div class="ghuser-card"><div class="user-header"><div class="user-info"><span class="user-name">@%s</span></div></div></div></a></div>`,
		html.EscapeString(url), html.EscapeString(username))
}

func renderGHCard(argsStr string, ghClient GHClient) string {
	params := []string{"show_owner", "theme", "title_color", "text_color", "icon_color", "bg_color", "hide_border", "cache_seconds", "locale"}
	named, _ := parseNamedPositionalArgs(argsStr, params, []string{"repo"})
	path := named["repo"]
	if path == "" {
		return ""
	}
	if strings.Contains(path, "/") {
		return renderGHRepo(path, ghClient)
	}
	return renderGHUser(path, ghClient)
}

// renderPoetry 渲染 {% poetry [title] [author:] [date:] [footer:] %} body {% endpoetry %}。
