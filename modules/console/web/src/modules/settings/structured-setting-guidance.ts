// 这里只描述用户能看到的效果；类型、默认值与枚举仍由上游 schema 决定。
const help: Record<string, string> = {
  "theme.appearance": "先选择整体风格，再调整配色与细节。这里的选择会影响公开网站，不会改变管理后台。",
  "theme.appearance.preset": "选择网站的整体视觉层次。下方为风格示意，实际效果可在草稿预览中查看。",
  "theme.appearance.color_scheme": "控制访客页面的明暗。自动模式跟随访客设备；访客已保存的个人配色可能优先。",
  "theme.appearance.colors": "分别控制主要交互、强调内容和正文链接的颜色。",
  "theme.appearance.colors.primary":
    "主题色用于主要交互与主题色元素；不会把整页背景改成此颜色。支持 #HEX、rgb() 或 hsl()。",
  "theme.appearance.colors.accent": "用于需要强调的内容和强调色元素，与普通链接颜色分开。",
  "theme.appearance.colors.link": "改变文章正文中链接的颜色，让可点击内容更容易辨认。",
  "theme.appearance.gradients": "为主要操作与搜索栏设置渐变背景，填写完整的 CSS 渐变表达式。",
  "theme.appearance.gradients.primary_action": "改变主要操作区域的渐变背景，填写完整的 CSS 渐变表达式。",
  "theme.appearance.gradients.search_bar": "改变前台搜索栏使用的渐变背景。",
  "theme.appearance.typography": "调整公开网站的阅读感受，包括正文、代码、字号与对齐方式。",
  "theme.appearance.typography.font_family": "分别设置正文与代码字体；字体需在访客设备上可用，或已由网站加载。",
  "theme.appearance.typography.font_family.body":
    "影响网站正文和常规文本。多个字体用逗号分隔，前一个不可用时尝试下一个。",
  "theme.appearance.typography.font_family.code": "影响行内代码和代码块，通常使用等宽字体。",
  "theme.appearance.typography.font_size": "分别控制基础文字、行内代码和代码块字号，可使用 px、rem 或百分比。",
  "theme.appearance.typography.font_size.root": "基础字号越大，网站文字与使用相对单位的元素通常也越大。",
  "theme.appearance.typography.font_size.inline_code": "只调整段落中行内代码的字号，例如 85% 表示正文大小的 85%。",
  "theme.appearance.typography.font_size.code_block": "只调整独立代码块的字号，不改变普通段落。",
  "theme.appearance.typography.content_align": "左对齐保留自然行尾；两端对齐会拉开字间距，让段落两侧更整齐。",
  "theme.appearance.typography.font_smoothing": "调整字体边缘的平滑方式，实际差异与浏览器、操作系统有关。",
  "theme.appearance.typography.font_weight": "将指定字重映射为实际使用的字重，影响标题和正文的粗细层次。",
  "theme.appearance.shape": "调整卡片和图片边缘，数值越大通常越圆润。",
  "theme.appearance.shape.corner": "控制圆角曲线形状；连续曲率的显示取决于浏览器支持。",
  "theme.appearance.shape.radius": "分别调整不同尺寸卡片、横条和图片的圆角，例如 16px。",
  "theme.appearance.backgrounds": "设置侧栏与整页背景。背景图片、遮罩和模糊会共同影响文字对比度。",
  "theme.appearance.backgrounds.leftbar": "只影响左侧栏背景，不改变文章内容区域。",
  "theme.appearance.backgrounds.leftbar.type": "选择侧栏使用渐变还是图片；选择图片后需提供图片地址。",
  "theme.appearance.backgrounds.leftbar.image": "侧栏背景图地址，仅在相应背景类型下可见。",
  "theme.appearance.backgrounds.leftbar.gradient": "为浅色与深色分别设置渐变色组，配色切换时使用对应的一组。",
  "theme.appearance.backgrounds.leftbar.opacity": "0 为完全透明，1 为完全不透明；降低后背景会更淡。",
  "theme.appearance.backgrounds.leftbar.backdrop": "模糊侧栏背景细节，较大的模糊半径会让背景更柔和。",
  "theme.appearance.backgrounds.page": "影响整页背景，建议同时检查文章文字是否清晰。",
  "theme.appearance.backgrounds.page.image": "设置公开网站的整页背景图片，使用可公开访问的图片地址。",
  "theme.appearance.backgrounds.page.backdrop": "调整整页背景的模糊、遮罩和饱和度，帮助内容保持可读。",
  "theme.appearance.backgrounds.page.backdrop.overlay": "覆盖在背景图片上的遮罩颜色；更不透明的遮罩会弱化背景。",
  "theme.appearance.backgrounds.page.backdrop.saturation": "改变背景颜色的鲜艳程度，例如 100% 为原始饱和度。",
  "theme.appearance.code_block": "只影响代码块的滚动条和语法高亮。",
  "theme.appearance.code_block.scrollbar_width": "改变代码块横向滚动条的粗细，例如 4px。",
  "theme.appearance.code_block.highlight_stylesheet": "加载指定的语法高亮样式；地址必须可访问，且与当前配色协调。",
  "theme.topbar": "页面顶部的品牌与导航区域。先启用，再设置内容；页面自身的覆盖项可能改变最终展示。",
  "theme.topbar.enabled": "开启后显示网站顶栏；关闭后访客页面不再展示该区域。",
  "theme.topbar.brand": "设置顶栏标识、名称和跳转地址，让访客知道身处哪个站点。",
  "theme.topbar.menu": "从左到右显示顶栏菜单；调整列表顺序即可改变前台顺序。",
  "theme.topbar.widgets": "在顶栏放置组件，最终展示取决于组件类型与内容。",
  "theme.leftbar": "网站左侧的品牌、搜索与菜单区域。页面布局可以单独覆盖其中的组件。",
  "theme.leftbar.enabled": "关闭后隐藏左侧栏，访客将无法从该区域使用菜单和搜索。",
  "theme.leftbar.default_state": "决定进入网站时侧栏默认展开还是收起；窄屏仍按移动端布局显示。",
  "theme.leftbar.brand": "设置侧栏品牌区的图片、标题、副标题和搜索入口。",
  "theme.leftbar.brand.search": "关闭后隐藏品牌区搜索入口；搜索服务本身在“搜索”分类中配置。",
  "theme.leftbar.brand.style": "标准样式保留较完整的品牌区，紧凑样式减少其占用空间。",
  "theme.leftbar.menu_columns": "控制侧栏菜单分成几列；项目较多时多列更紧凑。",
  "theme.leftbar.menu": "侧栏菜单按列表顺序显示；菜单标识还用于页面的当前项高亮。",
  "theme.leftbar.widgets": "设置全局侧栏组件；页面布局有覆盖时，以对应页面的组件列表为准。",
  "theme.leftbar.footer": "左侧栏底部的快捷操作入口，不是整站页脚。",
  "theme.rightbar": "网站右侧的目录、仓库等辅助组件区域，可由各类页面分别覆盖。",
  "theme.rightbar.enabled": "关闭后隐藏右侧栏；开启后仍需有可显示的组件。",
  "theme.rightbar.widgets": "设置默认右侧栏组件及顺序；各类页面可以使用自己的列表。",
  "theme.footer": "网站内容末尾的公共页脚，与文章自身的许可协议区分开。",
  "theme.footer.sitemap": "在页脚展示分组链接，适合放置站点栏目与外部入口。",
  "theme.footer.content": "显示在整站页脚的文字内容，支持现有的 Markdown 与主题占位符。",
  "theme.profiles": "只覆盖指定类型的页面，其他页面保持原样。未设置表示继承；空组件列表表示该区域不显示组件。",
  "theme.tags": "调整正文中标签组件的默认外观，文章内显式指定的参数仍可覆盖默认值。",
  "theme.tags.note": "改变提示块的默认颜色和边框，不影响普通段落。",
  "theme.tags.note.border": "开启后为提示块显示边框，关闭后弱化块的轮廓。",
  "theme.tags.checkbox.interactive": "开启后访客可在页面中勾选复选框；不会修改文章源内容。",
  "theme.tags.quot": "设置引用、话题和问题三类引用块使用的前后缀图标。",
  "theme.tags.emoji": "设置表情图片的默认来源及地址模板。",
  "theme.tags.gallery": "改变图库中图片的尺寸组织与默认比例。",
  "theme.features": "按需开启阅读与交互增强；部分功能需要页面有对应内容才会出现。",
  "theme.features.partial_navigation": "同站跳转时局部更新页面，减少整页刷新；脚本兼容性需在实际导航中检查。",
  "theme.features.color_scheme_switch": "开启后提供访客配色切换入口，访客可以选择自己的明暗偏好。",
  "theme.features.image_optimization": "启用主题图片优化处理，实际效果取决于图片与页面标记。",
  "theme.features.lazy_loading": "图片接近可见区域时再加载，减少初始加载量；这里还可调整入场过渡与宽高比。",
  "theme.features.lightbox": "让匹配选择器的图片可点击放大；需在实际文章图片上验证。",
  "theme.features.link_prefetch": "预先请求可能访问的站内页面，提高跳转速度，也会增加预加载请求。",
  "theme.features.reveal": "让内容进入视口时播放淡入或位移动画；持续时间和间隔单位为毫秒。",
  "theme.features.card_hover": "鼠标经过卡片时增加聚光或倾斜效果，触屏设备上不具备相同的悬停体验。",
  "theme.features.card_hover.spotlight": "开启后鼠标位置附近显示聚光层次。",
  "theme.features.card_hover.tilt": "开启后卡片随鼠标位置轻微倾斜。",
  "theme.features.heti": "为中文正文加入更细致的中西文间距与排版处理。",
  "theme.features.math": "选择数学公式渲染服务；文章包含公式时才能看到渲染差异。",
  "theme.features.diagrams": "选择图表渲染服务；文章包含对应图表代码时生效。",
  "theme.services": "配置正文组件所用的外部接口。保存只保存地址与参数，不代表服务已连通。",
  "theme.services.gravatar": "为使用 Gravatar 的头像选择请求地址，影响头像加载来源。",
  "theme.services.markdown": "为前台需要动态解析 Markdown 的组件提供渲染脚本。",
  "theme.services.site_info": "为站点信息组件提供数据来源，需要页面实际放置对应组件。",
  "theme.services.rating": "为评分组件提供服务地址，保存后需在实际评分区域检查。",
  "theme.services.vote": "为投票组件提供服务地址，保存后需在实际投票区域检查。",
  "theme.services.contributors": "设置展示哪些 GitHub 仓库的贡献者。",
  "theme.services.github_card": "设置 GitHub 信息卡的数据服务，影响卡片内统计信息。",
  "theme.services.github": "调整 GitHub API、原始文件与 Gist 的访问来源。",
  "theme.fallbacks": "当内容没有自己的资源时使用这些默认图片，不会替换已有的自定义图片。",
  "theme.fallbacks.avatar": "没有独立头像时使用的默认头像。",
  "theme.fallbacks.link_card": "链接卡片缺少图片时使用的默认图片。",
  "theme.fallbacks.cover": "内容缺少封面时使用的默认封面。",
  "theme.comments": "选择第三方评论服务并填写其参数；仅配置参数不会自动启用每类页面的评论。",
  "theme.search.algolia": "使用 Algolia 搜索时所需的应用、索引和搜索 Key；切换其他服务不会使用这些参数。",
};

const profileLabels: Record<string, string> = {
  home: "首页",
  blog_index: "文章列表",
  post: "文章页",
  topic: "专栏页",
  wiki_index: "文档列表",
  wiki: "文档页",
  notebook: "笔记本",
  notebooks: "笔记本列表",
  note: "笔记页",
  author: "作者页",
  page: "独立页面",
  settings: "访客设置页",
  error: "错误页",
};

export function structuredSettingHelp(path: string): string | undefined {
  if (Object.hasOwn(help, path)) return help[path];
  const profile = /^theme\.profiles\.([^.]+)(?:\.(.+))?$/.exec(path);
  if (profile && Object.hasOwn(profileLabels, profile[1])) {
    const label = profileLabels[profile[1]];
    if (!profile[2]) return `只调整${label}，不会改变其他类型的页面。`;
    if (profile[2] === "path") return `改变${label}的访问路径；修改前请检查已有链接是否需要同步。`;
    if (/^(leftbar|rightbar)$/.test(profile[2]))
      return `只覆盖${label}的${profile[2] === "leftbar" ? "左" : "右"}侧栏。`;
    if (/^(leftbar|rightbar)\.widgets$/.test(profile[2]))
      return `按顺序显示${label}的侧栏组件；空列表隐藏这些组件，恢复默认则沿用默认列表。`;
    if (profile[2] === "active_menu") return `指定${label}在菜单中高亮的项目，需与菜单项目的标识一致。`;
    if (profile[2] === "listing") return `控制${label}的条目数量、排序和摘要。`;
    if (profile[2] === "comments") return `只调整${label}的评论区域。`;
  }
  if (/^theme\.appearance\.shape\.radius\.[^.]+$/.test(path))
    return "填写带单位的圆角长度，例如 16px；越大越圆润，只影响对应类型的元素。";
  if (/^theme\.appearance\.backgrounds\..*\.radius$/.test(path))
    return "填写带单位的模糊半径，例如 100px；越大背景细节越模糊。";
  if (/^theme\.(topbar|leftbar)\.brand\.(name|tagline)$/.test(path))
    return "填写此区域显示的品牌文字；恢复默认时使用主题与站点的默认内容。";
  if (/^theme\.(topbar|leftbar)\.brand\.image$/.test(path))
    return "为品牌区域选择图片地址与展示样式，头像样式会裁切成头像形状。";
  return undefined;
}

export function isStructuredColor(path: string): boolean {
  return (
    /^theme\.appearance\.colors\.[^.]+$/.test(path) ||
    /^theme\.appearance\.backgrounds\.leftbar\.gradient\.(light|dark)\.\d+$/.test(path) ||
    /^theme\.(?:profiles\.[^.]+\.)?(?:topbar|leftbar)\.menu\.\d+\.accent$/.test(path) ||
    path === "theme.appearance.backgrounds.page.backdrop.overlay"
  );
}

const visualChoicePaths = new Set([
  "theme.appearance.preset",
  "theme.appearance.color_scheme",
  "theme.article.listing.card_layout",
  "theme.article.listing.pinned_layout",
  "theme.article.style",
  "theme.search.provider",
]);
export function structuredSettingChoicesAvailable(path: string): boolean {
  return visualChoicePaths.has(path);
}
