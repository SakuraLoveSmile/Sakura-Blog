package web

import (
	"html/template"
	"strings"
	"testing"

	"blog/internal/settings"
	"blog/internal/stellarmodels"
	"golang.org/x/net/html"
)

func TestEngineSettingsAvatarSkipsLazyImageProcessing(t *testing.T) {
	icons, err := settings.LoadIconsEmbed()
	if err != nil {
		t.Fatal(err)
	}
	engine, err := NewEngineFromEmbed(icons)
	if err != nil {
		t.Fatal(err)
	}
	data := prepareFixture(t, "settings")
	data.Site.StellarV2.Theme["comments"] = map[string]any{"provider": "artalk"}
	data.Site.StellarV2.Theme["fallbacks"] = map[string]any{"avatar": "/avatar-placeholder.png"}
	data.Content = template.HTML(`<p><img src="/content-image.png" alt="content"></p>`)
	data.Stellar = &settings.StellarRenderContext{
		Version: 2,
		Profile: "settings",
		Input: stellarmodels.Input{
			Page:        map[string]any{"layout": "settings", "permalink": data.Page.CanonicalURL, "content": string(data.Content)},
			FrontMatter: map[string]any{},
		},
	}

	output, err := engine.Render("layout/base", data)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := html.Parse(strings.NewReader(output))
	if err != nil {
		t.Fatal(err)
	}
	var avatar, contentImage *html.Node
	var visit func(*html.Node)
	visit = func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == "img" {
			for _, attr := range node.Attr {
				if attr.Key == "data-profile-avatar" {
					avatar = node
				}
				if attr.Key == "alt" && attr.Val == "content" {
					contentImage = node
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(doc)
	if avatar == nil {
		t.Fatal("settings profile avatar was not rendered")
	}
	avatarAttrs := map[string]string{}
	for _, attr := range avatar.Attr {
		avatarAttrs[attr.Key] = attr.Val
	}
	if !strings.Contains(" "+avatarAttrs["class"]+" ", " no-lazy ") {
		t.Fatalf("profile avatar is missing no-lazy: %v", avatar.Attr)
	}
	if avatarAttrs["src"] != "/blog/avatar-placeholder.png" || avatarAttrs["data-src"] != "" {
		t.Fatalf("profile avatar was rewritten by image processing: %v", avatar.Attr)
	}
	postData := prepareFixture(t, "post")
	postData.Content = template.HTML(`<p><img src="/content-image.png" alt="content"></p>`)
	postData.Stellar = &settings.StellarRenderContext{
		Version: 2,
		Profile: "post",
		Input: stellarmodels.Input{
			Page:        map[string]any{"layout": "post", "permalink": postData.Page.CanonicalURL, "content": string(postData.Content)},
			FrontMatter: map[string]any{},
		},
	}
	postOutput, err := engine.Render("layout/base", postData)
	if err != nil {
		t.Fatal(err)
	}
	postDoc, err := html.Parse(strings.NewReader(postOutput))
	if err != nil {
		t.Fatal(err)
	}
	visit(postDoc)
	if contentImage == nil {
		t.Fatal("ordinary content image was not rendered")
	}
	contentAttrs := map[string]string{}
	for _, attr := range contentImage.Attr {
		contentAttrs[attr.Key] = attr.Val
	}
	if contentAttrs["data-src"] != "/content-image.png" || contentAttrs["src"] == "/content-image.png" {
		t.Fatalf("ordinary content image did not retain lazy processing: %v", contentImage.Attr)
	}
}
