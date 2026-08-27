package post

import (
	"strings"
	"testing"

	"blog/internal/settings"
)

// ---------- Spec 63：组件库预览 Fixture ----------

func TestRewritePreviewDataAPIs(t *testing.T) {
	// 覆盖：rating/vote 端点改写 + data-id 隔离前缀；friends/sites 外部 URL 一律指向本地 Fixture；
	// md src 编码进查询参数（来源标识，fixture 端点不直连）；非 preview 产物不受影响。
	in := `<div class="tag-plugin ds-rating" data-api="/api/rating" data-id="demo-post"><div class="body"></div></div>` +
		`<div class="tag-plugin ds-vote" data-api="/api/vote" data-id="demo-post"></div>` +
		`<div class="tag-plugin users-wrap"><div class="data-service ds-friends" data-api="https://example.com/data.json"><div class="grid-box"></div></div></div>` +
		`<div class="tag-plugin sites-wrap"><div class="data-service ds-sites" data-api="https://example.com/sites.json"><div class="grid-box"></div></div></div>` +
		`<div class="data-service ds-mdrender" src="https://raw.githubusercontent.com/x/y/main/README.md" id="md_1"></div>`
	out := rewritePreviewDataAPIs(in)
	for _, want := range []string{
		`data-api="/api/v1/preview/rating"`,
		`data-id="preview-demo-post"`,
		`data-api="/api/v1/preview/vote"`,
		`data-api="/api/v1/preview/friends"`,
		`data-api="/api/v1/preview/sites"`,
		`src="/api/v1/preview/md?src=`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("rewrite missing %q in: %s", want, out)
		}
	}
	// 原外部端点不得作为直连残留（编码进查询参数的来源标识除外）
	for _, bad := range []string{`/api/rating"`, `/api/vote"`, "example.com/data.json", "example.com/sites.json"} {
		if strings.Contains(out, bad) {
			t.Errorf("rewrite should drop %q, got: %s", bad, out)
		}
	}
	// md src 查询参数编码：原 URL 的 / 与 : 被编码，属性不被引号/& 破坏
	if !strings.Contains(out, `src="/api/v1/preview/md?src=https%3A%2F%2Fraw.githubusercontent.com%2Fx%2Fy%2Fmain%2FREADME.md"`) {
		t.Errorf("md src should be query-escaped, got: %s", out)
	}
}

func TestRewritePreviewDataAPIsFriendsAndPosts(t *testing.T) {
	// friends posts:true 走 ds-friends_and_posts，同样改写为本地 Fixture 端点
	in := `<div class="tag-plugin users-posts-wrap"><div class="data-service ds-friends_and_posts" data-api="/api/friends-aggregated"><div class="grid-box"></div></div></div>`
	out := rewritePreviewDataAPIs(in)
	if !strings.Contains(out, `data-api="/api/v1/preview/friends"`) {
		t.Errorf("friends_and_posts rewrite missing: %s", out)
	}
}

func TestFixtureGHClient(t *testing.T) {
	repo, err := (fixtureGHClient{}).GetRepo("vuejs", "vue")
	if err != nil {
		t.Fatalf("fixture GetRepo: %v", err)
	}
	if repo.FullName != "vuejs/vue" || repo.Stars != 12800 || repo.Language != "Go" {
		t.Errorf("fixture repo unexpected: %+v", repo)
	}
	user, err := (fixtureGHClient{}).GetUser("octocat")
	if err != nil {
		t.Fatalf("fixture GetUser: %v", err)
	}
	if user.Login != "octocat" || user.Followers != 3200 {
		t.Errorf("fixture user unexpected: %+v", user)
	}
}

func TestRenderComponentPreview(t *testing.T) {
	s := NewService(t.TempDir())
	out, err := s.RenderComponentPreview("{% ghrepo vuejs/vue %}\n\n{% rating id:demo-post %}\n\n{% friends %}\n\n{% vote 有帮助吗 id:post-1 %}")
	if err != nil {
		t.Fatalf("RenderComponentPreview: %v", err)
	}
	for _, want := range []string{
		// ghrepo 使用确定性 Fixture 数据
		`<span class="repo-name">vuejs/vue</span>`,
		`>⭐ 12800<`,
		// rating：端点改写 + preview- 隔离 ID
		`data-api="/api/v1/preview/rating"`,
		`data-id="preview-demo-post"`,
		// vote：端点改写 + preview- 隔离 ID
		`data-api="/api/v1/preview/vote"`,
		`data-id="preview-post-1"`,
		// friends：缺省端点改写为本地 Fixture
		`data-api="/api/v1/preview/friends"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("component preview missing %q in: %s", want, out)
		}
	}
}

func TestRenderComponentPreviewStaticPlugins(t *testing.T) {
	s := NewService(t.TempDir())
	out, err := s.RenderComponentPreview("{% note color:green 提示 这是提示内容 %}\n\n{% banner title:我的博客 %}\n欢迎。\n{% endbanner %}\n\n{% poetry 静夜思 author:李白 %}\n床前明月光。\n{% endpoetry %}\n\n{% frame device:iphone width:320px %}\n![s](https://example.com/s.png)\n{% endframe %}")
	if err != nil {
		t.Fatalf("RenderComponentPreview: %v", err)
	}
	for _, want := range []string{
		`class="tag-plugin colorful note"`,
		`color="green"`,
		`class="tag-plugin banner"`,
		`class="tag-plugin poetry"`,
		`device="iphone"`,
		`class="tag-plugin frame"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("static plugin preview missing %q in: %s", want, out)
		}
	}
}

func TestRenderComponentPreviewInteractivePlugins(t *testing.T) {
	s := NewService(t.TempDir())
	out, err := s.RenderComponentPreview("{% tabs %}\n<!-- tab 标签一 -->\n第一页。\n<!-- tab 标签二 -->\n第二页。\n{% endtabs %}\n\n{% folding color:cyan open:true 详情 %}\n折叠内容。\n{% endfolding %}\n\n{% swiper %}\n![](https://example.com/1.png)\n![](https://example.com/2.png)\n{% endswiper %}")
	if err != nil {
		t.Fatalf("RenderComponentPreview: %v", err)
	}
	for _, want := range []string{
		`class="tag-plugin tabs"`,
		`<details class="tag-plugin colorful folding"`,
		`class="tag-plugin swiper fancybox"`,
		`<div class="swiper-slide">`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("interactive plugin preview missing %q in: %s", want, out)
		}
	}
}

func TestRenderComponentPreviewExternalPlugins(t *testing.T) {
	s := NewService(t.TempDir())
	out, err := s.RenderComponentPreview("{% mermaid %}\ngraph TB\n A --> B\n{% endmermaid %}\n\n{% md https://raw.githubusercontent.com/xaoxuu/hexo-theme-stellar/main/README.md %}")
	if err != nil {
		t.Fatalf("RenderComponentPreview: %v", err)
	}
	// mermaid：源码保留在容器内（客户端 math-mermaid-loader 负责渲染，失败降级为源码）
	if !strings.Contains(out, `<div class="mermaid">graph TB`) {
		t.Errorf("mermaid fixture missing: %s", out)
	}
	// md：src 改写为本地 Fixture 端点
	if !strings.Contains(out, `class="data-service ds-mdrender" src="/api/v1/preview/md?src=`) {
		t.Errorf("md fixture rewrite missing: %s", out)
	}
}

func TestRenderComponentPreviewLinksFixture(t *testing.T) {
	// {% links %} 在 Fixture 模式下服务端直出确定性分组（不依赖站点 links 配置注入）
	s := NewService(t.TempDir())
	out, err := s.RenderComponentPreview("{% links %}")
	if err != nil {
		t.Fatalf("RenderComponentPreview: %v", err)
	}
	for _, want := range []string{
		`<h2>推荐站点</h2>`,
		`<h2>小伙伴</h2>`,
		`<span class="title">Fixture Blog</span>`,
		`<span class="title">Alice</span>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("links fixture missing %q in: %s", want, out)
		}
	}
}

func TestRenderMomentsShortcodeUsesInjectedItems(t *testing.T) {
	s := NewService(t.TempDir())
	requestedLimit := 0
	s.SetSettingsManager(settings.NewManager(&settings.Site{Avatar: "/avatar.png", LogoTitle: "本站"}))
	s.SetMomentsFunc(func(limit int) []*settings.MomentItem {
		requestedLimit = limit
		return []*settings.MomentItem{
			{ID: 7, ContentHTML: "<p>公开动态</p>", TimeAgo: "刚刚", Date: "2026-08-27 10:00", Images: []string{"/one.png"}},
		}
	})

	out, err := s.Render("{% moments limit:2 %}")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if requestedLimit != 2 {
		t.Fatalf("moments shortcode should pass limit 2, got %d", requestedLimit)
	}
	for _, want := range []string{
		`<div class="tag-plugin timeline">`,
		`<div class="timenode">`,
		`<span>本站</span>`,
		`<p>公开动态</p>`,
		`data-fancybox="moment-7"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("moments shortcode missing %q in: %s", want, out)
		}
	}
}

func TestRenderComponentPreviewMomentsFixture(t *testing.T) {
	s := NewService(t.TempDir())
	out, err := s.RenderComponentPreview("{% moments limit:1 %}")
	if err != nil {
		t.Fatalf("RenderComponentPreview: %v", err)
	}
	for _, want := range []string{
		`<span>Fixture Blog</span>`,
		`<p>今天完成了瞬间组件的预览接入。</p>`,
		`data-fancybox="moment-1001"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("moments fixture missing %q in: %s", want, out)
		}
	}
	if strings.Contains(out, "这是可以插入任意页面的公开动态时间流") {
		t.Errorf("limit:1 should omit the second fixture item: %s", out)
	}
}

// ---------- Spec 64：暗色预览收尾（本地资源 / emoji / TOC / albums-posters variant） ----------

func TestRenderComponentPreviewEmojiLocalized(t *testing.T) {
	// component-library 模式：内置 emoji 源（qq/tieba/twemoji）CDN 地址改写为本地素材；
	// url: 分支的真实外链不受影响。
	s := NewService(t.TempDir())
	out, err := s.RenderComponentPreview("开心 {% emoji aini %} 点赞 {% emoji twemoji 1f44d %} 滑稽 {% emoji tieba huaji %}")
	if err != nil {
		t.Fatalf("RenderComponentPreview: %v", err)
	}
	for _, want := range []string{
		`src="/assets/img/preview/emoji/qq-aini.gif"`,
		`src="/assets/img/preview/emoji/twemoji-1f44d.svg"`,
		`src="/assets/img/preview/emoji/tieba-huaji.png"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("emoji fixture missing %q in: %s", want, out)
		}
	}
	if strings.Contains(out, "cdn.jsdelivr.net") {
		t.Errorf("emoji fixture should not keep CDN url: %s", out)
	}
	// url: 分支保持用户提供的真实 URL
	outURL, err := s.RenderComponentPreview("{% emoji url:https://example.com/x.png 名称 %}")
	if err != nil {
		t.Fatalf("RenderComponentPreview url: %v", err)
	}
	if !strings.Contains(outURL, `src="https://example.com/x.png"`) {
		t.Errorf("url: branch should keep real url: %s", outURL)
	}
	// 无 profile 的普通渲染保持 CDN 源（生产行为不变）
	outNormal, err := s.Render("开心 {% emoji aini %}")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(outNormal, "cdn.jsdelivr.net/gh/cdn-x/emoticons@3.1.0/qq/aini.gif") {
		t.Errorf("normal render should keep CDN emoji url: %s", outNormal)
	}
}

func TestRenderComponentPreviewTOCFixture(t *testing.T) {
	// component-library 模式：{% toc %} 的 details 壳内填充确定性两级目录；
	// 无 profile 时保持空壳（生产行为不变）。
	s := NewService(t.TempDir())
	out, err := s.RenderComponentPreview("{% toc 本文目录 %}")
	if err != nil {
		t.Fatalf("RenderComponentPreview: %v", err)
	}
	for _, want := range []string{
		`<details class="toc" open><summary>本文目录</summary>`,
		`<ol class="toc"`,
		`第一章 背景`,
		`1.1 项目动机`,
		`第二章 方案`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("toc fixture missing %q in: %s", want, out)
		}
	}
	outNormal, err := s.Render("{% toc 本文目录 %}")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if strings.Contains(outNormal, "第一章 背景") {
		t.Errorf("normal render should keep empty toc shell: %s", outNormal)
	}
}

func TestRenderComponentPreviewGHFixtureLocalAvatar(t *testing.T) {
	// GitHub Fixture 头像为本地素材，链接保留真实 GitHub 地址。
	s := NewService(t.TempDir())
	out, err := s.RenderComponentPreview("{% ghrepo vuejs/vue %}\n\n{% ghuser octocat %}")
	if err != nil {
		t.Fatalf("RenderComponentPreview: %v", err)
	}
	if !strings.Contains(out, `/assets/img/preview/avatar-gh-vuejs.svg?v=4&s=40`) {
		t.Errorf("ghrepo avatar should be local with resize params: %s", out)
	}
	if !strings.Contains(out, `/assets/img/preview/avatar-gh-octocat.svg?v=4&s=64`) {
		t.Errorf("ghuser avatar should be local with resize params: %s", out)
	}
	if strings.Contains(out, "avatars.githubusercontent.com") {
		t.Errorf("fixture should not hit github avatar cdn: %s", out)
	}
	if !strings.Contains(out, `https://github.com/vuejs/vue`) {
		t.Errorf("fixture should keep real github link: %s", out)
	}
}

func TestRewritePreviewDataAPIsAlbumsPostersVariant(t *testing.T) {
	// albums/posters 的 ds-friends data-api 改写为带 variant 的本地 Fixture 端点；
	// 普通 friends 改写不带 variant。
	in := `<div class="tag-plugin albums-wrap"><div class="data-service ds-friends" data-api="https://example.com/data.json"><div class="tag-plugin gallery grid-box"></div></div></div>` +
		`<div class="tag-plugin posters-wrap"><div class="data-service ds-friends" data-api="https://example.com/posters.json"><div class="tag-plugin gallery grid-box"></div></div></div>` +
		`<div class="tag-plugin users-wrap"><div class="data-service ds-friends" data-api="/api/friends"><div class="grid-box"></div></div></div>`
	out := rewritePreviewDataAPIs(in)
	for _, want := range []string{
		`data-api="/api/v1/preview/friends?variant=albums"`,
		`data-api="/api/v1/preview/friends?variant=posters"`,
		`data-api="/api/v1/preview/friends"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("rewrite missing %q in: %s", want, out)
		}
	}
	if strings.Contains(out, "example.com") {
		t.Errorf("rewrite should drop external api url: %s", out)
	}
}

func TestRenderComponentPreviewDefaultExamplesOffline(t *testing.T) {
	// 代表性默认示例经 component-library 渲染后，资源属性（src/data-src/data-api/source）
	// 不得残留公网地址（href 链接除外）；emoji/md/albums/sites 均被本地化改写。
	s := NewService(t.TempDir())
	examples := []string{
		"{% gallery %}\n![a](/assets/img/preview/gallery-1.jpg)\n{% endgallery %}",
		"{% banner title:t bg:/assets/img/preview/banner-bg.jpg avatar:/assets/img/preview/avatar-blog.svg %}b{% endbanner %}",
		"{% frame device:iphone width:320px %}\n![s](/assets/img/preview/frame-shot.jpg)\n{% endframe %}",
		"{% about avatar:/assets/img/preview/avatar-blog.svg %}a{% endabout %}",
		"{% image /assets/img/preview/image-demo.jpg 图 width:80% download:true %}",
		"{% video /assets/media/preview/sample-video.mp4 %}",
		"{% audio /assets/media/preview/sample-audio.mp3 %}",
		"{% swiper %}\n![](/assets/img/preview/swiper-1.jpg)\n{% endswiper %}",
		"{% md /assets/md/preview/example.md %}",
		"{% albums api:https://example.com/data.json size:m %}",
		"{% posters api:https://example.com/posters.json size:s %}",
		"{% sites api:https://example.com/sites.json %}",
		"{% chat iphone style:qq title:群 me:b %}\nalice:\n name: 小明\n avatar: /assets/img/preview/avatar-xiaoming.svg\n<!-- chatcell user:alice md:hi -->\n{% endchat %}",
		"{% emoji aini %}",
		"{% toc 目录 %}",
		"{% mermaid %}\ngraph TB\n A[开始] --> B[结束]\n{% endmermaid %}",
	}
	for _, ex := range examples {
		out, err := s.RenderComponentPreview(ex)
		if err != nil {
			t.Fatalf("RenderComponentPreview(%q): %v", ex, err)
		}
		for _, attr := range []string{`src="http`, `data-src="http`, `data-api="http`, `srcset="http`, `<source src="http`} {
			if strings.Contains(out, attr) {
				t.Errorf("example %q leaves external %s in: %s", ex, attr, out)
			}
		}
	}
}
