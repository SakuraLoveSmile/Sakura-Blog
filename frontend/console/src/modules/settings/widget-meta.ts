// 侧边栏组件共享元数据（Spec 58）：设置页 WidgetsTab（组件编排）与插件库
// WidgetLibraryView（方案保存）共同消费，避免两处名单漂移。
// 组件清单与缺省参数对齐后端 DefaultWidgetLibrary（backend/internal/template/model_widgets.go），
// 参数说明对照 backend/config/site.example.yml widgets 段注释。

export type WidgetFieldType = "text" | "number" | "checkbox" | "color" | "select" | "textarea";

export interface WidgetField {
  key: string;
  type: WidgetFieldType;
  /** 表单标签（中文），缺省展示 key */
  label?: string;
  /** 内置缺省值（= 后端 DefaultWidgetLibrary 对应参数） */
  def?: unknown;
  /** 补充说明 */
  hint?: string;
  /** type=select 时的候选值 */
  options?: string[];
}

export interface WidgetMeta {
  /** library 内置组件名（禁止作为方案名） */
  name: string;
  label: string;
  layout: string;
  desc: string;
  /** 参数 schema；空数组 = 该组件无额外可配置参数 */
  fields: WidgetField[];
}

export const WIDGET_META: WidgetMeta[] = [
  {
    name: "recent",
    label: "近期发布",
    layout: "recent",
    desc: "展示最新发布的文章列表。",
    fields: [
      { key: "limit", type: "number", label: "显示条数", def: 5 },
      { key: "rss", type: "text", label: "RSS 订阅链接", def: "/rss.xml", hint: "置空字符串可隐藏订阅按钮" },
    ],
  },
  {
    name: "tagcloud",
    label: "标签云",
    layout: "tagcloud",
    desc: "按文章数/名称排列的标签云，可配置字号区间与配色。",
    fields: [
      { key: "title", type: "text", label: "标题", def: "标签" },
      { key: "amount", type: "number", label: "显示标签数", def: 20 },
      { key: "min_font", type: "number", label: "最小字号 (px)", def: 12 },
      { key: "max_font", type: "number", label: "最大字号 (px)", def: 22 },
      { key: "orderby", type: "select", label: "排序字段", def: "count", options: ["count", "name"] },
      { key: "order", type: "number", label: "排序方向", def: -1, hint: "-1 降序 / 1 升序" },
      { key: "show_count", type: "checkbox", label: "显示文章数", def: false },
      { key: "color", type: "checkbox", label: "渐变色", def: false, hint: "开启后在起止颜色间渐变" },
      { key: "start_color", type: "color", label: "起始颜色", def: "#999", hint: "color 开启时生效" },
      { key: "end_color", type: "color", label: "结束颜色", def: "#1BCDFC", hint: "color 开启时生效" },
    ],
  },
  {
    name: "popular",
    label: "热门文章",
    layout: "popular",
    desc: "按阅读量排序的热门文章列表。",
    fields: [{ key: "limit", type: "number", label: "显示条数", def: 5 }],
  },
  {
    name: "related",
    label: "专栏系列",
    layout: "related",
    desc: "文章所属专栏的系列导航，仅文章属专栏时显示。",
    fields: [],
  },
  {
    name: "welcome",
    label: "欢迎语",
    layout: "markdown",
    desc: "通用 markdown 文案组件，可用于公告/欢迎语。",
    fields: [
      { key: "title", type: "text", label: "标题", def: "欢迎，旅行者：" },
      { key: "content", type: "textarea", label: "正文 (Markdown)", hint: "支持 Markdown 文本" },
    ],
  },
  {
    name: "author",
    label: "作者卡片",
    layout: "author",
    desc: "展示当前文章作者信息。",
    fields: [],
  },
  {
    name: "siteinfo",
    label: "站点统计",
    layout: "siteinfo",
    desc: "文章数、字数、阅读量等站点统计信息。",
    fields: [],
  },
  {
    name: "latest_comment",
    label: "最新评论",
    layout: "latest_comment",
    desc: "最近发布的评论列表（评论启用时）。",
    fields: [
      { key: "title", type: "text", label: "标题", def: "最新评论" },
      { key: "limit", type: "number", label: "显示条数", def: 5 },
    ],
  },
  {
    name: "toc",
    label: "文章目录",
    layout: "toc",
    desc: "从文章标题提取的目录，仅文章页有内容时显示。",
    fields: [
      { key: "list_number", type: "checkbox", label: "显示序号", def: false },
      { key: "min_depth", type: "number", label: "最小标题层级", def: 1 },
      { key: "max_depth", type: "number", label: "最大标题层级", def: 6 },
      { key: "collapse", type: "checkbox", label: "折叠子层级", def: false },
    ],
  },
  {
    name: "ghrepo",
    label: "GitHub 仓库卡",
    layout: "ghrepo",
    desc: "GitHub 仓库信息卡片（走本地 /api/gh 代理）。",
    fields: [{ key: "repo", type: "text", label: "仓库", hint: "格式 owner/repo" }],
  },
  {
    name: "ghuser",
    label: "GitHub 用户卡",
    layout: "ghuser",
    desc: "GitHub 用户信息卡片。",
    fields: [
      { key: "username", type: "text", label: "用户名" },
      { key: "avatar", type: "checkbox", label: "显示头像", def: true },
    ],
  },
  {
    name: "ghissues",
    label: "GitHub Issues",
    layout: "ghissues",
    desc: "展示指定仓库的 Issues 列表。",
    fields: [
      { key: "repo", type: "text", label: "仓库", hint: "格式 owner/repo" },
      { key: "title", type: "text", label: "标题", def: "Recent Issues" },
      { key: "limit", type: "number", label: "显示条数", def: 3 },
      { key: "labels", type: "text", label: "标签过滤", hint: "留空显示全部" },
    ],
  },
  {
    name: "timeline",
    label: "近期动态",
    layout: "timeline",
    desc: "友链 RSS / Issues API 聚合的动态列表，需配置 api。",
    fields: [
      { key: "title", type: "text", label: "标题", def: "近期动态" },
      { key: "api", type: "text", label: "API 地址", hint: "RSS 或 Issues API" },
      { key: "user", type: "text", label: "用户过滤" },
      { key: "limit", type: "number", label: "显示条数", def: 10 },
    ],
  },
  {
    name: "tree",
    label: "文库项目树",
    layout: "tree",
    desc: "wiki 文档页的项目文档树，仅文库页面有数据。",
    fields: [],
  },
  {
    name: "wiki_related",
    label: "文库相关项目",
    layout: "wiki_related",
    desc: "与当前文库项目同标签的其他项目推荐。",
    fields: [
      { key: "title", type: "text", label: "标题", def: "更多项目" },
      { key: "limit", type: "number", label: "显示条数", def: 5 },
    ],
  },
  {
    name: "tagtree",
    label: "笔记本标签树",
    layout: "tagtree",
    desc: "笔记本页面的标签树，仅笔记本页面有数据。",
    fields: [],
  },
];

export const BUILTIN_WIDGET_NAMES = new Set(WIDGET_META.map((w) => w.name));

export function metaOf(name: string): WidgetMeta | undefined {
  return WIDGET_META.find((w) => w.name === name);
}

/** 内置组件名 → layout（未知名返回空串） */
export function builtinLayoutOf(name: string): string {
  return metaOf(name)?.layout || "";
}

/** layout → 字段 schema（自定义组件与内置组件共享同 layout 时同样命中） */
const fieldsByLayout = new Map<string, WidgetField[]>(WIDGET_META.map((m) => [m.layout, m.fields]));

export function fieldsForLayout(layout: string): WidgetField[] | undefined {
  return fieldsByLayout.get(layout);
}

/** 无额外可配置参数的 layout 集合（author/siteinfo 等） */
export const NO_EXTRA_PARAM_LAYOUTS = new Set(
  WIDGET_META.filter((m) => m.fields.length === 0).map((m) => m.layout)
);

/** 内置组件缺省定义（layout + 各字段 def 值），作为方案编辑的初始表单 */
export function builtinDefaults(name: string): Record<string, any> {
  const m = metaOf(name);
  if (!m) return {};
  const out: Record<string, any> = { layout: m.layout };
  for (const f of m.fields) {
    if (f.def !== undefined) out[f.key] = f.def;
  }
  return out;
}
