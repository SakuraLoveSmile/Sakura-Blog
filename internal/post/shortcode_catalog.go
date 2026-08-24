package post

import "regexp"

// ShortcodeParamType is the input kind understood by the Console SchemaForm.
type ShortcodeParamType string

const (
	ShortcodeParamText     ShortcodeParamType = "text"
	ShortcodeParamSelect   ShortcodeParamType = "select"
	ShortcodeParamNumber   ShortcodeParamType = "number"
	ShortcodeParamSwitch   ShortcodeParamType = "switch"
	ShortcodeParamColor    ShortcodeParamType = "color"
	ShortcodeParamTextarea ShortcodeParamType = "textarea"
)

// ShortcodeParamRole describes where a value is placed in a shortcode.
type ShortcodeParamRole string

const (
	ShortcodeParamPositional ShortcodeParamRole = "positional"
	ShortcodeParamNamed      ShortcodeParamRole = "named"
	ShortcodeParamFlag       ShortcodeParamRole = "flag"
	ShortcodeParamContent    ShortcodeParamRole = "content"
)

type ShortcodeType string

const (
	ShortcodeInline ShortcodeType = "inline"
	ShortcodeBlock  ShortcodeType = "block"
	ShortcodeHybrid ShortcodeType = "hybrid"
)

type ShortcodeCapability string

const (
	ShortcodeStatic      ShortcodeCapability = "static"
	ShortcodeInteractive ShortcodeCapability = "interactive"
	ShortcodeService     ShortcodeCapability = "service"
	ShortcodeExternal    ShortcodeCapability = "external"
)

// ShortcodeParamSchema is the backend-owned schema consumed by Console forms.
type ShortcodeParamSchema struct {
	Label       string             `json:"label,omitempty"`
	Key         string             `json:"key"`
	Type        ShortcodeParamType `json:"type"`
	Role        ShortcodeParamRole `json:"role"`
	Default     any                `json:"default,omitempty"`
	Options     []string           `json:"options,omitempty"`
	Placeholder string             `json:"placeholder,omitempty"`
}

// ShortcodeParamDoc is the human-readable parameter reference for the editor.
type ShortcodeParamDoc struct {
	Name    string `json:"name"`
	Desc    string `json:"desc"`
	Default string `json:"default,omitempty"`
}

// ShortcodeCatalogEntry is the backend source of truth for parsed shortcode tags.
type ShortcodeCatalogEntry struct {
	Name        string                 `json:"name"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Signature   string                 `json:"signature"`
	Example     string                 `json:"example"`
	Type        ShortcodeType          `json:"type"`
	Category    string                 `json:"category"`
	Capability  ShortcodeCapability    `json:"capability"`
	Params      []ShortcodeParamSchema `json:"params"`
	ParamDocs   []ShortcodeParamDoc    `json:"param_docs"`
}

type shortcodeRegistration struct {
	entry  ShortcodeCatalogEntry
	parser *regexp.Regexp
}

var (
	shortcodeColors  = []string{"", "red", "orange", "yellow", "green", "cyan", "blue", "purple", "gray"}
	shortcodeDevices = []string{"iphone", "ipad", "macbook"}
)

func positional(key string, defaultValue any) ShortcodeParamSchema {
	return ShortcodeParamSchema{Key: key, Type: ShortcodeParamText, Role: ShortcodeParamPositional, Default: defaultValue}
}

func positionalSelect(key string, defaultValue any, options ...string) ShortcodeParamSchema {
	return ShortcodeParamSchema{Key: key, Type: ShortcodeParamSelect, Role: ShortcodeParamPositional, Default: defaultValue, Options: options}
}

func named(key string, defaultValue any) ShortcodeParamSchema {
	return ShortcodeParamSchema{Key: key, Type: ShortcodeParamText, Role: ShortcodeParamNamed, Default: defaultValue}
}

func namedSelect(key string, defaultValue any, options ...string) ShortcodeParamSchema {
	return ShortcodeParamSchema{Key: key, Type: ShortcodeParamSelect, Role: ShortcodeParamNamed, Default: defaultValue, Options: options}
}

func flag(key string, defaultValue bool) ShortcodeParamSchema {
	return ShortcodeParamSchema{Key: key, Type: ShortcodeParamSwitch, Role: ShortcodeParamFlag, Default: defaultValue}
}

func content(key string, defaultValue any) ShortcodeParamSchema {
	return ShortcodeParamSchema{Key: key, Type: ShortcodeParamTextarea, Role: ShortcodeParamContent, Default: defaultValue}
}

func shortcodeEntry(name string, typ ShortcodeType, category string, capability ShortcodeCapability, parser *regexp.Regexp, params ...ShortcodeParamSchema) shortcodeRegistration {
	if params == nil {
		params = []ShortcodeParamSchema{}
	}
	return shortcodeRegistration{
		entry: ShortcodeCatalogEntry{
			Name: name, Type: typ, Category: category, Capability: capability, Params: params,
		},
		parser: parser,
	}
}

// shortcodeRegistry is deliberately adjacent to the parser's regexp names:
// adding a parser without adding a catalog entry should be caught by review/tests.
var shortcodeRegistry = []shortcodeRegistration{
	shortcodeEntry("note", ShortcodeInline, "提示标注", ShortcodeStatic, reNote,
		namedSelect("color", "blue", shortcodeColors...), positional("title", nil), content("content", nil)),
	shortcodeEntry("quot", ShortcodeInline, "提示标注", ShortcodeStatic, reQuot,
		namedSelect("el", "p", "p", "h2", "h3", "h4", "h5", "h6"), named("icon", "hashtag"), named("text", "")),
	shortcodeEntry("mark", ShortcodeInline, "提示标注", ShortcodeStatic, reMark,
		positional("text", "重点内容"), namedSelect("color", "yellow", shortcodeColors...)),
	shortcodeEntry("button", ShortcodeInline, "提示标注", ShortcodeStatic, reButton,
		positional("text", "访问主页"), positional("url", "https://example.com"), namedSelect("color", "blue", shortcodeColors...), namedSelect("size", nil, "", "small", "large")),
	shortcodeEntry("checkbox", ShortcodeInline, "提示标注", ShortcodeStatic, reCheckbox,
		positional("text", "已完成的任务"), flag("checked", true), namedSelect("color", "green", shortcodeColors...)),
	shortcodeEntry("radio", ShortcodeInline, "提示标注", ShortcodeStatic, reCheckbox,
		positional("text", "选项一"), flag("checked", true), namedSelect("color", "blue", shortcodeColors...)),
	shortcodeEntry("hashtag", ShortcodeInline, "提示标注", ShortcodeStatic, reHashtag,
		positional("text", "日常"), positional("href", nil), namedSelect("color", nil, shortcodeColors...)),
	shortcodeEntry("emoji", ShortcodeInline, "提示标注", ShortcodeStatic, reEmoji,
		positionalSelect("source", "default", "default", "qq", "tieba", "aru", "blobcat", "twemoji"), positional("name", "aini"), named("height", nil)),
	shortcodeEntry("icon", ShortcodeInline, "提示标注", ShortcodeStatic, reIcon,
		positional("key", "example:planet"), namedSelect("color", "blue", shortcodeColors...), positional("text", nil)),

	shortcodeEntry("folding", ShortcodeBlock, "布局容器", ShortcodeInteractive, reFolding,
		namedSelect("color", nil, shortcodeColors...), flag("open", true), positional("title", "点击查看详情"), content("content", "这里是折叠内容，支持 **Markdown**。")),
	shortcodeEntry("tabs", ShortcodeBlock, "布局容器", ShortcodeInteractive, reTabs,
		named("active", nil), content("content", nil)),
	shortcodeEntry("timeline", ShortcodeBlock, "布局容器", ShortcodeStatic, reTimeline,
		content("content", nil)),
	shortcodeEntry("gallery", ShortcodeBlock, "布局容器", ShortcodeInteractive, reGallery,
		namedSelect("layout", "grid", "grid", "flow"), named("size", "mix"), named("ratio", "origin"), content("content", nil)),
	shortcodeEntry("folders", ShortcodeBlock, "布局容器", ShortcodeInteractive, reFolders,
		namedSelect("color", nil, shortcodeColors...), content("content", nil)),
	shortcodeEntry("grid", ShortcodeBlock, "布局容器", ShortcodeStatic, reGrid,
		named("w", "240px"), named("c", nil), named("gap", nil), named("br", nil), content("content", nil)),
	shortcodeEntry("table", ShortcodeBlock, "布局容器", ShortcodeStatic, reTable,
		namedSelect("style", nil, "scroll", "wrap", "compact"), content("content", nil)),
	shortcodeEntry("tip", ShortcodeBlock, "布局容器", ShortcodeStatic, reTip,
		named("text", nil), content("content", nil)),
	shortcodeEntry("blockquote", ShortcodeBlock, "布局容器", ShortcodeStatic, reBlockquoteTag,
		namedSelect("indent", false, "", "true", "false"), content("content", nil)),
	shortcodeEntry("banner", ShortcodeBlock, "布局容器", ShortcodeStatic, reBannerTag,
		named("title", nil), named("subtitle", nil), named("bg", nil), named("avatar", nil), named("link", nil), content("content", nil)),
	shortcodeEntry("navbar", ShortcodeHybrid, "布局容器", ShortcodeInteractive, reNavbarTag,
		named("active", nil), content("content", nil)),
	shortcodeEntry("frame", ShortcodeHybrid, "布局容器", ShortcodeStatic, reFrameTag,
		namedSelect("device", "iphone", shortcodeDevices...), named("width", nil), content("content", nil)),
	shortcodeEntry("box", ShortcodeBlock, "布局容器", ShortcodeStatic, reBox,
		namedSelect("color", nil, shortcodeColors...), namedSelect("child", nil, shortcodeColors...), positional("title", "卡片标题"), content("content", "卡片正文，支持 **Markdown**。")),
	shortcodeEntry("about", ShortcodeBlock, "布局容器", ShortcodeStatic, reAbout,
		named("avatar", nil), named("border", nil), named("height", nil), flag("back", false), content("content", nil)),
	shortcodeEntry("toc", ShortcodeInline, "布局容器", ShortcodeStatic, reTOCTag,
		positional("title", "TOC"), namedSelect("display", nil, "", "mobile")),

	shortcodeEntry("mbti", ShortcodeInline, "文艺装饰", ShortcodeStatic, reMbti,
		positional("type", "INTJ-T"), positional("scores", "93-72-78-83-51"), named("url", nil), named("image", nil)),
	shortcodeEntry("poetry", ShortcodeBlock, "文艺装饰", ShortcodeStatic, rePoetry,
		positional("title", nil), named("author", nil), named("date", nil), named("footer", nil), content("content", nil)),
	shortcodeEntry("paper", ShortcodeBlock, "文艺装饰", ShortcodeStatic, rePaperTag,
		namedSelect("style", nil, "", "underline"), named("title", nil), named("author", nil), named("date", nil), named("footer", nil), content("content", nil)),
	shortcodeEntry("reel", ShortcodeBlock, "文艺装饰", ShortcodeStatic, reReelTag,
		positional("title", nil), named("author", nil), named("date", nil), named("footer", nil), content("content", nil)),
	shortcodeEntry("psw", ShortcodeInline, "文艺装饰", ShortcodeStatic, reInlineLabel, positional("text", nil)),
	shortcodeEntry("emp", ShortcodeInline, "文艺装饰", ShortcodeStatic, reInlineLabel, positional("text", nil)),
	shortcodeEntry("wavy", ShortcodeInline, "文艺装饰", ShortcodeStatic, reInlineLabel, positional("text", nil)),
	shortcodeEntry("blur", ShortcodeInline, "文艺装饰", ShortcodeStatic, reInlineLabel, positional("text", nil)),
	shortcodeEntry("sub", ShortcodeInline, "文艺装饰", ShortcodeStatic, reSupSub, positional("text", nil), namedSelect("color", nil, shortcodeColors...)),
	shortcodeEntry("sup", ShortcodeInline, "文艺装饰", ShortcodeStatic, reSupSub, positional("text", nil), namedSelect("color", nil, shortcodeColors...)),
	shortcodeEntry("u", ShortcodeInline, "文艺装饰", ShortcodeStatic, reInlineLabel, positional("text", nil)),
	shortcodeEntry("del", ShortcodeInline, "文艺装饰", ShortcodeStatic, reInlineLabel, positional("text", nil)),
	shortcodeEntry("kbd", ShortcodeInline, "文艺装饰", ShortcodeStatic, reInlineLabel, positional("text", nil)),

	shortcodeEntry("image", ShortcodeInline, "媒体播放", ShortcodeStatic, reImage,
		positional("src", "/assets/img/preview/image-demo.jpg"), positional("alt", "风景图"), named("width", "80%"), named("bg", nil), flag("download", false), named("ratio", nil)),
	shortcodeEntry("link", ShortcodeInline, "媒体播放", ShortcodeStatic, reLink,
		positional("url", "https://example.com"), positional("title", "示例站点"), named("desc", nil), named("icon", nil)),
	shortcodeEntry("video", ShortcodeInline, "媒体播放", ShortcodeStatic, reVideo,
		positional("src", "/assets/media/preview/sample-video.mp4"), named("bilibili", nil), named("youtube", nil), named("ratio", "16/9"), named("width", "100%"), flag("autoplay", false)),
	shortcodeEntry("audio", ShortcodeInline, "媒体播放", ShortcodeStatic, reAudio,
		positional("src", "/assets/media/preview/sample-audio.mp3"), named("netease", nil), named("type", "audio/mp3"), flag("autoplay", false)),
	shortcodeEntry("swiper", ShortcodeBlock, "媒体播放", ShortcodeInteractive, reSwiper,
		namedSelect("width", "", "", "min", "max"), namedSelect("effect", "slide", "slide", "fade", "coverflow", "cards"), content("content", nil)),
	shortcodeEntry("albums", ShortcodeInline, "媒体播放", ShortcodeService, reAlbumsTag,
		named("repo", nil), named("api", nil), namedSelect("size", "s", "s", "m", "l", "xl", "mix")),
	shortcodeEntry("posters", ShortcodeInline, "媒体播放", ShortcodeService, rePostersTag,
		named("repo", nil), named("api", nil), namedSelect("size", "xs", "xs", "s", "m", "l", "xl", "mix")),
	shortcodeEntry("copy", ShortcodeInline, "媒体播放", ShortcodeStatic, reCopy,
		positional("text", "npm install stellar-theme"), named("git", nil), named("prefix", nil)),
	shortcodeEntry("rating", ShortcodeInline, "媒体播放", ShortcodeService, reRating,
		positional("id", "demo-post"), positional("title", "给这篇文章打分")),
	shortcodeEntry("vote", ShortcodeInline, "媒体播放", ShortcodeService, reVote,
		positional("title", nil), named("id", "default"), named("yes", "ri:arrow-up-line"), named("no", "ri:arrow-down-line")),
	shortcodeEntry("friends", ShortcodeInline, "媒体播放", ShortcodeService, reFriends, named("api", "/api/friends")),
	shortcodeEntry("links", ShortcodeInline, "媒体播放", ShortcodeStatic, reLinks),
	shortcodeEntry("sites", ShortcodeInline, "媒体播放", ShortcodeService, reSites, named("api", nil)),
	shortcodeEntry("md", ShortcodeInline, "媒体播放", ShortcodeExternal, reMDTag, positional("src", nil)),

	shortcodeEntry("mermaid", ShortcodeBlock, "数据图表", ShortcodeExternal, reMermaid, content("content", nil)),
	shortcodeEntry("okr", ShortcodeBlock, "数据图表", ShortcodeStatic, reOKR,
		positional("title", "o1"), named("percent", nil), namedSelect("status", "in_track", "in_track", "at_risk", "off_track", "finished", "unfinished"), named("color", nil), content("content", nil)),
	shortcodeEntry("chat", ShortcodeBlock, "数据图表", ShortcodeStatic, reChat,
		positional("device", nil), namedSelect("style", "qq", "qq", "wechat"), named("title", nil), namedSelect("scene", "group", "group", "single"), named("me", nil), content("content", nil)),

	shortcodeEntry("ghcard", ShortcodeInline, "GitHub", ShortcodeService, reGHCard,
		positional("repo", "anuraghazra/github-readme-stats"), named("theme", nil), flag("show_owner", true), named("title_color", nil)),
	shortcodeEntry("ghrepo", ShortcodeInline, "GitHub", ShortcodeService, reGHRepo, positional("repo", nil)),
	shortcodeEntry("ghuser", ShortcodeInline, "GitHub", ShortcodeService, reGHUser, positional("username", nil)),
}

// ShortcodeCatalog returns a defensive copy so callers cannot mutate the registry.
func ShortcodeCatalog() []ShortcodeCatalogEntry {
	items := make([]ShortcodeCatalogEntry, 0, len(shortcodeRegistry))
	for _, registration := range shortcodeRegistry {
		entry := registration.entry
		if display, ok := shortcodeDisplayMetadata[entry.Name]; ok {
			if display.FormFields != nil {
				entry.Params = append([]ShortcodeParamSchema(nil), display.FormFields...)
			}
			entry.Title = display.Title
			entry.Description = display.Description
			entry.Signature = display.Signature
			entry.Example = display.Example
			entry.ParamDocs = append([]ShortcodeParamDoc(nil), display.ParamDocs...)
		}
		entry.Params = append([]ShortcodeParamSchema(nil), entry.Params...)
		if entry.Params == nil {
			entry.Params = []ShortcodeParamSchema{}
		}
		for i := range entry.Params {
			entry.Params[i].Options = append([]string(nil), entry.Params[i].Options...)
		}
		if entry.ParamDocs == nil {
			entry.ParamDocs = []ShortcodeParamDoc{}
		}
		items = append(items, entry)
	}
	return items
}
