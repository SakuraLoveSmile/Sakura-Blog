// 展示文案只解释现有行为；字段类型、默认值与保存规则仍由原有配置维护。
const switchEffects: Record<string, [string, string]> = {
  "theme.article.banner.background": ["文章顶部显示横幅背景，让标题区更突出。", "文章顶部不显示横幅背景。"],
  "theme.article.listing.show_tags": ["文章列表卡片会显示文章标签。", "文章列表卡片隐藏标签，保留文章页自身设置。"],
  "theme.article.show_reading_time": ["文章信息中显示预计阅读时长。", "文章信息中隐藏预计阅读时长。"],
  "theme.article.footer.show_tags": ["正文结束后显示文章标签，读者可继续浏览同类内容。", "文章页脚不显示标签。"],
  "blog_extensions.content.enabled": [
    "文章扩展已启用，下方分享、打赏等已开启功能可在前台显示。",
    "文章扩展已关闭；下方配置会保留，启用后才生效。",
  ],
  "blog_extensions.content.read_next": [
    "启用文章扩展后，正文结束处可显示下一篇文章入口。",
    "正文结束处隐藏扩展的下一篇文章入口。",
  ],
  "blog_extensions.content.share.enabled": [
    "启用文章扩展后显示所选平台的分享入口。",
    "不显示扩展分享入口，已选平台仍保留。",
  ],
  "blog_extensions.content.reward.enabled": [
    "启用文章扩展并配置收款码后，文章可显示打赏入口。",
    "隐藏打赏入口，保留文案与收款码配置。",
  ],
  "blog_extensions.native_comments.enabled": [
    "允许显示 Blog 原生评论区；单页开关和审核规则仍会影响实际展示。",
    "关闭 Blog 原生评论；第三方评论由下方独立设置。",
  ],
  "blog_extensions.native_comments.home.enabled": ["原生评论开启后，首页也允许显示评论区。", "首页不展示原生评论区。"],
  "blog_extensions.native_comments.moderation": [
    "新评论需经审核后才公开显示。",
    "新评论无需经过这项审核开关，其他评论校验仍会执行。",
  ],
  "theme.open_graph.enabled": [
    "页面包含社交分享元信息；平台抓取后可生成标题、描述与图片卡片。",
    "页面不输出主题 Open Graph 元信息。",
  ],
  "blog_extensions.seo.no_index": [
    "页面向搜索引擎声明不应收录；已收录内容的更新取决于搜索引擎重新抓取。",
    "不再通过此开关禁止收录；是否收录仍由搜索引擎决定。",
  ],
  "theme.search.local.include_content": [
    "本地搜索可匹配正文；索引文件通常也会更大。仅用于本地静态索引。",
    "本地静态索引不包含正文，正文中的词语不会通过此索引命中。",
  ],
};
const effects: Record<string, Record<string, string>> = {
  "theme.article.style": {
    tech: "采用适合技术内容的文章排版；首行缩进仍可由下一项单独指定。",
    story: "采用叙事文章排版；自动缩进会随文章风格调整。",
  },
  "theme.article.paragraph_indent": {
    auto: "是否缩进由文章风格决定，适合让技术与叙事文章分别排版。",
    always: "段落首行始终缩进，适合连续阅读的叙事正文。",
    never: "段落首行不缩进，适合技术说明和条目内容。",
  },
  "theme.article.listing.card_layout": {
    hero: "文章列表使用大图卡片，让封面成为主要视觉；有封面的文章更容易看出区别。",
    classic: "文章列表使用经典图文卡片，标题与摘要更集中。",
  },
  "theme.article.listing.pinned_layout": {
    carousel: "多篇置顶文章以轮播组织；需要有置顶文章才能观察效果。",
    flat: "置顶文章直接平铺展示，读者无需切换轮播。",
  },
  "theme.search.provider": {
    local: "访客浏览器加载静态索引搜索；下方索引范围、正文与缓存设置生效。",
    go: "访客搜索直接请求 Blog 服务；下方本地静态索引设置不影响此模式。",
    algolia: "访客搜索交给 Algolia，需要下方参数和可用的远程索引。",
    none: "停用主题搜索服务；访客无法通过该搜索服务查询内容。",
  },
};
const help: Record<string, string> = {
  "theme.article.listing.cover_ratio": "宽 ÷ 高；相同宽度下数值越大，封面越扁。2 表示 2:1。",
  "theme.article.listing.excerpt_length": "控制文章列表的自动摘要长度；不截断文章正文。0 会隐藏自动摘要。",
  "theme.article.related_posts_limit": "控制文章结尾最多推荐几篇相关文章；0 隐藏推荐。需要存在可关联文章。",
  "blog_extensions.content.reward.text": "读者展开文章打赏区域时看到的说明。需先启用文章扩展和打赏。",
  "blog_extensions.native_comments.title": "读者在原生评论区顶部看到的标题。",
  "blog_extensions.native_comments.admin_badge": "管理员发表评论时，昵称旁显示的身份文字。",
  "theme.canonical.host": "影响页面声明的规范网址；应与希望搜索引擎收录的正式域名一致。",
  "blog_extensions.seo.default_description": "页面缺少单独描述时使用，可能出现在搜索结果或分享摘要中。",
  "blog_extensions.seo.default_keywords": "作为页面缺省关键词元信息，不保证影响搜索排名。",
  "theme.search.local.scope": "仅影响本地静态索引包含哪些内容；不影响文章是否公开。",
  "theme.inject.head_begin": "插入每个前台页面的 head 开始处，通常用于元信息或早期加载的资源。",
  "theme.inject.head_end": "插入每个前台页面的 head 结束前，通常用于样式或站点验证代码。",
  "theme.inject.body_begin": "插入前台 body 开始处；可改变页面结构或执行脚本。",
  "theme.inject.body_end": "插入前台 body 结束前；脚本效果需保存后在前台检查，静态预览不会执行。",
};
export function settingFieldEffect(path: string, value: unknown): string | undefined {
  if (path in switchEffects) return switchEffects[path][value ? 0 : 1];
  return effects[path]?.[String(value)] ?? help[path];
}
