package routes

import (
	"blog/internal/settings"
	"blog/internal/stellarmodels"
	"bytes"
	"strconv"
	"strings"

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

// Console 传入已渲染的 HTML；用 DOM 属性接入同一 typed 预览开关。
// 正式页面由 Markdown renderer 直接生成属性，不经过此适配。
func stellarFriendsPreviewHTML(body string, show bool) (string, error) {
	root := &html.Node{Type: html.DocumentNode}
	context := &html.Node{Type: html.ElementNode, Data: "body", DataAtom: atom.Body}
	nodes, err := html.ParseFragment(strings.NewReader(body), context)
	if err != nil {
		return "", err
	}
	for _, node := range nodes {
		root.AppendChild(node)
	}
	var walk func(*html.Node)
	walk = func(node *html.Node) {
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
			node.Attr = append(attrs, html.Attribute{Key: "data-show-posts", Val: strconv.FormatBool(show)})
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	var output bytes.Buffer
	for child := root.FirstChild; child != nil; child = child.NextSibling {
		if err := html.Render(&output, child); err != nil {
			return "", err
		}
	}
	return output.String(), nil
}
