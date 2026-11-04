import {
  canonicalFieldByPath,
  type CanonicalSettingsField,
  type CanonicalSettingsGroup,
} from "./settings-canonical-fields";

// 系统设置的页面注册表：5 组 12 页，每个字段路径只属于一页。
// 字段定义（标签、默认值、控件类型）只在 settings-canonical-fields.ts 维护，这里只按路径引用。
export type SettingsPageKey =
  | "site"
  | "seo"
  | "appearance"
  | "navigation"
  | "layout"
  | "post"
  | "listing"
  | "community"
  | "comments"
  | "feedback"
  | "extensions"
  | "advanced";
export type SettingsHostSection = "site" | "feedback";

export interface SettingsPageSection {
  id: string;
  title: string;
  description: string;
  /** 主题草稿字段路径；宿主页（站点、反馈）与仅作锚点的分区为空。 */
  paths: string[];
}
export interface SettingsPage {
  key: SettingsPageKey;
  /** 导航与手机列表中的名称。 */
  label: string;
  summary: string;
  /** 通过宿主分段接口独立保存；缺省表示共享主题草稿。 */
  host?: SettingsHostSection;
  sections: SettingsPageSection[];
  /** 整页一键恢复默认：移除该主题节点的全部覆盖。 */
  reset?: { path: string; label: string };
}
export interface SettingsGroup {
  key: string;
  label: string;
  pages: SettingsPageKey[];
}

const article = "theme.article";
const content = "blog_extensions.content";
const nativeComments = "blog_extensions.native_comments";
const friends = "blog_extensions.friends";
const friendsPage = "blog_extensions.pages.friends";
const aiLabel = `${content}.ai_label`;

export const settingsPages: Record<SettingsPageKey, SettingsPage> = {
  site: {
    key: "site",
    label: "站点信息",
    summary: "站点的名称、标识图片与对外描述；本页独立保存，保存后前台读取新值。",
    host: "site",
    sections: [
      { id: "id", title: "站点标识", description: "显示在前台首页、导航栏与浏览器标签中。", paths: [] },
      { id: "logo", title: "Logo", description: "站点头像与 Logo 文案会显示在前台导航区域。", paths: [] },
      {
        id: "icon",
        title: "浏览器与设备图标",
        description: "标签页、收藏夹与 iOS 主屏幕使用；留空时使用默认图标。",
        paths: [],
      },
      {
        id: "desc",
        title: "描述与地址",
        description: "用于首页与未单独设置描述的页面，也是搜索结果和分享卡片的来源。",
        paths: [],
      },
    ],
  },
  seo: {
    key: "seo",
    label: "SEO 与搜索",
    summary: "站内搜索服务、页面地址与分享信息、搜索引擎收录与站长验证；站点描述与 URL 在「站点信息」。",
    sections: [
      {
        id: "search",
        title: "站内搜索",
        description:
          "选择访客搜索所使用的服务。本地静态索引由 Blog 生成并随站点发布，Go 搜索直接请求 Blog 服务，其缓存固定为 30 秒。",
        paths: [
          "theme.search.provider",
          "theme.search.local.scope",
          "theme.search.local.include_content",
          "theme.search.local.cache_ttl_seconds",
        ],
      },
      {
        id: "social",
        title: "页面地址与社交信息",
        description: "设置规范地址、分享元信息以及站点对外关联信息。",
        paths: [
          "theme.canonical.host",
          "theme.canonical.allowed_hosts",
          "theme.open_graph.enabled",
          "theme.open_graph.twitter_id",
          "theme.structured_data.same_as",
        ],
      },
      {
        id: "crawler",
        title: "搜索引擎与站长验证",
        description: "设置缺省元信息、索引策略和搜索引擎验证标识。",
        paths: [
          "blog_extensions.seo.default_description",
          "blog_extensions.seo.default_keywords",
          "blog_extensions.seo.no_index",
          "blog_extensions.seo.robots_extra",
          "blog_extensions.seo.google_verification",
          "blog_extensions.seo.bing_verification",
          "blog_extensions.seo.baidu_verification",
        ],
      },
    ],
  },
  appearance: {
    key: "appearance",
    label: "主题外观",
    summary: "决定整站的质感、配色与字体；改动先进入草稿，可用页头的「前端预览」查看。",
    reset: { path: "theme.appearance", label: "主题外观" },
    sections: [
      {
        id: "preset",
        title: "外观预设",
        description: "决定卡片、侧栏与导航的整体质感，并选择明暗配色。",
        paths: ["theme.appearance.preset", "theme.appearance.color_scheme"],
      },
      {
        id: "color",
        title: "颜色",
        description: "主题色用于导航与强调元素，强调色用于标签与高亮，链接色用于正文链接；渐变用于主要操作与搜索栏。",
        paths: ["theme.appearance.colors", "theme.appearance.gradients"],
      },
      {
        id: "type",
        title: "字体与排版",
        description: "正文与代码的字体、字号和对齐方式。",
        paths: ["theme.appearance.typography", "theme.appearance.code_block"],
      },
      {
        id: "shape",
        title: "形状与圆角",
        description: "不同元素使用不同圆角，数值越大越圆润。",
        paths: ["theme.appearance.shape"],
      },
      {
        id: "bg",
        title: "背景",
        description: "左侧栏与页面的背景。",
        paths: ["theme.appearance.backgrounds"],
      },
    ],
  },
  navigation: {
    key: "navigation",
    label: "导航与页脚",
    summary: "顶栏、左侧栏与站点页脚；列表顺序即前台顺序。",
    sections: [
      {
        id: "topbar",
        title: "顶栏",
        description: "设置顶部品牌标识、菜单与组件。",
        paths: ["theme.topbar"],
      },
      {
        id: "leftbar",
        title: "左侧栏",
        description: "管理品牌、搜索、菜单及快捷操作；列表顺序即前台顺序。",
        paths: ["theme.leftbar"],
      },
      {
        id: "footer",
        title: "站点页脚",
        description: "编辑站点地图与页脚内容。",
        paths: ["theme.footer"],
      },
    ],
  },
  layout: {
    key: "layout",
    label: "页面与侧栏",
    summary: "各类页面的布局覆盖、右侧栏与侧栏组件定义。",
    sections: [
      {
        id: "profiles",
        title: "各类页面布局",
        description: "分别覆盖首页、文章、文档、笔记等页面。继承默认不会创建额外覆盖；空列表表示不展示对应项目。",
        paths: ["theme.profiles"],
      },
      {
        id: "rightbar",
        title: "右侧栏",
        description: "设置右侧栏开关与默认组件。",
        paths: ["theme.rightbar"],
      },
      {
        id: "widgets",
        title: "侧栏组件定义",
        description: "为侧栏组件标识配置内容。",
        paths: ["blog_extensions.data.widgets"],
      },
    ],
  },
  post: {
    key: "post",
    label: "文章页",
    summary: "文章页面的阅读体验、页脚与扩展；列表上的卡片样式在「列表与卡片」。",
    sections: [
      {
        id: "read",
        title: "阅读体验",
        description: "单篇文章可以覆盖其中支持的默认项。",
        paths: [
          `${article}.style`,
          `${article}.paragraph_indent`,
          `${article}.banner.background`,
          `${article}.show_reading_time`,
        ],
      },
      {
        id: "foot",
        title: "页脚与许可",
        description: "正文结束后的相关文章、标签、许可协议和分享入口。",
        paths: [
          `${article}.related_posts_limit`,
          `${article}.footer.show_tags`,
          `${article}.footer.license`,
          `${article}.footer.share`,
        ],
      },
      {
        id: "ext",
        title: "文章扩展",
        description: "启用后下列功能才会在前台生效；关闭时保留已填写的配置。",
        paths: [
          `${content}.enabled`,
          `${content}.banner_ratio`,
          `${content}.read_next`,
          `${content}.expire_days`,
          `${content}.edit_this_page`,
          `${content}.share.enabled`,
          `${content}.share.platforms`,
          `${content}.reward.enabled`,
          `${content}.reward.text`,
          `${content}.reward.wechat`,
          `${content}.reward.alipay`,
        ],
      },
      {
        id: "ai",
        title: "AI 创作标记",
        description: "设置文章默认标记与各档位外观；关闭某个档位会隐藏该档位的标记。",
        paths: [
          `${aiLabel}.default`,
          `${aiLabel}.manual`,
          `${aiLabel}.reviewed`,
          `${aiLabel}.polished`,
          `${aiLabel}.generated`,
        ],
      },
    ],
  },
  listing: {
    key: "listing",
    label: "列表与卡片",
    summary: "文章列表上的置顶与卡片样式，以及分类与标签的颜色、图标。",
    sections: [
      {
        id: "cards",
        title: "置顶与文章卡片",
        description: "设置文章列表的置顶方式与卡片版式。单篇文章可覆盖支持的默认项。",
        paths: [
          `${article}.listing.pinned_layout`,
          `${article}.listing.card_layout`,
          `${article}.listing.cover_ratio`,
          `${article}.listing.excerpt_length`,
          `${article}.listing.show_tags`,
        ],
      },
      {
        id: "taxonomy",
        title: "分类与标签样式",
        description: "分类颜色属于主题设置；分类与标签图标需要在「文章页 · 文章扩展」启用后才会显示。",
        paths: [`${article}.category_colors`, `${content}.category_icons`, `${content}.tag_icons`],
      },
    ],
  },
  community: {
    key: "community",
    label: "作者与友链",
    summary: "公开作者信息、友链更新方式与友链页面。",
    sections: [
      {
        id: "authors",
        title: "作者",
        description: "按作者标识管理名称、头像、简介及链接。",
        paths: ["blog_extensions.data.authors"],
      },
      {
        id: "refresh",
        title: "友链更新",
        description: "设置友链文章聚合、巡检与快照的刷新方式。",
        paths: [
          `${friends}.refresh_interval`,
          `${friends}.posts_per_link`,
          `${friends}.check_interval`,
          `${friends}.snapshot_interval`,
          `${friends}.snapshot_provider`,
        ],
      },
      {
        id: "page",
        title: "友链页面",
        description: "设置友链页标题与文章展示。",
        paths: [
          `${friendsPage}.title`,
          `${friendsPage}.hero`,
          `${friendsPage}.show_posts`,
          `${friendsPage}.refresh_interval`,
          `${friendsPage}.posts_per_link`,
        ],
      },
    ],
  },
  comments: {
    key: "comments",
    label: "评论",
    summary: "访客如何在文章下参与讨论：原生评论与邮件通知。",
    sections: [
      {
        id: "native",
        title: "原生评论",
        description: "Blog 自带的评论功能；单页开关与审核规则仍会影响前台实际展示。",
        paths: [
          `${nativeComments}.enabled`,
          `${nativeComments}.home.enabled`,
          `${nativeComments}.moderation`,
          `${nativeComments}.title`,
          `${nativeComments}.admin_badge`,
          `${nativeComments}.admin_email`,
          `${nativeComments}.gravatar_mirror`,
        ],
      },
      {
        id: "mail",
        title: "评论邮件通知",
        description: "SMTP 主机留空时不发送通知；保存配置不会发送测试邮件。",
        paths: [
          `${nativeComments}.notify.smtp_host`,
          `${nativeComments}.notify.smtp_port`,
          `${nativeComments}.notify.username`,
          `${nativeComments}.notify.password`,
          `${nativeComments}.notify.from`,
        ],
      },
    ],
  },
  feedback: {
    key: "feedback",
    label: "访客反馈",
    summary: "接入 Feedback 反馈组件并设置前台悬浮入口的外观；本页独立保存。",
    host: "feedback",
    sections: [
      {
        id: "service",
        title: "Feedback 服务接入",
        description: "填写服务地址与软件标识后才会渲染反馈组件。",
        paths: [],
      },
      {
        id: "launcher",
        title: "入口与截图",
        description: "悬浮入口的形态、停靠位置与呼出时的截图行为。",
        paths: [],
      },
    ],
  },
  extensions: {
    key: "extensions",
    label: "扩展与服务",
    summary: "标签插件、前台增强、预连接地址、外部服务与默认图片。",
    sections: [
      {
        id: "tags",
        title: "标签插件",
        description: "设置提示块、表情、引用和图库等内容组件。",
        paths: ["theme.tags"],
      },
      {
        id: "features",
        title: "前台增强",
        description: "管理局部导航、图片、动画、数学公式与图表。",
        paths: ["theme.features"],
      },
      {
        id: "preconnect",
        title: "预连接地址",
        description: "提前与外部资源域名建立连接，加快字体、图片等资源的加载。",
        paths: ["theme.preconnect"],
      },
      {
        id: "services",
        title: "外部服务",
        description:
          "设置头像、Markdown 渲染、贡献者与 GitHub 接口。站点信息、评分与投票已由站内原生功能提供，不再从这里配置。",
        paths: [
          "theme.services.gravatar",
          "theme.services.markdown",
          "theme.services.contributors",
          "theme.services.github_card",
          "theme.services.github",
        ],
      },
      {
        id: "fallbacks",
        title: "默认图片",
        description: "资源缺失时使用的头像、链接卡片和封面。",
        paths: ["theme.fallbacks"],
      },
    ],
  },
  advanced: {
    key: "advanced",
    label: "代码与原文",
    summary: "页面代码注入与完整配置原文；仅在需要导入、修复或检查原文时使用。",
    sections: [
      {
        id: "inject",
        title: "代码注入",
        description: "代码将原样输出到前台页面。只填写可信代码，错误脚本可能影响访客页面。",
        paths: ["theme.inject.head_begin", "theme.inject.head_end", "theme.inject.body_begin", "theme.inject.body_end"],
      },
      {
        id: "raw",
        title: "完整配置原文",
        description: "所有主题页面共享同一份未保存草稿。需要导入、修复或检查原文时，可展开下方完整配置。",
        paths: [],
      },
    ],
  },
};

export const settingsPageKeys = Object.keys(settingsPages) as SettingsPageKey[];

// 导航顺序与分组；与画板「归属对照」一致。
export const settingsGroups: SettingsGroup[] = [
  { key: "site", label: "站点", pages: ["site", "seo"] },
  { key: "appearance", label: "外观与布局", pages: ["appearance", "navigation", "layout"] },
  { key: "content", label: "内容", pages: ["post", "listing", "community"] },
  { key: "interaction", label: "互动", pages: ["comments", "feedback"] },
  { key: "extension", label: "扩展与高级", pages: ["extensions", "advanced"] },
];

export const DEFAULT_SETTINGS_PAGE: SettingsPageKey = "site";
// 「AI 与知识库」配置仍在助手模块，设置导航只放入口。
export const SETTINGS_AI_ROUTE = { path: "/assistant", query: { tab: "settings" } } as const;

export type SettingsTabTarget =
  | { kind: "page"; page: SettingsPageKey; raw?: boolean }
  | { kind: "route"; path: string; query?: Record<string, string> };

// 旧版 13 个 Tab 与更早的分段键仍可能出现在书签、通知与其他模块的跳转里。
const legacyTabs: Record<string, SettingsTabTarget> = {
  basic: { kind: "page", page: "site" },
  search: { kind: "page", page: "seo" },
  plugins: { kind: "page", page: "extensions" },
  services: { kind: "page", page: "extensions" },
  article: { kind: "page", page: "post" },
  inject: { kind: "page", page: "advanced" },
  data_cache: { kind: "page", page: "advanced" },
  theme_yaml: { kind: "page", page: "advanced" },
  cover: { kind: "page", page: "layout" },
  nav_tabs: { kind: "page", page: "navigation" },
  widgets: { kind: "page", page: "layout" },
  tag_plugins: { kind: "page", page: "extensions" },
  default: { kind: "page", page: "extensions" },
  menubar: { kind: "page", page: "navigation" },
  rightbar: { kind: "page", page: "layout" },
  footer: { kind: "page", page: "navigation" },
  authors: { kind: "page", page: "community" },
  stellar2: { kind: "page", page: "advanced", raw: true },
  ai: { kind: "route", path: "/assistant", query: { tab: "settings" } },
  security: { kind: "route", path: "/profile" },
};

export function isSettingsPageKey(value: unknown): value is SettingsPageKey {
  return typeof value === "string" && Object.hasOwn(settingsPages, value);
}

export function resolveSettingsTab(tab: unknown): SettingsTabTarget {
  if (isSettingsPageKey(tab)) return { kind: "page", page: tab };
  if (typeof tab === "string" && Object.hasOwn(legacyTabs, tab)) return legacyTabs[tab];
  return { kind: "page", page: DEFAULT_SETTINGS_PAGE };
}

export function settingsSectionAnchor(page: SettingsPageKey, sectionId: string): string {
  return `settings-sec-${page}-${sectionId}`;
}

export const settingsThemePages = settingsPageKeys.filter((key) => !settingsPages[key].host);

/** 字段是否同时改写 Go 搜索的覆盖路径。 */
export function canonicalFieldPaths(item: CanonicalSettingsField): string[] {
  return item.kind === "provider" ? [item.path, "blog_extensions.search.provider"] : [item.path];
}

export function settingsPageFields(page: SettingsPageKey): CanonicalSettingsField[] {
  return settingsPages[page].sections.flatMap((section) =>
    section.paths.map((path) => {
      const item = canonicalFieldByPath.get(path);
      if (!item) throw new Error(`设置页 ${page} 引用了未定义的字段 ${path}`);
      return item;
    }),
  );
}

/** 该页拥有的全部草稿路径（用于未保存计数与放弃本页修改）。 */
export function settingsPagePaths(page: SettingsPageKey): string[] {
  return settingsPageFields(page).flatMap(canonicalFieldPaths);
}

/** 交给规范表单渲染的分区；无字段的锚点分区不渲染。 */
export function settingsPageGroups(page: SettingsPageKey): CanonicalSettingsGroup[] {
  return settingsPages[page].sections
    .filter((section) => section.paths.length > 0)
    .map((section) => ({
      id: section.id,
      title: section.title,
      description: section.description,
      fields: section.paths.map((path) => {
        const item = canonicalFieldByPath.get(path);
        if (!item) throw new Error(`设置页 ${page} 引用了未定义的字段 ${path}`);
        return item;
      }),
    }));
}
