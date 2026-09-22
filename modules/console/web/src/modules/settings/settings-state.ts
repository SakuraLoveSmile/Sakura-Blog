export const ALL_SETTINGS_SECTIONS = [
  "site",
  "menubar",
  "rightbar",
  "footer",
  "comments",
  "article",
  "search",
  "site_tree",
  "wiki",
  "friends",
  "seo",
  "inject",
  "dashboard",
  "style",
  "widgets",
  "ai",
  "cover",
  "nav_tabs",
  "authors",
  "plugins",
  "tag_plugins",
  "default",
  "pages",
  "data_cache",
  "services",
  "feedback",
] as const;

export type SettingsData = Record<string, any>;

/**
 * Keep the legacy settings shape safe for the structured editors.
 * This only fills missing in-memory defaults; the backend section names and
 * persisted JSON shape remain unchanged.
 */
export function ensureSettingsDefaults(d: SettingsData): SettingsData {
  d.site = d.site || {};
  d.menubar = d.menubar || {};
  d.menubar.items = d.menubar.items || [];
  d.rightbar = d.rightbar || {};
  d.rightbar.welcome_linklist = d.rightbar.welcome_linklist || {};
  d.rightbar.welcome_linklist.columns = d.rightbar.welcome_linklist.columns || 0;
  d.rightbar.welcome_linklist.items = d.rightbar.welcome_linklist.items || [];
  d.footer = d.footer || {};
  d.footer.social = d.footer.social || [];
  d.footer.sitemap = d.footer.sitemap || [];
  d.comments = d.comments || {};
  d.comments.notify = d.comments.notify || {};
  d.site_tree = d.site_tree || {};
  d.site_tree.home = d.site_tree.home || {};
  d.site_tree.home.comments = d.site_tree.home.comments ?? false;
  d.article = d.article || {};
  d.article.copyright = d.article.copyright || {};
  d.article.reward = d.article.reward || {};
  d.article.share = d.article.share || {};
  d.article.share.platforms = d.article.share.platforms || [];
  d.article.read_next = d.article.read_next ?? true;
  d.article.pin_style = d.article.pin_style || "carousel";
  d.article.reading_time = d.article.reading_time ?? false;
  d.article.card_tags = d.article.card_tags ?? false;
  d.article.tags = d.article.tags ?? true;
  d.article.type = d.article.type || "tech";
  d.article.banner_ratio = d.article.banner_ratio ?? 0;
  d.article.auto_excerpt = d.article.auto_excerpt ?? 0;
  d.article.card_style = d.article.card_style ?? "hero";
  d.article.category_color = d.article.category_color || {};
  d.article.category_icon = d.article.category_icon || {};
  d.article.tag_icon = d.article.tag_icon || {};
  d.article.ai_label = d.article.ai_label || {};
  d.article.ai_label.default = d.article.ai_label.default || "";
  for (const tier of ["manual", "reviewed", "polished", "generated"] as const) {
    d.article.ai_label[tier] = d.article.ai_label[tier] || {};
    d.article.ai_label[tier].color = d.article.ai_label[tier].color || "";
    d.article.ai_label[tier].icon = d.article.ai_label[tier].icon || "";
  }
  d.article.edit_this_page = d.article.edit_this_page || {};
  d.friends = d.friends || {};
  d.friends.refresh_interval = d.friends.refresh_interval ?? 60;
  d.friends.posts_per_link = d.friends.posts_per_link ?? 20;
  d.friends.check_interval = d.friends.check_interval ?? 360;
  d.friends.snapshot_interval = d.friends.snapshot_interval ?? 1440;
  d.friends.snapshot_provider = d.friends.snapshot_provider || "https://image.thum.io/get/width/1280/crop/720/{url}";
  d.seo = d.seo || {};
  d.seo.preconnect = d.seo.preconnect || [];
  d.seo.canonical = d.seo.canonical || {};
  d.seo.canonical.officialHosts = d.seo.canonical.officialHosts || [];
  d.seo.open_graph = d.seo.open_graph || {};
  d.seo.open_graph.enable = d.seo.open_graph.enable ?? true;
  d.seo.structured_data = d.seo.structured_data || {};
  d.seo.structured_data.links = d.seo.structured_data.links || [];
  d.search = d.search || {};
  d.search.service = d.search.service || "local_search";
  d.search.local_search = d.search.local_search || {};
  d.search.local_search.field = d.search.local_search.field || "all";
  d.search.local_search.path = d.search.local_search.path || "/search.json";
  d.search.local_search.content = d.search.local_search.content ?? true;
  d.search.local_search.skip_search = d.search.local_search.skip_search || [];
  d.search.local_search.lazy_load = d.search.local_search.lazy_load ?? true;
  d.search.local_search.cache_ttl = d.search.local_search.cache_ttl ?? 86400;
  d.inject = d.inject || {};
  d.data_cache = d.data_cache || {};
  d.data_cache.enable = d.data_cache.enable ?? true;
  d.data_cache.default_ttl = d.data_cache.default_ttl ?? 3600;
  d.data_cache.ttl = d.data_cache.ttl || {};
  d.data_cache.max_entries = d.data_cache.max_entries ?? 200;
  const dataCacheTTLs: Record<string, number> = {
    giscus: 600,
    waline: 600,
    artalk: 600,
    memos: 600,
    "memos-user": 86400,
    sites: 86400,
    friends: 86400,
    friends_and_posts: 86400,
    siteinfo: 86400,
  };
  for (const [name, ttl] of Object.entries(dataCacheTTLs)) {
    d.data_cache.ttl[name] = d.data_cache.ttl[name] ?? ttl;
  }
  d.services = d.services || {};
  d.services.siteinfo = d.services.siteinfo || {};
  d.services.siteinfo.api = d.services.siteinfo.api ?? "";
  d.feedback = d.feedback || {};
  d.feedback.enabled = d.feedback.enabled ?? false;
  d.feedback.api_base = d.feedback.api_base || "";
  d.feedback.app_id = d.feedback.app_id || "";
  d.feedback.app_name = d.feedback.app_name || "";
  d.feedback.app_version = d.feedback.app_version || "";
  d.feedback.side = d.feedback.side || "right";
  d.feedback.launcher_mode = d.feedback.launcher_mode || "orb";
  d.feedback.capture_mode = d.feedback.capture_mode || "viewport";
  d.feedback.launcher_bottom = d.feedback.launcher_bottom || "25%";
  d.feedback.display = d.feedback.display || "front";
  d.ai = d.ai || {};
  d.ai.embedding_model = d.ai.embedding_model || "";
  d.ai.summary_enabled = d.ai.summary_enabled ?? true;
  d.ai.allow_user_api = d.ai.allow_user_api ?? true;
  d.ai.qa_enabled = d.ai.qa_enabled ?? false;
  d.ai.qa_allow_user_llm = d.ai.qa_allow_user_llm ?? true;
  const rawWiki = Array.isArray(d.wiki) ? d.wiki : [];
  d.wiki = rawWiki.map((w: any) => {
    const tags = Array.isArray(w?.tags)
      ? w.tags
      : String(w?.tags ?? "")
          .split(/[,，]/)
          .map((s: string) => s.trim())
          .filter(Boolean);
    return { ...w, hero: w?.hero || {}, _tagsText: tags.join(",") };
  });
  d.pages = d.pages || {};
  d.pages.friends = d.pages.friends || {};
  d.style = normalizeStyleSettings(d.style);
  d.widgets = d.widgets || {};
  d.widgets.library = d.widgets.library || {};
  d.widgets.pages = d.widgets.pages || {};
  d.plugins = d.plugins || {};
  d.plugins.adaptive_text = d.plugins.adaptive_text || {};
  d.plugins.adaptive_text.enable = d.plugins.adaptive_text.enable ?? true;
  d.plugins.card_hover = d.plugins.card_hover || {};
  d.plugins.card_hover.enable = d.plugins.card_hover.enable ?? false;
  d.plugins.card_hover.spotlight_color = d.plugins.card_hover.spotlight_color || "rgba(255, 255, 255, 0.25)";
  d.plugins.card_hover.max_tilt = d.plugins.card_hover.max_tilt ?? 3;
  d.plugins.preload = d.plugins.preload || {};
  d.plugins.scrollreveal = d.plugins.scrollreveal || {};
  d.plugins.heti = d.plugins.heti || {};
  d.plugins.fancybox = d.plugins.fancybox || {};
  d.plugins.fancybox.enable = d.plugins.fancybox.enable ?? true;
  d.plugins.swiper = d.plugins.swiper || {};
  d.plugins.katex = d.plugins.katex || {};
  d.plugins.mathjax = d.plugins.mathjax || {};
  d.plugins.mermaid = d.plugins.mermaid || {};
  d.plugins.copycode = d.plugins.copycode || {};
  d.plugins.swiper.enable = d.plugins.swiper.enable ?? true;
  d.plugins.katex.enable = d.plugins.katex.enable ?? true;
  d.plugins.mathjax.enable = d.plugins.mathjax.enable ?? false;
  d.plugins.mermaid.enable = d.plugins.mermaid.enable ?? true;
  d.plugins.copycode.enable = d.plugins.copycode.enable ?? true;
  const tagPlugins = (d.tag_plugins = d.tag_plugins || {});
  for (const key of [
    "note",
    "checkbox",
    "quot",
    "emoji",
    "icon",
    "button",
    "mark",
    "hashtag",
    "gallery",
    "timeline",
    "copy",
    "okr",
  ]) {
    tagPlugins[key] = tagPlugins[key] || {};
  }
  tagPlugins.note.default_color = tagPlugins.note.default_color ?? "";
  tagPlugins.note.border = tagPlugins.note.border ?? true;
  tagPlugins.checkbox.interactive = tagPlugins.checkbox.interactive ?? false;
  const legacyQuot = tagPlugins.quot;
  if (
    !legacyQuot.default &&
    (Object.prototype.hasOwnProperty.call(legacyQuot, "prefix") ||
      Object.prototype.hasOwnProperty.call(legacyQuot, "suffix"))
  ) {
    legacyQuot.default = { prefix: legacyQuot.prefix ?? "", suffix: legacyQuot.suffix ?? "" };
  }
  legacyQuot.default = {
    prefix: legacyQuot.default?.prefix ?? "quot:quote-left",
    suffix: legacyQuot.default?.suffix ?? "quot:quote-right",
  };
  legacyQuot.hashtag = {
    prefix: legacyQuot.hashtag?.prefix ?? "quot:hashtag",
    suffix: legacyQuot.hashtag?.suffix ?? "",
  };
  legacyQuot.question = {
    prefix: legacyQuot.question?.prefix ?? "quot:question",
    suffix: legacyQuot.question?.suffix ?? "",
  };
  tagPlugins.emoji = tagPlugins.emoji || {};
  const emojiSources: Record<string, string> = {
    default: "https://gcore.jsdelivr.net/gh/cdn-x/emoticons@3.1/qq/{name}.gif",
    twemoji: "https://gcore.jsdelivr.net/gh/twitter/twemoji/assets/svg/{name}.svg",
    qq: "https://gcore.jsdelivr.net/gh/cdn-x/emoticons@3.1/qq/{name}.gif",
    aru: "https://gcore.jsdelivr.net/gh/cdn-x/emoticons@3.1/aru/{name}.gif",
    tieba: "https://gcore.jsdelivr.net/gh/cdn-x/emoticons@3.1/tieba/{name}.png",
    blobcat: "https://gcore.jsdelivr.net/gh/cdn-x/emoticons@3.1/blobcat/{name}.gif",
  };
  for (const [name, source] of Object.entries(emojiSources)) {
    tagPlugins.emoji[name] = tagPlugins.emoji[name] ?? source;
  }
  tagPlugins.okr = tagPlugins.okr || {};
  tagPlugins.okr.border = tagPlugins.okr.border ?? true;
  tagPlugins.okr.status = tagPlugins.okr.status || {};
  const okrStatuses: Record<string, { color: string; label: string }> = {
    in_track: { color: "blue", label: "正常" },
    at_risk: { color: "yellow", label: "风险" },
    off_track: { color: "orange", label: "延期" },
    finished: { color: "green", label: "已完成" },
    unfinished: { color: "red", label: "未完成" },
  };
  for (const [name, status] of Object.entries(okrStatuses)) {
    tagPlugins.okr.status[name] = {
      color: tagPlugins.okr.status[name]?.color ?? status.color,
      label: tagPlugins.okr.status[name]?.label ?? status.label,
    };
  }
  d.default = d.default || {};
  d.default.image_onerror = d.default.image_onerror || "";
  d.cover = d.cover || {};
  d.cover.features = d.cover.features || [];
  d.nav_tabs = d.nav_tabs || {};
  d.nav_tabs.items = d.nav_tabs.items || [];
  d.authors = d.authors || {};
  return d;
}

export function deepCloneJsonSafe<T>(val: T): T {
  if (val === null || typeof val !== "object") return val;
  if (Array.isArray(val)) {
    return val.map((item) => deepCloneJsonSafe(item)) as unknown as T;
  }
  const copy: any = {};
  for (const key of Object.keys(val)) {
    copy[key] = deepCloneJsonSafe((val as any)[key]);
  }
  return copy;
}

/**
 * 样式设置规范化函数。
 * 纯函数：输入任意原始 style（包括 null/undefined/非对象/缺键/脏数据/深冻结对象），
 * 不修改原入参对象，输出结构完整、类型安全的规范化新对象。
 * 幂等性保证：normalizeStyleSettings(normalizeStyleSettings(x)) 结果完全一致。
 * 规范化规则：
 * 1. 保留所有未知自定义键与空值（""、0、false 严格保留，不被默认值替换）。
 * 2. header_prefix 必须为对象，若原值为空/数组/primitive 则重置为 {}；确保 h2..h5 为 string（缺省为 ""）。
 * 3. leftbar 必须为对象，ui_style 缺省为 "card"；blur_px 若为数字则转为 px 串（n>0 ? `${n}px` : ""），缺省为 ""。
 * 4. page_transition 必须为对象，enable 为布尔值（保留显式 false，缺省为 true）。
 * 5. codeblock 必须为对象，highlightjs_theme 缺省为 ""。
 * 6. border_radius, gradient, animated_avatar, loading, color, font_size, font_family, site 保证为对象。
 * 7. corner_shape 缺省为 "superellipse(1.25)"。
 * 8. error_page 缺省为 ""。
 */
export function normalizeStyleSettings(rawStyle: any): Record<string, any> {
  const style =
    !rawStyle || typeof rawStyle !== "object" || Array.isArray(rawStyle) ? {} : deepCloneJsonSafe(rawStyle);

  if (!style.color || typeof style.color !== "object" || Array.isArray(style.color)) {
    style.color = {};
  }

  if (!style.font_size || typeof style.font_size !== "object" || Array.isArray(style.font_size)) {
    style.font_size = {};
  }

  if (!style.font_family || typeof style.font_family !== "object" || Array.isArray(style.font_family)) {
    style.font_family = {};
  }

  if (!style.leftbar || typeof style.leftbar !== "object" || Array.isArray(style.leftbar)) {
    style.leftbar = {};
  }
  if (style.leftbar.ui_style === undefined || style.leftbar.ui_style === null) {
    style.leftbar.ui_style = "card";
  }
  if (typeof style.leftbar.blur_px === "number") {
    const n = style.leftbar.blur_px;
    style.leftbar.blur_px = n > 0 ? `${n}px` : "";
  } else if (style.leftbar.blur_px === undefined || style.leftbar.blur_px === null) {
    style.leftbar.blur_px = "";
  }

  if (!style.site || typeof style.site !== "object" || Array.isArray(style.site)) {
    style.site = {};
  }

  if (!style.codeblock || typeof style.codeblock !== "object" || Array.isArray(style.codeblock)) {
    style.codeblock = {};
  }
  if (style.codeblock.highlightjs_theme === undefined || style.codeblock.highlightjs_theme === null) {
    style.codeblock.highlightjs_theme = "";
  }

  if (!style.header_prefix || typeof style.header_prefix !== "object" || Array.isArray(style.header_prefix)) {
    style.header_prefix = {};
  }
  for (const k of ["h2", "h3", "h4", "h5"] as const) {
    if (style.header_prefix[k] === undefined || style.header_prefix[k] === null) {
      style.header_prefix[k] = "";
    }
  }

  if (!style.page_transition || typeof style.page_transition !== "object" || Array.isArray(style.page_transition)) {
    style.page_transition = {};
  }
  if (style.page_transition.enable === undefined || style.page_transition.enable === null) {
    style.page_transition.enable = true;
  }

  if (!style.border_radius || typeof style.border_radius !== "object" || Array.isArray(style.border_radius)) {
    style.border_radius = {};
  }

  if (!style.gradient || typeof style.gradient !== "object" || Array.isArray(style.gradient)) {
    style.gradient = {};
  }

  if (style.corner_shape === undefined || style.corner_shape === null) {
    style.corner_shape = "superellipse(1.25)";
  }

  if (!style.animated_avatar || typeof style.animated_avatar !== "object" || Array.isArray(style.animated_avatar)) {
    style.animated_avatar = {};
  }

  if (!style.loading || typeof style.loading !== "object" || Array.isArray(style.loading)) {
    style.loading = {};
  }

  if (style.error_page === undefined || style.error_page === null) {
    style.error_page = "";
  }

  return style;
}

export function sortWikiProjects(list: any[]): any[] {
  return [...list].sort((a, b) => {
    const sa = Number(a?.sort) || 0;
    const sb = Number(b?.sort) || 0;
    if (sa !== sb) return sb - sa;
    return String(a?.id ?? "").localeCompare(String(b?.id ?? ""));
  });
}
