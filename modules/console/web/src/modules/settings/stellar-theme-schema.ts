import registry from "../../../../../../internal/stellarv2/schema.json";

export interface SettingSchema {
  type: string[];
  default?: { kind: string; value?: any };
  values?: any[];
  properties?: Record<string, SettingSchema>;
  propertyOrder?: string[];
  items?: SettingSchema;
  additionalProperties?: SettingSchema;
  normalizer?: string;
  sealed?: boolean;
  minimum?: number;
  maximum?: number;
  exclusiveMinimum?: number;
}

// 与服务端共用锁定的上游 schema，避免另存一套类型、默认值及枚举。
export function themeSettingSchema(path: string): SettingSchema {
  let node = registry.theme as SettingSchema;
  for (const key of path.split(".").slice(1)) node = node.properties![key];
  return node;
}
export function themeSettingDefault(path: string): unknown {
  let value: any = registry.rawDefaults;
  for (const key of path.split(".").slice(1)) value = value?.[key];
  return value;
}
export const openSettingSchema: SettingSchema = { type: ["string", "number", "boolean", "null", "object", "array"] };
export function schemaDefault(node: SettingSchema): any {
  if (node.default?.kind === "literal") {
    const value = node.default.value;
    const type = value === null ? "null" : Array.isArray(value) ? "array" : typeof value;
    if (node.type.includes(type)) return value;
  }
  if (node.type.includes("null")) return null;
  if (node.type.includes("object")) return {};
  if (node.type.includes("array")) return [];
  if (node.type.includes("boolean")) return false;
  if (node.type.includes("number")) return 0;
  return "";
}

const labels: Record<string, string> = {
  enabled: "启用",
  image: "图片",
  src: "图片地址",
  variant: "图片样式",
  name: "名称",
  tagline: "副标题",
  href: "链接地址",
  brand: "品牌标识",
  menu: "菜单",
  widgets: "侧栏组件",
  menu_columns: "菜单列数",
  default_state: "默认展开状态",
  title: "标题",
  icon: "图标",
  url: "地址",
  id: "标识",
  type: "类型",
  items: "项目",
  children: "子项目",
  topbar: "顶栏",
  leftbar: "左侧栏",
  rightbar: "右侧栏",
  footer: "页脚",
  sitemap: "站点地图",
  content: "内容",
  actions: "快捷操作",
  search: "显示搜索",
  ghrepo: "GitHub 仓库",
  ghuser: "GitHub 用户",
  style: "样式",
  home: "首页",
  blog_index: "文章列表",
  post: "文章页",
  topic: "专栏",
  wiki_index: "文档列表",
  wiki: "文档页",
  notebook: "笔记本",
  notebooks: "笔记本列表",
  note: "笔记页",
  author: "作者页",
  page: "独立页面",
  settings: "访客设置页",
  error: "错误页",
  path: "页面路径",
  active_menu: "当前菜单标识",
  comments: "评论",
  provider: "服务提供方",
  options: "服务参数",
  listing_nav: "列表导航",
  tabs: "导航标签",
  listing: "列表",
  per_page: "每页数量",
  sort: "排序",
  field: "排序字段",
  direction: "排序方向",
  excerpt_length: "摘要字数",
  tag_icons: "标签图标",
  about: "关于信息",
  key: "名称",
  value: "值",
  preset: "外观预设",
  color_scheme: "配色模式",
  colors: "颜色",
  primary: "主题色",
  accent: "强调色",
  link: "链接色",
  gradients: "渐变",
  primary_action: "主要按钮背景",
  search_bar: "搜索栏背景",
  typography: "字体与排版",
  font_smoothing: "字体平滑",
  font_weight: "字重映射",
  font_family: "字体",
  body: "正文",
  code: "代码",
  font_size: "字号",
  root: "基础字号",
  inline_code: "行内代码",
  code_block: "代码块",
  content_align: "正文对齐",
  shape: "形状与圆角",
  corner: "曲率",
  radius: "圆角 / 模糊半径",
  card_large: "大卡片",
  card: "卡片",
  card_small: "小卡片",
  bar: "横条",
  image_large: "大图片",
  image_small: "小图片",
  backgrounds: "背景",
  gradient: "渐变颜色",
  light: "浅色",
  dark: "深色",
  opacity: "不透明度",
  backdrop: "背景模糊",
  overlay: "遮罩颜色",
  saturation: "饱和度",
  scrollbar_width: "滚动条宽度",
  highlight_stylesheet: "代码高亮样式地址",
  partial_navigation: "局部导航",
  swiper: "轮播",
  color_scheme_switch: "配色切换按钮",
  image_optimization: "图片优化",
  lazy_loading: "图片懒加载",
  transition: "过渡效果",
  auto_aspect_ratio: "自动宽高比",
  lightbox: "图片灯箱",
  selector: "图片选择器",
  link_prefetch: "链接预加载",
  reveal: "内容入场动画",
  duration: "持续时间（毫秒）",
  interval: "间隔（毫秒）",
  distance: "移动距离",
  blur: "模糊程度",
  card_hover: "卡片悬停",
  spotlight: "聚光效果",
  tilt: "倾斜效果",
  heti: "中文排版",
  math: "数学公式",
  diagrams: "图表",
  theme: "主题",
  css: "样式地址",
  js: "脚本地址",
  css_integrity: "样式完整性校验",
  gravatar: "Gravatar 头像",
  base_url: "服务根地址",
  markdown: "Markdown 渲染",
  site_info: "站点信息",
  endpoint: "接口地址",
  rating: "评分",
  vote: "投票",
  contributors: "贡献者",
  repositories: "仓库列表",
  github_card: "GitHub 卡片",
  github: "GitHub",
  api_url: "API 地址",
  raw_url: "原始文件地址",
  gist_url: "Gist 地址",
  avatar: "头像",
  link_card: "链接卡片",
  cover: "封面",
  default_color: "默认颜色",
  border: "显示边框",
  checkbox: "复选框",
  interactive: "允许交互",
  quot: "引用",
  default: "默认",
  prefix: "前缀图标",
  suffix: "后缀图标",
  hashtag: "话题标签",
  question: "问题",
  emoji: "表情",
  default_source: "默认来源",
  sources: "来源地址",
  button: "按钮",
  mark: "标记",
  gallery: "图库",
  size: "大小",
  aspect_ratio: "宽高比",
  repo: "仓库",
  repo_id: "仓库 ID",
  category: "分类",
  category_id: "分类 ID",
  mapping: "匹配方式",
  lang: "语言",
  description: "简介",
  hidden: "隐藏作者",
  target: "打开方式",
  rel: "链接关系",
  badge: "徽章",
  html: "HTML 内容",
};
const values: Record<string, string> = {
  auto: "自动",
  light: "浅色",
  dark: "深色",
  card: "卡片",
  glass: "玻璃",
  minimal: "极简",
  flat: "平面",
  expanded: "展开",
  collapsed: "折叠",
  avatar: "头像",
  icon: "图标",
  plain: "原图",
  regular: "标准",
  compact: "紧凑",
  left: "左对齐",
  justify: "两端对齐",
  asc: "升序",
  desc: "降序",
  date: "发布日期",
  updated: "更新时间",
  title: "标题",
  none: "无",
  gradient: "渐变",
  image: "图片",
  fade: "淡入",
  slide: "滑入",
  square: "正方形",
  mix: "混合",
};
export const settingLabel = (key: string): string => (Object.hasOwn(labels, key) ? labels[key] : key);
export const settingValueLabel = (value: unknown): string =>
  value === null
    ? "继承 / 关闭（空值）"
    : typeof value === "boolean"
      ? value
        ? "开启"
        : "关闭"
      : Object.hasOwn(values, String(value))
        ? values[String(value)]
        : String(value);
