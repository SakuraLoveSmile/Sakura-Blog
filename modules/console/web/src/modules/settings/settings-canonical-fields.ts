export type CanonicalSettingsTab = "post" | "comments" | "seo" | "search" | "advanced" | "ai";
export type CanonicalFieldKind =
  | "text"
  | "password"
  | "textarea"
  | "number"
  | "switch"
  | "select"
  | "list"
  | "choices"
  | "map"
  | "json"
  | "license"
  | "provider"
  | "image"
  | "ai-label";
export interface CanonicalSettingsField {
  path: string;
  label: string;
  kind: CanonicalFieldKind;
  default?: unknown;
  help?: string;
  placeholder?: string;
  options?: readonly (readonly [string, string])[];
  min?: number;
  max?: number;
  step?: number | "any";
}
export interface CanonicalSettingsGroup {
  title: string;
  description: string;
  fields: CanonicalSettingsField[];
}
const article = "theme.article";
const content = "blog_extensions.content";
const comments = "blog_extensions.native_comments";
const ai = "blog_extensions.ai";
export const DEFAULT_ARTICLE_LICENSE =
  "本文采用 [署名-非商业性使用-相同方式共享 4.0 国际](https://creativecommons.org/licenses/by-nc-sa/4.0/) 许可协议，转载请注明出处。";
const field = (
  path: string,
  label: string,
  kind: CanonicalFieldKind,
  extra: Omit<CanonicalSettingsField, "path" | "label" | "kind"> = {},
): CanonicalSettingsField => ({ path, label, kind, ...extra });
const toggle = (path: string, label: string, value = false, help?: string) =>
  field(path, label, "switch", { default: value, help });
const number = (path: string, label: string, value?: number, extra: Partial<CanonicalSettingsField> = {}) =>
  field(path, label, "number", { default: value, min: 0, step: 1, ...extra });
const text = (path: string, label: string, help?: string) => field(path, label, "text", { help });
const list = (path: string, label: string, value: string[] = []) =>
  field(path, label, "list", { default: value, help: "每行一项。" });

export const canonicalSettingsGroups: Record<CanonicalSettingsTab, CanonicalSettingsGroup[]> = {
  post: [
    {
      title: "文章与卡片",
      description: "设置文章的默认排版和文章列表展示。单篇文章可覆盖支持的默认项。",
      fields: [
        field(`${article}.style`, "文章风格", "select", {
          default: "tech",
          options: [
            ["tech", "技术文章"],
            ["story", "叙事文章"],
          ],
        }),
        field(`${article}.paragraph_indent`, "段落首行缩进", "select", {
          default: "auto",
          options: [
            ["auto", "按文章风格自动"],
            ["always", "始终缩进"],
            ["never", "不缩进"],
          ],
        }),
        field(`${article}.listing.pinned_layout`, "置顶布局", "select", {
          default: "carousel",
          options: [
            ["carousel", "轮播"],
            ["flat", "平铺"],
          ],
        }),
        field(`${article}.listing.card_layout`, "文章卡片", "select", {
          default: "hero",
          options: [
            ["hero", "大图卡片"],
            ["classic", "经典卡片"],
          ],
        }),
        number(`${article}.listing.cover_ratio`, "卡片封面宽高比", 2, {
          min: 0,
          step: "any",
          help: "请填写大于 0 的比例。",
        }),
        number(`${article}.listing.excerpt_length`, "摘要字数", 128, { help: "设为 0 时不显示自动摘要。" }),
        toggle(`${article}.listing.show_tags`, "卡片显示标签"),
        toggle(`${article}.show_reading_time`, "显示阅读时长"),
        number(`${article}.related_posts_limit`, "相关文章数量", 0, { help: "设为 0 时不显示相关文章。" }),
        field(`${article}.category_colors`, "分类颜色", "map", {
          default: { 探索号: "#f44336" },
          help: "分类名称对应颜色；未设置的分类沿用主题默认。",
        }),
      ],
    },
    {
      title: "文章页脚",
      description: "管理文章页脚标签、许可协议和主题分享入口。",
      fields: [
        toggle(`${article}.footer.show_tags`, "页脚显示标签", true),
        field(`${article}.footer.license`, "文章许可协议", "license", { placeholder: DEFAULT_ARTICLE_LICENSE }),
        field(`${article}.footer.share`, "主题分享平台", "choices", {
          default: ["qrcode", "weibo", "x", "telegram", "whatsapp", "email"],
          options: [
            ["qrcode", "二维码"],
            ["weibo", "微博"],
            ["x", "X"],
            ["telegram", "Telegram"],
            ["whatsapp", "WhatsApp"],
            ["email", "邮件"],
          ],
        }),
      ],
    },
    {
      title: "文章扩展",
      description: "以下扩展需启用后才会在前台生效；关闭时保留已填写的配置。",
      fields: [
        toggle(`${content}.enabled`, "启用文章扩展"),
        number(`${content}.banner_ratio`, "文章横幅宽高比", undefined, {
          step: "any",
          help: "设为 0 时沿用默认比例。",
        }),
        toggle(`${content}.read_next`, "显示下一篇文章", true),
        number(`${content}.expire_days`, "过期提醒天数", undefined, {
          min: undefined,
          step: "any",
          help: "未设置时不提醒；显式设为 0 或负数时使用 180 天。",
        }),
        field(`${content}.category_icons`, "分类图标", "map"),
        field(`${content}.tag_icons`, "标签图标", "map"),
        field(`${content}.edit_this_page`, "编辑文章链接", "map", {
          help: "来源路径对应仓库编辑地址；路径中的点号会完整保留。",
        }),
        toggle(`${content}.share.enabled`, "启用扩展分享"),
        field(`${content}.share.platforms`, "扩展分享平台", "choices", {
          default: [],
          options: [
            ["wechat", "微信"],
            ["link", "复制链接"],
            ["weibo", "微博"],
            ["email", "邮件"],
          ],
        }),
        toggle(`${content}.reward.enabled`, "启用打赏"),
        text(`${content}.reward.text`, "打赏文案"),
        field(`${content}.reward.wechat`, "微信收款码", "image"),
        field(`${content}.reward.alipay`, "支付宝收款码", "image"),
      ],
    },
    {
      title: "AI 创作标记",
      description: "设置文章默认标记和各档位外观。关闭某个档位会隐藏该档位标记。",
      fields: [
        field(`${content}.ai_label.default`, "默认创作标记", "select", {
          default: "",
          options: [["", "无默认标记"], ["manual", "人工创作"], ["reviewed", "AI 辅助审阅"], ["polished", "AI 润色"], ["generated", "AI 生成"]],
        }),
        ...(
          [
            ["manual", "人工创作", "#03a9f4", "default:shield-user"],
            ["reviewed", "AI 辅助审阅", "#4caf50", "default:shield-check"],
            ["polished", "AI 润色", "#4caf50", "default:shield-up"],
            ["generated", "AI 生成", "#ff9800", "default:shield-warning"],
          ] as const
        ).map(([key, label, color, icon]) =>
          field(`${content}.ai_label.${key}`, label, "ai-label", { default: { color, icon } }),
        ),
      ],
    },
  ],
  comments: [
    {
      title: "原生评论",
      description: "管理 Blog 自带的评论功能，与下方第三方评论分别配置。",
      fields: [
        toggle(`${comments}.enabled`, "启用原生评论"),
        toggle(`${comments}.home.enabled`, "首页启用原生评论"),
        toggle(`${comments}.moderation`, "评论需要审核"),
        field(`${comments}.title`, "评论区标题", "text", { default: "评论" }),
        field(`${comments}.admin_badge`, "管理员徽章", "text", { default: "博主" }),
        text(`${comments}.admin_email`, "管理员邮箱"),
        field(`${comments}.gravatar_mirror`, "Gravatar 镜像地址", "text", { default: "https://weavatar.com/avatar" }),
      ],
    },
    {
      title: "评论邮件通知",
      description: "SMTP 主机留空时不发送通知。本页保存配置不会发送测试邮件。",
      fields: [
        text(`${comments}.notify.smtp_host`, "SMTP 主机"),
        number(`${comments}.notify.smtp_port`, "SMTP 端口", undefined, { max: 65535 }),
        text(`${comments}.notify.username`, "SMTP 用户名"),
        field(`${comments}.notify.password`, "SMTP 密码", "password", {
          help: "保留 **** 表示不修改原密码；清空输入可清除密码。",
        }),
        text(`${comments}.notify.from`, "发件人邮箱"),
      ],
    },
    {
      title: "第三方评论",
      description: "用于 Giscus、Waline、Artalk 等主题评论服务，与原生评论独立。仅编辑需要调整的服务参数。",
      fields: [
        field("theme.comments", "第三方评论参数", "json", {
          help: "填写 JSON 对象；服务支持的开放参数、空值和大整数均按原文保留。",
        }),
      ],
    },
  ],
  seo: [
    {
      title: "页面地址与社交信息",
      description: "设置规范地址、分享元信息以及站点对外关联信息。",
      fields: [
        text("theme.canonical.host", "规范地址主机", "未设置时沿用站点地址。"),
        list("theme.canonical.allowed_hosts", "允许的主机名", ["localhost"]),
        toggle("theme.open_graph.enabled", "启用 Open Graph", true),
        text("theme.open_graph.twitter_id", "X / Twitter 账号"),
        list("theme.preconnect", "预连接地址"),
        list("theme.structured_data.same_as", "站点关联链接"),
      ],
    },
    {
      title: "搜索引擎与站长验证",
      description: "设置缺省元信息、索引策略和搜索引擎验证标识。",
      fields: [
        text("blog_extensions.seo.default_description", "默认描述"),
        text("blog_extensions.seo.default_keywords", "默认关键词"),
        toggle("blog_extensions.seo.no_index", "禁止搜索引擎收录"),
        field("blog_extensions.seo.robots_extra", "robots.txt 附加规则", "textarea"),
        text("blog_extensions.seo.google_verification", "Google 验证标识"),
        text("blog_extensions.seo.bing_verification", "Bing 验证标识"),
        text("blog_extensions.seo.baidu_verification", "百度验证标识"),
      ],
    },
  ],
  search: [
    {
      title: "搜索服务",
      description: "选择访客搜索所使用的服务。Go 搜索直接请求 Blog 服务，其缓存固定为 30 秒。",
      fields: [
        field("theme.search.provider", "搜索服务", "provider", {
          default: "local",
          options: [
            ["local", "本地静态索引"],
            ["go", "Go 服务搜索"],
            ["algolia", "Algolia"],
            ["none", "关闭搜索"],
          ],
        }),
        field("theme.search.local.scope", "本地索引范围", "select", {
          default: "all",
          options: [
            ["all", "全部内容"],
            ["post", "仅文章"],
            ["page", "仅独立页面"],
          ],
        }),
        toggle("theme.search.local.include_content", "本地索引包含正文", true),
        number("theme.search.local.cache_ttl_seconds", "本地索引缓存时长（秒）", 86400, {
          help: "设为 0 时不缓存。此项仅用于静态索引，对 Go 服务搜索无效。",
        }),
      ],
    },
    {
      title: "Algolia",
      description: "仅使用 Algolia 时需要配置此对象；切换服务会保留已有参数。",
      fields: [
        field("theme.search.algolia", "Algolia 参数", "json", {
          help: "填写 JSON 对象，例如 appId、apiKey、indexName。未编辑的参数按原文保留。",
        }),
      ],
    },
    {
      title: "索引说明",
      description: "索引地址和加载方式由当前搜索服务管理。旧版排除路径、索引生成路径和延迟加载选项已不再使用。",
      fields: [],
    },
  ],
  advanced: [
    {
      title: "代码注入",
      description: "代码将原样输出到前台页面。只填写可信代码，错误脚本可能影响访客页面。",
      fields: [
        field("theme.inject.head_begin", "头部开始（<head> 之后）", "textarea"),
        field("theme.inject.head_end", "头部结束（</head> 之前）", "textarea"),
        field("theme.inject.body_begin", "正文开始（<body> 之后）", "textarea"),
        field("theme.inject.body_end", "正文结束（</body> 之前）", "textarea"),
      ],
    },
    {
      title: "其他高级配置",
      description:
        "旧版动态数据缓存选项已不再使用。其他主题设置可在「Stellar 2」标签编辑，所有标签共享同一份未保存草稿。",
      fields: [],
    },
  ],
  ai: [
    {
      title: "本地 AI 服务",
      description: "用于文章属性生成与知识库问答的 OpenAI 兼容接口。密钥只保存在服务端，表单中显示为 ****。",
      fields: [
        text(
          `${ai}.base_url`,
          "接口地址",
          "OpenAI 兼容服务的根地址，例如 https://api.openai.com/v1；系统会追加 /chat/completions 与 /embeddings。",
        ),
        field(`${ai}.api_key`, "API Key", "password", {
          help: "保留 **** 表示不修改原密钥；清空输入可清除密钥。",
        }),
        text(`${ai}.model`, "对话模型", "文章属性生成与知识库问答使用的模型名称。"),
        text(`${ai}.embedding_model`, "向量模型", "知识库向量化与检索使用的 Embedding 模型名称；留空时知识库不可用。"),
      ],
    },
    {
      title: "AI 摘要",
      description: "控制前台文章页的 AI 摘要入口，以及读者能否使用自己的接口。",
      fields: [
        toggle(`${ai}.summary.enabled`, "启用 AI 摘要", false, "关闭时前台不展示摘要面板。"),
        toggle(
          `${ai}.summary.allow_user_api`,
          "允许读者使用自己的接口",
          false,
          "需先开启 AI 摘要。读者填写的 Key 仅随本次请求透传，不会被保存。",
        ),
      ],
    },
    {
      title: "AI 问答",
      description: "控制前台知识库问答入口，以及读者能否提供自己的对话模型。",
      fields: [
        toggle(`${ai}.ask.enabled`, "启用知识库问答", false, "默认关闭。仅配置向量模型不会公开问答接口。"),
        toggle(`${ai}.ask.allow_user_llm`, "允许读者使用自己的对话模型", false, "仅在开启知识库问答后生效。"),
      ],
    },
  ],
};

export function canonicalSettingsPaths(tab: CanonicalSettingsTab): string[] {
  const paths = canonicalSettingsGroups[tab].flatMap((group) => group.fields.map((item) => item.path));
  if (tab === "search") paths.push("blog_extensions.search.provider");
  return paths;
}

export function effectiveSearchProvider(themeProvider: unknown, blogProvider: unknown): string {
  if (blogProvider === "go") return "go";
  if (themeProvider === null) return "none";
  return typeof themeProvider === "string" ? themeProvider : "local";
}

export function searchProviderChanges(provider: string): [string, unknown][] {
  return provider === "go"
    ? [["blog_extensions.search.provider", "go"]]
    : [
        ["blog_extensions.search.provider", undefined],
        ["theme.search.provider", provider === "none" ? null : provider],
      ];
}
