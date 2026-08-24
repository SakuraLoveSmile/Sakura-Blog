<script setup lang="ts">
import { ref, computed, onMounted, watch, defineAsyncComponent } from "vue";
import { useRoute } from "vue-router";
import { settingsApi, previewApi } from "@/api";
import { Icon } from "@iconify/vue";
// Spec 62：tab 组件按需加载——设置页全量静态打包含 8 个 tab，多数 tab 极少打开
const MenubarEditor = defineAsyncComponent(() => import("./MenubarEditor.vue"));
const AppearanceTab = defineAsyncComponent(() => import("./AppearanceTab.vue"));
const WidgetsTab = defineAsyncComponent(() => import("./WidgetsTab.vue"));
const ThemeYamlTab = defineAsyncComponent(() => import("./ThemeYamlTab.vue"));
const PluginsTab = defineAsyncComponent(() => import("./PluginsTab.vue"));
const CoverTab = defineAsyncComponent(() => import("./CoverTab.vue"));
const NavTabsTab = defineAsyncComponent(() => import("./NavTabsTab.vue"));
const AuthorsTab = defineAsyncComponent(() => import("./AuthorsTab.vue"));
import AttachmentField from "@/components/AttachmentField.vue";
import IconPicker from "@/components/IconPicker.vue";
import { VPageHeader, VSwitch, VCard, VButton, Toast } from "@/components/ui";

// 全部段结构化编辑；「高级·JSON」保留 JSON 兜底（一般无需使用）。
// 两级分组导航：第一级为左侧子导航（大分组），第二级为右侧内容内的小分段。
const groups = [
  {
    key: "site",
    label: "站点",
    icon: "ri:home-4-line",
    desc: "配置站点基本信息、菜单与文案",
    tabs: [
      { key: "site", label: "站点" },
      { key: "menubar", label: "菜单" },
      { key: "rightbar", label: "右栏" },
      { key: "footer", label: "页脚" },
      { key: "default", label: "素材" },
    ],
  },
  {
    key: "appearance",
    label: "外观",
    icon: "ri:palette-line",
    desc: "自定义主题、Hero 与组件编排",
    tabs: [
      { key: "appearance", label: "外观" },
      { key: "theme_yaml", label: "主题 YAML" },
      { key: "cover", label: "首页 Hero" },
      { key: "nav_tabs", label: "导航标签" },
      { key: "widgets", label: "组件编排" },
      { key: "plugins", label: "插件" },
    ],
  },
  {
    key: "content",
    label: "内容",
    icon: "ri:article-line",
    desc: "管理文章、评论、文库与作者",
    tabs: [
      { key: "article", label: "文章" },
      { key: "comments", label: "评论" },
      { key: "notebook", label: "笔记本" },
      { key: "authors", label: "作者" },
      { key: "tag_plugins", label: "标签插件" },
    ],
  },
  {
    key: "system",
    label: "系统",
    icon: "ri:settings-4-line",
    desc: "SEO、代码注入、AI 与安全",
    tabs: [
      { key: "seo", label: "SEO" },
      { key: "search", label: "搜索" },
      { key: "inject", label: "代码注入" },
      { key: "ai", label: "AI" },
      { key: "security", label: "安全" },
      { key: "data_cache", label: "数据缓存" },
      { key: "advanced", label: "高级·JSON" },
    ],
  },
];
const allSections = [
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
  "notebook",
  "tag_plugins",
  "default",
  "pages",
  "data_cache",
];
const platformOptions = ["wechat", "weibo", "email", "link", "twitter", "telegram"];
// 标签插件配色选项（对齐原版 colorful 色板）
const tagColorOptions = ["red", "orange", "yellow", "green", "cyan", "blue", "purple"];

const route = useRoute();
const active = ref("site");
const activeGroup = ref(groups[0].key);
const data = ref<Record<string, any>>({});
const rawText = ref<Record<string, string>>({});
const loading = ref(true);
const loadError = ref("");
const savingKey = ref("");

function ensureDefaults(d: Record<string, any>) {
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
  // read_next 缺省与后端默认一致（true）；旧库 article 段可能缺键
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
  d.inject = d.inject || {};
  // data_cache：动态数据客户端缓存（enable 缺省 true 与 base.html 注入默认一致）
  d.data_cache = d.data_cache || {};
  d.data_cache.enable = d.data_cache.enable ?? true;
  d.data_cache.default_ttl = d.data_cache.default_ttl ?? 3600;
  d.data_cache.ttl = d.data_cache.ttl || {};
  d.ai = d.ai || {};
  d.ai.summary_enabled = d.ai.summary_enabled ?? true;
  d.ai.allow_user_api = d.ai.allow_user_api ?? true;
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
  d.style = d.style || {};
  d.style.color = d.style.color || {};
  d.style.font_size = d.style.font_size || {};
  d.style.font_family = d.style.font_family || {};
  d.style.leftbar = d.style.leftbar || {};
  d.style.site = d.style.site || {};
  d.style.codeblock = d.style.codeblock || {};
  // 类型守卫（源头兜底）：存量数据经「高级 JSON」保存后 header_prefix 可能为非对象 primitive，
  // 对 primitive 赋属性会抛 TypeError 导致设置页白屏 → 非对象（含数组）整体重置为 {}
  const hpRaw = d.style.header_prefix;
  if (!hpRaw || typeof hpRaw !== "object" || Array.isArray(hpRaw)) {
    d.style.header_prefix = {};
  }
  d.widgets = d.widgets || {};
  d.widgets.library = d.widgets.library || {};
  d.widgets.pages = d.widgets.pages || {};
  d.plugins = d.plugins || {};
  d.plugins.adaptive_text = d.plugins.adaptive_text || {};
  d.plugins.adaptive_text.enable = d.plugins.adaptive_text.enable ?? true;
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
  // 标签插件默认参数（对齐原版 tag_plugins 段）
  const tp = (d.tag_plugins = d.tag_plugins || {});
  tp.note = tp.note || {};
  tp.quot = tp.quot || {};
  tp.emoji = tp.emoji || {};
  tp.icon = tp.icon || {};
  tp.button = tp.button || {};
  tp.mark = tp.mark || {};
  tp.hashtag = tp.hashtag || {};
  tp.gallery = tp.gallery || {};
  tp.timeline = tp.timeline || {};
  tp.copy = tp.copy || {};
  // 占位素材组（对齐原版 default 段）
  d.default = d.default || {};
  d.default.image_onerror = d.default.image_onerror || "";
  d.style.loading = d.style.loading || {};
  d.style.error_page = d.style.error_page || "";
  d.style.animated_avatar = d.style.animated_avatar || {};
  d.notebook = d.notebook || {};
  d.notebook.auto_excerpt = d.notebook.auto_excerpt ?? 128;
  d.notebook.order_by = d.notebook.order_by || "-updated";
  d.notebook.share = d.notebook.share ?? false;
  d.cover = d.cover || {};
  d.cover.features = d.cover.features || [];
  d.nav_tabs = d.nav_tabs || {};
  d.nav_tabs.items = d.nav_tabs.items || [];
  d.authors = d.authors || {};
  return d;
}

function sortWikiProjects(list: any[]): any[] {
  return [...list].sort((a, b) => {
    const sa = Number(a?.sort) || 0;
    const sb = Number(b?.sort) || 0;
    if (sa !== sb) return sb - sa;
    return String(a?.id ?? "").localeCompare(String(b?.id ?? ""));
  });
}

function wikiPayload() {
  return (data.value.wiki as any[]).map(({ _tagsText, ...w }) => {
    const hero = w.hero || {};
    const hasHero = Boolean(hero.title || hero.description || hero.background);
    return {
      ...w,
      hero: hasHero ? hero : undefined,
      sort: Number(w.sort) || 0,
      tags: String(_tagsText || "")
        .split(/[,，]/)
        .map((s) => s.trim())
        .filter(Boolean),
    };
  });
}

async function load() {
  loading.value = true;
  loadError.value = "";
  try {
    const { data: res } = await settingsApi.all();
    data.value = ensureDefaults({ ...res });
    data.value.wiki = sortWikiProjects(data.value.wiki);
    for (const s of allSections) {
      rawText.value[s] = JSON.stringify(res[s] ?? null, null, 2);
    }
  } catch {
    loadError.value = "加载设置失败";
  } finally {
    loading.value = false;
  }
}

async function save(section: string, payload?: unknown) {
  savingKey.value = section;
  try {
    const body = payload !== undefined ? payload : data.value[section];
    await settingsApi.updateSection(section, body);
    rawText.value[section] = JSON.stringify(body, null, 2);
    Toast.success("已保存，前台已生效");
  } catch {
  } finally {
    savingKey.value = "";
  }
}

async function saveJson(section: string) {
  savingKey.value = "json:" + section;
  try {
    const payload = JSON.parse(rawText.value[section]);
    await settingsApi.updateSection(section, payload);
    if (section === "wiki") {
      const list = Array.isArray(payload) ? payload : [];
      data.value.wiki = sortWikiProjects(
        list.map((w: any) => {
          const tags = Array.isArray(w?.tags)
            ? w.tags
            : String(w?.tags ?? "")
                .split(/[,，]/)
                .map((s: string) => s.trim())
                .filter(Boolean);
          return { ...w, _tagsText: tags.join(",") };
        }),
      );
    } else {
      data.value[section] = payload;
      ensureDefaults(data.value);
    }
    Toast.success("已保存，前台已生效");
  } catch {
  } finally {
    savingKey.value = "";
  }
}

async function saveComments() {
  await save("comments");
  await save("site_tree");
}

const testing = ref(false);
const testResult = ref("");
const testOk = ref(false);
async function testMail() {
  testing.value = true;
  testResult.value = "";
  try {
    const { data: res } = await settingsApi.testNotify();
    if (res.ok) {
      testOk.value = true;
      testResult.value = "测试邮件已发送，请查收";
    } else {
      testOk.value = false;
      testResult.value = res.error || "发送失败";
    }
  } catch {
    testOk.value = false;
    testResult.value = "请求失败";
  } finally {
    testing.value = false;
  }
}

function removeAt(arr: any[], i: number) {
  arr.splice(i, 1);
}
function moveItem(arr: any[], i: number, dir: -1 | 1) {
  const j = i + dir;
  if (j < 0 || j >= arr.length) return;
  [arr[i], arr[j]] = [arr[j], arr[i]];
}

const dragWikiIndex = ref<number | null>(null);
function onWikiDragStart(i: number) {
  dragWikiIndex.value = i;
}
function onWikiDragOver(e: DragEvent, i: number) {
  e.preventDefault();
  if (dragWikiIndex.value === null || dragWikiIndex.value === i) return;
  const arr = data.value.wiki;
  const [moved] = arr.splice(dragWikiIndex.value, 1);
  arr.splice(i, 0, moved);
  dragWikiIndex.value = i;
  syncWikiSort();
}
function onWikiDragEnd() {
  dragWikiIndex.value = null;
}
function syncWikiSort() {
  const arr = data.value.wiki || [];
  arr.forEach((w: any, idx: number) => {
    w.sort = arr.length - idx;
  });
}
function moveWikiItem(i: number, dir: -1 | 1) {
  moveItem(data.value.wiki, i, dir);
  syncWikiSort();
}

function togglePlatform(p: string) {
  const arr = data.value.article.share.platforms as string[];
  const i = arr.indexOf(p);
  if (i >= 0) arr.splice(i, 1);
  else arr.push(p);
}
function addRightbarLink() {
  data.value.rightbar.welcome_linklist.items.push({ title: "", url: "", icon: "" });
}
function addSocial() {
  data.value.footer.social.push({ title: "", url: "", icon: "", icon_key: "" });
}
function addSitemapGroup() {
  data.value.footer.sitemap.push({ title: "", items: [] });
}
function addSitemapItem(g: any) {
  g.items = g.items || [];
  g.items.push({ title: "", url: "" });
}

function addMapEntry(target: "category_color" | "category_icon" | "tag_icon") {
  data.value.article[target] = data.value.article[target] || {};
  let key = "新名称";
  let i = 2;
  while (Object.prototype.hasOwnProperty.call(data.value.article[target], key)) key = `新名称${i++}`;
  data.value.article[target][key] = "";
}

function renameMapEntry(target: "category_color" | "category_icon" | "tag_icon", oldKey: string, event: Event) {
  const nextKey = (event.target as HTMLInputElement).value;
  const map = data.value.article[target] || {};
  if (nextKey === oldKey) return;
  const value = map[oldKey];
  delete map[oldKey];
  map[nextKey || oldKey] = value;
}

function removeMapEntry(target: "category_color" | "category_icon" | "tag_icon", key: string) {
  delete data.value.article[target]?.[key];
}

function updateSkipSearch(event: Event) {
  data.value.search.local_search.skip_search = (event.target as HTMLTextAreaElement).value
    .split(/\r?\n/)
    .map((s: string) => s.trim())
    .filter(Boolean);
}
function addWikiProject() {
  // Spec 62：新增项目生成稳定 id 作 v-for key
  data.value.wiki.push({
    id: crypto.randomUUID(),
    name: "",
    title: "",
    description: "",
    icon: "",
    tags: [],
    url: "",
    repo: "",
    preview: "auto",
    sort: 0,
    hero: {},
    _tagsText: "",
  });
}

const inputCls = "apple-input";
const textareaCls = "apple-textarea";
const codeAreaCls = "apple-textarea code-style";
const selectCls = "apple-select";

function selectGroup(key: string) {
  activeGroup.value = key;
  const group = groups.find((g) => g.key === key);
  if (group) active.value = group.tabs[0].key;
}

const currentGroupTabs = computed(() => groups.find((g) => g.key === activeGroup.value)?.tabs ?? []);
const currentGroup = computed(() => groups.find((g) => g.key === activeGroup.value) ?? groups[0]);
const currentTab = computed(
  () => currentGroupTabs.value.find((t) => t.key === active.value) ?? currentGroupTabs.value[0],
);

function saveActive() {
  switch (active.value) {
    case "menubar":
      save("menubar");
      break;
    case "rightbar":
      save("rightbar");
      break;
    case "footer":
      save("footer");
      break;
    case "wiki":
      save("wiki", wikiPayload());
      break;
    case "comments":
      saveComments();
      break;
    case "article":
      save("article");
      break;
    case "search":
      save("search");
      break;
    case "seo":
      save("seo");
      break;
    case "inject":
      save("inject");
      break;
    case "ai":
      save("ai");
      break;
    case "appearance":
      save("style");
      break;
    case "cover":
      save("cover");
      break;
    case "nav_tabs":
      save("nav_tabs");
      break;
    case "widgets":
      save("widgets");
      break;
    case "plugins":
      save("plugins");
      break;
    case "notebook":
      save("notebook");
      break;
    case "tag_plugins":
      save("tag_plugins");
      break;
    case "default":
      save("default");
      break;
    case "authors":
      save("authors");
      break;
    case "data_cache":
      save("data_cache");
      break;
    case "site":
      save("site");
      break;
    default:
      break;
  }
}

const saveDisabled = computed(
  () => loading.value || !!loadError.value || active.value === "advanced" || savingKey.value !== "",
);
const activeSaving = computed(() => savingKey.value === active.value || savingKey.value === "json:" + active.value);

// ---------- 前端预览（所见即所得） ----------
// 每个设置项映射到具体前台区域：page=渲染哪种前台页，selectors=提取聚焦的 DOM 区域。
// 后端用草稿配置渲染整页 HTML，前端解析后只保留目标区域塞入 iframe，编辑哪项看哪项。
interface PreviewFocus {
  page: string;
  selectors: string[];
  label: string;
}
const PREVIEW_FOCUS: Record<string, PreviewFocus> = {
  site: { page: "index", selectors: [".navbar.top", "footer.page-footer"], label: "顶栏与页脚" },
  menubar: { page: "index", selectors: ["aside.l_left"], label: "左侧菜单" },
  rightbar: { page: "index", selectors: ["aside.l_right"], label: "右栏组件" },
  footer: { page: "index", selectors: ["footer.page-footer"], label: "页脚" },
  lang: { page: "index", selectors: [".navbar.top", ".l_main"], label: "前台文案" },
  appearance: { page: "index", selectors: ["#l_cover", ".l_body"], label: "整体外观" },
  cover: { page: "index", selectors: ["#l_cover"], label: "首页 Hero" },
  nav_tabs: { page: "index", selectors: [".navbar.top"], label: "导航标签" },
  widgets: { page: "index", selectors: ["aside.l_left", "aside.l_right"], label: "左右栏组件" },
  plugins: { page: "post", selectors: [".l_main"], label: "文章页（插件效果）" },
  article: { page: "index", selectors: [".l_main"], label: "文章卡片" },
  comments: { page: "post", selectors: ["#comments"], label: "评论区" },
  wiki: { page: "wiki", selectors: [".l_main"], label: "文库页" },
  authors: { page: "index", selectors: ["aside.l_right"], label: "作者卡片" },
};

const previewOpen = ref(false);
const previewViewport = ref<"desktop" | "tablet" | "mobile">("desktop");
const previewHtml = ref("");
const previewLoading = ref(false);
let previewTimer: ReturnType<typeof setTimeout> | undefined;
let previewSeq = 0;

const previewFocus = computed<PreviewFocus | null>(() => PREVIEW_FOCUS[active.value] ?? null);

// 收集当前内存中的草稿分段（未保存的编辑也参与预览）
function previewSections(): Record<string, unknown> {
  const secs: Record<string, unknown> = {};
  for (const k of allSections) {
    if (data.value[k] !== undefined) secs[k] = data.value[k];
  }
  return secs;
}

// 从整页 HTML 中提取聚焦区域，重组为独立文档（保留原 head 样式，body 仅含目标片段）。
// 同时与上一次渲染结果做 DOM 对比：变化的子元素加 pv-changed 脉冲高亮，
// 区域根元素加 pv-flash 轻闪，让用户立刻看到「改到了哪里」。
let prevRawHtml = "";

// 递归对比同构子元素，把最小变化粒度标记到 newEl 上（属性/文本级变化落在自身）
function diffMark(oldEl: Element, newEl: Element): void {
  if (oldEl.outerHTML === newEl.outerHTML) return;
  const oldKids = Array.from(oldEl.children);
  const newKids = Array.from(newEl.children);
  let descended = false;
  for (let i = 0; i < newKids.length; i++) {
    const o = oldKids[i];
    const n = newKids[i];
    if (o && o.tagName === n.tagName && o.outerHTML !== n.outerHTML) {
      diffMark(o, n);
      descended = true;
    }
  }
  if (!descended) newEl.classList.add("pv-changed");
}

// 过滤预览 head 中的脚本：避免每次编辑（700ms 防抖）刷新预览都重新加载执行
// jQuery/KaTeX/Mermaid CDN 与统计类注入 → 脚本执行风暴 + 重复计数。
// 策略：
// - 外链 <script src>（jQuery/KaTeX×2/Mermaid 及用户注入）全部移除——纯样式预览不需要交互脚本；
// - 内联脚本默认移除：JSON-LD 结构化数据（无视觉影响）、KaTeX/Mermaid 初始化（其 CDN 已移除）、
//   Service Worker 注册（PWA 功能）、Inject.Head 用户代码注入（统计脚本会重复计数）；
// - 保留例外：prefers_theme=auto 的明暗跟随脚本（matchMedia 按系统设置 data-theme，
//   决定首屏深色/浅色主题渲染，影响预览视觉正确性，见 backend head.html L176）。
function filterPreviewHead(head: HTMLHeadElement): void {
  for (const s of Array.from(head.querySelectorAll("script"))) {
    const t = s.textContent || "";
    if (!s.src && t.includes("prefers-color-scheme") && t.includes("data-theme-auto")) continue;
    s.remove();
  }
}

function buildFragmentDoc(fullHtml: string, focus: PreviewFocus): string {
  const doc = new DOMParser().parseFromString(fullHtml, "text/html");
  const prevDoc = prevRawHtml ? new DOMParser().parseFromString(prevRawHtml, "text/html") : null;
  for (const sel of focus.selectors) {
    const el = doc.querySelector(sel);
    if (!el) continue;
    const old = prevDoc?.querySelector(sel);
    if (!old) {
      el.classList.add("pv-flash");
      continue;
    }
    const changed = old.outerHTML !== el.outerHTML;
    if (changed) {
      el.classList.add("pv-flash");
      diffMark(old, el);
    }
  }
  prevRawHtml = fullHtml;

  const frags = focus.selectors.map((sel) => doc.querySelector(sel)?.outerHTML ?? "").filter(Boolean);
  const bodyInner = frags.length ? frags.join("\n") : doc.body.innerHTML;
  // 只保留样式/布局必需节点：link stylesheet、style、meta 原样保留，脚本按 filterPreviewHead 策略过滤
  filterPreviewHead(doc.head);
  const headHtml = doc.head.innerHTML;
  const bodyClass = doc.body.className || "";
  // 主题与前台一致：base.html 按 style.prefers_theme 在 <html> 上预设 data-theme（dark/light/auto），
  // 原样保留该属性；auto 模式的 data-theme-auto + 明暗跟随脚本（filterPreviewHead 中保留）
  // 共同保证预览明暗与前台一致——替代原「强制深色」脚本，CSS 从首帧起即按前台配置渲染
  const themeAttr = doc.documentElement.getAttribute("data-theme");
  const themeAutoAttr = doc.documentElement.hasAttribute("data-theme-auto") ? " data-theme-auto" : "";
  // 变更高亮特效：区域轻闪 + 变化元素脉冲描边
  // 注意：不覆盖 body 背景——前台 main.css 按 data-theme 自绘深浅背景，
  // 控制台变量（--color-background）在前台不存在，注入会把深色底盖成白
  const fx = `<style>
body{padding:20px !important}
.pv-flash{animation:pvFlash 1.1s cubic-bezier(0.32,0.72,0,1) 1;border-radius:10px}
@keyframes pvFlash{0%{background-color:color-mix(in srgb,var(--color-primary, #007AFF) 14%,transparent)}100%{background-color:transparent}}
 .pv-changed{animation:pvPulse 1.8s cubic-bezier(0.32,0.72,0,1) 1;border-radius:6px;outline:2px solid var(--color-primary, #007AFF);outline-offset:3px}
@keyframes pvPulse{0%{box-shadow:0 0 0 0 color-mix(in srgb,var(--color-primary, #007AFF) 55%,transparent)}100%{box-shadow:0 0 0 16px color-mix(in srgb,var(--color-primary, #007AFF) 0%,transparent)}}
</style>`;
  return `<!doctype html><html lang="zh-CN" data-theme="${themeAttr || "dark"}"${themeAutoAttr}><head>${headHtml}${fx}</head><body class="${bodyClass}">${bodyInner}</body></html>`;
}

async function refreshPreview() {
  if (!previewOpen.value || !previewFocus.value) return;
  const focus = previewFocus.value;
  const seq = ++previewSeq;
  previewLoading.value = true;
  try {
    const { data: res } = await previewApi.site(previewSections(), focus.page);
    if (seq !== previewSeq) return;
    previewHtml.value = buildFragmentDoc(res.html || "", focus);
  } catch {
    if (seq === previewSeq) previewHtml.value = "";
  } finally {
    if (seq === previewSeq) previewLoading.value = false;
  }
}

function schedulePreview() {
  // 预览关闭时零开销（deep watch 仅遍历，不发请求）
  if (!previewOpen.value) return;
  clearTimeout(previewTimer);
  // Spec 62：防抖 700→1000ms——预览开启时连续输入（含 AuthorsTab 每键重建）减少整站渲染频率
  previewTimer = setTimeout(refreshPreview, 1000);
}

watch(previewOpen, (open) => {
  if (open) refreshPreview();
});
// 切换设置项 = 切换预览焦点；编辑内容变化 = 防抖重渲染
watch(active, () => {
  if (previewOpen.value) refreshPreview();
});
watch(data, schedulePreview, { deep: true });

onMounted(() => {
  const tab = route.query.tab;
  if (typeof tab === "string") {
    const group = groups.find((g) => g.tabs.some((t) => t.key === tab));
    if (group) {
      activeGroup.value = group.key;
      active.value = tab;
    }
  }
  load();
});
</script>

<template>
  <div class="settings-view" :class="{ 'preview-docked': previewOpen }">
    <VPageHeader title="系统设置">
      <template #icon>
        <Icon icon="ri:settings-3-line" class="text-primary" />
      </template>
      <template #subtitle>站点全局偏好、主题外观定制、内容交互与系统安全配置</template>
      <template #actions>
        <VButton size="sm" :type="previewOpen ? 'primary' : 'secondary'" @click="previewOpen = !previewOpen">
          <template #icon><Icon icon="ri:eye-line" /></template>
          <span class="sm:hidden">预览</span>
          <span class="hidden sm:inline">前端预览</span>
        </VButton>
        <VButton type="primary" size="sm" :disabled="saveDisabled" :loading="activeSaving" @click="saveActive">
          <template #icon><Icon icon="ri:save-line" /></template>
          <span class="sm:hidden">保存</span>
          <span class="hidden sm:inline">保存当前设置</span>
        </VButton>
      </template>
    </VPageHeader>

    <div class="settings-layout">
      <div class="settings-container">
        <nav class="settings-nav">
          <div class="settings-nav-list">
            <button
              v-for="g in groups"
              :key="g.key"
              type="button"
              class="settings-nav-item"
              :class="{ active: activeGroup === g.key }"
              @click="selectGroup(g.key)"
            >
              <span class="settings-nav-item-icon"><Icon :icon="g.icon" width="16" height="16" /></span>
              <span>{{ g.label }}</span>
            </button>
          </div>
        </nav>

        <div class="settings-content">
          <div>
            <div class="settings-section-title">{{ currentTab.label }}</div>
            <div class="settings-section-desc">{{ currentGroup.desc }}</div>
          </div>

          <div v-if="currentGroupTabs.length > 1" class="apple-segmented rect settings-subtabs subtabs-scroll">
            <button
              v-for="t in currentGroupTabs"
              :key="t.key"
              type="button"
              :class="{ active: active === t.key }"
              @click="active = t.key"
            >
              {{ t.label }}
            </button>
          </div>

          <div v-if="loading" class="settings-state">
            <Icon icon="ri:loader-4-line" class="settings-spin" width="22" height="22" />
            <span>加载中...</span>
          </div>
          <div v-else-if="loadError" class="settings-state settings-state-error">
            <Icon icon="ri:error-warning-line" width="22" height="22" />
            <span>设置加载失败：{{ loadError }}</span>
            <VButton type="secondary" size="sm" class="mt-2" @click="load">重试</VButton>
          </div>

          <template v-else>
            <VCard v-show="active === 'site'" title="站点信息">
              <div class="form-grid form-grid-2">
                <div class="field"><label>站点标题</label><input v-model="data.site.title" :class="inputCls" /></div>
                <div class="field"><label>副标题</label><input v-model="data.site.subtitle" :class="inputCls" /></div>
              </div>
              <div class="field">
                <label>站点描述</label
                ><textarea v-model="data.site.description" rows="2" :class="textareaCls"></textarea>
              </div>
              <div class="field">
                <label>站点 URL（canonical/OG/sitemap 用）</label
                ><input v-model="data.site.url" :class="inputCls" placeholder="https://example.com" />
              </div>

              <div class="form-grid form-grid-2">
                <div class="field">
                  <label>头像 URL</label>
                  <AttachmentField v-model="data.site.avatar" placeholder="输入图片 URL" />
                </div>
                <div class="field">
                  <label>头像悬停背景图</label>
                  <AttachmentField v-model="data.site.avatar_bg" placeholder="输入图片 URL" />
                </div>
              </div>

              <div v-if="data.site.avatar || data.site.avatar_bg" class="preview-box">
                <p class="preview-label">实时预览</p>
                <div class="preview-row">
                  <div v-if="data.site.avatar" class="preview-item">
                    <p class="preview-item-label">导航栏头像 32×32</p>
                    <div class="avatar-32"><img :src="data.site.avatar" alt="" /></div>
                  </div>
                  <div v-if="data.site.avatar" class="preview-item">
                    <p class="preview-item-label">分享卡片 128×128</p>
                    <div class="avatar-128"><img :src="data.site.avatar" alt="" /></div>
                  </div>
                  <div v-if="data.site.avatar && data.site.avatar_bg" class="preview-item">
                    <p class="preview-item-label">悬停效果</p>
                    <div class="avatar-hover">
                      <img :src="data.site.avatar_bg" class="avatar-hover-bg" alt="" />
                      <img :src="data.site.avatar" class="avatar-hover-fg" alt="" />
                    </div>
                  </div>
                </div>
              </div>

              <div class="form-grid form-grid-2">
                <div class="field">
                  <label>Logo 主标题</label><input v-model="data.site.logo_title" :class="inputCls" />
                </div>
                <div class="field">
                  <label>Logo 副标题（含 | 拆滚动）</label><input v-model="data.site.logo_sub" :class="inputCls" />
                </div>
              </div>

              <div class="form-grid form-grid-2">
                <div class="field">
                  <label>Favicon</label>
                  <AttachmentField
                    v-model="data.site.favicon_url"
                    placeholder="留空用默认 /favicon.ico"
                    show-thumb
                    fallback-src="/favicon.ico"
                  />
                </div>
                <div class="field">
                  <label>Apple Touch Icon</label>
                  <AttachmentField
                    v-model="data.site.apple_touch_icon"
                    placeholder="留空用默认"
                    show-thumb
                    fallback-src="/apple-touch-icon.png"
                  />
                </div>
              </div>

              <div class="card-actions">
                <VButton type="primary" size="sm" :loading="savingKey === 'site'" @click="save('site')"
                  >保存站点信息</VButton
                >
              </div>
            </VCard>

            <VCard v-show="active === 'menubar'" title="顶部菜单">
              <MenubarEditor
                :items="data.menubar.items"
                :columns="data.menubar.columns"
                @update:columns="data.menubar.columns = $event"
                @save="save('menubar')"
              />
            </VCard>

            <VCard v-show="active === 'rightbar'" title="右栏设置">
              <div class="field">
                <label>欢迎语正文（留空用默认文案，Spec Task 5）</label
                ><textarea
                  v-model="data.rightbar.intro"
                  rows="3"
                  :class="textareaCls"
                  placeholder="输入右栏欢迎语正文，换行会保留显示"
                ></textarea>
              </div>
              <p class="hint">首页右栏「欢迎语」组件内的链接列表（welcome_linklist）。</p>
              <div class="field field-narrow">
                <label>列数（0=自动）</label
                ><input
                  v-model.number="data.rightbar.welcome_linklist.columns"
                  type="number"
                  min="0"
                  :class="inputCls"
                />
              </div>
              <div class="editable-list">
                <div v-for="(it, i) in data.rightbar.welcome_linklist.items" :key="i" class="editable-row">
                  <input v-model="it.title" placeholder="标题" :class="inputCls" class="col-3" />
                  <input v-model="it.url" placeholder="/url 或 https://..." :class="inputCls" class="col-4" />
                  <div class="col-3">
                    <IconPicker v-model="it.icon" compact placeholder="图标 key 或图片 URL" />
                  </div>
                  <div class="row-actions">
                    <button
                      type="button"
                      :disabled="Number(i) === 0"
                      class="row-btn"
                      @click="moveItem(data.rightbar.welcome_linklist.items, Number(i), -1)"
                    >
                      <Icon icon="ri:arrow-up-s-line" />
                    </button>
                    <button
                      type="button"
                      :disabled="Number(i) === data.rightbar.welcome_linklist.items.length - 1"
                      class="row-btn"
                      @click="moveItem(data.rightbar.welcome_linklist.items, Number(i), 1)"
                    >
                      <Icon icon="ri:arrow-down-s-line" />
                    </button>
                    <button
                      type="button"
                      class="row-btn row-btn-danger"
                      @click="removeAt(data.rightbar.welcome_linklist.items, Number(i))"
                    >
                      <Icon icon="ri:delete-bin-line" />
                    </button>
                  </div>
                </div>
              </div>
              <button type="button" class="add-btn" @click="addRightbarLink">
                <Icon icon="ri:add-line" /> 添加链接
              </button>
              <div class="card-actions">
                <VButton type="primary" size="sm" :loading="savingKey === 'rightbar'" @click="save('rightbar')"
                  >保存右栏</VButton
                >
              </div>
            </VCard>

            <VCard v-show="active === 'footer'" title="页脚设置">
              <div class="subsection">
                <p class="hint">
                  社交图标（icon 填 raw HTML 如 &lt;img src="..."&gt;，icon_key 填 icons.yml 的 key，二选一，icon 优先）
                </p>
                <div class="editable-list">
                  <div v-for="(s, i) in data.footer.social" :key="i" class="editable-row">
                    <input v-model="s.title" placeholder="标题" :class="inputCls" class="col-2" />
                    <input v-model="s.url" placeholder="链接" :class="inputCls" class="col-3" />
                    <div class="col-2">
                      <IconPicker v-model="s.icon_key" compact placeholder="icon_key" />
                    </div>
                    <input v-model="s.icon" placeholder="icon raw HTML（优先）" :class="inputCls" class="col-3" />
                    <div class="row-actions">
                      <button
                        type="button"
                        :disabled="Number(i) === 0"
                        class="row-btn"
                        @click="moveItem(data.footer.social, Number(i), -1)"
                      >
                        <Icon icon="ri:arrow-up-s-line" />
                      </button>
                      <button
                        type="button"
                        :disabled="Number(i) === data.footer.social.length - 1"
                        class="row-btn"
                        @click="moveItem(data.footer.social, Number(i), 1)"
                      >
                        <Icon icon="ri:arrow-down-s-line" />
                      </button>
                      <button
                        type="button"
                        class="row-btn row-btn-danger"
                        @click="removeAt(data.footer.social, Number(i))"
                      >
                        <Icon icon="ri:delete-bin-line" />
                      </button>
                    </div>
                  </div>
                </div>
                <button type="button" class="add-btn" @click="addSocial">
                  <Icon icon="ri:add-line" /> 添加社交图标
                </button>
              </div>
              <div class="subsection">
                <p class="hint">站点地图分组（页脚多列链接）</p>
                <div v-for="(g, gi) in data.footer.sitemap" :key="gi" class="nested-group">
                  <div class="nested-group-head">
                    <input v-model="g.title" placeholder="组名" :class="inputCls" class="flex-1" />
                    <button
                      type="button"
                      :disabled="Number(gi) === 0"
                      class="row-btn"
                      @click="moveItem(data.footer.sitemap, Number(gi), -1)"
                    >
                      <Icon icon="ri:arrow-up-s-line" />
                    </button>
                    <button
                      type="button"
                      :disabled="Number(gi) === data.footer.sitemap.length - 1"
                      class="row-btn"
                      @click="moveItem(data.footer.sitemap, Number(gi), 1)"
                    >
                      <Icon icon="ri:arrow-down-s-line" />
                    </button>
                    <button
                      type="button"
                      class="row-btn row-btn-danger"
                      @click="removeAt(data.footer.sitemap, Number(gi))"
                    >
                      <Icon icon="ri:delete-bin-line" />
                    </button>
                  </div>
                  <div v-for="(it, ii) in g.items" :key="ii" class="editable-row nested-row">
                    <input v-model="it.title" placeholder="标题" :class="inputCls" class="col-4" />
                    <input v-model="it.url" placeholder="/url" :class="inputCls" class="col-6" />
                    <div class="row-actions">
                      <button
                        type="button"
                        :disabled="Number(ii) === 0"
                        class="row-btn"
                        @click="moveItem(g.items, Number(ii), -1)"
                      >
                        <Icon icon="ri:arrow-up-s-line" />
                      </button>
                      <button
                        type="button"
                        :disabled="Number(ii) === g.items.length - 1"
                        class="row-btn"
                        @click="moveItem(g.items, Number(ii), 1)"
                      >
                        <Icon icon="ri:arrow-down-s-line" />
                      </button>
                      <button type="button" class="row-btn row-btn-danger" @click="removeAt(g.items, Number(ii))">
                        <Icon icon="ri:delete-bin-line" />
                      </button>
                    </div>
                  </div>
                  <button type="button" class="add-btn add-btn-indent" @click="addSitemapItem(g)">
                    <Icon icon="ri:add-line" /> 添加链接
                  </button>
                </div>
                <button type="button" class="add-btn" @click="addSitemapGroup">
                  <Icon icon="ri:add-line" /> 添加分组
                </button>
              </div>
              <div class="subsection">
                <p class="subsection-title">
                  自定义页脚文案（Markdown，对齐原版 footer.content；空=内置 Powered-by 行）
                </p>
                <textarea
                  v-model="data.footer.content"
                  rows="3"
                  :class="textareaCls"
                  placeholder="本站由 [作者](/) 使用 [Stellar](https://github.com/xaoxuu/hexo-theme-stellar) 主题创建。"
                ></textarea>
              </div>
              <div class="card-actions">
                <VButton type="primary" size="sm" :loading="savingKey === 'footer'" @click="save('footer')"
                  >保存页脚</VButton
                >
              </div>
            </VCard>

            <VCard v-show="active === 'default'" title="占位素材">
              <p class="hint">各场景占位图 URL（对齐原版 default 段）；留空回退内置默认素材。</p>
              <div class="form-grid form-grid-2">
                <div class="field"><label>头像</label><AttachmentField v-model="data.default.avatar" /></div>
                <div class="field"><label>链接卡片图标</label><AttachmentField v-model="data.default.link" /></div>
                <div class="field"><label>封面</label><AttachmentField v-model="data.default.cover" /></div>
                <div class="field"><label>正文插图（预留）</label><AttachmentField v-model="data.default.image" /></div>
                <div class="field field-wide">
                  <label>图片加载失败占位图（image_onerror）</label
                  ><AttachmentField v-model="data.default.image_onerror" placeholder="可填图片 URL 或 data URI" />
                </div>
                <div class="field">
                  <label>项目卡片（预留）</label><AttachmentField v-model="data.default.project" />
                </div>
                <div class="field"><label>横幅（预留）</label><AttachmentField v-model="data.default.banner" /></div>
                <div class="field"><label>专栏</label><AttachmentField v-model="data.default.topic" /></div>
                <div class="field"><label>加载图标</label><AttachmentField v-model="data.default.loading" /></div>
              </div>
              <div class="card-actions">
                <VButton type="primary" size="sm" :loading="savingKey === 'default'" @click="save('default')"
                  >保存素材</VButton
                >
              </div>
            </VCard>

            <VCard v-show="active === 'notebook'" title="笔记本设置">
              <p class="hint">对齐 Stellar notebook 段；具体笔记本/front-matter 可覆盖这些默认值。</p>
              <div class="form-grid form-grid-2">
                <div class="field">
                  <label>自动摘要长度（字符）</label
                  ><input v-model.number="data.notebook.auto_excerpt" type="number" min="0" :class="inputCls" />
                </div>
                <div class="field">
                  <label>排序字段</label
                  ><input v-model="data.notebook.order_by" :class="inputCls" placeholder="-updated" />
                </div>
              </div>
              <div class="switch-row">
                <div class="switch-row-label"><div class="switch-row-name">显示分享按钮</div></div>
                <VSwitch v-model="data.notebook.share" />
              </div>
              <div class="field">
                <label>许可协议（文本或 false）</label
                ><input
                  :value="typeof data.notebook.license === 'string' ? data.notebook.license : ''"
                  :class="inputCls"
                  placeholder="留空或输入许可文案"
                  @input="data.notebook.license = ($event.target as HTMLInputElement).value"
                />
              </div>
              <div class="card-actions">
                <VButton type="primary" size="sm" :loading="savingKey === 'notebook'" @click="save('notebook')"
                  >保存笔记本设置</VButton
                >
              </div>
            </VCard>

            <VCard v-show="active === 'wiki'" title="文库项目">
              <p class="hint">/wiki 页项目卡片。拖拽排序；tags 逗号分隔。</p>
              <div
                v-for="(w, i) in data.wiki"
                :key="w.id"
                draggable="true"
                class="wiki-card"
                :class="{ 'wiki-card-dragging': dragWikiIndex === Number(i) }"
                @dragstart="onWikiDragStart(Number(i))"
                @dragover="onWikiDragOver($event, Number(i))"
                @dragend="onWikiDragEnd"
              >
                <div class="wiki-card-head">
                  <Icon icon="ri:drag-move-2-line" class="drag-handle" />
                  <div v-if="w.icon" class="wiki-icon">
                    <img :src="w.icon" alt="" @error="($event.target as HTMLImageElement).style.display = 'none'" />
                  </div>
                  <div v-else class="wiki-icon wiki-icon-empty"><Icon icon="ri:booklet-line" /></div>
                  <span class="wiki-title">{{ w.title || w.name || w.id || "新项目" }}</span>
                  <div class="row-actions">
                    <button
                      type="button"
                      :disabled="Number(i) === 0"
                      class="row-btn"
                      title="上移"
                      @click="moveWikiItem(Number(i), -1)"
                    >
                      <Icon icon="ri:arrow-up-s-line" />
                    </button>
                    <button
                      type="button"
                      :disabled="Number(i) === data.wiki.length - 1"
                      class="row-btn"
                      title="下移"
                      @click="moveWikiItem(Number(i), 1)"
                    >
                      <Icon icon="ri:arrow-down-s-line" />
                    </button>
                    <button
                      type="button"
                      class="row-btn row-btn-danger"
                      title="删除"
                      @click="removeAt(data.wiki, Number(i))"
                    >
                      <Icon icon="ri:delete-bin-line" />
                    </button>
                  </div>
                </div>
                <div class="form-grid form-grid-3">
                  <div class="field"><label>id</label><input v-model="w.id" :class="inputCls" /></div>
                  <div class="field"><label>标题</label><input v-model="w.title" :class="inputCls" /></div>
                  <div class="field">
                    <label>排序（大在前）</label><input v-model.number="w.sort" type="number" :class="inputCls" />
                  </div>
                </div>
                <div class="field"><label>描述</label><input v-model="w.description" :class="inputCls" /></div>
                <div class="form-grid form-grid-3">
                  <div class="field">
                    <label>图标（图片 URL）</label><AttachmentField v-model="w.icon" placeholder="https://..." />
                  </div>
                  <div class="field">
                    <label>GitHub 仓库</label><input v-model="w.repo" :class="inputCls" placeholder="owner/repo" />
                  </div>
                  <div class="field">
                    <label>预览布局</label>
                    <select v-model="w.preview" :class="selectCls">
                      <option value="auto">auto（自动）</option>
                      <option value="wide">wide（宽版）</option>
                      <option value="none">none（无预览）</option>
                    </select>
                  </div>
                </div>
                <div class="field">
                  <label>链接 URL</label>
                  <div class="field-inline">
                    <input v-model="w.url" :class="inputCls" placeholder="/posts/xxx 或 https://..." />
                    <button
                      type="button"
                      class="icon-btn"
                      :disabled="!w.id"
                      title="链接到 wiki 项目文档"
                      @click="w.url = '/wiki/' + w.id"
                    >
                      <Icon icon="ri:link-m" />
                    </button>
                  </div>
                </div>
                <div class="field">
                  <label>标签（逗号分隔）</label
                  ><input v-model="w._tagsText" :class="inputCls" placeholder="Go, 开源" />
                </div>
                <div class="form-grid form-grid-3">
                  <div class="field">
                    <label>hero 标题</label
                    ><input v-model="w.hero.title" :class="inputCls" placeholder="留空不渲染 hero" />
                  </div>
                  <div class="field">
                    <label>hero 副标题</label><input v-model="w.hero.description" :class="inputCls" />
                  </div>
                  <div class="field">
                    <label>hero 背景图</label><AttachmentField v-model="w.hero.background" placeholder="https://..." />
                  </div>
                </div>
              </div>
              <button type="button" class="add-btn" @click="addWikiProject">
                <Icon icon="ri:add-line" /> 添加项目
              </button>
              <div class="card-actions">
                <VButton type="primary" size="sm" :loading="savingKey === 'wiki'" @click="save('wiki', wikiPayload())"
                  >保存文库</VButton
                >
              </div>
            </VCard>

            <VCard v-show="active === 'comments'" title="评论设置">
              <div class="switch-row">
                <div class="switch-row-label">
                  <div class="switch-row-name">首页显示评论区</div>
                  <div class="switch-row-desc">对齐 site_tree.home.comments；文章页评论不受此开关影响</div>
                </div>
                <VSwitch v-model="data.site_tree.home.comments" />
              </div>
              <div class="switch-row">
                <div class="switch-row-label">
                  <div class="switch-row-name">启用评论</div>
                  <div class="switch-row-desc">关闭需重启才断开路由</div>
                </div>
                <VSwitch v-model="data.comments.enabled" />
              </div>
              <div class="switch-row">
                <div class="switch-row-label">
                  <div class="switch-row-name">先审后发</div>
                  <div class="switch-row-desc">新评论需审核后才显示</div>
                </div>
                <VSwitch v-model="data.comments.moderation" />
              </div>
              <div class="form-grid form-grid-2">
                <div class="field">
                  <label>评论区标题</label><input v-model="data.comments.title" :class="inputCls" />
                </div>
                <div class="field">
                  <label>博主徽章文案</label><input v-model="data.comments.admin_badge" :class="inputCls" />
                </div>
              </div>
              <div class="form-grid form-grid-2">
                <div class="field">
                  <label>博主邮箱</label><input v-model="data.comments.admin_email" :class="inputCls" />
                </div>
                <div class="field">
                  <label>Gravatar 镜像</label><input v-model="data.comments.gravatar_mirror" :class="inputCls" />
                </div>
              </div>
              <div class="subsection">
                <p class="subsection-title">邮件通知（SMTP，主机留空则关闭）</p>
                <div class="form-grid form-grid-2">
                  <div class="field">
                    <label>SMTP 主机</label><input v-model="data.comments.notify.smtp_host" :class="inputCls" />
                  </div>
                  <div class="field">
                    <label>端口</label
                    ><input v-model.number="data.comments.notify.smtp_port" type="number" :class="inputCls" />
                  </div>
                  <div class="field">
                    <label>用户名</label><input v-model="data.comments.notify.username" :class="inputCls" />
                  </div>
                  <div class="field">
                    <label>密码</label
                    ><input v-model="data.comments.notify.password" type="password" :class="inputCls" />
                  </div>
                  <div class="field">
                    <label>发件人（留空用用户名）</label><input v-model="data.comments.notify.from" :class="inputCls" />
                  </div>
                </div>
              </div>
              <div class="card-actions card-actions-inline">
                <VButton
                  type="primary"
                  size="sm"
                  :loading="savingKey === 'comments' || savingKey === 'site_tree'"
                  @click="saveComments"
                  >保存评论设置</VButton
                >
                <VButton type="secondary" size="sm" :disabled="testing" @click="testMail">{{
                  testing ? "发送中..." : "发送测试邮件"
                }}</VButton>
                <span v-if="testResult" class="test-result" :class="testOk ? 'is-ok' : 'is-err'">{{ testResult }}</span>
              </div>
            </VCard>

            <VCard v-show="active === 'article'" title="文章设置">
              <div class="subsection">
                <p class="subsection-title">排版（对齐原版 article 段）</p>
                <div class="switch-row">
                  <div class="switch-row-label"><div class="switch-row-name">正文段落首行缩进两字符</div></div>
                  <VSwitch v-model="data.article.indent" />
                </div>
                <div class="switch-row">
                  <div class="switch-row-label">
                    <div class="switch-row-name">上下篇导航（read_next）</div>
                    <div class="switch-row-desc">文章页底部「接下来阅读」较新/较早文章卡片</div>
                  </div>
                  <VSwitch v-model="data.article.read_next" />
                </div>
                <div class="form-grid form-grid-3">
                  <div class="field">
                    <label>置顶文章样式</label
                    ><select v-model="data.article.pin_style" :class="selectCls">
                      <option value="carousel">轮播</option>
                      <option value="flat">平铺到列表顶部</option>
                    </select>
                  </div>
                  <div class="switch-row">
                    <div class="switch-row-label"><div class="switch-row-name">显示字数与阅读时长</div></div>
                    <VSwitch v-model="data.article.reading_time" />
                  </div>
                  <div class="switch-row">
                    <div class="switch-row-label">
                      <div class="switch-row-name">文章卡片显示标签</div>
                      <div class="switch-row-desc">最多显示 5 个</div>
                    </div>
                    <VSwitch v-model="data.article.card_tags" />
                  </div>
                  <div class="switch-row">
                    <div class="switch-row-label">
                      <div class="switch-row-name">文章末尾显示标签</div>
                      <div class="switch-row-desc">正文后、页脚前的标签行</div>
                    </div>
                    <VSwitch v-model="data.article.tags" />
                  </div>
                </div>
                <div class="form-grid form-grid-3">
                  <div class="field">
                    <label>卡片封面宽高比（0=主题默认，如 2）</label
                    ><input
                      v-model.number="data.article.cover_ratio"
                      type="number"
                      step="0.1"
                      min="0"
                      :class="inputCls"
                    />
                  </div>
                  <div class="field">
                    <label>相关文章数上限（0=关）</label
                    ><input v-model.number="data.article.related_posts" type="number" min="0" :class="inputCls" />
                  </div>
                  <div class="field">
                    <label>过期提醒阈值（天，0=缺省 180）</label
                    ><input v-model.number="data.article.expire_days" type="number" min="0" :class="inputCls" />
                  </div>
                </div>
                <div class="form-grid form-grid-3">
                  <div class="field">
                    <label>文章布局类型</label
                    ><select v-model="data.article.type" :class="selectCls">
                      <option value="tech">tech（技术）</option>
                      <option value="story">story（图文）</option>
                    </select>
                  </div>
                  <div class="field">
                    <label>文首横幅宽高比</label
                    ><input
                      v-model.number="data.article.banner_ratio"
                      type="number"
                      step="0.1"
                      min="0"
                      :class="inputCls"
                      placeholder="2.5"
                    />
                  </div>
                  <div class="field">
                    <label>自动摘要长度（字符，0=关闭）</label
                    ><input
                      v-model.number="data.article.auto_excerpt"
                      type="number"
                      min="0"
                      :class="inputCls"
                      placeholder="128"
                    />
                  </div>
                  <div class="field">
                    <label>有封面时的文章卡片样式</label
                    ><select v-model="data.article.card_style" :class="selectCls">
                      <option value="hero">hero（全图文字）</option>
                      <option value="classic">classic（摘要卡）</option>
                    </select>
                  </div>
                </div>
              </div>
              <div class="subsection">
                <p class="subsection-title">分类与标签映射</p>
                <p class="hint">分类颜色使用 CSS 颜色；图标字段可输入图标 key 或从图标库选择。</p>
                <div
                  v-for="target in ['category_color', 'category_icon', 'tag_icon'] as const"
                  :key="target"
                  class="mapping-block"
                >
                  <div class="mapping-head">
                    <span>{{
                      target === "category_color" ? "分类颜色" : target === "category_icon" ? "分类图标" : "标签图标"
                    }}</span
                    ><button type="button" class="add-btn" @click="addMapEntry(target)">
                      <Icon icon="ri:add-line" /> 添加
                    </button>
                  </div>
                  <div v-for="key in Object.keys(data.article[target])" :key="key" class="editable-row">
                    <input
                      :value="key"
                      :class="inputCls"
                      class="col-4"
                      placeholder="名称"
                      @change="renameMapEntry(target, key, $event)"
                    />
                    <input
                      v-if="target === 'category_color'"
                      v-model="data.article[target][key]"
                      :class="inputCls"
                      class="col-6"
                      placeholder="#1BCDFC"
                    />
                    <IconPicker
                      v-else
                      v-model="data.article[target][key]"
                      compact
                      placeholder="图标 key"
                      class="col-6"
                    />
                    <button
                      type="button"
                      class="row-btn row-btn-danger"
                      title="删除"
                      @click="removeMapEntry(target, key)"
                    >
                      <Icon icon="ri:delete-bin-line" />
                    </button>
                  </div>
                </div>
              </div>
              <div class="subsection">
                <p class="subsection-title">版权声明</p>
                <div class="switch-row">
                  <div class="switch-row-label"><div class="switch-row-name">显示版权声明</div></div>
                  <VSwitch v-model="data.article.copyright.enabled" />
                </div>
                <div class="form-grid form-grid-2">
                  <div class="field">
                    <label>协议名</label><input v-model="data.article.copyright.license" :class="inputCls" />
                  </div>
                  <div class="field">
                    <label>协议链接</label><input v-model="data.article.copyright.license_url" :class="inputCls" />
                  </div>
                </div>
              </div>
              <div class="subsection">
                <p class="subsection-title">分享平台</p>
                <div class="chip-group">
                  <label
                    v-for="p in platformOptions"
                    :key="p"
                    class="chip"
                    :class="{ active: data.article.share.platforms.includes(p) }"
                  >
                    <input
                      type="checkbox"
                      :checked="data.article.share.platforms.includes(p)"
                      @change="togglePlatform(p)"
                    />
                    <span>{{ p }}</span>
                  </label>
                </div>
                <div class="switch-row">
                  <div class="switch-row-label"><div class="switch-row-name">启用分享</div></div>
                  <VSwitch v-model="data.article.share.enabled" />
                </div>
              </div>
              <div class="subsection">
                <p class="subsection-title">打赏</p>
                <div class="switch-row">
                  <div class="switch-row-label"><div class="switch-row-name">启用打赏</div></div>
                  <VSwitch v-model="data.article.reward.enabled" />
                </div>
                <div class="form-grid form-grid-3">
                  <div class="field">
                    <label>按钮文案</label><input v-model="data.article.reward.text" :class="inputCls" />
                  </div>
                  <div class="field">
                    <label>微信收款码 URL</label><AttachmentField v-model="data.article.reward.wechat" />
                  </div>
                  <div class="field">
                    <label>支付宝收款码 URL</label><AttachmentField v-model="data.article.reward.alipay" />
                  </div>
                </div>
              </div>
              <div class="card-actions">
                <VButton type="primary" size="sm" :loading="savingKey === 'article'" @click="save('article')"
                  >保存文章设置</VButton
                >
              </div>
              <div class="subsection friends-subsection">
                <p class="subsection-title">友链巡检与 RSS 聚合（friends 段，独立保存）</p>
                <div class="form-grid form-grid-2">
                  <div class="field">
                    <label>RSS 刷新间隔（分钟，≤0 用默认 60）</label
                    ><input v-model.number="data.friends.refresh_interval" type="number" :class="inputCls" />
                  </div>
                  <div class="field">
                    <label>每友链文章数量（≤0 用默认 20）</label
                    ><input v-model.number="data.friends.posts_per_link" type="number" min="1" :class="inputCls" />
                  </div>
                  <div class="field">
                    <label>连通性巡检间隔（分钟，≤0 关闭）</label
                    ><input v-model.number="data.friends.check_interval" type="number" :class="inputCls" />
                  </div>
                  <div class="field">
                    <label>截图缓存间隔（分钟，≤0 关闭）</label
                    ><input v-model.number="data.friends.snapshot_interval" type="number" :class="inputCls" />
                  </div>
                  <div class="field field-wide">
                    <label>截图服务 URL 模板（使用 {url}）</label
                    ><input v-model="data.friends.snapshot_provider" :class="inputCls" />
                  </div>
                </div>
                <VButton type="secondary" size="sm" :loading="savingKey === 'friends'" @click="save('friends')"
                  >保存友链设置</VButton
                >
              </div>
            </VCard>

            <VCard v-show="active === 'tag_plugins'" title="标签插件默认参数">
              <p class="hint">
                对齐原版 _config.yml tag_plugins 段：未在具体标签里指定的参数用这里的默认值；留空=内置默认。
              </p>
              <div class="subsection">
                <p class="subsection-title">默认配色</p>
                <div class="form-grid form-grid-3">
                  <div class="field">
                    <label>note 提示框</label
                    ><select v-model="data.tag_plugins.note.default_color" :class="selectCls">
                      <option value="">默认</option>
                      <option v-for="c in tagColorOptions" :key="c" :value="c">{{ c }}</option>
                    </select>
                  </div>
                  <div class="field">
                    <label>mark 荧光标记</label
                    ><select v-model="data.tag_plugins.mark.default_color" :class="selectCls">
                      <option value="">默认</option>
                      <option v-for="c in tagColorOptions" :key="c" :value="c">{{ c }}</option>
                    </select>
                  </div>
                  <div class="field">
                    <label>button 按钮</label
                    ><select v-model="data.tag_plugins.button.default_color" :class="selectCls">
                      <option value="">默认</option>
                      <option v-for="c in tagColorOptions" :key="c" :value="c">{{ c }}</option>
                    </select>
                  </div>
                  <div class="field">
                    <label>icon 图标</label
                    ><select v-model="data.tag_plugins.icon.default_color" :class="selectCls">
                      <option value="">默认</option>
                      <option v-for="c in tagColorOptions" :key="c" :value="c">{{ c }}</option>
                    </select>
                  </div>
                  <div class="field">
                    <label>hashtag 话题（空=循环色）</label
                    ><select v-model="data.tag_plugins.hashtag.default_color" :class="selectCls">
                      <option value="">循环</option>
                      <option v-for="c in tagColorOptions" :key="c" :value="c">{{ c }}</option>
                    </select>
                  </div>
                </div>
              </div>
              <div class="subsection">
                <p class="subsection-title">quot 小标题默认图标（icons.yml key 或图片 URL）</p>
                <div class="form-grid form-grid-2">
                  <div class="field">
                    <label>前缀</label
                    ><IconPicker v-model="data.tag_plugins.quot.prefix" placeholder="如 quot:quote-left" />
                  </div>
                  <div class="field">
                    <label>后缀</label
                    ><IconPicker v-model="data.tag_plugins.quot.suffix" placeholder="如 quot:quote-right" />
                  </div>
                </div>
              </div>
              <div class="subsection">
                <p class="subsection-title">gallery 默认布局</p>
                <div class="form-grid form-grid-3">
                  <div class="field">
                    <label>布局</label
                    ><select v-model="data.tag_plugins.gallery.layout" :class="selectCls">
                      <option value="">默认</option>
                      <option value="grid">grid</option>
                      <option value="flow">flow</option>
                    </select>
                  </div>
                  <div class="field">
                    <label>尺寸</label
                    ><input v-model="data.tag_plugins.gallery.size" :class="inputCls" placeholder="mix" />
                  </div>
                  <div class="field">
                    <label>宽高比</label
                    ><input v-model="data.tag_plugins.gallery.ratio" :class="inputCls" placeholder="origin" />
                  </div>
                </div>
              </div>
              <div class="subsection">
                <p class="subsection-title">其它</p>
                <div class="form-grid form-grid-2">
                  <div class="field">
                    <label>timeline 限高（如 80vh，空=不限）</label
                    ><input v-model="data.tag_plugins.timeline.max_height" :class="inputCls" />
                  </div>
                  <div class="field">
                    <label>copy 成功提示文案</label
                    ><input v-model="data.tag_plugins.copy.toast" :class="inputCls" placeholder="复制成功" />
                  </div>
                </div>
              </div>
              <div class="subsection">
                <p class="subsection-title">emoji 表情源（{name} 占位；留空=内置 CDN 源）</p>
                <div class="form-grid form-grid-2">
                  <div class="field">
                    <label>default / qq</label><input v-model="data.tag_plugins.emoji.default" :class="inputCls" />
                  </div>
                  <div class="field">
                    <label>aru</label><input v-model="data.tag_plugins.emoji.aru" :class="inputCls" />
                  </div>
                  <div class="field">
                    <label>tieba</label><input v-model="data.tag_plugins.emoji.tieba" :class="inputCls" />
                  </div>
                  <div class="field">
                    <label>blobcat</label><input v-model="data.tag_plugins.emoji.blobcat" :class="inputCls" />
                  </div>
                  <div class="field">
                    <label>twemoji</label><input v-model="data.tag_plugins.emoji.twemoji" :class="inputCls" />
                  </div>
                </div>
              </div>
              <div class="card-actions">
                <VButton type="primary" size="sm" :loading="savingKey === 'tag_plugins'" @click="save('tag_plugins')"
                  >保存标签插件设置</VButton
                >
              </div>
            </VCard>

            <VCard v-show="active === 'seo'" title="SEO 设置">
              <div class="subsection">
                <p class="subsection-title">站点来源与社交卡片</p>
                <div class="form-grid form-grid-2">
                  <div class="field">
                    <label>源站主机（originalHost）</label
                    ><input v-model="data.seo.canonical.originalHost" :class="inputCls" placeholder="example.com" />
                  </div>
                  <div class="field">
                    <label>Twitter ID</label
                    ><input
                      v-model="data.seo.open_graph.twitter_id"
                      :class="inputCls"
                      placeholder="username（不含 @）"
                    />
                  </div>
                </div>
                <div class="switch-row">
                  <div class="switch-row-label">
                    <div class="switch-row-name">启用 Open Graph</div>
                    <div class="switch-row-desc">输出 og:title、og:description 与 Twitter 站点标记</div>
                  </div>
                  <VSwitch v-model="data.seo.open_graph.enable" />
                </div>
                <div class="field">
                  <label>官方主机（每行一个）</label
                  ><textarea
                    :value="data.seo.canonical.officialHosts.join('\n')"
                    rows="3"
                    :class="textareaCls"
                    placeholder="localhost"
                    @input="
                      (e) =>
                        (data.seo.canonical.officialHosts = (e.target as HTMLTextAreaElement).value
                          .split(/\r?\n/)
                          .map((s: string) => s.trim())
                          .filter(Boolean))
                    "
                  ></textarea>
                </div>
                <div class="field">
                  <label>预连接域名（每行一个）</label
                  ><textarea
                    :value="data.seo.preconnect.join('\n')"
                    rows="3"
                    :class="textareaCls"
                    placeholder="https://cdn.jsdelivr.net"
                    @input="
                      (e) =>
                        (data.seo.preconnect = (e.target as HTMLTextAreaElement).value
                          .split(/\r?\n/)
                          .map((s: string) => s.trim())
                          .filter(Boolean))
                    "
                  ></textarea>
                </div>
                <div class="field">
                  <label>结构化数据社交链接（每行一个）</label
                  ><textarea
                    :value="data.seo.structured_data.links.join('\n')"
                    rows="3"
                    :class="textareaCls"
                    placeholder="https://github.com/username"
                    @input="
                      (e) =>
                        (data.seo.structured_data.links = (e.target as HTMLTextAreaElement).value
                          .split(/\r?\n/)
                          .map((s: string) => s.trim())
                          .filter(Boolean))
                    "
                  ></textarea>
                </div>
              </div>
              <div class="field">
                <label>默认 Meta Description（文章未单独设置时使用）</label
                ><textarea v-model="data.seo.default_description" rows="2" :class="textareaCls"></textarea>
              </div>
              <div class="field">
                <label>默认 Meta Keywords（逗号分隔）</label
                ><input v-model="data.seo.default_keywords" :class="inputCls" placeholder="博客, Go, 前端" />
              </div>
              <div class="field">
                <label>robots.txt 附加规则（原样追加到默认规则之后）</label
                ><textarea
                  v-model="data.seo.robots_extra"
                  rows="4"
                  :class="codeAreaCls"
                  placeholder="User-agent: GPTBot&#10;Disallow: /"
                ></textarea>
              </div>
              <div class="switch-row">
                <div class="switch-row-label">
                  <div class="switch-row-name">禁止搜索引擎收录</div>
                  <div class="switch-row-desc">全站输出 noindex meta，请谨慎开启</div>
                </div>
                <VSwitch v-model="data.seo.no_index" />
              </div>
              <div class="subsection">
                <p class="subsection-title">站点验证（留空不输出对应 meta 标签）</p>
                <div class="form-grid form-grid-3">
                  <div class="field">
                    <label>Google 验证码</label><input v-model="data.seo.google_verification" :class="inputCls" />
                  </div>
                  <div class="field">
                    <label>Bing 验证码</label><input v-model="data.seo.bing_verification" :class="inputCls" />
                  </div>
                  <div class="field">
                    <label>百度验证码</label><input v-model="data.seo.baidu_verification" :class="inputCls" />
                  </div>
                </div>
              </div>
              <div class="card-actions">
                <VButton type="primary" size="sm" :loading="savingKey === 'seo'" @click="save('seo')"
                  >保存 SEO 设置</VButton
                >
              </div>
            </VCard>

            <VCard v-show="active === 'search'" title="搜索设置">
              <p class="hint">对齐 Stellar search.local_search；修改后会立即使本地索引失效并重建。</p>
              <div class="form-grid form-grid-3">
                <div class="field">
                  <label>搜索服务</label
                  ><select v-model="data.search.service" :class="selectCls">
                    <option value="local_search">local_search</option>
                    <option value="todo">todo</option>
                  </select>
                </div>
                <div class="field">
                  <label>索引范围</label
                  ><select v-model="data.search.local_search.field" :class="selectCls">
                    <option value="all">全部</option>
                    <option value="post">文章</option>
                    <option value="page">页面</option>
                  </select>
                </div>
                <div class="field">
                  <label>索引路径</label
                  ><input v-model="data.search.local_search.path" :class="inputCls" placeholder="/search.json" />
                </div>
              </div>
              <div class="switch-row">
                <div class="switch-row-label">
                  <div class="switch-row-name">搜索正文内容</div>
                  <div class="switch-row-desc">关闭后仍可搜索标题，但不会把正文写入索引</div>
                </div>
                <VSwitch v-model="data.search.local_search.content" />
              </div>
              <div class="field">
                <label>排除路径（每行一个）</label
                ><textarea
                  :value="data.search.local_search.skip_search.join('\n')"
                  rows="4"
                  :class="textareaCls"
                  placeholder="/post/private"
                  @input="updateSkipSearch"
                ></textarea>
              </div>
              <div class="card-actions">
                <VButton type="primary" size="sm" :loading="savingKey === 'search'" @click="save('search')"
                  >保存搜索设置</VButton
                >
              </div>
            </VCard>

            <VCard v-show="active === 'inject'" title="代码注入">
              <div class="warning-hint">
                <Icon icon="ri:error-warning-line" width="14" height="14" />
                <span
                  >注入内容将不经转义原样输出到前台所有页面，可执行任意脚本。请只粘贴你完全信任的代码（统计、字体、自定义样式等），错误或恶意代码可能导致页面故障与安全风险，由此产生的后果由站长自行承担。</span
                >
              </div>
              <div class="field">
                <label>头部注入（输出在 &lt;/head&gt; 之前）</label
                ><textarea
                  v-model="data.inject.inject_head"
                  rows="6"
                  :class="codeAreaCls"
                  placeholder="<script>...</script> 或 <link ...>"
                ></textarea>
              </div>
              <div class="field">
                <label>页脚注入（输出在 &lt;/body&gt; 之前）</label
                ><textarea
                  v-model="data.inject.inject_footer"
                  rows="6"
                  :class="codeAreaCls"
                  placeholder="<script>...</script>"
                ></textarea>
              </div>
              <div class="card-actions">
                <VButton type="primary" size="sm" :loading="savingKey === 'inject'" @click="save('inject')"
                  >保存代码注入</VButton
                >
              </div>
            </VCard>

            <VCard v-show="active === 'ai'" title="AI 属性生成">
              <p class="hint">编辑器「AI 属性生成」使用的 OpenAI 兼容接口（DeepSeek / 通义 / Ollama 等均可）。</p>
              <div class="field">
                <label>API 地址（如 https://api.deepseek.com/v1）</label
                ><input v-model="data.ai.base_url" :class="inputCls" placeholder="https://api.openai.com/v1" />
              </div>
              <div class="field">
                <label>API Key（本地模型可留空；已保存时显示 ****，留空保存不覆盖）</label
                ><input v-model="data.ai.api_key" type="password" autocomplete="off" :class="inputCls" />
              </div>
              <div class="field">
                <label>模型名（如 deepseek-chat / gpt-4o-mini）</label
                ><input v-model="data.ai.model" :class="inputCls" />
              </div>
              <div class="subsection">
                <p class="subsection-title">AI 文章摘要</p>
                <div class="switch-row">
                  <div class="switch-row-label">
                    <div class="switch-row-desc">前台展示 AI 摘要（关闭后文章页不渲染摘要块）</div>
                  </div>
                  <VSwitch v-model="data.ai.summary_enabled" />
                </div>
                <div class="switch-row">
                  <div class="switch-row-label">
                    <div class="switch-row-desc">允许读者临时接入自己的 API 生成摘要（Key 不落本站）</div>
                  </div>
                  <VSwitch v-model="data.ai.allow_user_api" />
                </div>
              </div>
              <div class="card-actions">
                <VButton type="primary" size="sm" :loading="savingKey === 'ai'" @click="save('ai')"
                  >保存 AI 设置</VButton
                >
              </div>
            </VCard>

            <VCard v-show="active === 'security'" title="安全设置">
              <div class="warning-hint">
                <Icon icon="ri:information-line" width="14" height="14" />
                <span>密码管理已迁移至 <router-link to="/profile" class="link">个人资料页</router-link>。</span>
              </div>
              <p class="hint">忘记密码：删除数据库 settings 表 admin 段后重启，回落 site.yml 初始密码。</p>
            </VCard>

            <VCard v-show="active === 'data_cache'" title="数据缓存">
              <p class="hint">
                友链、时间线、GitHub 信息等动态数据的浏览器本地缓存（TTL
                内命中直接显示缓存，过期后先显示缓存再后台刷新）。对齐原版 Stellar data_cache 段。
              </p>
              <div class="switch-row">
                <div class="switch-row-label">
                  <div class="switch-row-name">启用本地缓存</div>
                  <div class="switch-row-desc">关闭后每次访问都重新拉取动态数据（TTL=0）</div>
                </div>
                <VSwitch v-model="data.data_cache.enable" />
              </div>
              <div class="form-grid form-grid-2">
                <div class="field">
                  <label>默认缓存时长（秒）</label>
                  <input
                    v-model.number="data.data_cache.default_ttl"
                    type="number"
                    min="0"
                    :class="inputCls"
                    placeholder="3600"
                  />
                </div>
              </div>
              <p class="hint">按服务覆盖缓存时长（ttl）请通过「高级 · JSON」编辑。</p>
              <div class="card-actions card-actions-inline">
                <VButton type="primary" size="sm" :loading="savingKey === 'data_cache'" @click="save('data_cache')"
                  >保存数据缓存设置</VButton
                >
              </div>
            </VCard>

            <VCard v-show="active === 'advanced'" title="高级 · JSON 兜底编辑">
              <p class="hint">全部段 JSON 兜底编辑，一般无需使用（结构化表单已覆盖）。</p>
              <div v-for="s in allSections" :key="s" class="json-block">
                <div class="json-block-head">
                  <span class="json-block-title">{{ s }}</span>
                  <VButton type="secondary" size="xs" :loading="savingKey === 'json:' + s" @click="saveJson(s)"
                    >保存 {{ s }}</VButton
                  >
                </div>
                <textarea v-model="rawText[s]" rows="8" :class="codeAreaCls"></textarea>
              </div>
            </VCard>

            <VCard v-show="active === 'appearance'" title="外观设置">
              <div class="subsection">
                <div class="field">
                  <label>默认主题（对齐原版 style.prefers_theme）</label>
                  <select v-model="data.style.prefers_theme" :class="selectCls">
                    <option value="">深色（默认）</option>
                    <option value="light">浅色</option>
                    <option value="auto">跟随系统</option>
                  </select>
                </div>
              </div>
              <AppearanceTab :data="data" :saving="savingKey" @save="save" />
            </VCard>
            <VCard v-show="active === 'widgets'" title="组件编排">
              <WidgetsTab :data="data" :saving="savingKey" @save="save" />
            </VCard>
            <VCard v-show="active === 'theme_yaml'" title="主题 YAML">
              <ThemeYamlTab />
            </VCard>
            <VCard v-show="active === 'plugins'" title="插件">
              <PluginsTab :data="data.plugins" :saving="savingKey" @save="() => save('plugins')" />
            </VCard>
            <VCard v-show="active === 'cover'" title="首页 Hero">
              <CoverTab :data="data.cover" :saving="savingKey" @save="() => save('cover')" />
            </VCard>
            <VCard v-show="active === 'nav_tabs'" title="导航标签">
              <NavTabsTab :data="data.nav_tabs" :saving="savingKey" @save="() => save('nav_tabs')" />
            </VCard>
            <VCard v-show="active === 'authors'" title="作者管理">
              <AuthorsTab :data="data.authors" :saving="savingKey" @save="() => save('authors')" />
            </VCard>
          </template>
        </div>
      </div>

      <!-- 前端预览面板（并排右栏，不遮挡设置内容；sticky 跟随滚动） -->
      <aside v-if="previewOpen" class="preview-panel">
        <div class="preview-panel-header">
          <div class="preview-focus-label">
            <Icon icon="ri:eye-line" class="text-text-tertiary" />
            <span v-if="previewFocus">预览：{{ previewFocus.label }}</span>
            <span v-else>该设置无直接前台视觉区域</span>
          </div>
          <div class="flex items-center gap-1.5">
            <!-- 视口切换器 -->
            <div class="apple-segmented rect mr-1">
              <button
                type="button"
                :class="{ active: previewViewport === 'desktop' }"
                title="桌面端视口 (100%)"
                class="!p-1.5"
                @click="previewViewport = 'desktop'"
              >
                <Icon icon="ri:macbook-line" class="text-xs" />
              </button>
              <button
                type="button"
                :class="{ active: previewViewport === 'tablet' }"
                title="平板端视口 (768px)"
                class="!p-1.5"
                @click="previewViewport = 'tablet'"
              >
                <Icon icon="ri:tablet-line" class="text-xs" />
              </button>
              <button
                type="button"
                :class="{ active: previewViewport === 'mobile' }"
                title="移动端视口 (375px)"
                class="!p-1.5"
                @click="previewViewport = 'mobile'"
              >
                <Icon icon="ri:smartphone-line" class="text-xs" />
              </button>
            </div>

            <button type="button" title="刷新预览" class="preview-icon-btn" @click="refreshPreview">
              <Icon icon="ri:refresh-line" />
            </button>
            <button type="button" title="关闭预览" class="preview-icon-btn" @click="previewOpen = false">
              <Icon icon="ri:close-line" />
            </button>
          </div>
        </div>
        <div class="preview-panel-body">
          <template v-if="previewFocus">
            <div v-if="previewLoading" class="preview-loading">
              <Icon icon="ri:loader-4-line" class="settings-spin" width="20" height="20" />
              <span>渲染中…</span>
            </div>
            <!-- sandbox 隔离：不授予 allow-same-origin，iframe 运行于 opaque origin，
               inject 段脚本可执行但无法读取控制台会话凭据；预览资源均为绝对路径，
               opaque origin 下跨源子资源（CSS/图片/字体）加载不受影响 -->
            <div class="flex justify-center w-full h-full">
              <iframe
                v-if="previewHtml"
                :srcdoc="previewHtml"
                sandbox="allow-scripts"
                title="前端预览"
                class="preview-frame transition-all duration-300"
                :style="{
                  maxWidth: previewViewport === 'mobile' ? '375px' : previewViewport === 'tablet' ? '768px' : '100%',
                  borderRadius: previewViewport !== 'desktop' ? '12px' : '0',
                  boxShadow: previewViewport !== 'desktop' ? '0 8px 30px rgba(0,0,0,0.5)' : 'none',
                }"
              />
              <div v-else-if="!previewLoading" class="preview-loading">暂无预览</div>
            </div>
          </template>
          <div v-else class="preview-loading">
            <Icon icon="ri:information-line" width="20" height="20" />
            <span>当前设置项（如 SEO/AI/安全）不直接决定前台某个视觉区域，保存后在前台查看效果。</span>
          </div>
        </div>
      </aside>
    </div>
  </div>
</template>

<style scoped>
.settings-view {
  min-height: 100%;
  background: var(--color-background);
}
.settings-page-header {
  height: 56px;
  padding: 0 24px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-bottom: 1px solid var(--color-border);
  background: color-mix(in srgb, #000 60%, transparent);
  backdrop-filter: saturate(180%) blur(20px);
  -webkit-backdrop-filter: saturate(180%) blur(20px);
  position: sticky;
  top: 0;
  z-index: 20;
}
.settings-page-header-title {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 19px;
  font-weight: 600;
  letter-spacing: -0.2px;
  color: var(--color-foreground);
}
.settings-page-header-icon {
  color: var(--color-primary);
  font-size: 20px;
}
.settings-container {
  display: flex;
  gap: 24px;
  padding: 24px;
  max-width: 1100px;
  margin: 0 auto;
  width: 100%;
  box-sizing: border-box;
}
.settings-nav {
  width: 200px;
  flex-shrink: 0;
  padding-top: 4px;
}
.settings-nav-list {
  display: flex;
  flex-direction: column;
  gap: 2px;
  position: sticky;
  top: 84px;
}
.settings-nav-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  border-radius: 10px;
  font-size: 13.5px;
  color: var(--color-text-tertiary);
  cursor: pointer;
  transition: all var(--duration-fast) var(--ease-standard);
  text-align: left;
  background: transparent;
  border: none;
}
.settings-nav-item:hover {
  background: color-mix(in srgb, #fff 4%, transparent);
  color: var(--color-foreground);
}
.settings-nav-item.active {
  color: var(--color-primary);
  font-weight: 500;
  background: color-mix(in srgb, var(--color-primary) 12%, transparent);
}
.settings-nav-item-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  opacity: 0.8;
}
.settings-nav-item.active .settings-nav-item-icon {
  opacity: 1;
}
.settings-content {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.settings-section-title {
  font-size: 22px;
  font-weight: 600;
  color: var(--color-foreground);
  letter-spacing: -0.3px;
}
.settings-section-desc {
  font-size: 13px;
  color: var(--color-text-tertiary);
  margin-top: 4px;
}
.settings-subtabs {
  align-self: flex-start;
  flex-wrap: wrap;
}
.settings-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 10px;
  padding: 60px 0;
  font-size: 13px;
  color: var(--color-text-tertiary);
}
.settings-state-error {
  color: var(--color-state-error);
}
.settings-spin {
  animation: settings-spin 1s linear infinite;
}
@keyframes settings-spin {
  to {
    transform: rotate(360deg);
  }
}
.field {
  margin-bottom: 16px;
}
.field:last-child {
  margin-bottom: 0;
}
.field label {
  display: block;
  font-size: 12px;
  color: var(--color-text-tertiary);
  margin-bottom: 6px;
}
.field-narrow {
  max-width: 160px;
}
.field-wide {
  grid-column: 1 / -1;
}
.field-inline {
  display: flex;
  gap: 8px;
  align-items: center;
}
.field-inline .apple-input {
  flex: 1;
}
.form-grid {
  display: grid;
  gap: 12px;
  margin-bottom: 16px;
}
.form-grid:last-child {
  margin-bottom: 0;
}
.form-grid-2 {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}
.form-grid-3 {
  grid-template-columns: repeat(3, minmax(0, 1fr));
}
.hint {
  font-size: 12px;
  color: var(--color-text-quaternary);
  margin: 0 0 12px;
}
.subsection {
  margin-top: 24px;
}
.subsection-title {
  font-size: 12px;
  font-weight: 600;
  color: var(--color-text-secondary);
  margin: 0 0 12px;
}
.card-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 24px;
}
.card-actions-inline {
  flex-wrap: wrap;
}
.friends-subsection {
  margin-top: 24px;
}
.icon-btn {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  border-radius: var(--radius-sm);
  background: color-mix(in srgb, var(--color-primary) 12%, transparent);
  color: var(--color-primary);
  border: none;
  cursor: pointer;
  transition: background var(--duration-fast) var(--ease-standard);
}
.icon-btn:hover:not(:disabled) {
  background: color-mix(in srgb, var(--color-primary) 20%, transparent);
}
.icon-btn:disabled {
  opacity: 0.3;
  cursor: not-allowed;
}
.add-btn {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 13px;
  color: var(--color-primary);
  background: transparent;
  border: none;
  cursor: pointer;
  padding: 6px 0;
  transition: opacity var(--duration-fast) var(--ease-standard);
}
.add-btn:hover {
  opacity: 0.8;
}
.add-btn-indent {
  padding-left: 16px;
}
.editable-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
  margin-bottom: 12px;
}
.editable-row {
  display: grid;
  grid-template-columns: repeat(12, minmax(0, 1fr));
  gap: 12px;
  align-items: center;
}
.col-2 {
  grid-column: span 2;
}
.col-3 {
  grid-column: span 3;
}
.col-4 {
  grid-column: span 4;
}
.col-6 {
  grid-column: span 6;
}
.row-actions {
  grid-column: span 2;
  display: flex;
  align-items: center;
  gap: 4px;
  justify-content: flex-end;
}
.row-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border-radius: 6px;
  color: var(--color-text-quaternary);
  background: transparent;
  border: none;
  cursor: pointer;
  transition: all var(--duration-fast) var(--ease-standard);
}
.row-btn:hover:not(:disabled) {
  background: var(--color-secondary);
  color: var(--color-foreground);
}
.row-btn:disabled {
  opacity: 0.25;
  cursor: not-allowed;
}
.row-btn-danger {
  color: var(--color-state-error);
}
.row-btn-danger:hover:not(:disabled) {
  background: color-mix(in srgb, var(--color-state-error) 12%, transparent);
  color: var(--color-state-error);
}
.nested-group {
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  padding: 16px;
  margin-bottom: 12px;
  display: flex;
  flex-direction: column;
  gap: 12px;
  background: color-mix(in srgb, #000 20%, var(--color-card));
}
.nested-group-head {
  display: flex;
  align-items: center;
  gap: 6px;
}
.preview-box {
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  padding: 16px;
  margin-bottom: 16px;
}
.preview-label {
  font-size: 12px;
  font-weight: 500;
  color: var(--color-text-tertiary);
  margin: 0 0 12px;
}
.preview-row {
  display: flex;
  flex-wrap: wrap;
  gap: 24px;
  align-items: flex-end;
}
.preview-item {
  text-align: center;
}
.preview-item-label {
  font-size: 10px;
  color: var(--color-text-tertiary);
  margin: 0 0 8px;
}
.avatar-32 {
  width: 32px;
  height: 32px;
  border-radius: 9999px;
  overflow: hidden;
  border: 1px solid var(--color-border);
}
.avatar-128 {
  width: 128px;
  height: 128px;
  border-radius: 9999px;
  overflow: hidden;
  border: 1px solid var(--color-border);
}
.avatar-32 img,
.avatar-128 img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.avatar-hover {
  position: relative;
  width: 96px;
  height: 96px;
  border-radius: 9999px;
  overflow: hidden;
  border: 2px solid transparent;
  transition: border-color var(--duration-normal) var(--ease-standard);
}
.avatar-hover:hover {
  border-color: var(--color-primary);
}
.avatar-hover-bg {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.avatar-hover-fg {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  object-fit: cover;
  transition: transform var(--duration-slow) var(--ease-standard);
}
.avatar-hover:hover .avatar-hover-fg {
  transform: scale(1.1);
}
.switch-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
  padding: 12px 0;
  border-bottom: 1px solid var(--color-border);
}
.switch-row:last-child {
  border-bottom: none;
}
.switch-row-name {
  font-size: 13.5px;
  font-weight: 500;
  color: var(--color-foreground);
}
.switch-row-desc {
  font-size: 12px;
  color: var(--color-text-tertiary);
  margin-top: 3px;
  line-height: 1.4;
  max-width: 460px;
}
.warning-hint {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  font-size: 12px;
  line-height: 1.6;
  color: var(--color-state-warning);
  background: color-mix(in srgb, var(--color-state-warning) 8%, transparent);
  border: 1px solid color-mix(in srgb, var(--color-state-warning) 25%, transparent);
  border-radius: var(--radius-md);
  padding: 12px 14px;
  margin-bottom: 14px;
}
.warning-hint .link {
  color: var(--color-primary);
  text-decoration: none;
  font-weight: 500;
}
.warning-hint .link:hover {
  text-decoration: underline;
}
.chip-group {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 12px;
}
.chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  font-size: 12.5px;
  color: var(--color-text-secondary);
  background: var(--color-secondary);
  border: 1px solid var(--color-border);
  border-radius: 9999px;
  cursor: pointer;
  transition: all var(--duration-fast) var(--ease-standard);
  user-select: none;
}
.chip input {
  position: absolute;
  opacity: 0;
  pointer-events: none;
}
.chip:hover {
  border-color: var(--color-border-strong, var(--color-border));
  color: var(--color-foreground);
}
.chip.active {
  background: color-mix(in srgb, var(--color-primary) 12%, transparent);
  border-color: color-mix(in srgb, var(--color-primary) 40%, transparent);
  color: var(--color-primary);
}
.test-result {
  font-size: 12px;
}
.test-result.is-ok {
  color: var(--color-state-success);
}
.test-result.is-err {
  color: var(--color-state-error);
}
.json-block {
  margin-bottom: 16px;
}
.json-block:last-child {
  margin-bottom: 0;
}
.json-block-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}
.json-block-title {
  font-size: 12px;
  font-weight: 600;
  color: var(--color-text-secondary);
  font-family: var(--font-mono, ui-monospace, monospace);
}
.wiki-card {
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  padding: 16px;
  margin-bottom: 12px;
  transition: all var(--duration-normal) var(--ease-standard);
}
.wiki-card:hover {
  border-color: var(--color-border-strong, var(--color-border));
}
.wiki-card-dragging {
  opacity: 0.4;
}
.wiki-card-head {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 14px;
}
.drag-handle {
  color: var(--color-text-quaternary);
  font-size: 18px;
  cursor: grab;
  flex-shrink: 0;
}
.drag-handle:active {
  cursor: grabbing;
}
.wiki-icon {
  width: 32px;
  height: 32px;
  border-radius: var(--radius-sm);
  overflow: hidden;
  border: 1px solid var(--color-border);
  background: var(--color-secondary);
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: center;
}
.wiki-icon img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.wiki-icon-empty {
  color: var(--color-text-quaternary);
}
.wiki-title {
  flex: 1;
  font-size: 14px;
  font-weight: 500;
  color: var(--color-text-secondary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

@media (max-width: 860px) {
  .settings-container {
    flex-direction: column;
    padding: 16px;
    gap: 24px;
  }
  .settings-nav {
    width: 100%;
  }
  .settings-nav-list {
    position: static;
    flex-direction: row;
    flex-wrap: wrap;
    gap: 4px;
  }
  .settings-nav-item {
    min-height: 44px;
    padding: 10px 16px;
    border-radius: 9999px;
  }
  .form-grid-2,
  .form-grid-3 {
    grid-template-columns: 1fr;
  }
  /* 窄屏：下坞面板加高，便于查看 */
  .preview-panel {
    height: 55vh;
  }
}

/* ===== 移动端补充优化 ===== */
@media (max-width: 640px) {
  .settings-container {
    padding: 12px;
    gap: 12px;
  }
  .settings-content {
    gap: 12px;
  }

  /* 主导航更紧凑（保持 44px 触达高度） */
  .settings-nav-list {
    gap: 4px;
  }
  .settings-nav-item {
    padding: 8px 12px;
    font-size: 12px;
    gap: 6px;
  }
  .settings-nav-item-icon svg {
    width: 14px;
    height: 14px;
  }

  /* 子分段 Tab 横向滚动 */
  .subtabs-scroll {
    overflow-x: auto;
    -webkit-overflow-scrolling: touch;
    scrollbar-width: none;
    flex-wrap: nowrap;
  }
  .subtabs-scroll::-webkit-scrollbar {
    display: none;
  }

  /* 段落标题更紧凑 */
  .settings-section-title {
    font-size: 15px;
  }
  .settings-section-desc {
    font-size: 12px;
  }

  /* 表单字段更紧凑 */
  .field {
    margin-bottom: 12px;
  }
  .field label {
    font-size: 12px;
    margin-bottom: 4px;
  }
  .form-grid {
    gap: 12px;
  }

  /* 卡片内边距缩小 */
  .settings-content :deep(.card-body) {
    padding: 12px !important;
  }

  /* 卡片标题更紧凑 */
  .settings-content :deep(.card-header) {
    padding: 12px 16px !important;
    font-size: 13px !important;
  }
}

/* ===== 前端预览面板（底部停靠，不遮挡、不挤压设置内容） ===== */
/* sticky bottom：吸附在内容滚动区底部，水平方向跟随内容区（不侵入左侧导航） */
.preview-panel {
  position: sticky;
  bottom: 0;
  z-index: 30;
  height: 45vh;
  display: flex;
  flex-direction: column;
  border: 1px solid var(--color-border);
  border-bottom: none;
  border-radius: 12px 12px 0 0;
  background: var(--color-card);
  box-shadow: 0 -8px 24px rgba(0, 0, 0, 0.35);
}
.preview-panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 8px 12px;
  border-bottom: 1px solid var(--color-border);
}
.preview-focus-label {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  font-weight: 500;
  color: var(--color-text-secondary);
}
.preview-icon-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border-radius: var(--radius-md);
  color: var(--color-text-tertiary);
  transition: all var(--duration-fast) var(--ease-standard);
}
.preview-icon-btn:hover {
  background: var(--color-accent);
  color: var(--color-foreground);
}
.preview-panel-body {
  position: relative;
  flex: 1;
  min-height: 0;
}
.preview-frame {
  width: 100%;
  height: 100%;
  border: 0;
  background: var(--color-background, #fff);
}
.preview-loading {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  color: var(--color-text-tertiary);
  font-size: 13px;
}
</style>
