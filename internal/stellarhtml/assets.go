package stellarhtml

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

// Asset.Path 为不含首个斜杠的上游路径，发布和部署前缀由调用者决定。
type Asset struct {
	Path        string
	ContentType string
	Body        []byte
}

var svgComment = regexp.MustCompile(`(?s)<!--[\s\S]*?-->`)
var svgData = regexp.MustCompile(`(?is)^data:image/svg\+xml((?:;[^,]*)?),(.*)$`)
var base64Marker = regexp.MustCompile(`(?i);base64(?:;|$)`)

// IconAssets 将可信图标数据分为 deferred-icons 使用的命名空间 JSON。
// 自定义图标来源的校验属于调用者职责。
func IconAssets(icons map[string]string) ([]Asset, error) {
	sets := map[string]map[string]string{}
	for key, value := range icons {
		if !strings.HasPrefix(strings.TrimSpace(value), "<svg") {
			continue
		}
		ns := strings.SplitN(key, ":", 2)[0]
		if ns == "" {
			continue
		}
		// 官方运行时直接将命名空间作为 URL 路径段。
		if ns == "." || ns == ".." || strings.ContainsAny(ns, "/\\?#%\x00") {
			return nil, fmt.Errorf("stellar icon namespace %q is not a safe asset path", ns)
		}
		if sets[ns] == nil {
			sets[ns] = map[string]string{}
		}
		sets[ns][key] = svgComment.ReplaceAllString(value, "")
	}
	names := make([]string, 0, len(sets))
	for ns := range sets {
		names = append(names, ns)
	}
	sort.Strings(names)
	assets := make([]Asset, 0, len(names))
	for _, ns := range names {
		data, err := json.Marshal(map[string]any{ns: sets[ns]})
		if err != nil {
			return nil, err
		}
		assets = append(assets, Asset{Path: "js/icons/" + ns + ".json", ContentType: "application/json; charset=utf-8", Body: data})
	}
	return assets, nil
}

// ImageFallbackAsset 保留 SVG 署名，以解码后的原始字节生成内容哈希。
// 与上游一致，非 SVG 或错误 data URL 返回 nil，保留原始 URL 语义。
// 本函数不访问网络或文件系统。
func ImageFallbackAsset(value string) (*Asset, error) {
	m := svgData.FindStringSubmatch(value)
	if m == nil {
		return nil, nil
	}
	decoded, err := url.PathUnescape(m[2])
	if err != nil || !utf8.ValidString(decoded) {
		return nil, nil
	}
	body := []byte(decoded)
	if base64Marker.MatchString(m[1]) {
		// Node Buffer.from 接受 URL-safe 字符、空白和非严格填充。
		if padding := strings.IndexByte(decoded, '='); padding >= 0 {
			decoded = decoded[:padding]
		}
		clean := strings.Map(func(r rune) rune {
			switch {
			case r == '-':
				return '+'
			case r == '_':
				return '/'
			case r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '+' || r == '/':
				return r
			default:
				return -1
			}
		}, decoded)
		if len(clean)%4 == 1 {
			clean = clean[:len(clean)-1]
		}
		body, err = base64.RawStdEncoding.DecodeString(clean)
		if err != nil {
			return nil, nil
		}
	}
	hash := sha256.Sum256(body)
	return &Asset{Path: "images/stellar-error." + hex.EncodeToString(hash[:])[:16] + ".svg", ContentType: "image/svg+xml", Body: body}, nil
}

// RenderIcon 对应上游 utils.icon。args 必须是模板产生的可信属性。
// 未知键显示为文本；非内联 SVG 输出延迟图标占位符。
func RenderIcon(key, args string, inline bool, icons map[string]string) string {
	value := icons[key]
	known := value != ""
	if value == "" {
		value = key
	}
	if strings.HasPrefix(value, "/") || strings.HasPrefix(value, "https://") || strings.HasPrefix(value, "http://") {
		return `<img ` + args + ` src="` + escapeAttribute(value) + `" />`
	}
	if !known {
		return escapeAttribute(value)
	}
	if inline {
		return value
	}
	if strings.HasPrefix(value, "<svg") {
		return `<svg class="icon" viewBox="0 0 1 1" data-icon="` + escapeAttribute(key) + `" aria-hidden="true"></svg>`
	}
	return value
}
