package stellarmodels

import (
	"net/url"
	"regexp"
	"strings"
)

// HeroInput 保留集合与页面路径，确保仅首页启用 Hero。模板优先展示视频，
// 模型仍保留完整 effect 配置，供官方客户端使用。
type HeroInput struct{ Theme, Collection, Item, SiteConfig, FrontMatter Object }

func BuildHero(in HeroInput) Object {
	c, item := in.Collection, in.Item
	hero := obj(get(c, "presentation.hero"))
	repo := str(get(c, "source.repository"))
	source, release := "", ""
	if repo != "" {
		source = "https://github.com/" + repo
		release = str(get(in.Theme, "services.github.apiUrl")) + "/repos/" + repo + "/tags"
	}
	return Object{"enabled": get(c, "route.homepage") == get(item, "route.path") && hero["enabled"] == true, "background": HeroBackground(hero["background"]), "preview": clone(coalesce(hero["preview"], Object{})), "actions": clone(array(hero["actions"])), "title": first(get(c, "identity.headline"), get(c, "identity.name")), "description": first(get(c, "identity.description"), str(in.FrontMatter["description"])), "repository": repo, "sourceUrl": source, "releaseApi": release, "projectName": first(get(c, "identity.name"), c["id"]), "siteName": text(first(in.SiteConfig["title"]))}
}
func HeroBackground(v any) Object {
	r := cp(obj(v))
	if r["parallax"] == nil {
		r["parallax"] = 0.2
	}
	if effect := obj(r["effect"]); len(effect) > 0 {
		effect = cp(effect)
		var background string
		switch effect["type"] {
		case "strands", "galaxy":
			background = "#000000"
		case "light-rays":
			background = "#000000"
			if get(effect, "options.lightMode") == true {
				background = "#ffffff"
			}
		case "ferrofluid":
			background = str(get(effect, "options.backgroundColor"))
			if background == "" {
				background = "#03010A"
			}
			if !strings.HasPrefix(background, "#") {
				background = "#" + background
			}
		}
		if background != "" {
			effect["presentation"] = Object{"background": background}
		}
		r["effect"] = effect
	}
	return r
}
func attr(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "&", "&amp;"), "\"", "&quot;")
}

// ReadmeHTML 生成官方 data-service 的源描述，不抓取仓库或读取文件。
// 宿主负责添加 Blog 服务封装标记。
func ReadmeHTML(in Input, c, item Object) string {
	repo := str(get(c, "source.repository"))
	if repo == "" || jsTrim(str(item["content"])) != "" || get(c, "route.homepage") != get(item, "route.path") {
		return ""
	}
	raw := origin(get(in.Theme, "services.github.rawUrl"))
	branch := text(first(get(c, "source.branch"), "HEAD"))
	src := raw + "/" + repo + "/" + branch + "/README.md"
	base := ""
	if u, err := url.Parse(src); err == nil && u.Host != "" {
		segs := []string{}
		for _, s := range strings.Split(u.EscapedPath(), "/") {
			if s != "" {
				segs = append(segs, s)
			}
		}
		if len(segs) >= 3 {
			base = raw + "/" + strings.Join(segs[:3], "/") + "/"
			src = base + strings.Join(segs[3:], "/")
		}
	}
	r := "<div class=\"data-service ds-mdrender\" src=\"" + attr(src) + "\""
	if base != "" {
		r += " data-base=\"" + attr(base) + "\""
	}
	return r + " data-replace=\"true\" data-heading=\"true\"></div>"
}

var aiLabels = map[string]Object{"manual": {"color": "#03a9f4", "icon": "default:shield-user"}, "reviewed": {"color": "#4caf50", "icon": "default:shield-check"}, "polished": {"color": "#4caf50", "icon": "default:shield-up"}, "generated": {"color": "#ff9800", "icon": "default:shield-warning"}}

func ResolveAILabel(value string) Object { return cp(aiLabels[value]) }

// BuildAILabelHTML 与上游一致，要求图标 HTML 和 extraClass 来自可信模板；
// 用户可见文本输出前执行转义。
func BuildAILabelHTML(value, text, extraClass, iconHTML string, noColor bool) string {
	def := aiLabels[value]
	if def == nil || text == "" {
		return ""
	}
	class := "ai-label"
	if extraClass != "" {
		class = extraClass + " " + class
	}
	r := "<span class=\"" + class + "\""
	if !noColor {
		r += " style=\"color:" + str(def["color"]) + "\""
	}
	return r + ">" + iconHTML + escapeHTML(text) + "</span>"
}

var entity = regexp.MustCompile(`^&(?:#[0-9]{1,7}|#[Xx][a-fA-F0-9]{1,6}|\w+);`)

func escapeHTML(s string) string {
	var out strings.Builder
	replacements := map[byte]string{'&': "&amp;", '<': "&lt;", '>': "&gt;", '"': "&quot;", '\'': "&#39;", '`': "&#96;", '/': "&#x2F;", '=': "&#x3D;"}
	for i := 0; i < len(s); i++ {
		if s[i] == '&' && entity.MatchString(s[i:]) {
			out.WriteByte(s[i])
			continue
		}
		if replacement, ok := replacements[s[i]]; ok {
			out.WriteString(replacement)
		} else {
			out.WriteByte(s[i])
		}
	}
	return out.String()
}
