package routes

import (
	"blog/internal/settings"
	"blog/internal/stellarhtml"
	"blog/internal/stellarmodels"
	"bytes"
	"html/template"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// 友链页宿主默认值只补缺省字段，不改存储原文，也不覆盖显式 Page 配置。
// 旧 Hero 的标题、描述和背景继续使用严格 banner 与独立 banner_image DOM。
func stellarFriendsDefaults(site *settings.Site, config, blog stellarmodels.Object) (stellarmodels.Object, stellarmodels.Object) {
	config = stellarCopyObject(config)
	defaults := stellarmodels.Object{}
	friends := site.Pages.Friends
	if _, exists := config["title"]; !exists && friends.Title != "" {
		config["title"] = friends.Title
	}
	if friends.Hero == nil {
		return config, defaults
	}
	banner := stellarCopyObject(stellarObject(config["banner"]))
	for key, value := range map[string]string{"headline": friends.Hero.Title, "tagline": friends.Hero.Description} {
		if _, exists := banner[key]; !exists && value != "" {
			banner[key] = value
		}
	}
	if friends.Hero.Background != "" {
		if _, exists := stellarObject(blog["content"])["banner_image"]; !exists {
			defaults["banner_image"] = friends.Hero.Background
		}
		if _, exists := banner["background"]; !exists {
			banner["background"] = true
		}
	}
	if len(banner) > 0 {
		config["banner"] = banner
	}
	return config, defaults
}

// 正式页面与 Console 预览共用友链页外壳和实时本站资料，不改存储原文。
func stellarFriendsContentHTML(body string, site *settings.Site) (string, error) {
	root := &html.Node{Type: html.ElementNode, Data: "div", DataAtom: atom.Div,
		Attr: []html.Attribute{{Key: "class", Val: "friends-page"}}}
	context := &html.Node{Type: html.ElementNode, Data: "body", DataAtom: atom.Body}
	nodes, err := html.ParseFragment(strings.NewReader(body), context)
	if err != nil {
		return "", err
	}
	for _, node := range nodes {
		root.AppendChild(node)
	}
	var exchange *html.Node
	var headingText func(*html.Node) string
	headingText = func(node *html.Node) string {
		if node.Type == html.TextNode {
			return node.Data
		}
		var text strings.Builder
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			text.WriteString(headingText(child))
		}
		return text.String()
	}
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if exchange == nil && node.Type == html.ElementNode && node.DataAtom == atom.H2 && strings.Contains(headingText(node), "交换友链") {
			exchange = node
		}
		service := false
		for _, attr := range node.Attr {
			if attr.Key == "class" && hasTag(strings.Fields(attr.Val), "ds-friends_and_posts") {
				service = true
			}
		}
		if service {
			attrs := node.Attr[:0]
			for _, attr := range node.Attr {
				if attr.Key != "data-show-posts" {
					attrs = append(attrs, attr)
				}
			}
			node.Attr = append(attrs, html.Attribute{Key: "data-show-posts", Val: strconv.FormatBool(site.Pages.Friends.ShowPostsOn())})
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	if exchange != nil {
		// 资料属于交换分区，放在该分区正文之后、下一个同级标题之前。
		section := exchange
		for section.Parent != root {
			section = section.Parent
		}
		next := section.NextSibling
		for next != nil && next.DataAtom != atom.H1 && next.DataAtom != atom.H2 {
			next = next.NextSibling
		}
		info, err := stellarFriendsSiteInfo(site)
		if err != nil {
			return "", err
		}
		nodes, err := html.ParseFragment(strings.NewReader(info), context)
		if err != nil {
			return "", err
		}
		for _, node := range nodes {
			root.InsertBefore(node, next)
		}
	}
	var output bytes.Buffer
	if err := html.Render(&output, root); err != nil {
		return "", err
	}
	return output.String(), nil
}

var friendsSiteInfoTemplate = template.Must(template.New("friends-site-info").Parse(`<section class="friends-site-info" aria-labelledby="friends-site-info-title">
<h2 id="friends-site-info-title">本站信息</h2>
<p>当你添加本站友链时，建议复制以下信息：</p>
<div class="highlight yaml" tabindex="0" role="region" aria-label="本站友链 YAML"><div class="code"><pre><code class="language-yaml">{{.}}</code></pre></div></div>
</section>`))

func stellarFriendsSiteInfo(site *settings.Site) (string, error) {
	info := struct {
		Title       string `yaml:"title"`
		URL         string `yaml:"url"`
		Avatar      string `yaml:"avatar"`
		Description string `yaml:"description"`
		Feed        string `yaml:"feed"`
	}{
		Title: site.Title, URL: settings.NormalizeSiteURL(site.URL), Description: site.Description,
	}
	if site.Avatar != "" {
		// 站点 URL 是目录根；共用资源解析器处理子目录、中文域名和协议校验。
		info.Avatar, _ = stellarhtml.ResolveHTTPResource(site.Avatar, info.URL+"/")
	}
	info.Feed = settings.JoinSiteURL(info.URL, "/atom.xml")
	// 使用既有 YAML 编码器保留冒号、引号与换行，访客可以直接复制完整配置。
	encoded, err := yaml.Marshal(info)
	if err != nil {
		return "", err
	}
	var output bytes.Buffer
	if err := friendsSiteInfoTemplate.Execute(&output, string(encoded)); err != nil {
		return "", err
	}
	return output.String(), nil
}
