package web

import (
	"crypto/md5"
	"fmt"
	"html/template"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf16"
)

// stellarOpenGraph 对齐固定 head.ejs 与 Hexo 8 open_graph helper。
// canonical link 和 JSON-LD 继续使用宿主规范，显式 OG URL 只影响社交元信息。
func stellarOpenGraph(d *StellarRenderData, model map[string]any) (template.HTML, error) {
	page := prepCloneMap(stellarMap(d.Stellar.Data["hostPage"]))
	for key, value := range stellarMap(d.Stellar.Data["page"]) {
		page[key] = value
	}
	site := map[string]any{"title": d.Site.Title, "description": d.Site.Description, "url": d.Site.URL, "language": d.Stellar.Language, "avatar": d.Site.Avatar}
	if d.ViewData.Stellar != nil {
		for key, value := range d.ViewData.Stellar.Input.SiteConfig {
			site[key] = value
		}
	}
	page["title"] = stellarFirst(page["title"], d.Page.Title)
	page["content"] = stellarFirst(string(d.Content), page["content"])
	content := stellarText(page["content"])
	page["excerpt"] = stellarFirst(page["excerpt"], d.Page.Excerpt)
	page["lang"] = stellarFirst(page["lang"], d.Stellar.Language)
	args := prepCloneMap(stellarMap(model["args"]))
	if len(model) == 0 {
		if id := stellarGet(d.Stellar.Theme, "openGraph.twitterId"); stellarTruth(id) {
			args["twitter_id"] = id
		}
		if d.Stellar.Layout == "post" && stellarTruth(page["cover"]) {
			args["twitter_card"] = "summary_large_image"
		}
		image := stellarFirst(page["cover"], stellarOGFirstImage(content), site["avatar"])
		if !stellarTruth(image) && stellarTruth(site["email"]) {
			image = fmt.Sprintf("https://www.gravatar.com/avatar/%x", md5.Sum([]byte(strings.ToLower(stellarText(site["email"])))))
		}
		args["image"] = image
		for key, value := range stellarMap(stellarGet(d.Stellar.Data, "page.seo.openGraph")) {
			args[key] = prepClone(value)
		}
	}
	var b strings.Builder
	emit := func(kind, name string, value any) {
		fmt.Fprintf(&b, `<meta data-stellar-page-meta %s="%s"`, kind, stellarEscape(name))
		if name == "og:title" || name == "og:site_name" || name == "twitter:title" {
			fmt.Fprintf(&b, ` content="%s"`, stellarEscape(stellarOGText(value)))
		} else if stellarTruth(value) {
			fmt.Fprintf(&b, ` content="%s"`, stellarOGEscape(stellarOGText(value)))
		}
		b.WriteString(">")
	}
	desc := stellarOGDescription(stellarFirst(args["description"], page["description"], page["excerpt"], page["content"], site["description"]))
	if desc != "" {
		emit("name", "description", desc)
	}
	kind := stellarFirst(args["type"], map[bool]string{true: "article", false: "website"}[d.Stellar.Layout == "post"])
	title := stellarFirst(page["title"], site["title"])
	if len(model) > 0 {
		title = model["title"]
	}
	siteName := site["title"]
	if len(model) > 0 {
		siteName = model["siteName"]
	}
	ogURL := stellarOGPrettyURL(stellarText(stellarFirst(args["url"], page["permalink"], stellarPublicPageURL(d))), stellarMap(site["pretty_urls"]))
	emit("property", "og:type", kind)
	emit("property", "og:title", stellarOGStripHTML(stellarOGText(title)))
	encodedURL, err := stellarOGEncodeURL(ogURL)
	if err != nil {
		return "", fmt.Errorf("stellar open_graph.url: %w", err)
	}
	emit("property", "og:url", encodedURL)
	emit("property", "og:site_name", stellarOGStripHTML(stellarOGText(siteName)))
	if desc != "" {
		emit("property", "og:description", desc)
	}
	if language := stellarFirst(args["language"], page["lang"], page["language"], site["language"]); stellarTruth(language) {
		emit("property", "og:locale", stellarOGLocale(stellarOGText(language)))
	}
	imageInput := stellarFirst(args["image"], args["images"], page["photos"], []any{})
	images := stellarList(imageInput)
	if images == nil {
		images = []any{imageInput}
	}
	if len(images) == 0 {
		for _, match := range stellarOGImages.FindAllStringSubmatch(content, -1) {
			images = append(images, match[1])
		}
	}
	base := stellarText(stellarFirst(ogURL, site["url"]))
	resolved := []string{}
	for _, image := range images {
		imageText := stellarOGText(image)
		if image == nil {
			imageText = "null"
		}
		u, err := stellarOGResolve(imageText, base)
		if err != nil {
			return "", fmt.Errorf("stellar open_graph.image: %w", err)
		}
		if !strings.HasPrefix(u, "data:") {
			resolved = append(resolved, u)
			emit("property", "og:image", u)
		}
	}
	// helper 先处理日期；head.ejs 再追加模型日期，保留显式 null 的真实回退行为。
	for _, pair := range [][2]string{{"date", "article:published_time"}, {"updated", "article:modified_time"}} {
		if args[pair[0]] == false {
			continue
		}
		value := args[pair[0]]
		if !stellarTruth(value) {
			value = stellarOGDate(stellarFirst(page[pair[0]], map[string]string{"date": d.Page.Date, "updated": d.Page.Updated}[pair[0]]))
		}
		if date, ok := value.(time.Time); ok && !date.IsZero() {
			emit("property", pair[1], date.UTC().Format("2006-01-02T15:04:05.000Z"))
		}
	}
	if len(model) > 0 {
		if stellarTruth(model["publishedTime"]) {
			emit("property", "article:published_time", model["publishedTime"])
		}
		if stellarTruth(model["modifiedTime"]) {
			emit("property", "article:modified_time", model["modifiedTime"])
		}
	}
	if author := stellarFirst(args["author"], site["author"]); stellarTruth(author) {
		emit("property", "article:author", author)
	}
	tags := stellarList(model["tags"])
	if len(model) == 0 {
		tagInput := stellarFirst(page["tags"], site["keywords"])
		tags = stellarList(tagInput)
		if text, ok := tagInput.(string); ok {
			tags = []any{text}
		}
		sort.SliceStable(tags, func(i, j int) bool { return stellarOGText(tags[i]) < stellarOGText(tags[j]) })
	}
	for _, tag := range tags {
		if name := stellarFirst(stellarGet(tag, "name"), tag); stellarTruth(name) {
			emit("property", "article:tag", name)
		}
	}
	emit("name", "twitter:card", stellarFirst(args["twitter_card"], "summary"))
	if image := args["twitter_image"]; stellarTruth(image) {
		u, err := stellarOGResolve(stellarOGText(image), base)
		if err != nil {
			return "", fmt.Errorf("stellar open_graph.twitter_image: %w", err)
		}
		emit("name", "twitter:image", u)
	} else if len(resolved) > 0 {
		emit("name", "twitter:image", resolved[0])
	}
	if id := args["twitter_id"]; stellarTruth(id) {
		text, ok := id.(string)
		if !ok {
			return "", fmt.Errorf("stellar open_graph.twitter_id: expected string")
		}
		if !strings.HasPrefix(text, "@") {
			text = "@" + text
		}
		emit("name", "twitter:creator", text)
	}
	for _, pair := range [][3]string{{"twitter_site", "name", "twitter:site"}, {"fb_admins", "property", "fb:admins"}, {"fb_app_id", "property", "fb:app_id"}} {
		if value := args[pair[0]]; stellarTruth(value) {
			emit(pair[1], pair[2], value)
		}
	}
	twitterTitle := stellarFirst(page["title"], site["title"])
	if len(model) > 0 {
		twitterTitle = model["twitterTitle"]
	}
	// 模型及有标题的独立页面由 head.ejs 强制重写 twitter:title。
	if len(model) > 0 || stellarTruth(page["title"]) {
		emit("name", "twitter:title", stellarOGStripHTML(stellarOGText(twitterTitle)))
	}
	return template.HTML(b.String()), nil
}

var stellarOGImages = regexp.MustCompile(`(?i)<img [^>]*src=['"]([^'"]+)([^>]*>)`)

func stellarOGFirstImage(content string) any {
	for _, pattern := range stellarContentImagePatterns {
		if match := pattern.FindStringSubmatch(content); len(match) > 1 {
			return match[1]
		}
	}
	return nil
}
func stellarOGText(value any) string {
	if list := stellarList(value); list != nil {
		parts := []string{}
		for _, v := range list {
			parts = append(parts, stellarOGText(v))
		}
		return strings.Join(parts, ",")
	}
	if _, ok := value.(map[string]any); ok {
		return "[object Object]"
	}
	return stellarText(value)
}
func stellarOGLocale(language string) string {
	locales := map[string]string{"en": "en_US", "de": "de_DE", "es": "es_ES", "fr": "fr_FR", "hu": "hu_HU", "id": "id_ID", "it": "it_IT", "ja": "ja_JP", "ko": "ko_KR", "nl": "nl_NL", "ru": "ru_RU", "th": "th_TH", "tr": "tr_TR", "vi": "vi_VN"}
	if value := locales[language]; value != "" {
		return value
	}
	if len(language) == 5 {
		separator := "_"
		if strings.Contains(language, "-") {
			separator = "-"
		}
		parts := strings.Split(language, separator)
		if len(parts) == 2 {
			return strings.ToLower(parts[0]) + "_" + strings.ToUpper(parts[1])
		}
	}
	return ""
}
func stellarOGDate(value any) any {
	if _, ok := value.(time.Time); ok {
		return value
	}
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, stellarText(value)); err == nil {
			return t
		}
	}
	return nil
}
func stellarOGResolve(image, base string) (string, error) {
	b, err := url.Parse(base)
	if err != nil || !b.IsAbs() {
		return "", fmt.Errorf("invalid base URL")
	}
	if (b.Scheme == "http" || b.Scheme == "https" || b.Scheme == "ftp") && b.Host == "" {
		return "", fmt.Errorf("invalid base URL")
	}
	u, err := url.Parse(image)
	if err != nil {
		return "", fmt.Errorf("invalid image URL: %w", err)
	}
	u = b.ResolveReference(u)
	if (u.Scheme == "http" || u.Scheme == "https" || u.Scheme == "ftp") && u.Host == "" {
		return "", fmt.Errorf("invalid image URL")
	}
	return u.String(), nil
}
func stellarOGPrettyURL(value string, options map[string]any) string {
	if options["trailing_index"] == false {
		value = strings.TrimSuffix(value, "index.html")
	}
	if options["trailing_html"] == false && !strings.HasSuffix(value, "index.html") {
		value = strings.TrimSuffix(value, ".html")
	}
	return value
}
func stellarOGEncodeURL(value string) (string, error) {
	u, err := url.Parse(value)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}
	if u.IsAbs() {
		if (u.Scheme == "http" || u.Scheme == "https" || u.Scheme == "ftp") && u.Host == "" {
			return "", fmt.Errorf("invalid URL")
		}
		if u.Opaque != "" || u.Scheme == "file" {
			return value, nil
		}
		query, err := url.PathUnescape(u.RawQuery)
		if err == nil {
			u.RawQuery = stellarOGEncodeURI(query)
		}
		return u.String(), nil
	}
	decoded, err := url.PathUnescape(value)
	if err == nil {
		value = decoded
	}
	return stellarOGEncodeURI(value), nil
}

func stellarOGEncodeURI(value string) string {
	var b strings.Builder
	for i := 0; i < len(value); i++ {
		c := value[i]
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune(";,/?:@&=+$#-_.!~*'()", rune(c)) {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}
func stellarOGDescription(value any) string {
	text, ok := value.(string)
	if !ok {
		return ""
	}
	units := utf16.Encode([]rune(stellarOGStripHTML(text)))
	if len(units) > 200 {
		units = units[:200]
	}
	return strings.ReplaceAll(strings.TrimSpace(string(utf16.Decode(units))), "\n", " ")
}

func stellarOGStripHTML(s string) string {
	state, depth := 0, 0
	quote := byte(0)
	start, plain := -1, 0
	var out strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch state {
		case 0:
			if c == '<' {
				out.WriteString(s[plain:i])
				state = 1
				start = i
			}
		case 1:
			switch c {
			case '<':
				if quote == 0 {
					depth++
				}
			case '>':
				if quote == 0 {
					if depth > 0 {
						depth--
					} else {
						quote = 0
						state = 0
						start = -1
						plain = i + 1
					}
				}
			case '\'', '"':
				if c == quote {
					quote = 0
				} else if quote == 0 {
					quote = c
				}
			case '-':
				if i-start == 3 && s[start+1] == '!' && s[start+2] == '-' {
					state = 2
				}
			case ' ', '\n':
				if i-start == 1 {
					state = 0
					out.WriteString("< ")
					start = -1
					plain = i + 1
				}
			}
		case 2:
			if c == '>' {
				if i >= 2 && s[i-1] == '-' && s[i-2] == '-' {
					state = 0
					plain = i + 1
				}
				start = -1
			}
		}
	}
	if state == 0 && plain < len(s) {
		out.WriteString(s[plain:])
	}
	return out.String()
}

var stellarOGEntity = regexp.MustCompile(`^(?:#[0-9]{1,7}|#[xX][a-fA-F0-9]{1,6}|[A-Za-z0-9_]+);`)

func stellarOGEscape(text string) string {
	var b strings.Builder
	replacements := map[byte]string{'&': "&amp;", '<': "&lt;", '>': "&gt;", '"': "&quot;", '\'': "&#39;", '`': "&#96;", '/': "&#x2F;", '=': "&#x3D;"}
	for i := 0; i < len(text); i++ {
		if text[i] == '&' && stellarOGEntity.MatchString(text[i+1:]) {
			b.WriteByte(text[i])
			continue
		}
		if escaped, ok := replacements[text[i]]; ok {
			b.WriteString(escaped)
		} else {
			b.WriteByte(text[i])
		}
	}
	return b.String()
}
