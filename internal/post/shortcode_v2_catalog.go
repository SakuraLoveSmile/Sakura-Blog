package post

import (
	"regexp"
	"strconv"
	"strings"
)

// v2 定义附着在同一注册项；渲染、插入表单、编辑与保存共用参数来源。
func init() {
	for i := range shortcodeRegistry {
		r := &shortcodeRegistry[i]
		e := r.entry
		switch e.Name {
		case "about", "links", "moments", "ghrepo", "ghuser", "mermaid":
			continue // 独立 Blog 扩展，不污染官方标签契约。
		}
		e.Params = append([]ShortcodeParamSchema(nil), e.Params...)
		for j := range e.Params {
			e.Params[j].Default = nil
		}
		e.Edit = ShortcodeEditSchema{Enabled: true}
		// 未写出的参数无默认值；下面的默认值只用于插入新标签。
		switch e.Name {
		case "tip":
			e.Type = ShortcodeInline
			e.Params = []ShortcodeParamSchema{positional("text", "词句", nil), named("pop", "注解", nil)}
		case "note":
			e.Params = []ShortcodeParamSchema{named("color", "配色", nil), positional("title", "标题（可选）", nil), positional("content", "内容", nil)}
		case "quot":
			e.Params = []ShortcodeParamSchema{named("el", "标签", "p"), named("icon", "预设/图标", nil), named("color", "图标配色", nil), named("prefix", "前缀图标", nil), named("suffix", "后缀图标", nil), positional("text", "文本", nil)}
		case "mark":
			e.Params = append(e.Params, named("tip", "注解", nil))
		case "gallery":
			e.Params = []ShortcodeParamSchema{named("layout", "布局", "grid"), named("size", "尺寸", "mix"), named("aspect_ratio", "宽高比", "square"), content("content", "图片", nil)}
		case "frame":
			e.Type = ShortcodeInline
			e.Params = append([]ShortcodeParamSchema(nil), shortcodeFrameInlineParams...)
			e.Params[0].Default = nil
		case "navbar":
			e.Type = ShortcodeInline
			e.Params = []ShortcodeParamSchema{named("active", "高亮URL", nil), positional("links", "导航链接", nil)}
		case "banner":
			e.Params = []ShortcodeParamSchema{named("bg", "背景", nil), named("avatar", "头像", nil), named("link", "链接", nil), positional("title", "标题", nil), positional("subtitle", "副标题", nil), content("content", "导航", nil)}
		case "emoji":
			for j := range e.Params {
				if e.Params[j].Key == "name" {
					e.Params[j].NamedAlias = true
				}
			}
		case "friends":
			e.Params = []ShortcodeParamSchema{named("repo", "GitHub 仓库", nil), named("api", "数据URL", nil), named("posts", "文章列表（true/false）", nil), positional("group", "分组", nil)}
		case "sites":
			e.Params = []ShortcodeParamSchema{named("repo", "GitHub 仓库", nil), named("api", "数据URL", nil), positional("group", "分组", nil)}
		case "timeline":
			e.Params = []ShortcodeParamSchema{named("type", "数据服务类型", nil), named("api", "数据URL", nil), named("user", "用户", nil), named("limit", "数量", nil), named("hide", "隐藏字段", nil), content("content", "节点", nil)}
		case "okr":
			e.Params[0].Key = "oIndex"
		case "chat":
			for j := range e.Params {
				if e.Params[j].Key == "scene" {
					e.Params[j].Options = []string{"", "group", "person"}
					e.Params[j].Default = nil
				}
			}
		case "grid":
			for j := range e.Params {
				if e.Params[j].Key == "w" {
					e.Params[j].Default = "280px"
				}
			}
		case "dropdown":
			e.Params = append(e.Params, named("icon", "图标", nil))
		}
		// boolean 字符串需要区分省略和显式 false，不能初始化成 false 而丢失存在性。
		for j := range e.Params {
			if e.Params[j].Role == ShortcodeParamFlag {
				e.Params[j].Role = ShortcodeParamNamed
				e.Params[j].Type = ShortcodeParamSelect
				e.Params[j].Options = []string{"", "true", "false"}
				e.Params[j].Default = nil
			}
		}
		e.Signature = v2CatalogSignature(e)
		e.Example = v2CatalogExample(e)
		for _, p := range e.Params {
			e.ParamDocs = append(e.ParamDocs, ShortcodeParamDoc{Name: p.Key, Desc: p.Label})
		}
		r.stellarV2 = &e
	}
	e := ShortcodeCatalogEntry{Name: "gist", Title: "GitHub Gist", Type: ShortcodeInline, Category: "GitHub", Capability: ShortcodeExternal,
		Params: []ShortcodeParamSchema{named("file", "文件名", nil), positional("gist", "owner/id", nil)}, Edit: ShortcodeEditSchema{Enabled: true},
		Signature: "{% gist owner/id [file:name] %}", Example: "{% gist octocat/1 %}"}
	shortcodeRegistry = append(shortcodeRegistry, shortcodeRegistration{entry: e, parser: regexp.MustCompile(`{%\s*gist\s*(.*?)\s*%}`), stellarV2: &e, v2Only: true})
}

func ShortcodeCatalogStellarV2() []ShortcodeCatalogEntry {
	var entries []ShortcodeCatalogEntry
	for _, r := range shortcodeRegistry {
		if r.stellarV2 == nil {
			continue
		}
		e := *r.stellarV2
		e.Params = append([]ShortcodeParamSchema(nil), e.Params...)
		for i := range e.Params {
			e.Params[i].Options = append([]string(nil), e.Params[i].Options...)
		}
		if d, ok := shortcodeDisplayMetadata[e.Name]; ok {
			e.Title = d.Title
			e.Description = d.Description
			e.Services = append([]string(nil), d.Services...)
			e.Plugins = append([]string(nil), d.Plugins...)
		}
		entries = append(entries, e)
	}
	return entries
}

func shortcodeV2Entry(name string) (ShortcodeCatalogEntry, bool) {
	for _, r := range shortcodeRegistry {
		if r.entry.Name == name && r.stellarV2 != nil {
			return *r.stellarV2, true
		}
	}
	return ShortcodeCatalogEntry{}, false
}

func v2CatalogSignature(e ShortcodeCatalogEntry) string {
	var parts []string
	for _, p := range e.Params {
		switch p.Role {
		case ShortcodeParamNamed:
			parts = append(parts, "["+p.Key+":value]")
		case ShortcodeParamPositional:
			parts = append(parts, p.Key)
		}
	}
	signature := "{% " + e.Name
	if len(parts) > 0 {
		signature += " " + strings.Join(parts, " ")
	}
	signature += " %}"
	if e.Type == ShortcodeBlock {
		signature += "\ncontent\n{% end" + e.Name + " %}"
	}
	return signature
}
func v2CatalogExample(e ShortcodeCatalogEntry) string {
	// 使用官方语法候选，例子只是插入预览，不成为参数的隐含默认值。
	sample := map[string]string{"src": "https://example.com/image.png", "url": "https://example.com/", "href": "/tags/example/", "title": "示例", "text": "示例文本", "content": "示例正文", "repo": "xaoxuu/hexo-theme-stellar", "group": "default", "key": "default:star", "source": "blobcat", "name": "aini", "device": "iphone11", "img": "https://example.com/image.png", "gist": "octocat/1", "oIndex": "o1", "type": "INTJ-T", "scores": "93-72-78-83-51", "links": "[首页](/) [关于](/about/)"}
	values := map[string]any{}
	for _, p := range e.Params {
		if p.Role == ShortcodeParamPositional {
			values[p.Key] = sample[p.Key]
		}
		if p.Role == ShortcodeParamContent {
			values[p.Key] = "示例正文"
		}
	}
	switch e.Name {
	case "tabs":
		values["content"] = "<!-- tab 第一项 -->\n**正文**\n<!-- tab 第二项 -->\n正文"
	case "folders":
		values["content"] = "<!-- folder 第一项 -->\n正文"
	case "grid":
		values["content"] = "<!-- cell -->\n正文\n<!-- cell -->\n正文"
	case "gallery", "swiper":
		values["content"] = "![图片](https://example.com/image.png)"
	case "timeline":
		values["content"] = "<!-- node 今天 -->\n正文"
	case "dropdown":
		values["content"] = "- [首页](/)\n- [关于](/about/)"
	case "banner":
		values["content"] = "{% navbar [首页](/) [关于](/about/) %}"
	case "table":
		values["content"] = "| A | B |\n| - | - |\n| 1 | 2 |"
	case "paper":
		values["title"] = "示例"
		values["content"] = "<!-- section 第一章 -->\n正文\n<!-- paragraph -->\n正文"
	case "okr":
		values["content"] = "目标\n<!-- okr kr1 percent:50 -->\n任务一\n<!-- okr kr2 percent:100 -->\n任务二"
	case "chat":
		values["title"] = "聊天"
		values["content"] = "alice:\n  name: Alice\n  avatar: /avatar.png\n<!-- chatcell user:alice md:你好 -->"
	case "friends", "albums", "posters", "sites":
		values["repo"] = "xaoxuu/hexo-theme-stellar"
	}
	if e.Name == "frame" {
		values["img"] = sample["img"]
	}
	// init时registry尚未写回；根据字段局部生成，不调用登记查找。
	var args []string
	for _, p := range e.Params {
		if p.Role != ShortcodeParamContent {
			if v := v2String(values[p.Key]); v != "" {
				if p.Role == ShortcodeParamNamed {
					args = append(args, p.Key+":"+strconv.Quote(v))
				} else {
					args = append(args, strconv.Quote(v))
				}
			}
		}
	}
	raw := "{% " + e.Name
	if len(args) > 0 {
		raw += " " + strings.Join(args, " ")
	}
	raw += " %}"
	if e.Type == ShortcodeBlock {
		raw += "\n" + v2String(values["content"]) + "\n{% end" + e.Name + " %}"
	}
	return raw
}
