package post

import (
	"embed"
	"fmt"
	"github.com/goccy/go-yaml"
	"html"
	"math"
	"net/url"
	"strconv"
	"strings"
)

//go:embed shortcode_v2_data/*.yml
var v2LanguageFiles embed.FS
var v2TagLanguages = func() map[string]map[string]any {
	out := map[string]map[string]any{}
	for _, language := range []string{"zh-CN", "zh-TW", "en"} {
		data, err := v2LanguageFiles.ReadFile("shortcode_v2_data/" + language + ".yml")
		if err != nil {
			panic(err)
		}
		m := map[string]any{}
		if err := yaml.Unmarshal(data, &m); err != nil {
			panic(err)
		}
		out[language] = m
	}
	return out
}()

func (r *shortcodeV2Renderer) localized(key string, values ...any) string {
	language := r.language
	if language == "zh" || language == "zh-Hans" {
		language = "zh-CN"
	}
	if language == "zh-Hant" {
		language = "zh-TW"
	}
	m := v2TagLanguages[language]
	if m == nil {
		m = v2TagLanguages["en"]
	}
	var value any = m
	for _, part := range strings.Split(key, ".") {
		value = v2Object(value)[part]
	}
	text := v2String(value)
	if text == "" {
		return key
	}
	if len(values) > 0 {
		return fmt.Sprintf(text, values...)
	}
	return text
}
func (r *shortcodeV2Renderer) mbti(a map[string]string) string {
	typeCode := strings.ToUpper(strings.TrimSpace(a["type"]))
	if typeCode == "" {
		return ""
	}
	key := typeCode
	if len(key) > 4 {
		key = key[:4]
	}
	role := mbtiRoles[key]
	themeColor := mbtiRoleColors[role]
	typeName := r.localized("tag_plugins.mbti.types." + key)
	if strings.HasPrefix(typeName, "tag_plugins.") {
		typeName = typeCode
	}
	title := html.EscapeString(r.localized("tag_plugins.mbti.title", typeCode, typeName))
	profile := strings.TrimSpace(a["url"])
	if profile != "" && !strings.HasPrefix(strings.ToLower(profile), "http://") && !strings.HasPrefix(strings.ToLower(profile), "https://") {
		if v2SafeURL(profile) != "" {
			profile = "https://www.16personalities.com/ch/" + url.PathEscape("档案") + "/" + profile
		} else {
			profile = ""
		}
	}
	var b strings.Builder
	b.WriteString(`<div class="tag-plugin colorful note mbti"` + v2Attr("color", themeColor) + `>`)
	if a["image"] != "" {
		b.WriteString(`<img class="mbti-image"` + v2Attr("src", a["image"]) + ` alt="" />`)
	}
	b.WriteString(`<div class="mbti-title">`)
	if profile != "" {
		b.WriteString(`<a class="mbti-title-link"` + v2Attr("href", profile) + ` target="_blank" rel="external nofollow noopener noreferrer">` + title + `</a>`)
	} else {
		b.WriteString(`<span class="mbti-title-text">` + title + `</span>`)
	}
	b.WriteString(`</div><div class="mbti-body"><div class="mbti-traits">`)
	code := strings.Replace(typeCode, "-", "", 1)
	lefts := []byte{'E', 'N', 'T', 'J', 'A'}
	scores := strings.Split(a["scores"], "-")
	for i, t := range mbtiTraits {
		pct := 0
		if i < len(scores) {
			fmt.Sscanf(scores[i], "%d", &pct)
		}
		pct = max(0, min(100, pct))
		leftSide := i < len(code) && code[i] == lefts[i]
		if i == 4 && (len(code) <= 4 || (code[4] != 'A' && code[4] != 'T')) {
			leftSide = true
		}
		position := pct
		reversed := " reversed"
		if leftSide {
			position = 100 - pct
			reversed = ""
		}
		active := "right"
		if position <= 50 {
			active = "left"
		}
		b.WriteString(`<div class="mbti-trait` + reversed + `" style="--theme:` + t.color + `; --theme-a30:` + mbtiAlphaHex(t.color, 0.3) + `"><div class="mbti-labels">`)
		left, right := html.EscapeString(r.localized("tag_plugins.mbti.traits."+t.key+".left")), html.EscapeString(r.localized("tag_plugins.mbti.traits."+t.key+".right"))
		if active == "left" {
			fmt.Fprintf(&b, `<span class="mbti-label left active">%s <span class="pct">%d%%</span></span><span class="mbti-label right">%s</span>`, left, pct, right)
		} else {
			fmt.Fprintf(&b, `<span class="mbti-label left">%s</span><span class="mbti-label right active"><span class="pct">%d%%</span> %s</span>`, left, pct, right)
		}
		fmt.Fprintf(&b, `</div><div class="mbti-progress"><div class="fill" style="width:%d%%"></div><span class="mbti-dot" style="left:%d%%"></span></div></div>`, 100-pct, position)
	}
	b.WriteString(`</div></div></div>`)
	return b.String()
}
func v2Number(s string) float64 {
	n, e := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if e != nil || math.IsNaN(n) || math.IsInf(n, 0) {
		return 0
	}
	return n
}
func (r *shortcodeV2Renderer) okr(a map[string]string, body string) string {
	parts := r.splitCaptured(reOKRItem, body)
	if len(parts) < 3 {
		return ""
	}
	title, note := okrSplitTitleNote(parts[0])
	parts = parts[1:]
	type result struct {
		index, title, note, status string
		percent                    float64
	}
	var krs []result
	sum := 0.0
	for i := 0; i+1 < len(parts); i++ {
		if !strings.HasPrefix(parts[i], "kr") {
			continue
		}
		if strings.HasPrefix(parts[i+1], "kr") {
			continue
		}
		params := v2MapArgs(splitArgs(parts[i]), []ShortcodeParamSchema{named("percent", "", nil), named("status", "", nil), positional("krIndex", "", nil)})
		t, n := okrSplitTitleNote(parts[i+1])
		percent := v2Number(params["percent"])
		sum += percent
		krs = append(krs, result{index: params["krIndex"], title: t, note: n, status: params["status"], percent: percent})
		i++
	}
	percent := v2Number(a["percent"])
	if percent == 0 {
		if len(krs) > 0 {
			percent = math.Round((sum/float64(len(krs)))*100) / 100
		} else {
			percent = math.NaN()
		}
	}
	item := func(kind string, v result) string {
		status := v.status
		colors := map[string]string{"in_track": "blue", "at_risk": "yellow", "off_track": "orange", "finished": "green", "unfinished": "red"}
		if colors[status] == "" {
			status = "in_track"
			if v.percent >= 100 {
				status = "finished"
			}
		}
		p := strconv.FormatFloat(math.Trunc(v.percent), 'f', -1, 64)
		return `<div class="okr-item ` + kind + `"` + v2Attr("id", "okr-"+strings.ToLower(v.index)) + `><div class="okr-left"><span class="title">` + html.EscapeString(strings.ToUpper(v.index)) + `</span></div><div class="okr-center"><span class="title">` + html.EscapeString(v.title) + `</span><div class="note">` + v2Compact(r.markdown(v.note)) + `</div></div><div class="okr-right colorful"` + v2Attr("color", colors[status]) + `><div class="labels"><span class="status label">` + html.EscapeString(r.localized("tag_plugins.okr.status."+status)) + `</span><span class="status percent">` + p + `%</span></div><div class="progress"><div class="fill" style="width:` + p + `%;"></div></div></div></div>`
	}
	el := `<div class="tag-plugin colorful okr"` + v2Attrs(a, "color") + `>` + item("o", result{index: a["oIndex"], title: title, note: note, status: a["status"], percent: percent})
	for _, kr := range krs {
		el += item("kr", kr)
	}
	return el + `</div>`
}
