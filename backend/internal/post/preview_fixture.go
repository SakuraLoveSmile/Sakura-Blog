package post

// Spec 63：组件库标签预览 Fixture 支持（preview_profile=component-library）。
// 预览不依赖公网、不写入真实数据，仍能展示标签的结构、样式、交互与典型数据状态：
//   - ghrepo/ghuser/ghcard 使用确定性 GitHub Fixture（不发网络请求）；
//   - {% links %} 友链分组使用确定性 Fixture；
//   - rating/vote/friends/sites/md 的 data-api/src 改写为本地 /api/v1/preview/* Fixture 端点，
//     其中 rating/vote 的 data-id 加 preview- 前缀（隔离 ID，永不与真实内容冲突）；
//   - Fixture 只服务预览：不新增数据库迁移、不持久化用户操作，线上渲染与 API 行为不变。

import (
	"net/url"
	"regexp"
	"strings"

	tmpl "blog/internal/template"
)

// ---------- GitHub Fixture ----------

// fixtureGHClient 确定性 GitHub Fixture：任意 owner/repo、username 返回固定示例数据。
type fixtureGHClient struct{}

func (fixtureGHClient) GetRepo(owner, repo string) (*GHRepoInfo, error) {
	return &GHRepoInfo{
		Name:        repo,
		FullName:    owner + "/" + repo,
		Description: "预览 Fixture 仓库：组件库预览使用的确定性示例数据。",
		Stars:       12800,
		Forks:       864,
		Language:    "Go",
		License:     "MIT License",
		OwnerAvatar: "https://avatars.githubusercontent.com/u/1?v=4",
		OwnerLogin:  owner,
		HTMLURL:     "https://github.com/" + owner + "/" + repo,
	}, nil
}

func (fixtureGHClient) GetUser(username string) (*GHUserInfo, error) {
	return &GHUserInfo{
		Login:     username,
		Name:      "Fixture User",
		Bio:       "预览 Fixture 用户：组件库预览使用的确定性示例资料。",
		AvatarURL: "https://avatars.githubusercontent.com/u/2?v=4",
		Followers: 3200,
		Following: 128,
		Repos:     96,
		HTMLURL:   "https://github.com/" + username,
	}, nil
}

// ---------- 友链 Fixture ----------

// fixtureLinksRender 确定性友链分组（{% links %} 服务端直出用）。
// 头像用内联 SVG data URI（离线可用，不依赖外链占位图）。
func fixtureLinksRender() ([]tmpl.LinkGroup, string) {
	return []tmpl.LinkGroup{
		{
			Title: "推荐站点",
			Items: []tmpl.LinkItem{
				{Name: "Fixture Blog", URL: "https://example.com", Desc: "预览 Fixture 示例站点", Avatar: fixtureAvatar("B")},
				{Name: "Fixture Docs", URL: "https://docs.example.com", Desc: "示例文档站", Avatar: fixtureAvatar("D")},
			},
		},
		{
			Title: "小伙伴",
			Items: []tmpl.LinkItem{
				{Name: "Alice", URL: "https://alice.example.com", Desc: "前端工程师", Avatar: fixtureAvatar("A")},
				{Name: "Bob", URL: "https://bob.example.com", Desc: "后端工程师", Avatar: fixtureAvatar("B")},
			},
		},
	}, ""
}

// fixtureAvatar 生成离线可用的内联 SVG 圆形头像 data URI（首字母白字）。
func fixtureAvatar(letter string) string {
	return "data:image/svg+xml;charset=utf-8," + url.PathEscape(
		"<svg xmlns='http://www.w3.org/2000/svg' width='96' height='96'><rect width='96' height='96' rx='48' fill='#4a6cf7'/><text x='48' y='62' font-size='40' text-anchor='middle' fill='white'>"+letter+"</text></svg>")
}

// fixtureLinksHTML 渲染 {% links %} 的 Fixture 直出 HTML。
func fixtureLinksHTML() string {
	groups, defaultLink := fixtureLinksRender()
	return renderLinkGroupsHTML(groups, defaultLink)
}

// ---------- 数据端点改写 ----------

var (
	// ds-rating：data-api=/api/rating → /api/v1/preview/rating，data-id 前缀 preview-
	rePreviewRatingAPIID = regexp.MustCompile(`(class="[^"]*\bds-rating\b[^"]*" data-api=")/api/rating(" data-id=")([^"]*)(")`)
	// ds-vote：data-api=/api/vote → /api/v1/preview/vote，data-id 前缀 preview-
	rePreviewVoteAPIID = regexp.MustCompile(`(class="[^"]*\bds-vote\b[^"]*" data-api=")/api/vote(" data-id=")([^"]*)(")`)
	// ds-friends / ds-friends_and_posts：data-api 一律指向本地 Fixture 端点（外部 URL 不直连）
	rePreviewFriendsAPI = regexp.MustCompile(`(class="[^"]*\bds-friends(?:_and_posts)?\b[^"]*" data-api=")[^"]*(")`)
	// ds-sites：data-api 一律指向本地 Fixture 端点
	rePreviewSitesAPI = regexp.MustCompile(`(class="[^"]*\bds-sites\b[^"]*" data-api=")[^"]*(")`)
	// ds-mdrender：src 改写为本地 Fixture 端点（原始 src 编码进查询参数）
	rePreviewMDSrc = regexp.MustCompile(`(class="[^"]*\bds-mdrender\b[^"]*" src=")([^"]*)(")`)
)

// rewritePreviewDataAPIs 把渲染产物中的数据标签端点改写为本地 Fixture 端点。
// 纯字符串改写，作用于 goldmark 输出后的 HTML（Unsafe 模式原样保留 raw HTML）。
func rewritePreviewDataAPIs(html string) string {
	html = rePreviewRatingAPIID.ReplaceAllString(html, `${1}/api/v1/preview/rating${2}preview-${3}${4}`)
	html = rePreviewVoteAPIID.ReplaceAllString(html, `${1}/api/v1/preview/vote${2}preview-${3}${4}`)
	html = rePreviewFriendsAPI.ReplaceAllString(html, `${1}/api/v1/preview/friends${2}`)
	html = rePreviewSitesAPI.ReplaceAllString(html, `${1}/api/v1/preview/sites${2}`)
	html = rePreviewMDSrc.ReplaceAllStringFunc(html, func(m string) string {
		sub := rePreviewMDSrc.FindStringSubmatch(m)
		if len(sub) != 4 {
			return m
		}
		return sub[1] + "/api/v1/preview/md?src=" + url.QueryEscape(sub[2]) + sub[3]
	})
	return html
}

// ---------- 组件库 Fixture 渲染入口 ----------

// RenderComponentPreview 组件库 Fixture 渲染：与 Render 共用同一 goldmark 管线，
// 但 GitHub/友链使用确定性 Fixture，数据标签端点改写为本地 Fixture 路径。
func (s *Service) RenderComponentPreview(body string) (string, error) {
	renderMD := func(md string) string {
		md = protectMathDelimiters(md)
		var sb strings.Builder
		if err := s.md.Convert([]byte(md), &sb); err != nil {
			return md
		}
		return sb.String()
	}
	processed := processShortcodes(protectMathDelimiters(body), renderMD, s.renderInlineMD, s.iconFunc, fixtureGHClient{}, fixtureLinksHTML)
	var sb strings.Builder
	if err := s.md.Convert([]byte(processed), &sb); err != nil {
		return "", err
	}
	return rewritePreviewDataAPIs(sb.String()), nil
}
