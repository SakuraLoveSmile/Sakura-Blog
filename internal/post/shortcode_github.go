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
	sb.WriteString(fmt.Sprintf(`<span>⭐ %d</span>`, info.Stars))
	sb.WriteString(fmt.Sprintf(`<span>🍴 %d</span>`, info.Forks))
	if langDot != "" {
		sb.WriteString(`<span class="repo-lang">` + langDot + `</span>`)
	}
	if info.License != "" {
		sb.WriteString(`<span>📄 ` + html.EscapeString(info.License) + `</span>`)
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
