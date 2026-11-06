package routes

import (
	"bytes"
	"net/http"
	"strings"
	"testing"

	"blog/internal/content"
	"blog/internal/settings"
	"blog/internal/store"
	stellarweb "blog/modules/stellar/web"

	"github.com/goccy/go-yaml"
	"golang.org/x/net/html"
)

const friendsInfoTestMarkdown = `## 友链列表

{% friends api:/api/friends-aggregated posts:true %}

## 如何交换友链？

交换说明正文。

## 后续说明

其他内容。
`

func TestStellarFriendsSiteInfoUsesLiveSettingsWithoutChangingPages(t *testing.T) {
	ws, st, _, router := stellarRouteFixture(t)
	site := *ws.site.Load()
	site.Title, site.URL = `我的博客: "记录"`, "https://public.example/blog"
	site.Avatar, site.Description = "/avatar.png", "记录生活与技术。\n第二行: \"带引号\" # 保留原文\n"
	showPosts := false
	site.Pages.Friends.ShowPosts = &showPosts
	ws.site.Store(&site)
	engine, err := stellarweb.NewEngineFromEmbed(ws.icons)
	if err != nil {
		t.Fatal(err)
	}
	ws.engine = engine
	for _, slug := range []string{FriendsSlug, "unrelated"} {
		if _, err := st.CreatePage(&store.Page{Slug: slug, Title: "页面标题", ContentMD: friendsInfoTestMarkdown,
			ContentHTML: "<p>保存时的缓存</p>", Status: store.StatusPublished, StellarVersion: 2, StellarConfig: `{}`}); err != nil {
			t.Fatal(err)
		}
	}
	before, err := st.GetPageBySlug(FriendsSlug)
	if err != nil {
		t.Fatal(err)
	}
	render := func(path string) string {
		t.Helper()
		response := stellarRequest(router, path)
		if response.Code != http.StatusOK {
			t.Fatalf("%s => %d: %s", path, response.Code, response.Body.String())
		}
		return response.Body.String()
	}

	rendered := render("/friends")
	if !strings.Contains(rendered, "/assets/css/friends-page.css?v=") {
		t.Fatal("正式友链页缺少专用标题样式")
	}
	page := friendsInfoTestOneClass(t, rendered, "friends-page")
	info := friendsInfoTestOneClass(t, friendsInfoTestHTML(t, page), "friends-site-info")
	data := friendsInfoTestYAML(t, info)
	for key, want := range map[string]string{
		"title": site.Title, "url": settings.NormalizeSiteURL(site.URL), "avatar": "https://public.example/avatar.png",
		"description": site.Description, "feed": "https://public.example/blog/atom.xml",
	} {
		if data[key] != want {
			t.Errorf("本站资料整段复制后 %s = %q，期望 %q", key, data[key], want)
		}
	}
	markup := friendsInfoTestHTML(t, page)
	previous, next := info.PrevSibling, info.NextSibling
	for previous != nil && previous.Type == html.TextNode && strings.TrimSpace(previous.Data) == "" {
		previous = previous.PrevSibling
	}
	for next != nil && next.Type == html.TextNode && strings.TrimSpace(next.Data) == "" {
		next = next.NextSibling
	}
	if previous == nil || next == nil || friendsInfoTestText(previous) != "交换说明正文。" ||
		next.Data != "h2" || friendsInfoTestText(next) != "后续说明" {
		t.Fatal("本站资料未放在交换说明后、下一分区前")
	}
	if !strings.Contains(markup, `data-show-posts="false"`) {
		t.Fatal("增加本站资料改变了友链动态开关")
	}
	for _, class := range []string{"friends-page", "friends-site-info"} {
		if len(friendsInfoTestNodes(render("/unrelated"), class)) != 0 {
			t.Errorf("友链页内容泄漏到普通页面: %s", class)
		}
	}

	updated := site
	updated.Title, updated.Description = "更新后的博客", "更新后的简介。"
	updated.URL, updated.Avatar = "https://new.example/", "images/avatar.png"
	ws.site.Store(&updated)
	info = friendsInfoTestOneClass(t, render("/friends"), "friends-site-info")
	data = friendsInfoTestYAML(t, info)
	for key, want := range map[string]string{
		"title": updated.Title, "description": updated.Description, "url": settings.NormalizeSiteURL(updated.URL),
		"avatar": "https://new.example/images/avatar.png", "feed": "https://new.example/atom.xml",
	} {
		if data[key] != want {
			t.Errorf("本站设置更新后 %s = %q，期望 %q", key, data[key], want)
		}
	}
	after, err := st.GetPageBySlug(FriendsSlug)
	if err != nil {
		t.Fatal(err)
	}
	if after.ContentMD != before.ContentMD || after.ContentHTML != before.ContentHTML ||
		after.StellarConfig != before.StellarConfig || after.BlogExtensions != before.BlogExtensions ||
		after.Title != before.Title || after.Status != before.Status {
		t.Fatal("只读展示改写了页面正文、派生缓存或设置")
	}

	after.ContentMD = "## 自定义友链\n\n没有交换说明的自定义内容。"
	if err := st.UpdatePage(FriendsSlug, after); err != nil {
		t.Fatal(err)
	}
	if len(friendsInfoTestNodes(render("/friends"), "friends-site-info")) != 0 {
		t.Fatal("没有交换标题的自定义页面被追加了本站资料")
	}
}

func TestStellarFriendsSiteInfoMatchesDocumentPreviews(t *testing.T) {
	ws, _, _, _ := stellarRouteFixture(t)
	site := *ws.site.Load()
	site.Title, site.URL = "公开博客", "https://public.example/blog/"
	site.Avatar, site.Description = "images/avatar.png", "公开站点简介。"
	ws.site.Store(&site)
	engine, err := stellarweb.NewEngineFromEmbed(ws.icons)
	if err != nil {
		t.Fatal(err)
	}
	ws.engine = engine
	for _, source := range []string{"markdown", "html"} {
		t.Run(source, func(t *testing.T) {
			input := content.StellarPreviewInput{
				StellarVersion: 2, StellarConfig: `{}`, Layout: "page", Path: "/friends", Title: "友链",
				RequestURL: "https://admin.example/api/v1/preview", PreviewToken: "friends-info-test",
				HTML: `<h2>如何交换友链？</h2><p>交换说明正文。</p>`,
			}
			if source == "markdown" {
				markdown := friendsInfoTestMarkdown
				input.Markdown = &markdown
			}
			rendered, err := ws.renderContentPreview(input)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(rendered, "/assets/css/friends-page.css?v=") {
				t.Fatal("友链文档预览缺少专用标题样式")
			}
			info := friendsInfoTestOneClass(t, rendered, "friends-site-info")
			data := friendsInfoTestYAML(t, info)
			for key, want := range map[string]string{
				"title": site.Title, "description": site.Description, "url": settings.NormalizeSiteURL(site.URL),
				"avatar": "https://public.example/blog/images/avatar.png", "feed": "https://public.example/blog/atom.xml",
			} {
				if data[key] != want {
					t.Errorf("预览 %s 未使用公开站点资料：得到 %q，期望 %q", key, data[key], want)
				}
			}
			if strings.Contains(friendsInfoTestText(info), "admin.example") {
				t.Fatal("预览后台地址被误作本站友链资料")
			}
		})
	}
}

func TestStellarFriendsSiteInfoEscapesTextAndRejectsUnsafeAvatar(t *testing.T) {
	ws, st, _, router := stellarRouteFixture(t)
	site := *ws.site.Load()
	site.Title = `<img src=x onerror="alert(1)">`
	site.Description = `<script>alert("description")</script>`
	site.Avatar = `javascript:alert("avatar")`
	ws.site.Store(&site)
	engine, err := stellarweb.NewEngineFromEmbed(ws.icons)
	if err != nil {
		t.Fatal(err)
	}
	ws.engine = engine
	if _, err := st.CreatePage(&store.Page{Slug: FriendsSlug, Title: "友链", ContentMD: friendsInfoTestMarkdown,
		Status: store.StatusPublished, StellarVersion: 2, StellarConfig: `{}`}); err != nil {
		t.Fatal(err)
	}
	response := stellarRequest(router, "/friends")
	if response.Code != http.StatusOK {
		t.Fatal(response.Code, response.Body.String())
	}
	info := friendsInfoTestOneClass(t, response.Body.String(), "friends-site-info")
	data := friendsInfoTestYAML(t, info)
	if data["title"] != site.Title || data["description"] != site.Description || data["avatar"] != "" {
		t.Fatalf("站名与简介应完整保留为 YAML 字符串，无效头像应为空：%#v", data)
	}
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode {
			if node.Data == "script" || node.Data == "img" {
				t.Errorf("本站资料中的作者文字或无效头像被当作元素: %s", node.Data)
			}
			for _, attr := range node.Attr {
				if strings.HasPrefix(attr.Key, "on") || strings.HasPrefix(strings.ToLower(attr.Val), "javascript:") {
					t.Errorf("本站资料含可执行属性: %s=%s", attr.Key, attr.Val)
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(info)
}

func friendsInfoTestYAML(t *testing.T, info *html.Node) map[string]string {
	t.Helper()
	if info.Data != "section" {
		t.Fatal("本站资料应作为独立 section 展示")
	}
	var heading, explanation *html.Node
	for node := info.FirstChild; node != nil; node = node.NextSibling {
		if node.Type != html.ElementNode {
			continue
		}
		if node.Data == "h2" {
			for _, attr := range node.Attr {
				if attr.Key == "id" && attr.Val == "friends-site-info-title" {
					heading = node
				}
			}
		}
		if node.Data == "p" && strings.TrimSpace(friendsInfoTestText(node)) != "" {
			explanation = node
		}
	}
	if heading == nil || friendsInfoTestText(heading) != "本站信息" || explanation == nil {
		t.Fatal("本站资料缺少指定的二级标题或说明文字")
	}
	highlight := friendsInfoTestOneClass(t, friendsInfoTestHTML(t, info), "highlight")
	yamlClass := false
	for _, attr := range highlight.Attr {
		if attr.Key == "class" && hasTag(strings.Fields(attr.Val), "yaml") {
			yamlClass = true
		}
	}
	if highlight.Data != "div" || !yamlClass {
		t.Fatal("本站资料应复用 YAML 高亮代码块")
	}
	codeContainer := friendsInfoTestOneClass(t, friendsInfoTestHTML(t, highlight), "code")
	directHighlight := false
	if parent := codeContainer.Parent; parent != nil && parent.Data == "div" {
		for _, attr := range parent.Attr {
			if attr.Key == "class" && hasTag(strings.Fields(attr.Val), "highlight") && hasTag(strings.Fields(attr.Val), "yaml") {
				directHighlight = true
			}
		}
	}
	if codeContainer.Data != "div" || !directHighlight {
		t.Fatal("YAML 代码块缺少 div.highlight.yaml > div.code")
	}
	firstElement := func(node *html.Node) *html.Node {
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			if child.Type == html.ElementNode {
				return child
			}
		}
		return nil
	}
	pre := firstElement(codeContainer)
	if pre == nil || pre.Data != "pre" {
		t.Fatal("YAML 代码块缺少 div.code > pre")
	}
	code := firstElement(pre)
	if code == nil || code.Data != "code" {
		t.Fatal("YAML 代码块缺少 div.code > pre > code")
	}
	var data map[string]string
	if err := yaml.Unmarshal([]byte(friendsInfoTestText(code)), &data); err != nil {
		t.Fatalf("可复制的本站资料不是有效 YAML：%v", err)
	}
	if len(data) != 5 {
		t.Fatalf("本站资料应恰有 title/url/avatar/description/feed 五项：%#v", data)
	}
	for _, key := range []string{"title", "url", "avatar", "description", "feed"} {
		if _, exists := data[key]; !exists {
			t.Errorf("本站资料 YAML 缺少 %s", key)
		}
	}
	return data
}

func friendsInfoTestOneClass(t *testing.T, body, class string) *html.Node {
	t.Helper()
	nodes := friendsInfoTestNodes(body, class)
	if len(nodes) != 1 {
		t.Fatalf("期望 1 个 .%s，实际 %d", class, len(nodes))
	}
	return nodes[0]
}

func friendsInfoTestNodes(body, class string) []*html.Node {
	root, err := html.Parse(strings.NewReader(body))
	if err != nil {
		return nil
	}
	var nodes []*html.Node
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		for _, attr := range node.Attr {
			if attr.Key == "class" && hasTag(strings.Fields(attr.Val), class) {
				nodes = append(nodes, node)
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	return nodes
}

func friendsInfoTestText(node *html.Node) string {
	if node.Type == html.TextNode {
		return node.Data
	}
	var text strings.Builder
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		text.WriteString(friendsInfoTestText(child))
	}
	return text.String()
}

func friendsInfoTestHTML(t *testing.T, node *html.Node) string {
	t.Helper()
	var output bytes.Buffer
	if err := html.Render(&output, node); err != nil {
		t.Fatal(err)
	}
	return output.String()
}
