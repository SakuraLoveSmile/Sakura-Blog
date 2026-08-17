package post

import (
	"regexp"
	"strings"
)

// --- 安全审计 2026-08-16：shortcode 输出统一转义/白名单辅助 ---
// shortcode 在 goldmark 渲染前展开、输出直入最终 HTML（WithUnsafe），参数全部来自
// 文章/瞬间作者的 markdown 源码，故所有插值必须先经 html.EscapeString 或以下白名单校验。

// 安全审计 2026-08-16：颜色参数白名单——仅放行 #hex、rgb()/rgba() 数字形式、纯字母-连字符色名
// （Stellar colorful 体系的 red/blue/light-blue 等均在此形态内），杜绝分号/引号/括号注入 style。
var (
	reSafeColorHex  = regexp.MustCompile(`^#[0-9a-fA-F]{3,8}$`)
	reSafeColorRGB  = regexp.MustCompile(`^rgba?\(\s*[0-9]{1,3}\s*,\s*[0-9]{1,3}\s*,\s*[0-9]{1,3}\s*(,\s*(0(\.[0-9]+)?|1(\.0+)?|[0-9]{1,3})\s*)?\)$`)
	reSafeColorName = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9-]{0,19}$`)
)

// safeColor 校验用户可控的颜色参数，非法值返回空串（调用方丢弃该参数或回退默认色）。
func safeColor(v string) string {
	v = strings.TrimSpace(v)
	if reSafeColorHex.MatchString(v) || reSafeColorRGB.MatchString(v) || reSafeColorName.MatchString(v) {
		return v
	}
	return ""
}

// 安全审计 2026-08-16：CSS 长度参数白名单（与 reEmojiHeight 同一口径）。
var reSafeCSSNum = regexp.MustCompile(`^[0-9]*\.?[0-9]+(px|em|rem|%|vh|vw)?$`)

// safeCSSNum 校验 CSS 长度参数（image/grid 的 width/height/padding/gap/br 等），非法值返回空串。
func safeCSSNum(v string) string {
	v = strings.TrimSpace(v)
	if reSafeCSSNum.MatchString(v) {
		return v
	}
	return ""
}

// 安全审计 2026-08-16：宽高比参数白名单——放行 16/9、1.5、0.618 等数字比例形式。
var reSafeCSSRatio = regexp.MustCompile(`^[0-9]+(\.[0-9]+)?(/[0-9]+(\.[0-9]+)?)?$`)

// safeCSSRatio 校验 aspect-ratio 类参数，非法值返回空串。
func safeCSSRatio(v string) string {
	v = strings.TrimSpace(v)
	if reSafeCSSRatio.MatchString(v) {
		return v
	}
	return ""
}

// 安全审计 2026-08-16：URL scheme 白名单——仅放行 http/https/mailto 与相对路径/锚点，
// 拒绝 javascript:、data:、vbscript: 等危险 scheme。
var reSafeURLScheme = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.-]*:`)

// safeURL 校验用户可控的链接参数，非法 scheme 返回空串（调用方丢弃 href 或回退默认）。
func safeURL(u string) string {
	u = strings.TrimSpace(u)
	if u == "" {
		return ""
	}
	if m := reSafeURLScheme.FindString(u); m != "" {
		switch strings.ToLower(m) {
		case "http:", "https:", "mailto:":
			return u
		}
		return ""
	}
	return u // 无 scheme：相对路径 / 锚点 / 协议相对 //host
}

// 安全审计 2026-08-16：class 词白名单——字母开头，限字母数字-_，供拼入 class 的参数（chat style/scene 等）。
var reSafeClassWord = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]{0,29}$`)

// safeClassWord 校验拼入 class 属性的单词参数，非法值返回空串。
func safeClassWord(v string) string {
	v = strings.TrimSpace(v)
	if reSafeClassWord.MatchString(v) {
		return v
	}
	return ""
}

// 安全审计 2026-08-16：icon key 白名单——iconify 形如 solar:hashtag-square-bold，
// 拒绝携带 HTML 元字符的 key 进入 iconFunc（即使其内部未转义也无法注入）。
var reSafeIconKey = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9:_.-]{0,63}$`)

// safeIconKey 校验图标 key 参数，非法值返回空串（调用方降级为空占位）。
func safeIconKey(v string) string {
	v = strings.TrimSpace(v)
	if reSafeIconKey.MatchString(v) {
		return v
	}
	return ""
}

// 安全审计 2026-08-16：quot el 标签白名单——禁止任意标签名注入，非法回退 p。
var quotElSet = map[string]bool{"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true, "p": true, "div": true, "blockquote": true}

// safeQuotEl 校验 quot 的 el 参数，仅放行白名单标签名，非法回退 p。
func safeQuotEl(el string) string {
	if quotElSet[el] {
		return el
	}
	return "p"
}
