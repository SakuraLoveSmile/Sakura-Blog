// Console 插件库元数据（Spec 23）。
// 数据源对照 internal/post/shortcode.go 现有实现逐一核对，纯前端静态数据。
// 插件变更频率低，与 shortcode.go 同步靠人工维护。

export type PluginCategory = "提示标注" | "布局容器" | "媒体播放" | "数据图表" | "GitHub";
export type PluginType = "inline" | "block";

export interface PluginParam {
  name: string;
  desc: string;
  default?: string;
}

export interface PluginMeta {
  /** 标签名（不含 {% %}） */
  name: string;
  /** 中文标题 */
  title: string;
  category: PluginCategory;
  type: PluginType;
  /** 一句话简介 */
  desc: string;
  /** 语法签名 */
  signature: string;
  /** 参数表 */
  params: PluginParam[];
  /** 可运行示例（直接送 preview/markdown 渲染） */
  example: string;
}

export const PLUGIN_CATEGORIES: PluginCategory[] = [
  "提示标注",
  "布局容器",
  "媒体播放",
  "数据图表",
  "GitHub",
];

export const plugins: PluginMeta[] = [
  // ==================== 提示标注 ====================
  {
    name: "note",
    title: "提示框 Note",
    category: "提示标注",
    type: "inline",
    desc: "带标题与配色的提示块，用于强调说明。",
    signature: "{% note [color:颜色] [标题] 内容 %}",
    params: [
      { name: "color", desc: "配色：red/orange/yellow/green/cyan/blue/purple/gray", default: "blue" },
      { name: "标题", desc: "可选，第二个位置参数起为正文", default: "—" },
    ],
    example: "{% note color:green 提示 这是一条成功提示，支持 **Markdown** 行内语法。 %}",
  },
  {
    name: "quot",
    title: "小标题 Quot",
    category: "提示标注",
    type: "inline",
    desc: "带前缀图标的强调小标题，可作为锚点标题。",
    signature: "{% quot [el:h2] [icon:图标] text:文本 %}",
    params: [
      { name: "el", desc: "标题标签：p/h2/h3/h4", default: "p" },
      { name: "icon", desc: "前缀图标 key（Iconify）", default: "hashtag" },
      { name: "text", desc: "标题文本（也可用位置参数）", default: "—" },
    ],
    example: "{% quot el:h2 text:这是一个小标题 %}",
  },
  {
    name: "mark",
    title: "荧光标记 Mark",
    category: "提示标注",
    type: "inline",
    desc: "行内荧光笔高亮文本。",
    signature: "{% mark 文本 [color:颜色] %}",
    params: [{ name: "color", desc: "高亮颜色", default: "yellow" }],
    example: "这里有一段 {% mark 重点内容 color:cyan %} 需要高亮。",
  },
  {
    name: "button",
    title: "按钮 Button",
    category: "提示标注",
    type: "inline",
    desc: "可点击的链接按钮。",
    signature: "{% button 文本 链接 [color:颜色] [size:尺寸] %}",
    params: [
      { name: "color", desc: "按钮配色", default: "blue" },
      { name: "size", desc: "尺寸：small/large", default: "—" },
    ],
    example: "{% button 访问主页 https://example.com color:purple %}",
  },
  {
    name: "checkbox",
    title: "复选框 Checkbox",
    category: "提示标注",
    type: "inline",
    desc: "行内复选框，可用于清单展示。",
    signature: "{% checkbox [checked] 文本 [color:颜色] %}",
    params: [
      { name: "checked", desc: "位置参数，出现即勾选", default: "false" },
      { name: "color", desc: "配色", default: "blue" },
    ],
    example: "{% checkbox checked 已完成的任务 color:green %}\n{% checkbox 待办的任务 %}",
  },
  {
    name: "hashtag",
    title: "话题标签 Hashtag",
    category: "提示标注",
    type: "inline",
    desc: "带 # 图标的话题标签，颜色可循环或指定。",
    signature: "{% hashtag 文本 [链接] [color:颜色] %}",
    params: [
      { name: "href", desc: "可选跳转链接（位置参数）", default: "—" },
      { name: "color", desc: "缺省时按红橙黄绿青蓝紫循环", default: "循环" },
    ],
    example: "{% hashtag 日常 %} {% hashtag 技术 color:blue %} {% hashtag 随笔 %}",
  },
  {
    name: "emoji",
    title: "表情 Emoji",
    category: "提示标注",
    type: "inline",
    desc: "行内表情图，支持 QQ 表情与 twemoji。",
    signature: "{% emoji [来源] 名称 [height:高度] %}",
    params: [
      { name: "source", desc: "来源：default(QQ)/twemoji，省略用默认", default: "default" },
      { name: "height", desc: "自定义高度", default: "—" },
    ],
    example: "开心 {% emoji default 1 %} 点赞 {% emoji twemoji 1f44d %}",
  },
  {
    name: "icon",
    title: "内联图标 Icon",
    category: "提示标注",
    type: "inline",
    desc: "内联 Iconify 图标，可附文本。",
    signature: "{% icon key [color:颜色] [文本] %}",
    params: [
      { name: "key", desc: "Iconify 图标 key", default: "—" },
      { name: "color", desc: "图标颜色", default: "blue" },
    ],
    example: "{% icon solar:star-bold color:orange %} 收藏 {% icon solar:heart-bold color:red %} 喜欢",
  },

  // ==================== 布局容器 ====================
  {
    name: "folding",
    title: "折叠块 Folding",
    category: "布局容器",
    type: "block",
    desc: "可展开/收起的折叠区块，正文支持 Markdown。",
    signature: "{% folding [color:颜色] [open:true] 标题 %} 正文 {% endfolding %}",
    params: [
      { name: "color", desc: "配色", default: "—" },
      { name: "open", desc: "true 时默认展开", default: "false" },
    ],
    example: "{% folding color:cyan open:true 点击查看详情 %}\n这里是折叠内容，支持 **Markdown**。\n{% endfolding %}",
  },
  {
    name: "tabs",
    title: "选项卡 Tabs",
    category: "布局容器",
    type: "block",
    desc: "多标签切换容器，用 <!-- tab 标题 --> 分隔各页。",
    signature: "{% tabs [active:序号] %} <!-- tab 标题 --> 内容 {% endtabs %}",
    params: [
      { name: "active", desc: "默认激活的标签序号（从 1 起）", default: "1" },
      { name: "align", desc: "标签对齐方式", default: "—" },
    ],
    example: "{% tabs %}\n<!-- tab 标签一 -->\n第一页内容。\n<!-- tab 标签二 -->\n第二页内容。\n{% endtabs %}",
  },
  {
    name: "timeline",
    title: "时间轴 Timeline",
    category: "布局容器",
    type: "block",
    desc: "垂直时间轴，用 <!-- node 标题 --> 分隔节点。",
    signature: "{% timeline %} <!-- node 时间 --> 内容 {% endtimeline %}",
    params: [],
    example: "{% timeline %}\n<!-- node 2024-01 -->\n项目启动。\n<!-- node 2024-06 -->\n首个版本发布。\n{% endtimeline %}",
  },
  {
    name: "gallery",
    title: "图片墙 Gallery",
    category: "布局容器",
    type: "block",
    desc: "网格/瀑布流图片墙，正文每行一个 Markdown 图片。",
    signature: "{% gallery [layout:grid] [size:mix] [ratio:origin] %} ![alt](src) {% endgallery %}",
    params: [
      { name: "layout", desc: "布局：grid/flow", default: "grid" },
      { name: "size", desc: "尺寸模式", default: "mix" },
      { name: "ratio", desc: "宽高比", default: "origin" },
    ],
    example: "{% gallery %}\n![](https://picsum.photos/400/300?1)\n![](https://picsum.photos/400/300?2)\n![](https://picsum.photos/400/300?3)\n{% endgallery %}",
  },
  {
    name: "folders",
    title: "折叠组 Folders",
    category: "布局容器",
    type: "block",
    desc: "多个折叠项的分组，用 <!-- folder 标题 --> 分隔。",
    signature: "{% folders [color:颜色] %} <!-- folder 标题 --> 内容 {% endfolders %}",
    params: [{ name: "color", desc: "配色", default: "—" }],
    example: "{% folders color:blue %}\n<!-- folder 第一章 -->\n内容一。\n<!-- folder 第二章 -->\n内容二。\n{% endfolders %}",
  },
  {
    name: "grid",
    title: "网格 Grid",
    category: "布局容器",
    type: "block",
    desc: "响应式网格布局，用 <!-- cell --> 分隔单元格。",
    signature: "{% grid [w:列宽] [c:列数] [gap:间距] [br:圆角] %} <!-- cell --> 内容 {% endgrid %}",
    params: [
      { name: "w", desc: "最小列宽（如 240px），与 c 二选一", default: "240px" },
      { name: "c", desc: "固定列数", default: "—" },
      { name: "gap", desc: "单元格间距", default: "—" },
      { name: "br", desc: "单元格圆角", default: "—" },
    ],
    example: "{% grid c:2 gap:12px %}\n<!-- cell -->\n左栏内容。\n<!-- cell -->\n右栏内容。\n{% endgrid %}",
  },
  {
    name: "box",
    title: "卡片盒 Box",
    category: "布局容器",
    type: "block",
    desc: "带标题的卡片容器，正文支持 Markdown。",
    signature: "{% box [color:颜色] [child:子色] 标题 %} 正文 {% endbox %}",
    params: [
      { name: "color", desc: "外框配色", default: "—" },
      { name: "child", desc: "子元素配色", default: "—" },
    ],
    example: "{% box color:green 卡片标题 %}\n卡片正文，支持 **Markdown**。\n{% endbox %}",
  },
  {
    name: "poetry",
    title: "诗词 Poetry",
    category: "布局容器",
    type: "block",
    desc: "居中排版的诗词区块，含标题/作者/日期/页脚。",
    signature: "{% poetry [标题] [author:作者] [date:日期] [footer:页脚] %} 正文 {% endpoetry %}",
    params: [
      { name: "author", desc: "作者", default: "—" },
      { name: "date", desc: "日期", default: "—" },
      { name: "footer", desc: "页脚注", default: "—" },
    ],
    example: "{% poetry 静夜思 author:李白 %}\n床前明月光，\n疑是地上霜。\n举头望明月，\n低头思故乡。\n{% endpoetry %}",
  },

  // ==================== 媒体播放 ====================
  {
    name: "image",
    title: "增强图片 Image",
    category: "媒体播放",
    type: "inline",
    desc: "带尺寸/背景/下载/比例的增强图片。",
    signature: "{% image src [alt] [width:宽] [bg:背景] [download:true] [ratio:比例] %}",
    params: [
      { name: "width", desc: "宽度（如 300px / 50%）", default: "100%" },
      { name: "bg", desc: "背景色", default: "—" },
      { name: "download", desc: "true 显示下载按钮", default: "false" },
      { name: "ratio", desc: "宽高比（如 16/9）", default: "—" },
    ],
    example: "{% image https://picsum.photos/600/360 风景图 width:80% download:true %}",
  },
  {
    name: "link",
    title: "链接卡片 Link",
    category: "媒体播放",
    type: "inline",
    desc: "链接卡片，带描述时为上下结构(rich)，否则左右结构(plain)。",
    signature: "{% link url [标题] [desc:描述] [icon:图标URL] %}",
    params: [
      { name: "desc", desc: "描述，提供则用 rich 布局", default: "—" },
      { name: "icon", desc: "图标/缩略图 URL", default: "—" },
    ],
    example: "{% link https://example.com 示例站点 desc:一个示例链接卡片 %}",
  },
  {
    name: "video",
    title: "视频 Video",
    category: "媒体播放",
    type: "inline",
    desc: "视频播放器，支持本地/Bilibili/YouTube。",
    signature: "{% video src %} / {% video bilibili:BV号 %} / {% video youtube:ID %}",
    params: [
      { name: "bilibili", desc: "B 站 BV 号", default: "—" },
      { name: "youtube", desc: "YouTube 视频 ID", default: "—" },
      { name: "ratio", desc: "宽高比", default: "16/9" },
      { name: "width", desc: "最大宽度", default: "100%" },
      { name: "autoplay", desc: "自动播放", default: "false" },
    ],
    example: "{% video https://www.w3schools.com/html/mov_bbb.mp4 %}",
  },
  {
    name: "audio",
    title: "音频 Audio",
    category: "媒体播放",
    type: "inline",
    desc: "音频播放器，支持本地与网易云外链。",
    signature: "{% audio src %} / {% audio netease:歌曲ID %}",
    params: [
      { name: "netease", desc: "网易云歌曲 ID", default: "—" },
      { name: "type", desc: "MIME 类型", default: "audio/mp3" },
      { name: "autoplay", desc: "自动播放", default: "false" },
    ],
    example: "{% audio https://www.w3schools.com/html/horse.mp3 %}",
  },
  {
    name: "swiper",
    title: "图片轮播 Swiper",
    category: "媒体播放",
    type: "block",
    desc: "图片轮播，正文为若干 Markdown 图片。",
    signature: "{% swiper [width:宽] [effect:效果] %} ![](src) {% endswiper %}",
    params: [
      { name: "width", desc: "轮播宽度", default: "—" },
      { name: "effect", desc: "切换效果", default: "slide" },
    ],
    example: "{% swiper %}\n![](https://picsum.photos/600/340?1)\n![](https://picsum.photos/600/340?2)\n![](https://picsum.photos/600/340?3)\n{% endswiper %}",
  },
  {
    name: "copy",
    title: "复制按钮 Copy",
    category: "媒体播放",
    type: "inline",
    desc: "一键复制文本，支持 git 仓库地址快捷生成。",
    signature: "{% copy 文本 %} / {% copy git:user/repo %}",
    params: [
      { name: "git", desc: "ssh/gh/https，按仓库路径生成克隆命令", default: "—" },
      { name: "prefix", desc: "前缀标签文本", default: "—" },
    ],
    example: "{% copy npm install stellar-theme %}\n{% copy git:user/repo %}",
  },
  {
    name: "rating",
    title: "评分 Rating",
    category: "媒体播放",
    type: "inline",
    desc: "五星评分组件，对接本地 /api/rating 数据服务。",
    signature: "{% rating id [title:标题] %}",
    params: [
      { name: "id", desc: "评分对象唯一 ID", default: "default" },
      { name: "title", desc: "标题（位置参数）", default: "—" },
    ],
    example: "{% rating demo-post 给这篇文章打分 %}",
  },

  // ==================== 数据图表 ====================
  {
    name: "mermaid",
    title: "流程图 Mermaid",
    category: "数据图表",
    type: "block",
    desc: "Mermaid 图表（流程图/时序图/甘特图等），正文为原始 Mermaid 代码。",
    signature: "{% mermaid %} 图表代码 {% endmermaid %}",
    params: [],
    example: "{% mermaid %}\ngraph TB\n  A[开始] --> B{判断}\n  B -->|是| C[执行]\n  B -->|否| D[结束]\n{% endmermaid %}",
  },
  {
    name: "okr",
    title: "OKR 目标",
    category: "数据图表",
    type: "block",
    desc: "OKR 目标与关键结果展示，用 <!-- okr krN --> 分隔 KR。",
    signature: "{% okr o1 [percent:50] [status:in_track] %} 目标 <!-- okr kr1 percent:80 --> 关键结果 {% endokr %}",
    params: [
      { name: "percent", desc: "完成百分比", default: "—" },
      { name: "status", desc: "in_track/at_risk/off_track/finished/unfinished", default: "in_track" },
      { name: "color", desc: "配色", default: "—" },
    ],
    example: "{% okr o1 percent:60 status:in_track %}\n提升博客阅读体验\n<!-- okr kr1 percent:80 -->\n完成标签插件三期\n<!-- okr kr2 percent:40 -->\n上线站内搜索\n{% endokr %}",
  },
  {
    name: "chat",
    title: "聊天记录 Chat",
    category: "数据图表",
    type: "block",
    desc: "模拟聊天界面，顶部 YAML 定义参与者，正文用 <!-- chatcell --> / <!-- chattip --> 编排消息。",
    signature: "{% chat device [style:qq] [title:标题] [scene:group] [me:用户key] %} 用户YAML + 消息 {% endchat %}",
    params: [
      { name: "style", desc: "样式：qq/wechat", default: "qq" },
      { name: "scene", desc: "场景：group/single", default: "group" },
      { name: "me", desc: "靠右显示的参与者 key", default: "—" },
    ],
    example: "{% chat iphone style:qq title:技术交流群 me:b %}\nalice:\n  name: 小明\n  avatar: https://picsum.photos/80/80?1\nbob:\n  name: 小红\n  avatar: https://picsum.photos/80/80?2\n<!-- chattip 上午 10:00 -->\n<!-- chatcell user:alice md:大家好！ -->\n<!-- chatcell user:bob md:你好呀～ -->\n{% endchat %}",
  },

  // ==================== GitHub ====================
  {
    name: "ghcard",
    title: "GitHub 统计卡片",
    category: "GitHub",
    type: "inline",
    desc: "github-readme-stats 卡片，经本地 /api/gh/card 代理缓存。",
    signature: "{% ghcard user/repo [theme:主题] %} / {% ghcard user %}",
    params: [
      { name: "theme", desc: "卡片主题", default: "—" },
      { name: "show_owner", desc: "显示仓库所有者", default: "true" },
      { name: "title_color", desc: "标题颜色", default: "—" },
    ],
    example: "{% ghcard anuraghazra/github-readme-stats %}",
  },
  {
    name: "ghrepo",
    title: "GitHub 仓库卡片",
    category: "GitHub",
    type: "inline",
    desc: "静态仓库卡片（星标/Fork/语言/许可证），API 失败降级为链接。",
    signature: "{% ghrepo user/repo %}",
    params: [],
    example: "{% ghrepo vuejs/vue %}",
  },
  {
    name: "ghuser",
    title: "GitHub 用户卡片",
    category: "GitHub",
    type: "inline",
    desc: "静态用户卡片（头像/简介/粉丝/仓库数），API 失败降级为链接。",
    signature: "{% ghuser username %}",
    params: [],
    example: "{% ghuser octocat %}",
  },
];
