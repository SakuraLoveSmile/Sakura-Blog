<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, watch } from "vue";
import { Icon } from "@iconify/vue";
import { settingsApi, previewApi } from "@/api";
import { Toast, Dialog, VLoading } from "@/components/ui";
import {
  WIDGET_META,
  BUILTIN_WIDGET_NAMES,
  metaOf,
  fieldsForLayout,
  NO_EXTRA_PARAM_LAYOUTS,
  builtinDefaults,
} from "../settings/widget-meta";
import { buildPreviewDoc } from "./preview/preview-document";

// 侧边栏组件库视图（Spec 58）：浏览 16 种内置侧边栏组件、编辑参数并保存为命名方案。
// 方案 = widgets.library 的命名条目（name → layout + 参数），零后端改动；
// 保存后经 PUT /settings/widgets 整段写回（含 pages，防止覆盖编排序列）。

interface PageBars {
  leftbar?: string[];
  rightbar?: string[];
}

const loading = ref(true);
const loadError = ref("");
const widgets = ref<{ library: Record<string, any>; pages: Record<string, PageBars> }>({
  library: {},
  pages: {},
});

async function load() {
  loading.value = true;
  loadError.value = "";
  try {
    const { data } = await settingsApi.all();
    const w = (data?.widgets || {}) as any;
    widgets.value = {
      library: w.library && typeof w.library === "object" ? w.library : {},
      pages: w.pages && typeof w.pages === "object" ? w.pages : {},
    };
    select("builtin", selName.value);
  } catch {
    loadError.value = "加载设置失败";
  } finally {
    loading.value = false;
  }
}

async function putWidgets() {
  await settingsApi.updateSection("widgets", widgets.value);
}

onMounted(load);

// ---------- 选择与草稿 ----------
type SelKind = "builtin" | "scheme";
const selKind = ref<SelKind>("builtin");
const selName = ref("tagcloud");
// draft = 当前编辑中的定义（含 layout），保存时写回 library
const draft = ref<Record<string, any>>({ layout: "tagcloud" });

function select(kind: SelKind, name: string) {
  selKind.value = kind;
  selName.value = name;
  if (kind === "builtin") {
    // 内置组件：缺省参数为基底，叠加用户已有同名覆盖（WidgetsTab 里自定义过的参数）
    const override = widgets.value.library[name];
    draft.value = {
      ...builtinDefaults(name),
      ...(override && typeof override === "object" ? override : {}),
      layout: builtinDefaults(name).layout,
    };
  } else {
    const entry = widgets.value.library[name];
    draft.value = entry && typeof entry === "object" ? JSON.parse(JSON.stringify(entry)) : {};
  }
  jsonText.value = JSON.stringify(draft.value, null, 2);
  schedulePreview(true);
}

const selMeta = computed(() => (selKind.value === "builtin" ? metaOf(selName.value) : undefined));
const draftLayout = computed(() => String(draft.value?.layout || ""));
const draftFields = computed(() => fieldsForLayout(draftLayout.value));
const noExtra = computed(() => NO_EXTRA_PARAM_LAYOUTS.has(draftLayout.value));

// ---------- 列表（搜索过滤） ----------
const search = ref("");
const filteredMeta = computed(() => {
  const kw = search.value.trim().toLowerCase();
  if (!kw) return WIDGET_META;
  return WIDGET_META.filter((m) => m.name.toLowerCase().includes(kw) || m.label.toLowerCase().includes(kw));
});
const schemeNames = computed(() =>
  Object.keys(widgets.value.library)
    .filter((n) => !BUILTIN_WIDGET_NAMES.has(n))
    .sort(),
);
const filteredSchemes = computed(() => {
  const kw = search.value.trim().toLowerCase();
  return kw ? schemeNames.value.filter((n) => n.toLowerCase().includes(kw)) : schemeNames.value;
});
function schemeLayout(name: string): string {
  return String(widgets.value.library[name]?.layout || "");
}
function isSelected(kind: SelKind, name: string): boolean {
  return selKind.value === kind && selName.value === name;
}

// ---------- 方案名校验 ----------
function validateSchemeName(name: string): string {
  const n = name.trim();
  if (!n) return "方案名不能为空";
  if (!/^[A-Za-z0-9_\u4e00-\u9fa5-]+$/.test(n)) return "方案名仅支持字母、数字、下划线、中划线与中文";
  if (n.startsWith("__")) return "方案名不能以双下划线开头（保留前缀）";
  if (BUILTIN_WIDGET_NAMES.has(n)) return `「${n}」是内置组件名，不能作为方案名`;
  return "";
}

// ---------- 保存 ----------
const saving = ref(false);

function cleanDraft(): Record<string, any> {
  const out: Record<string, any> = {};
  for (const [k, v] of Object.entries(draft.value)) {
    if (v === undefined || v === null || v === "") continue;
    out[k] = v;
  }
  if (!out.layout) out.layout = draftLayout.value || selName.value;
  return out;
}

// 「保存为方案」（内置组件详情）与「另存为」（方案详情）共用
const nameInputVisible = ref(false);
const nameInputValue = ref("");

function openNameInput() {
  nameInputValue.value = "";
  nameInputVisible.value = true;
}

async function confirmNameInput() {
  const err = validateSchemeName(nameInputValue.value);
  if (err) {
    Toast.error(err);
    return;
  }
  if (widgets.value.library[nameInputValue.value.trim()]) {
    Toast.error(`方案「${nameInputValue.value.trim()}」已存在，请换一个名字`);
    return;
  }
  const name = nameInputValue.value.trim();
  saving.value = true;
  try {
    widgets.value.library[name] = cleanDraft();
    await putWidgets();
    Toast.success("方案已保存", { description: `「${name}」可在设置 → 组件编排中选用` });
    nameInputVisible.value = false;
    select("scheme", name);
  } catch {
  } finally {
    saving.value = false;
  }
}

async function saveScheme() {
  if (selKind.value !== "scheme") return;
  saving.value = true;
  try {
    widgets.value.library[selName.value] = cleanDraft();
    await putWidgets();
    Toast.success("已保存，前台已生效");
  } catch {
  } finally {
    saving.value = false;
  }
}

// ---------- 删除（含 pages 引用检查） ----------
const PAGE_LABELS: Record<string, string> = {
  index: "首页",
  post: "文章页",
  page: "独立页",
  archive: "归档页",
  categories: "分类页",
  tags: "标签页",
  author: "作者页",
};

function referencedPages(name: string): string[] {
  const out: string[] = [];
  for (const [pageType, bars] of Object.entries(widgets.value.pages)) {
    const used = [...(bars?.leftbar || []), ...(bars?.rightbar || [])].includes(name);
    if (used) out.push(PAGE_LABELS[pageType] || pageType);
  }
  return out;
}

function removeScheme() {
  if (selKind.value !== "scheme") return;
  const name = selName.value;
  const refs = referencedPages(name);
  Dialog.warning({
    title: `删除方案「${name}」？`,
    description: refs.length
      ? `该方案正被页面类型「${refs.join("、")}」的左右栏序列引用，删除后对应位置将不再渲染此组件。`
      : "删除后不可恢复，可随时重新保存新方案。",
    confirmText: "删除",
    confirmType: "danger",
    onConfirm: async () => {
      try {
        delete widgets.value.library[name];
        await putWidgets();
        Toast.success("方案已删除");
        select("builtin", WIDGET_META[0].name);
      } catch {}
    },
  });
}

// ---------- JSON 兜底 ----------
const jsonText = ref("");
const jsonError = ref("");

function onJsonInput(val: string) {
  jsonText.value = val;
  try {
    const parsed = JSON.parse(val);
    if (parsed && typeof parsed === "object" && !Array.isArray(parsed)) {
      draft.value = parsed;
      jsonError.value = "";
    } else {
      jsonError.value = "需为 JSON 对象";
    }
  } catch {
    jsonError.value = "JSON 格式错误";
  }
}

function setField(f: { key: string }, value: unknown) {
  draft.value[f.key] = value;
  jsonText.value = JSON.stringify(draft.value, null, 2);
}

const inputCls =
  "w-full rounded-lg border border-border px-3 py-1.5 text-sm bg-transparent focus:outline-none focus:ring-1 focus:ring-primary";

// ---------- 渲染预览（复用设置页草稿预览管线 POST /preview/site） ----------
// 把当前草稿以保留名 __preview__ 注入草稿 widgets（不落库），leftbar 仅挂该组件，
// 后端用真实前台管线渲染整页后，前端提取该 widget 片段塞入 iframe。
const PREVIEW_NAME = "__preview__";

/** 组件 layout → 预览载体页面类型（数据上下文匹配） */
function previewPageFor(layout: string): string {
  if (["toc", "author", "related"].includes(layout)) return "post";
  if (["tree", "wiki_related"].includes(layout)) return "wiki";
  if (layout === "tagtree") return "notebooks";
  return "index";
}

const NO_OUTPUT_HINTS: Record<string, string> = {
  related: "专栏功能当前已禁用，前台不渲染此组件",
  tree: "文库组件仅文库文档页有数据，预览以首个项目模拟",
  wiki_related: "文库组件仅文库文档页有数据，预览以首个项目模拟",
  tagtree: "笔记本组件仅笔记本页有数据，预览以首个笔记本模拟",
  ghrepo: "需配置 repo/username/api 参数",
  ghuser: "需配置 repo/username/api 参数",
  ghissues: "需配置 repo/username/api 参数",
  timeline: "需配置 repo/username/api 参数",
};

const previewHtml = ref("");
const previewLoading = ref(false);
const previewError = ref("");
let previewTimer: ReturnType<typeof setTimeout> | undefined;
let previewSeq = 0;

async function refreshPreview() {
  const seq = ++previewSeq;
  previewLoading.value = true;
  previewError.value = "";
  try {
    const page = previewPageFor(draftLayout.value);
    const sections = {
      widgets: {
        library: { ...widgets.value.library, [PREVIEW_NAME]: cleanDraft() },
        pages: { [page]: { leftbar: [PREVIEW_NAME], rightbar: [] } },
      },
    };
    const { data } = await previewApi.site(sections, page);
    if (seq !== previewSeq) return;
    previewHtml.value = buildWidgetDoc(data?.html || "", draftLayout.value);
  } catch {
    if (seq === previewSeq) previewError.value = "预览渲染失败";
  } finally {
    if (seq === previewSeq) previewLoading.value = false;
  }
}

function schedulePreview(immediate = false) {
  clearTimeout(previewTimer);
  previewTimer = setTimeout(refreshPreview, immediate ? 0 : 500);
}

// 从整页 HTML 提取预览组件：保留侧栏视觉骨架（.l_left > .sidebg 玻璃质感 +
// .leftbar-container > .widgets 作用域样式：组件文本色/间距均绑定这些祖先选择器），
// 去掉 logo/菜单 header、footer 社交区与搜索框，只留预览组件本身。
// iframe 文档构建复用共享预览运行时（preview/preview-document.ts，Spec 63）：
// 内置 ctx/def/deps/utils shim（data_cache 关闭），body 仅保留页面 services scan 块
// （其 utils.js 调用由内置 shim 承接），head 脚本全量剥离、保留 stylesheet/style/meta。
function buildWidgetDoc(fullHtml: string, layout = ""): string {
  const doc = new DOMParser().parseFromString(fullHtml, "text/html");
  const aside = doc.querySelector("aside.l_left");
  let bodyInner = "";
  if (aside) {
    aside.querySelector("header.header")?.remove();
    aside.querySelector("footer.footer")?.remove();
    aside.querySelector(".search-wrapper")?.remove();
    const widget = aside.querySelector(".widgets > widget:last-child");
    if (!widget) {
      const hint = NO_OUTPUT_HINTS[layout] || "该组件在当前参数与页面类型下无输出（如 GitHub 系需配置 repo/username）";
      bodyInner = `<p style="color:var(--text-p3,#999);font-size:13px;padding:12px">${hint}</p>`;
      aside.querySelector(".widgets")?.remove();
    }
    bodyInner = bodyInner || aside.outerHTML;
  } else {
    bodyInner = `<p style="color:var(--text-p3,#999);font-size:13px;padding:12px">预览渲染失败：未找到侧栏结构</p>`;
  }
  const bodyClass = doc.body.className || "";
  // 数据服务懒加载 scan 块（base.html 注入，URL 已服务端渲染）：复用页面清单，
  // 内置 shim 提供 utils/ctx/def/deps，不再需要页面自身的 utils 内联块。
  const scanScr = Array.from(doc.body.querySelectorAll("script")).find(
    (s) => !s.src && (s.textContent || "").includes("var services = ["),
  );
  // iframe 内布局修正：侧栏宽度/定位不依赖整页 grid；容器高度自适应；去掉 .widgets 滚动与底部渐隐 mask
  const fx = `<style>
body{padding:12px !important}
aside.l_left{position:relative;width:300px;max-width:90%;margin:0 auto;max-height:none}
.l_left .leftbar-container{height:auto}
.l_left .leftbar-container >.widgets{overflow:visible;mask:none;-webkit-mask:none;flex-grow:0}
</style>`;
  // Spec 64：预览统一固定暗色（buildPreviewDoc 不接收主题参数）
  return buildPreviewDoc({
    bodyHtml: bodyInner,
    scripts: ["/assets/js/main.js", "/assets/js/tagcloud.js"],
    bodyClass,
    wrapContent: false,
    extraHead: fx,
    inlineScripts: scanScr ? [scanScr.outerHTML] : [],
  });
}

// 草稿参数变化 → 防抖重渲染；切换选中项立即渲染
watch(draft, () => schedulePreview(), { deep: true });
onBeforeUnmount(() => clearTimeout(previewTimer));
</script>

<template>
  <div class="h-full flex flex-col">
    <VLoading v-if="loading" />
    <div v-else-if="loadError" class="p-6 text-sm text-state-error">{{ loadError }}</div>

    <div v-else class="flex-1 flex flex-col overflow-hidden md:flex-row">
      <!-- 左侧：搜索 + 内置组件 + 已保存方案 -->
      <aside class="w-full shrink-0 border-b border-border flex flex-col bg-card md:w-64 md:border-r md:border-b-0">
        <div class="p-3 border-b border-border">
          <div class="relative">
            <Icon icon="ri:search-line" class="absolute left-2.5 top-1/2 -translate-y-1/2 text-text-quaternary" />
            <input
              v-model="search"
              placeholder="搜索组件 / 方案..."
              class="w-full pl-8 pr-3 py-1.5 text-sm rounded-lg border border-border focus:outline-none focus:ring-1 focus:ring-primary"
            />
          </div>
        </div>

        <div class="flex-1 overflow-y-auto py-2 max-h-64 md:max-h-none">
          <!-- 内置组件组 -->
          <p class="px-4 py-1 text-[11px] font-medium text-text-quaternary">内置组件 {{ filteredMeta.length }}</p>
          <button
            v-for="m in filteredMeta"
            :key="m.name"
            @click="select('builtin', m.name)"
            :class="[
              'w-full text-left px-4 py-2 flex items-center gap-2 transition-colors',
              isSelected('builtin', m.name) ? 'bg-primary/10' : 'hover:bg-accent',
            ]"
          >
            <span
              :class="[
                'text-sm truncate',
                isSelected('builtin', m.name) ? 'font-medium text-primary' : 'text-text-secondary',
              ]"
              >{{ m.label }}</span
            >
            <span class="ml-auto text-[11px] text-text-quaternary font-mono shrink-0">{{ m.name }}</span>
          </button>

          <!-- 已保存方案组 -->
          <p class="mt-2 px-4 py-1 text-[11px] font-medium text-text-quaternary">
            已保存方案 {{ filteredSchemes.length }}
          </p>
          <button
            v-for="n in filteredSchemes"
            :key="n"
            @click="select('scheme', n)"
            :class="[
              'w-full text-left px-4 py-2 flex items-center gap-2 transition-colors',
              isSelected('scheme', n) ? 'bg-primary/10' : 'hover:bg-accent',
            ]"
          >
            <Icon icon="ri:bookmark-line" class="text-primary text-sm shrink-0" />
            <span
              :class="[
                'text-sm truncate',
                isSelected('scheme', n) ? 'font-medium text-primary' : 'text-text-secondary',
              ]"
              >{{ n }}</span
            >
            <span class="ml-auto text-[10px] px-1 py-px rounded bg-primary/10 text-primary font-mono shrink-0">{{
              schemeLayout(n)
            }}</span>
          </button>
          <p v-if="!filteredSchemes.length" class="px-4 py-2 text-xs text-text-quaternary">
            暂无方案 — 选择内置组件编辑参数后「保存为方案」
          </p>
        </div>
      </aside>

      <!-- 右侧：详情 + 参数表单 + 保存操作 -->
      <section class="min-h-0 flex-1 overflow-y-auto p-6 space-y-5">
        <!-- 标题区 -->
        <div>
          <div class="flex items-center gap-2 flex-wrap">
            <h3 class="text-xl font-semibold text-foreground">
              {{ selKind === "builtin" ? selMeta?.label || selName : selName }}
            </h3>
            <code class="text-xs px-1.5 py-0.5 rounded bg-secondary text-text-tertiary font-mono">{{
              draftLayout
            }}</code>
            <span
              v-if="selKind === 'scheme'"
              class="text-[11px] px-1.5 py-0.5 rounded-full bg-primary/10 text-primary inline-flex items-center gap-1"
            >
              <Icon icon="ri:bookmark-line" />方案
            </span>
          </div>
          <p class="text-sm text-text-tertiary mt-1">
            {{ selKind === "builtin" ? selMeta?.desc : `基于 ${draftLayout} 布局的自定义方案` }}
          </p>
        </div>

        <!-- 参数表单 -->
        <div class="rounded-xl border border-border overflow-hidden">
          <div class="px-4 py-2 bg-secondary border-b border-border">
            <span class="text-xs font-medium text-text-tertiary">参数设置</span>
          </div>
          <div class="p-4">
            <template v-if="draftFields && draftFields.length">
              <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
                <div v-for="f in draftFields" :key="f.key" :class="f.type === 'textarea' ? 'md:col-span-2' : ''">
                  <label class="mb-1 flex items-baseline gap-1.5 text-xs text-text-tertiary">
                    {{ f.label || f.key }}
                    <code class="font-mono text-[10px] text-text-quaternary">{{ f.key }}</code>
                  </label>
                  <input
                    v-if="f.type === 'checkbox'"
                    type="checkbox"
                    :checked="!!draft[f.key]"
                    @change="setField(f, ($event.target as HTMLInputElement).checked)"
                    class="mt-1.5 h-4 w-4 cursor-pointer rounded border-border accent-primary"
                  />
                  <select
                    v-else-if="f.type === 'select'"
                    :value="draft[f.key]"
                    @change="setField(f, ($event.target as HTMLSelectElement).value)"
                    :class="inputCls"
                  >
                    <option v-for="o in f.options" :key="o" :value="o">{{ o }}</option>
                  </select>
                  <textarea
                    v-else-if="f.type === 'textarea'"
                    rows="4"
                    :value="draft[f.key]"
                    @input="setField(f, ($event.target as HTMLTextAreaElement).value)"
                    :placeholder="f.hint || ''"
                    :class="inputCls + ' font-mono resize-y'"
                  ></textarea>
                  <div v-else-if="f.type === 'color'" class="flex items-center gap-2">
                    <input
                      type="color"
                      :value="draft[f.key] || '#000000'"
                      @input="setField(f, ($event.target as HTMLInputElement).value)"
                      class="h-8 w-10 cursor-pointer rounded border border-border bg-transparent p-0.5"
                    />
                    <input
                      type="text"
                      :value="draft[f.key]"
                      @input="setField(f, ($event.target as HTMLInputElement).value)"
                      :class="inputCls + ' font-mono'"
                    />
                  </div>
                  <input
                    v-else
                    :type="f.type === 'number' ? 'number' : 'text'"
                    :value="draft[f.key]"
                    @input="
                      setField(
                        f,
                        f.type === 'number'
                          ? Number(($event.target as HTMLInputElement).value)
                          : ($event.target as HTMLInputElement).value,
                      )
                    "
                    :placeholder="f.hint || ''"
                    :class="inputCls + (f.type === 'number' ? '' : ' font-mono')"
                  />
                  <p
                    v-if="f.hint && f.type !== 'textarea' && f.type !== 'text'"
                    class="mt-1 text-[11px] text-text-quaternary"
                  >
                    {{ f.hint }}
                  </p>
                </div>
              </div>
            </template>

            <p v-else-if="noExtra" class="text-xs text-text-quaternary">该组件无额外可配置参数。</p>

            <!-- 未知 layout：JSON 兜底 -->
            <div v-else>
              <label class="mb-1 block text-xs text-text-tertiary">参数 (JSON)</label>
              <textarea
                :value="jsonText"
                @input="onJsonInput(($event.target as HTMLTextAreaElement).value)"
                rows="6"
                spellcheck="false"
                :class="inputCls + ' font-mono text-xs'"
              ></textarea>
              <p v-if="jsonError" class="mt-1 text-[11px] text-state-error">{{ jsonError }}</p>
            </div>
          </div>
        </div>

        <!-- 渲染预览（草稿参数，走前台真实渲染管线） -->
        <div class="rounded-xl border border-border overflow-hidden">
          <div class="flex items-center justify-between px-4 py-2 bg-secondary border-b border-border">
            <span class="text-xs font-medium text-text-tertiary flex items-center gap-1.5">
              渲染预览
              <span class="text-[11px] text-text-quaternary">草稿参数 · {{ previewPageFor(draftLayout) }} 页载体</span>
              <span v-if="previewLoading" class="text-[11px] text-text-quaternary">渲染中...</span>
            </span>
            <button
              @click="schedulePreview(true)"
              class="flex items-center gap-1 text-xs text-primary transition-opacity hover:opacity-80"
            >
              <Icon icon="ri:refresh-line" />
              刷新
            </button>
          </div>
          <div class="relative bg-secondary">
            <p v-if="previewError" class="px-4 py-6 text-center text-sm text-state-error">{{ previewError }}</p>
            <iframe
              v-else
              :srcdoc="previewHtml"
              class="w-full border-0"
              style="min-height: 260px; max-height: 480px; resize: vertical; overflow: auto"
              sandbox="allow-same-origin allow-scripts"
            ></iframe>
          </div>
        </div>

        <!-- 操作区 -->
        <div class="rounded-xl border border-border overflow-hidden">
          <div class="px-4 py-2 bg-secondary border-b border-border">
            <span class="text-xs font-medium text-text-tertiary">
              {{ selKind === "builtin" ? "保存为方案（可在设置 → 组件编排中选用）" : "方案操作" }}
            </span>
          </div>
          <div class="p-4 space-y-3">
            <!-- 内置组件：保存为方案 -->
            <template v-if="selKind === 'builtin'">
              <div class="flex items-center gap-2">
                <input
                  v-if="nameInputVisible"
                  v-model="nameInputValue"
                  placeholder="输入方案名，如「彩色标签云」"
                  class="max-w-xs"
                  :class="inputCls"
                  @keyup.enter="confirmNameInput"
                />
                <button
                  v-if="nameInputVisible"
                  @click="confirmNameInput"
                  :disabled="saving || !nameInputValue.trim()"
                  class="inline-flex items-center gap-1 rounded-full bg-primary px-4 py-1.5 text-sm font-medium text-foreground transition-opacity hover:opacity-90 disabled:opacity-50"
                >
                  保存为方案
                </button>
                <button
                  v-else
                  @click="openNameInput"
                  class="inline-flex items-center gap-1 rounded-full bg-primary px-4 py-1.5 text-sm font-medium text-foreground transition-opacity hover:opacity-90"
                >
                  <Icon icon="ri:bookmark-3-line" />
                  保存为方案
                </button>
                <span class="text-xs text-text-quaternary"> 以当前参数创建命名方案，不改动内置组件缺省 </span>
              </div>
              <p v-if="widgets.library[selName]" class="text-[11px] text-text-quaternary">
                当前 library 已存在同名覆盖配置（来自组件编排的参数编辑），上方表单已包含该覆盖。
              </p>
            </template>

            <!-- 已保存方案：保存 / 另存为 / 删除 -->
            <template v-else>
              <div class="flex items-center gap-2 flex-wrap">
                <button
                  @click="saveScheme"
                  :disabled="saving"
                  class="inline-flex items-center gap-1 rounded-full bg-primary px-4 py-1.5 text-sm font-medium text-foreground transition-opacity hover:opacity-90 disabled:opacity-50"
                >
                  保存
                </button>
                <button
                  @click="openNameInput"
                  class="inline-flex items-center gap-1 rounded-full border border-border px-4 py-1.5 text-sm text-text-secondary transition-colors hover:bg-accent"
                >
                  另存为
                </button>
                <button
                  @click="removeScheme"
                  class="inline-flex items-center gap-1 rounded-full border border-border px-4 py-1.5 text-sm text-state-error transition-colors hover:bg-state-error/10"
                >
                  <Icon icon="ri:delete-bin-line" />
                  删除
                </button>
                <span v-if="referencedPages(selName).length" class="text-xs text-text-quaternary">
                  被页面类型「{{ referencedPages(selName).join("、") }}」引用中
                </span>
              </div>
              <div v-if="nameInputVisible" class="flex items-center gap-2">
                <input
                  v-model="nameInputValue"
                  placeholder="输入新方案名"
                  class="max-w-xs"
                  :class="inputCls"
                  @keyup.enter="confirmNameInput"
                />
                <button
                  @click="confirmNameInput"
                  :disabled="saving || !nameInputValue.trim()"
                  class="inline-flex items-center gap-1 rounded-full bg-primary px-4 py-1.5 text-sm font-medium text-foreground transition-opacity hover:opacity-90 disabled:opacity-50"
                >
                  保存为新方案
                </button>
              </div>
            </template>
          </div>
        </div>
      </section>
    </div>
  </div>
</template>
