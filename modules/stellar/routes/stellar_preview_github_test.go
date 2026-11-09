package routes

import (
	"strings"
	"testing"

	"blog/internal/content"
	"blog/internal/post"
	stellarweb "blog/modules/stellar/web"
)

// previewGitHubClient 固定 GitHub 数据，验证预览走真实卡片渲染而不是 data-service 空壳。
type previewGitHubClient struct{ calls int }

func (c *previewGitHubClient) GetRepo(owner, repo string) (*post.GHRepoInfo, error) {
	c.calls++
	return &post.GHRepoInfo{
		Name: repo, FullName: owner + "/" + repo, Description: "预览卡片描述",
		Stars: 42, Forks: 7, Language: "Go", License: "MIT License",
		OwnerAvatar: "https://avatars.example/u", OwnerLogin: owner,
		HTMLURL: "https://github.com/" + owner + "/" + repo,
	}, nil
}

func (c *previewGitHubClient) GetUser(username string) (*post.GHUserInfo, error) {
	c.calls++
	return &post.GHUserInfo{
		Login: username, Name: "预览用户", Bio: "预览用户简介",
		AvatarURL: "https://avatars.example/u", Followers: 9, Following: 3, Repos: 12,
		HTMLURL: "https://github.com/" + username,
	}, nil
}

// 编辑器预览必须渲染与前台一致的 GitHub 卡片（带卡片结构、星标与简介）。
// 回归背景：预览曾复用图片快照的 Offline 语义，blog_ghrepo/blog_ghuser 只输出
// 无样式的 data-service 空壳，正文只剩一行裸链接（BLG-20261108-01）。
func TestStellarContentPreviewRendersGitHubCards(t *testing.T) {
	ws, st, _, _ := stellarRouteFixture(t)
	createStellarPost(t, st, "one", "")
	engine, err := stellarweb.NewEngineFromEmbed(ws.icons)
	if err != nil {
		t.Fatal(err)
	}
	ws.engine = engine
	gh := &previewGitHubClient{}
	ws.posts.SetGHClient(gh)
	markdown := "{% blog_ghrepo vuejs/vue %}\n\n{% blog_ghuser octocat %}"
	input := content.StellarPreviewInput{
		StellarVersion: 2,
		StellarConfig:  `{"title":"Preview"}`,
		Title:          "Preview",
		Layout:         "post",
		Path:           "/post/one",
		RequestURL:     "http://127.0.0.1:8109/console/posts",
		PreviewToken:   "github-card-preview",
		Markdown:       &markdown,
		Mode:           content.StellarPreviewModeContent,
	}
	output, err := ws.renderContentPreviewSnapshot(input)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"ghrepo-card", "repo-name", "vuejs/vue", "预览卡片描述",
		"repo-stat-icon", `aria-label="42 stars"`, `aria-label="7 forks"`,
		`class="lang-dot"`, ">MIT License</span>",
		"ghuser-card", "user-name", "预览用户", "@octocat", "预览用户简介", "9 followers",
		// 卡片外观来自项目补充样式；预览文档缺少它时卡片高度为 0、只剩裸文本。
		"/assets/css/tag-plugins.css",
	} {
		if !strings.Contains(output.Document, want) {
			t.Errorf("preview missing GitHub card content %q", want)
		}
	}
	for _, unwanted := range []string{"ds-ghinfo ds-ghrepo", "ds-ghinfo ds-ghuser", `<span type="text" id="full_name">`} {
		if strings.Contains(output.Document, unwanted) {
			t.Errorf("preview kept offline data-service shell %q", unwanted)
		}
	}
	if gh.calls != 2 {
		t.Fatalf("preview GitHub client calls=%d, want 2 (repo + user)", gh.calls)
	}
}
