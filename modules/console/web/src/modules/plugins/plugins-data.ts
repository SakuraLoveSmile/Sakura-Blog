// Console 插件库元数据（Spec 23）。
// 数据源对照 internal/post/shortcode.go 现有实现逐一核对，纯前端静态数据。
// 插件变更频率低，与 shortcode.go 同步靠人工维护。

export type PluginCategory ="提示标注" |"布局容器" |"文艺装饰" |"媒体播放" |"数据图表" |"GitHub";
export type PluginType ="inline" |"block";

export interface PluginParam {
 name: string;
 desc: string;
 default?: string;
}

/**
 * 结构化参数表单字段（Spec 29）。
 * 供 PluginParamForm.vue 适配 SchemaForm 并拼装 shortcode：
 * - role=positional：位置参数，按字段顺序拼在标签名后
 * - role=named：命名参数，拼为 key:value（值非空才拼）
 * - role=flag：布尔开关，true 时仅拼 key（如 checked/open/download）
 * - role=content：正文，块级插件放入 {% name %}…{% endname %} 之间
 */
export interface PluginFormField {
 key: string;
 label: string;
 type:"text" |"select" |"color" |"textarea" |"flag";
 role:"positional" |"named" |"content" |"flag";
 options?: string[];
 default?: string;
 placeholder?: string;
}

/**
 * 预览能力分类（Spec 63 组件库真实预览）：
 * - static：只需 CSS + HTML，服务端渲染即完整；
 * - interactive：需要前台交互初始化（tabs/folding/swiper 等，加载 main.js / services/swiper.js）；
 * - service：需要数据 service 与本地 Fixture（rating/vote/friends/sites/albums/posters/GitHub 卡片）；
 * - external：需要 Mermaid/KaTeX 或外部 Markdown 加载器（math-mermaid-loader.js / mdrender.js + marked）。
 */
export type PluginCapability = "static" | "interactive" | "service" | "external";

/** 插件预览描述（PLUGIN_PREVIEW 表项）。 */
export interface PluginPreviewInfo {
 capability: PluginCapability;
 /** 需要加载的前台 service 资源名（映射见 preview/resources.ts 的 SERVICE_SCRIPTS；mermaid 特例映射到 math-mermaid-loader.js） */
 services?: string[];
 /** 本地 Fixture 端点名（service 类，对应 /api/v1/preview/*；gh=GitHub 服务端 Fixture） */
 fixture?: string;
 /** 需要加载 main.js 复用前台交互初始化（tabs/folding/folders/navbar） */
 mainJs?: boolean;
 /** 推荐预览高度 px（异步内容较高时用，缺省 420） */
 height?: number;
 /** 资源不可用 / 参数缺失时的降级文案 */
 fallback?: string;
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
 /** 可选：结构化参数表单 schema（Spec 29）。未定义时编辑器/插件库回退到可编辑示例文本 */
 form?: PluginFormField[];
}

export const PLUGIN_CATEGORIES: PluginCategory[] = [
"提示标注",
"布局容器",
"文艺装饰",
"媒体播放",
"数据图表",
"GitHub",
];

export const plugins: PluginMeta[] = [
 // ==================== 提示标注 ====================
 {
 name:"note",
 title:"提示框 Note",
 category:"提示标注",
 type:"inline",
 desc:"带标题与配色的提示块，用于强调说明。",
 signature:"{% note [color:颜色] [标题] 内容 %}",
 params: [
 { name:"color", desc:"配色：red/orange/yellow/green/cyan/blue/purple/gray", default:"blue" },
 { name:"标题", desc:"可选，第二个位置参数起为正文", default:"—" },
 ],
 example:"{% note color:green 提示 这是一条成功提示，支持 **Markdown** 行内语法。 %}",
 form: [
 { key:"color", label:"配色", type:"select", role:"named", options: ["red","orange","yellow","green","cyan","blue","purple","gray"], default:"blue" },
 { key:"title", label:"标题（可选）", type:"text", role:"positional", placeholder:"提示" },
 { key:"content", label:"内容", type:"textarea", role:"content", default:"这是一条提示，支持 **Markdown** 行内语法。" },
 ],
 },
 {
 name:"quot",
 title:"小标题 Quot",
 category:"提示标注",
 type:"inline",
 desc:"带前缀图标的强调小标题，可作为锚点标题。",
 signature:"{% quot [el:h2] [icon:图标] text:文本 %}",
 params: [
 { name:"el", desc:"标题标签：p/h2/h3/h4/h5/h6", default:"p" },
 { name:"icon", desc:"前缀图标 key（Iconify）", default:"hashtag" },
 { name:"text", desc:"标题文本（也可用位置参数）", default:"—" },
 ],
 example:"{% quot el:h2 text:这是一个小标题 %}",
 form: [
 { key:"el", label:"标签", type:"select", role:"named", options: ["p","h2","h3","h4","h5","h6"], default:"p" },
 { key:"icon", label:"图标 key", type:"text", role:"named", default:"hashtag" },
 { key:"text", label:"标题文本", type:"text", role:"named", default:"这是一个小标题" },
 ],
 },
 {
 name:"mark",
 title:"荧光标记 Mark",
 category:"提示标注",
 type:"inline",
 desc:"行内荧光笔高亮文本。",
 signature:"{% mark 文本 [color:颜色] %}",
 params: [{ name:"color", desc:"高亮颜色", default:"yellow" }],
 example:"这里有一段 {% mark 重点内容 color:cyan %} 需要高亮。",
 form: [
 { key:"text", label:"文本", type:"text", role:"positional", default:"重点内容" },
 { key:"color", label:"颜色", type:"select", role:"named", options: ["red","orange","yellow","green","cyan","blue","purple","gray"], default:"yellow" },
 ],
 },
 {
 name:"button",
 title:"按钮 Button",
 category:"提示标注",
 type:"inline",
 desc:"可点击的链接按钮。",
 signature:"{% button 文本 链接 [color:颜色] [size:尺寸] %}",
 params: [
 { name:"color", desc:"按钮配色", default:"blue" },
 { name:"size", desc:"尺寸：small/large", default:"—" },
 ],
 example:"{% button 访问主页 https://example.com color:purple %}",
 form: [
 { key:"text", label:"按钮文本", type:"text", role:"positional", default:"访问主页" },
 { key:"url", label:"链接", type:"text", role:"positional", default:"https://example.com" },
 { key:"color", label:"配色", type:"select", role:"named", options: ["red","orange","yellow","green","cyan","blue","purple","gray"], default:"blue" },
 { key:"size", label:"尺寸", type:"select", role:"named", options: ["","small","large"] },
 ],
 },
 {
 name:"checkbox",
 title:"复选框 Checkbox",
 category:"提示标注",
 type:"inline",
 desc:"行内复选框，可用于清单展示。",
 signature:"{% checkbox [checked] 文本 [color:颜色] %}",
 params: [
 { name:"checked", desc:"位置参数，出现即勾选", default:"false" },
 { name:"color", desc:"配色", default:"blue" },
 ],
 example:"{% checkbox checked 已完成的任务 color:green %}\n{% checkbox 待办的任务 %}",
 form: [
 { key:"text", label:"文本", type:"text", role:"positional", default:"已完成的任务" },
 { key:"checked", label:"默认勾选", type:"flag", role:"flag", default:"true" },
 { key:"color", label:"配色", type:"select", role:"named", options: ["red","orange","yellow","green","cyan","blue","purple","gray"], default:"green" },
 ],
 },
 {
 name:"radio",
 title:"单选框 Radio",
 category:"提示标注",
 type:"inline",
 desc:"行内单选框，与 checkbox 同源（symbol 参数可换勾选项符号）。",
 signature:"{% radio [checked] 文本 [color:颜色] %}",
 params: [
 { name:"checked", desc:"位置参数，出现即选中", default:"false" },
 { name:"color", desc:"配色", default:"blue" },
 ],
 example:"{% radio checked 选项一 color:blue %}\n{% radio 选项二 %}",
 form: [
 { key:"text", label:"文本", type:"text", role:"positional", default:"选项一" },
 { key:"checked", label:"默认选中", type:"flag", role:"flag", default:"true" },
 { key:"color", label:"配色", type:"select", role:"named", options: ["red","orange","yellow","green","cyan","blue","purple","gray"], default:"blue" },
 ],
 },
 {
 name:"hashtag",
 title:"话题标签 Hashtag",
 category:"提示标注",
 type:"inline",
 desc:"带 # 图标的话题标签，颜色可循环或指定。",
 signature:"{% hashtag 文本 [链接] [color:颜色] %}",
 params: [
 { name:"href", desc:"可选跳转链接（位置参数）", default:"—" },
 { name:"color", desc:"缺省时按红橙黄绿青蓝紫循环", default:"循环" },
 ],
 example:"{% hashtag 日常 %} {% hashtag 技术 color:blue %} {% hashtag 随笔 %}",
 form: [
 { key:"text", label:"文本", type:"text", role:"positional", default:"日常" },
 { key:"href", label:"链接（可选）", type:"text", role:"positional" },
 { key:"color", label:"颜色（留空循环）", type:"select", role:"named", options: ["","red","orange","yellow","green","cyan","blue","purple","gray"] },
 ],
 },
 {
 name:"emoji",
 title:"表情 Emoji",
 category:"提示标注",
 type:"inline",
 desc:"行内表情图，支持 QQ/贴吧/阿鲁/猫咪/twemoji 五种来源（完整清单见「素材速查」）。",
 signature:"{% emoji [来源] 名称 [height:高度] %}",
 params: [
 { name:"source", desc:"来源：default(QQ)/qq/tieba/aru/blobcat/twemoji，省略用 default", default:"default" },
 { name:"height", desc:"自定义高度", default:"—" },
 ],
 example:"开心 {% emoji aini %} 点赞 {% emoji twemoji 1f44d %} 滑稽 {% emoji tieba huaji %}",
 },
 {
 name:"icon",
 title:"内联图标 Icon",
 category:"提示标注",
 type:"inline",
 desc:"内联 Iconify 图标，可附文本。",
 signature:"{% icon key [color:颜色] [文本] %}",
 params: [
 { name:"key", desc:"Iconify 图标 key", default:"—" },
 { name:"color", desc:"图标颜色", default:"blue" },
 ],
 example:"{% icon example:planet color:blue %} 星球 {% icon example:chat color:red %} 点赞",
 form: [
  { key:"key", label:"图标 key", type:"text", role:"positional", default:"example:planet" },
 { key:"color", label:"颜色", type:"select", role:"named", options: ["red","orange","yellow","green","cyan","blue","purple","gray"], default:"blue" },
 { key:"text", label:"附文本（可选）", type:"text", role:"positional" },
 ],
 },

 // ==================== 布局容器 ====================
 {
 name:"folding",
 title:"折叠块 Folding",
 category:"布局容器",
 type:"block",
 desc:"可展开/收起的折叠区块，正文支持 Markdown。",
 signature:"{% folding [color:颜色] [open:true] 标题 %} 正文 {% endfolding %}",
 params: [
 { name:"color", desc:"配色", default:"—" },
 { name:"open", desc:"true 时默认展开", default:"false" },
 ],
 example:"{% folding color:cyan open:true 点击查看详情 %}\n这里是折叠内容，支持 **Markdown**。\n{% endfolding %}",
 form: [
 { key:"color", label:"配色", type:"select", role:"named", options: ["","red","orange","yellow","green","cyan","blue","purple","gray"] },
 { key:"open", label:"默认展开", type:"flag", role:"flag", default:"true" },
 { key:"title", label:"标题", type:"text", role:"positional", default:"点击查看详情" },
 { key:"content", label:"正文", type:"textarea", role:"content", default:"这里是折叠内容，支持 **Markdown**。" },
 ],
 },
 {
 name:"tabs",
 title:"选项卡 Tabs",
 category:"布局容器",
 type:"block",
 desc:"多标签切换容器，用 <!-- tab 标题 --> 分隔各页。",
 signature:"{% tabs [active:序号] %} <!-- tab 标题 --> 内容 {% endtabs %}",
 params: [
 { name:"active", desc:"默认激活的标签序号（从 1 起）", default:"1" },
 { name:"align", desc:"标签对齐方式", default:"—" },
 ],
 example:"{% tabs %}\n<!-- tab 标签一 -->\n第一页内容。\n<!-- tab 标签二 -->\n第二页内容。\n{% endtabs %}",
 },
 {
 name:"timeline",
 title:"时间轴 Timeline",
 category:"布局容器",
 type:"block",
 desc:"垂直时间轴，用 <!-- node 标题 --> 分隔节点。",
 signature:"{% timeline %} <!-- node 时间 --> 内容 {% endtimeline %}",
 params: [],
 example:"{% timeline %}\n<!-- node 2024-01 -->\n项目启动。\n<!-- node 2024-06 -->\n首个版本发布。\n{% endtimeline %}",
 },
 {
 name:"gallery",
 title:"图片墙 Gallery",
 category:"布局容器",
 type:"block",
 desc:"网格/瀑布流图片墙，正文每行一个 Markdown 图片。",
 signature:"{% gallery [layout:grid] [size:mix] [ratio:origin] %} ![alt](src) {% endgallery %}",
 params: [
 { name:"layout", desc:"布局：grid/flow", default:"grid" },
 { name:"size", desc:"尺寸模式", default:"mix" },
 { name:"ratio", desc:"宽高比", default:"origin" },
 ],
 example:"{% gallery %}\n![首页界面](/assets/img/preview/gallery-1.jpg)\n![文章界面](/assets/img/preview/gallery-2.jpg)\n![技术分享](/assets/img/preview/gallery-3.jpg)\n{% endgallery %}",
 form: [
  { key:"layout", label:"布局", type:"select", role:"named", options: ["grid","flow"], default:"grid" },
  { key:"size", label:"尺寸模式", type:"text", role:"named", default:"mix" },
  { key:"ratio", label:"宽高比", type:"text", role:"named", default:"origin" },
  { key:"content", label:"图片（每行一个 Markdown 图片）", type:"textarea", role:"content", default:"![首页界面](/assets/img/preview/gallery-1.jpg)\n![文章界面](/assets/img/preview/gallery-2.jpg)\n![技术分享](/assets/img/preview/gallery-3.jpg)" },
 ],
 },
 {
 name:"folders",
 title:"折叠组 Folders",
 category:"布局容器",
 type:"block",
 desc:"多个折叠项的分组，用 <!-- folder 标题 --> 分隔。",
 signature:"{% folders [color:颜色] %} <!-- folder 标题 --> 内容 {% endfolders %}",
 params: [{ name:"color", desc:"配色", default:"—" }],
 example:"{% folders color:blue %}\n<!-- folder 第一章 -->\n内容一。\n<!-- folder 第二章 -->\n内容二。\n{% endfolders %}",
 },
 {
 name:"grid",
 title:"网格 Grid",
 category:"布局容器",
 type:"block",
 desc:"响应式网格布局，用 <!-- cell --> 分隔单元格。",
 signature:"{% grid [w:列宽] [c:列数] [gap:间距] [br:圆角] %} <!-- cell --> 内容 {% endgrid %}",
 params: [
 { name:"w", desc:"最小列宽（如 240px），与 c 二选一", default:"240px" },
 { name:"c", desc:"固定列数", default:"—" },
 { name:"gap", desc:"单元格间距", default:"—" },
 { name:"br", desc:"单元格圆角", default:"—" },
 ],
 example:"{% grid bg:card c:2 gap:12px %}\n<!-- cell -->\n### 博客技术\n这里写博客相关的技术分享、踩坑记录与教程笔记，卡片样式一眼可辨。\n<!-- cell -->\n### 生活随笔\n记录日常生活的片段、旅行见闻与读书感想。\n{% endgrid %}",
 },
 {
 name:"blockquote",
 title:"增强引用 Blockquote",
 category:"布局容器",
 type:"block",
 desc:"带引号图标的增强引用块，支持首行缩进。",
 signature:"{% blockquote [indent:true/false] %} 引用正文 {% endblockquote %}",
 params: [
 { name:"indent", desc:"段落首行缩进 2 字符", default:"false" },
 ],
 example:"{% blockquote indent:true %}\n引用正文，支持多段落 Markdown。\n{% endblockquote %}",
 },
 {
 name:"banner",
 title:"横幅 Banner",
 category:"布局容器",
 type:"block",
 desc:"横幅容器：背景图 + 头像 + 标题/副标题 + 正文，可整块链接跳转。",
 signature:"{% banner [title:标题] [subtitle:副标题] [bg:背景图] [avatar:头像] [link:链接] %} 正文 {% endbanner %}",
 params: [
 { name:"title", desc:"标题文本", default:"—" },
 { name:"subtitle", desc:"副标题文本", default:"—" },
 { name:"bg", desc:"背景图片 URL", default:"—" },
 { name:"avatar", desc:"头像图片 URL", default:"—" },
 { name:"link", desc:"点击跳转链接（整块包裹）", default:"—" },
 ],
 example:"{% banner title:我的博客 subtitle:记录技术成长 bg:/assets/img/preview/banner-bg.jpg avatar:/assets/img/preview/avatar-blog.svg %}\n欢迎来到我的小站。\n{% endbanner %}",
 form: [
  { key:"title", label:"标题", type:"text", role:"named", default:"我的博客" },
  { key:"subtitle", label:"副标题", type:"text", role:"named", default:"记录技术成长" },
  { key:"bg", label:"背景图 URL", type:"text", role:"named", default:"/assets/img/preview/banner-bg.jpg" },
  { key:"avatar", label:"头像 URL（可选）", type:"text", role:"named", default:"/assets/img/preview/avatar-blog.svg" },
  { key:"link", label:"跳转链接（可选）", type:"text", role:"named" },
  { key:"content", label:"正文", type:"textarea", role:"content", default:"欢迎来到我的小站。" },
 ],
 },
 {
 name:"navbar",
 title:"文章内导航 Navbar",
 category:"布局容器",
 type:"block",
 desc:"文章内导航栏，正文每行一个 [文本](链接)，active 项高亮。",
 signature:"{% navbar active:首页 %}\n[首页](/) [关于](/about) [作品](/works)\n{% endnavbar %}",
 params: [
 { name:"active", desc:"当前高亮项的链接文本", default:"—" },
 ],
 example:"{% navbar active:首页 %}\n[首页](https://example.com/)\n[关于](https://example.com/about)\n[作品](https://example.com/works)\n{% endnavbar %}",
 form: [
 { key:"active", label:"高亮项文本", type:"text", role:"named", default:"首页" },
 { key:"content", label:"导航链接（每行 [文本](链接)）", type:"textarea", role:"content", default:"[首页](https://example.com/)\n[关于](https://example.com/about)\n[作品](https://example.com/works)" },
 ],
 },
 {
 name:"frame",
 title:"设备框架 Frame",
 category:"布局容器",
 type:"block",
 desc:"把图片/视频嵌入 iPhone/iPad/MacBook 纯 CSS 设备外壳。",
 signature:"{% frame device:iphone [width:最大宽度] %} ![截图](url) {% endframe %}",
 params: [
 { name:"device", desc:"设备：iphone/ipad/macbook", default:"iphone" },
 { name:"width", desc:"容器最大宽度（如 320px）", default:"—" },
 ],
 example:"{% frame device:iphone width:320px %}\n![手机界面截图](/assets/img/preview/frame-shot.jpg)\n{% endframe %}",
 form: [
  { key:"device", label:"设备", type:"select", role:"named", options: ["iphone","ipad","macbook"], default:"iphone" },
  { key:"width", label:"最大宽度（可选）", type:"text", role:"named", default:"320px" },
  { key:"content", label:"媒体内容（Markdown 图片/视频）", type:"textarea", role:"content", default:"![手机界面截图](/assets/img/preview/frame-shot.jpg)" },
 ],
 },
 {
 name:"box",
 title:"卡片盒 Box",
 category:"布局容器",
 type:"block",
 desc:"带标题的卡片容器，正文支持 Markdown。",
 signature:"{% box [color:颜色] [child:子色] 标题 %} 正文 {% endbox %}",
 params: [
 { name:"color", desc:"外框配色", default:"—" },
 { name:"child", desc:"子元素配色", default:"—" },
 ],
 example:"{% box color:green 卡片标题 %}\n卡片正文，支持 **Markdown**。\n{% endbox %}",
 form: [
 { key:"color", label:"外框配色", type:"select", role:"named", options: ["","red","orange","yellow","green","cyan","blue","purple","gray"] },
 { key:"child", label:"子元素配色", type:"select", role:"named", options: ["","red","orange","yellow","green","cyan","blue","purple","gray"] },
 { key:"title", label:"标题", type:"text", role:"positional", default:"卡片标题" },
 { key:"content", label:"正文", type:"textarea", role:"content", default:"卡片正文，支持 **Markdown**。" },
 ],
 },
 {
 name:"about",
 title:"关于页 About",
 category:"布局容器",
 type:"block",
 desc:"关于页卡片：头像 + 正文（Markdown），可选返回箭头与头像边框。",
 signature:"{% about [avatar:头像] [border:边框色] [height:头像高] [back:true] %} 正文 {% endabout %}",
 params: [
 { name:"avatar", desc:"头像图片 URL", default:"—" },
 { name:"border", desc:"头像边框颜色", default:"—" },
 { name:"height", desc:"头像高度", default:"—" },
 { name:"back", desc:"非空时显示返回首页箭头", default:"—" },
 ],
 example:"{% about avatar:/assets/img/preview/avatar-blog.svg %}\n这里写关于页正文，支持 **Markdown**。\n{% endabout %}",
 },
 {
 name:"toc",
 title:"文章目录 TOC",
 category:"布局容器",
 type:"inline",
 desc:"文章内目录组件（wiki: 项目树暂未启用）；display:mobile 仅移动端显示。",
 signature:"{% toc [title] [display:mobile] %}",
 params: [
 { name:"title", desc:"目录标题（位置参数）", default:"TOC" },
 { name:"display", desc:"mobile=仅移动端显示", default:"—" },
 ],
 example:"{% toc 本文目录 %}",
 },
 {
 name:"mbti",
 title:"人格卡片 MBTI",
 category:"文艺装饰",
 type:"inline",
 desc:"MBTI 人格特征卡片：类型名 + 五维百分比进度条 + 可选档案链接（1.38 新增）。",
 signature:"{% mbti 类型 百分比-百分比-百分比-百分比-百分比 [url:档案链接] [image:横幅图片] %}",
 params: [
 { name:"type", desc:"人格类型代码（如 INTJ-T，大小写均可）", default:"—" },
 { name:"scores", desc:"五个百分比，- 分隔，顺序：精力-认知-决策-生活-身份", default:"—" },
 { name:"url", desc:"16Personalities 档案链接（可选，url:https://...）", default:"—" },
 { name:"image", desc:"标题上方横幅图片地址（可选）", default:"—" },
 ],
 example:"{% mbti INTJ-T 93-72-78-83-51 url:https://www.16personalities.com/ch/档案/9d09a4297ffa2 %}",
 form: [
  { key:"type", label:"人格类型", type:"text", role:"positional", default:"INTJ-T", placeholder:"INTJ-T" },
  { key:"scores", label:"五维百分比", type:"text", role:"positional", default:"93-72-78-83-51", placeholder:"93-72-78-83-51" },
  { key:"url", label:"档案链接（可选）", type:"text", role:"named", placeholder:"https://www.16personalities.com/..." },
  { key:"image", label:"横幅图片（可选）", type:"text", role:"named", placeholder:"https://..." },
 ],
 },
 {
 name:"poetry",
 title:"诗词 Poetry",
 category:"文艺装饰",
 type:"block",
 desc:"居中排版的诗词区块，含标题/作者/日期/页脚。",
 signature:"{% poetry [标题] [author:作者] [date:日期] [footer:页脚] %} 正文 {% endpoetry %}",
 params: [
 { name:"author", desc:"作者", default:"—" },
 { name:"date", desc:"日期", default:"—" },
 { name:"footer", desc:"页脚注", default:"—" },
 ],
 example:"{% poetry 静夜思 author:李白 %}\n床前明月光，\n疑是地上霜。\n举头望明月，\n低头思故乡。\n{% endpoetry %}",
 },
 {
 name:"paper",
 title:"纸张 Paper",
 category:"文艺装饰",
 type:"block",
 desc:"信纸排版卡片，正文可用 <!-- paragraph/section/line --> 标记分段。",
 signature:"{% paper [style:underline] [title:标题] [author:作者] [date:日期] [footer:页脚] %} 正文 {% endpaper %}",
 params: [
 { name:"style", desc:"标题下划线装饰：underline", default:"—" },
 { name:"title", desc:"标题", default:"—" },
 { name:"author", desc:"作者（页脚区）", default:"—" },
 { name:"date", desc:"日期（页脚区）", default:"—" },
 { name:"footer", desc:"页脚注", default:"—" },
 ],
 example:"{% paper title:一封家书 author:张三 %}\n<!-- paragraph -->\n见字如面。\n<!-- line right -->\n勿念。\n{% endpaper %}",
 },
 {
 name:"reel",
 title:"卷轴 Reel",
 category:"文艺装饰",
 type:"block",
 desc:"胶片卷轴式卡片，正文按行排列，含标题/作者/日期/页脚。",
 signature:"{% reel [标题] [author:作者] [date:日期] [footer:页脚] %} 正文 {% endreel %}",
 params: [
 { name:"author", desc:"作者", default:"—" },
 { name:"date", desc:"日期", default:"—" },
 { name:"footer", desc:"页脚注", default:"—" },
 ],
 example:"{% reel 观影手记 author:阿甘 date:2026-07 %}\n第 1 帧：开场长镜头。\n第 2 帧：主角登场。\n{% endreel %}",
 },
 {
 name:"psw",
 title:"密码遮罩 Psw",
 category:"文艺装饰",
 type:"inline",
 desc:"行内密码遮罩，默认模糊隐藏，悬停显示明文。",
 signature:"{% psw 隐藏文本 %}",
 params: [],
 example:"这里是 {% psw 密码内容 %} 遮罩文本。",
 },
 {
 name:"emp",
 title:"着重强调 Emp",
 category:"文艺装饰",
 type:"inline",
 desc:"行内着重强调：下划线 + 粗体。",
 signature:"{% emp 着重文本 %}",
 params: [],
 example:"这段话里 {% emp 重点强调 %} 一下。",
 },
 {
 name:"wavy",
 title:"波浪线 Wavy",
 category:"文艺装饰",
 type:"inline",
 desc:"行内波浪线装饰文本。",
 signature:"{% wavy 波浪文本 %}",
 params: [],
 example:"这里用 {% wavy 波浪线 %} 标注。",
 },
 {
 name:"blur",
 title:"模糊 Blur",
 category:"文艺装饰",
 type:"inline",
 desc:"行内模糊文本，悬停变清晰。",
 signature:"{% blur 模糊文本 %}",
 params: [],
 example:"剧透警告：{% blur 结局大反转 %}。",
 },
 {
 name:"sub",
 title:"下角标 Sub",
 category:"文艺装饰",
 type:"inline",
 desc:"行内下标文本，可选彩色。",
 signature:"{% sub [color:颜色] 下标文本 %}",
 params: [{ name:"color", desc:"配色：red/orange/yellow/green/cyan/blue/purple/gray", default:"—" }],
 example:"化学式 H{% sub 2 %}O 与上标 m{% sup 2 %}。",
 },
 {
 name:"sup",
 title:"上角标 Sup",
 category:"文艺装饰",
 type:"inline",
 desc:"行内上标文本，可选彩色（与 sub 配对）。",
 signature:"{% sup [color:颜色] 上标文本 %}",
 params: [{ name:"color", desc:"配色：red/orange/yellow/green/cyan/blue/purple/gray", default:"—" }],
 example:"面积 m{% sup 2 %}。",
 },
 {
 name:"u",
 title:"下划线 U",
 category:"文艺装饰",
 type:"inline",
 desc:"行内下划线文本。",
 signature:"{% u 文本 %}",
 params: [],
 example:"这里需要 {% u 下划线 %} 标注。",
 },
 {
 name:"del",
 title:"删除线 Del",
 category:"文艺装饰",
 type:"inline",
 desc:"行内删除线文本。",
 signature:"{% del 文本 %}",
 params: [],
 example:"原价 {% del 999 元 %}，现价 599 元。",
 },
 {
 name:"kbd",
 title:"键盘键 Kbd",
 category:"文艺装饰",
 type:"inline",
 desc:"行内键盘按键样式。",
 signature:"{% kbd 按键 %}",
 params: [],
 example:"按下 {% kbd Ctrl %} + {% kbd C %} 复制。",
 },

 // ==================== 媒体播放 ====================
 {
 name:"image",
 title:"增强图片 Image",
 category:"媒体播放",
 type:"inline",
 desc:"带尺寸/背景/下载/比例的增强图片。",
 signature:"{% image src [alt] [width:宽] [bg:背景] [download:true] [ratio:比例] %}",
 params: [
 { name:"width", desc:"宽度（如 300px / 50%）", default:"100%" },
 { name:"bg", desc:"背景色", default:"—" },
 { name:"download", desc:"true 显示下载按钮", default:"false" },
 { name:"ratio", desc:"宽高比（如 16/9）", default:"—" },
 ],
 example:"{% image /assets/img/preview/image-demo.jpg 博客界面截图 width:80% download:true %}",
 form: [
  { key:"src", label:"图片地址", type:"text", role:"positional", default:"/assets/img/preview/image-demo.jpg" },
 { key:"alt", label:"替代文本", type:"text", role:"positional", default:"风景图" },
 { key:"width", label:"宽度", type:"text", role:"named", default:"80%" },
 { key:"bg", label:"背景色", type:"text", role:"named" },
 { key:"download", label:"显示下载按钮", type:"flag", role:"flag" },
 { key:"ratio", label:"宽高比（如 16/9）", type:"text", role:"named" },
 ],
 },
 {
 name:"link",
 title:"链接卡片 Link",
 category:"媒体播放",
 type:"inline",
 desc:"链接卡片，带描述时为上下结构(rich)，否则左右结构(plain)。",
 signature:"{% link url [标题] [desc:描述] [icon:图标URL] %}",
 params: [
 { name:"desc", desc:"描述，提供则用 rich 布局", default:"—" },
 { name:"icon", desc:"图标/缩略图 URL", default:"—" },
 ],
 example:"{% link https://example.com 示例站点 desc:一个示例链接卡片 %}",
 form: [
 { key:"url", label:"链接", type:"text", role:"positional", default:"https://example.com" },
 { key:"title", label:"标题", type:"text", role:"positional", default:"示例站点" },
 { key:"desc", label:"描述（提供则用 rich 布局）", type:"text", role:"named", default:"一个示例链接卡片" },
 { key:"icon", label:"图标/缩略图 URL", type:"text", role:"named" },
 ],
 },
 {
 name:"video",
 title:"视频 Video",
 category:"媒体播放",
 type:"inline",
 desc:"视频播放器，支持本地/Bilibili/YouTube。",
 signature:"{% video src %} / {% video bilibili:BV号 %} / {% video youtube:ID %}",
 params: [
 { name:"bilibili", desc:"B 站 BV 号", default:"—" },
 { name:"youtube", desc:"YouTube 视频 ID", default:"—" },
 { name:"ratio", desc:"宽高比", default:"16/9" },
 { name:"width", desc:"最大宽度", default:"100%" },
 { name:"autoplay", desc:"自动播放", default:"false" },
 ],
 example:"{% video /assets/media/preview/sample-video.mp4 %}",
 form: [
  { key:"src", label:"视频地址", type:"text", role:"positional", default:"/assets/media/preview/sample-video.mp4" },
 { key:"bilibili", label:"B 站 BV 号（可选）", type:"text", role:"named" },
 { key:"ratio", label:"宽高比", type:"text", role:"named", default:"16/9" },
 { key:"width", label:"最大宽度", type:"text", role:"named", default:"100%" },
 { key:"autoplay", label:"自动播放", type:"flag", role:"flag" },
 ],
 },
 {
 name:"audio",
 title:"音频 Audio",
 category:"媒体播放",
 type:"inline",
 desc:"音频播放器，支持本地与网易云外链。",
 signature:"{% audio src %} / {% audio netease:歌曲ID %}",
 params: [
 { name:"netease", desc:"网易云歌曲 ID", default:"—" },
 { name:"type", desc:"MIME 类型", default:"audio/mp3" },
 { name:"autoplay", desc:"自动播放", default:"false" },
 ],
 example:"{% audio /assets/media/preview/sample-audio.mp3 %}",
 form: [
  { key:"src", label:"音频地址", type:"text", role:"positional", default:"/assets/media/preview/sample-audio.mp3" },
 { key:"netease", label:"网易云歌曲 ID（可选）", type:"text", role:"named" },
 { key:"autoplay", label:"自动播放", type:"flag", role:"flag" },
 ],
 },
 {
 name:"swiper",
 title:"图片轮播 Swiper",
 category:"媒体播放",
 type:"block",
 desc:"图片轮播，正文为若干 Markdown 图片；width 控制容器宽度语义。",
 signature:"{% swiper [width:min/max] [effect:slide/fade/coverflow/cards] %} ![](src) {% endswiper %}",
 params: [
 { name:"width", desc:"min→窄轮播（slide 25% 宽）/max→容器限宽 800px", default:"—" },
 { name:"effect", desc:"slide/fade/coverflow/cards（后两者需 JS 参数透传）", default:"slide" },
 ],
 example:"{% swiper %}\n![](/assets/img/preview/swiper-1.jpg)\n![](/assets/img/preview/swiper-2.jpg)\n![](/assets/img/preview/swiper-3.jpg)\n{% endswiper %}",
 form: [
  { key:"width", label:"宽度语义", type:"select", role:"named", options: ["","min","max"], default:"" },
  { key:"effect", label:"切换效果", type:"select", role:"named", options: ["slide","fade","coverflow","cards"], default:"slide" },
  { key:"content", label:"轮播图片（每行一张 Markdown 图片）", type:"textarea", role:"content", default:"![](/assets/img/preview/swiper-1.jpg)\n![](/assets/img/preview/swiper-2.jpg)" },
 ],
 },
 {
 name:"albums",
 title:"专辑卡片 Albums",
 category:"媒体播放",
 type:"inline",
 desc:"方形网格专辑卡片，支持 repo/api 数据服务与 size 尺寸档位。",
 signature:"{% albums [repo:owner/repo] [api:数据URL] [size:s/m/l/xl/mix] %}",
 params: [
 { name:"repo", desc:"GitHub 仓库，自动拼 raw data.json 地址", default:"—" },
 { name:"api", desc:"数据服务 URL（friends 兼容格式）", default:"—" },
 { name:"size", desc:"卡片尺寸：s/m/l/xl/mix", default:"s" },
 ],
 example:"{% albums api:https://example.com/data.json size:m %}",
 },
 {
 name:"posters",
 title:"海报卡片 Posters",
 category:"媒体播放",
 type:"inline",
 desc:"竖版海报网格卡片，支持 repo/api 数据服务与 size 尺寸档位。",
 signature:"{% posters [repo:owner/repo] [api:数据URL] [size:xs/s/m/l/xl/mix] %}",
 params: [
 { name:"repo", desc:"GitHub 仓库，自动拼 raw data.json 地址", default:"—" },
 { name:"api", desc:"数据服务 URL（friends 兼容格式）", default:"—" },
 { name:"size", desc:"卡片尺寸：xs/s/m/l/xl/mix", default:"xs" },
 ],
 example:"{% posters api:https://example.com/posters.json size:s %}",
 },
 {
 name:"copy",
 title:"复制按钮 Copy",
 category:"媒体播放",
 type:"inline",
 desc:"一键复制文本，支持 git 仓库地址快捷生成。",
 signature:"{% copy 文本 %} / {% copy git:user/repo %}",
 params: [
 { name:"git", desc:"ssh/gh/https，按仓库路径生成克隆命令", default:"—" },
 { name:"prefix", desc:"前缀标签文本", default:"—" },
 ],
 example:"{% copy npm install stellar-theme %}\n{% copy git:user/repo %}",
 form: [
 { key:"text", label:"复制文本", type:"text", role:"positional", default:"npm install stellar-theme" },
 { key:"git", label:"git 仓库（user/repo，可选）", type:"text", role:"named" },
 ],
 },
 {
 name:"rating",
 title:"评分 Rating",
 category:"媒体播放",
 type:"inline",
 desc:"五星评分组件，对接本地 /api/rating 数据服务。",
 signature:"{% rating id [title:标题] %}",
 params: [
 { name:"id", desc:"评分对象唯一 ID", default:"default" },
 { name:"title", desc:"标题（位置参数）", default:"—" },
 ],
 example:"{% rating demo-post 给这篇文章打分 %}",
 form: [
 { key:"id", label:"评分对象 ID", type:"text", role:"positional", default:"demo-post" },
 { key:"title", label:"标题", type:"text", role:"positional", default:"给这篇文章打分" },
 ],
 },

 {
 name:"vote",
 title:"投票 Vote",
 category:"媒体播放",
 type:"inline",
 desc:"赞/踩投票组件，对接本地 /api/vote 数据服务。",
 signature:"{% vote 标题 [id:唯一ID] [yes:图标key] [no:图标key] %}",
 params: [
 { name:"id", desc:"投票对象唯一 ID", default:"default" },
 { name:"yes", desc:"赞成按钮图标 key", default:"ri:arrow-up-line" },
 { name:"no", desc:"反对按钮图标 key", default:"ri:arrow-down-line" },
 ],
 example:"{% vote 这篇文章有帮助吗 id:demo-post %}",
 },
 {
 name:"friends",
 title:"友链卡片 Friends",
 category:"媒体播放",
 type:"inline",
 desc:"友链网格卡片，缺省读本站友链聚合 /api/friends，可用 api 指定外部数据。",
 signature:"{% friends [api:数据URL] %}",
 params: [{ name:"api", desc:"数据服务 URL（friends 兼容格式）", default:"/api/friends" }],
 example:"{% friends %}",
 },
 {
 name:"sites",
 title:"站点卡片 Sites",
 category:"媒体播放",
 type:"inline",
 desc:"站点推荐网格，外部 API 驱动（api 参数必填）。",
 signature:"{% sites api:数据URL %}",
 params: [{ name:"api", desc:"数据服务 URL（必填）", default:"—" }],
 example:"{% sites api:https://example.com/sites.json %}",
 },
 {
 name:"md",
 title:"外部 Markdown Md",
 category:"媒体播放",
 type:"inline",
 desc:"拉取外部 Markdown 文件并渲染（客户端 ds-mdrender 服务）。",
 signature:"{% md src %}",
 params: [{ name:"src", desc:"Markdown 文件 URL", default:"—" }],
 example:"{% md /assets/md/preview/example.md %}",
 },

 // ==================== 数据图表 ====================
 {
 name:"mermaid",
 title:"流程图 Mermaid",
 category:"数据图表",
 type:"block",
 desc:"Mermaid 图表（流程图/时序图/甘特图等），正文为原始 Mermaid 代码。",
 signature:"{% mermaid %} 图表代码 {% endmermaid %}",
 params: [],
 example:"{% mermaid %}\ngraph TB\n A[开始] --> B{判断}\n B -->|是| C[执行]\n B -->|否| D[结束]\n{% endmermaid %}",
 },
 {
 name:"okr",
 title:"OKR 目标",
 category:"数据图表",
 type:"block",
 desc:"OKR 目标与关键结果展示，用 <!-- okr krN --> 分隔 KR。",
 signature:"{% okr o1 [percent:50] [status:in_track] %} 目标 <!-- okr kr1 percent:80 --> 关键结果 {% endokr %}",
 params: [
 { name:"percent", desc:"完成百分比", default:"—" },
 { name:"status", desc:"in_track/at_risk/off_track/finished/unfinished", default:"in_track" },
 { name:"color", desc:"配色", default:"—" },
 ],
 example:"{% okr o1 percent:60 status:in_track %}\n提升博客阅读体验\n<!-- okr kr1 percent:80 -->\n完成标签插件三期\n<!-- okr kr2 percent:40 -->\n上线站内搜索\n{% endokr %}",
 },
 {
 name:"chat",
 title:"聊天记录 Chat",
 category:"数据图表",
 type:"block",
 desc:"模拟聊天界面，顶部 YAML 定义参与者，正文用 <!-- chatcell --> / <!-- chattip --> 编排消息；scene:group 全员左对齐。",
 signature:"{% chat device [style:qq] [title:标题] [scene:group/single] [me:用户key] %} 用户YAML + 消息 {% endchat %}",
 params: [
 { name:"style", desc:"样式：qq/wechat", default:"qq" },
 { name:"scene", desc:"场景：group（全员左对齐）/single（me 靠右）", default:"group" },
 { name:"me", desc:"single 模式下靠右显示的参与者 key", default:"—" },
 ],
 example:"{% chat iphone style:qq title:技术交流群 me:b %}\nalice:\n  name: 小明\n  avatar: /assets/img/preview/avatar-xiaoming.svg\nbob:\n  name: 小红\n  avatar: /assets/img/preview/avatar-xiaohong.svg\n<!-- chattip 上午 10:00 -->\n<!-- chatcell user:alice md:大家好！ -->\n<!-- chatcell user:bob md:你好呀～ -->\n{% endchat %}",
 },

 // ==================== GitHub ====================
 {
 name:"ghcard",
 title:"GitHub 统计卡片",
 category:"GitHub",
 type:"inline",
 desc:"github-readme-stats 卡片，经本地 /api/gh/card 代理缓存。",
 signature:"{% ghcard user/repo [theme:主题] %} / {% ghcard user %}",
 params: [
 { name:"theme", desc:"卡片主题", default:"—" },
 { name:"show_owner", desc:"显示仓库所有者", default:"true" },
 { name:"title_color", desc:"标题颜色", default:"—" },
 ],
 example:"{% ghcard anuraghazra/github-readme-stats %}",
 form: [
 { key:"repo", label:"user/repo 或 user", type:"text", role:"positional", default:"anuraghazra/github-readme-stats" },
 { key:"theme", label:"主题", type:"text", role:"named" },
 { key:"show_owner", label:"显示仓库所有者", type:"flag", role:"flag", default:"true" },
 ],
 },
 {
 name:"ghrepo",
 title:"GitHub 仓库卡片",
 category:"GitHub",
 type:"inline",
 desc:"静态仓库卡片（星标/Fork/语言/许可证），API 失败降级为链接。",
 signature:"{% ghrepo user/repo %}",
 params: [],
 example:"{% ghrepo vuejs/vue %}",
 },
 {
 name:"ghuser",
 title:"GitHub 用户卡片",
 category:"GitHub",
 type:"inline",
 desc:"静态用户卡片（头像/简介/粉丝/仓库数），API 失败降级为链接。",
 signature:"{% ghuser username %}",
 params: [],
 example:"{% ghuser octocat %}",
 },
];

// ==================== 预览能力清单（Spec 63） ====================
// 每个插件一条预览描述，供共享预览运行时（preview/）按能力加载资源与 Fixture。
// 静态插件的预览描述由 previewInfoFor 兜底（capability=static），无需逐条登记。

const STATIC_PLUGIN_PREVIEW: PluginPreviewInfo = {
 capability: "static",
};

export const PLUGIN_PREVIEW: Record<string, PluginPreviewInfo> = {
 // ---- 布局容器：交互初始化 ----
 folding: {
  capability: "interactive",
  mainJs: true,
  height: 180,
  fallback: "折叠块为原生 details 结构，无需脚本即可展开/收起。",
 },
 tabs: {
  capability: "interactive",
  mainJs: true,
  height: 220,
  fallback: "Tab 切换依赖前台 main.js，加载失败时仅显示第一个标签页。",
 },
 gallery: {
  capability: "interactive",
  height: 320,
  fallback: "图片墙展示本地示例图；灯箱（fancybox）资源不可用时点击无放大效果。",
 },
 folders: {
  capability: "interactive",
  mainJs: true,
  height: 260,
  fallback: "折叠组为原生 details 结构，无需脚本即可展开/收起。",
 },
 navbar: {
  capability: "interactive",
  mainJs: true,
  height: 160,
  fallback: "导航栏高亮依赖前台脚本，加载失败时仅显示静态链接。",
 },
 grid: {
  capability: "static",
  height: 240,
  fallback: "信息卡为原生网格结构，列宽/间距/圆角由样式直接呈现。",
 },
 // ---- 媒体播放：Swiper ----
 swiper: {
  capability: "interactive",
  services: ["swiper"],
  height: 320,
  fallback: "Swiper 资源加载失败时轮播不可滑动，图片仍按列表展示。",
 },
 // ---- 数据 service + 本地 Fixture ----
 albums: {
  capability: "service",
  services: ["friends"],
  fixture: "friends",
  height: 300,
  fallback: "专辑数据依赖 ds-friends 服务与本地 Fixture，加载失败时显示空态。",
 },
 posters: {
  capability: "service",
  services: ["friends"],
  fixture: "friends",
  height: 300,
  fallback: "海报数据依赖 ds-friends 服务与本地 Fixture，加载失败时显示空态。",
 },
 rating: {
  capability: "service",
  services: ["rating"],
  fixture: "rating",
  height: 180,
  fallback: "评分数据为预览 Fixture，不写入真实统计；加载失败时显示 0 分。",
 },
 vote: {
  capability: "service",
  services: ["vote"],
  fixture: "vote",
  height: 180,
  fallback: "投票数据为预览 Fixture，不写入真实统计；加载失败时显示 0 票。",
 },
 friends: {
  capability: "service",
  services: ["friends"],
  fixture: "friends",
  height: 320,
  fallback: "友链数据为本地 Fixture；加载失败时显示空态。",
 },
 sites: {
  capability: "service",
  services: ["sites"],
  fixture: "sites",
  height: 320,
  fallback: "站点数据为本地 Fixture；加载失败时显示空态。",
 },
 // ---- GitHub 卡片：服务端确定性 Fixture ----
 ghcard: {
  capability: "service",
  fixture: "gh",
  height: 200,
  fallback: "GitHub 卡片使用服务端确定性 Fixture，不发公网请求。",
 },
 ghrepo: {
  capability: "service",
  fixture: "gh",
  height: 200,
  fallback: "仓库卡片使用服务端确定性 Fixture，不发公网请求。",
 },
 ghuser: {
  capability: "service",
  fixture: "gh",
  height: 200,
  fallback: "用户卡片使用服务端确定性 Fixture，不发公网请求。",
 },
 // ---- 数据图表 ----
 chat: {
  capability: "static",
  height: 360,
  fallback: "聊天记录为服务端渲染结构，头像为本地素材。",
 },
 // ---- 外部资源：Mermaid / 外部 Markdown ----
 mermaid: {
  capability: "external",
  services: ["mermaid"],
  height: 320,
  fallback: "Mermaid 为本地 vendor（11.16.1），加载失败时保留图表源码并显示本提示。",
 },
 md: {
  capability: "external",
  services: ["mdrender"],
  fixture: "md",
  height: 320,
  fallback: "外部 Markdown 为本地 Fixture；marked 为本地 vendor，加载失败时显示本提示。",
 },
 // ---- 布局容器 / 文艺装饰 / 提示标注：数据上下文说明 ----
 timeline: {
  capability: "static",
  height: 260,
  fallback: "时间轴内容来自示例正文，无需外部数据。",
 },
 toc: {
  capability: "static",
  height: 160,
  fallback: "预览展示 TOC 容器骨架；真实文章页由前台脚本填充目录项。",
 },
 emoji: {
  capability: "static",
  height: 120,
  fallback: "表情素材为本地资源；未收录的表情名显示占位。",
 },
 image: {
  capability: "static",
  height: 300,
  fallback: "示例图片为本地资源；手动填入的外链图片在无网络时显示占位。",
 },
 video: {
  capability: "static",
  height: 320,
  fallback: "示例视频为本地资源（体积受控）。",
 },
 audio: {
  capability: "static",
  height: 120,
  fallback: "示例音频为本地资源（体积受控）。",
 },
};

/** 取插件预览描述：未登记项按静态插件处理（只注入 CSS + HTML）。 */
export function previewInfoFor(p: Pick<PluginMeta, "name">): PluginPreviewInfo {
 return PLUGIN_PREVIEW[p.name] || STATIC_PLUGIN_PREVIEW;
}
