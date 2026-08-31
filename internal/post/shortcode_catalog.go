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

// ShortcodeEditVariant 描述 hybrid 标签当前 raw 实际采用的形式。
type ShortcodeEditVariant string

const (
	ShortcodeVariantInline ShortcodeEditVariant = "inline"
	ShortcodeVariantBlock  ShortcodeEditVariant = "block"
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
	LabelKey    string             `json:"label_key,omitempty"`
	Hint        string             `json:"hint,omitempty"`
	HintKey     string             `json:"hint_key,omitempty"`
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

// ShortcodeEditSchema describes the safety boundary of the visual editor.
// Params remains the field layout; this schema owns whether editing is
// enabled and which values are required to preserve renderer semantics.
type ShortcodeEditSchema struct {
	Enabled     bool       `json:"enabled"`
	Required    []string   `json:"required,omitempty"`
	RequiredAny [][]string `json:"required_any,omitempty"`
	LegacyArgs  []string   `json:"legacy_args,omitempty"`
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
	Services    []string               `json:"services,omitempty"`
	Plugins     []string               `json:"plugins,omitempty"`
	Fixture     string                 `json:"fixture,omitempty"`
	MainJS      bool                   `json:"main_js,omitempty"`
	Height      int                    `json:"height,omitempty"`
	Fallback    string                 `json:"fallback,omitempty"`
	Params      []ShortcodeParamSchema `json:"params"`
	ParamDocs   []ShortcodeParamDoc    `json:"param_docs"`
	Edit        ShortcodeEditSchema    `json:"edit"`
}

type shortcodeRegistration struct {
	entry  ShortcodeCatalogEntry
	parser *regexp.Regexp
}

var (
	shortcodeColors  = []string{"", "red", "orange", "yellow", "green", "cyan", "blue", "purple", "gray"}
	shortcodeDevices = []string{"iphone", "ipad", "macbook"}
)

// --- 参数构造器 ---
// catalog 是参数真值：Console 表单、插件库插入、可视化属性编辑共用同一份 schema。
// 每个字段必须对应当前 renderer 真实消费的参数，避免属性编辑重新生成时静默丢参数。

func positional(key, label string, defaultValue any) ShortcodeParamSchema {
	return ShortcodeParamSchema{Key: key, Label: label, Type: ShortcodeParamText, Role: ShortcodeParamPositional, Default: defaultValue}
}

func positionalSelect(key, label string, defaultValue any, options ...string) ShortcodeParamSchema {
	return ShortcodeParamSchema{Key: key, Label: label, Type: ShortcodeParamSelect, Role: ShortcodeParamPositional, Default: defaultValue, Options: options}
}

func named(key, label string, defaultValue any) ShortcodeParamSchema {
	return ShortcodeParamSchema{Key: key, Label: label, Type: ShortcodeParamText, Role: ShortcodeParamNamed, Default: defaultValue}
}

func namedNumber(key, label string, defaultValue any) ShortcodeParamSchema {
	return ShortcodeParamSchema{Key: key, Label: label, Type: ShortcodeParamNumber, Role: ShortcodeParamNamed, Default: defaultValue}
}

func namedSelect(key, label string, defaultValue any, options ...string) ShortcodeParamSchema {
	return ShortcodeParamSchema{Key: key, Label: label, Type: ShortcodeParamSelect, Role: ShortcodeParamNamed, Default: defaultValue, Options: options}
}

func flag(key, label string, defaultValue bool) ShortcodeParamSchema {
	return ShortcodeParamSchema{Key: key, Label: label, Type: ShortcodeParamSwitch, Role: ShortcodeParamFlag, Default: defaultValue}
}

func content(key, label string, defaultValue any) ShortcodeParamSchema {
	return ShortcodeParamSchema{Key: key, Label: label, Type: ShortcodeParamTextarea, Role: ShortcodeParamContent, Default: defaultValue}
}

// withPlaceholder / withHint 只补充展示信息，不改变参数语义。
func (p ShortcodeParamSchema) withPlaceholder(v string) ShortcodeParamSchema {
	p.Placeholder = v
	return p
}

func (p ShortcodeParamSchema) withHint(v string) ShortcodeParamSchema {
	p.Hint = v
	return p
}

func shortcodeEntry(name string, typ ShortcodeType, category string, capability ShortcodeCapability, parser *regexp.Regexp, params ...ShortcodeParamSchema) shortcodeRegistration {
	if params == nil {
		params = []ShortcodeParamSchema{}
	}
	return shortcodeRegistration{
		entry: ShortcodeCatalogEntry{
			Name: name, Type: typ, Category: category, Capability: capability, Params: params,
			Edit: ShortcodeEditSchema{Enabled: true},
		},
		parser: parser,
	}
}

func (r shortcodeRegistration) withEdit(schema ShortcodeEditSchema) shortcodeRegistration {
	r.entry.Edit = schema
	return r
}

// shortcodeRegistry is deliberately adjacent to the parser's regexp names:
// adding a parser without adding a catalog entry should be caught by review/tests.
var shortcodeRegistry = []shortcodeRegistration{
	// --- 提示标注 ---
	shortcodeEntry("note", ShortcodeInline, "提示标注", ShortcodeStatic, reNote,
		namedSelect("color", "配色", "blue", shortcodeColors...), positional("title", "标题（可选）", nil), content("content", "内容", nil)),
	// prefix/suffix 为渲染器真实消费的参数（shortcode_okr.go renderQuot），缺声明会让属性编辑重建时丢失
	shortcodeEntry("quot", ShortcodeInline, "提示标注", ShortcodeStatic, reQuot,
		namedSelect("el", "标签", "p", "p", "h2", "h3", "h4", "h5", "h6"), named("icon", "图标 key", "hashtag"),
		named("prefix", "前缀图标 key（留空用主题默认）", nil), named("suffix", "后缀图标 key（留空用主题默认）", nil),
		named("text", "标题文本", nil)).withEdit(ShortcodeEditSchema{Enabled: true, LegacyArgs: []string{"text"}}),
	shortcodeEntry("mark", ShortcodeInline, "提示标注", ShortcodeStatic, reMark,
		positional("text", "文本", "重点内容"), namedSelect("color", "颜色", "yellow", shortcodeColors...)).withEdit(ShortcodeEditSchema{Enabled: true, Required: []string{"text"}}),
	shortcodeEntry("button", ShortcodeInline, "提示标注", ShortcodeStatic, reButton,
		positional("text", "按钮文本", "访问主页"), positional("url", "链接", "https://example.com"),
		namedSelect("color", "配色", "blue", shortcodeColors...), namedSelect("size", "尺寸", nil, "", "small", "large"), named("icon", "图标 key", nil)).withEdit(ShortcodeEditSchema{Enabled: true, Required: []string{"text", "url"}}),
	shortcodeEntry("checkbox", ShortcodeInline, "提示标注", ShortcodeStatic, reCheckbox,
		positional("text", "文本", "已完成的任务"), flag("checked", "默认勾选", true),
		namedSelect("color", "配色", "green", shortcodeColors...), named("symbol", "勾选符号", nil)).withEdit(ShortcodeEditSchema{Enabled: true, Required: []string{"text"}}),
	shortcodeEntry("radio", ShortcodeInline, "提示标注", ShortcodeStatic, reCheckbox,
		positional("text", "文本", "选项一"), flag("checked", "默认选中", true),
		namedSelect("color", "配色", "blue", shortcodeColors...), named("symbol", "选中符号", nil)).withEdit(ShortcodeEditSchema{Enabled: true, Required: []string{"text"}}),
	shortcodeEntry("hashtag", ShortcodeInline, "提示标注", ShortcodeStatic, reHashtag,
		positional("text", "文本", "日常"), positional("href", "链接（可选）", nil),
		namedSelect("color", "颜色（留空循环）", nil, shortcodeColors...)),
	shortcodeEntry("emoji", ShortcodeInline, "提示标注", ShortcodeStatic, reEmoji,
		positionalSelect("source", "来源（留空用默认源）", nil, "", "default", "qq", "tieba", "aru", "blobcat", "twemoji"),
		positional("name", "表情名称", "aini"), named("url", "图片 URL（URL 模式）", nil), named("height", "高度", nil)),
	shortcodeEntry("icon", ShortcodeInline, "提示标注", ShortcodeStatic, reIcon,
		positional("key", "图标 key", "example:planet"), namedSelect("color", "颜色", "blue", shortcodeColors...),
		named("style", "内联样式", nil), positional("text", "附文本（可选）", nil)),

	// --- 布局容器 ---
	shortcodeEntry("folding", ShortcodeBlock, "布局容器", ShortcodeInteractive, reFolding,
		namedSelect("color", "配色", nil, shortcodeColors...),
		// child 对齐原版 folding.js 的 ['color','child','open']；只放行 main.css 定义的两类子元素排版
		namedSelect("child", "子元素排版", nil, "", "codeblock", "iframe"),
		flag("open", "默认展开", true),
		positional("title", "标题", "点击查看详情"), content("content", "正文", "这里是折叠内容，支持 **Markdown**。")),
	shortcodeEntry("tabs", ShortcodeBlock, "布局容器", ShortcodeInteractive, reTabs,
		named("active", "默认激活序号（从 1 起）", nil), namedSelect("align", "对齐方式", nil, "", "left", "center", "right"),
		content("content", "Tabs 内容（<!-- tab 标题 --> 分隔）", nil)),
	// api 进入客户端数据服务模式（容器变 ds-timeline）；user/limit/hide 由 services/timeline.js 读取，
	// 属性口径与本仓库 widgets/timeline.html 保持一致。原版允许任意 key 进 data-*，本项目收窄为白名单。
	shortcodeEntry("timeline", ShortcodeBlock, "布局容器", ShortcodeStatic, reTimeline,
		named("api", "数据服务 URL（填了则由客户端渲染时间流）", nil),
		named("user", "只显示这些用户（逗号分隔）", nil),
		named("limit", "透传给客户端（数量经 api 的 per_page 控制）", nil),
		named("hide", "隐藏字段：user/title/footer（逗号分隔）", nil),
		content("content", "时间轴内容（<!-- node 时间 --> 分隔）", nil)),
	// 上游 dropdown.js 注册的 tag 级 icon 参数实际未被渲染使用，本项目不声明。
	// 下拉项图标依赖 ctx.utils.icon（本项目无对应解析器），仅输出 data-icon 占位容器。
	shortcodeEntry("dropdown", ShortcodeBlock, "布局容器", ShortcodeInteractive, reDropdown,
		namedSelect("direction", "弹出方向", nil, "", "up", "down"),
		namedSelect("align", "对齐", nil, "", "left", "right"),
		flag("open", "默认展开", false),
		positional("title", "触发按钮文字", "菜单"),
		content("content", "下拉项（每行一个 - [标题](链接)，可前缀 icon:key）", "- [首页](/)\n- [关于](/about)")),
	shortcodeEntry("gallery", ShortcodeBlock, "布局容器", ShortcodeInteractive, reGallery,
		namedSelect("layout", "布局", "grid", "grid", "flow"), named("size", "尺寸模式", "mix"),
		named("ratio", "宽高比", "origin"), content("content", "图片（每行一个 Markdown 图片）", nil)),
	shortcodeEntry("folders", ShortcodeBlock, "布局容器", ShortcodeInteractive, reFolders,
		namedSelect("color", "配色", nil, shortcodeColors...), content("content", "折叠项（<!-- folder 标题 --> 分隔）", nil)),
	shortcodeEntry("grid", ShortcodeBlock, "布局容器", ShortcodeStatic, reGrid,
		namedSelect("bg", "背景样式", nil, "", "card"), named("w", "最小列宽", "240px"), named("c", "固定列数", nil),
		named("gap", "单元格间距", nil), named("br", "单元格圆角", nil), content("content", "单元格（<!-- cell --> 分隔）", nil)),
	shortcodeEntry("table", ShortcodeBlock, "布局容器", ShortcodeStatic, reTable,
		namedSelect("style", "样式", nil, "scroll", "wrap", "compact"), content("content", "表格 Markdown", nil)),
	shortcodeEntry("tip", ShortcodeBlock, "布局容器", ShortcodeStatic, reTip,
		named("text", "提示文字", nil), content("content", "提示内容", nil)).withEdit(ShortcodeEditSchema{Enabled: true, LegacyArgs: []string{"text"}}),
	shortcodeEntry("blockquote", ShortcodeBlock, "布局容器", ShortcodeStatic, reBlockquoteTag,
		namedSelect("indent", "首行缩进", nil, "", "true", "false"), content("content", "引用正文", nil)),
	shortcodeEntry("banner", ShortcodeBlock, "布局容器", ShortcodeStatic, reBannerTag,
		named("title", "标题", nil), named("subtitle", "副标题", nil), named("bg", "背景图 URL", nil),
		named("avatar", "头像 URL", nil), named("link", "跳转链接", nil), content("content", "正文", nil)).withEdit(ShortcodeEditSchema{Enabled: true, LegacyArgs: []string{"title", "subtitle"}}),
	shortcodeEntry("navbar", ShortcodeHybrid, "布局容器", ShortcodeInteractive, reNavbarTag,
		named("active", "高亮项", nil), content("content", "导航链接（每行 [文本](链接)）", nil)),
	shortcodeEntry("frame", ShortcodeHybrid, "布局容器", ShortcodeStatic, reFrameTag,
		namedSelect("device", "设备", "iphone", shortcodeDevices...), named("width", "最大宽度", nil),
		content("content", "媒体内容（Markdown 图片/视频）", nil)).withEdit(ShortcodeEditSchema{Enabled: true, LegacyArgs: []string{"device"}}),
	shortcodeEntry("box", ShortcodeBlock, "布局容器", ShortcodeStatic, reBox,
		namedSelect("color", "外框配色", nil, shortcodeColors...), namedSelect("child", "子元素配色", nil, shortcodeColors...),
		positional("title", "标题", "卡片标题"), content("content", "正文", "卡片正文，支持 **Markdown**。")),
	shortcodeEntry("about", ShortcodeBlock, "布局容器", ShortcodeStatic, reAbout,
		named("avatar", "头像 URL", nil), named("border", "头像边框", nil), named("height", "头像高度", nil),
		flag("back", "显示返回箭头", false), content("content", "正文", nil)),
	shortcodeEntry("toc", ShortcodeInline, "布局容器", ShortcodeStatic, reTOCTag,
		positional("title", "目录标题", "TOC"), named("wiki", "Wiki 项目（未启用时输出空）", nil),
		namedSelect("open", "默认展开", nil, "", "true", "false"), namedSelect("display", "显示方式", nil, "", "mobile")),

	// --- 文艺装饰 ---
	shortcodeEntry("mbti", ShortcodeInline, "文艺装饰", ShortcodeStatic, reMbti,
		positional("type", "人格类型", "INTJ-T").withPlaceholder("INTJ-T"),
		positional("scores", "五维百分比", "93-72-78-83-51").withPlaceholder("93-72-78-83-51"),
		named("url", "档案链接", nil), named("image", "横幅图片", nil)),
	shortcodeEntry("poetry", ShortcodeBlock, "文艺装饰", ShortcodeStatic, rePoetry,
		positional("title", "标题", nil), named("author", "作者", nil), named("date", "日期", nil),
		named("footer", "页脚", nil), content("content", "正文", nil)),
	shortcodeEntry("paper", ShortcodeBlock, "文艺装饰", ShortcodeStatic, rePaperTag,
		namedSelect("style", "标题下划线装饰", nil, "", "underline"), named("title", "标题", nil),
		named("author", "作者", nil), named("date", "日期", nil), named("footer", "页脚", nil),
		content("content", "正文（<!-- paragraph/section/line --> 标记）", nil)),
	shortcodeEntry("reel", ShortcodeBlock, "文艺装饰", ShortcodeStatic, reReelTag,
		positional("title", "标题", nil), named("author", "作者", nil), named("date", "日期", nil),
		named("footer", "页脚", nil), content("content", "正文", nil)),
	shortcodeEntry("psw", ShortcodeInline, "文艺装饰", ShortcodeStatic, reInlineLabel, positional("text", "隐藏文本", nil)),
	shortcodeEntry("emp", ShortcodeInline, "文艺装饰", ShortcodeStatic, reInlineLabel, positional("text", "着重文本", nil)),
	shortcodeEntry("wavy", ShortcodeInline, "文艺装饰", ShortcodeStatic, reInlineLabel, positional("text", "波浪文本", nil)),
	shortcodeEntry("blur", ShortcodeInline, "文艺装饰", ShortcodeStatic, reInlineLabel, positional("text", "模糊文本", nil)),
	shortcodeEntry("sub", ShortcodeInline, "文艺装饰", ShortcodeStatic, reSupSub,
		positional("text", "下标文本", nil), namedSelect("color", "配色", nil, shortcodeColors...)),
	shortcodeEntry("sup", ShortcodeInline, "文艺装饰", ShortcodeStatic, reSupSub,
		positional("text", "上标文本", nil), namedSelect("color", "配色", nil, shortcodeColors...)),
	shortcodeEntry("u", ShortcodeInline, "文艺装饰", ShortcodeStatic, reInlineLabel, positional("text", "文本", nil)),
	shortcodeEntry("del", ShortcodeInline, "文艺装饰", ShortcodeStatic, reInlineLabel, positional("text", "文本", nil)),
	shortcodeEntry("kbd", ShortcodeInline, "文艺装饰", ShortcodeStatic, reInlineLabel, positional("text", "按键", nil)),

	// --- 媒体播放 / 数据服务 ---
	shortcodeEntry("image", ShortcodeInline, "媒体播放", ShortcodeStatic, reImage,
		positional("src", "图片地址", "/assets/img/preview/image-demo.jpg"), positional("alt", "替代文本", "风景图"),
		named("width", "宽度", "80%"), named("height", "高度", nil), named("bg", "背景色", nil),
		// download 对齐原版 image.js 的 [true/false/url]：渲染器已支持 URL 形态，
		// 建模为 flag 会让 download:https://… 在属性面板直接报错（未知值不是布尔）
		named("download", "下载按钮：true 用图片本身，或填下载 URL", nil), named("padding", "内边距", nil),
		named("fancybox", "灯箱分组", nil), named("ratio", "宽高比（如 16/9）", nil)).withEdit(ShortcodeEditSchema{Enabled: true, Required: []string{"src"}}),
	shortcodeEntry("link", ShortcodeInline, "媒体播放", ShortcodeStatic, reLink,
		positional("url", "链接", "https://example.com"), positional("title", "标题", "示例站点"),
		named("desc", "描述（提供则用 rich 布局）", nil), named("icon", "图标/缩略图 URL", nil)),
	shortcodeEntry("video", ShortcodeInline, "媒体播放", ShortcodeStatic, reVideo,
		positional("src", "视频地址", "/assets/media/preview/sample-video.mp4"), named("bilibili", "B 站 BV 号", nil),
		named("youtube", "YouTube 视频 ID", nil), named("type", "MIME 类型", nil),
		named("ratio", "宽高比", "16/9"), named("width", "最大宽度", "100%"), flag("autoplay", "自动播放", false)).withEdit(ShortcodeEditSchema{Enabled: true, RequiredAny: [][]string{{"src", "bilibili", "youtube"}}}),
	shortcodeEntry("audio", ShortcodeInline, "媒体播放", ShortcodeStatic, reAudio,
		positional("src", "音频地址", "/assets/media/preview/sample-audio.mp3"), named("netease", "网易云歌曲 ID", nil),
		named("type", "MIME 类型", "audio/mp3"), flag("autoplay", "自动播放", false)).withEdit(ShortcodeEditSchema{Enabled: true, RequiredAny: [][]string{{"src", "netease"}}}),
	shortcodeEntry("swiper", ShortcodeBlock, "媒体播放", ShortcodeInteractive, reSwiper,
		namedSelect("width", "宽度语义", "", "", "min", "max"),
		namedSelect("effect", "切换效果", "slide", "slide", "fade", "coverflow", "cards"),
		content("content", "轮播图片（每行一张 Markdown 图片）", nil)),
	shortcodeEntry("albums", ShortcodeInline, "媒体播放", ShortcodeService, reAlbumsTag,
		named("repo", "GitHub 仓库（owner/repo）", nil), named("api", "数据服务 URL", nil),
		namedSelect("size", "卡片尺寸", "s", "s", "m", "l", "xl", "mix")),
	shortcodeEntry("posters", ShortcodeInline, "媒体播放", ShortcodeService, rePostersTag,
		named("repo", "GitHub 仓库（owner/repo）", nil), named("api", "数据服务 URL", nil),
		namedSelect("size", "卡片尺寸", "xs", "xs", "s", "m", "l", "xl", "mix")),
	shortcodeEntry("copy", ShortcodeInline, "媒体播放", ShortcodeStatic, reCopy,
		positional("text", "复制文本", "npm install stellar-theme"), named("git", "git 仓库（user/repo）", nil),
		named("prefix", "前缀标签文本", nil)).withEdit(ShortcodeEditSchema{Enabled: true, Required: []string{"text"}}),
	shortcodeEntry("rating", ShortcodeInline, "媒体播放", ShortcodeService, reRating,
		named("id", "评分对象 ID", "demo-post"), named("icon", "评分图标 key", nil),
		positional("title", "标题", "给这篇文章打分")),
	shortcodeEntry("vote", ShortcodeInline, "媒体播放", ShortcodeService, reVote,
		positional("title", "标题", nil), named("id", "投票对象 ID", "default"),
		named("yes", "赞成图标 key", "ri:arrow-up-line"), named("no", "反对图标 key", "ri:arrow-down-line")),
	shortcodeEntry("friends", ShortcodeInline, "媒体播放", ShortcodeService, reFriends,
		named("api", "数据服务 URL", "/api/friends"), flag("posts", "带 RSS 文章列表", false)),
	shortcodeEntry("links", ShortcodeInline, "媒体播放", ShortcodeStatic, reLinks),
	shortcodeEntry("moments", ShortcodeInline, "媒体播放", ShortcodeService, reMoments,
		namedNumber("limit", "显示条数", 5).withHint("最多显示 50 条")),
	shortcodeEntry("sites", ShortcodeInline, "媒体播放", ShortcodeService, reSites, named("api", "数据服务 URL", nil)),
	shortcodeEntry("md", ShortcodeInline, "媒体播放", ShortcodeExternal, reMDTag,
		positional("src", "Markdown 文件 URL", nil), namedSelect("wrap", "包裹容器", nil, "", "true", "false")),

	// --- 数据图表 ---
	shortcodeEntry("mermaid", ShortcodeBlock, "数据图表", ShortcodeExternal, reMermaid, content("content", "Mermaid 源码", nil)),
	shortcodeEntry("okr", ShortcodeBlock, "数据图表", ShortcodeStatic, reOKR,
		positional("title", "目标编号", "o1"), named("percent", "完成百分比", nil),
		namedSelect("status", "状态", "in_track", "in_track", "at_risk", "off_track", "finished", "unfinished"),
		namedSelect("color", "配色", nil, shortcodeColors...),
		content("content", "目标与 KR（<!-- okr krN --> 分隔）", nil)),
	shortcodeEntry("chat", ShortcodeBlock, "数据图表", ShortcodeStatic, reChat,
		positional("device", "设备", nil).withPlaceholder("iphone"),
		namedSelect("style", "样式", "qq", "qq", "wechat"), named("title", "标题", nil),
		namedSelect("scene", "场景", "group", "group", "single"), named("me", "靠右显示的参与者 key（single 模式）", nil),
		// labelColorStyle 为渲染器真实消费的参数（shortcode_rich.go renderChat），缺声明会让属性编辑重建时丢失
		named("labelColorStyle", "标签配色风格（渲染器透传给成员标签）", nil),
		content("content", "参与者 YAML + 消息（<!-- chattip --> / <!-- chatcell -->）", nil)),

	// --- GitHub ---
	shortcodeEntry("ghcard", ShortcodeInline, "GitHub", ShortcodeService, reGHCard,
		positional("repo", "user/repo 或 user", "anuraghazra/github-readme-stats"), named("theme", "主题", nil),
		flag("show_owner", "显示仓库所有者", true), named("title_color", "标题颜色", nil),
		named("text_color", "文本颜色", nil), named("icon_color", "图标颜色", nil),
		named("bg_color", "背景颜色", nil), flag("hide_border", "隐藏边框", false),
		named("cache_seconds", "缓存秒数", nil), named("locale", "语言", nil)),
	shortcodeEntry("ghrepo", ShortcodeInline, "GitHub", ShortcodeService, reGHRepo, positional("repo", "user/repo", nil)),
	shortcodeEntry("ghuser", ShortcodeInline, "GitHub", ShortcodeService, reGHUser, positional("username", "GitHub 用户名", nil)),
}

// ShortcodeCatalog returns a defensive copy so callers cannot mutate the registry.
func ShortcodeCatalog() []ShortcodeCatalogEntry {
	items := make([]ShortcodeCatalogEntry, 0, len(shortcodeRegistry))
	for _, registration := range shortcodeRegistry {
		entry := registration.entry
		if display, ok := shortcodeDisplayMetadata[entry.Name]; ok {
			if display.PreviewSet {
				entry.Services = append([]string(nil), display.Services...)
				entry.Plugins = append([]string(nil), display.Plugins...)
				entry.Fixture = display.Fixture
				entry.MainJS = display.MainJS
				entry.Height = display.Height
				entry.Fallback = display.Fallback
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
		entry.Edit.Required = append([]string(nil), entry.Edit.Required...)
		entry.Edit.RequiredAny = append([][]string(nil), entry.Edit.RequiredAny...)
		for i := range entry.Edit.RequiredAny {
			entry.Edit.RequiredAny[i] = append([]string(nil), entry.Edit.RequiredAny[i]...)
		}
		entry.Edit.LegacyArgs = append([]string(nil), entry.Edit.LegacyArgs...)
		if entry.ParamDocs == nil {
			entry.ParamDocs = []ShortcodeParamDoc{}
		}
		items = append(items, entry)
	}
	return items
}

// findShortcodeCatalogEntry 按标签名查 catalog；可视化编辑只处理 catalog 内标签。
func findShortcodeCatalogEntry(name string) (ShortcodeCatalogEntry, bool) {
	for _, item := range ShortcodeCatalog() {
		if item.Name == name {
			return item, true
		}
	}
	return ShortcodeCatalogEntry{}, false
}
