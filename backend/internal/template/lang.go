package template

import "fmt"

// zhCN 文案表，键名对齐原版 Stellar languages/zh-CN.yml。
// 本项目特有文案以 nav. / comment. / password. / topic. / friend. / wiki. / widget. / js. 前缀区分。
var zhCN = map[string]string{
	// ── btn 组（对齐 zh-CN.yml btn.*）──────────────────────────
	"btn.home":           "主页",
	"btn.blog":           "文章",
	"btn.wiki":           "文档",
	"btn.topic":          "专栏",
	"btn.notebook":       "笔记本",
	"btn.recent_publish": "近期发布",
	"btn.all_wiki":       "所有项目",
	"btn.categories":     "分类",
	"btn.tags":           "标签",
	"btn.archives":       "归档",
	"btn.all_posts":      "所有文章",
	"btn.edit":           "编辑本文",
	"btn.top":            "回到顶部",
	"btn.comments":       "参与讨论",
	"btn.all":            "全部",
	"btn.toc":            "目录",
	"btn.menu":           "菜单",

	// ── meta 组（对齐 zh-CN.yml meta.*）────────────────────────
	"meta.recent_update":  "最近更新",
	"meta.tag_tree":       "标签",
	"meta.all_notes":      "所有笔记",
	"meta.toc":            "本文目录",
	"meta.read_next":      "接下来阅读",
	"meta.prev":           "回顾上一篇",
	"meta.next":           "接下来阅读",
	"meta.older":          "较早文章",
	"meta.newer":          "较新文章",
	"meta.references":     "参考资料",
	"meta.related_posts":  "您可能感兴趣的文章",
	"meta.back_to_top":    "回到顶部",
	"meta.more":           "更多",
	"meta.created_author": "本文由 %s 发布于",
	"meta.created":        "发布于",
	"meta.updated":        "更新于",
	"meta.license":        "许可协议",
	"meta.share":          "分享文章",
	"meta.contributors":   "本文贡献者",

	// 时间后缀（对齐 zh-CN.yml meta.date_suffix.*）
	"date_suffix.just":  "刚刚",
	"date_suffix.min":   "分钟前",
	"date_suffix.hour":  "小时前",
	"date_suffix.day":   "天前",
	"date_suffix.month": "个月前",

	// ── page 组（对齐 zh-CN.yml page.*）────────────────────────
	"page.error.what":   "很抱歉，您访问的页面不存在",
	"page.error.why":    "可能是输入地址有误或该地址已被删除",
	"page.error.action": "返回主页",

	// ── search 组（对齐 zh-CN.yml search.*）────────────────────
	"search.search":    "站内搜索",
	"search.search_in": "在 %s 中搜索",
	"search.no_result": "没有找到内容！",

	// ── message 组（对齐 zh-CN.yml message.*）──────────────────
	"message.copied": "复制成功",

	// ── symbol 组（对齐 zh-CN.yml symbol.*）────────────────────
	"symbol.comma":      "，",
	"symbol.period":     "。",
	"symbol.colon":      "：",
	"symbol.brackets_l": "（",
	"symbol.brackets_r": "）",

	// ── 本项目特有文案 ─────────────────────────────────────────

	// 导航前缀（动态 tab 显示"分类：xxx"等）
	"nav.category": "分类：%s",
	"nav.tag":      "标签：%s",
	"nav.topic":    "专栏：%s",

	// 面包屑
	"breadcrumb.home":     "首页",
	"breadcrumb.blog":     "博客",
	"breadcrumb.topic":    "专栏",
	"breadcrumb.notes":    "笔记",
	"breadcrumb.moments":  "瞬间",
	"breadcrumb.friends":  "友链",
	"breadcrumb.links":    "友链",
	"breadcrumb.wiki":     "文库",
	"breadcrumb.notebook": "笔记本",
	"breadcrumb.note":     "笔记",

	// 文章页
	"article.expire":         "本文最后更新于 %d 天前，其中的信息可能已经过时，请谨慎参考。",
	"article.copyright":      "版权声明",
	"article.author":         "作者",
	"article.original_link":  "原文链接",
	"article.reward":         "打赏",
	"article.reward_default": "打赏",

	// 分享
	"share.wechat":        "微信分享",
	"share.weibo":         "分享到微博",
	"share.email":         "邮件分享",
	"share.copy_link":     "复制链接",
	"share.wechat_qrcode": "微信二维码",
	"share.wechat_pay":    "微信",
	"share.alipay":        "支付宝",
	"share.alipay_qrcode": "支付宝收款码",
	"share.prompt_copy":   "复制链接",

	// 评论区
	"comment.reply":       "回复",
	"comment.cancel":      "取消",
	"comment.placeholder": "编写评论，支持 Markdown",
	"comment.nickname":    "昵称",
	"comment.email":       "邮箱",
	"comment.website":     "网站",
	"comment.website_opt": "网站（选填）",
	"comment.email_hint":  "邮箱仅用于头像与回复通知，不会公开",
	"comment.submit":      "发布评论",
	"comment.count":       "%d 条评论",
	"comment.no_comment":  "暂无评论",
	"comment.loading":     "加载评论中…", // Spec 50 评论区懒加载占位
	"comment.reply_to":    "回复",
	"comment.like":        "赞",

	// 500 错误页（404 用 page.error.*）
	"error500.what": "很抱歉，服务器遇到了内部错误",
	"error500.why":  "请稍后再试，或返回主页继续浏览",

	// 笔记 / 瞬间 / 专栏 / 友链空态
	"notes.empty":      "暂无内容",
	"moments.empty":    "还没有瞬间，敬请期待。",
	"topic.empty":      "还没有专栏。",
	"topic.post_count": "共 %d 篇文章",
	"topic.post_empty": "该专栏下暂无文章。",
	"friend.empty":     "暂无友链",
	"wiki.empty":       "暂无文库项目",
	"wiki.no_docs":     "该项目暂无文档",

	// 笔记本（Spec 35b）
	"notebook.title":      "笔记本",
	"notebook.empty":      "暂无笔记本",
	"notebook.note_empty": "暂无笔记",
	"notebook.note_count": "共 %d 篇笔记",

	// Widget 标题缺省值
	"widget.siteinfo":       "站点统计",
	"widget.tagcloud":       "标签",
	"widget.recent":         "最近更新",
	"widget.popular":        "热门文章",
	"widget.latest_comment": "最新评论",
	"widget.toc":            "本文目录",
	"widget.author_posts":   "全部文章",
	"widget.tagtree":        "标签",
	"widget.posts":          "文章",
	"widget.tags":           "标签",
	"widget.comments":       "评论",
	"widget.views":          "阅读量",

	// 欢迎语缺省（markdown widget fallback）
	"widget.welcome":      "这里是用 Go 重建的 Stellar 风格博客，欢迎随处逛逛。",
	"widget.welcome_hint": "左侧可以搜索文章，右下角可以切换深浅色主题。",

	// 作者页（Spec 38 多作者体系）
	"author.page_title": "作者：%s",
	"author.all_posts":  "全部文章",

	// 相关 widget
	"widget.related": "专栏：%s",

	// 密码页
	"password.title":     "需要口令",
	"password.body":      "本文已加密，请输入阅读密码后继续。",
	"password.label":     "阅读密码",
	"password.submit":    "解锁阅读",
	"password.unlocking": "解锁中...",
	"password.error":     "密码错误，请重试",
	"password.net_error": "网络错误，请检查连接后重试",

	// 主题切换
	"theme.toggle": "切换主题",

	// AI 文章摘要（Spec 53）
	"ai.summary.title":       "AI 摘要",
	"ai.summary.open":        "用你自己的 API 生成 AI 摘要",
	"ai.summary.endpoint":    "接口地址（OpenAI 兼容 base_url）",
	"ai.summary.model":       "模型名称",
	"ai.summary.key":         "API Key",
	"ai.summary.remember":    "记住接口与模型（不含 Key）",
	"ai.summary.generate":    "生成摘要",
	"ai.summary.required":    "请填写接口地址与模型",
	"ai.summary.generating":  "生成中…",
	"ai.summary.privacy":     "Key 仅随本次请求透传，不会被本站保存",
	"ai.summary.done":        "摘要已生成（仅本次会话展示）",
	"ai.summary.error":       "生成失败：%s",
	"ai.summary.bad_gateway": "上游接口调用失败，请检查地址/Key/模型",

	// JS ctx 注入（base.html 的 ctx 对象）
	"js.copy":         "复制",
	"js.copy_success": "复制成功",
	"js.copied":       "已复制",
}

// Lang 返回 key 对应的中文文案，支持 fmt.Sprintf 式参数替换。
// 未命中返回 key 本身（便于排查遗漏）。
func Lang(key string, args ...interface{}) string {
	v, ok := zhCN[key]
	if !ok {
		return key
	}
	if len(args) > 0 {
		return fmt.Sprintf(v, args...)
	}
	return v
}
